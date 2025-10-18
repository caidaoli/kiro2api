package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"kiro2api/config"
	"kiro2api/converter"
	"kiro2api/logger"
	"kiro2api/types"
	"kiro2api/utils"

	"github.com/gin-gonic/gin"
)

// respondErrorWithCode 标准化的错误响应结构
// 统一返回: {"error": {"message": string, "code": string}}
func respondErrorWithCode(c *gin.Context, statusCode int, code string, format string, args ...any) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": fmt.Sprintf(format, args...),
			"code":    code,
		},
	})
}

// respondError 简化封装，依据statusCode映射默认code
func respondError(c *gin.Context, statusCode int, format string, args ...any) {
	var code string
	switch statusCode {
	case http.StatusBadRequest:
		code = "bad_request"
	case http.StatusUnauthorized:
		code = "unauthorized"
	case http.StatusForbidden:
		code = "forbidden"
	case http.StatusNotFound:
		code = "not_found"
	case http.StatusTooManyRequests:
		code = "rate_limited"
	default:
		code = "internal_error"
	}
	respondErrorWithCode(c, statusCode, code, format, args...)
}

// 通用请求处理错误函数
func handleRequestBuildError(c *gin.Context, err error) {
	logger.Error("构建请求失败", addReqFields(c, logger.Err(err))...)
	respondError(c, http.StatusInternalServerError, "构建请求失败: %v", err)
}

func handleRequestSendError(c *gin.Context, err error) {
	logger.Error("发送请求失败", addReqFields(c, logger.Err(err))...)
	respondError(c, http.StatusInternalServerError, "发送请求失败: %v", err)
}

func handleResponseReadError(c *gin.Context, err error) {
	logger.Error("读取响应体失败", addReqFields(c, logger.Err(err))...)
	respondError(c, http.StatusInternalServerError, "读取响应体失败: %v", err)
}

// 通用请求执行函数
func executeCodeWhispererRequest(c *gin.Context, anthropicReq types.AnthropicRequest, tokenInfo types.TokenInfo, isStream bool) (*http.Response, error) {
	req, err := buildCodeWhispererRequest(c, anthropicReq, tokenInfo, isStream)
	if err != nil {
		// 检查是否是模型未找到错误，如果是，则响应已经发送，不需要再次处理
		if _, ok := err.(*types.ModelNotFoundErrorType); ok {
			return nil, err
		}
		handleRequestBuildError(c, err)
		return nil, err
	}

	resp, err := utils.DoRequest(req)
	if err != nil {
		handleRequestSendError(c, err)
		return nil, err
	}

	if handleCodeWhispererError(c, resp) {
		resp.Body.Close()
		return nil, fmt.Errorf("CodeWhisperer API error")
	}

	// 上游响应成功，记录方向与会话
	logger.Debug("上游响应成功",
		addReqFields(c,
			logger.String("direction", "upstream_response"),
			logger.Int("status_code", resp.StatusCode),
		)...)

	return resp, nil
}

// execCWRequest 供测试覆盖的请求执行入口（可在测试中替换）
var execCWRequest = executeCodeWhispererRequest

// buildCodeWhispererRequest 构建通用的CodeWhisperer请求
func buildCodeWhispererRequest(c *gin.Context, anthropicReq types.AnthropicRequest, tokenInfo types.TokenInfo, isStream bool) (*http.Request, error) {
	cwReq, err := converter.BuildCodeWhispererRequest(anthropicReq, c)
	if err != nil {
		// 检查是否是模型未找到错误
		if modelNotFoundErr, ok := err.(*types.ModelNotFoundErrorType); ok {
			// 直接返回用户期望的JSON格式
			c.JSON(http.StatusBadRequest, modelNotFoundErr.ErrorData)
			return nil, err
		}
		return nil, fmt.Errorf("构建CodeWhisperer请求失败: %v", err)
	}

	cwReqBody, err := utils.SafeMarshal(cwReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 临时调试：记录发送给CodeWhisperer的请求内容
	// 补充：当工具直传启用时输出工具名称预览
	var toolNamesPreview string
	if len(cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools) > 0 {
		names := make([]string, 0, len(cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools))
		for _, t := range cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools {
			if t.ToolSpecification.Name != "" {
				names = append(names, t.ToolSpecification.Name)
			}
		}
		toolNamesPreview = strings.Join(names, ",")
	}

	logger.Debug("发送给CodeWhisperer的请求",
		logger.String("direction", "upstream_request"),
		logger.Int("request_size", len(cwReqBody)),
		logger.String("request_body", string(cwReqBody)),
		logger.Int("tools_count", len(cwReq.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.Tools)),
		logger.String("tools_names", toolNamesPreview))

	req, err := http.NewRequest("POST", config.CodeWhispererURL, bytes.NewReader(cwReqBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+tokenInfo.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}

	// 添加上游请求必需的header
	req.Header.Set("x-amzn-kiro-agent-mode", "spec")
	req.Header.Set("x-amz-user-agent", "aws-sdk-js/1.0.18 KiroIDE-0.2.13-66c23a8c5d15afabec89ef9954ef52a119f10d369df04d548fc6c1eac694b0d1")
	req.Header.Set("user-agent", "aws-sdk-js/1.0.18 ua/2.1 os/darwin#25.0.0 lang/js md/nodejs#20.16.0 api/codewhispererstreaming#1.0.18 m/E KiroIDE-0.2.13-66c23a8c5d15afabec89ef9954ef52a119f10d369df04d548fc6c1eac694b0d1")

	return req, nil
}

// handleCodeWhispererError 处理CodeWhisperer API错误响应 (重构后符合SOLID原则)
func handleCodeWhispererError(c *gin.Context, resp *http.Response) bool {
	if resp.StatusCode == http.StatusOK {
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取错误响应失败",
			addReqFields(c,
				logger.String("direction", "upstream_response"),
				logger.Err(err),
			)...)
		respondError(c, http.StatusInternalServerError, "%s", "读取响应失败")
		return true
	}

	logger.Error("上游响应错误",
		addReqFields(c,
			logger.String("direction", "upstream_response"),
			logger.Int("status_code", resp.StatusCode),
			logger.Int("response_len", len(body)),
			logger.String("response_body", string(body)),
		)...)

	// 特殊处理：403错误表示token失效 (保持向后兼容)
	if resp.StatusCode == http.StatusForbidden {
		logger.Warn("收到403错误，token可能已失效")
		respondErrorWithCode(c, http.StatusUnauthorized, "unauthorized", "%s", "Token已失效，请重试")
		return true
	}

	// *** 新增：使用错误映射器处理错误，符合Claude API规范 ***
	errorMapper := NewErrorMapper()
	claudeError := errorMapper.MapCodeWhispererError(resp.StatusCode, body)

	// 根据映射结果发送符合Claude规范的响应
	if claudeError.StopReason == "max_tokens" {
		// CONTENT_LENGTH_EXCEEDS_THRESHOLD -> max_tokens stop_reason
		logger.Info("内容长度超限，映射为max_tokens stop_reason",
			addReqFields(c,
				logger.String("upstream_reason", "CONTENT_LENGTH_EXCEEDS_THRESHOLD"),
				logger.String("claude_stop_reason", "max_tokens"),
			)...)
		errorMapper.SendClaudeError(c, claudeError)
	} else {
		// 其他错误使用传统方式处理 (向后兼容)
		respondErrorWithCode(c, http.StatusInternalServerError, "cw_error", "CodeWhisperer Error: %s", string(body))
	}

	return true
}

// StreamEventSender 统一的流事件发送接口
type StreamEventSender interface {
	SendEvent(c *gin.Context, data any) error
	SendError(c *gin.Context, message string, err error) error
}

// AnthropicStreamSender Anthropic格式的流事件发送器
type AnthropicStreamSender struct{}

func (s *AnthropicStreamSender) SendEvent(c *gin.Context, data any) error {
	var eventType string

	if dataMap, ok := data.(map[string]any); ok {
		if t, exists := dataMap["type"]; exists {
			eventType = t.(string)
		}

	}

	json, err := utils.SafeMarshal(data)
	if err != nil {
		return err
	}

	// 压缩日志：仅记录事件类型与负载长度
	logger.Debug("发送SSE事件",
		addReqFields(c,
			// logger.String("direction", "downstream_send"),
			logger.String("event", eventType),
			// logger.Int("payload_len", len(json)),
			logger.String("payload_preview", string(json)),
		)...)

	fmt.Fprintf(c.Writer, "event: %s\n", eventType)
	fmt.Fprintf(c.Writer, "data: %s\n\n", string(json))
	c.Writer.Flush()
	return nil
}

func (s *AnthropicStreamSender) SendError(c *gin.Context, message string, _ error) error {
	errorResp := map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "overloaded_error",
			"message": message,
		},
	}
	return s.SendEvent(c, errorResp)
}

// OpenAIStreamSender OpenAI格式的流事件发送器
type OpenAIStreamSender struct{}

func (s *OpenAIStreamSender) SendEvent(c *gin.Context, data any) error {

	json, err := utils.SafeMarshal(data)
	if err != nil {
		return err
	}

	// 压缩日志：记录负载长度
	logger.Debug("发送OpenAI SSE事件",
		addReqFields(c,
			logger.String("direction", "downstream_send"),
			logger.Int("payload_len", len(json)),
		)...)

	fmt.Fprintf(c.Writer, "data: %s\n\n", string(json))
	c.Writer.Flush()
	return nil
}

func (s *OpenAIStreamSender) SendError(c *gin.Context, message string, _ error) error {
	errorResp := map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "server_error",
			"code":    "internal_error",
		},
	}

	json, err := utils.FastMarshal(errorResp)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.Writer, "data: %s\n\n", string(json))
	c.Writer.Flush()
	return nil
}

// RequestContext 请求处理上下文，封装通用的请求处理逻辑
type RequestContext struct {
	GinContext  *gin.Context
	AuthService interface {
		GetToken() (types.TokenInfo, error)
		GetTokenWithUsage() (*types.TokenWithUsage, error)
	}
	RequestType string // "anthropic" 或 "openai"
}

// GetTokenAndBody 通用的token获取和请求体读取
// 返回: tokenInfo, requestBody, error
func (rc *RequestContext) GetTokenAndBody() (types.TokenInfo, []byte, error) {
	// 获取token
	tokenInfo, err := rc.AuthService.GetToken()
	if err != nil {
		logger.Error("获取token失败", logger.Err(err))
		respondError(rc.GinContext, http.StatusInternalServerError, "获取token失败: %v", err)
		return types.TokenInfo{}, nil, err
	}

	// 读取请求体
	body, err := rc.GinContext.GetRawData()
	if err != nil {
		logger.Error("读取请求体失败", logger.Err(err))
		respondError(rc.GinContext, http.StatusBadRequest, "读取请求体失败: %v", err)
		return types.TokenInfo{}, nil, err
	}

	// 记录请求日志
	logger.Debug(fmt.Sprintf("收到%s请求", rc.RequestType),
		addReqFields(rc.GinContext,
			logger.String("direction", "client_request"),
			logger.String("body", string(body)),
			logger.Int("body_size", len(body)),
			logger.String("remote_addr", rc.GinContext.ClientIP()),
			logger.String("user_agent", rc.GinContext.GetHeader("User-Agent")),
		)...)

	return tokenInfo, body, nil
}

// GetTokenWithUsageAndBody 获取token（包含使用信息）和请求体
// 返回: tokenWithUsage, requestBody, error
func (rc *RequestContext) GetTokenWithUsageAndBody() (*types.TokenWithUsage, []byte, error) {
	// 获取token（包含使用信息）
	tokenWithUsage, err := rc.AuthService.GetTokenWithUsage()
	if err != nil {
		logger.Error("获取token失败", logger.Err(err))
		respondError(rc.GinContext, http.StatusInternalServerError, "获取token失败: %v", err)
		return nil, nil, err
	}

	// 读取请求体
	body, err := rc.GinContext.GetRawData()
	if err != nil {
		logger.Error("读取请求体失败", logger.Err(err))
		respondError(rc.GinContext, http.StatusBadRequest, "读取请求体失败: %v", err)
		return nil, nil, err
	}

	// 记录请求日志
	logger.Debug(fmt.Sprintf("收到%s请求", rc.RequestType),
		addReqFields(rc.GinContext,
			logger.String("direction", "client_request"),
			logger.String("body", string(body)),
			logger.Int("body_size", len(body)),
			logger.String("remote_addr", rc.GinContext.ClientIP()),
			logger.String("user_agent", rc.GinContext.GetHeader("User-Agent")),
			logger.Float64("available_count", tokenWithUsage.AvailableCount),
		)...)

	return tokenWithUsage, body, nil
}
