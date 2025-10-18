package utils

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"time"

	"kiro2api/config"
)

var (
	// SharedHTTPClient 共享的HTTP客户端实例，优化了连接池和性能配置
	SharedHTTPClient *http.Client
)

func init() {
	// 检查TLS配置并记录日志
	skipTLS := shouldSkipTLSVerify()
	if skipTLS {
		os.Stderr.WriteString("[WARNING] TLS证书验证已禁用 - 仅适用于开发/调试环境\n")
	}

	// 创建统一的HTTP客户端
	SharedHTTPClient = &http.Client{
		Transport: &http.Transport{
			// 连接建立配置
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: config.HTTPClientKeepAlive,
				DualStack: true,
			}).DialContext,

			// TLS配置
			TLSHandshakeTimeout: config.HTTPClientTLSHandshakeTimeout,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipTLS,
				MinVersion:         tls.VersionTLS12,
				MaxVersion:         tls.VersionTLS13,
				CipherSuites: []uint16{
					tls.TLS_AES_256_GCM_SHA384,
					tls.TLS_CHACHA20_POLY1305_SHA256,
					tls.TLS_AES_128_GCM_SHA256,
				},
			},

			// HTTP配置
			ForceAttemptHTTP2:  false,
			DisableCompression: false,
		},
	}
}

// shouldSkipTLSVerify 根据GIN_MODE决定是否跳过TLS证书验证
func shouldSkipTLSVerify() bool {
	return os.Getenv("GIN_MODE") == "debug"
}

// DoRequest 执行HTTP请求
func DoRequest(req *http.Request) (*http.Response, error) {
	return SharedHTTPClient.Do(req)
}
