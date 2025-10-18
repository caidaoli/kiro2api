package server

import (
	"fmt"
	"io"
	"strings"

	"kiro2api/logger"
	"kiro2api/parser"
	"kiro2api/types"
	"kiro2api/utils"

	"github.com/gin-gonic/gin"
)

// StreamProcessorContext 流处理上下文，封装所有流处理状态
// 遵循单一职责原则：专注于流式数据处理
type StreamProcessorContext struct {
	// 请求上下文
	c           *gin.Context
	req         types.AnthropicRequest
	token       *types.TokenWithUsage
	sender      StreamEventSender
	messageID   string
	inputTokens int

	// 状态管理器
	sseStateManager   *SSEStateManager
	stopReasonManager *StopReasonManager
	tokenEstimator    *utils.TokenEstimator

	// 流解析器
	compliantParser *parser.CompliantEventStreamParser

	// 统计信息
	totalOutputTokens    int // 累计发送给客户端的输出 token 数
	totalReadBytes       int
	totalProcessedEvents int
	lastParseErr         error

	// 工具调用跟踪
	toolUseIdByBlockIndex map[int]string
	completedToolUseIds   map[string]bool // 已完成的工具ID集合（用于stop_reason判断）
	
	// *** 新增：JSON字节累加器（修复分段整除精度损失） ***
	// 问题：每个 input_json_delta 单独计算 len(partialJSON)/4 会导致小于4字节的分段被舍弃
	// 解决：累加每个块的JSON字节数，在 content_block_stop 时一次性计算 token
	jsonBytesByBlockIndex map[int]int // 每个工具块累积的JSON字节数
}

// NewStreamProcessorContext 创建流处理上下文
func NewStreamProcessorContext(
	c *gin.Context,
	req types.AnthropicRequest,
	token *types.TokenWithUsage,
	sender StreamEventSender,
	messageID string,
	inputTokens int,
) *StreamProcessorContext {
	return &StreamProcessorContext{
		c:                     c,
		req:                   req,
		token:                 token,
		sender:                sender,
		messageID:             messageID,
		inputTokens:           inputTokens,
		sseStateManager:       NewSSEStateManager(false),
		stopReasonManager:     NewStopReasonManager(req),
		tokenEstimator:        utils.NewTokenEstimator(),
		compliantParser:       parser.NewCompliantEventStreamParser(),
		toolUseIdByBlockIndex: make(map[int]string),
		completedToolUseIds:   make(map[string]bool),
		jsonBytesByBlockIndex: make(map[int]int), // *** 初始化JSON字节累加器 ***
	}
}

// Cleanup 清理资源
// 完整清理所有状态，防止内存泄漏
func (ctx *StreamProcessorContext) Cleanup() {
	// 重置解析器状态
	if ctx.compliantParser != nil {
		ctx.compliantParser.Reset()
	}

	// 清理工具调用映射
	if ctx.toolUseIdByBlockIndex != nil {
		// 清空map，释放内存
		for k := range ctx.toolUseIdByBlockIndex {
			delete(ctx.toolUseIdByBlockIndex, k)
		}
		ctx.toolUseIdByBlockIndex = nil
	}

	// 清理已完成工具集合
	if ctx.completedToolUseIds != nil {
		for k := range ctx.completedToolUseIds {
			delete(ctx.completedToolUseIds, k)
		}
		ctx.completedToolUseIds = nil
	}

	// 清理管理器引用，帮助GC
	ctx.sseStateManager = nil
	ctx.stopReasonManager = nil
	ctx.tokenEstimator = nil
}

// initializeSSEResponse 初始化SSE响应头
func initializeSSEResponse(c *gin.Context) error {
	// 设置SSE响应头，禁用反向代理缓冲
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 确认底层Writer支持Flush
	if _, ok := c.Writer.(io.Writer); !ok {
		return fmt.Errorf("writer不支持SSE刷新")
	}

	c.Writer.Flush()
	return nil
}

// sendInitialEvents 发送初始事件
func (ctx *StreamProcessorContext) sendInitialEvents(eventCreator func(string, int, string) []map[string]any) error {
	// 直接使用上下文中的 inputTokens（已经通过 TokenEstimator 精确计算）
	initialEvents := eventCreator(ctx.messageID, ctx.inputTokens, ctx.req.Model)

	// 注意：初始事件现在只包含 message_start 和 ping
	// content_block_start 会在收到实际内容时由 sse_state_manager 自动生成
	// 这避免了发送空内容块（如果上游只返回 tool_use 而没有文本）
	for _, event := range initialEvents {
		// 使用状态管理器发送事件
		if err := ctx.sseStateManager.SendEvent(ctx.c, ctx.sender, event); err != nil {
			logger.Error("初始SSE事件发送失败", logger.Err(err))
			return err
		}
	}

	return nil
}

// processToolUseStart 处理工具使用开始事件
func (ctx *StreamProcessorContext) processToolUseStart(dataMap map[string]any) {
	cb, ok := dataMap["content_block"].(map[string]any)
	if !ok {
		return
	}

	cbType, _ := cb["type"].(string)
	if cbType != "tool_use" {
		return
	}

	// 提取索引
	idx := extractIndex(dataMap)
	if idx < 0 {
		return
	}

	// 提取tool_use_id
	id, _ := cb["id"].(string)
	if id == "" {
		return
	}

	// 记录索引到tool_use_id的映射
	ctx.toolUseIdByBlockIndex[idx] = id

	logger.Debug("转发tool_use开始",
		logger.String("tool_use_id", id),
		logger.String("tool_name", getStringField(cb, "name")),
		logger.Int("index", idx))
}

// processToolUseStop 处理工具使用结束事件
func (ctx *StreamProcessorContext) processToolUseStop(dataMap map[string]any) {
	idx := extractIndex(dataMap)
	if idx < 0 {
		return
	}

	// *** 修复：在块结束时计算累加的JSON字节数的token ***
	// 使用进一法（向上取整）确保不低估token消耗
	if jsonBytes, exists := ctx.jsonBytesByBlockIndex[idx]; exists && jsonBytes > 0 {
		tokens := (jsonBytes + 3) / 4  // 进一法: ceil(jsonBytes / 4)
		ctx.totalOutputTokens += tokens
		delete(ctx.jsonBytesByBlockIndex, idx)
		
		logger.Debug("content_block_stop计算JSON tokens",
			logger.Int("block_index", idx),
			logger.Int("json_bytes", jsonBytes),
			logger.Int("tokens", tokens))
	}

	if toolId, exists := ctx.toolUseIdByBlockIndex[idx]; exists && toolId != "" {
		// *** 关键修复：在删除前先记录到已完成工具集合 ***
		// 问题：直接删除导致sendFinalEvents()中len(toolUseIdByBlockIndex)==0
		// 结果：stop_reason错误判断为end_turn而非tool_use
		// 解决：先添加到completedToolUseIds，保持工具调用的证据
		ctx.completedToolUseIds[toolId] = true

		delete(ctx.toolUseIdByBlockIndex, idx)
	} else {
		logger.Debug("非tool_use或未知索引的内容块结束",
			logger.Int("block_index", idx))
	}
}

// 直传模式：不再进行文本聚合

// sendFinalEvents 发送结束事件
func (ctx *StreamProcessorContext) sendFinalEvents() error {
	// 关闭所有未关闭的content_block
	activeBlocks := ctx.sseStateManager.GetActiveBlocks()
	for index, block := range activeBlocks {
		if block.Started && !block.Stopped {
			stopEvent := map[string]any{
				"type":  "content_block_stop",
				"index": index,
			}
			logger.Debug("最终事件前关闭未关闭的content_block", logger.Int("index", index))
			if err := ctx.sseStateManager.SendEvent(ctx.c, ctx.sender, stopEvent); err != nil {
				logger.Error("关闭content_block失败", logger.Err(err), logger.Int("index", index))
			}
		}
	}

	// 更新工具调用状态
	// 使用已完成工具集合来判断，因为toolUseIdByBlockIndex在stop时已被清空
	hasActiveTools := len(ctx.toolUseIdByBlockIndex) > 0
	hasCompletedTools := len(ctx.completedToolUseIds) > 0

	// logger.Debug("更新工具调用状态",
	// 	logger.Bool("has_active_tools", hasActiveTools),
	// 	logger.Bool("has_completed_tools", hasCompletedTools),
	// 	logger.Int("active_count", len(ctx.toolUseIdByBlockIndex)),
	// 	logger.Int("completed_count", len(ctx.completedToolUseIds)))

	ctx.stopReasonManager.UpdateToolCallStatus(hasActiveTools, hasCompletedTools)

	// *** 关键修复：使用累计的实际发送 token 数 ***
	// 设计原则：token 计费应该基于实际发送给客户端的 SSE 事件内容
	// totalOutputTokens 在每次发送事件时累计，确保与实际输出内容一致
	outputTokens := ctx.totalOutputTokens

	// *** 完善的最小 token 保护机制 ***
	// 问题：某些边缘情况（如只有空格、特殊字符等）可能导致 totalOutputTokens 为 0
	// 保护条件：只要处理了事件或有完成的内容块，output_tokens 就不应该为 0
	if outputTokens < 1 {
		// 检查是否有任何内容被发送
		hasContent := len(ctx.completedToolUseIds) > 0 ||
		              len(ctx.toolUseIdByBlockIndex) > 0 ||
		              ctx.totalProcessedEvents > 0

		if hasContent {
			outputTokens = 1  // 最小保护：至少 1 token
			logger.Debug("触发最小token保护",
				logger.Int("processed_events", ctx.totalProcessedEvents),
				logger.Int("completed_tools", len(ctx.completedToolUseIds)),
				logger.Int("active_tools", len(ctx.toolUseIdByBlockIndex)))
		}
	}

	// 确定stop_reason
	stopReason := ctx.stopReasonManager.DetermineStopReason()

	logger.Debug("创建结束事件",
		logger.String("stop_reason", stopReason),
		logger.String("stop_reason_description", GetStopReasonDescription(stopReason)),
		logger.Int("output_tokens", outputTokens))

	// 创建并发送结束事件
	finalEvents := createAnthropicFinalEvents(outputTokens, ctx.inputTokens, stopReason)
	for _, event := range finalEvents {
		if err := ctx.sseStateManager.SendEvent(ctx.c, ctx.sender, event); err != nil {
			logger.Error("结束事件发送违规", logger.Err(err))
		}
	}

	return nil
}

// 辅助函数

// extractIndex 从数据映射中提取索引
func extractIndex(dataMap map[string]any) int {
	if v, ok := dataMap["index"].(int); ok {
		return v
	}
	if f, ok := dataMap["index"].(float64); ok {
		return int(f)
	}
	return -1
}

// getStringField 从映射中安全提取字符串字段
func getStringField(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// EventStreamProcessor 事件流处理器
// 遵循单一职责原则：专注于处理事件流
type EventStreamProcessor struct {
	ctx *StreamProcessorContext
}

// NewEventStreamProcessor 创建事件流处理器
func NewEventStreamProcessor(ctx *StreamProcessorContext) *EventStreamProcessor {
	return &EventStreamProcessor{
		ctx: ctx,
	}
}

// ProcessEventStream 处理事件流的主循环
func (esp *EventStreamProcessor) ProcessEventStream(reader io.Reader) error {
	buf := make([]byte, 1024)

	for {
		n, err := reader.Read(buf)
		esp.ctx.totalReadBytes += n

		if n > 0 {
			// 解析事件流
			events, parseErr := esp.ctx.compliantParser.ParseStream(buf[:n])
			esp.ctx.lastParseErr = parseErr

			if parseErr != nil {
				logger.Warn("符合规范的解析器处理失败",
					addReqFields(esp.ctx.c,
						logger.Err(parseErr),
						logger.Int("read_bytes", n),
						logger.String("direction", "upstream_response"),
					)...)
			}

			esp.ctx.totalProcessedEvents += len(events)

			// 处理每个事件
			for _, event := range events {
				if err := esp.processEvent(event); err != nil {
					return err
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				logger.Debug("响应流结束",
					addReqFields(esp.ctx.c,
						logger.Int("total_read_bytes", esp.ctx.totalReadBytes),
					)...)
			} else {
				logger.Error("读取响应流时发生错误",
					addReqFields(esp.ctx.c,
						logger.Err(err),
						logger.Int("total_read_bytes", esp.ctx.totalReadBytes),
						logger.String("direction", "upstream_response"),
					)...)
			}
			break
		}
	}

	// 直传模式：无需冲刷剩余文本
	return nil
}

// processEvent 处理单个事件
func (esp *EventStreamProcessor) processEvent(event parser.SSEEvent) error {
	dataMap, ok := event.Data.(map[string]any)
	if !ok {
		logger.Warn("事件数据类型不匹配,跳过", logger.String("event_type", event.Event))
		return nil
	}

	eventType, _ := dataMap["type"].(string)

	// 处理不同类型的事件
	switch eventType {
	case "content_block_start":
		esp.ctx.processToolUseStart(dataMap)

	case "content_block_delta":
		// 直传：不做聚合
		// 但需要统计输出字符数（在后面统一处理）

	case "content_block_stop":
		esp.ctx.processToolUseStop(dataMap)

	case "message_delta":

	case "exception":
		// 处理上游异常事件，检查是否需要映射为max_tokens
		if esp.handleExceptionEvent(dataMap) {
			return nil // 已转换并发送，不转发原始exception事件
		}
	}

	// 使用状态管理器发送事件（直传）
	if err := esp.ctx.sseStateManager.SendEvent(esp.ctx.c, esp.ctx.sender, dataMap); err != nil {
		logger.Error("SSE事件发送违规", logger.Err(err))
		// 非严格模式下，违规事件被跳过但不中断流
	}

	// *** 关键修复：基于实际发送的 SSE 事件内容累计 token ***
	// 设计原则：只统计包含实际内容的事件，忽略结构性事件
	// 原因：
	// 1. 计费准确性：客户端消费的是实际内容，而不是事件结构
	// 2. 一致性：与非流式响应的 token 计算逻辑保持一致
	// 3. 符合 Claude 官方计费规则：只计算内容 token，不计算结构开销
	switch eventType {
	case "content_block_delta":
		// 内容增量事件：累计实际文本或 JSON 内容的 token
		if delta, ok := dataMap["delta"].(map[string]any); ok {
			deltaType, _ := delta["type"].(string)
			
			switch deltaType {
			case "text_delta":
				// 文本内容增量
				if text, ok := delta["text"].(string); ok {
					esp.ctx.totalOutputTokens += esp.ctx.tokenEstimator.EstimateTextTokens(text)
				}
			
			case "input_json_delta":
				// *** 修复：累加JSON字节数，延迟到content_block_stop时统一计算 ***
				// 问题：分段整除导致精度损失（例如 3字节/4=0, 2字节/4=0）
				// 解决：累加所有分段的字节数，在块结束时一次性计算 token
				if partialJSON, ok := delta["partial_json"].(string); ok {
					index := extractIndex(dataMap)
					esp.ctx.jsonBytesByBlockIndex[index] += len(partialJSON)
				}
			}
		}
	
	case "content_block_start":
		// 内容块开始事件：累计结构性 token
		// 根据 Claude 官方文档，tool_use 块的结构字段（type, id, name）也会消耗 token
		if contentBlock, ok := dataMap["content_block"].(map[string]any); ok {
			blockType, _ := contentBlock["type"].(string)
			
			if blockType == "tool_use" {
				// 工具调用结构开销：
				// - "type": "tool_use" ≈ 3 tokens
				// - "id": "toolu_xxx" ≈ 8 tokens  
				// - "name" 关键字 ≈ 1 token
				// - 工具名称本身的 token（使用 estimateToolName 计算）
				esp.ctx.totalOutputTokens += 12 // 结构字段固定开销
				
				if toolName, ok := contentBlock["name"].(string); ok {
					esp.ctx.totalOutputTokens += esp.ctx.tokenEstimator.EstimateTextTokens(toolName)
				}
			}
		}
	
	// 其他事件类型（message_start, content_block_stop, message_delta, message_stop 等）
	// 不包含实际内容，不累计 token
	}

	esp.ctx.c.Writer.Flush()
	return nil
}

// processContentBlockDelta 处理content_block_delta事件
// 返回true表示已处理（聚合），不需要转发原始事件
// processContentBlockDelta 已废弃（直传模式不再需要）

// handleExceptionEvent 处理上游异常事件，检查是否需要映射为max_tokens
// 返回true表示已处理并转换，不需要转发原始exception事件
func (esp *EventStreamProcessor) handleExceptionEvent(dataMap map[string]any) bool {
	// 提取异常类型
	exceptionType, _ := dataMap["exception_type"].(string)

	// 检查是否为内容长度超限异常
	if exceptionType == "ContentLengthExceededException" ||
		strings.Contains(exceptionType, "CONTENT_LENGTH_EXCEEDS") {

		logger.Info("检测到内容长度超限异常，映射为max_tokens stop_reason",
			addReqFields(esp.ctx.c,
				logger.String("exception_type", exceptionType),
				logger.String("claude_stop_reason", "max_tokens"))...)

		// 关闭所有活跃的content_block
		activeBlocks := esp.ctx.sseStateManager.GetActiveBlocks()
		for index, block := range activeBlocks {
			if block.Started && !block.Stopped {
				stopEvent := map[string]any{
					"type":  "content_block_stop",
					"index": index,
				}
				_ = esp.ctx.sseStateManager.SendEvent(esp.ctx.c, esp.ctx.sender, stopEvent)
			}
		}

		// 构造符合Claude规范的max_tokens响应
		maxTokensEvent := map[string]any{
			"type": "message_delta",
			"delta": map[string]any{
				"stop_reason":   "max_tokens",
				"stop_sequence": nil,
			},
			"usage": map[string]any{
				"input_tokens":  esp.ctx.inputTokens,
				"output_tokens": esp.ctx.totalOutputTokens,
			},
		}

		// 发送max_tokens事件
		if err := esp.ctx.sseStateManager.SendEvent(esp.ctx.c, esp.ctx.sender, maxTokensEvent); err != nil {
			logger.Error("发送max_tokens响应失败", logger.Err(err))
			return false
		}

		// 发送message_stop事件
		stopEvent := map[string]any{
			"type": "message_stop",
		}
		if err := esp.ctx.sseStateManager.SendEvent(esp.ctx.c, esp.ctx.sender, stopEvent); err != nil {
			logger.Error("发送message_stop失败", logger.Err(err))
			return false
		}

		esp.ctx.c.Writer.Flush()

		return true // 已转换并发送，不转发原始exception
	}

	// 其他类型的异常，正常转发
	return false
}

// 直传模式：无flush逻辑
