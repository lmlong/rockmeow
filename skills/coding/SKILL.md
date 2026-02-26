---
name: coding
description: MUST use opencode tool for ALL coding tasks - writing, editing, analyzing code
metadata: {"nanobot":{"emoji":"💻","always":true}}
always: true
---
# Coding Tasks - MUST Use OpenCode

**CRITICAL: For ANY coding-related task, you MUST use ONLY the `opencode` tool.**

## ABSOLUTE RULES

1. ✅ **ONLY use `opencode`** for coding tasks (分析、优化、编写代码)
2. ❌ **NEVER use `file` tool** after opencode completes
3. ❌ **NEVER use `shell` tool** to verify opencode's work
4. ❌ **NEVER use `workspace` tool** to check files created by opencode
5. ✅ **SPLIT large tasks** into smaller steps (each opencode call < 20 minutes)
6. ⚠️ **下载和上传代码必须使用 git-workflow skill**（见下方）

**opencode returns complete results. Trust it. No verification needed.**

## 🚨 重要：下载和上传代码

**下载代码（git clone）和上传代码（git push）必须使用 `git-workflow` skill！**

### 下载代码
```
skill --name git-workflow
# 然后执行:
python3 ./scripts/git_download.py
```

### 上传代码
```
skill --name git-workflow
# 然后执行:
python3 ./scripts/git_upload.py
```

**❌ 禁止直接使用 git clone / git push 命令！**

## Task Size Guidelines

### Small Tasks (single opencode call)
- Fix a bug in one file
- Add a single function
- Refactor one module
- Write a simple feature

### Large Tasks (MUST split into multiple calls)
- Analyze entire project structure
- Optimize multiple files
- Major refactoring across modules
- Full project code review

**Split Pattern:**
```
Step 1: skill --name git-workflow (下载代码)
Step 2: opencode - "Analyze project structure, list main files and their purposes"
Step 3: opencode - "Review file X for optimization opportunities"
Step 4: opencode - "Apply optimizations to file X"
Step 5: skill --name git-workflow (上传代码)
...continue as needed...
```

## What opencode Handles

- Creating files
- Writing code
- Running tests
- Verifying results

**You do NOT need to check or verify anything after opencode returns.**

## Usage

```json
{"action": "prompt", "task": "Write a Go function that does X", "agent": "build"}
```

## WRONG Behavior (DO NOT DO THIS)

```
User: 下载代码，优化，并上库
You: [calls shell git clone]  ← WRONG! 应该使用 git-workflow skill
     [calls opencode]
     [calls shell git push]   ← WRONG! 应该使用 git-workflow skill
```

## CORRECT Behavior

```
User: 下载代码，优化，并上库
You: [calls skill --name git-workflow]  ✅ 下载代码
     [calls opencode]                    ✅ 分析和优化
     [calls skill --name git-workflow]  ✅ 上传代码
```

**opencode result is final. Report it directly to user. No follow-up tools.**
