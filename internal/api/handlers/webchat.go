package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/lingguard/internal/webchat"
)

// WebChatHandler WebChat API 处理器（Gin 版本）
type WebChatHandler struct {
	handler *webchat.HTTPHandler
}

// NewWebChatHandler 创建 WebChat 处理器
func NewWebChatHandler(handler *webchat.HTTPHandler) *WebChatHandler {
	return &WebChatHandler{handler: handler}
}

// RegisterRoutes 注册路由
func (h *WebChatHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 会话列表
	r.GET("/api/webchat/sessions", gin.WrapF(h.handler.HandleSessions))
	r.POST("/api/webchat/sessions", gin.WrapF(h.handler.HandleSessions))

	// 单个会话
	r.GET("/api/webchat/session", gin.WrapF(h.handler.HandleSession))
	r.DELETE("/api/webchat/session", gin.WrapF(h.handler.HandleSession))
}
