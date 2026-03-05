// Package webchat - WebChat HTTP 处理器
package webchat

import (
	"encoding/json"
	"net/http"

	"github.com/lingguard/pkg/logger"
)

// HTTPHandler WebChat HTTP 处理器
type HTTPHandler struct {
	store Store
}

// NewHTTPHandler 创建处理器
func NewHTTPHandler(store Store) *HTTPHandler {
	return &HTTPHandler{store: store}
}

// RegisterRoutes 注册路由
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/webchat/sessions", h.handleSessions)
	mux.HandleFunc("/api/webchat/session", h.handleSession)
	logger.Info("WebChatAPI routes registered")
}

// handleSessions 处理会话列表
func (h *HTTPHandler) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getSessions(w, r)
	case http.MethodPost:
		h.saveSessions(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSession 处理单个会话
func (h *HTTPHandler) handleSession(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getSession(w, r)
	case http.MethodDelete:
		h.deleteSession(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getSessions 获取所有会话
func (h *HTTPHandler) getSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.store.GetAll()
	if err != nil {
		logger.Error("Failed to get sessions", "error", err)
		http.Error(w, "Failed to get sessions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// saveSessions 批量保存会话
func (h *HTTPHandler) saveSessions(w http.ResponseWriter, r *http.Request) {
	var sessions []*Session
	if err := json.NewDecoder(r.Body).Decode(&sessions); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.store.SaveBatch(sessions); err != nil {
		logger.Error("Failed to save sessions", "error", err)
		http.Error(w, "Failed to save sessions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// getSession 获取单个会话
func (h *HTTPHandler) getSession(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing session id", http.StatusBadRequest)
		return
	}

	session, err := h.store.Get(id)
	if err != nil {
		logger.Error("Failed to get session", "error", err, "sessionId", id)
		http.Error(w, "Failed to get session", http.StatusInternalServerError)
		return
	}

	if session == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// deleteSession 删除会话
func (h *HTTPHandler) deleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing session id", http.StatusBadRequest)
		return
	}

	if err := h.store.Delete(id); err != nil {
		logger.Error("Failed to delete session", "error", err, "sessionId", id)
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
