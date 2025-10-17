package converter

import (
	"testing"

	"kiro2api/types"

	"github.com/stretchr/testify/assert"
)

func TestValidateAndProcessTools_EmptyTools(t *testing.T) {
	tools := []types.OpenAITool{}

	result, err := validateAndProcessTools(tools)

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestValidateAndProcessTools_ValidTool(t *testing.T) {
	tools := []types.OpenAITool{
		{
			Type: "function",
			Function: types.OpenAIFunction{
				Name:        "get_weather",
				Description: "Get weather information",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{
							"type":        "string",
							"description": "City name",
						},
					},
					"required": []any{"location"},
				},
			},
		},
	}

	result, err := validateAndProcessTools(tools)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "get_weather", result[0].Name)
	assert.Equal(t, "Get weather information", result[0].Description)
	assert.NotNil(t, result[0].InputSchema)
}

func TestValidateAndProcessTools_InvalidType(t *testing.T) {
	tools := []types.OpenAITool{
		{
			Type: "invalid_type",
			Function: types.OpenAIFunction{
				Name: "test",
			},
		},
	}

	result, err := validateAndProcessTools(tools)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的工具类型")
	assert.Empty(t, result)
}

func TestValidateAndProcessTools_EmptyFunctionName(t *testing.T) {
	tools := []types.OpenAITool{
		{
			Type: "function",
			Function: types.OpenAIFunction{
				Name: "",
				Parameters: map[string]any{
					"type": "object",
				},
			},
		},
	}

	result, err := validateAndProcessTools(tools)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "函数名称不能为空")
	assert.Empty(t, result)
}

func TestValidateAndProcessTools_NilParameters(t *testing.T) {
	tools := []types.OpenAITool{
		{
			Type: "function",
			Function: types.OpenAIFunction{
				Name:       "test_func",
				Parameters: nil,
			},
		},
	}

	result, err := validateAndProcessTools(tools)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "参数schema不能为空")
	assert.Empty(t, result)
}

func TestValidateAndProcessTools_MultipleTools(t *testing.T) {
	tools := []types.OpenAITool{
		{
			Type: "function",
			Function: types.OpenAIFunction{
				Name:        "tool1",
				Description: "First tool",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"param1": map[string]any{"type": "string"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: types.OpenAIFunction{
				Name:        "tool2",
				Description: "Second tool",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"param2": map[string]any{"type": "number"},
					},
				},
			},
		},
	}

	result, err := validateAndProcessTools(tools)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "tool1", result[0].Name)
	assert.Equal(t, "tool2", result[1].Name)
}

func TestValidateAndProcessTools_MixedValidInvalid(t *testing.T) {
	tools := []types.OpenAITool{
		{
			Type: "function",
			Function: types.OpenAIFunction{
				Name: "valid_tool",
				Parameters: map[string]any{
					"type": "object",
				},
			},
		},
		{
			Type: "invalid_type",
			Function: types.OpenAIFunction{
				Name: "invalid_tool",
			},
		},
	}

	result, err := validateAndProcessTools(tools)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的工具类型")
	// 当有错误时，可能返回空或部分工具，取决于实现
	if len(result) > 0 {
		assert.Equal(t, "valid_tool", result[0].Name)
	}
}

func TestValidateAndProcessTools_FilterWebSearch(t *testing.T) {
	tools := []types.OpenAITool{
		{
			Type: "function",
			Function: types.OpenAIFunction{
				Name:        "get_weather",
				Description: "Get weather information",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: types.OpenAIFunction{
				Name:        "web_search",
				Description: "Search the web",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: types.OpenAIFunction{
				Name:        "calculator",
				Description: "Perform calculations",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"expression": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	result, err := validateAndProcessTools(tools)

	// web_search should be filtered out silently, no error
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "get_weather", result[0].Name)
	assert.Equal(t, "calculator", result[1].Name)

	// Ensure web_search is not in the result
	for _, tool := range result {
		assert.NotEqual(t, "web_search", tool.Name)
		assert.NotEqual(t, "websearch", tool.Name)
	}
}

func TestValidateAndProcessTools_FilterWebSearchVariant(t *testing.T) {
	tools := []types.OpenAITool{
		{
			Type: "function",
			Function: types.OpenAIFunction{
				Name:        "websearch",
				Description: "Search the web (variant name)",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: types.OpenAIFunction{
				Name:        "valid_tool",
				Description: "A valid tool",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"param": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	result, err := validateAndProcessTools(tools)

	// websearch variant should also be filtered
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "valid_tool", result[0].Name)
}

func TestConvertOpenAIToolChoiceToAnthropic_StringAuto(t *testing.T) {
	result := convertOpenAIToolChoiceToAnthropic("auto")

	assert.NotNil(t, result)
	toolChoice, ok := result.(*types.ToolChoice)
	assert.True(t, ok)
	assert.Equal(t, "auto", toolChoice.Type)
}

func TestConvertOpenAIToolChoiceToAnthropic_StringRequired(t *testing.T) {
	result := convertOpenAIToolChoiceToAnthropic("required")

	assert.NotNil(t, result)
	toolChoice, ok := result.(*types.ToolChoice)
	assert.True(t, ok)
	assert.Equal(t, "any", toolChoice.Type)
}

func TestConvertOpenAIToolChoiceToAnthropic_StringAny(t *testing.T) {
	result := convertOpenAIToolChoiceToAnthropic("any")

	assert.NotNil(t, result)
	toolChoice, ok := result.(*types.ToolChoice)
	assert.True(t, ok)
	assert.Equal(t, "any", toolChoice.Type)
}

func TestConvertOpenAIToolChoiceToAnthropic_StringNone(t *testing.T) {
	result := convertOpenAIToolChoiceToAnthropic("none")

	assert.Nil(t, result, "none应该返回nil")
}

func TestConvertOpenAIToolChoiceToAnthropic_StringUnknown(t *testing.T) {
	result := convertOpenAIToolChoiceToAnthropic("unknown")

	assert.NotNil(t, result)
	toolChoice, ok := result.(*types.ToolChoice)
	assert.True(t, ok)
	assert.Equal(t, "auto", toolChoice.Type, "未知字符串应该默认为auto")
}

func TestConvertOpenAIToolChoiceToAnthropic_Nil(t *testing.T) {
	result := convertOpenAIToolChoiceToAnthropic(nil)

	assert.Nil(t, result)
}

func TestConvertOpenAIToolChoiceToAnthropic_MapWithFunction(t *testing.T) {
	choice := map[string]any{
		"type": "function",
		"function": map[string]any{
			"name": "my_tool",
		},
	}

	result := convertOpenAIToolChoiceToAnthropic(choice)

	assert.NotNil(t, result)
	toolChoice, ok := result.(*types.ToolChoice)
	assert.True(t, ok)
	assert.Equal(t, "tool", toolChoice.Type)
	assert.Equal(t, "my_tool", toolChoice.Name)
}

func TestConvertOpenAIToolChoiceToAnthropic_MapInvalid(t *testing.T) {
	choice := map[string]any{
		"type": "invalid",
	}

	result := convertOpenAIToolChoiceToAnthropic(choice)

	assert.NotNil(t, result)
	toolChoice, ok := result.(*types.ToolChoice)
	assert.True(t, ok)
	assert.Equal(t, "auto", toolChoice.Type, "无效map应该返回auto")
}

func TestConvertOpenAIToolChoiceToAnthropic_StructType(t *testing.T) {
	choice := types.OpenAIToolChoice{
		Type: "function",
		Function: &types.OpenAIToolChoiceFunction{
			Name: "struct_tool",
		},
	}

	result := convertOpenAIToolChoiceToAnthropic(choice)

	assert.NotNil(t, result)
	toolChoice, ok := result.(*types.ToolChoice)
	assert.True(t, ok)
	assert.Equal(t, "tool", toolChoice.Type)
	assert.Equal(t, "struct_tool", toolChoice.Name)
}

func TestConvertOpenAIToolChoiceToAnthropic_StructTypeAuto(t *testing.T) {
	choice := types.OpenAIToolChoice{
		Type: "auto",
	}

	result := convertOpenAIToolChoiceToAnthropic(choice)

	assert.NotNil(t, result)
	toolChoice, ok := result.(*types.ToolChoice)
	assert.True(t, ok)
	assert.Equal(t, "auto", toolChoice.Type)
}

func TestConvertOpenAIToolChoiceToAnthropic_UnknownType(t *testing.T) {
	result := convertOpenAIToolChoiceToAnthropic(12345)

	assert.NotNil(t, result)
	toolChoice, ok := result.(*types.ToolChoice)
	assert.True(t, ok)
	assert.Equal(t, "auto", toolChoice.Type, "未知类型应该返回auto")
}

func TestConvertOpenAIContentToAnthropic_String(t *testing.T) {
	content := "Hello, world!"

	result, err := convertOpenAIContentToAnthropic(content)

	assert.NoError(t, err)
	assert.Equal(t, "Hello, world!", result)
}

func TestConvertOpenAIContentToAnthropic_ArrayOfBlocks(t *testing.T) {
	content := []any{
		map[string]any{
			"type": "text",
			"text": "Hello",
		},
	}

	result, err := convertOpenAIContentToAnthropic(content)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestConvertOpenAIContentToAnthropic_Default(t *testing.T) {
	content := 12345

	result, err := convertOpenAIContentToAnthropic(content)

	assert.NoError(t, err)
	assert.Equal(t, 12345, result)
}

func TestConvertOpenAIContentToAnthropic_FilterWebSearchInHistory(t *testing.T) {
	content := []any{
		map[string]any{
			"type": "text",
			"text": "Let me search for that information.",
		},
		map[string]any{
			"type": "tool_use",
			"id":   "call_123",
			"name": "web_search",
			"input": map[string]any{
				"query": "test query",
			},
		},
		map[string]any{
			"type": "tool_use",
			"id":   "call_456",
			"name": "calculator",
			"input": map[string]any{
				"expression": "2+2",
			},
		},
	}

	result, err := convertOpenAIContentToAnthropic(content)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 转换后的内容应该过滤掉web_search，只保留text和calculator
	resultArray, ok := result.([]any)
	assert.True(t, ok)
	assert.Len(t, resultArray, 2, "应该过滤掉web_search，只保留2个块")

	// 验证第一个是text块
	block1, ok := resultArray[0].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "text", block1["type"])

	// 验证第二个是calculator的tool_use块
	block2, ok := resultArray[1].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "tool_use", block2["type"])
	assert.Equal(t, "calculator", block2["name"])
}

func TestConvertOpenAIContentToAnthropic_FilterWebSearchVariantInHistory(t *testing.T) {
	content := []any{
		map[string]any{
			"type": "tool_use",
			"id":   "call_789",
			"name": "websearch",
			"input": map[string]any{
				"query": "another query",
			},
		},
		map[string]any{
			"type": "text",
			"text": "Some text",
		},
	}

	result, err := convertOpenAIContentToAnthropic(content)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 转换后的内容应该过滤掉websearch变体，只保留text
	resultArray, ok := result.([]any)
	assert.True(t, ok)
	assert.Len(t, resultArray, 1, "应该过滤掉websearch，只保留1个块")

	// 验证是text块
	block, ok := resultArray[0].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "text", block["type"])
}
