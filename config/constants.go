package config

import "time"

// Token管理常量
const (
	// TokenCacheKeyFormat token缓存key格式
	TokenCacheKeyFormat = "token_%d"

	// TokenRefreshCleanupDelay token刷新完成后的清理延迟
	TokenRefreshCleanupDelay = 5 * time.Second
)

// 消息处理常量
const (
	// MessageIDFormat 消息ID格式
	MessageIDFormat = "msg_%s"

	// MessageIDTimeFormat 消息ID时间格式
	MessageIDTimeFormat = "20060102150405"

	// RetryDelay 重试延迟
	RetryDelay = 100 * time.Millisecond
)

// Token估算常量
const (
	// BaseToolsOverhead 基础工具开销（tokens）
	BaseToolsOverhead = 100

	// ShortTextThreshold 短文本阈值（字符数）
	ShortTextThreshold = 100

	// LongTextThreshold 长文本阈值（字符数）
	LongTextThreshold = 1000
)

// EventStream解析器常量
const (
	// EventStreamMinMessageSize AWS EventStream最小消息长度（字节）
	EventStreamMinMessageSize = 16

	// EventStreamMaxMessageSize AWS EventStream最大消息长度（16MB）
	EventStreamMaxMessageSize = 16 * 1024 * 1024
)

// Token计算常量
const (
	// TokenEstimationRatio 字符到token的估算比例
	// 用于工具调用参数的JSON内容token估算
	TokenEstimationRatio = 4
)
