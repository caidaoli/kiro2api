package utils

import (
	"math"
	"testing"

	"kiro2api/types"
)

// TestCase 定义测试用例结构
type TestCase struct {
	Name        string                    // 测试名称
	Request     *types.CountTokensRequest // 测试请求
	Expected    int                       // 预期token数（来自真实API或标准）
	Description string                    // 测试场景描述
}

// calculateError 计算误差百分比
func calculateError(estimated, expected int) float64 {
	if expected == 0 {
		return 0
	}
	return math.Abs(float64(estimated-expected)) / float64(expected) * 100
}

// TestTokenEstimatorAccuracy 测试本地token估算器的精确度
func TestTokenEstimatorAccuracy(t *testing.T) {
	estimator := NewTokenEstimator()

	// 定义测试用例
	testCases := []TestCase{
		// 1. 基础文本测试
		{
			Name: "简单英文消息",
			Request: &types.CountTokensRequest{
				Messages: []types.AnthropicRequestMessage{
					{
						Role:    "user",
						Content: "Hello, how are you today?",
					},
				},
			},
			Expected:    13,
			Description: "纯英文短消息",
		},
		{
			Name: "简单中文消息",
			Request: &types.CountTokensRequest{
				Messages: []types.AnthropicRequestMessage{
					{
						Role:    "user",
						Content: "你好，今天天气怎么样？",
					},
				},
			},
			Expected:    18,
			Description: "纯中文短消息",
		},
		{
			Name: "中英混合消息",
			Request: &types.CountTokensRequest{
				Messages: []types.AnthropicRequestMessage{
					{
						Role:    "user",
						Content: "你好world，今天的weather很好",
					},
				},
			},
			Expected:    20,
			Description: "中英混合文本",
		},

		// 2. 系统提示词测试
		{
			Name: "带系统提示词",
			Request: &types.CountTokensRequest{
				System: []types.AnthropicSystemMessage{
					{
						Type: "text",
						Text: "You are a helpful assistant.",
					},
				},
				Messages: []types.AnthropicRequestMessage{
					{
						Role:    "user",
						Content: "Hello!",
					},
				},
			},
			Expected:    18,
			Description: "测试系统提示词的token开销",
		},

		// 3. 单工具测试
		{
			Name: "单个简单工具",
			Request: &types.CountTokensRequest{
				Messages: []types.AnthropicRequestMessage{
					{
						Role:    "user",
						Content: "What is the weather?",
					},
				},
				Tools: []types.AnthropicTool{
					{
						Name:        "get_weather",
						Description: "Get current weather for a location",
						InputSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"location": map[string]any{
									"type":        "string",
									"description": "City name",
								},
							},
							"required": []string{"location"},
						},
					},
				},
			},
			Expected:    403,
			Description: "单工具场景",
		},

		// 4. 多工具测试
		{
			Name: "3个工具",
			Request: &types.CountTokensRequest{
				Messages: []types.AnthropicRequestMessage{
					{
						Role:    "user",
						Content: "Help me with tasks",
					},
				},
				Tools: []types.AnthropicTool{
					{
						Name:        "get_weather",
						Description: "Get weather",
						InputSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"location": map[string]any{"type": "string"},
							},
						},
					},
					{
						Name:        "search_web",
						Description: "Search the web",
						InputSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"query": map[string]any{"type": "string"},
							},
						},
					},
					{
						Name:        "send_email",
						Description: "Send an email",
						InputSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"to":      map[string]any{"type": "string"},
								"subject": map[string]any{"type": "string"},
								"body":    map[string]any{"type": "string"},
							},
						},
					},
				},
			},
			Expected:    650,
			Description: "多工具场景",
		},

		// 5. 复杂工具名测试
		{
			Name: "MCP风格工具名",
			Request: &types.CountTokensRequest{
				Messages: []types.AnthropicRequestMessage{
					{
						Role:    "user",
						Content: "Navigate back",
					},
				},
				Tools: []types.AnthropicTool{
					{
						Name:        "mcp__Playwright__browser_navigate_back",
						Description: "Navigate to previous page",
						InputSchema: map[string]any{
							"type":       "object",
							"properties": map[string]any{},
						},
					},
				},
			},
			Expected:    380,
			Description: "测试长工具名（下划线、驼峰）",
		},

		// 6. 长文本测试
		{
			Name: "长文本消息",
			Request: &types.CountTokensRequest{
				Messages: []types.AnthropicRequestMessage{
					{
						Role: "user",
						Content: `Please analyze the following code and provide suggestions for improvement.
The code implements a token estimation algorithm that uses heuristic rules to estimate
token counts for AI model inputs. It handles multiple scenarios including text messages,
system prompts, tool definitions, and complex content blocks. The algorithm considers
factors like character density, language type (Chinese vs English), and structural overhead.`,
					},
				},
			},
			Expected:    95,
			Description: "长文本消息",
		},

		// 7. 复杂内容块测试
		{
			Name: "多类型内容块",
			Request: &types.CountTokensRequest{
				Messages: []types.AnthropicRequestMessage{
					{
						Role: "user",
						Content: []types.ContentBlock{
							{
								Type: "text",
								Text: stringPtr("Analyze this image:"),
							},
							{
								Type: "image",
								Source: &types.ImageSource{
									Type:      "base64",
									MediaType: "image/jpeg",
									Data:      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
								},
							},
						},
					},
				},
			},
			Expected:    1515,
			Description: "混合内容块（文本+图片）",
		},
	}

	// 执行测试
	t.Logf("\n=== Token估算精确度测试 ===\n")
	t.Logf("%-30s | %10s | %10s | %10s | %s\n",
		"测试名称", "估算值", "预期值", "误差", "状态")
	t.Logf("%s\n", "--------------------------------------------------------------------------------")

	totalError := 0.0
	passCount := 0

	for _, tc := range testCases {
		estimated := estimator.EstimateTokens(tc.Request)
		error := calculateError(estimated, tc.Expected)
		totalError += error

		status := "✅"
		if error > 20 {
			status = "⚠️"
		}
		if error > 50 {
			status = "❌"
		}

		if error <= 15 {
			passCount++
		}

		t.Logf("%-30s | %10d | %10d | %9.2f%% | %s\n",
			tc.Name, estimated, tc.Expected, error, status)
	}

	avgError := totalError / float64(len(testCases))
	accuracy := float64(passCount) / float64(len(testCases)) * 100

	t.Logf("\n=== 统计汇总 ===\n")
	t.Logf("测试用例总数: %d\n", len(testCases))
	t.Logf("优秀(<15%%误差): %d/%d (%.1f%%)\n", passCount, len(testCases), accuracy)
	t.Logf("平均误差: %.2f%%\n", avgError)

	// 判定标准
	if avgError < 10 {
		t.Logf("✅ 卓越 - 平均误差<10%%\n")
	} else if avgError < 15 {
		t.Logf("✅ 优秀 - 平均误差<15%%\n")
	} else if avgError < 20 {
		t.Logf("⚠️ 良好 - 平均误差<20%%\n")
	} else {
		t.Logf("⚠️ 需改进 - 平均误差>20%%\n")
	}
}

// BenchmarkTokenEstimator 性能基准测试
func BenchmarkTokenEstimator(b *testing.B) {
	estimator := NewTokenEstimator()

	req := &types.CountTokensRequest{
		Messages: []types.AnthropicRequestMessage{
			{
				Role:    "user",
				Content: "Hello, world! How are you today?",
			},
		},
		Tools: []types.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get weather information",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		estimator.EstimateTokens(req)
	}
}

// 辅助函数
func stringPtr(s string) *string {
	return &s
}

// TestEstimateOutputTokens 测试输出token估算方法
func TestEstimateOutputTokens(t *testing.T) {
	estimator := NewTokenEstimator()

	tests := []struct {
		name        string
		textContent string
		hasToolUse  bool
		expected    int
		description string
	}{
		{
			name:        "简单文本无工具",
			textContent: "Hello, world!",
			hasToolUse:  false,
			expected:    4,
			description: "纯文本响应，无工具调用",
		},
		{
			name:        "简单文本有工具",
			textContent: "Hello, world!",
			hasToolUse:  true,
			expected:    5,
			description: "文本响应 + 工具调用结构化开销",
		},
		{
			name:        "空文本",
			textContent: "",
			hasToolUse:  false,
			expected:    0,
			description: "空响应",
		},
		{
			name:        "长文本无工具",
			textContent: "This is a longer text response that contains multiple sentences.",
			hasToolUse:  false,
			expected:    22,
			description: "长文本响应，无工具调用",
		},
		{
			name:        "长文本有工具",
			textContent: "This is a longer text response that contains multiple sentences.",
			hasToolUse:  true,
			expected:    26,
			description: "长文本响应 + 工具调用结构化开销",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimator.EstimateOutputTokens(tt.textContent, tt.hasToolUse)

			// 允许±30%的误差
			tolerance := float64(tt.expected) * 0.3
			diff := math.Abs(float64(result - tt.expected))

			if diff > tolerance {
				t.Errorf("%s: 估算值=%d, 预期值=%d, 误差=%.1f%%",
					tt.name, result, tt.expected, diff/float64(tt.expected)*100)
			} else {
				t.Logf("✅ %s: 估算值=%d, 预期值=%d",
					tt.name, result, tt.expected)
			}
		})
	}
}

// TestEstimateOutputTokens_Consistency 测试与原有逻辑的一致性
func TestEstimateOutputTokens_Consistency(t *testing.T) {
	estimator := NewTokenEstimator()

	testCases := []struct {
		textContent string
		hasToolUse  bool
	}{
		{"Hello", false},
		{"Hello", true},
		{"This is a test message", false},
		{"This is a test message", true},
		{"你好世界", false},
		{"你好世界", true},
		{"", false},
		{"", true},
	}

	for _, tc := range testCases {
		// 新方法
		newResult := estimator.EstimateOutputTokens(tc.textContent, tc.hasToolUse)

		// 原有逻辑（模拟）
		baseTokens := estimator.EstimateTextTokens(tc.textContent)
		oldResult := baseTokens
		if tc.hasToolUse {
			oldResult = int(float64(baseTokens) * 1.2)
		}
		if oldResult < 1 && len(tc.textContent) > 0 {
			oldResult = 1
		}

		if newResult != oldResult {
			t.Errorf("不一致: textContent=%q, hasToolUse=%v, 新方法=%d, 原逻辑=%d",
				tc.textContent, tc.hasToolUse, newResult, oldResult)
		}
	}

	t.Logf("✅ 所有测试用例与原有逻辑一致")
}
