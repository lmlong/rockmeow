---
name: system
description: System operations and shell command execution
metadata: {"nanobot":{"emoji":"💻","always":true,"requires":{"bins":["bash","sh"]}}}
---
# System Operations

Execute shell commands to interact with the operating system.

## ⚠️ 重要限制

**所有操作都严格限制在工作目录内：**
- Shell 命令在工作目录下执行
- 只能访问工作目录内的文件
- 不能访问工作目录之外的路径
- 不能切换到其他目录
- 不能修改 LingGuard 的配置文件

## Shell Tool

Use the `shell` tool to run commands:

```json
{
  "command": "ls -la",
  "timeout": 30
}
```

**Parameters:**
- `command` (required): The shell command to execute
- `timeout` (optional): Timeout in seconds, default 30

## Common Operations

### File System
- `ls -la` - List files with details
- `find . -name "*.go"` - Find Go files in current directory
- `grep -r "pattern" .` - Search for pattern in current directory

### Process Management
- `ps aux` - List processes
- `top -n 1` - System status

### Network
- `curl -I https://example.com` - HTTP headers
- `ping -c 3 google.com` - Network connectivity

## Safety Guidelines

1. Always check what a command does before executing
2. Use `--dry-run` flags when available
3. Avoid destructive commands unless explicitly requested
4. Be cautious with `rm`, `chmod`, `chown`
5. Don't execute commands with `sudo` unless necessary

## Output Format

The tool returns both stdout and stderr:
```
stdout: <command output>
stderr: <error output>
error: <error message if any>
```
