package subagent

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/llm"
)

// MockTool 模拟工具
type MockTool struct {
	name        string
	description string
	executeFunc func(ctx context.Context, params json.RawMessage) (string, error)
}

func (t *MockTool) Name() string                       { return t.name }
func (t *MockTool) Description() string                { return t.description }
func (t *MockTool) Parameters() map[string]interface{} { return nil }
func (t *MockTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	if t.executeFunc != nil {
		return t.executeFunc(ctx, params)
	}
	return "mock result", nil
}
func (t *MockTool) IsDangerous() bool { return false }

// MockProvider 模拟 LLM 提供商
type MockProvider struct {
	name      string
	model     string
	callCount int
	mu        sync.Mutex
}

func (p *MockProvider) Name() string         { return p.name }
func (p *MockProvider) Model() string        { return p.model }
func (p *MockProvider) SupportsTools() bool  { return true }
func (p *MockProvider) SupportsVision() bool { return false }
func (p *MockProvider) Stream(ctx context.Context, req *llm.Request) (<-chan llm.StreamEvent, error) {
	return nil, nil
}

func (p *MockProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	resp := &llm.Response{
		ID:    "mock-response",
		Model: p.model,
		Choices: []struct {
			Index        int         `json:"index"`
			Message      llm.Message `json:"message"`
			FinishReason string      `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: llm.Message{
					Role:    "assistant",
					Content: "Task completed successfully",
				},
				FinishReason: "stop",
			},
		},
	}

	p.callCount++
	return resp, nil
}

func TestNewSubagentConfig(t *testing.T) {
	cfg := DefaultSubagentConfig()

	if cfg.MaxIterations != 15 {
		t.Errorf("Expected MaxIterations=15, got %d", cfg.MaxIterations)
	}

	if len(cfg.EnabledTools) == 0 {
		t.Error("EnabledTools should not be empty")
	}
}

func TestNewSubagent(t *testing.T) {
	task := "Test task"
	context := "Test context"

	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})

	provider := &MockProvider{name: "mock", model: "mock-model"}
	cfg := DefaultSubagentConfig()

	sub := NewSubagent(task, context, provider, registry, cfg)

	if sub.ID() == "" {
		t.Error("ID should not be empty")
	}

	if sub.Task() != task {
		t.Errorf("Expected Task=%s, got %s", task, sub.Task())
	}

	if sub.Status() != StatusPending {
		t.Errorf("Expected Status=%s, got %s", StatusPending, sub.Status())
	}
}

func TestSubagentStatus(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	sub := NewSubagent("Test task", "", provider, registry, nil)

	// 初始状态应该是 pending
	if sub.Status() != StatusPending {
		t.Errorf("Expected initial status to be pending, got %s", sub.Status())
	}

	// 测试状态变更
	sub.setStatus(StatusRunning)
	if sub.Status() != StatusRunning {
		t.Errorf("Expected status to be running, got %s", sub.Status())
	}

	sub.setResult("test result")
	if sub.Result() != "test result" {
		t.Errorf("Expected result 'test result', got %s", sub.Result())
	}
}

func TestSubagentManagerSpawn(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})
	registry.Register(&MockTool{name: "task", description: "Should be filtered out"})
	registry.Register(&MockTool{name: "task_status", description: "Should be filtered out"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	cfg := &SubagentConfig{
		MaxIterations: 5,
		EnabledTools:  []string{"shell"}, // 只允许 shell，过滤掉 task
	}

	manager := NewSubagentManager(provider, registry, cfg)

	ctx := context.Background()
	sub, err := manager.Spawn(ctx, "Test task", "Test context")

	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if sub == nil {
		t.Fatal("Subagent should not be nil")
	}

	if sub.ID() == "" {
		t.Error("Subagent ID should not be empty")
	}

	// 验证任务已注册
	_, exists := manager.GetStatus(sub.ID())
	if !exists {
		t.Error("Task should be registered in manager")
	}

	// 验证任务数量
	if manager.Count() != 1 {
		t.Errorf("Expected 1 task, got %d", manager.Count())
	}
}

func TestSubagentManagerGetStatus(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	manager := NewSubagentManager(provider, registry, nil)

	ctx := context.Background()
	sub, _ := manager.Spawn(ctx, "Test task", "")

	// 等待一小段时间让任务开始
	time.Sleep(10 * time.Millisecond)

	// 查询任务
	found, exists := manager.GetStatus(sub.ID())
	if !exists {
		t.Fatal("Task should exist")
	}

	if found.ID() != sub.ID() {
		t.Errorf("Expected ID=%s, got %s", sub.ID(), found.ID())
	}
}

func TestSubagentManagerListTasks(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	manager := NewSubagentManager(provider, registry, nil)

	ctx := context.Background()

	// 创建多个任务
	sub1, _ := manager.Spawn(ctx, "Task 1", "")
	sub2, _ := manager.Spawn(ctx, "Task 2", "")

	tasks := manager.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(tasks))
	}

	// 验证任务 ID 存在
	taskIDs := make(map[string]bool)
	for _, task := range tasks {
		taskIDs[task.ID()] = true
	}

	if !taskIDs[sub1.ID()] || !taskIDs[sub2.ID()] {
		t.Error("Both tasks should be in the list")
	}
}

func TestSubagentManagerRemove(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	manager := NewSubagentManager(provider, registry, nil)

	ctx := context.Background()
	sub, _ := manager.Spawn(ctx, "Test task", "")

	if manager.Count() != 1 {
		t.Errorf("Expected 1 task before remove, got %d", manager.Count())
	}

	manager.Remove(sub.ID())

	if manager.Count() != 0 {
		t.Errorf("Expected 0 tasks after remove, got %d", manager.Count())
	}

	_, exists := manager.GetStatus(sub.ID())
	if exists {
		t.Error("Task should not exist after remove")
	}
}

func TestSubagentManagerNotifyChannel(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	manager := NewSubagentManager(provider, registry, nil)

	// 验证通知通道存在
	notifyCh := manager.NotifyChannel()
	if notifyCh == nil {
		t.Fatal("NotifyChannel should not be nil")
	}
}

func TestTaskSummary(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	sub := NewSubagent("Test task", "Test context", provider, registry, nil)

	summary := sub.GetSummary()

	if summary.ID != sub.ID() {
		t.Errorf("Summary ID mismatch")
	}

	if summary.Task != "Test task" {
		t.Errorf("Summary Task mismatch")
	}

	if summary.Status != StatusPending {
		t.Errorf("Summary Status mismatch")
	}
}

func TestFilteredRegistry(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})
	registry.Register(&MockTool{name: "read", description: "Mock read"})
	registry.Register(&MockTool{name: "task", description: "Should be filtered"})
	registry.Register(&MockTool{name: "task_status", description: "Should be filtered"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	cfg := &SubagentConfig{
		MaxIterations: 5,
		EnabledTools:  []string{"shell", "read"}, // 白名单
	}

	manager := NewSubagentManager(provider, registry, cfg)

	ctx := context.Background()
	sub, _ := manager.Spawn(ctx, "Test task", "")

	// 验证子代理被创建
	if sub == nil {
		t.Fatal("Subagent should not be nil")
	}

	// 注意：这里我们无法直接访问子代理的工具注册表
	// 但可以通过 GetToolDefinitions 来间接验证
	// 在实际测试中可能需要更多的集成测试
}

func TestDefaultEnabledTools(t *testing.T) {
	tools := DefaultEnabledTools()

	// 验证默认工具列表不包含 task 和 task_status
	for _, tool := range tools {
		if tool == "task" || tool == "task_status" {
			t.Errorf("Default tools should not contain '%s'", tool)
		}
	}

	// 验证包含基本工具
	hasShell := false
	for _, tool := range tools {
		if tool == "shell" {
			hasShell = true
			break
		}
	}

	if !hasShell {
		t.Error("Default tools should contain 'shell'")
	}
}

func TestManagerListSummaries(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	manager := NewSubagentManager(provider, registry, nil)

	ctx := context.Background()
	manager.Spawn(ctx, "Task 1", "")
	manager.Spawn(ctx, "Task 2", "")

	summaries := manager.ListSummaries()

	if len(summaries) != 2 {
		t.Errorf("Expected 2 summaries, got %d", len(summaries))
	}
}

func TestManagerCountByStatus(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	manager := NewSubagentManager(provider, registry, nil)

	ctx := context.Background()
	manager.Spawn(ctx, "Task 1", "")
	manager.Spawn(ctx, "Task 2", "")

	// 所有任务初始状态应该是 pending 或 running
	pendingCount := manager.CountByStatus(StatusPending)
	runningCount := manager.CountByStatus(StatusRunning)

	totalNew := pendingCount + runningCount
	if totalNew != 2 {
		t.Errorf("Expected 2 pending or running tasks, got %d", totalNew)
	}
}

func TestSubagentRun(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	sub := NewSubagent("Test task", "Test context", provider, registry, nil)

	ctx := context.Background()

	// 运行子代理
	sub.Run(ctx)

	// 验证状态变为 completed
	if sub.Status() != StatusCompleted {
		t.Errorf("Expected status to be completed, got %s", sub.Status())
	}

	// 验证有结果
	if sub.Result() == "" {
		t.Error("Result should not be empty after completion")
	}
}

func TestManagerGetResult(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&MockTool{name: "shell", description: "Mock shell"})

	provider := &MockProvider{name: "mock", model: "mock-model"}

	manager := NewSubagentManager(provider, registry, nil)

	// 测试不存在的任务
	_, err := manager.GetResult("non-existent")
	if err == nil {
		t.Error("Should return error for non-existent task")
	}
}
