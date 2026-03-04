---
name: coding
description: 代码分析、优化。触发词：分析代码、优化代码、重构、debug、写代码
metadata: {"nanobot":{"emoji":"💻"}}
---
# 代码分析优化

## 调用 opencode

```json
{"action": "prompt", "task": "分析并优化代码", "agent": "build"}
```

## 降级方案

仅当 opencode 不可用时：
- 读取：`{"operation": "read", "path": "xxx"}`
- 编辑：`{"operation": "edit", "path": "xxx", "old_string": "旧", "new_string": "新"}`
