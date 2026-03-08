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
	// WebChat 会话 API（内部 WebUI）
	webchat := r.Group("/webchat")
	{
		// 会话列表
		webchat.GET("/sessions", gin.WrapF(h.handler.HandleSessions))
		webchat.POST("/sessions", gin.WrapF(h.handler.HandleSessions))

		// 单个会话
		webchat.GET("/session", gin.WrapF(h.handler.HandleSession))
		webchat.DELETE("/session", gin.WrapF(h.handler.HandleSession))
	}
}
