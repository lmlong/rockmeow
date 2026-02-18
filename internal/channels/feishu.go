package channels

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/stream"
)

// Message type display mapping for non-text messages
var msgTypeMap = map[string]string{
	"image":   "[image]",
	"audio":   "[audio]",
	"file":    "[file]",
	"sticker": "[sticker]",
	"video":   "[video]",
}

// FeishuChannel 飞书 WebSocket 渠道
type FeishuChannel struct {
	cfg              *config.FeishuConfig
	client           *lark.Client
	wsClient         *larkws.Client
	handler          MessageHandler
	streamingHandler StreamingMessageHandler
	mu               sync.RWMutex
	running          bool
	allowMap         map[string]bool

	// Message deduplication
	processedMsgs sync.Map // map[string]time.Time
	dedupeMu      sync.Mutex

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// NewFeishuChannel 创建飞书渠道
func NewFeishuChannel(cfg *config.FeishuConfig, handler MessageHandler) *FeishuChannel {
	allowMap := make(map[string]bool)
	for _, id := range cfg.AllowFrom {
		allowMap[id] = true
	}
	fc := &FeishuChannel{
		cfg:      cfg,
		handler:  handler,
		allowMap: allowMap,
	}
	// 检查是否实现了流式处理器接口
	if sh, ok := handler.(StreamingMessageHandler); ok {
		fc.streamingHandler = sh
	}
	return fc
}

// Name 返回渠道名称
func (f *FeishuChannel) Name() string { return "feishu" }

// Start 启动渠道
func (f *FeishuChannel) Start(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.running {
		return nil
	}

	// Create context for graceful shutdown
	f.ctx, f.cancel = context.WithCancel(ctx)

	// Create Lark Client for sending messages
	f.client = lark.NewClient(f.cfg.AppID, f.cfg.AppSecret)

	// Create event handler
	eventDispatcher := dispatcher.NewEventDispatcher(f.cfg.VerificationToken, f.cfg.EncryptKey).
		OnP2MessageReceiveV1(f.handleMessage)

	// Create WebSocket client with debug logging
	f.wsClient = larkws.NewClient(f.cfg.AppID, f.cfg.AppSecret,
		larkws.WithEventHandler(eventDispatcher),
		larkws.WithAutoReconnect(true),
		larkws.WithLogLevel(larkcore.LogLevelDebug),
	)

	// Start WebSocket client in a separate goroutine with reconnect loop
	go func() {
		for {
			select {
			case <-f.ctx.Done():
				return
			default:
				if err := f.wsClient.Start(f.ctx); err != nil {
					logger.Warn("Feishu WebSocket error, reconnecting in 5s...", "error", err)
					time.Sleep(5 * time.Second)
				}
			}
		}
	}()

	f.running = true
	logger.Info("Feishu channel started with WebSocket long connection")
	logger.Info("No public IP required - using WebSocket to receive events")
	return nil
}

// Stop 停止渠道
func (f *FeishuChannel) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.cancel != nil {
		f.cancel()
	}
	f.running = false
	logger.Info("Feishu channel stopped")
	return nil
}

// IsRunning 检查是否运行中
func (f *FeishuChannel) IsRunning() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.running
}

// Send 主动发送消息（实现 SendableChannel 接口）
func (f *FeishuChannel) Send(ctx context.Context, to string, content string) error {
	return f.sendReply(ctx, to, content)
}

// handleMessage 处理接收到的消息
func (f *FeishuChannel) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event.Event == nil || event.Event.Message == nil {
		return nil
	}

	msg := event.Event.Message
	sender := event.Event.Sender

	// Get message ID for deduplication
	messageID := safeString(msg.MessageId)
	if messageID == "" {
		return nil
	}

	// Deduplication check
	if f.isProcessed(messageID) {
		logger.Debug("Skipping duplicate message", "messageID", messageID)
		return nil
	}
	f.markProcessed(messageID)

	// Skip bot messages
	if sender != nil && safeString(sender.SenderType) == "bot" {
		return nil
	}

	// Get sender ID
	var senderID string
	if sender != nil && sender.SenderId != nil && sender.SenderId.OpenId != nil {
		senderID = *sender.SenderId.OpenId
	}
	if senderID == "" {
		return nil
	}

	// Permission check
	if len(f.allowMap) > 0 && !f.allowMap[senderID] {
		logger.Warn("Access denied on channel feishu. Add to allowFrom list to grant access.", "sender", senderID)
		return nil
	}

	chatID := safeString(msg.ChatId)
	chatType := safeString(msg.ChatType)
	msgType := safeString(msg.MessageType)

	// Add reaction to indicate "seen"
	go f.addReaction(messageID, "THUMBSUP")

	// Parse message content
	var content string
	var mediaPaths []string

	if msgType == "text" {
		content = f.parseTextContent(msg.Content)
	} else if msgType == "image" {
		// 下载图片并保存到本地
		imagePath, err := f.downloadImage(ctx, msg.Content, safeString(msg.MessageId))
		if err != nil {
			logger.Warn("Failed to download image", "error", err)
			content = "[image: download failed]"
		} else {
			mediaPaths = append(mediaPaths, imagePath)
			content = "[image]"
			logger.Debug("Downloaded image", "path", imagePath)
		}
	} else if msgType == "video" {
		// 下载视频并保存到本地（部分模型支持）
		videoPath, err := f.downloadVideo(ctx, msg.Content, safeString(msg.MessageId))
		if err != nil {
			logger.Warn("Failed to download video", "error", err)
			content = "[video: download failed]"
		} else {
			mediaPaths = append(mediaPaths, videoPath)
			content = "[video]"
			logger.Debug("Downloaded video", "path", videoPath)
		}
	} else {
		content = msgTypeMap[msgType]
		if content == "" {
			content = fmt.Sprintf("[%s]", msgType)
		}
	}

	if content == "" && len(mediaPaths) == 0 {
		return nil
	}

	// Determine reply target: group -> chat_id, p2p -> sender open_id
	replyTo := chatID
	if chatType == "p2p" {
		replyTo = senderID
	}

	// Build Message
	channelMsg := &Message{
		ID:        messageID,
		SessionID: "feishu-" + senderID,
		Content:   strings.TrimSpace(content),
		Media:     mediaPaths,
		Channel:   "feishu",
		UserID:    replyTo, // Use reply_to as the user ID for delivery
		Metadata: map[string]any{
			"chat_id":    chatID,
			"open_id":    senderID,
			"chat_type":  chatType,
			"msg_type":   msgType,
			"reply_to":   replyTo,
			"message_id": messageID,
		},
	}

	logger.Debug("Received message", "sender", senderID, "chatType", chatType, "content", truncateLog(channelMsg.Content, 100))

	// 检查是否支持流式处理
	if f.streamingHandler != nil {
		return f.handleMessageStream(ctx, channelMsg, replyTo)
	}

	// Call handler (non-streaming fallback)
	reply, err := f.handler.HandleMessage(ctx, channelMsg)
	if err != nil {
		logger.Error("Handler error", "error", err)
		return err
	}

	// Send reply
	if reply != "" {
		return f.sendReply(ctx, replyTo, reply)
	}
	return nil
}

// handleMessageStream 流式处理消息
func (f *FeishuChannel) handleMessageStream(ctx context.Context, msg *Message, replyTo string) error {
	var contentBuilder strings.Builder
	var messageID string
	var lastUpdate time.Time
	updateInterval := 500 * time.Millisecond // 最小更新间隔

	err := f.streamingHandler.HandleMessageStream(ctx, msg, func(event stream.StreamEvent) {
		switch event.Type {
		case stream.EventText:
			contentBuilder.WriteString(event.Content)

			// 节流更新：避免过于频繁调用 API
			now := time.Now()
			if now.Sub(lastUpdate) < updateInterval {
				return
			}
			lastUpdate = now

			// 更新或创建消息
			content := contentBuilder.String()
			if content != "" {
				if messageID == "" {
					// 发送初始消息
					if id, err := f.sendReplyAsync(ctx, replyTo, content); err == nil {
						messageID = id
					}
				} else {
					// 更新消息
					f.updateReply(ctx, messageID, content)
				}
			}

		case stream.EventToolStart:
			// 工具开始时更新消息显示工具执行状态
			toolContent := contentBuilder.String() + fmt.Sprintf("\n\n⚙️ 正在执行工具: %s...", event.ToolName)
			if messageID == "" {
				if id, err := f.sendReplyAsync(ctx, replyTo, toolContent); err == nil {
					messageID = id
				}
			} else {
				f.updateReply(ctx, messageID, toolContent)
			}

		case stream.EventToolEnd:
			// 工具结束时可以显示结果摘要（可选）

		case stream.EventDone:
			// 最终更新
			content := contentBuilder.String()
			if content != "" {
				if messageID == "" {
					f.sendReply(ctx, replyTo, content)
				} else {
					f.updateReply(ctx, messageID, content)
				}
			}

		case stream.EventError:
			logger.Error("Stream error", "error", event.Error)
			errorContent := contentBuilder.String() + fmt.Sprintf("\n\n❌ 错误: %s", event.Error.Error())
			if messageID == "" {
				f.sendReply(ctx, replyTo, errorContent)
			} else {
				f.updateReply(ctx, messageID, errorContent)
			}
		}
	})

	return err
}

// addReaction 添加表情反应
func (f *FeishuChannel) addReaction(messageID, emojiType string) {
	if f.client == nil {
		return
	}

	req := larkim.NewCreateMessageReactionReqBuilder().
		MessageId(messageID).
		Body(larkim.NewCreateMessageReactionReqBodyBuilder().
			ReactionType(larkim.NewEmojiBuilder().
				EmojiType(emojiType).
				Build()).
			Build()).
		Build()

	resp, err := f.client.Im.MessageReaction.Create(context.Background(), req)
	if err != nil {
		logger.Debug("Failed to add reaction", "error", err)
		return
	}

	if resp.Code != 0 {
		logger.Debug("Failed to add reaction", "code", resp.Code, "msg", resp.Msg)
	} else {
		logger.Debug("Added reaction to message", "emoji", emojiType, "messageID", messageID)
	}
}

// sendReply 发送回复消息 (使用 Interactive Card)
func (f *FeishuChannel) sendReply(ctx context.Context, receiveID, content string) error {
	if receiveID == "" {
		return fmt.Errorf("receive_id is empty")
	}

	// Determine receive_id_type based on ID format
	// open_id starts with "ou_", chat_id starts with "oc_"
	receiveIDType := larkim.ReceiveIdTypeOpenId
	if strings.HasPrefix(receiveID, "oc_") {
		receiveIDType = larkim.ReceiveIdTypeChatId
	}

	// Build interactive card with markdown support
	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"elements": []map[string]any{
			{
				"tag":     "markdown",
				"content": content,
			},
		},
	}

	cardJSON, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("marshal card: %w", err)
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType(larkim.MsgTypeInteractive).
			Content(string(cardJSON)).
			Build()).
		Build()

	resp, err := f.client.Im.Message.Create(ctx, req)
	if err != nil {
		logger.Error("Failed to send reply", "error", err)
		return err
	}

	if resp.Code != 0 {
		logger.Error("Failed to send reply", "code", resp.Code, "msg", resp.Msg)
		return fmt.Errorf("send message failed: %s", resp.Msg)
	}

	logger.Debug("Reply sent successfully", "to", receiveID)
	return nil
}

// sendReplyAsync 发送回复消息并返回消息ID (用于流式更新)
func (f *FeishuChannel) sendReplyAsync(ctx context.Context, receiveID, content string) (string, error) {
	if receiveID == "" {
		return "", fmt.Errorf("receive_id is empty")
	}

	// Determine receive_id_type based on ID format
	receiveIDType := larkim.ReceiveIdTypeOpenId
	if strings.HasPrefix(receiveID, "oc_") {
		receiveIDType = larkim.ReceiveIdTypeChatId
	}

	// Build interactive card with markdown support
	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"elements": []map[string]any{
			{
				"tag":     "markdown",
				"content": content,
			},
		},
	}

	cardJSON, err := json.Marshal(card)
	if err != nil {
		return "", fmt.Errorf("marshal card: %w", err)
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType(larkim.MsgTypeInteractive).
			Content(string(cardJSON)).
			Build()).
		Build()

	resp, err := f.client.Im.Message.Create(ctx, req)
	if err != nil {
		logger.Error("Failed to send reply", "error", err)
		return "", err
	}

	if resp.Code != 0 {
		logger.Error("Failed to send reply", "code", resp.Code, "msg", resp.Msg)
		return "", fmt.Errorf("send message failed: %s", resp.Msg)
	}

	logger.Debug("Reply sent successfully", "to", receiveID, "messageID", safeString(resp.Data.MessageId))
	return safeString(resp.Data.MessageId), nil
}

// updateReply 更新已发送的消息 (用于流式更新)
func (f *FeishuChannel) updateReply(ctx context.Context, messageID, content string) error {
	if messageID == "" {
		return fmt.Errorf("message_id is empty")
	}

	// Build interactive card with markdown support
	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"elements": []map[string]any{
			{
				"tag":     "markdown",
				"content": content,
			},
		},
	}

	cardJSON, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("marshal card: %w", err)
	}

	req := larkim.NewPatchMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(string(cardJSON)).
			Build()).
		Build()

	resp, err := f.client.Im.Message.Patch(ctx, req)
	if err != nil {
		logger.Debug("Failed to update reply", "error", err)
		return err
	}

	if resp.Code != 0 {
		logger.Debug("Failed to update reply", "code", resp.Code, "msg", resp.Msg)
		return fmt.Errorf("update message failed: %s", resp.Msg)
	}

	logger.Debug("Reply updated successfully", "messageID", messageID)
	return nil
}

// isProcessed 检查消息是否已处理
func (f *FeishuChannel) isProcessed(messageID string) bool {
	f.dedupeMu.Lock()
	defer f.dedupeMu.Unlock()

	// Clean up old entries (keep last 1 hour)
	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)
	var toDelete []string
	f.processedMsgs.Range(func(key, value any) bool {
		if t, ok := value.(time.Time); ok && t.Before(cutoff) {
			toDelete = append(toDelete, key.(string))
		}
		return true
	})
	for _, k := range toDelete {
		f.processedMsgs.Delete(k)
	}

	_, exists := f.processedMsgs.Load(messageID)
	return exists
}

// markProcessed 标记消息为已处理
func (f *FeishuChannel) markProcessed(messageID string) {
	f.processedMsgs.Store(messageID, time.Now())
}

// parseTextContent 解析文本消息内容
func (f *FeishuChannel) parseTextContent(content *string) string {
	if content == nil || *content == "" {
		return ""
	}

	// Feishu text message format: {"text":"actual content"}
	var textMsg struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(*content), &textMsg); err == nil {
		return textMsg.Text
	}

	// If parsing fails, return raw content
	return *content
}

// safeString 安全获取字符串指针
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// truncateLog 截断日志内容
func truncateLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// escapeJSONString 转义 JSON 字符串中的特殊字符
func escapeJSONString(s string) string {
	var result strings.Builder
	for _, c := range s {
		switch c {
		case '"':
			result.WriteString(`\"`)
		case '\\':
			result.WriteString(`\\`)
		case '\n':
			result.WriteString(`\n`)
		case '\r':
			result.WriteString(`\r`)
		case '\t':
			result.WriteString(`\t`)
		default:
			result.WriteRune(c)
		}
	}
	return result.String()
}

// downloadImage 下载飞书图片并保存到本地
func (f *FeishuChannel) downloadImage(ctx context.Context, content *string, messageID string) (string, error) {
	if content == nil || f.client == nil {
		return "", fmt.Errorf("invalid parameters")
	}

	// 解析图片消息内容: {"image_key": "img_xxx"}
	var imageMsg struct {
		ImageKey string `json:"image_key"`
	}
	if err := json.Unmarshal([]byte(*content), &imageMsg); err != nil {
		return "", fmt.Errorf("parse image content: %w", err)
	}

	if imageMsg.ImageKey == "" {
		return "", fmt.Errorf("empty image_key")
	}

	// 创建媒体目录
	home, _ := os.UserHomeDir()
	mediaDir := filepath.Join(home, ".lingguard", "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return "", fmt.Errorf("create media dir: %w", err)
	}

	// 生成文件名
	ext := ".jpg" // 默认扩展名
	timestamp := time.Now().UnixNano()
	filename := fmt.Sprintf("feishu_%d_%s%s", timestamp, messageID[:8], ext)
	filePath := filepath.Join(mediaDir, filename)

	// 获取图片资源请求
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(imageMsg.ImageKey).
		Type("image").
		Build()

	// 获取图片并直接保存到文件
	resp, err := f.client.Im.MessageResource.Get(ctx, req)
	if err != nil {
		return "", fmt.Errorf("get image resource: %w", err)
	}

	if resp.Code != 0 {
		return "", fmt.Errorf("get image failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	// 使用 SDK 提供的 WriteFile 方法保存文件
	if err := resp.WriteFile(filePath); err != nil {
		return "", fmt.Errorf("write image file: %w", err)
	}

	return filePath, nil
}

// downloadVideo 下载飞书视频并保存到本地
func (f *FeishuChannel) downloadVideo(ctx context.Context, content *string, messageID string) (string, error) {
	if content == nil || f.client == nil {
		return "", fmt.Errorf("invalid parameters")
	}

	// 解析视频消息内容: {"file_key": "file_xxx"}
	var videoMsg struct {
		FileKey string `json:"file_key"`
	}
	if err := json.Unmarshal([]byte(*content), &videoMsg); err != nil {
		return "", fmt.Errorf("parse video content: %w", err)
	}

	if videoMsg.FileKey == "" {
		return "", fmt.Errorf("empty file_key")
	}

	// 创建媒体目录
	home, _ := os.UserHomeDir()
	mediaDir := filepath.Join(home, ".lingguard", "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return "", fmt.Errorf("create media dir: %w", err)
	}

	// 生成文件名
	ext := ".mp4" // 默认扩展名
	timestamp := time.Now().UnixNano()
	filename := fmt.Sprintf("feishu_%d_%s%s", timestamp, messageID[:8], ext)
	filePath := filepath.Join(mediaDir, filename)

	// 获取视频资源请求
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(videoMsg.FileKey).
		Type("file").
		Build()

	// 获取视频并直接保存到文件
	resp, err := f.client.Im.MessageResource.Get(ctx, req)
	if err != nil {
		return "", fmt.Errorf("get video resource: %w", err)
	}

	if resp.Code != 0 {
		return "", fmt.Errorf("get video failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	// 使用 SDK 提供的 WriteFile 方法保存文件
	if err := resp.WriteFile(filePath); err != nil {
		return "", fmt.Errorf("write video file: %w", err)
	}

	return filePath, nil
}

// downloadImageFromBase64 从 base64 数据保存图片（用于发送）
func (f *FeishuChannel) downloadImageFromBase64(base64Data, mimeType, messageID string) (string, error) {
	// 创建媒体目录
	home, _ := os.UserHomeDir()
	mediaDir := filepath.Join(home, ".lingguard", "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return "", fmt.Errorf("create media dir: %w", err)
	}

	// 解码 base64
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	// 确定扩展名
	ext := ".bin"
	switch mimeType {
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	}

	// 生成文件名
	timestamp := time.Now().UnixNano()
	filename := fmt.Sprintf("feishu_%d_%s%s", timestamp, messageID[:8], ext)
	filePath := filepath.Join(mediaDir, filename)

	// 保存文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("save file: %w", err)
	}

	return filePath, nil
}

// Compile-time check for unused code (regexp for markdown table parsing if needed later)
var _ = regexp.MustCompile(``)
