package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingguard/internal/session"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestChatHandler_HandleChat_NonStream(t *testing.T) {
	// 创建 Session Manager
	sessionMgr := session.NewManager(nil, 50)

	// 创建 Chat Handler（没有 Agent）
	handler := NewChatHandler(nil, sessionMgr)

	// 创建路由
	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	// 创建请求
	reqBody := ChatRequest{
		Message: "Hello, world!",
	}
	body, _ := json.Marshal(reqBody)

	// 创建 HTTP 请求
	req := httptest.NewRequest("POST", "/v1/agents/default/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应（由于没有 Agent，应该返回 503 错误）
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d: %s", w.Code, w.Body.String())
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if resp.Error.Code != "service_unavailable" {
		t.Errorf("Expected error code 'service_unavailable', got '%s'", resp.Error.Code)
	}
}

func TestChatHandler_HandleChat_InvalidRequest(t *testing.T) {
	// 创建 Session Manager
	sessionMgr := session.NewManager(nil, 50)

	// 创建 Chat Handler
	handler := NewChatHandler(nil, sessionMgr)

	// 创建路由
	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	// 创建空请求（缺少 message 字段）
	req := httptest.NewRequest("POST", "/v1/agents/default/chat", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应（应该返回 400 错误）
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if resp.Error.Code != "invalid_request" {
		t.Errorf("Expected error code 'invalid_request', got '%s'", resp.Error.Code)
	}
}

func TestChatHandler_HandleChat_WithSessionID(t *testing.T) {
	// 创建 Session Manager
	sessionMgr := session.NewManager(nil, 50)

	// 创建 Chat Handler
	handler := NewChatHandler(nil, sessionMgr)

	// 创建路由
	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	// 创建请求（带 session_id）
	reqBody := ChatRequest{
		Message:   "Hello, world!",
		SessionID: "test-session-123",
	}
	body, _ := json.Marshal(reqBody)

	// 创建 HTTP 请求
	req := httptest.NewRequest("POST", "/v1/agents/default/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证会话已创建
	sess := sessionMgr.GetOrCreate("test-session-123")
	if sess == nil {
		t.Error("Session should be created")
	}
}
