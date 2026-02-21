package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func newTestWorkspaceMgr(t *testing.T, path string) *WorkspaceManager {
	if path == "" {
		tmpDir, err := os.MkdirTemp("", "lingguard-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		t.Cleanup(func() { os.RemoveAll(tmpDir) })
		path = tmpDir
	}
	return NewWorkspaceManager(path, "")
}

func TestShellToolBasic(t *testing.T) {
	mgr := newTestWorkspaceMgr(t, "")
	tool := NewShellTool(mgr, false)

	if tool.Name() != "shell" {
		t.Errorf("Expected name=shell, got %s", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	if !tool.IsDangerous() {
		t.Error("Shell tool should be dangerous")
	}

	params := tool.Parameters()
	if params["type"] != "object" {
		t.Error("Parameters should be object type")
	}
}

func TestShellToolExecute(t *testing.T) {
	mgr := newTestWorkspaceMgr(t, "")
	tool := NewShellTool(mgr, false)
	ctx := context.Background()

	params := json.RawMessage(`{"command":"echo hello"}`)
	result, err := tool.Execute(ctx, params)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	t.Logf("Result: %s", result)
}

func TestShellToolWithTimeout(t *testing.T) {
	mgr := newTestWorkspaceMgr(t, "")
	tool := NewShellTool(mgr, false)
	ctx := context.Background()

	params := json.RawMessage(`{"command":"sleep 0.1","timeout":1}`)
	_, err := tool.Execute(ctx, params)

	if err != nil {
		t.Fatalf("Execute with timeout failed: %v", err)
	}
}

func TestShellToolInvalidParams(t *testing.T) {
	mgr := newTestWorkspaceMgr(t, "")
	tool := NewShellTool(mgr, false)
	ctx := context.Background()

	params := json.RawMessage(`{}`)
	_, err := tool.Execute(ctx, params)

	if err != nil {
		t.Error("Empty params should still work with default timeout")
	}
}

func TestShellToolSandbox(t *testing.T) {
	mgr := newTestWorkspaceMgr(t, "/tmp")
	tool := NewShellTool(mgr, true)

	if !tool.sandboxed {
		t.Error("Tool should be sandboxed")
	}
}

func TestShellToolDangerousCommand(t *testing.T) {
	mgr := newTestWorkspaceMgr(t, "")
	tool := NewShellTool(mgr, true)
	ctx := context.Background()

	// 测试危险命令检测
	params := json.RawMessage(`{"command":"rm -rf /"}`)
	_, err := tool.Execute(ctx, params)

	if err == nil {
		t.Error("Dangerous command should be blocked in sandbox mode")
	}
}

func TestFileToolBasic(t *testing.T) {
	mgr := newTestWorkspaceMgr(t, "")
	tool := NewFileTool(mgr, false)

	if tool.Name() != "file" {
		t.Errorf("Expected name=file, got %s", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	if !tool.IsDangerous() {
		t.Error("File tool should be dangerous")
	}
}

func TestFileToolReadWrite(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "lingguard-file-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewWorkspaceManager(tmpDir, "")
	tool := NewFileTool(mgr, false)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "test.txt")

	// 写入文件
	writeParams := json.RawMessage(`{"operation":"write","path":"` + testFile + `","content":"Hello World"}`)
	result, err := tool.Execute(ctx, writeParams)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	t.Logf("Write result: %s", result)

	// 读取文件
	readParams := json.RawMessage(`{"operation":"read","path":"` + testFile + `"}`)
	result, err = tool.Execute(ctx, readParams)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if result != "Hello World" {
		t.Errorf("Expected content='Hello World', got %s", result)
	}
}

func TestFileToolEdit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-file-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewWorkspaceManager(tmpDir, "")
	tool := NewFileTool(mgr, false)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "edit.txt")

	// 先写入
	tool.Execute(ctx, json.RawMessage(`{"operation":"write","path":"`+testFile+`","content":"Hello World"}`))

	// 编辑
	editParams := json.RawMessage(`{"operation":"edit","path":"` + testFile + `","old_string":"World","new_string":"Go"}`)
	_, err = tool.Execute(ctx, editParams)
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	// 验证结果
	result, _ := tool.Execute(ctx, json.RawMessage(`{"operation":"read","path":"`+testFile+`"}`))
	if result != "Hello Go" {
		t.Errorf("Expected 'Hello Go', got %s", result)
	}
}

func TestFileToolList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-file-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建一些文件
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("1"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	mgr := NewWorkspaceManager(tmpDir, "")
	tool := NewFileTool(mgr, false)
	ctx := context.Background()

	params := json.RawMessage(`{"operation":"list","path":"` + tmpDir + `"}`)
	result, err := tool.Execute(ctx, params)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	t.Logf("List result: %s", result)
}

func TestFileToolInvalidOperation(t *testing.T) {
	mgr := newTestWorkspaceMgr(t, "")
	tool := NewFileTool(mgr, false)
	ctx := context.Background()

	params := json.RawMessage(`{"operation":"invalid","path":"/tmp"}`)
	_, err := tool.Execute(ctx, params)

	if err == nil {
		t.Error("Invalid operation should return error")
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()
	mgr := newTestWorkspaceMgr(t, "")

	shellTool := NewShellTool(mgr, false)
	fileTool := NewFileTool(mgr, false)

	registry.Register(shellTool)
	registry.Register(fileTool)

	// 测试 Get
	tl, ok := registry.Get("shell")
	if !ok {
		t.Error("Should find shell tool")
	}
	if tl.Name() != "shell" {
		t.Errorf("Expected shell tool, got %s", tl.Name())
	}

	// 测试 List
	tools := registry.List()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	// 测试 GetToolDefinitions
	defs := registry.GetToolDefinitions()
	if len(defs) != 2 {
		t.Errorf("Expected 2 definitions, got %d", len(defs))
	}
}

func TestToolDefinition(t *testing.T) {
	mgr := newTestWorkspaceMgr(t, "")
	tool := NewShellTool(mgr, false)
	def := Definition(tool)

	if def["type"] != "function" {
		t.Error("Definition type should be function")
	}

	fn, ok := def["function"].(map[string]interface{})
	if !ok {
		t.Fatal("Function should be a map")
	}

	if fn["name"] != "shell" {
		t.Errorf("Expected function name=shell, got %s", fn["name"])
	}
}

func TestWorkspaceTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lingguard-workspace-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewWorkspaceManager(tmpDir, "")
	tool := NewWorkspaceTool(mgr)
	ctx := context.Background()

	// Test pwd
	pwdParams := json.RawMessage(`{"operation":"pwd"}`)
	result, err := tool.Execute(ctx, pwdParams)
	if err != nil {
		t.Fatalf("pwd failed: %v", err)
	}
	t.Logf("pwd result: %s", result)

	// Verify workspace
	if mgr.Get() != tmpDir {
		t.Errorf("Expected workspace=%s, got %s", tmpDir, mgr.Get())
	}

	// Test ls
	lsParams := json.RawMessage(`{"operation":"ls"}`)
	result, err = tool.Execute(ctx, lsParams)
	if err != nil {
		t.Fatalf("ls failed: %v", err)
	}
	t.Logf("ls result: %s", result)

	// Test cd should fail (no longer supported)
	cdParams := json.RawMessage(`{"operation":"cd","path":"subdir"}`)
	_, err = tool.Execute(ctx, cdParams)
	if err == nil {
		t.Error("Expected cd to fail, but it succeeded")
	}
	t.Logf("cd error (expected): %v", err)
}

func TestFileToolSymlinkProtection(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "lingguard-symlink-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建工作区外的敏感文件
	outsideDir, err := os.MkdirTemp("", "lingguard-outside-*")
	if err != nil {
		t.Fatalf("Failed to create outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	sensitiveFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(sensitiveFile, []byte("SECRET DATA"), 0644); err != nil {
		t.Fatalf("Failed to create sensitive file: %v", err)
	}

	// 在工作区内创建指向外部文件的符号链接
	symlinkPath := filepath.Join(tmpDir, "link_to_secret")
	if err := os.Symlink(sensitiveFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// 创建启用沙箱的文件工具
	mgr := NewWorkspaceManager(tmpDir, "")
	tool := NewFileTool(mgr, true) // sandboxed = true
	ctx := context.Background()

	// 尝试通过符号链接读取外部文件（应该失败）
	params := json.RawMessage(`{"operation":"read","path":"` + symlinkPath + `"}`)
	_, err = tool.Execute(ctx, params)
	if err == nil {
		t.Error("Reading through symlink to outside workspace should be blocked")
	} else {
		t.Logf("Symlink protection worked: %v", err)
	}

	// 尝试通过符号链接写入外部文件（应该失败）
	writeParams := json.RawMessage(`{"operation":"write","path":"` + symlinkPath + `","content":"hacked"}`)
	_, err = tool.Execute(ctx, writeParams)
	if err == nil {
		t.Error("Writing through symlink to outside workspace should be blocked")
	} else {
		t.Logf("Symlink write protection worked: %v", err)
	}

	// 工作区内的正常文件应该可以访问
	normalFile := filepath.Join(tmpDir, "normal.txt")
	normalParams := json.RawMessage(`{"operation":"write","path":"` + normalFile + `","content":"hello"}`)
	_, err = tool.Execute(ctx, normalParams)
	if err != nil {
		t.Errorf("Writing to normal file in workspace should succeed: %v", err)
	}

	// 路径遍历攻击（应该失败）
	traversalParams := json.RawMessage(`{"operation":"read","path":"` + filepath.Join(tmpDir, "..", "outside", "secret.txt") + `"}`)
	_, err = tool.Execute(ctx, traversalParams)
	if err == nil {
		t.Error("Path traversal attack should be blocked")
	} else {
		t.Logf("Path traversal protection worked: %v", err)
	}
}
