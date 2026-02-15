package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStore_Init(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 检查 MEMORY.md 是否创建
	memoryFile := filepath.Join(tmpDir, "MEMORY.md")
	if _, err := os.Stat(memoryFile); os.IsNotExist(err) {
		t.Error("MEMORY.md not created")
	}

	// 检查 HISTORY.md 是否创建
	historyFile := filepath.Join(tmpDir, "HISTORY.md")
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		t.Error("HISTORY.md not created")
	}
}

func TestFileStore_AddMemory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 添加记忆
	if err := store.AddMemory("User Preferences", "User prefers Go over Python"); err != nil {
		t.Fatalf("failed to add memory: %v", err)
	}

	// 验证内容
	content, err := store.GetMemory()
	if err != nil {
		t.Fatalf("failed to get memory: %v", err)
	}

	if content == "" {
		t.Error("memory content is empty")
	}
}

func TestFileStore_SearchMemory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 添加记忆
	if err := store.AddMemory("User Preferences", "User prefers dark mode"); err != nil {
		t.Fatalf("failed to add memory: %v", err)
	}

	// 验证文件内容
	content, _ := store.GetMemory()
	t.Logf("Memory content:\n%s", content)

	// 搜索记忆（使用简单的关键词）
	results, err := store.SearchMemory("dark")
	if err != nil {
		t.Fatalf("failed to search memory: %v", err)
	}

	t.Logf("Search results: %v", results)

	if len(results) == 0 {
		t.Error("expected to find 'dark' in memory")
	}
}

func TestFileStore_AddHistory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 添加历史
	if err := store.AddHistory("Test Event", "This is a test", map[string]string{
		"key": "value",
	}); err != nil {
		t.Fatalf("failed to add history: %v", err)
	}

	// 获取最近历史
	history, err := store.GetRecentHistory(10)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}

	if len(history) == 0 {
		t.Error("history is empty")
	}
}

func TestFileStore_DailyLog(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 写入每日日志
	if err := store.WriteDailyLog("Completed important task"); err != nil {
		t.Fatalf("failed to write daily log: %v", err)
	}

	// 获取最近日志
	logs, err := store.GetRecentDailyLogs(1)
	if err != nil {
		t.Fatalf("failed to get daily logs: %v", err)
	}

	if len(logs) == 0 {
		t.Error("daily logs is empty")
	}
}

func TestContextBuilder_BuildContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-memory-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 添加一些记忆
	store.AddMemory("Project Context", "This is a Go project")
	store.WriteDailyLog("Working on memory system")

	builder := NewContextBuilder(store)
	ctx, err := builder.BuildContext(1)
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}

	if ctx == "" {
		t.Error("context is empty")
	}
}
