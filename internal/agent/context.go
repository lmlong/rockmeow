// Package agent 上下文构建
package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lingguard/pkg/llm"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/memory"
)

// buildContext 构建上下文
func (a *Agent) buildContext(sessionID string) ([]llm.Message, error) {
	return a.buildContextWithMedia(sessionID, false)
}

// buildContextWithMedia 构建上下文（支持多模态）
// hasMedia 表示当前消息是否包含媒体，用于决定是否为最后一条用户消息构建多模态内容
func (a *Agent) buildContextWithMedia(sessionID string, hasMedia bool) ([]llm.Message, error) {
	messages := make([]llm.Message, 0)
	historyLen := 0

	// 构建系统提示 - 技能信息放在最前面，确保 LLM 注意到
	var systemPrompt string

	// 首先添加技能上下文（最重要）
	if a.skillsMgr != nil {
		skillsContext := a.skillsMgr.GetSkillsContext()
		if skillsContext != "" {
			systemPrompt = skillsContext
		}
	}

	// 然后添加用户配置的系统提示
	if a.config.SystemPrompt != "" {
		if systemPrompt != "" {
			systemPrompt = systemPrompt + "\n\n" + a.config.SystemPrompt
		} else {
			systemPrompt = a.config.SystemPrompt
		}
	}

	// 注入用户 Soul 定义到系统提示
	if a.profileStore != nil {
		soulDefinition := a.profileStore.GetSoulDefinition(sessionID)
		if soulDefinition != "" {
			soulContext := fmt.Sprintf("\n\n## 助手人格设定\n%s", soulDefinition)
			systemPrompt = systemPrompt + soulContext
		}
	}

	// 添加当前时间信息
	currentTime := time.Now().Format("2006-01-02 15:04:05 Monday")
	timeInfo := fmt.Sprintf("当前时间: %s", currentTime)
	if systemPrompt != "" {
		systemPrompt = systemPrompt + "\n\n" + timeInfo
	} else {
		systemPrompt = timeInfo
	}

	// 添加工作目录信息
	if a.config.Workspace != "" {
		workspaceInfo := fmt.Sprintf("工作目录: %s\n\n重要规则:\n- 所有文件操作都应该相对于工作目录进行\n- git clone 时先 cd %s\n- 下载的代码应该放在工作目录下", a.config.Workspace, a.config.Workspace)
		if systemPrompt != "" {
			systemPrompt = systemPrompt + "\n\n" + workspaceInfo
		} else {
			systemPrompt = workspaceInfo
		}
	}

	// 获取会话历史消息（使用 MemoryWindow）
	s := a.sessions.GetOrCreate(sessionID)
	history := s.GetHistory(a.config.MemoryWindow)
	historyLen = len(history)

	// 自动召回：基于用户消息搜索相关记忆
	if a.config.MemoryConfig != nil && a.config.MemoryConfig.AutoRecall && a.IsVectorSearchEnabled() && historyLen > 0 {
		// 获取最近的用户消息
		var lastUserMessage string
		for i := historyLen - 1; i >= 0; i-- {
			if history[i].Role == "user" {
				lastUserMessage = history[i].Content
				break
			}
		}

		if lastUserMessage != "" {
			topK := a.config.MemoryConfig.AutoRecallTopK
			if topK <= 0 {
				topK = 3
			}
			minScore := a.config.MemoryConfig.AutoRecallMinScore
			if minScore <= 0 {
				minScore = 0.3
			}

			// 搜索相关记忆
			relevantMemories := a.searchRelevantMemories(lastUserMessage, topK, minScore)
			if len(relevantMemories) > 0 {
				memContext := a.formatRelevantMemories(relevantMemories)
				if systemPrompt != "" {
					systemPrompt = systemPrompt + "\n\n" + memContext
				} else {
					systemPrompt = memContext
				}
			}
		}
	}

	// 添加记忆上下文（参考 nanobot）
	if a.memoryBuilder != nil {
		recentDays := 3
		if a.config.MemoryConfig != nil && a.config.MemoryConfig.RecentDays > 0 {
			recentDays = a.config.MemoryConfig.RecentDays
		}
		memContext, err := a.memoryBuilder.BuildContext(recentDays)
		if err == nil && memContext != "" {
			if systemPrompt != "" {
				systemPrompt = systemPrompt + "\n\n" + memContext
			} else {
				systemPrompt = memContext
			}
		}
	}

	// 添加系统提示
	if systemPrompt != "" {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	for i, msg := range history {
		llmMsg := llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// 如果这是最后一条用户消息且有多模态内容，构建多模态消息
		if hasMedia && i == historyLen-1 && msg.Role == "user" && len(msg.Media) > 0 {
			contentParts, err := a.buildMultimodalContent(msg.Content, msg.Media)
			if err != nil {
				logger.Warn("Failed to build multimodal content", "error", err)
			} else {
				llmMsg.ContentParts = contentParts
				llmMsg.Content = "" // 清空 Content，使用 ContentParts
			}
		}

		messages = append(messages, llmMsg)
	}

	return messages, nil
}

// searchRelevantMemories 搜索相关记忆（自动召回）
func (a *Agent) searchRelevantMemories(query string, topK int, minScore float32) []*memory.VectorRecord {
	if a.hybridStore == nil || !a.hybridStore.IsVectorEnabled() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	records, err := a.hybridStore.Search(ctx, query, topK)
	if err != nil {
		logger.Warn("Auto-recall search failed", "query", query, "error", err)
		return nil
	}

	// 过滤低分结果
	var filtered []*memory.VectorRecord
	for _, r := range records {
		if r.Score >= minScore {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) > 0 {
		logger.Info("Auto-recall found relevant memories", "query", query, "count", len(filtered))
	}

	return filtered
}

// formatRelevantMemories 格式化相关记忆为上下文
func (a *Agent) formatRelevantMemories(records []*memory.VectorRecord) string {
	if len(records) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 相关记忆 (自动召回)\n\n")
	sb.WriteString("以下是与当前对话相关的历史记忆：\n\n")

	for _, r := range records {
		// 限制内容长度
		content := r.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("- %s\n", content))
	}

	return sb.String()
}

// captureMemories 自动捕获记忆
func (a *Agent) captureMemories(sessionID string, messages []llm.Message) {
	if a.hybridStore == nil && a.memoryStore == nil {
		logger.Debug("Auto-capture skipped: no memory store")
		return
	}

	logger.Debug("Auto-capture analyzing messages", "session", sessionID, "messageCount", len(messages))

	maxChars := 500
	if a.config.MemoryConfig != nil && a.config.MemoryConfig.CaptureMaxChars > 0 {
		maxChars = a.config.MemoryConfig.CaptureMaxChars
	}

	capturedCount := 0
	// 分析用户消息
	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}

		result := memory.AnalyzeForCapture(msg.Content, maxChars)
		if !result.Captured {
			logger.Debug("Auto-capture skipped message", "reason", "no trigger match", "content", msg.Content[:min(30, len(msg.Content))])
			continue
		}

		// 存储记忆
		category := string(result.Category)
		if a.hybridStore != nil {
			if err := a.hybridStore.AddMemory(category, result.Content); err != nil {
				logger.Warn("Auto-capture failed", "error", err)
			} else {
				logger.Info("Auto-captured memory", "category", category, "content", result.Content[:min(50, len(result.Content))])
				capturedCount++
			}
		} else if a.memoryStore != nil {
			if err := a.memoryStore.AddMemory(category, result.Content); err != nil {
				logger.Warn("Auto-capture failed", "error", err)
			} else {
				logger.Info("Auto-captured memory", "category", category, "content", result.Content[:min(50, len(result.Content))])
				capturedCount++
			}
		}
	}

	if capturedCount == 0 {
		logger.Debug("Auto-capture completed: no memories captured")
	}

	// 检查会话压缩
	a.checkSessionCompress(sessionID)
}

// checkSessionCompress 检查并执行会话压缩
func (a *Agent) checkSessionCompress(sessionID string) {
	if a.sessionCompressor == nil {
		return
	}

	cfg := a.config.SessionCompress
	if cfg == nil || !cfg.Enabled {
		return
	}

	// 检查是否需要压缩
	if !a.sessions.ShouldCompressSession(sessionID, a.sessionCompressor) {
		return
	}

	logger.Info("Session compression triggered", "session", sessionID)

	result, err := a.sessions.CompressAndReplace(sessionID, a.sessionCompressor)
	if err != nil {
		logger.Warn("Session compression failed", "error", err)
		return
	}

	if result.Compressed {
		logger.Info("Session compressed",
			"session", sessionID,
			"original", result.OriginalCount,
			"new", result.NewCount,
			"summaryLen", len(result.Summary))
	}
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
