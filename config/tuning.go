package config

import "time"

// Tuning 性能和行为调优参数
// 从硬编码提取为可配置常量，遵循 KISS 原则
const (
	// ========== 解析器配置 ==========

	// ParserMaxErrors 解析器容忍的最大错误次数
	// 用于所有解析器，防止死循环
	ParserMaxErrors = 5

	// ========== Token缓存配置 ==========

	// TokenCacheTTL Token缓存的生存时间
	// 过期后需要重新刷新
	TokenCacheTTL = 5 * time.Minute

	// HTTPClientKeepAlive HTTP客户端Keep-Alive间隔
	HTTPClientKeepAlive = 30 * time.Second

	// HTTPClientTLSHandshakeTimeout HTTP客户端TLS握手超时
	HTTPClientTLSHandshakeTimeout = 15 * time.Second
)
