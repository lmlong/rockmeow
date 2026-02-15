package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileTool 文件操作工具
type FileTool struct {
	workspaceMgr *WorkspaceManager
	sandboxed    bool
}

// NewFileTool 创建文件工具
func NewFileTool(workspaceMgr *WorkspaceManager, sandboxed bool) *FileTool {
	return &FileTool{
		workspaceMgr: workspaceMgr,
		sandboxed:    sandboxed,
	}
}

func (t *FileTool) Name() string { return "file" }

func (t *FileTool) Description() string {
	return "Read, write, edit, and list files"
}

func (t *FileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"read", "write", "edit", "list"},
				"description": "The file operation to perform",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The file or directory path",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write (for write operation)",
			},
			"old_string": map[string]interface{}{
				"type":        "string",
				"description": "String to replace (for edit operation)",
			},
			"new_string": map[string]interface{}{
				"type":        "string",
				"description": "Replacement string (for edit operation)",
			},
		},
		"required": []string{"operation", "path"},
	}
}

func (t *FileTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Operation string `json:"operation"`
		Path      string `json:"path"`
		Content   string `json:"content,omitempty"`
		OldString string `json:"old_string,omitempty"`
		NewString string `json:"new_string,omitempty"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	// 安全检查
	if t.sandboxed {
		if err := t.validatePath(p.Path); err != nil {
			return "", err
		}
	}

	switch p.Operation {
	case "read":
		return t.readFile(p.Path)
	case "write":
		return t.writeFile(p.Path, p.Content)
	case "edit":
		return t.editFile(p.Path, p.OldString, p.NewString)
	case "list":
		return t.listDir(p.Path)
	default:
		return "", fmt.Errorf("unknown operation: %s", p.Operation)
	}
}

func (t *FileTool) IsDangerous() bool { return true }

func (t *FileTool) validatePath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	absWorkspace, _ := filepath.Abs(t.workspaceMgr.Get())
	if !strings.HasPrefix(absPath, absWorkspace) {
		return fmt.Errorf("path outside workspace: %s", path)
	}

	return nil
}

func (t *FileTool) readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(data), nil
}

func (t *FileTool) writeFile(path, content string) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote to %s", path), nil
}

func (t *FileTool) editFile(path, oldString, newString string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	newContent := strings.ReplaceAll(string(content), oldString, newString)
	if newContent == string(content) {
		return "No changes made (old_string not found)", nil
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully edited %s", path), nil
}

func (t *FileTool) listDir(path string) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("failed to list directory: %w", err)
	}

	var result strings.Builder
	for _, entry := range entries {
		prefix := "file"
		if entry.IsDir() {
			prefix = "dir"
		}
		result.WriteString(fmt.Sprintf("%s: %s\n", prefix, entry.Name()))
	}

	return result.String(), nil
}
