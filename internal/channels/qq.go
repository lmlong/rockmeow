package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/stream"
)

// QQ Bot API endpoints
const (
	qqAPIBase    = "https://api.sgroup.qq.com"
	qqGatewayURL = "wss://api.sgroup.qq.com/websocket"
)

// QQ opcode constants
const (
	qqOpDispatch            = 0
	qqOpHeartbeat           = 1
	qqOpIdentify            = 2
	qqOpResume              = 6
	qqOpReconnect           = 7
	qqOpInvalidSession      = 9
	qqOpHello               = 10
	qqOpHeartbeatAck        = 11
	qqOpHTTPCallbackAck     = 12
	qqOpPlatformCallbackAck = 13
)

// QQ event types
const (
	qqEventReady            = "READY"
	qqEventC2CMessageCreate = "C2C_MESSAGE_CREATE"
	qqEventDirectMessage    = "DIRECT_MESSAGE_CREATE"
)

// QQ payload structures
type qqPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  int             `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

type qqHelloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type qqIdentifyData struct {
	Token      qqToken `json:"token"`
	Intents    int     `json:"intents"`
	Shard      []int   `json:"shard,omitempty"`
	Properties qqProps `json:"properties,omitempty"`
}

type qqToken struct {
	AppID string `json:"appId"`
	Token string `json:"token"` // 实际上是 secret
}

type qqProps struct {
	OS      string `json:"$os"`
	Browser string `json:"$browser"`
	Device  string `json:"$device"`
}

type qqReadyData struct {
	Version   int    `json:"version"`
	SessionID string `json:"session_id"`
	User      qqUser `json:"user"`
	Shard     []int  `json:"shard"`
}

type qqUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

type qqC2CMessage struct {
	ID        string   `json:"id"`
	Content   string   `json:"content"`
	Timestamp string   `json:"timestamp"`
	Author    qqAuthor `json:"author"`
}

type qqAuthor struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// QQ intent flags
const (
	qqIntentGuilds                = 1 << 0
	qqIntentGuildMembers          = 1 << 1
	qqIntentGuildMessages         = 1 << 9
	qqIntentGuildMessageReactions = 1 << 10
	qqIntentDirectMessage         = 1 << 12
	qqIntentOpenForumEvent        = 1 << 28
	qqIntentAudioAction           = 1 << 29
	qqIntentPublicMessages        = 1 << 30
)

// QQChannel QQ机器人渠道 (使用 WebSocket Gateway)
type QQChannel struct {
	cfg              *config.QQConfig
	handler          MessageHandler
	streamingHandler StreamingMessageHandler
	allowMap         map[string]bool

	// WebSocket connection
	conn      *websocket.Conn
	connMu    sync.Mutex
	running   bool
	sessionID string
	sequence  int

	// Heartbeat
	heartbeatInterval time.Duration
	heartbeatTicker   *time.Ticker
	lastHeartbeatAck  time.Time

	// Message deduplication
	processedMsgs sync.Map
	dedupeMu      sync.Mutex

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// HTTP client for API calls
	httpClient *http.Client
}

// NewQQChannel 创建QQ渠道
func NewQQChannel(cfg *config.QQConfig, handler MessageHandler) *QQChannel {
	allowMap := make(map[string]bool)
	for _, id := range cfg.AllowFrom {
		allowMap[id] = true
	}
	qc := &QQChannel{
		cfg:        cfg,
		handler:    handler,
		allowMap:   allowMap,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	// 检查是否实现了流式处理器接口
	if sh, ok := handler.(StreamingMessageHandler); ok {
		qc.streamingHandler = sh
	}
	return qc
}

// Name 返回渠道名称
func (q *QQChannel) Name() string { return "qq" }

// Start 启动渠道
func (q *QQChannel) Start(ctx context.Context) error {
	if q.running {
		return nil
	}

	// Create context for graceful shutdown
	q.ctx, q.cancel = context.WithCancel(ctx)
	q.running = true

	// Start connection loop in goroutine
	go q.connectionLoop()

	logger.Info("QQ channel started (C2C private message)")
	logger.Info("Using WebSocket Gateway - no public IP required")
	return nil
}

// Stop 停止渠道
func (q *QQChannel) Stop() error {
	if q.cancel != nil {
		q.cancel()
	}
	q.running = false
	q.stopHeartbeat()
	q.closeConnection()
	logger.Info("QQ channel stopped")
	return nil
}

// IsRunning 检查是否运行中
func (q *QQChannel) IsRunning() bool {
	return q.running
}

// Send 主动发送消息
func (q *QQChannel) Send(ctx context.Context, to string, content string) error {
	return q.sendC2CMessage(ctx, to, content)
}

// connectionLoop WebSocket连接循环
func (q *QQChannel) connectionLoop() {
	for q.running {
		select {
		case <-q.ctx.Done():
			return
		default:
			if err := q.connect(); err != nil {
				logger.Warn("QQ WebSocket connection failed, reconnecting in 5s...", "error", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// Connection established, start message loop
			q.messageLoop()

			// Connection lost, wait before reconnect
			if q.running {
				logger.Info("QQ connection lost, reconnecting in 5s...")
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// connect 建立WebSocket连接
func (q *QQChannel) connect() error {
	q.connMu.Lock()
	defer q.connMu.Unlock()

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(q.ctx, qqGatewayURL, nil)
	if err != nil {
		return fmt.Errorf("dial gateway: %w", err)
	}
	q.conn = conn

	// Start reading messages
	return nil
}

// closeConnection 关闭WebSocket连接
func (q *QQChannel) closeConnection() {
	q.connMu.Lock()
	defer q.connMu.Unlock()
	if q.conn != nil {
		q.conn.Close()
		q.conn = nil
	}
}

// messageLoop 消息循环
func (q *QQChannel) messageLoop() {
	defer q.closeConnection()

	for {
		select {
		case <-q.ctx.Done():
			return
		default:
			if q.conn == nil {
				return
			}

			_, message, err := q.conn.ReadMessage()
			if err != nil {
				if q.running {
					logger.Warn("QQ WebSocket read error", "error", err)
				}
				return
			}

			if err := q.handlePayload(message); err != nil {
				logger.Warn("QQ payload handling error", "error", err)
			}
		}
	}
}

// handlePayload 处理WebSocket消息
func (q *QQChannel) handlePayload(data []byte) error {
	var payload qqPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// Update sequence number
	if payload.S > 0 {
		q.sequence = payload.S
	}

	switch payload.Op {
	case qqOpHello:
		// Server hello, need to identify
		var helloData qqHelloData
		if err := json.Unmarshal(payload.D, &helloData); err != nil {
			return fmt.Errorf("unmarshal hello: %w", err)
		}
		q.heartbeatInterval = time.Duration(helloData.HeartbeatInterval) * time.Millisecond
		q.startHeartbeat()
		return q.identify()

	case qqOpDispatch:
		return q.handleDispatch(payload.T, payload.D)

	case qqOpHeartbeatAck:
		q.lastHeartbeatAck = time.Now()
		logger.Debug("QQ heartbeat acknowledged")

	case qqOpReconnect:
		logger.Info("QQ server requested reconnect")
		return fmt.Errorf("reconnect requested")

	case qqOpInvalidSession:
		logger.Warn("QQ invalid session, will re-identify")
		q.sessionID = ""
		return fmt.Errorf("invalid session")

	default:
		logger.Debug("QQ unknown opcode", "opcode", payload.Op)
	}

	return nil
}

// handleDispatch 处理事件分发
func (q *QQChannel) handleDispatch(eventType string, data json.RawMessage) error {
	switch eventType {
	case qqEventReady:
		var ready qqReadyData
		if err := json.Unmarshal(data, &ready); err != nil {
			return fmt.Errorf("unmarshal ready: %w", err)
		}
		q.sessionID = ready.SessionID
		logger.Info("QQ bot ready", "username", ready.User.Username, "session", ready.SessionID[:8])

	case qqEventC2CMessageCreate, qqEventDirectMessage:
		var msg qqC2CMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return fmt.Errorf("unmarshal message: %w", err)
		}
		go q.handleMessage(&msg)

	default:
		logger.Debug("QQ unhandled event", "event", eventType)
	}
	return nil
}

// identify 发送身份认证
func (q *QQChannel) identify() error {
	identify := qqPayload{
		Op: qqOpIdentify,
		D: mustMarshal(qqIdentifyData{
			Token: qqToken{
				AppID: q.cfg.AppID,
				Token: q.cfg.Secret,
			},
			Intents: qqIntentDirectMessage | qqIntentPublicMessages,
			Properties: qqProps{
				OS:      "linux",
				Browser: "lingguard",
				Device:  "lingguard",
			},
		}),
	}
	return q.sendPayload(&identify)
}

// startHeartbeat 启动心跳
func (q *QQChannel) startHeartbeat() {
	if q.heartbeatTicker != nil {
		q.heartbeatTicker.Stop()
	}
	q.heartbeatTicker = time.NewTicker(q.heartbeatInterval)
	q.lastHeartbeatAck = time.Now()

	go func() {
		for {
			select {
			case <-q.ctx.Done():
				return
			case <-q.heartbeatTicker.C:
				if err := q.sendHeartbeat(); err != nil {
					logger.Warn("QQ heartbeat failed", "error", err)
					return
				}
				// Check if we're receiving acks
				if time.Since(q.lastHeartbeatAck) > q.heartbeatInterval*3 {
					logger.Warn("QQ heartbeat timeout, reconnecting...")
					q.closeConnection()
					return
				}
			}
		}
	}()
}

// stopHeartbeat 停止心跳
func (q *QQChannel) stopHeartbeat() {
	if q.heartbeatTicker != nil {
		q.heartbeatTicker.Stop()
		q.heartbeatTicker = nil
	}
}

// sendHeartbeat 发送心跳
func (q *QQChannel) sendHeartbeat() error {
	heartbeat := qqPayload{
		Op: qqOpHeartbeat,
		D:  json.RawMessage("null"),
		S:  q.sequence,
	}
	return q.sendPayload(&heartbeat)
}

// sendPayload 发送WebSocket消息
func (q *QQChannel) sendPayload(payload *qqPayload) error {
	q.connMu.Lock()
	defer q.connMu.Unlock()
	if q.conn == nil {
		return fmt.Errorf("connection not established")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return q.conn.WriteMessage(websocket.TextMessage, data)
}

// handleMessage 处理接收到的消息
func (q *QQChannel) handleMessage(msg *qqC2CMessage) {
	// Deduplication check
	if q.isProcessed(msg.ID) {
		logger.Debug("Skipping duplicate QQ message", "id", msg.ID)
		return
	}
	q.markProcessed(msg.ID)

	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return
	}

	userID := msg.Author.ID
	if userID == "" {
		return
	}

	// Permission check
	if len(q.allowMap) > 0 && !q.allowMap[userID] {
		logger.Warn("Access denied on channel qq. Add to allowFrom list to grant access.", "sender", userID)
		return
	}

	// Build Message
	channelMsg := &Message{
		ID:        msg.ID,
		SessionID: "qq-" + userID,
		Content:   content,
		Channel:   "qq",
		UserID:    userID,
		Metadata: map[string]any{
			"user_id":    userID,
			"username":   msg.Author.Username,
			"message_id": msg.ID,
		},
	}

	logger.Debug("Received QQ message", "sender", userID, "content", truncateContent(content, 100))

	// 检查是否支持流式处理
	if q.streamingHandler != nil {
		q.handleMessageStream(q.ctx, channelMsg, userID)
		return
	}

	// Call handler (non-streaming fallback)
	reply, err := q.handler.HandleMessage(q.ctx, channelMsg)
	if err != nil {
		logger.Error("Handler error", "error", err)
		return
	}

	// Send reply
	if reply != "" {
		if err := q.sendC2CMessage(q.ctx, userID, reply); err != nil {
			logger.Error("Failed to send QQ reply", "error", err)
		}
	}
}

// handleMessageStream 流式处理消息
func (q *QQChannel) handleMessageStream(ctx context.Context, msg *Message, userID string) {
	var contentBuilder strings.Builder
	var lastContent string
	var lastUpdate time.Time
	updateInterval := 500 * time.Millisecond // QQ API 限流较严，降低更新频率

	err := q.streamingHandler.HandleMessageStream(ctx, msg, func(event stream.StreamEvent) {
		switch event.Type {
		case stream.EventText:
			contentBuilder.WriteString(event.Content)

			// 节流更新
			now := time.Now()
			if now.Sub(lastUpdate) < updateInterval {
				return
			}
			lastUpdate = now

			// QQ 不支持消息编辑，需要发送新消息或累积后发送
			// 这里采用累积后一次性发送的策略

		case stream.EventToolStart:
			// 工具执行状态（静默处理，最终结果会在完成时显示）

		case stream.EventDone:
			// 最终发送
			content := contentBuilder.String()
			if content != "" {
				if err := q.sendC2CMessage(ctx, userID, content); err != nil {
					logger.Error("Failed to send QQ message", "error", err)
				}
			}

		case stream.EventError:
			logger.Error("Stream error", "error", event.Error)
			errorContent := contentBuilder.String() + fmt.Sprintf("\n\n错误: %s", event.Error.Error())
			if errorContent != "" && errorContent != lastContent {
				q.sendC2CMessage(ctx, userID, errorContent)
			}
		}
	})

	if err != nil {
		logger.Error("Stream handling error", "error", err)
	}
}

// sendC2CMessage 发送私聊消息
func (q *QQChannel) sendC2CMessage(ctx context.Context, openid string, content string) error {
	if openid == "" {
		return fmt.Errorf("openid is empty")
	}

	// QQ API 限流：每分钟最多 5 条消息给同一用户
	// 这里简单实现，实际可能需要更复杂的限流控制

	url := fmt.Sprintf("%s/v2/users/%s/messages", qqAPIBase, openid)

	body := map[string]any{
		"content":  content,
		"msg_type": 0, // 文本消息
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// QQ Bot API Authorization: Bot {appid}.{secret}
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s.%s", q.cfg.AppID, q.cfg.Secret))
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("QQ API error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	logger.Debug("QQ message sent", "to", openid)
	return nil
}

// isProcessed 检查消息是否已处理
func (q *QQChannel) isProcessed(messageID string) bool {
	q.dedupeMu.Lock()
	defer q.dedupeMu.Unlock()

	// Clean up old entries (keep last 1 hour)
	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)
	var toDelete []string
	q.processedMsgs.Range(func(key, value any) bool {
		if t, ok := value.(time.Time); ok && t.Before(cutoff) {
			toDelete = append(toDelete, key.(string))
		}
		return true
	})
	for _, k := range toDelete {
		q.processedMsgs.Delete(k)
	}

	_, exists := q.processedMsgs.Load(messageID)
	return exists
}

// markProcessed 标记消息为已处理
func (q *QQChannel) markProcessed(messageID string) {
	q.processedMsgs.Store(messageID, time.Now())
}

// mustMarshal 辅助函数：必须成功的 JSON 序列化
func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// truncateContent 截断内容用于日志
func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
