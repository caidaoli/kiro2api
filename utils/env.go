package utils

import (
	"os"
	"strconv"
	"strings"
)

// IsDebugMode 检查是否启用调试模式
// 综合检查DEBUG、LOG_LEVEL、GIN_MODE等环境变量
func IsDebugMode() bool {
	// 检查DEBUG环境变量
	if debug := os.Getenv("DEBUG"); debug == "true" || debug == "1" {
		return true
	}

	// 检查LOG_LEVEL是否为debug
	if logLevel := os.Getenv("LOG_LEVEL"); strings.ToLower(logLevel) == "debug" {
		return true
	}

	// 检查GIN_MODE是否为debug
	if ginMode := os.Getenv("GIN_MODE"); ginMode == "debug" {
		return true
	}

	return false
}

// GetEnvWithDefault 获取环境变量，如果不存在则返回默认值
func GetEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetEnvBool 获取布尔类型环境变量
// 接受的true值：true, 1, yes, on（不区分大小写）
func GetEnvBool(key string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// GetEnvBoolWithDefault 获取布尔类型环境变量（带默认值）
func GetEnvBoolWithDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return GetEnvBool(key)
	}
	return defaultValue
}

// GetEnvIntWithDefault 获取整数类型环境变量（带默认值）
func GetEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
