---
name: coding
description: MUST use opencode tool for ALL coding tasks - writing, editing, analyzing code
metadata: {"nanobot":{"emoji":"💻","always":true}}
always: true
---
# Coding Tasks - MUST Use OpenCode

**CRITICAL: For ANY coding-related task, use ONLY the `opencode` tool. DO NOT use other tools.**

## Rules

1. **Use ONLY opencode tool** for coding tasks
2. **DO NOT** use `file`, `shell`, or `workspace` tools to check or verify OpenCode's work
3. **DO NOT** run additional commands after opencode completes
4. OpenCode handles everything: writing, editing, running, and verifying code

## How to Use opencode

Send the complete task to opencode in ONE call:

```json
{"action": "prompt", "task": "Fix ioutil usage in database/leveldb_job.go, replace with os package", "agent": "build"}
```

OpenCode will:
- Find and read the file
- Make the changes
- Verify the fix works

## Example - Code Fix Task

**Request**: "修复 database/leveldb_job.go 中的 ioutil 使用"

**Correct** - One opencode call:
```json
{"action": "prompt", "task": "Fix ioutil usage in database/leveldb_job.go. Replace ioutil.ReadAll with io.ReadAll and ioutil.NopCloser with io.NopCloser. Update imports.", "agent": "build"}
```

**WRONG** - Using multiple tools:
```
opencode → shell → file → shell  <-- DO NOT DO THIS
```

## Timeout Handling

If opencode times out:
1. **DO NOT** fall back to file/shell tools
2. Tell user: "OpenCode 处理超时，请稍后重试或简化任务"
3. Suggest breaking the task into smaller pieces

## Tips for Complex Tasks

For large tasks, split into smaller ones:
```
Task 1: "Fix ioutil in file A"
Task 2: "Fix ioutil in file B"
```

Instead of: "Fix ioutil in all files"

## Quick Reference

| Task | Action | Example |
|------|--------|---------|
| Write code | prompt | `{"action": "prompt", "task": "Create xxx"}` |
| Fix code | prompt | `{"action": "prompt", "task": "Fix xxx in file.go"}` |
| Run tests | shell | `{"action": "shell", "task": "go test ./..."}` |
| Code review | command | `{"action": "command", "task": "/review"}` |

**One opencode call = Complete task. No follow-up tools needed.**
