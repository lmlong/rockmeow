---
name: coding
description: 代码分析、优化。当用户说"分析代码"、"优化代码"、"重构"、"debug"时使用
metadata: {"nanobot":{"emoji":"💻"}}
---
# 编码任务

## ⚡ 必须调用 opencode

**立即调用 opencode 工具（使用 action=prompt）**：

```json
{
  "action": "prompt",
  "task": "分析 ~/.lingguard/workspace/tasksched 目录下的代码，找出问题并进行优化",
  "agent": "build"
}
```

**不要用 shell cat 读取代码，必须用 opencode！**

---

## ⚠️ opencode 只用于分析和优化代码

✅ `{"action": "prompt", "task": "分析优化代码", "agent": "build"}`
❌ 不要用 opencode 执行 git/脚本/shell

---

## 降级方案

仅当 opencode 不可用时，才用默认工具：
- 读取：`{"operation": "read", "path": "xxx"}`
- 编辑：`{"operation": "edit", "path": "xxx", "old_string": "旧", "new_string": "新"}`
