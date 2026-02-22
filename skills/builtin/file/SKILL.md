---
name: file
description: 文件操作。当用户说"读取文件"、"写入文件"、"编辑文件"、"查看文件"、"列出目录"、"创建文件"、"修改文件"时使用
metadata: {"nanobot":{"emoji":"📁"}}
---
# File Operations

Use the `file` tool to work with files and directories.

## ⚠️ 重要限制

**所有文件操作都严格限制在工作目录内：**
- 只能访问配置文件中指定的工作目录
- 不能访问工作目录之外的任何路径
- 不能切换到其他目录
- 不能修改 LingGuard 的配置文件
- 如果用户请求访问工作目录外的文件，必须明确告知用户这个限制

## Operations

### Read a File

```json
{
  "operation": "read",
  "path": "relative/path/to/file.txt"
}
```

Returns the file contents as a string. Path can be relative to workspace.

### Write a File

```json
{
  "operation": "write",
  "path": "relative/path/to/file.txt",
  "content": "File contents here"
}
```

Creates the file (and parent directories) if it doesn't exist.

### Edit a File

```json
{
  "operation": "edit",
  "path": "relative/path/to/file.txt",
  "old_string": "text to replace",
  "new_string": "replacement text"
}
```

Replaces all occurrences of `old_string` with `new_string`.

### List Directory

```json
{
  "operation": "list",
  "path": "relative/path/to/directory"
}
```

Returns entries with type prefix: `dir: dirname` or `file: filename`.

## Best Practices

1. **Read before edit**: Always read a file first to understand its structure
2. **Precise old_string**: Make `old_string` unique to avoid unintended replacements
3. **Use relative paths**: Paths are relative to workspace directory
4. **Backup important files**: Consider copying before major edits
