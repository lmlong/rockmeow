// Package memory 记忆系统 - 上下文构建器
package memory

import (
	"fmt"
	"strings"
	"time"
)

// ContextBuilder 上下文构建器（参考 nanobot）
type ContextBuilder struct {
	store *FileStore
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(store *FileStore) *ContextBuilder {
	return &ContextBuilder{store: store}
}

// BuildContext 构建记忆上下文
// 返回包含长期记忆和最近历史的上下文字符串
func (b *ContextBuilder) BuildContext(includeRecentDays int) (string, error) {
	var context strings.Builder

	// 1. 加载长期记忆（MEMORY.md）
	memory, err := b.store.GetMemory()
	if err != nil {
		return "", fmt.Errorf("load memory: %w", err)
	}

	// 过滤掉注释和空行，只保留有价值的内容
	cleanMemory := b.cleanMemoryContent(memory)
	if cleanMemory != "" {
		context.WriteString("## Long-term Memory\n\n")
		context.WriteString(cleanMemory)
		context.WriteString("\n\n")
	}

	// 2. 加载最近的每日日志
	if includeRecentDays > 0 {
		dailyLogs, err := b.store.GetRecentDailyLogs(includeRecentDays)
		if err == nil && len(dailyLogs) > 0 {
			context.WriteString("## Recent Activity\n\n")
			// 按日期倒序
			for i := 0; i < includeRecentDays; i++ {
				date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
				if log, ok := dailyLogs[date]; ok {
					context.WriteString(fmt.Sprintf("### %s\n", date))
					context.WriteString(log)
					context.WriteString("\n")
				}
			}
		}
	}

	// 3. 加载最近的历史记录
	recentHistory, err := b.store.GetRecentHistory(50)
	if err == nil && len(recentHistory) > 0 {
		context.WriteString("## Recent History\n\n")
		// 只保留最近的几个事件
		start := len(recentHistory) - 20
		if start < 0 {
			start = 0
		}
		for _, line := range recentHistory[start:] {
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
				context.WriteString(line + "\n")
			}
		}
	}

	return context.String(), nil
}

// BuildContextWithQuery 基于查询构建相关上下文
// 使用 grep 搜索相关记忆
func (b *ContextBuilder) BuildContextWithQuery(query string, includeRecentDays int) (string, error) {
	var context strings.Builder

	// 首先获取基础上下文
	baseContext, err := b.BuildContext(includeRecentDays)
	if err != nil {
		return "", err
	}

	// 搜索相关记忆
	searchResults, err := b.store.SearchAll(query)
	if err == nil && len(searchResults) > 0 {
		context.WriteString("## Relevant Memories\n\n")
		for file, lines := range searchResults {
			context.WriteString(fmt.Sprintf("### From %s\n", file))
			for _, line := range lines {
				// 限制行长度
				if len(line) > 200 {
					line = line[:200] + "..."
				}
				context.WriteString(line + "\n")
			}
			context.WriteString("\n")
		}
	}

	context.WriteString(baseContext)
	return context.String(), nil
}

// cleanMemoryContent 清理记忆内容，移除注释和格式化
func (b *ContextBuilder) cleanMemoryContent(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 跳过空行
		if trimmed == "" {
			continue
		}

		// 跳过 HTML 注释
		if strings.HasPrefix(trimmed, "<!--") || strings.HasSuffix(trimmed, "-->") {
			continue
		}

		// 保留标题（降低一级）
		if strings.HasPrefix(trimmed, "# ") {
			result.WriteString("###" + trimmed[1:] + "\n")
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			result.WriteString("####" + trimmed[2:] + "\n")
			continue
		}

		// 保留列表项和内容
		result.WriteString(line + "\n")
	}

	return result.String()
}

// MemoryTools 记忆操作工具（可供 Agent 调用）

// MemoryTools 记忆工具集合
type MemoryTools struct {
	store *FileStore
}

// NewMemoryTools 创建记忆工具
func NewMemoryTools(store *FileStore) *MemoryTools {
	return &MemoryTools{store: store}
}

// Remember 记录长期记忆
func (t *MemoryTools) Remember(category, fact string) error {
	return t.store.AddMemory(category, fact)
}

// Recall 回忆（搜索记忆）
func (t *MemoryTools) Recall(query string) (map[string][]string, error) {
	return t.store.SearchAll(query)
}

// LogEvent 记录事件到每日日志
func (t *MemoryTools) LogEvent(event string) error {
	return t.store.WriteDailyLog(event)
}

// GetContext 获取当前上下文
func (t *MemoryTools) GetContext() (string, error) {
	builder := NewContextBuilder(t.store)
	return builder.BuildContext(3) // 最近3天
}
