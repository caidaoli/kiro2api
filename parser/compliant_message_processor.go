package parser

import (
	"kiro2api/logger"
	"kiro2api/utils"
	"strings"
)

// CompliantMessageProcessor 符合规范的消息处理器
type CompliantMessageProcessor struct {
	sessionManager     *SessionManager
	toolManager        *ToolLifecycleManager
	eventHandlers      map[string]EventHandler // 统一的事件处理器（包含标准和旧格式）
	completionBuffer   []string
	legacyToolState    *toolIndexState               // 添加旧格式事件的工具状态
	toolDataAggregator *SonicStreamingJSONAggregator // 统一的工具调用数据聚合器
	// 运行时状态：跟踪已开始的工具与其内容块索引，用于按增量输出
	startedTools   map[string]bool
	toolBlockIndex map[string]int
}

// EventHandler 事件处理器接口
type EventHandler interface {
	Handle(message *EventStreamMessage) ([]SSEEvent, error)
}

// NewCompliantMessageProcessor 创建符合规范的消息处理器
func NewCompliantMessageProcessor() *CompliantMessageProcessor {
	processor := &CompliantMessageProcessor{
		sessionManager:   NewSessionManager(),
		toolManager:      NewToolLifecycleManager(),
		eventHandlers:    make(map[string]EventHandler),
		completionBuffer: make([]string, 0, 16),
		startedTools:     make(map[string]bool),
		toolBlockIndex:   make(map[string]int),
	}

	// 创建Sonic聚合器，并设置参数更新回调
	processor.toolDataAggregator = NewSonicStreamingJSONAggregatorWithCallback(
		func(toolUseId string, fullParams string) {
			// logger.Debug("Sonic聚合器回调：更新工具参数",
			// 	logger.String("toolUseId", toolUseId),
			// 	logger.String("fullParams", func() string {
			// 		if len(fullParams) > 100 {
			// 			return fullParams[:100] + "..."
			// 		}
			// 		return fullParams
			// 	}()))

			// 调用工具管理器更新参数
			processor.toolManager.UpdateToolArgumentsFromJSON(toolUseId, fullParams)
		})

	processor.registerEventHandlers()
	return processor
}

// Reset 重置处理器状态
func (cmp *CompliantMessageProcessor) Reset() {
	cmp.sessionManager.Reset()
	cmp.toolManager.Reset()
	cmp.completionBuffer = cmp.completionBuffer[:0]
	// 重置旧格式工具状态
	if cmp.legacyToolState != nil {
		cmp.legacyToolState.fullReset()
	}
}

// registerEventHandlers 注册所有事件处理器
func (cmp *CompliantMessageProcessor) registerEventHandlers() {
	// 标准事件处理器
	cmp.eventHandlers[EventTypes.COMPLETION] = &CompletionEventHandler{cmp}
	cmp.eventHandlers[EventTypes.COMPLETION_CHUNK] = &CompletionChunkEventHandler{cmp}
	cmp.eventHandlers[EventTypes.TOOL_CALL_REQUEST] = &ToolCallRequestHandler{cmp.toolManager}
	// 移除非标准事件处理器：TOOL_CALL_RESULT, TOOL_EXECUTION_START, TOOL_EXECUTION_END
	cmp.eventHandlers[EventTypes.TOOL_CALL_ERROR] = &ToolCallErrorHandler{cmp.toolManager}
	cmp.eventHandlers[EventTypes.SESSION_START] = &SessionStartHandler{cmp.sessionManager}
	cmp.eventHandlers[EventTypes.SESSION_END] = &SessionEndHandler{cmp.sessionManager}

	// 标准事件处理器 - 将assistantResponseEvent作为标准事件
	cmp.eventHandlers[EventTypes.ASSISTANT_RESPONSE_EVENT] = &StandardAssistantResponseEventHandler{cmp}

	// 旧格式兼容处理器（合并到统一的eventHandlers中）
	cmp.eventHandlers[EventTypes.TOOL_USE_EVENT] = &LegacyToolUseEventHandler{
		toolManager: cmp.toolManager,
		aggregator:  cmp.toolDataAggregator,
	}
}

// ProcessMessage 处理单个消息
func (cmp *CompliantMessageProcessor) ProcessMessage(message *EventStreamMessage) ([]SSEEvent, error) {
	messageType := message.GetMessageType()
	eventType := message.GetEventType()

	logger.Debug("处理消息",
		logger.String("message_type", messageType),
		logger.String("event_type", eventType),
		logger.Int("payload_len", len(message.Payload)),
		logger.String("payload_preview", func() string {
			if len(message.Payload) > 100 {
				return string(message.Payload[:100]) + "..."
			}
			return string(message.Payload)
		}()))

	// 根据消息类型分别处理
	switch messageType {
	case MessageTypes.EVENT:
		return cmp.processEventMessage(message, eventType)
	case MessageTypes.ERROR:
		return cmp.processErrorMessage(message)
	case MessageTypes.EXCEPTION:
		return cmp.processExceptionMessage(message)
	default:
		logger.Warn("未知消息类型", logger.String("message_type", messageType))
		return []SSEEvent{}, nil
	}
}

// processEventMessage 处理事件消息
func (cmp *CompliantMessageProcessor) processEventMessage(message *EventStreamMessage, eventType string) ([]SSEEvent, error) {
	// 查找并处理事件
	if handler, exists := cmp.eventHandlers[eventType]; exists {
		return handler.Handle(message)
	}

	// 未知事件类型，记录日志但不报错
	logger.Debug("未知事件类型",
		logger.String("event_type", eventType),
		logger.Any("available_handlers", func() []string {
			var keys []string
			for k := range cmp.eventHandlers {
				keys = append(keys, k)
			}
			return keys
		}()))
	return []SSEEvent{}, nil
}

// processErrorMessage 处理错误消息
func (cmp *CompliantMessageProcessor) processErrorMessage(message *EventStreamMessage) ([]SSEEvent, error) {
	var errorData map[string]any
	if len(message.Payload) > 0 {
		if err := utils.FastUnmarshal(message.Payload, &errorData); err != nil {
			logger.Warn("解析错误消息载荷失败", logger.Err(err))
			errorData = map[string]any{
				"message": string(message.Payload),
			}
		}
	}

	errorCode := ""
	errorMessage := ""

	if errorData != nil {
		if code, ok := errorData["__type"].(string); ok {
			errorCode = code
		}
		if msg, ok := errorData["message"].(string); ok {
			errorMessage = msg
		}
	}

	return []SSEEvent{
		{
			Event: "error",
			Data: map[string]any{
				"type":          "error",
				"error_code":    errorCode,
				"error_message": errorMessage,
				"raw_data":      errorData,
			},
		},
	}, nil
}

// processExceptionMessage 处理异常消息
func (cmp *CompliantMessageProcessor) processExceptionMessage(message *EventStreamMessage) ([]SSEEvent, error) {
	var exceptionData map[string]any
	if len(message.Payload) > 0 {
		if err := utils.FastUnmarshal(message.Payload, &exceptionData); err != nil {
			logger.Warn("解析异常消息载荷失败", logger.Err(err))
			exceptionData = map[string]any{
				"message": string(message.Payload),
			}
		}
	}

	exceptionType := ""
	exceptionMessage := ""

	if exceptionData != nil {
		if eType, ok := exceptionData["__type"].(string); ok {
			exceptionType = eType
		}
		if msg, ok := exceptionData["message"].(string); ok {
			exceptionMessage = msg
		}
	}

	return []SSEEvent{
		{
			Event: "exception",
			Data: map[string]any{
				"type":              "exception",
				"exception_type":    exceptionType,
				"exception_message": exceptionMessage,
				"raw_data":          exceptionData,
			},
		},
	}, nil
}

// GetSessionManager 获取会话管理器
func (cmp *CompliantMessageProcessor) GetSessionManager() *SessionManager {
	return cmp.sessionManager
}

// GetCompletionBuffer 获取聚合的完整内容
func (cmp *CompliantMessageProcessor) GetCompletionBuffer() string {
	if len(cmp.completionBuffer) == 0 {
		return ""
	}
	return strings.Join(cmp.completionBuffer, "")
}

// GetToolManager 获取工具管理器
func (cmp *CompliantMessageProcessor) GetToolManager() *ToolLifecycleManager {
	return cmp.toolManager
}

// === 事件处理器实现在 message_event_handlers.go 中 ===
