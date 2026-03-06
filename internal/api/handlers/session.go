package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingguard/internal/session"
)

// SessionHandler Session API 处理器
type SessionHandler struct {
	sessionMgr *session.Manager
}

// NewSessionHandler 创建 Session 处理器
func NewSessionHandler(sessionMgr *session.Manager) *SessionHandler {
	return &SessionHandler{sessionMgr: sessionMgr}
}

// GetSessionManager 获取会话管理器
func (h *SessionHandler) GetSessionManager() *session.Manager {
	return h.sessionMgr
}

// RegisterRoutes 注册路由
func (h *SessionHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/sessions", h.ListSessions)
	r.GET("/sessions/:session_id", h.GetSession)
	r.DELETE("/sessions/:session_id", h.DeleteSession)
	r.POST("/sessions/:session_id/clear", h.ClearSession)
}

// ListSessions 列出会话
func (h *SessionHandler) ListSessions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	agentID := c.Query("agent_id")

	sessions := h.sessionMgr.List(limit, offset, agentID)
	total := h.sessionMgr.Count()

	c.JSON(200, gin.H{
		"sessions": sessions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// GetSession 获取会话详情
func (h *SessionHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	detail, err := h.sessionMgr.Get(sessionID)
	if err != nil {
		c.JSON(404, ErrorResponse{
			Error: ErrorDetail{
				Code:    "session_not_found",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(200, detail)
}

// DeleteSession 删除会话
func (h *SessionHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	if err := h.sessionMgr.Delete(sessionID); err != nil {
		c.JSON(404, ErrorResponse{
			Error: ErrorDetail{
				Code:    "session_not_found",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "session deleted",
		"id":      sessionID,
	})
}

// ClearSession 清空会话历史
func (h *SessionHandler) ClearSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	if err := h.sessionMgr.ClearHistory(sessionID); err != nil {
		c.JSON(404, ErrorResponse{
			Error: ErrorDetail{
				Code:    "session_not_found",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "session cleared",
		"id":      sessionID,
	})
}
