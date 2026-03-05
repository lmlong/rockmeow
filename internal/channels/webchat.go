// Package channels - WebChat 渠道实现
// 参考 OpenClaw 的 WebChat channel 设计：
// - 使用 WebSocket 进行双向通信
// - 复用 Gateway/WebUI 的 HTTP 服务器
// - 支持流式响应
package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/stream"
)

// WebChatChannel Web 聊天渠道
type WebChatChannel struct {
	cfg              *config.WebChatConfig
	handler          MessageHandler
	streamingHandler StreamingMessageHandler

	// 连接管理
	connections map[string]*WebChatConnection
	connMu      sync.RWMutex

	// 状态
	running bool
	mu      sync.RWMutex

	// 广播通道
	broadcast chan *BroadcastMessage
}

// WebChatConnection 表示一个 WebSocket 连接
type WebChatConnection struct {
	ID        string
	SessionID string
	UserID    string
	Conn      *websocket.Conn
	Send      chan []byte
	Close     chan struct{}
}

// BroadcastMessage 广播消息
type BroadcastMessage struct {
	SessionID string
	Content   string
}

// WebSocket 消息类型
type WSMessage struct {
	Type    string `json:"type"`    // "chat", "ping"
	Content string `json:"content"` // 消息内容
}

type WSResponse struct {
	Type      string `json:"type"`      // "chat", "stream", "stream_end", "pong", "error"
	Content   string `json:"content"`   // 消息内容
	SessionID string `json:"sessionId"` // 会话 ID
	Done      bool   `json:"done"`      // 是否完成（流式结束时为 true）
}

// NewWebChatChannel 创建 WebChat 渠道
func NewWebChatChannel(cfg *config.WebChatConfig, handler MessageHandler) *WebChatChannel {
	ch := &WebChatChannel{
		cfg:         cfg,
		handler:     handler,
		connections: make(map[string]*WebChatConnection),
		broadcast:   make(chan *BroadcastMessage, 100),
	}

	// 检查是否支持流式
	if streamingHandler, ok := handler.(StreamingMessageHandler); ok {
		ch.streamingHandler = streamingHandler
	}

	return ch
}

// Name 返回渠道名称
func (c *WebChatChannel) Name() string {
	return "webchat"
}

// Start 启动渠道
func (c *WebChatChannel) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	c.running = true

	// 启动广播处理协程
	go c.handleBroadcast(ctx)

	logger.Info("WebChat channel started")
	return nil
}

// Stop 停止渠道
func (c *WebChatChannel) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	c.running = false

	// 关闭所有连接
	c.connMu.Lock()
	for _, conn := range c.connections {
		close(conn.Close)
		conn.Conn.Close()
	}
	c.connections = make(map[string]*WebChatConnection)
	c.connMu.Unlock()

	// 关闭广播通道
	close(c.broadcast)

	logger.Info("WebChat channel stopped")
	return nil
}

// IsRunning 返回是否正在运行
func (c *WebChatChannel) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// Send 发送消息到指定会话（实现 SendableChannel 接口）
func (c *WebChatChannel) Send(ctx context.Context, sessionID string, content string) error {
	c.connMu.RLock()
	conn, ok := c.connections[sessionID]
	c.connMu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	resp := WSResponse{
		Type:      "chat",
		Content:   content,
		SessionID: sessionID,
		Done:      true,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	select {
	case conn.Send <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// HandleWebSocket 处理 WebSocket 连接
func (c *WebChatChannel) HandleWebSocket(conn *websocket.Conn, sessionID string) {
	// 创建连接对象
	wc := &WebChatConnection{
		ID:        sessionID,
		SessionID: sessionID,
		UserID:    sessionID, // WebChat 中 UserID 等于 SessionID
		Conn:      conn,
		Send:      make(chan []byte, 100),
		Close:     make(chan struct{}),
	}

	// 注册连接
	c.connMu.Lock()
	c.connections[sessionID] = wc
	c.connMu.Unlock()

	logger.Info("WebChat connection established", "sessionId", sessionID)

	// 启动写协程
	go c.writePump(wc)

	// 发送欢迎消息
	welcome := WSResponse{
		Type:      "chat",
		Content:   "👋 你好！我是 LingGuard，很高兴为你服务。",
		SessionID: sessionID,
		Done:      true,
	}
	if data, err := json.Marshal(welcome); err == nil {
		wc.Send <- data
	}

	// 读取消息循环
	c.readPump(wc)
}

// readPump 读取 WebSocket 消息
func (c *WebChatChannel) readPump(conn *WebChatConnection) {
	defer func() {
		// 清理连接
		c.connMu.Lock()
		delete(c.connections, conn.SessionID)
		c.connMu.Unlock()
		close(conn.Close)
		conn.Conn.Close()
		logger.Info("WebChat connection closed", "sessionId", conn.SessionID)
	}()

	// 设置读取配置
	conn.Conn.SetReadLimit(512 * 1024) // 512KB
	conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.Conn.SetPongHandler(func(string) error {
		conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warn("WebSocket read error", "error", err, "sessionId", conn.SessionID)
			}
			break
		}

		// 重置读取超时
		conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// 解析消息
		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Warn("Failed to parse WebSocket message", "error", err, "sessionId", conn.SessionID)
			continue
		}

		// 处理消息
		switch msg.Type {
		case "ping":
			// 心跳响应
			resp := WSResponse{Type: "pong"}
			if data, err := json.Marshal(resp); err == nil {
				conn.Send <- data
			}
		case "switch":
			// 切换会话
			c.switchSession(conn, msg.Content)
		case "chat":
			// 处理聊天消息
			c.handleChatMessage(conn, msg.Content)
		default:
			logger.Warn("Unknown message type", "type", msg.Type, "sessionId", conn.SessionID)
		}
	}
}

// writePump 写入 WebSocket 消息
func (c *WebChatChannel) writePump(conn *WebChatConnection) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Conn.Close()
	}()

	for {
		select {
		case <-conn.Close:
			return
		case message, ok := <-conn.Send:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logger.Warn("WebSocket write error", "error", err, "sessionId", conn.SessionID)
				return
			}

			// 批量发送队列中的消息
			messages := make([][]byte, 0)
			for {
				select {
				case msg := <-conn.Send:
					messages = append(messages, msg)
					if len(messages) >= 10 {
						goto sendBatch
					}
				default:
					goto sendBatch
				}
			}
		sendBatch:
			for _, msg := range messages {
				if err := conn.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					logger.Warn("WebSocket write error", "error", err, "sessionId", conn.SessionID)
					return
				}
			}

		case <-ticker.C:
			// 发送心跳
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// switchSession 切换会话
func (c *WebChatChannel) switchSession(conn *WebChatConnection, newSessionID string) {
	if newSessionID == "" {
		return
	}

	oldSessionID := conn.SessionID

	// 更新连接映射
	c.connMu.Lock()
	delete(c.connections, oldSessionID)
	conn.SessionID = newSessionID
	conn.UserID = newSessionID
	c.connections[newSessionID] = conn
	c.connMu.Unlock()

	logger.Info("WebChat session switched", "from", oldSessionID, "to", newSessionID)

	// 发送确认
	resp := WSResponse{
		Type:      "switched",
		SessionID: newSessionID,
		Content:   "会话已切换",
		Done:      true,
	}
	if data, err := json.Marshal(resp); err == nil {
		conn.Send <- data
	}
}

// handleChatMessage 处理聊天消息
func (c *WebChatChannel) handleChatMessage(conn *WebChatConnection, content string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 构造消息
	msg := &Message{
		ID:        fmt.Sprintf("wc-%d", time.Now().UnixNano()),
		SessionID: conn.SessionID,
		Content:   content,
		Channel:   "webchat",
		UserID:    conn.UserID,
		Metadata: map[string]any{
			"source": "websocket",
		},
	}

	// 优先使用流式处理
	if c.streamingHandler != nil {
		c.handleStreamingResponse(ctx, conn, msg)
		return
	}

	// 回退到非流式处理
	response, err := c.handler.HandleMessage(ctx, msg)
	if err != nil {
		logger.Error("Failed to handle message", "error", err, "sessionId", conn.SessionID)
		resp := WSResponse{
			Type:      "error",
			Content:   fmt.Sprintf("处理消息失败: %s", err.Error()),
			SessionID: conn.SessionID,
		}
		if data, err := json.Marshal(resp); err == nil {
			conn.Send <- data
		}
		return
	}

	// 发送响应
	resp := WSResponse{
		Type:      "chat",
		Content:   response,
		SessionID: conn.SessionID,
		Done:      true,
	}
	if data, err := json.Marshal(resp); err == nil {
		conn.Send <- data
	}
}

// handleStreamingResponse 处理流式响应
func (c *WebChatChannel) handleStreamingResponse(ctx context.Context, conn *WebChatConnection, msg *Message) {
	var fullResponse strings.Builder

	callback := func(event stream.StreamEvent) {
		switch event.Type {
		case stream.EventText:
			// 流式发送文本块
			fullResponse.WriteString(event.Content)
			resp := WSResponse{
				Type:      "stream",
				Content:   event.Content,
				SessionID: conn.SessionID,
				Done:      false,
			}
			if data, err := json.Marshal(resp); err == nil {
				select {
				case conn.Send <- data:
				default:
					logger.Warn("Send buffer full, dropping stream chunk", "sessionId", conn.SessionID)
				}
			}

		case stream.EventDone:
			// 发送完成标记
			resp := WSResponse{
				Type:      "stream_end",
				Content:   fullResponse.String(),
				SessionID: conn.SessionID,
				Done:      true,
			}
			if data, err := json.Marshal(resp); err == nil {
				conn.Send <- data
			}

		case stream.EventError:
			// 发送错误
			errMsg := ""
			if event.Error != nil {
				errMsg = event.Error.Error()
			}
			resp := WSResponse{
				Type:      "error",
				Content:   errMsg,
				SessionID: conn.SessionID,
				Done:      true,
			}
			if data, err := json.Marshal(resp); err == nil {
				conn.Send <- data
			}
		}
	}

	err := c.streamingHandler.HandleMessageStream(ctx, msg, callback)
	if err != nil {
		logger.Error("Failed to handle streaming message", "error", err, "sessionId", conn.SessionID)
		resp := WSResponse{
			Type:      "error",
			Content:   fmt.Sprintf("处理消息失败: %s", err.Error()),
			SessionID: conn.SessionID,
			Done:      true,
		}
		if data, err := json.Marshal(resp); err == nil {
			conn.Send <- data
		}
	}
}

// handleBroadcast 处理广播消息
func (c *WebChatChannel) handleBroadcast(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.broadcast:
			if !ok {
				return
			}

			c.connMu.RLock()
			conn, ok := c.connections[msg.SessionID]
			c.connMu.RUnlock()

			if ok {
				resp := WSResponse{
					Type:      "chat",
					Content:   msg.Content,
					SessionID: msg.SessionID,
					Done:      true,
				}
				if data, err := json.Marshal(resp); err == nil {
					select {
					case conn.Send <- data:
					default:
						logger.Warn("Send buffer full, dropping broadcast", "sessionId", msg.SessionID)
					}
				}
			}
		}
	}
}

// Broadcast 广播消息到指定会话
func (c *WebChatChannel) Broadcast(sessionID string, content string) {
	select {
	case c.broadcast <- &BroadcastMessage{SessionID: sessionID, Content: content}:
	default:
		logger.Warn("Broadcast buffer full, dropping message", "sessionId", sessionID)
	}
}

// GetConnectionCount 获取当前连接数
func (c *WebChatChannel) GetConnectionCount() int {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return len(c.connections)
}

// 确保 WebChatChannel 实现 Channel 和 SendableChannel 接口
var _ Channel = (*WebChatChannel)(nil)
var _ SendableChannel = (*WebChatChannel)(nil)
