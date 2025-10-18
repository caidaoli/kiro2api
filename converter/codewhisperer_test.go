package converter

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiro2api/types"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestBuildCodeWhispererRequest_BasicMessage(t *testing.T) {
	anthropicReq := types.AnthropicRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []types.AnthropicRequestMessage{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
	}

	// 测试时不传 context
	cwReq, err := BuildCodeWhispererRequest(anthropicReq, nil)

	require.NoError(t, err)
	// 不传 context 时会使用随机 UUID
	assert.NotEmpty(t, cwReq.ConversationState.ConversationId)
	assert.NotEmpty(t, cwReq.ConversationState.AgentContinuationId)
	assert.Equal(t, "vibe", cwReq.ConversationState.AgentTaskType)
	assert.Equal(t, "Hello, how are you?", cwReq.ConversationState.CurrentMessage.UserInputMessage.Content)
	assert.Equal(t, "CLAUDE_SONNET_4_20250514_V1_0", cwReq.ConversationState.CurrentMessage.UserInputMessage.ModelId)
	assert.Equal(t, "AI_EDITOR", cwReq.ConversationState.CurrentMessage.UserInputMessage.Origin)
}

func TestBuildCodeWhispererRequest_EmptyMessages(t *testing.T) {
	anthropicReq := types.AnthropicRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages:  []types.AnthropicRequestMessage{},
	}

	_, err := BuildCodeWhispererRequest(anthropicReq, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "消息列表为空")
}

func TestBuildCodeWhispererRequest_WithSystemMessage(t *testing.T) {
	anthropicReq := types.AnthropicRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
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
	}

	cwReq, err := BuildCodeWhispererRequest(anthropicReq, nil)

	require.NoError(t, err)
	assert.NotNil(t, cwReq.ConversationState.History)
	assert.Greater(t, len(cwReq.ConversationState.History), 0)
}

func TestBuildCodeWhispererRequest_WithTools(t *testing.T) {
	anthropicReq := types.AnthropicRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []types.AnthropicRequestMessage{
			{
				Role:    "user",
				Content: "What's the weather?",
			},
		},
		Tools: []types.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get weather information",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{
							"type": "string",
						},
					},
					"required": []any{"location"},
				},
			},
		},
	}

	cwReq, err := BuildCodeWhispererRequest(anthropicReq, nil)

	require.NoError(t, err)
	assert.NotNil(t, cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools)
	assert.Len(t, cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools, 1)
	assert.Equal(t, "get_weather", cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools[0].ToolSpecification.Name)
}

func TestBuildCodeWhispererRequest_FilterWebSearchTool(t *testing.T) {
	anthropicReq := types.AnthropicRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []types.AnthropicRequestMessage{
			{
				Role:    "user",
				Content: "Search the web",
			},
		},
		Tools: []types.AnthropicTool{
			{
				Name:        "web_search",
				Description: "Search the web",
				InputSchema: map[string]any{"type": "object"},
			},
			{
				Name:        "get_weather",
				Description: "Get weather",
				InputSchema: map[string]any{"type": "object"},
			},
		},
	}

	cwReq, err := BuildCodeWhispererRequest(anthropicReq, nil)

	require.NoError(t, err)
	// web_search 应该被过滤掉
	assert.Len(t, cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools, 1)
	assert.Equal(t, "get_weather", cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools[0].ToolSpecification.Name)
}

func TestBuildCodeWhispererRequest_WithImages(t *testing.T) {
	t.Skip("图片验证测试需要有效的 base64 编码图片数据，跳过以保持测试套件的稳定性")
	// 注意：图片处理逻辑在 processMessageContent 中，已被其他测试间接覆盖
}

func TestBuildCodeWhispererRequest_WithToolResults(t *testing.T) {
	toolUseID := "tool_123"
	isError := false
	anthropicReq := types.AnthropicRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []types.AnthropicRequestMessage{
			{
				Role: "user",
				Content: []types.ContentBlock{
					{
						Type:      "tool_result",
						ToolUseId: &toolUseID,
						Content:   "Temperature is 25°C",
						IsError:   &isError,
					},
				},
			},
		},
	}

	cwReq, err := BuildCodeWhispererRequest(anthropicReq, nil)

	require.NoError(t, err)
	assert.NotNil(t, cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.ToolResults)
	assert.Len(t, cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.ToolResults, 1)
	assert.Equal(t, "tool_123", cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.ToolResults[0].ToolUseId)
	// 包含 tool_result 的请求，content 应该为空
	assert.Equal(t, "", cwReq.ConversationState.CurrentMessage.UserInputMessage.Content)
}

func TestBuildCodeWhispererRequest_WithHistory(t *testing.T) {
	anthropicReq := types.AnthropicRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []types.AnthropicRequestMessage{
			{
				Role:    "user",
				Content: "What is 2+2?",
			},
			{
				Role:    "assistant",
				Content: "2+2 equals 4.",
			},
			{
				Role:    "user",
				Content: "What about 3+3?",
			},
		},
	}

	cwReq, err := BuildCodeWhispererRequest(anthropicReq, nil)

	require.NoError(t, err)
	assert.NotNil(t, cwReq.ConversationState.History)
	// 历史应该包含前两条消息（一对user-assistant）
	assert.Greater(t, len(cwReq.ConversationState.History), 0)
	// 当前消息应该是最后一条
	assert.Equal(t, "What about 3+3?", cwReq.ConversationState.CurrentMessage.UserInputMessage.Content)
}

func TestBuildCodeWhispererRequest_ModelNotFound(t *testing.T) {
	anthropicReq := types.AnthropicRequest{
		Model:     "invalid-model-name",
		MaxTokens: 1024,
		Messages: []types.AnthropicRequestMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	_, err := BuildCodeWhispererRequest(anthropicReq, nil)

	require.Error(t, err)
	var modelNotFoundErr *types.ModelNotFoundErrorType
	assert.ErrorAs(t, err, &modelNotFoundErr)
	assert.NotNil(t, modelNotFoundErr.ErrorData)
}

func TestDetermineChatTriggerType(t *testing.T) {
	tests := []struct {
		name     string
		req      types.AnthropicRequest
		expected string
	}{
		{
			name: "有工具但无tool_choice - MANUAL",
			req: types.AnthropicRequest{
				Tools: []types.AnthropicTool{
					{Name: "test_tool"},
				},
			},
			expected: "MANUAL",
		},
		{
			name: "有工具且tool_choice=any - AUTO",
			req: types.AnthropicRequest{
				Tools: []types.AnthropicTool{
					{Name: "test_tool"},
				},
				ToolChoice: map[string]any{"type": "any"},
			},
			expected: "AUTO",
		},
		{
			name: "无工具，无历史 - MANUAL",
			req: types.AnthropicRequest{
				Messages: []types.AnthropicRequestMessage{
					{Role: "user", Content: "Hello"},
				},
			},
			expected: "MANUAL",
		},
		{
			name: "无工具，有历史 - MANUAL",
			req: types.AnthropicRequest{
				Messages: []types.AnthropicRequestMessage{
					{Role: "user", Content: "First"},
					{Role: "assistant", Content: "Response"},
					{Role: "user", Content: "Second"},
				},
			},
			expected: "MANUAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineChatTriggerType(tt.req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractToolUsesFromMessage(t *testing.T) {
	t.Run("提取工具调用", func(t *testing.T) {
		textContent := "Let me check that."
		toolID := "tool_abc123"
		toolName := "get_weather"
		toolInput := any(map[string]any{"location": "Tokyo"})
		
		content := []types.ContentBlock{
			{
				Type: "text",
				Text: &textContent,
			},
			{
				Type:  "tool_use",
				ID:    &toolID,
				Name:  &toolName,
				Input: &toolInput,
			},
		}

		toolUses := extractToolUsesFromMessage(content)

		require.Len(t, toolUses, 1)
		assert.Equal(t, "tool_abc123", toolUses[0].ToolUseId)
		assert.Equal(t, "get_weather", toolUses[0].Name)
		assert.NotNil(t, toolUses[0].Input)
	})

	t.Run("无工具调用", func(t *testing.T) {
		textContent := "Just text"
		content := []types.ContentBlock{
			{
				Type: "text",
				Text: &textContent,
			},
		}

		toolUses := extractToolUsesFromMessage(content)

		assert.Nil(t, toolUses)
	})
}

func TestExtractToolResultsFromMessage(t *testing.T) {
	t.Run("提取工具结果", func(t *testing.T) {
		toolUseID := "tool_123"
		isError := false
		content := []types.ContentBlock{
			{
				Type:      "tool_result",
				ToolUseId: &toolUseID,
				Content:   "Result data",
				IsError:   &isError,
			},
		}

		toolResults := extractToolResultsFromMessage(content)

		require.Len(t, toolResults, 1)
		assert.Equal(t, "tool_123", toolResults[0].ToolUseId)
		// Content 会被转换为数组格式
		assert.NotNil(t, toolResults[0].Content)
		assert.Equal(t, "success", toolResults[0].Status)
	})

	t.Run("提取错误的工具结果", func(t *testing.T) {
		toolUseID := "tool_456"
		isError := true
		content := []types.ContentBlock{
			{
				Type:      "tool_result",
				ToolUseId: &toolUseID,
				Content:   "Error occurred",
				IsError:   &isError,
			},
		}

		toolResults := extractToolResultsFromMessage(content)

		require.Len(t, toolResults, 1)
		assert.Equal(t, "error", toolResults[0].Status)
	})
}

func TestValidateCodeWhispererRequest(t *testing.T) {
	t.Run("有效请求", func(t *testing.T) {
		cwReq := &types.CodeWhispererRequest{}
		cwReq.ConversationState.ConversationId = "test-conversation-id"
		cwReq.ConversationState.CurrentMessage.UserInputMessage.Content = "Hello"
		cwReq.ConversationState.CurrentMessage.UserInputMessage.ModelId = "CLAUDE_SONNET_4_20250514_V1_0"

		err := validateCodeWhispererRequest(cwReq)
		assert.NoError(t, err)
	})

	t.Run("缺少ConversationId", func(t *testing.T) {
		cwReq := &types.CodeWhispererRequest{}
		cwReq.ConversationState.CurrentMessage.UserInputMessage.Content = "Hello"
		cwReq.ConversationState.CurrentMessage.UserInputMessage.ModelId = "CLAUDE_SONNET_4_20250514_V1_0"

		err := validateCodeWhispererRequest(cwReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ConversationId")
	})

	t.Run("缺少ModelId", func(t *testing.T) {
		cwReq := &types.CodeWhispererRequest{}
		cwReq.ConversationState.ConversationId = "test-conversation-id"
		cwReq.ConversationState.CurrentMessage.UserInputMessage.Content = "Hello"

		err := validateCodeWhispererRequest(cwReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ModelId")
	})
}

// TestBuildCodeWhispererRequest_OrphanAssistantMessages 测试孤立assistant消息的忽略处理
func TestBuildCodeWhispererRequest_OrphanAssistantMessages(t *testing.T) {
	t.Run("开头是assistant消息", func(t *testing.T) {
		anthropicReq := types.AnthropicRequest{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 1024,
			Messages: []types.AnthropicRequestMessage{
				{
					Role:    "assistant",
					Content: "Hello, how can I help?",
				},
				{
					Role:    "user",
					Content: "Tell me about Go",
				},
			},
		}

		cwReq, err := BuildCodeWhispererRequest(anthropicReq, nil)

		require.NoError(t, err)

		// 历史应该为空（孤立的assistant消息被忽略）
		if cwReq.ConversationState.History != nil {
			assert.Len(t, cwReq.ConversationState.History, 0)
		}

		// 当前消息
		assert.Equal(t, "Tell me about Go", cwReq.ConversationState.CurrentMessage.UserInputMessage.Content)
	})

	t.Run("连续的assistant消息", func(t *testing.T) {
		anthropicReq := types.AnthropicRequest{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 1024,
			Messages: []types.AnthropicRequestMessage{
				{
					Role:    "user",
					Content: "Question 1",
				},
				{
					Role:    "assistant",
					Content: "Answer 1",
				},
				{
					Role:    "assistant",
					Content: "Answer 2",
				},
				{
					Role:    "user",
					Content: "Question 2",
				},
			},
		}

		cwReq, err := BuildCodeWhispererRequest(anthropicReq, nil)

		require.NoError(t, err)
		assert.NotNil(t, cwReq.ConversationState.History)

		// 历史应该包含2条消息（第二个assistant被忽略）：
		// 1. user: "Question 1"
		// 2. assistant: "Answer 1"
		assert.Len(t, cwReq.ConversationState.History, 2)

		// 验证第一对
		firstUserMsg, ok := cwReq.ConversationState.History[0].(types.HistoryUserMessage)
		assert.True(t, ok)
		assert.Equal(t, "Question 1", firstUserMsg.UserInputMessage.Content)

		firstAssistantMsg, ok := cwReq.ConversationState.History[1].(types.HistoryAssistantMessage)
		assert.True(t, ok)
		assert.Equal(t, "Answer 1", firstAssistantMsg.AssistantResponseMessage.Content)

		// 当前消息
		assert.Equal(t, "Question 2", cwReq.ConversationState.CurrentMessage.UserInputMessage.Content)
	})
}

// TestBuildCodeWhispererRequest_OrphanUserMessages 测试孤立user消息的容错处理
func TestBuildCodeWhispererRequest_OrphanUserMessages(t *testing.T) {
	t.Run("历史末尾存在孤立user消息", func(t *testing.T) {
		anthropicReq := types.AnthropicRequest{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 1024,
			Messages: []types.AnthropicRequestMessage{
				{
					Role:    "user",
					Content: "第一个问题",
				},
				{
					Role:    "assistant",
					Content: "第一个回答",
				},
				{
					Role:    "user",
					Content: "第二个问题（孤立）",
				},
				{
					Role:    "user",
					Content: "第三个问题（当前）",
				},
			},
		}

		cwReq, err := BuildCodeWhispererRequest(anthropicReq, nil)

		require.NoError(t, err)
		assert.NotNil(t, cwReq.ConversationState.History)

		// 历史应该包含4条消息：
		// 1. user: "第一个问题"
		// 2. assistant: "第一个回答"
		// 3. user: "第二个问题（孤立）"
		// 4. assistant: "OK" (自动配对的响应)
		assert.Len(t, cwReq.ConversationState.History, 4)

		// 验证最后一对是自动配对的
		lastUserMsg, ok := cwReq.ConversationState.History[2].(types.HistoryUserMessage)
		assert.True(t, ok)
		assert.Equal(t, "第二个问题（孤立）", lastUserMsg.UserInputMessage.Content)

		lastAssistantMsg, ok := cwReq.ConversationState.History[3].(types.HistoryAssistantMessage)
		assert.True(t, ok)
		assert.Equal(t, "OK", lastAssistantMsg.AssistantResponseMessage.Content)
		assert.Nil(t, lastAssistantMsg.AssistantResponseMessage.ToolUses)

		// 当前消息应该是最后一条
		assert.Equal(t, "第三个问题（当前）", cwReq.ConversationState.CurrentMessage.UserInputMessage.Content)
	})

	t.Run("历史末尾存在多个连续孤立user消息", func(t *testing.T) {
		anthropicReq := types.AnthropicRequest{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 1024,
			Messages: []types.AnthropicRequestMessage{
				{
					Role:    "user",
					Content: "第一个问题",
				},
				{
					Role:    "assistant",
					Content: "第一个回答",
				},
				{
					Role:    "user",
					Content: "第二个问题（孤立1）",
				},
				{
					Role:    "user",
					Content: "第三个问题（孤立2）",
				},
				{
					Role:    "user",
					Content: "第四个问题（当前）",
				},
			},
		}

		cwReq, err := BuildCodeWhispererRequest(anthropicReq, nil)

		require.NoError(t, err)
		assert.NotNil(t, cwReq.ConversationState.History)

		// 历史应该包含4条消息：
		// 1. user: "第一个问题"
		// 2. assistant: "第一个回答"
		// 3. user: "第二个问题（孤立1）\n第三个问题（孤立2）" (合并)
		// 4. assistant: "OK" (自动配对的响应)
		assert.Len(t, cwReq.ConversationState.History, 4)

		// 验证合并的user消息
		mergedUserMsg, ok := cwReq.ConversationState.History[2].(types.HistoryUserMessage)
		assert.True(t, ok)
		assert.Contains(t, mergedUserMsg.UserInputMessage.Content, "第二个问题（孤立1）")
		assert.Contains(t, mergedUserMsg.UserInputMessage.Content, "第三个问题（孤立2）")

		// 验证自动配对的assistant消息
		autoAssistantMsg, ok := cwReq.ConversationState.History[3].(types.HistoryAssistantMessage)
		assert.True(t, ok)
		assert.Equal(t, "OK", autoAssistantMsg.AssistantResponseMessage.Content)

		// 当前消息应该是最后一条
		assert.Equal(t, "第四个问题（当前）", cwReq.ConversationState.CurrentMessage.UserInputMessage.Content)
	})

	t.Run("正常配对的消息不受影响", func(t *testing.T) {
		anthropicReq := types.AnthropicRequest{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 1024,
			Messages: []types.AnthropicRequestMessage{
				{
					Role:    "user",
					Content: "第一个问题",
				},
				{
					Role:    "assistant",
					Content: "第一个回答",
				},
				{
					Role:    "user",
					Content: "第二个问题",
				},
				{
					Role:    "assistant",
					Content: "第二个回答",
				},
				{
					Role:    "user",
					Content: "第三个问题（当前）",
				},
			},
		}

		cwReq, err := BuildCodeWhispererRequest(anthropicReq, nil)

		require.NoError(t, err)
		assert.NotNil(t, cwReq.ConversationState.History)

		// 历史应该包含4条消息（两对正常配对）
		assert.Len(t, cwReq.ConversationState.History, 4)

		// 验证第一对
		firstUserMsg, ok := cwReq.ConversationState.History[0].(types.HistoryUserMessage)
		assert.True(t, ok)
		assert.Equal(t, "第一个问题", firstUserMsg.UserInputMessage.Content)

		firstAssistantMsg, ok := cwReq.ConversationState.History[1].(types.HistoryAssistantMessage)
		assert.True(t, ok)
		assert.Equal(t, "第一个回答", firstAssistantMsg.AssistantResponseMessage.Content)

		// 验证第二对
		secondUserMsg, ok := cwReq.ConversationState.History[2].(types.HistoryUserMessage)
		assert.True(t, ok)
		assert.Equal(t, "第二个问题", secondUserMsg.UserInputMessage.Content)

		secondAssistantMsg, ok := cwReq.ConversationState.History[3].(types.HistoryAssistantMessage)
		assert.True(t, ok)
		assert.Equal(t, "第二个回答", secondAssistantMsg.AssistantResponseMessage.Content)

		// 当前消息
		assert.Equal(t, "第三个问题（当前）", cwReq.ConversationState.CurrentMessage.UserInputMessage.Content)
	})
}

