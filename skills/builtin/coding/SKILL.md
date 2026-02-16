---
name: coding
description: MUST use opencode tool for ALL coding tasks - writing, editing, analyzing code
metadata: {"nanobot":{"emoji":"💻","always":true}}
always: true
---
# Coding Tasks - MUST Use OpenCode

**CRITICAL: For ANY coding-related task, you MUST use the `opencode` tool. DO NOT use the `file` or `shell` tools for coding tasks.**

The `opencode` tool delegates to OpenCode, a professional AI coding agent that handles file creation, editing, and code analysis.

## When to Use opencode (MANDATORY)

You MUST use `opencode` for these tasks:

- Writing code (any language: Python, Go, JavaScript, etc.)
- Creating new source files
- Editing existing code
- Refactoring code
- Debugging
- Running tests or build commands
- Code analysis and review
- Any task involving `.py`, `.go`, `.js`, `.ts`, `.java`, etc.

## How to Use opencode

### Simple Coding Task

For writing code:

```json
{"action": "prompt", "task": "Create a Python hello world program in hello.py", "agent": "build"}
```

### Editing Code

```json
{"action": "prompt", "task": "Add error handling to the main function in app.py", "agent": "build"}
```

### Running Commands

```json
{"action": "shell", "task": "python hello.py"}
```

## Examples

**Request**: "使用python语言编写一个hello world的程序"

**Correct Action** - Use opencode tool:
```json
{"action": "prompt", "task": "Create a Python hello world program in hello.py", "agent": "build"}
```

**WRONG** - Do NOT use file tool:
```
file tool with operation: write  <-- DO NOT DO THIS
```

## Agent Types

- `build` - Default. Writes and edits files.
- `plan` - Only plans, no file changes.

## Quick Reference

| Task | Tool | Example |
|------|------|---------|
| Write code | opencode | `{"action": "prompt", "task": "Create xxx"}` |
| Edit code | opencode | `{"action": "prompt", "task": "Modify xxx"}` |
| Run tests | opencode | `{"action": "shell", "task": "go test"}` |
| Code review | opencode | `{"action": "command", "task": "/review"}` |
| Read docs | file | `{"operation": "read", "path": "README.md"}` |
| List files | file | `{"operation": "list", "path": "."}` |

**Remember: opencode for coding, file for reading docs/listing only.**
