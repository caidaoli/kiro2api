package config

import (
	"os"
	"strconv"
)

// ModelMap 模型映射表
//
// Value 约定：
//   - "CLAUDE_XXX_V1_0" 格式：CodeWhisperer 后端历史精确 ModelId
//   - "claude-xxx-N.M" 格式：kiro-cli 层短 ID，CodeWhisperer 后端同样接受
//   - "auto":        让 Kiro 路由层自动选择（用于未知/新模型兜底）
var ModelMap = map[string]string{
	// Sonnet
	"claude-sonnet-4-5":          "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-5-20250929": "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-20250514":   "CLAUDE_SONNET_4_20250514_V1_0",
	"claude-3-7-sonnet-20250219": "CLAUDE_3_7_SONNET_20250219_V1_0",
	// Sonnet 4.6（kiro-cli model_id=claude-sonnet-4.6，1M 上下文）
	"claude-sonnet-4-6":          "claude-sonnet-4.6",
	"claude-sonnet-4-6-20260205": "claude-sonnet-4.6",

	// Opus
	// Opus 4.5（kiro-cli model_id=claude-opus-4.5，200K 上下文）
	"claude-opus-4-5":          "claude-opus-4.5",
	"claude-opus-4-5-20251101": "claude-opus-4.5",
	// Opus 4.6（kiro-cli model_id=claude-opus-4.6，1M 上下文）
	"claude-opus-4-6":          "claude-opus-4.6",
	"claude-opus-4-6-20260205": "claude-opus-4.6",

	// Haiku
	"claude-3-5-haiku-20241022": "auto",
	// Haiku 4.5（kiro-cli model_id=claude-haiku-4.5，200K 上下文）
	"claude-haiku-4-5":          "claude-haiku-4.5",
	"claude-haiku-4-5-20251001": "claude-haiku-4.5",
}

// RefreshTokenURL 刷新token的URL (social方式)
const RefreshTokenURL = "https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken"

// IdcRefreshTokenURL IdC认证方式的刷新token URL
const IdcRefreshTokenURL = "https://oidc.us-east-1.amazonaws.com/token"

// CodeWhispererURL CodeWhisperer API的URL
const CodeWhispererURL = "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse"

// MaxToolDescriptionLength 工具描述的最大长度（字符数）
// 可通过环境变量 MAX_TOOL_DESCRIPTION_LENGTH 配置，默认 10000
var MaxToolDescriptionLength = getEnvIntWithDefault("MAX_TOOL_DESCRIPTION_LENGTH", 10000)

// getEnvIntWithDefault 获取整数类型环境变量（带默认值）
func getEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
