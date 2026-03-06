package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/lingguard/pkg/logger"
)

// WebSocketHandler WebSocket 处理器接口
type WebSocketHandler interface {
	HandleWebSocket(conn *websocket.Conn, sessionID string)
}

// WebSocketHandlerFunc WebSocket 处理器函数
type WebSocketHandlerFunc struct {
	Handler func(conn *websocket.Conn, sessionID string)
}

// HandleWebSocket 实现 WebSocketHandler 接口
func (f WebSocketHandlerFunc) HandleWebSocket(conn *websocket.Conn, sessionID string) {
	f.Handler(conn, sessionID)
}

// websocketUpgrader WebSocket 升级器
var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，CORS 由中间件处理
	},
}

// HandleWebSocket 处理 WebSocket 连接
func HandleWebSocket(handler WebSocketHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从查询参数获取 session ID
		sessionID := c.Query("session")
		if sessionID == "" {
			sessionID = uuid.New().String()
		}

		// 升级为 WebSocket
		conn, err := websocketUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Warn("WebSocket upgrade failed", "error", err)
			return
		}

		logger.Info("WebSocket connected", "sessionId", sessionID)
		handler.HandleWebSocket(conn, sessionID)
	}
}
