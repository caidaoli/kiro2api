package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestConversationIDStability 测试ConversationId在1小时内保持稳定
func TestConversationIDStability(t *testing.T) {
	manager := NewConversationIDManager()

	// 创建模拟的HTTP请求上下文
	createContext := func() *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
		c.Request.Header.Set("User-Agent", "test-client/1.0")
		c.Request.RemoteAddr = "192.168.1.100:12345"
		return c
	}

	// 第一次生成
	ctx1 := createContext()
	convID1 := manager.GenerateConversationID(ctx1)
	require.NotEmpty(t, convID1)

	// 立即再次生成（应该相同）
	ctx2 := createContext()
	convID2 := manager.GenerateConversationID(ctx2)
	assert.Equal(t, convID1, convID2, "同一客户端在短时间内应该生成相同的ConversationId")

	// 模拟多次请求（在同一小时内）
	for i := 0; i < 10; i++ {
		ctx := createContext()
		convID := manager.GenerateConversationID(ctx)
		assert.Equal(t, convID1, convID, "第%d次请求的ConversationId应该保持不变", i+1)
	}
}

// TestAgentContinuationIDStability 测试AgentContinuationId在1小时内保持稳定
func TestAgentContinuationIDStability(t *testing.T) {
	// 创建模拟的HTTP请求上下文
	createContext := func() *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
		c.Request.Header.Set("User-Agent", "test-client/1.0")
		c.Request.RemoteAddr = "192.168.1.100:12345"
		return c
	}

	// 第一次生成
	ctx1 := createContext()
	agentID1 := GenerateStableAgentContinuationID(ctx1)
	require.NotEmpty(t, agentID1)

	// 立即再次生成（应该相同）
	ctx2 := createContext()
	agentID2 := GenerateStableAgentContinuationID(ctx2)
	assert.Equal(t, agentID1, agentID2, "同一客户端在短时间内应该生成相同的AgentContinuationId")

	// 模拟多次请求（在同一小时内）
	for i := 0; i < 10; i++ {
		ctx := createContext()
		agentID := GenerateStableAgentContinuationID(ctx)
		assert.Equal(t, agentID1, agentID, "第%d次请求的AgentContinuationId应该保持不变", i+1)
	}
}

// TestConversationAndAgentIDConsistency 测试ConversationId和AgentContinuationId在同一会话内的一致性
func TestConversationAndAgentIDConsistency(t *testing.T) {
	manager := NewConversationIDManager()

	// 创建模拟的HTTP请求上下文
	createContext := func() *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
		c.Request.Header.Set("User-Agent", "test-client/1.0")
		c.Request.RemoteAddr = "192.168.1.100:12345"
		return c
	}

	// 生成第一组ID
	ctx1 := createContext()
	convID1 := manager.GenerateConversationID(ctx1)
	agentID1 := GenerateStableAgentContinuationID(ctx1)

	// 模拟同一会话内的多次请求
	for i := 0; i < 20; i++ {
		ctx := createContext()
		convID := manager.GenerateConversationID(ctx)
		agentID := GenerateStableAgentContinuationID(ctx)

		assert.Equal(t, convID1, convID, "第%d次请求的ConversationId应该保持不变", i+1)
		assert.Equal(t, agentID1, agentID, "第%d次请求的AgentContinuationId应该保持不变", i+1)
	}

	t.Logf("会话内ID稳定性验证通过:")
	t.Logf("  ConversationId: %s", convID1)
	t.Logf("  AgentContinuationId: %s", agentID1)
}

// TestDifferentClientsGetDifferentIDs 测试不同客户端获得不同的ID
func TestDifferentClientsGetDifferentIDs(t *testing.T) {
	manager := NewConversationIDManager()

	// 客户端1
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
	c1.Request.Header.Set("User-Agent", "client-1")
	c1.Request.RemoteAddr = "192.168.1.100:12345"

	convID1 := manager.GenerateConversationID(c1)
	agentID1 := GenerateStableAgentContinuationID(c1)

	// 客户端2（不同IP）
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
	c2.Request.Header.Set("User-Agent", "client-1")
	c2.Request.RemoteAddr = "192.168.1.101:12345"

	convID2 := manager.GenerateConversationID(c2)
	agentID2 := GenerateStableAgentContinuationID(c2)

	assert.NotEqual(t, convID1, convID2, "不同IP的客户端应该有不同的ConversationId")
	assert.NotEqual(t, agentID1, agentID2, "不同IP的客户端应该有不同的AgentContinuationId")

	// 客户端3（不同User-Agent）
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
	c3.Request.Header.Set("User-Agent", "client-2")
	c3.Request.RemoteAddr = "192.168.1.100:12345"

	convID3 := manager.GenerateConversationID(c3)
	agentID3 := GenerateStableAgentContinuationID(c3)

	assert.NotEqual(t, convID1, convID3, "不同User-Agent的客户端应该有不同的ConversationId")
	assert.NotEqual(t, agentID1, agentID3, "不同User-Agent的客户端应该有不同的AgentContinuationId")
}

// TestCustomHeadersOverride 测试自定义头部可以覆盖生成的ID
func TestCustomHeadersOverride(t *testing.T) {
	manager := NewConversationIDManager()

	// 测试自定义ConversationId
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
	c1.Request.Header.Set("User-Agent", "test-client")
	c1.Request.Header.Set("X-Conversation-ID", "custom-conv-123")
	c1.Request.RemoteAddr = "192.168.1.100:12345"

	convID := manager.GenerateConversationID(c1)
	assert.Equal(t, "custom-conv-123", convID, "应该使用自定义的ConversationId")

	// 测试自定义AgentContinuationId
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
	c2.Request.Header.Set("User-Agent", "test-client")
	c2.Request.Header.Set("X-Agent-Continuation-ID", "custom-agent-456")
	c2.Request.RemoteAddr = "192.168.1.100:12345"

	agentID := GenerateStableAgentContinuationID(c2)
	assert.Equal(t, "custom-agent-456", agentID, "应该使用自定义的AgentContinuationId")
}

// TestIDFormatValidity 测试生成的ID格式是否有效
func TestIDFormatValidity(t *testing.T) {
	manager := NewConversationIDManager()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "test-client")
	c.Request.RemoteAddr = "192.168.1.100:12345"

	// 测试ConversationId格式
	convID := manager.GenerateConversationID(c)
	assert.NotEmpty(t, convID)
	assert.Contains(t, convID, "conv-", "ConversationId应该以'conv-'开头")
	assert.Len(t, convID, 21, "ConversationId应该是21个字符（conv- + 16个十六进制字符）")

	// 测试AgentContinuationId格式（UUID格式）
	agentID := GenerateStableAgentContinuationID(c)
	assert.NotEmpty(t, agentID)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
		agentID, "AgentContinuationId应该是标准UUID格式")
}

// TestExtractClientInfo 测试客户端信息提取
func TestExtractClientInfo(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "test-client/1.0")
	c.Request.Header.Set("X-Conversation-ID", "custom-conv-id")
	c.Request.Header.Set("X-Agent-Continuation-ID", "custom-agent-id")
	c.Request.RemoteAddr = "192.168.1.100:12345"

	info := ExtractClientInfo(c)

	assert.Equal(t, "192.168.1.100", info["client_ip"])
	assert.Equal(t, "test-client/1.0", info["user_agent"])
	assert.Equal(t, "custom-conv-id", info["custom_conv_id"])
	assert.Equal(t, "custom-agent-id", info["custom_agent_cont_id"])
}

// TestTimeWindowBoundary 测试时间窗口边界情况
func TestTimeWindowBoundary(t *testing.T) {
	// 注意：这个测试依赖于系统时间，可能在跨小时边界时失败
	// 但它验证了时间窗口的基本逻辑
	manager := NewConversationIDManager()

	createContext := func() *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
		c.Request.Header.Set("User-Agent", "test-client")
		c.Request.RemoteAddr = "192.168.1.100:12345"
		return c
	}

	// 在同一分钟内生成多个ID
	ids := make([]string, 5)
	for i := 0; i < 5; i++ {
		ctx := createContext()
		ids[i] = manager.GenerateConversationID(ctx)
		time.Sleep(100 * time.Millisecond) // 短暂延迟
	}

	// 验证所有ID相同（因为在同一小时内）
	for i := 1; i < len(ids); i++ {
		assert.Equal(t, ids[0], ids[i], "同一小时内的ID应该相同")
	}
}

// BenchmarkConversationIDGeneration 基准测试：ConversationId生成性能
func BenchmarkConversationIDGeneration(b *testing.B) {
	manager := NewConversationIDManager()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "bench-client")
	c.Request.RemoteAddr = "192.168.1.100:12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.GenerateConversationID(c)
	}
}

// BenchmarkAgentContinuationIDGeneration 基准测试：AgentContinuationId生成性能
func BenchmarkAgentContinuationIDGeneration(b *testing.B) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "bench-client")
	c.Request.RemoteAddr = "192.168.1.100:12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateStableAgentContinuationID(c)
	}
}
