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
	mu      sync.RWMutex
	sender  MessageSender
	channel string
	chatID  string
}

func NewMessageTool(sender MessageSender) *MessageTool {
	return &MessageTool{sender: sender}
}

func (t *MessageTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channel = channel
	t.chatID = chatID
}

func (t *MessageTool) Name() string { return "message" }

func (t *MessageTool) Description() string {
	return `发送消息通知给用户（仅用于长时间任务的进度通知）`
}

func (t *MessageTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "通知内容",
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
	channel, chatID := t.channel, t.chatID
	t.mu.RUnlock()

	if channel == "" || chatID == "" {
		return "skipped: no channel context", nil
	}
	if t.sender == nil {
		return "skipped: no sender", nil
	}
	if err := t.sender.SendMessage(channel, chatID, p.Content); err != nil {
		return "", err
	}
	return "ok", nil
}

func (t *MessageTool) IsDangerous() bool         { return false }
func (t *MessageTool) ShouldLoadByDefault() bool { return true }
