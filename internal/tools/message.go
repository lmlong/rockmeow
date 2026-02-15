package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// MessageSender 消息发送接口
type MessageSender interface {
	SendMessage(channelName string, to string, content string) error
}

// MessageTool 消息发送工具
type MessageTool struct {
	mu     sync.RWMutex
	sender MessageSender
	// 上下文信息（由 Agent 设置）
	channel string
	chatID  string
}

// NewMessageTool 创建消息发送工具
func NewMessageTool(sender MessageSender) *MessageTool {
	return &MessageTool{
		sender: sender,
	}
}

// SetContext 设置消息发送上下文
func (t *MessageTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channel = channel
	t.chatID = chatID
}

// GetContext 获取当前上下文
func (t *MessageTool) GetContext() (channel, chatID string) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.channel, t.chatID
}

func (t *MessageTool) Name() string { return "message" }

func (t *MessageTool) Description() string {
	return "Send a message to the user on the current channel. Use for progress updates or notifications."
}

func (t *MessageTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The message content to send",
			},
		},
		"required": []string{"content"},
	}
}

func (t *MessageTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Content string `json:"content"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Content == "" {
		return "", fmt.Errorf("content is required")
	}

	t.mu.RLock()
	channel := t.channel
	chatID := t.chatID
	t.mu.RUnlock()

	if channel == "" || chatID == "" {
		return "Warning: No channel context set. Message not sent (running in CLI mode?).", nil
	}

	if t.sender == nil {
		return "Warning: Message sender not configured.", nil
	}

	if err := t.sender.SendMessage(channel, chatID, p.Content); err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	return fmt.Sprintf("Message sent to %s:%s", channel, chatID), nil
}

func (t *MessageTool) IsDangerous() bool { return false }
