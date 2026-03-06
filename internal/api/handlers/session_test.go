package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingguard/internal/session"
)

func TestSessionHandler_ListSessions(t *testing.T) {
	// 创建 Session Manager
	sessionMgr := session.NewManager(nil, 50)

	// 创建一些测试会话
	sess1 := sessionMgr.GetOrCreate("session-1")
	sess1.AddMessage("user", "Hello")
	sess1.AddMessage("assistant", "Hi there!")

	sess2 := sessionMgr.GetOrCreate("session-2")
	sess2.AddMessage("user", "How are you?")

	// 创建 Session Handler
	handler := NewSessionHandler(sessionMgr)

	// 创建路由
	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	// 创建 HTTP 请求
	req := httptest.NewRequest("GET", "/v1/sessions?limit=10&offset=0", nil)

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	sessions := resp["sessions"].([]interface{})
	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}

	total := resp["total"].(float64)
	if total != 2 {
		t.Errorf("Expected total 2, got %d", int(total))
	}
}

func TestSessionHandler_GetSession(t *testing.T) {
	// 创建 Session Manager
	sessionMgr := session.NewManager(nil, 50)

	// 创建测试会话
	sess := sessionMgr.GetOrCreate("test-session-123")
	sess.AddMessage("user", "Hello")
	sess.AddMessage("assistant", "Hi there!")

	// 创建 Session Handler
	handler := NewSessionHandler(sessionMgr)

	// 创建路由
	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	// 创建 HTTP 请求
	req := httptest.NewRequest("GET", "/v1/sessions/test-session-123", nil)

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp session.SessionDetail
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if resp.ID != "test-session-123" {
		t.Errorf("Expected session ID 'test-session-123', got '%s'", resp.ID)
	}

	if len(resp.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(resp.Messages))
	}
}

func TestSessionHandler_GetSession_NotFound(t *testing.T) {
	// 创建 Session Manager
	sessionMgr := session.NewManager(nil, 50)

	// 创建 Session Handler
	handler := NewSessionHandler(sessionMgr)

	// 创建路由
	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	// 创建 HTTP 请求（不存在的会话）
	req := httptest.NewRequest("GET", "/v1/sessions/non-existent", nil)

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestSessionHandler_DeleteSession(t *testing.T) {
	// 创建 Session Manager
	sessionMgr := session.NewManager(nil, 50)

	// 创建测试会话
	sessionMgr.GetOrCreate("session-to-delete")

	// 创建 Session Handler
	handler := NewSessionHandler(sessionMgr)

	// 创建路由
	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	// 创建 HTTP 请求
	req := httptest.NewRequest("DELETE", "/v1/sessions/session-to-delete", nil)

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// 验证会话已被删除
	if sessionMgr.Count() != 0 {
		t.Errorf("Expected 0 sessions after delete, got %d", sessionMgr.Count())
	}
}

func TestSessionHandler_ClearSession(t *testing.T) {
	// 创建 Session Manager
	sessionMgr := session.NewManager(nil, 50)

	// 创建测试会话
	sess := sessionMgr.GetOrCreate("session-to-clear")
	sess.AddMessage("user", "Hello")
	sess.AddMessage("assistant", "Hi!")

	// 验证初始状态
	if len(sess.GetHistory(100)) != 2 {
		t.Errorf("Expected 2 messages initially, got %d", len(sess.GetHistory(100)))
	}

	// 创建 Session Handler
	handler := NewSessionHandler(sessionMgr)

	// 创建路由
	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	// 创建 HTTP 请求
	req := httptest.NewRequest("POST", "/v1/sessions/session-to-clear/clear", nil)

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// 验证会话历史已清空
	if len(sess.GetHistory(100)) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(sess.GetHistory(100)))
	}

	// 验证会话仍然存在
	if sessionMgr.Count() != 1 {
		t.Errorf("Expected 1 session after clear, got %d", sessionMgr.Count())
	}
}
