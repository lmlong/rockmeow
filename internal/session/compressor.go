// Package session 会话压缩
package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/pkg/llm"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/memory"
)

// Compressor 会话压缩器
type Compressor struct {
	provider providers.Provider
	config   *config.SessionCompressConfig
}

// NewCompressor 创建会话压缩器
func NewCompressor(provider providers.Provider, cfg *config.SessionCompressConfig) *Compressor {
	if cfg == nil {
		cfg = &config.SessionCompressConfig{
			Enabled:       true,
			Threshold:     50,
			KeepRecent:    5,
			SummaryMaxLen: 500,
		}
	}
	return &Compressor{
		provider: provider,
		config:   cfg,
	}
}

// ShouldCompress 检查是否需要压缩
func (c *Compressor) ShouldCompress(messageCount int) bool {
	if c.config == nil || !c.config.Enabled {
		return false
	}
	threshold := c.config.Threshold
	if threshold <= 0 {
		threshold = 50
	}
	return messageCount >= threshold
}

// CompressSession 压缩会话消息
// 返回压缩后的消息列表和是否执行了压缩
func (c *Compressor) CompressSession(ctx context.Context, messages []*memory.Message) ([]*memory.Message, bool, error) {
	if !c.ShouldCompress(len(messages)) {
		return messages, false, nil
	}

	keepRecent := c.config.KeepRecent
	if keepRecent <= 0 {
		keepRecent = 5
	}
	if keepRecent >= len(messages) {
		return messages, false, nil
	}

	// 分离：待压缩 vs 保留
	toCompress := messages[:len(messages)-keepRecent]
	toKeep := messages[len(messages)-keepRecent:]

	logger.Info("Compressing session messages",
		"total", len(messages),
		"toCompress", len(toCompress),
		"toKeep", len(toKeep))

	// 生成摘要
	summary, err := c.generateSummary(ctx, toCompress)
	if err != nil {
		return nil, false, fmt.Errorf("generate summary: %w", err)
	}

	// 构建新的消息列表
	newMessages := make([]*memory.Message, 0, 1+len(toKeep))

	// 添加摘要消息（作为 system 消息）
	newMessages = append(newMessages, &memory.Message{
		ID:        generateID(),
		Role:      "system",
		Content:   fmt.Sprintf("[对话摘要] %s", summary),
		Timestamp: time.Now(),
	})

	// 添加保留的原始消息
	newMessages = append(newMessages, toKeep...)

	logger.Info("Session compressed",
		"originalCount", len(messages),
		"newCount", len(newMessages),
		"summaryLen", len(summary))

	return newMessages, true, nil
}

// generateSummary 生成对话摘要
func (c *Compressor) generateSummary(ctx context.Context, messages []*memory.Message) (string, error) {
	// 构建对话文本
	var dialogText strings.Builder
	for _, msg := range messages {
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		} else if msg.Role == "system" {
			role = "系统"
		} else if msg.Role == "tool" {
			role = "工具"
		}
		content := msg.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		dialogText.WriteString(fmt.Sprintf("%s: %s\n", role, content))
	}

	// 默认摘要提示词
	summaryPrompt := c.config.SummaryPrompt
	if summaryPrompt == "" {
		summaryPrompt = `请将以下对话历史压缩为简洁的摘要，要求：
1. 保留关键信息和重要决策
2. 保留用户表达过的偏好和需求
3. 记录已完成和未完成的任务
4. 使用简洁的中文表达
5. 不要超过%d个字符

对话历史：
%s

请直接输出摘要内容，不要包含任何前缀或解释。`
	}

	maxLen := c.config.SummaryMaxLen
	if maxLen <= 0 {
		maxLen = 500
	}

	prompt := fmt.Sprintf(summaryPrompt, maxLen, dialogText.String())

	// 调用 LLM 生成摘要
	req := &llm.Request{
		Model: c.provider.Model(),
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := c.provider.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("LLM complete: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	summary := resp.Choices[0].Message.Content

	// 限制摘要长度
	if len(summary) > maxLen {
		summary = summary[:maxLen] + "..."
	}

	return summary, nil
}

// CompressResult 压缩结果
type CompressResult struct {
	OriginalCount int    // 原始消息数
	NewCount      int    // 压缩后消息数
	Summary       string // 生成的摘要
	Compressed    bool   // 是否执行了压缩
}

// CompressAndReplace 压缩会话并替换消息
// 这个方法会直接修改会话的消息列表
func (m *Manager) CompressAndReplace(sessionKey string, compressor *Compressor) (*CompressResult, error) {
	m.mu.RLock()
	s, ok := m.sessions[sessionKey]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionKey)
	}

	result := &CompressResult{}

	s.messagesMu.RLock()
	messages := make([]*memory.Message, len(s.Messages))
	copy(messages, s.Messages)
	s.messagesMu.RUnlock()

	result.OriginalCount = len(messages)

	newMessages, compressed, err := compressor.CompressSession(context.Background(), messages)
	if err != nil {
		return nil, err
	}

	result.Compressed = compressed
	result.NewCount = len(newMessages)

	if !compressed {
		return result, nil
	}

	// 替换消息
	s.messagesMu.Lock()
	s.Messages = newMessages
	s.UpdatedAt = time.Now()
	s.messagesMu.Unlock()

	// 如果有摘要，提取
	if len(newMessages) > 0 && strings.HasPrefix(newMessages[0].Content, "[对话摘要]") {
		result.Summary = strings.TrimPrefix(newMessages[0].Content, "[对话摘要] ")
	}

	// 持久化压缩后的会话
	if m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 先清除旧数据
		if err := m.store.Clear(ctx, sessionKey); err != nil {
			logger.Warn("Failed to clear session before persisting compressed", "error", err)
		}

		// 写入压缩后的消息
		for _, msg := range newMessages {
			if err := m.store.Add(ctx, sessionKey, msg); err != nil {
				logger.Warn("Failed to persist compressed message", "error", err)
			}
		}
	}

	logger.Info("Session compression completed",
		"session", sessionKey,
		"original", result.OriginalCount,
		"new", result.NewCount)

	return result, nil
}

// ShouldCompressSession 检查会话是否需要压缩
func (m *Manager) ShouldCompressSession(sessionKey string, compressor *Compressor) bool {
	m.mu.RLock()
	s, ok := m.sessions[sessionKey]
	m.mu.RUnlock()

	if !ok {
		return false
	}

	s.messagesMu.RLock()
	count := len(s.Messages)
	s.messagesMu.RUnlock()

	return compressor.ShouldCompress(count)
}
