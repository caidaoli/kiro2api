package utils

import (
	"math"
	"strings"

	"kiro2api/config"
	"kiro2api/types"
)

// TokenEstimator 本地token估算器
// 设计原则：
// - KISS: 简单高效的估算算法，避免引入复杂的tokenizer库
// - 向后兼容: 支持所有Claude模型和消息格式
// - 性能优先: 本地计算，响应时间<5ms
type TokenEstimator struct{}

// NewTokenEstimator 创建token估算器实例
func NewTokenEstimator() *TokenEstimator {
	return &TokenEstimator{}
}

// EstimateTokens 估算消息的token数量
// 算法说明：
// - 基础估算: 英文平均4字符/token，中文平均1.5字符/token
// - 固定开销: 消息角色标记、JSON结构等
// - 工具开销: 每个工具定义约50-200 tokens
//
// 注意：此为快速估算，与官方tokenizer可能有±10%误差
func (e *TokenEstimator) EstimateTokens(req *types.CountTokensRequest) int {
	totalTokens := 0

	// 1. 系统提示词（system prompt）
	for _, sysMsg := range req.System {
		if sysMsg.Text != "" {
			totalTokens += e.EstimateTextTokens(sysMsg.Text)
			totalTokens += 2 // 系统提示的固定开销（P0优化：从3降至2）
		}
	}

	// 2. 消息内容（messages）
	for _, msg := range req.Messages {
		// 角色标记开销（"user"/"assistant" + JSON结构）
		// 优化：根据官方测试调整
		totalTokens += 3

		// 消息内容
		switch content := msg.Content.(type) {
		case string:
			// 文本消息
			totalTokens += e.EstimateTextTokens(content)
		case []any:
			// 复杂内容块（文本、图片、文档等）
			for _, block := range content {
				totalTokens += e.estimateContentBlock(block)
			}
		case []types.ContentBlock:
			// 类型化内容块
			for _, block := range content {
				totalTokens += e.estimateTypedContentBlock(block)
			}
		default:
			// 其他格式：保守估算为JSON长度
			if jsonBytes, err := SafeMarshal(content); err == nil {
				totalTokens += len(jsonBytes) / 4
			}
		}
	}

	// 3. 工具定义（tools）
	toolCount := len(req.Tools)
	if toolCount > 0 {
		// 工具开销策略：根据工具数量自适应调整
		// - 少量工具（1-3个）：每个工具高开销（包含大量元数据和结构信息）
		// - 大量工具（10+个）：共享开销 + 小增量（避免线性叠加过高）
		var baseToolsOverhead int
		var perToolOverhead int

		if toolCount == 1 {
			// 单工具场景：高开销（包含tools数组初始化、类型信息等）
			// 优化：平衡简单工具(403)和复杂工具(874)的估算
			baseToolsOverhead = 0
			perToolOverhead = 320 // 最优平衡值
		} else if toolCount <= 5 {
			// 少量工具：中等开销
			baseToolsOverhead = config.BaseToolsOverhead // 从150降至100
			perToolOverhead = 120                        // 从150降至120
		} else {
			// 大量工具：共享开销 + 低增量
			baseToolsOverhead = 180 // 从250降至180
			perToolOverhead = 60    // 从80降至60
		}

		totalTokens += baseToolsOverhead

		for _, tool := range req.Tools {
			// 工具名称（特殊处理：下划线分词导致token数增加）
			nameTokens := e.estimateToolName(tool.Name)
			totalTokens += nameTokens

			// 工具描述
			totalTokens += e.EstimateTextTokens(tool.Description)

			// 工具schema（JSON Schema）
			if tool.InputSchema != nil {
				if jsonBytes, err := SafeMarshal(tool.InputSchema); err == nil {
					// Schema编码密度：根据工具数量自适应
					// 优化：平衡编码密度
					var schemaCharsPerToken float64
					if toolCount == 1 {
						schemaCharsPerToken = 1.9 // 单工具平衡值
					} else if toolCount <= 5 {
						schemaCharsPerToken = 2.2 // 少量工具
					} else {
						schemaCharsPerToken = 2.5 // 大量工具
					}

					schemaLen := len(jsonBytes)
					schemaTokens := int(math.Ceil(float64(schemaLen) / schemaCharsPerToken))  // 进一法

					// $schema字段URL开销（优化：降低开销）
					if strings.Contains(string(jsonBytes), "$schema") {
						if toolCount == 1 {
							schemaTokens += 10 // 从15降至10
						} else {
							schemaTokens += 5 // 从8降至5
						}
					}

					// 最小schema开销（优化：降低最小值）
					minSchemaTokens := 50 // 从80降至50
					if toolCount > 5 {
						minSchemaTokens = 30 // 从40降至30
					}
					if schemaTokens < minSchemaTokens {
						schemaTokens = minSchemaTokens
					}

					totalTokens += schemaTokens
				}
			}

			totalTokens += perToolOverhead
		}
	}

	// 4. 基础请求开销（API格式固定开销）
	// 优化：根据官方测试调整
	totalTokens += 4 // 调整至4以匹配官方

	return totalTokens
}

// estimateToolName 估算工具名称的token数量
// 工具名称通常包含下划线、驼峰等特殊结构，tokenizer会进行更细粒度的分词
// 例如: "mcp__Playwright__browser_navigate_back"
// 可能被分为: ["mcp", "__", "Play", "wright", "__", "browser", "_", "navigate", "_", "back"]
func (e *TokenEstimator) estimateToolName(name string) int {
	if name == "" {
		return 0
	}

	// 基础估算：按字符长度（使用进一法）
	baseTokens := (len(name) + 1) / 2 // 工具名称通常极其密集（比普通文本密集2倍）

	// 下划线分词惩罚：每个下划线可能导致额外的token
	underscoreCount := strings.Count(name, "_")
	underscorePenalty := underscoreCount // 每个下划线约1个额外token

	// 驼峰分词惩罚：大写字母可能是分词边界
	camelCaseCount := 0
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			camelCaseCount++
		}
	}
	camelCasePenalty := camelCaseCount / 2 // 每2个大写字母约1个额外token

	totalTokens := baseTokens + underscorePenalty + camelCasePenalty
	if totalTokens < 2 {
		totalTokens = 2 // 最少2个token
	}

	return totalTokens
}

// EstimateTextTokens 估算纯文本的token数量
// 混合语言处理：
// - 检测中文字符比例
// - 中文: 1.5字符/token（汉字信息密度高）
// - 英文: 4字符/token（标准GPT tokenizer比率）
func (e *TokenEstimator) EstimateTextTokens(text string) int {
	if text == "" {
		return 0
	}

	// 转换为rune数组以正确计算Unicode字符数
	runes := []rune(text)
	runeCount := len(runes)

	if runeCount == 0 {
		return 0
	}

	// 统计中文字符数（扫描全部字符）
	chineseChars := 0
	for _, r := range runes {
		// 中文字符范围（CJK统一汉字）
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseChars++
		}
	}

	// 混合语言token估算
	// 根据官方测试数据精确校准：
	// 纯中文: '你'(1字符)→2tokens, '你好'(2字符)→3tokens
	// 混合: '你好hello'(2中+5英)→4tokens = 2中文 + 2英文
	// 结论: 纯中文有基础开销，混合文本无额外开销

	nonChineseChars := runeCount - chineseChars

	// 判断是否为纯中文
	isPureChinese := (nonChineseChars == 0)

	// 中文token计算
	chineseTokens := 0
	if chineseChars > 0 {
		if isPureChinese {
			chineseTokens = 1 + chineseChars // 纯中文: 基础1 + 字符数
		} else {
			chineseTokens = chineseChars // 混合文本: 仅字符数
		}
	}

	// 英文/数字字符密度优化
	// 短期优化: 进一步调整以降低纯英文误差
	nonChineseTokens := 0
	if nonChineseChars > 0 {
		// 根据文本长度动态调整字符密度
		var charsPerToken float64
		if nonChineseChars < 50 {
			// 超短文本(1-50字符): 密度低(分词多)
			charsPerToken = 2.8
		} else if nonChineseChars < 100 {
			// 短文本(50-100字符): 标准密度
			charsPerToken = 2.6
		} else {
			// 中长文本(100+字符): 密度高(更多常见词)
			charsPerToken = 2.5
		}

		nonChineseTokens = int(math.Ceil(float64(nonChineseChars) / charsPerToken))  // 进一法
		if nonChineseTokens < 1 {
			nonChineseTokens = 1 // 至少1 token
		}
	}

	tokens := chineseTokens + nonChineseTokens

	// 长文本压缩系数 (短期优化: 细化阈值)
	// 原因: BPE编码的token密度随文本长度增长而提高
	// 新增分段: 50/100/200/300/500/1000字符
	if runeCount >= config.LongTextThreshold {
		// 超长文本(1000+字符): 压缩40%
		tokens = int(float64(tokens) * 0.60)
	} else if runeCount >= 500 {
		// 长文本(500-1000字符): 压缩30%
		tokens = int(float64(tokens) * 0.70)
	} else if runeCount >= 300 {
		// 中长文本(300-500字符): 压缩20%
		tokens = int(float64(tokens) * 0.80)
	} else if runeCount >= 200 {
		// 中等文本(200-300字符): 压缩15%
		tokens = int(float64(tokens) * 0.85)
	} else if runeCount >= config.ShortTextThreshold {
		// 较长文本(100-200字符): 压缩10%
		tokens = int(float64(tokens) * 0.90)
	} else if runeCount >= 50 {
		// 普通文本(50-100字符): 压缩5%
		tokens = int(float64(tokens) * 0.95)
	}
	// <50字符: 不压缩

	if tokens < 1 {
		tokens = 1 // 最少1个token
	}

	return tokens
}

// EstimateToolUseTokens 精确估算工具调用的token数量
// 用于非流式响应，基于实际的工具调用信息进行精确计算
//
// 参数:
//   - toolName: 工具名称
//   - toolInput: 工具参数（map[string]any）
//
// 返回:
//   - 估算的token数量
//
// Token组成:
//   - 结构字段: "type", "id", "name", "input" 关键字
//   - 工具名称: 使用estimateToolName精确计算
//   - 参数内容: JSON序列化后的token数
//
// 设计原则:
//   - 精确计算: 基于实际工具调用信息，而非简单系数
//   - 一致性: 与EstimateTokens中的工具定义计算保持一致的方法
//   - 适用场景: 非流式响应（handlers.go），有完整工具信息
func (e *TokenEstimator) EstimateToolUseTokens(toolName string, toolInput map[string]any) int {
	totalTokens := 0

	// 1. JSON结构字段开销
	// "type": "tool_use" ≈ 3 tokens
	totalTokens += 3

	// "id": "toolu_01A09q90qw90lq917835lq9" ≈ 8 tokens
	// (固定格式的UUID，约8个token)
	totalTokens += 8

	// "name" 关键字 ≈ 1 token
	totalTokens += 1

	// 2. 工具名称（使用与输入侧相同的精确方法）
	nameTokens := e.estimateToolName(toolName)
	totalTokens += nameTokens

	// 3. "input" 关键字 ≈ 1 token
	totalTokens += 1

	// 4. 参数内容（JSON序列化）
	if len(toolInput) > 0 {
		if jsonBytes, err := SafeMarshal(toolInput); err == nil {
			// 使用标准的4字符/token比率
			inputTokens := len(jsonBytes) / config.TokenEstimationRatio
			totalTokens += inputTokens
		}
	} else {
		// 空参数对象 {} ≈ 1 token
		totalTokens += 1
	}

	return totalTokens
}

// estimateContentBlock 估算单个内容块的token数量（通用map格式）
// 支持的内容类型：
// - text: 文本块
// - image: 图片（固定1500 tokens估算）
// - document: 文档（根据大小估算）
func (e *TokenEstimator) estimateContentBlock(block any) int {
	blockMap, ok := block.(map[string]any)
	if !ok {
		return 10 // 未知格式，保守估算
	}

	blockType, _ := blockMap["type"].(string)

	switch blockType {
	case "text":
		// 文本块
		if text, ok := blockMap["text"].(string); ok {
			return e.EstimateTextTokens(text)
		}
		return 10

	case "image":
		// 图片：官方文档显示约1000-2000 tokens
		// 参考: https://docs.anthropic.com/en/docs/build-with-claude/vision
		return 1500

	case "document":
		// 文档：根据大小估算（简化处理）
		return 500

	case "tool_use":
		// 工具调用（在历史消息中的 assistant 消息可能包含）
		toolName, _ := blockMap["name"].(string)
		toolInput, _ := blockMap["input"].(map[string]any)
		return e.EstimateToolUseTokens(toolName, toolInput)

	case "tool_result":
		// 工具执行结果
		content := blockMap["content"]
		switch c := content.(type) {
		case string:
			return e.EstimateTextTokens(c)
		case []any:
			total := 0
			for _, item := range c {
				total += e.estimateContentBlock(item)
			}
			return total
		default:
			return 50
		}

	default:
		// 未知类型：JSON长度估算
		if jsonBytes, err := SafeMarshal(block); err == nil {
			return len(jsonBytes) / 4
		}
		return 10
	}
}

// estimateTypedContentBlock 估算类型化内容块的token数量
func (e *TokenEstimator) estimateTypedContentBlock(block types.ContentBlock) int {
	switch block.Type {
	case "text":
		if block.Text != nil {
			return e.EstimateTextTokens(*block.Text)
		}
		return 10

	case "image":
		// 图片：官方文档显示约1000-2000 tokens
		return 1500

	case "tool_use":
		// 工具调用（在历史消息中的 assistant 消息可能包含）
		toolName := ""
		if block.Name != nil {
			toolName = *block.Name
		}
		toolInput := make(map[string]any)
		if block.Input != nil {
			if input, ok := (*block.Input).(map[string]any); ok {
				toolInput = input
			}
		}
		return e.EstimateToolUseTokens(toolName, toolInput)

	case "tool_result":
		// 工具执行结果
		switch content := block.Content.(type) {
		case string:
			return e.EstimateTextTokens(content)
		case []any:
			total := 0
			for _, item := range content {
				total += e.estimateContentBlock(item)
			}
			return total
		default:
			return 50
		}

	default:
		// 未知类型
		return 10
	}
}

// IsValidClaudeModel 验证是否为有效的Claude模型
// 支持所有Claude系列模型（不限制具体版本号）
func IsValidClaudeModel(model string) bool {
	if model == "" {
		return false
	}

	model = strings.ToLower(model)

	// 支持的模型前缀
	validPrefixes := []string{
		"claude-",          // 所有Claude模型
		"gpt-",             // OpenAI兼容模式（codex渠道）
		"gemini-",          // Gemini兼容模式
		"text-",            // 传统completion模型
		"anthropic.claude", // Bedrock格式
	}

	for _, prefix := range validPrefixes {
		if strings.HasPrefix(model, prefix) {
			return true
		}
	}

	return false
}
