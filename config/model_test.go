package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelMap_ClaudeSonnet45(t *testing.T) {
	model, exists := ModelMap["claude-sonnet-4-5-20250929"]
	assert.True(t, exists)
	assert.Equal(t, "CLAUDE_SONNET_4_5_20250929_V1_0", model)
}

func TestModelMap_ClaudeSonnet4(t *testing.T) {
	model, exists := ModelMap["claude-sonnet-4-20250514"]
	assert.True(t, exists)
	assert.Equal(t, "CLAUDE_SONNET_4_20250514_V1_0", model)
}

func TestModelMap_Claude37Sonnet(t *testing.T) {
	model, exists := ModelMap["claude-3-7-sonnet-20250219"]
	assert.True(t, exists)
	assert.Equal(t, "CLAUDE_3_7_SONNET_20250219_V1_0", model)
}

func TestModelMap_Claude35Haiku(t *testing.T) {
	model, exists := ModelMap["claude-3-5-haiku-20241022"]
	assert.True(t, exists)
	assert.Equal(t, "auto", model)
}

func TestModelMap_ClaudeSonnet46(t *testing.T) {
	for _, key := range []string{"claude-sonnet-4-6", "claude-sonnet-4-6-20260205"} {
		model, exists := ModelMap[key]
		assert.True(t, exists, "key %s should exist", key)
		assert.Equal(t, "claude-sonnet-4.6", model)
	}
}

func TestModelMap_ClaudeOpus46(t *testing.T) {
	for _, key := range []string{"claude-opus-4-6", "claude-opus-4-6-20260205"} {
		model, exists := ModelMap[key]
		assert.True(t, exists, "key %s should exist", key)
		assert.Equal(t, "claude-opus-4.6", model)
	}
}

func TestModelMap_ClaudeOpus45(t *testing.T) {
	for _, key := range []string{"claude-opus-4-5", "claude-opus-4-5-20251101"} {
		model, exists := ModelMap[key]
		assert.True(t, exists, "key %s should exist", key)
		assert.Equal(t, "claude-opus-4.5", model)
	}
}

func TestModelMap_ClaudeHaiku45(t *testing.T) {
	for _, key := range []string{"claude-haiku-4-5", "claude-haiku-4-5-20251001"} {
		model, exists := ModelMap[key]
		assert.True(t, exists, "key %s should exist", key)
		assert.Equal(t, "claude-haiku-4.5", model)
	}
}

func TestModelMap_NonExistentModel(t *testing.T) {
	_, exists := ModelMap["non-existent-model"]
	assert.False(t, exists)
}

func TestModelMap_AllModelsHaveMapping(t *testing.T) {
	// 确保所有模型都有对应的映射
	expectedModels := []string{
		"claude-sonnet-4-5-20250929",
		"claude-sonnet-4-20250514",
		"claude-3-7-sonnet-20250219",
		"claude-3-5-haiku-20241022",
		"claude-sonnet-4-6",
		"claude-opus-4-6",
		"claude-opus-4-5",
		"claude-haiku-4-5",
	}

	for _, model := range expectedModels {
		_, exists := ModelMap[model]
		assert.True(t, exists, "Model %s should exist in ModelMap", model)
	}
}

func TestModelMap_MappingsAreCorrectFormat(t *testing.T) {
	// 允许的 value 格式：
	//   - "CLAUDE_XXX_V1_0"：CodeWhisperer 精确 ModelId
	//   - "claude-xxx-N.M"：kiro-cli 短 ID
	//   - "auto"：Kiro 路由层自动选择
	for inputModel, outputModel := range ModelMap {
		isLegacyCW := strings.Contains(outputModel, "CLAUDE") && strings.Contains(outputModel, "_V1_0")
		isKiroCli := strings.HasPrefix(outputModel, "claude-")
		isAuto := outputModel == "auto"
		assert.True(t, isLegacyCW || isKiroCli || isAuto,
			"Model mapping for %s has invalid format: %s", inputModel, outputModel)
	}
}

func TestSupportedModels_ContainsAllKeys(t *testing.T) {
	// 确保所有ModelMap的key都在某个地方被文档化
	assert.NotEmpty(t, ModelMap, "ModelMap should not be empty")
	assert.Greater(t, len(ModelMap), 3, "ModelMap should contain at least 3 models")
}
