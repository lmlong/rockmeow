package subagent

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/lingguard/internal/providers"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/logger"
)

// SubagentManager 子代理管理器
type SubagentManager struct {
	provider     providers.Provider
	toolRegistry *tools.Registry
	config       *SubagentConfig

	mu    sync.RWMutex
	tasks map[string]*Subagent

	// 结果通知通道
	notify chan *Subagent

	// 清理相关
	ctx       context.Context
	cancel    context.CancelFunc
	cleanupTk *time.Ticker
}

// NewSubagentManager 创建子代理管理器
func NewSubagentManager(provider providers.Provider, toolRegistry *tools.Registry, config *SubagentConfig) *SubagentManager {
	if config == nil {
		config = DefaultSubagentConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &SubagentManager{
		provider:     provider,
		toolRegistry: toolRegistry,
		config:       config,
		tasks:        make(map[string]*Subagent),
		notify:       make(chan *Subagent, 100), // 带缓冲的通道
		ctx:          ctx,
		cancel:       cancel,
		cleanupTk:    time.NewTicker(5 * time.Minute), // 每5分钟清理一次
	}

	// 启动自动清理 goroutine
	go m.cleanupLoop()

	return m
}

// Spawn 创建并启动后台子代理
func (m *SubagentManager) Spawn(ctx context.Context, task, context string) (*Subagent, error) {
	// 创建子代理的工具注册表（白名单过滤）
	subToolRegistry := m.createFilteredRegistry()

	// 创建子代理
	sub := NewSubagent(task, context, m.provider, subToolRegistry, m.config)

	// 注册任务
	m.mu.Lock()
	m.tasks[sub.ID()] = sub
	m.mu.Unlock()

	// 在 goroutine 中执行（添加 panic 恢复）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Subagent goroutine panic recovered", "error", r, "stack", string(debug.Stack()))
			}
		}()
		sub.Run(ctx)

		// 任务完成后发送通知
		select {
		case m.notify <- sub:
			// 通知已发送
		default:
			// 通道满，丢弃通知
			logger.Warn("Subagent notify channel full, dropping notification", "subagentID", sub.ID())
		}
	}()

	return sub, nil
}

// cleanupLoop 定期清理已完成/失败的任务
func (m *SubagentManager) cleanupLoop() {
	for {
		select {
		case <-m.ctx.Done():
			if m.cleanupTk != nil {
				m.cleanupTk.Stop()
			}
			return
		case <-m.cleanupTk.C:
			cleared := m.Clear()
			if cleared > 0 {
				logger.Debug("SubagentManager auto cleanup", "cleared", cleared)
			}
		}
	}
}

// Close 关闭管理器，停止清理 goroutine
func (m *SubagentManager) Close() {
	if m.cancel != nil {
		m.cancel()
	}
}

// createFilteredRegistry 创建过滤后的工具注册表
func (m *SubagentManager) createFilteredRegistry() *tools.Registry {
	filtered := tools.NewRegistry()

	// 获取允许的工具列表
	allowedTools := m.config.EnabledTools
	if allowedTools == nil {
		// nil 表示允许所有工具（排除 task 和 task_status 以防止无限嵌套）
		blockedTools := map[string]bool{
			"task":        true,
			"task_status": true,
		}
		for _, tool := range m.toolRegistry.List() {
			if !blockedTools[tool.Name()] {
				filtered.Register(tool)
			}
		}
		return filtered
	}

	// 使用白名单过滤
	allowedSet := make(map[string]bool)
	for _, name := range allowedTools {
		allowedSet[name] = true
	}

	for _, tool := range m.toolRegistry.List() {
		if allowedSet[tool.Name()] {
			filtered.Register(tool)
		}
	}

	return filtered
}

// GetStatus 获取任务状态
func (m *SubagentManager) GetStatus(taskID string) (*Subagent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sub, exists := m.tasks[taskID]
	return sub, exists
}

// ListTasks 列出所有任务
func (m *SubagentManager) ListTasks() []*Subagent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Subagent, 0, len(m.tasks))
	for _, sub := range m.tasks {
		tasks = append(tasks, sub)
	}
	return tasks
}

// ListByStatus 按状态列出任务
func (m *SubagentManager) ListByStatus(status TaskStatus) []*Subagent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Subagent, 0)
	for _, sub := range m.tasks {
		if sub.Status() == status {
			tasks = append(tasks, sub)
		}
	}
	return tasks
}

// NotifyChannel 返回结果通知通道
// 调用者可以从此通道接收任务完成通知
func (m *SubagentManager) NotifyChannel() <-chan *Subagent {
	return m.notify
}

// Remove 移除任务记录
func (m *SubagentManager) Remove(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tasks, taskID)
}

// Clear 清理已完成或失败的任务
func (m *SubagentManager) Clear() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, sub := range m.tasks {
		if sub.Status() == StatusCompleted || sub.Status() == StatusFailed {
			delete(m.tasks, id)
			count++
		}
	}
	return count
}

// Count 返回任务总数
func (m *SubagentManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tasks)
}

// CountByStatus 返回指定状态的任务数量
func (m *SubagentManager) CountByStatus(status TaskStatus) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, sub := range m.tasks {
		if sub.Status() == status {
			count++
		}
	}
	return count
}

// TaskSummary 任务摘要信息
type TaskSummary struct {
	ID           string     `json:"id"`
	Task         string     `json:"task"`
	Status       TaskStatus `json:"status"`
	Error        string     `json:"error,omitempty"`
	StartedAt    string     `json:"startedAt,omitempty"`
	CompletedAt  string     `json:"completedAt,omitempty"`
	Duration     string     `json:"duration,omitempty"`
	ResultLength int        `json:"resultLength,omitempty"`
}

// GetSummary 获取任务摘要
func (s *Subagent) GetSummary() TaskSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := TaskSummary{
		ID:     s.id,
		Task:   s.task,
		Status: s.status,
		Error:  s.error,
	}

	if !s.startedAt.IsZero() {
		summary.StartedAt = s.startedAt.Format("2006-01-02 15:04:05")
	}

	if !s.completedAt.IsZero() {
		summary.CompletedAt = s.completedAt.Format("2006-01-02 15:04:05")
		if !s.startedAt.IsZero() {
			summary.Duration = s.completedAt.Sub(s.startedAt).String()
		}
	}

	if s.result != "" {
		summary.ResultLength = len(s.result)
	}

	return summary
}

// ListSummaries 获取所有任务的摘要
func (m *SubagentManager) ListSummaries() []TaskSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summaries := make([]TaskSummary, 0, len(m.tasks))
	for _, sub := range m.tasks {
		summaries = append(summaries, sub.GetSummary())
	}
	return summaries
}

// GetResult 获取任务的完整结果
func (m *SubagentManager) GetResult(taskID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sub, exists := m.tasks[taskID]
	if !exists {
		return "", fmt.Errorf("task not found: %s", taskID)
	}

	if sub.Status() == StatusFailed {
		return "", fmt.Errorf("task failed: %s", sub.Error())
	}

	if sub.Status() != StatusCompleted {
		return "", fmt.Errorf("task not completed yet, current status: %s", sub.Status())
	}

	return sub.Result(), nil
}
