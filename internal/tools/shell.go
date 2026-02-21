package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ShellTool Shell 执行工具
type ShellTool struct {
	workspaceMgr *WorkspaceManager
	sandboxed    bool
}

// NewShellTool 创建 Shell 工具
func NewShellTool(workspaceMgr *WorkspaceManager, sandboxed bool) *ShellTool {
	return &ShellTool{
		workspaceMgr: workspaceMgr,
		sandboxed:    sandboxed,
	}
}

func (t *ShellTool) Name() string { return "shell" }

func (t *ShellTool) Description() string {
	return "Execute shell commands. Use with caution."
}

func (t *ShellTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds (default: 30)",
			},
		},
		"required": []string{"command"},
	}
}

func (t *ShellTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Timeout == 0 {
		p.Timeout = 30
	}

	// 安全检查
	if t.sandboxed {
		if err := t.validateCommand(p.Command); err != nil {
			return "", err
		}
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, time.Duration(p.Timeout)*time.Second)
	defer cancel()

	// 执行命令
	cmd := exec.CommandContext(ctx, "bash", "-c", p.Command)
	if t.workspaceMgr != nil {
		cmd.Dir = t.workspaceMgr.Get()
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := fmt.Sprintf("stdout:\n%s\nstderr:\n%s",
		stdout.String(), stderr.String())

	if err != nil {
		result += fmt.Sprintf("\nerror: %s", err)
	}

	return result, nil
}

func (t *ShellTool) IsDangerous() bool { return true }

// 危险命令黑名单模式（参考 nanobot）
var dangerousPatterns = []string{
	// 文件系统破坏
	`\brm\s+-[rf]{1,2}\b`,            // rm -r, rm -rf, rm -fr
	`\brm\s+-(?:[a-z])*r(?:[a-z])*f`, // rm -Rf, rm -fr 等变体
	`\bdel\s+/[fq]\b`,                // del /f, del /q (Windows)
	`\brmdir\s+/s\b`,                 // rmdir /s (Windows)

	// 磁盘操作
	`\b(?:mkfs|diskpart)\b`, // 磁盘格式化/分区
	`\bdd\s+if=`,            // dd 磁盘写入
	`>\s*/dev/sd`,           // 写入磁盘设备
	`>\s*/dev/hd`,           // 写入 IDE 磁盘

	// 系统控制
	`\b(?:shutdown|reboot|poweroff|halt|init\s+[06])\b`, // 系统电源控制

	// Fork 炸弹
	`:\(\)\s*\{.*\};\s*:`, // :(){ :|:& };:

	// 权限提升
	`\b(?:sudo|su|doas)\s+`, // 权限提升（可选：根据需求启用）

	// 网络危险操作
	`\b(?:iptables|ufw|firewall-cmd)\b`, // 防火墙修改

	// 系统关键目录
	`/dev/(?:null|zero|random|urandom)`, // 设备文件
}

var denyRegexps []*regexp.Regexp

func init() {
	// 预编译正则表达式
	for _, pattern := range dangerousPatterns {
		denyRegexps = append(denyRegexps, regexp.MustCompile("(?i)"+pattern))
	}
}

func (t *ShellTool) validateCommand(cmd string) error {
	// 1. 检查危险命令黑名单
	for _, re := range denyRegexps {
		if re.MatchString(cmd) {
			return fmt.Errorf("dangerous command detected: %s", re.String())
		}
	}

	// 2. 如果启用了沙箱，检查路径遍历
	if t.sandboxed && t.workspaceMgr != nil {
		if strings.Contains(cmd, "../") || strings.Contains(cmd, "..\\") {
			// 检测到路径遍历尝试
			return fmt.Errorf("path traversal detected in command")
		}
	}

	return nil
}
