package config

// ModelMap 模型映射表
var ModelMap = map[string]string{
	"claude-sonnet-4-5":          "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-5-20250929": "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-20250514":   "CLAUDE_SONNET_4_20250514_V1_0",
	"claude-3-7-sonnet-20250219": "CLAUDE_3_7_SONNET_20250219_V1_0",
	"claude-3-5-haiku-20241022":  "auto",
	"claude-haiku-4-5-20251001":  "auto",
}

// RefreshTokenURL 刷新token的URL (social方式)
const RefreshTokenURL = "https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken"

// IdcRefreshTokenURL IdC认证方式的刷新token URL
const IdcRefreshTokenURL = "https://oidc.us-east-1.amazonaws.com/token"

// CodeWhispererURL CodeWhisperer API的URL
const CodeWhispererURL = "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse"
