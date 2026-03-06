// TODO(architecture): This file is too large (1463 lines) and should be split into:
// - feishu/client.go (WebSocket client management)
// - feishu/message.go (Message handling)
// - feishu/stream.go (Streaming response handling)
// - feishu/media.go (Media upload/download)
// - feishu/card.go (Message card operations)
// Priority: P1 - Estimated effort: 2-3 days
// Related: #code-quality #refactoring

package channels

import (
	"bytes"
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
	"github.com/lingguard/pkg/memory"
	"github.com/lingguard/pkg/speech"
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
	speechCfg        *config.SpeechConfig
	workspace        string // 工作目录，媒体文件保存在 workspace/media
	client           *lark.Client
	wsClient         *larkws.Client
	handler          MessageHandler
	streamingHandler StreamingMessageHandler
	speechService    speech.Service
	mu               sync.RWMutex
	running          bool
	allowMap         map[string]bool
	profileStore     *memory.ProfileStore // 用户档案存储
	soulConfig       *config.SoulConfig   // Soul 配置

	// Message deduplication - 使用 map + RWMutex 替代 sync.Map，避免并发问题
	processedMsgs map[string]time.Time
	dedupeMu      sync.RWMutex

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// Soul 定义状态管理（用于追踪等待用户定义 Soul 的状态）
	soulPendingUsers map[string]time.Time // 等待 Soul 定义的用户及时间
	soulPendingMu    sync.RWMutex
}

// NewFeishuChannel 创建飞书渠道
func NewFeishuChannel(cfg *config.FeishuConfig, speechCfg *config.SpeechConfig, providers map[string]config.ProviderConfig, workspace string, handler MessageHandler, profileStore *memory.ProfileStore, soulConfig *config.SoulConfig) *FeishuChannel {
	allowMap := make(map[string]bool)
	for _, id := range cfg.AllowFrom {
		allowMap[id] = true
	}
	fc := &FeishuChannel{
		cfg:              cfg,
		speechCfg:        speechCfg,
		workspace:        workspace,
		handler:          handler,
		allowMap:         allowMap,
		processedMsgs:    make(map[string]time.Time),
		profileStore:     profileStore,
		soulConfig:       soulConfig,
		soulPendingUsers: make(map[string]time.Time),
	}
	// 检查是否实现了流式处理器接口
	if sh, ok := handler.(StreamingMessageHandler); ok {
		fc.streamingHandler = sh
	}
	// 初始化语音识别服务
	if speechCfg != nil && speechCfg.Enabled {
		// 如果没有配置 apiKey，从对应 provider 获取
		apiKey := speechCfg.APIKey
		if apiKey == "" && speechCfg.Provider != "" {
			if providerCfg, ok := providers[speechCfg.Provider]; ok {
				apiKey = providerCfg.APIKey
			}
		}
		svc, err := speech.NewService(&speech.Config{
			Provider: speechCfg.Provider,
			APIKey:   apiKey,
			APIBase:  speechCfg.APIBase,
			Model:    speechCfg.Model,
			Format:   speechCfg.Format,
			Language: speechCfg.Language,
			Timeout:  speechCfg.Timeout,
		})
		if err != nil {
			logger.Warn("Failed to init speech service", "error", err)
		} else {
			fc.speechService = svc
			logger.Info("Speech recognition enabled for Feishu channel", "provider", speechCfg.Provider)
		}
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

	logger.Info("Initializing Feishu channel", "appId", maskAppID(f.cfg.AppID))

	// Create Lark Client for sending messages
	f.client = lark.NewClient(f.cfg.AppID, f.cfg.AppSecret)
	logger.Info("Lark API client created")

	// Create event handler
	eventDispatcher := dispatcher.NewEventDispatcher(f.cfg.VerificationToken, f.cfg.EncryptKey).
		OnP2MessageReceiveV1(f.handleMessage)
	logger.Info("Event dispatcher created")

	// Create WebSocket client with debug logging
	f.wsClient = larkws.NewClient(f.cfg.AppID, f.cfg.AppSecret,
		larkws.WithEventHandler(eventDispatcher),
		larkws.WithAutoReconnect(true),
		larkws.WithLogLevel(larkcore.LogLevelDebug),
	)
	logger.Info("WebSocket client created")

	// Start WebSocket client in a separate goroutine with reconnect loop
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Feishu WebSocket goroutine panic recovered", "error", r)
			}
		}()

		reconnectCount := 0
		for {
			select {
			case <-f.ctx.Done():
				logger.Info("Feishu WebSocket stopping...")
				return
			default:
				logger.Info("Feishu WebSocket connecting...", "attempt", reconnectCount+1)
				if err := f.wsClient.Start(f.ctx); err != nil {
					reconnectCount++
					logger.Warn("Feishu WebSocket connection failed, reconnecting in 5s...", "error", err, "attempt", reconnectCount)
					time.Sleep(5 * time.Second)
				} else {
					// Normal exit (context cancelled)
					logger.Info("Feishu WebSocket connection closed normally")
					return
				}
			}
		}
	}()

	f.running = true
	logger.Info("Feishu channel started with WebSocket long connection")
	logger.Info("No public IP required - using WebSocket to receive events")
	return nil
}

// maskAppID masks middle part of AppID for logging
func maskAppID(appID string) string {
	if len(appID) <= 8 {
		return appID
	}
	return appID[:4] + "****" + appID[len(appID)-4:]
}

// Stop 停止渠道
func (f *FeishuChannel) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.cancel != nil {
		f.cancel()
	}

	// 尝试关闭 WebSocket 客户端
	if f.wsClient != nil {
		// lark WebSocket 客户端没有 Stop 方法，但 context 取消应该会停止它
		// 给它一小段时间来清理
		time.Sleep(100 * time.Millisecond)
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
		logger.Debug("Skipping bot message", "messageID", messageID)
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

	// Log message received
	logger.Info("Message received", "messageId", messageID, "chatId", chatID, "chatType", chatType, "msgType", msgType, "sender", senderID)

	// Add reaction to indicate "seen"
	go f.addReaction(messageID, "OK")

	// Parse message content
	var content string
	var mediaPaths []string

	if msgType == "text" {
		content = f.parseTextContent(msg.Content)
	} else if msgType == "audio" {
		// 下载音频并进行语音识别
		audioPath, err := f.downloadAudio(ctx, msg.Content, safeString(msg.MessageId))
		if err != nil {
			logger.Warn("Failed to download audio", "error", err)
			content = "[audio: download failed]"
		} else if f.speechService != nil {
			// 语音识别
			result, err := f.speechService.Transcribe(ctx, audioPath)
			if err != nil {
				logger.Warn("Failed to transcribe audio", "error", err)
				content = fmt.Sprintf("[audio: transcription failed, file saved to %s]", audioPath)
			} else {
				content = result.Text
				logger.Info("Audio transcribed", "text", result.Text, "duration", result.Duration, "messageId", messageID)
			}
		} else {
			content = "[audio: speech recognition not configured]"
			logger.Info("Downloaded audio but speech service not available", "path", audioPath, "messageId", messageID)
		}
	} else if msgType == "image" {
		// 下载图片并保存到本地
		imagePath, err := f.downloadImage(ctx, msg.Content, safeString(msg.MessageId))
		if err != nil {
			logger.Warn("Failed to download image", "error", err)
			content = "[image: download failed]"
		} else {
			mediaPaths = append(mediaPaths, imagePath)
			content = fmt.Sprintf("[image saved to: %s]", imagePath)
			logger.Info("Downloaded image for multimodal processing", "path", imagePath, "messageId", messageID)
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
			logger.Info("Downloaded video for multimodal processing", "path", videoPath, "messageId", messageID)
		}
	} else if msgType == "media" {
		// 飞书媒体消息（视频/图片混合），包含 file_key 和 image_key
		mediaPath, mediaType, err := f.downloadMedia(ctx, msg.Content, safeString(msg.MessageId))
		if err != nil {
			logger.Warn("Failed to download media", "error", err)
			content = "[media: download failed]"
		} else {
			mediaPaths = append(mediaPaths, mediaPath)
			content = fmt.Sprintf("[%s]", mediaType)
			logger.Info("Downloaded media for multimodal processing", "type", mediaType, "path", mediaPath, "messageId", messageID)
		}
	} else if msgType == "post" {
		// 飞书富文本消息，可能包含嵌入的媒体（图片/视频）
		// 解析 post 内容提取所有媒体文件
		mediaInfos, err := f.downloadPostAllMedia(ctx, msg.Content, safeString(msg.MessageId))
		if err != nil {
			logger.Warn("Failed to download post media", "error", err)
			content = "[post: media download failed]"
		} else if len(mediaInfos) > 0 {
			// 收集所有媒体路径
			var typeSet = make(map[string]bool)
			for _, info := range mediaInfos {
				mediaPaths = append(mediaPaths, info.Path)
				typeSet[info.Type] = true
			}
			// 构建媒体类型描述
			var typeStrs []string
			if typeSet["image"] {
				typeStrs = append(typeStrs, "image")
			}
			if typeSet["video"] {
				typeStrs = append(typeStrs, "video")
			}
			mediaDesc := strings.Join(typeStrs, "+")
			if len(mediaInfos) > 1 {
				mediaDesc = fmt.Sprintf("%dx%s", len(mediaInfos), mediaDesc)
			}

			// 同时提取文本内容，保留用户的文本请求
			textContent := f.parsePostContent(msg.Content)
			if textContent != "" && textContent != "[post]" {
				content = fmt.Sprintf("[%s] %s", mediaDesc, textContent)
			} else {
				content = fmt.Sprintf("[%s]", mediaDesc)
			}
			logger.Info("Downloaded post media for multimodal processing", "count", len(mediaInfos), "types", mediaDesc, "messageId", messageID)
		} else {
			// 没有媒体，解析纯文本内容
			content = f.parsePostContent(msg.Content)
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

	// SessionID: 群聊使用 chatID，私聊使用 senderID
	// 这样不同群的消息互不阻塞
	sessionID := "feishu-" + chatID
	if chatType == "p2p" {
		sessionID = "feishu-" + senderID
	}

	// Build Message
	channelMsg := &Message{
		ID:        messageID,
		SessionID: sessionID,
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

	// Soul 引导逻辑（仅私聊）
	if f.soulConfig != nil && f.soulConfig.Enabled && chatType == "p2p" && f.profileStore != nil {
		// 检查用户是否正在等待定义 Soul
		if f.isPendingSoulDefinition(senderID) {
			// 用户回复了 Soul 定义
			userInput := strings.TrimSpace(channelMsg.Content)
			if strings.ToLower(userInput) == "跳过" || strings.ToLower(userInput) == "skip" {
				// 用户跳过，使用默认 Soul
				defaultSoul := f.soulConfig.DefaultSoul
				if defaultSoul == "" {
					defaultSoul = "我是一个友好、专业的 AI 助手，致力于帮助用户解决问题。"
				}
				f.profileStore.MarkSoulDefined(senderID, defaultSoul)
				f.clearPendingSoulDefinition(senderID)
				logger.Info("User skipped Soul definition, using default", "userId", senderID)
				// 发送确认消息
				f.sendReply(ctx, replyTo, "好的，我将使用默认设置。有什么我可以帮助你的吗？")
				return nil
			}

			// 保存用户定义的 Soul
			f.profileStore.MarkSoulDefined(senderID, userInput)
			f.clearPendingSoulDefinition(senderID)
			logger.Info("Soul definition saved", "userId", senderID, "definition", truncateLog(userInput, 50))
			// 发送确认消息
			f.sendReply(ctx, replyTo, fmt.Sprintf("感谢你的设定！我已经记住了：%s\n\n现在让我开始为你服务吧，有什么我可以帮助你的？", userInput))
			return nil
		}

		// 检查是否是首次交互或未定义 Soul
		profile, _ := f.profileStore.GetProfile(senderID)
		if profile == nil {
			// 首次交互，创建用户档案并发送引导消息
			f.profileStore.CreateProfile(senderID, "feishu")
			f.sendSoulGuideMessage(ctx, replyTo, senderID)
			return nil
		}

		if !profile.SoulDefined {
			// 用户存在但未定义 Soul，发送引导消息
			f.sendSoulGuideMessage(ctx, replyTo, senderID)
			return nil
		}

		// 检查是否是"你是谁?"问题 - 触发 Soul 重新定义
		if isSoulQuestion(channelMsg.Content) {
			currentSoul := profile.SoulDefinition
			var response string
			if currentSoul != "" {
				response = fmt.Sprintf("根据你之前的设定，我是：\n\n%s\n\n如果你想修改或补充我的人格设定，请直接告诉我你希望我成为什么样的助手。", currentSoul)
			} else {
				response = "我还没有特别的人格设定。\n\n你想让我成为什么样的助手？请告诉我你的期望。"
			}
			// 设置等待状态，让用户可以重新定义 Soul
			f.setPendingSoulDefinition(senderID)
			f.sendReply(ctx, replyTo, response)
			logger.Info("Soul question detected, waiting for redefinition", "userId", senderID)
			return nil
		}
	}

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
		// 检查是否包含生成的图片、视频或音频
		if strings.Contains(reply, "[GENERATED_IMAGE:") || strings.Contains(reply, "[GENERATED_VIDEO:") || strings.Contains(reply, "[GENERATED_AUDIO:") {
			return f.SendWithImages(ctx, replyTo, reply)
		}
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
			// 检查工具结果是否包含生成的图片、视频或音频，如果有则上传并发送
			if strings.Contains(event.ToolResult, "[GENERATED_IMAGE:") || strings.Contains(event.ToolResult, "[GENERATED_VIDEO:") || strings.Contains(event.ToolResult, "[GENERATED_AUDIO:") {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Error("Feishu media upload goroutine panic recovered", "error", r)
						}
					}()

					// 处理图片
					imagePattern := regexp.MustCompile(`\[GENERATED_IMAGE:([^\]]+)\]`)
					imageMatches := imagePattern.FindAllStringSubmatch(event.ToolResult, -1)
					for _, match := range imageMatches {
						if len(match) > 1 {
							imagePath := match[1]
							logger.Info("Auto-uploading generated image", "path", imagePath)

							imageKey, err := f.uploadImage(context.Background(), imagePath)
							if err != nil {
								logger.Warn("Failed to upload image", "path", imagePath, "error", err)
								continue
							}

							if err := f.sendImageMessage(context.Background(), replyTo, imageKey); err != nil {
								logger.Warn("Failed to send image message", "error", err)
							}
						}
					}

					// 处理视频
					videoPattern := regexp.MustCompile(`\[GENERATED_VIDEO:([^\]]+)\]`)
					videoMatches := videoPattern.FindAllStringSubmatch(event.ToolResult, -1)
					for _, match := range videoMatches {
						if len(match) > 1 {
							videoPath := match[1]
							logger.Info("Auto-uploading generated video", "path", videoPath)

							fileKey, err := f.uploadFile(context.Background(), videoPath)
							if err != nil {
								logger.Warn("Failed to upload video", "path", videoPath, "error", err)
								continue
							}

							if err := f.sendFileMessage(context.Background(), replyTo, fileKey); err != nil {
								logger.Warn("Failed to send video message", "error", err)
							}
						}
					}

					// 处理音频
					audioPattern := regexp.MustCompile(`\[GENERATED_AUDIO:([^\]]+)\]`)
					audioMatches := audioPattern.FindAllStringSubmatch(event.ToolResult, -1)
					for _, match := range audioMatches {
						if len(match) > 1 {
							audioPath := match[1]
							logger.Info("Auto-uploading generated audio", "path", audioPath)

							fileKey, err := f.uploadFile(context.Background(), audioPath)
							if err != nil {
								logger.Warn("Failed to upload audio", "path", audioPath, "error", err)
								continue
							}

							if err := f.sendFileMessage(context.Background(), replyTo, fileKey); err != nil {
								logger.Warn("Failed to send audio message", "error", err)
							}
						}
					}
				}()
			}

		case stream.EventDone:
			// 最终更新
			content := contentBuilder.String()
			if content != "" {
				// 检查是否包含生成的图片、视频或音频
				if strings.Contains(content, "[GENERATED_IMAGE:") || strings.Contains(content, "[GENERATED_VIDEO:") || strings.Contains(content, "[GENERATED_AUDIO:") {
					// 使用 SendWithImages 发送带媒体的消息
					f.SendWithImages(ctx, replyTo, content)
				} else if messageID == "" {
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
	for k, t := range f.processedMsgs {
		if t.Before(cutoff) {
			delete(f.processedMsgs, k)
		}
	}

	_, exists := f.processedMsgs[messageID]
	return exists
}

// markProcessed 标记消息为已处理
func (f *FeishuChannel) markProcessed(messageID string) {
	f.dedupeMu.Lock()
	defer f.dedupeMu.Unlock()
	f.processedMsgs[messageID] = time.Now()
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
	mediaDir := filepath.Join(f.workspace, "media")
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
	mediaDir := filepath.Join(f.workspace, "media")
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

// downloadAudio 下载飞书音频并保存到本地
func (f *FeishuChannel) downloadAudio(ctx context.Context, content *string, messageID string) (string, error) {
	if content == nil || f.client == nil {
		return "", fmt.Errorf("invalid parameters")
	}

	// 解析音频消息内容: {"file_key": "file_xxx", "duration": 5000}
	var audioMsg struct {
		FileKey  string `json:"file_key"`
		Duration int    `json:"duration"`
	}
	if err := json.Unmarshal([]byte(*content), &audioMsg); err != nil {
		return "", fmt.Errorf("parse audio content: %w", err)
	}

	if audioMsg.FileKey == "" {
		return "", fmt.Errorf("empty file_key")
	}

	// 创建媒体目录
	mediaDir := filepath.Join(f.workspace, "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return "", fmt.Errorf("create media dir: %w", err)
	}

	// 生成文件名 (飞书语音通常是 opus 格式)
	ext := ".opus"
	timestamp := time.Now().UnixNano()
	shortID := messageID
	if len(messageID) > 8 {
		shortID = messageID[:8]
	}
	filename := fmt.Sprintf("feishu_audio_%d_%s%s", timestamp, shortID, ext)
	filePath := filepath.Join(mediaDir, filename)

	// 获取音频资源请求
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(audioMsg.FileKey).
		Type("file").
		Build()

	// 获取音频并直接保存到文件
	resp, err := f.client.Im.MessageResource.Get(ctx, req)
	if err != nil {
		return "", fmt.Errorf("get audio resource: %w", err)
	}

	if resp.Code != 0 {
		return "", fmt.Errorf("get audio failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	// 使用 SDK 提供的 WriteFile 方法保存文件
	if err := resp.WriteFile(filePath); err != nil {
		return "", fmt.Errorf("write audio file: %w", err)
	}

	return filePath, nil
}

// downloadMedia 下载飞书媒体消息（视频/图片混合）并保存到本地
func (f *FeishuChannel) downloadMedia(ctx context.Context, content *string, messageID string) (string, string, error) {
	if content == nil || f.client == nil {
		return "", "", fmt.Errorf("invalid parameters")
	}

	// 解析媒体消息内容: {"file_key": "file_xxx", "file_name": "xxx.MOV", "image_key": "img_xxx", "duration": 17000}
	var mediaMsg struct {
		FileKey  string `json:"file_key"`
		FileName string `json:"file_name"`
		ImageKey string `json:"image_key"`
		Duration int    `json:"duration"`
	}
	if err := json.Unmarshal([]byte(*content), &mediaMsg); err != nil {
		return "", "", fmt.Errorf("parse media content: %w", err)
	}

	// 创建媒体目录
	mediaDir := filepath.Join(f.workspace, "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return "", "", fmt.Errorf("create media dir: %w", err)
	}

	// 判断媒体类型和确定扩展名
	var fileKey string
	var mediaType string
	var ext string

	if mediaMsg.FileKey != "" {
		// 有文件键，说明是视频或文件
		fileKey = mediaMsg.FileKey
		mediaType = "video"
		// 从文件名获取扩展名
		if mediaMsg.FileName != "" {
			if idx := strings.LastIndex(mediaMsg.FileName, "."); idx != -1 {
				ext = mediaMsg.FileName[idx:]
			}
		}
		if ext == "" {
			ext = ".mp4" // 默认视频扩展名
		}
	} else if mediaMsg.ImageKey != "" {
		// 只有图片键
		fileKey = mediaMsg.ImageKey
		mediaType = "image"
		ext = ".jpg"
	} else {
		return "", "", fmt.Errorf("no file_key or image_key in media message")
	}

	// 生成文件名
	timestamp := time.Now().UnixNano()
	shortID := messageID
	if len(messageID) > 8 {
		shortID = messageID[:8]
	}
	filename := fmt.Sprintf("feishu_%d_%s%s", timestamp, shortID, ext)
	filePath := filepath.Join(mediaDir, filename)

	// 获取媒体资源请求
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(fileKey).
		Type("file").
		Build()

	// 获取媒体并直接保存到文件
	resp, err := f.client.Im.MessageResource.Get(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("get media resource: %w", err)
	}

	if resp.Code != 0 {
		return "", "", fmt.Errorf("get media failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	// 使用 SDK 提供的 WriteFile 方法保存文件
	if err := resp.WriteFile(filePath); err != nil {
		return "", "", fmt.Errorf("write media file: %w", err)
	}

	return filePath, mediaType, nil
}

// downloadPostMedia 从飞书 post 消息中提取并下载媒体文件
// post 消息格式: {"title":"","content":[[{"tag":"media","file_key":"xxx","image_key":"yyy"}]]}
func (f *FeishuChannel) downloadPostMedia(ctx context.Context, content *string, messageID string) (string, string, error) {
	if content == nil || f.client == nil {
		return "", "", nil
	}

	// 解析 post 消息内容
	var postMsg struct {
		Title   string `json:"title"`
		Content [][]struct {
			Tag      string `json:"tag"`
			FileKey  string `json:"file_key"`
			ImageKey string `json:"image_key"`
			Text     string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(*content), &postMsg); err != nil {
		return "", "", nil // 不是有效的 post 格式，返回空
	}

	// 查找第一个媒体元素
	var fileKey, imageKey string
	var hasMedia bool

	for _, paragraph := range postMsg.Content {
		for _, element := range paragraph {
			if element.Tag == "media" || element.Tag == "img" {
				fileKey = element.FileKey
				imageKey = element.ImageKey
				hasMedia = true
				break
			}
		}
		if hasMedia {
			break
		}
	}

	if !hasMedia || (fileKey == "" && imageKey == "") {
		return "", "", nil // 没有媒体
	}

	// 创建媒体目录
	mediaDir := filepath.Join(f.workspace, "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return "", "", fmt.Errorf("create media dir: %w", err)
	}

	// 判断媒体类型和确定扩展名
	var actualKey string
	var mediaType string
	var ext string

	if fileKey != "" {
		// 有文件键，说明是视频
		actualKey = fileKey
		mediaType = "video"
		ext = ".mp4"
	} else if imageKey != "" {
		// 只有图片键
		actualKey = imageKey
		mediaType = "image"
		ext = ".jpg"
	}

	// 生成文件名
	timestamp := time.Now().UnixNano()
	shortID := messageID
	if len(messageID) > 8 {
		shortID = messageID[:8]
	}
	filename := fmt.Sprintf("feishu_%d_%s%s", timestamp, shortID, ext)
	filePath := filepath.Join(mediaDir, filename)

	// 获取媒体资源请求
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(actualKey).
		Type("file").
		Build()

	// 获取媒体并直接保存到文件
	resp, err := f.client.Im.MessageResource.Get(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("get post media resource: %w", err)
	}

	if resp.Code != 0 {
		return "", "", fmt.Errorf("get post media failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	// 使用 SDK 提供的 WriteFile 方法保存文件
	if err := resp.WriteFile(filePath); err != nil {
		return "", "", fmt.Errorf("write post media file: %w", err)
	}

	logger.Info("Downloaded media for multimodal processing", "type", mediaType, "path", filePath, "messageId", messageID)
	return filePath, mediaType, nil
}

// MediaInfo 媒体信息
type MediaInfo struct {
	Path string
	Type string // "image" or "video"
}

// downloadPostAllMedia 从飞书 post 消息中提取并下载所有媒体文件
func (f *FeishuChannel) downloadPostAllMedia(ctx context.Context, content *string, messageID string) ([]MediaInfo, error) {
	if content == nil || f.client == nil {
		return nil, nil
	}

	// 解析 post 消息内容
	var postMsg struct {
		Title   string `json:"title"`
		Content [][]struct {
			Tag      string `json:"tag"`
			FileKey  string `json:"file_key"`
			ImageKey string `json:"image_key"`
			Text     string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(*content), &postMsg); err != nil {
		return nil, nil // 不是有效的 post 格式，返回空
	}

	// 收集所有媒体元素
	type mediaItem struct {
		fileKey  string
		imageKey string
	}
	var mediaItems []mediaItem

	for _, paragraph := range postMsg.Content {
		for _, element := range paragraph {
			if element.Tag == "media" || element.Tag == "img" {
				mediaItems = append(mediaItems, mediaItem{
					fileKey:  element.FileKey,
					imageKey: element.ImageKey,
				})
			}
		}
	}

	if len(mediaItems) == 0 {
		return nil, nil // 没有媒体
	}

	// 创建媒体目录
	mediaDir := filepath.Join(f.workspace, "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return nil, fmt.Errorf("create media dir: %w", err)
	}

	var results []MediaInfo
	timestamp := time.Now().UnixNano()
	shortID := messageID
	if len(messageID) > 8 {
		shortID = messageID[:8]
	}

	for i, item := range mediaItems {
		// 判断媒体类型和确定扩展名
		var actualKey string
		var mediaType string
		var ext string

		if item.fileKey != "" {
			actualKey = item.fileKey
			mediaType = "video"
			ext = ".mp4"
		} else if item.imageKey != "" {
			actualKey = item.imageKey
			mediaType = "image"
			ext = ".jpg"
		} else {
			continue
		}

		// 生成文件名（带序号）
		filename := fmt.Sprintf("feishu_%d_%s_%d%s", timestamp, shortID, i, ext)
		filePath := filepath.Join(mediaDir, filename)

		// 获取媒体资源请求
		req := larkim.NewGetMessageResourceReqBuilder().
			MessageId(messageID).
			FileKey(actualKey).
			Type("file").
			Build()

		// 获取媒体并直接保存到文件
		resp, err := f.client.Im.MessageResource.Get(ctx, req)
		if err != nil {
			logger.Warn("Failed to download post media", "index", i, "error", err)
			continue
		}

		if resp.Code != 0 {
			logger.Warn("Post media download failed", "index", i, "code", resp.Code, "msg", resp.Msg)
			continue
		}

		// 使用 SDK 提供的 WriteFile 方法保存文件
		if err := resp.WriteFile(filePath); err != nil {
			logger.Warn("Failed to write post media file", "index", i, "error", err)
			continue
		}

		results = append(results, MediaInfo{Path: filePath, Type: mediaType})
		logger.Info("Downloaded media for multimodal processing", "index", i, "type", mediaType, "path", filePath, "messageId", messageID)
	}

	return results, nil
}

// parsePostContent 从飞书 post 消息中提取纯文本内容
func (f *FeishuChannel) parsePostContent(content *string) string {
	if content == nil {
		return ""
	}

	// 解析 post 消息内容
	var postMsg struct {
		Title   string `json:"title"`
		Content [][]struct {
			Tag  string `json:"tag"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(*content), &postMsg); err != nil {
		return "[post]"
	}

	// 提取所有文本
	var texts []string
	if postMsg.Title != "" {
		texts = append(texts, postMsg.Title)
	}
	for _, paragraph := range postMsg.Content {
		for _, element := range paragraph {
			if element.Tag == "text" && element.Text != "" {
				texts = append(texts, element.Text)
			}
		}
	}

	if len(texts) == 0 {
		return "[post]"
	}
	return strings.Join(texts, "\n")
}

// downloadImageFromBase64 从 base64 数据保存图片（用于发送）
func (f *FeishuChannel) downloadImageFromBase64(base64Data, mimeType, messageID string) (string, error) {
	// 创建媒体目录
	mediaDir := filepath.Join(f.workspace, "media")
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

// uploadImage 上传图片到飞书并返回 image_key
func (f *FeishuChannel) uploadImage(ctx context.Context, imagePath string) (string, error) {
	// 读取图片文件
	fileData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image file: %w", err)
	}

	// 上传图片到飞书
	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType("message").
			Image(bytes.NewReader(fileData)).
			Build()).
		Build()

	resp, err := f.client.Im.Image.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("upload image: %w", err)
	}

	if resp.Code != 0 {
		return "", fmt.Errorf("upload image failed: %s", resp.Msg)
	}

	if resp.Data == nil || resp.Data.ImageKey == nil {
		return "", fmt.Errorf("upload image: no image_key in response")
	}

	return *resp.Data.ImageKey, nil
}

// sendImageMessage 发送图片消息
func (f *FeishuChannel) sendImageMessage(ctx context.Context, receiveID, imageKey string) error {
	if receiveID == "" {
		return fmt.Errorf("receive_id is empty")
	}

	// Determine receive_id_type based on ID format
	receiveIDType := larkim.ReceiveIdTypeOpenId
	if strings.HasPrefix(receiveID, "oc_") {
		receiveIDType = larkim.ReceiveIdTypeChatId
	}

	// Build image message content
	imageContent := map[string]any{
		"image_key": imageKey,
	}
	contentJSON, err := json.Marshal(imageContent)
	if err != nil {
		return fmt.Errorf("marshal image content: %w", err)
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType(larkim.MsgTypeImage).
			Content(string(contentJSON)).
			Build()).
		Build()

	resp, err := f.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("send image message: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("send image failed: %s", resp.Msg)
	}

	logger.Debug("Image sent successfully", "to", receiveID, "messageID", safeString(resp.Data.MessageId))
	return nil
}

// SendWithImages 发送消息（支持图片、视频、音频）
// 检测内容中的 [GENERATED_IMAGE:路径]、[GENERATED_VIDEO:路径]、[GENERATED_AUDIO:路径] 标记，上传并发送
func (f *FeishuChannel) SendWithImages(ctx context.Context, receiveID, content string) error {
	// 正则匹配 [GENERATED_IMAGE:路径]
	imagePattern := regexp.MustCompile(`\[GENERATED_IMAGE:([^\]]+)\]`)
	imageMatches := imagePattern.FindAllStringSubmatch(content, -1)

	// 正则匹配 [GENERATED_VIDEO:路径]
	videoPattern := regexp.MustCompile(`\[GENERATED_VIDEO:([^\]]+)\]`)
	videoMatches := videoPattern.FindAllStringSubmatch(content, -1)

	// 正则匹配 [GENERATED_AUDIO:路径]
	audioPattern := regexp.MustCompile(`\[GENERATED_AUDIO:([^\]]+)\]`)
	audioMatches := audioPattern.FindAllStringSubmatch(content, -1)

	// 先发送文本内容（移除所有媒体标记）
	textContent := imagePattern.ReplaceAllString(content, "")
	textContent = videoPattern.ReplaceAllString(textContent, "")
	textContent = audioPattern.ReplaceAllString(textContent, "")

	// 发送文本消息
	if strings.TrimSpace(textContent) != "" {
		if err := f.sendReply(ctx, receiveID, textContent); err != nil {
			logger.Warn("Failed to send text message", "error", err)
		}
	}

	// 发送图片
	for _, match := range imageMatches {
		if len(match) > 1 {
			imagePath := match[1]
			logger.Info("Uploading generated image", "path", imagePath)

			imageKey, err := f.uploadImage(ctx, imagePath)
			if err != nil {
				logger.Warn("Failed to upload image", "path", imagePath, "error", err)
				continue
			}

			if err := f.sendImageMessage(ctx, receiveID, imageKey); err != nil {
				logger.Warn("Failed to send image message", "error", err)
			}
		}
	}

	// 发送视频
	for _, match := range videoMatches {
		if len(match) > 1 {
			videoPath := match[1]
			logger.Info("Uploading generated video", "path", videoPath)

			fileKey, err := f.uploadFile(ctx, videoPath)
			if err != nil {
				logger.Warn("Failed to upload video", "path", videoPath, "error", err)
				continue
			}

			if err := f.sendFileMessage(ctx, receiveID, fileKey); err != nil {
				logger.Warn("Failed to send video message", "error", err)
			}
		}
	}

	// 发送音频
	for _, match := range audioMatches {
		if len(match) > 1 {
			audioPath := match[1]
			logger.Info("Uploading generated audio", "path", audioPath)

			fileKey, err := f.uploadFile(ctx, audioPath)
			if err != nil {
				logger.Warn("Failed to upload audio", "path", audioPath, "error", err)
				continue
			}

			if err := f.sendFileMessage(ctx, receiveID, fileKey); err != nil {
				logger.Warn("Failed to send audio message", "error", err)
			}
		}
	}

	return nil
}

// uploadFile 上传文件到飞书
func (f *FeishuChannel) uploadFile(ctx context.Context, filePath string) (string, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	// 获取文件名
	fileName := filepath.Base(filePath)

	req := larkim.NewCreateFileReqBuilder().
		Body(larkim.NewCreateFileReqBodyBuilder().
			FileType("stream").
			FileName(fileName).
			File(bytes.NewReader(fileData)).
			Build()).
		Build()

	resp, err := f.client.Im.File.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("upload file: %w", err)
	}

	if resp.Code != 0 {
		return "", fmt.Errorf("upload file failed: %s", resp.Msg)
	}

	if resp.Data == nil || resp.Data.FileKey == nil {
		return "", fmt.Errorf("upload file: no file_key in response")
	}

	return *resp.Data.FileKey, nil
}

// sendFileMessage 发送文件消息
func (f *FeishuChannel) sendFileMessage(ctx context.Context, receiveID, fileKey string) error {
	if receiveID == "" {
		return fmt.Errorf("receive_id is empty")
	}

	// Determine receive_id_type based on ID format
	receiveIDType := larkim.ReceiveIdTypeOpenId
	if strings.HasPrefix(receiveID, "oc_") {
		receiveIDType = larkim.ReceiveIdTypeChatId
	}

	// Build file message content
	fileContent := map[string]any{
		"file_key": fileKey,
	}
	contentJSON, err := json.Marshal(fileContent)
	if err != nil {
		return fmt.Errorf("marshal file content: %w", err)
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType(larkim.MsgTypeFile).
			Content(string(contentJSON)).
			Build()).
		Build()

	resp, err := f.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("send file message: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("send file failed: %s", resp.Msg)
	}

	logger.Debug("File sent successfully", "to", receiveID, "messageID", safeString(resp.Data.MessageId))
	return nil
}

// Compile-time check for unused code (regexp for markdown table parsing if needed later)
var _ = regexp.MustCompile(``)

// sendSoulGuideMessage 发送 Soul 引导消息
func (f *FeishuChannel) sendSoulGuideMessage(ctx context.Context, replyTo, userID string) {
	guideMsg := f.getSoulGuideMessage()
	if err := f.sendReply(ctx, replyTo, guideMsg); err != nil {
		logger.Warn("Failed to send Soul guide message", "error", err)
		return
	}
	// 标记用户正在等待定义 Soul
	f.setPendingSoulDefinition(userID)
	logger.Info("Soul guide message sent", "userId", userID)
}

// getSoulGuideMessage 获取 Soul 引导消息
func (f *FeishuChannel) getSoulGuideMessage() string {
	if f.soulConfig != nil && f.soulConfig.GuideMessage != "" {
		return f.soulConfig.GuideMessage
	}
	return `你好！我是你的 AI 助手。

在开始之前，我想了解一下你希望我成为什么样的助手。

你可以告诉我：
- 我的性格特点（如：温柔、幽默、专业...）
- 我应该如何称呼你
- 你希望我的回复风格（简洁/详细、活泼/稳重...）
- 其他任何你期望的特质

请直接回复你的想法，或者回复"跳过"使用默认设置。`
}

// setPendingSoulDefinition 设置用户正在等待定义 Soul
func (f *FeishuChannel) setPendingSoulDefinition(userID string) {
	f.soulPendingMu.Lock()
	defer f.soulPendingMu.Unlock()
	f.soulPendingUsers[userID] = time.Now()
}

// isPendingSoulDefinition 检查用户是否正在等待定义 Soul
func (f *FeishuChannel) isPendingSoulDefinition(userID string) bool {
	f.soulPendingMu.RLock()
	defer f.soulPendingMu.RUnlock()
	_, exists := f.soulPendingUsers[userID]
	return exists
}

// clearPendingSoulDefinition 清除用户的 Soul 定义等待状态
func (f *FeishuChannel) clearPendingSoulDefinition(userID string) {
	f.soulPendingMu.Lock()
	defer f.soulPendingMu.Unlock()
	delete(f.soulPendingUsers, userID)
}

// soulQuestionPatterns "你是谁?"相关问题的匹配模式
var soulQuestionPatterns = []string{
	"你是谁",
	"你叫什么",
	"你的名字",
	"介绍一下你自己",
	"自我介绍",
	"who are you",
	"what is your name",
	"tell me about yourself",
}

// isSoulQuestion 检测消息是否为"你是谁?"相关问题
func isSoulQuestion(content string) bool {
	content = strings.ToLower(content)
	for _, pattern := range soulQuestionPatterns {
		if strings.Contains(content, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}
