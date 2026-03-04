---
name: git-sync
description: Git 代码同步（下载/上传）。当用户说"下载代码"、"上传代码"、"git clone"、"git push"、"上库"时使用
metadata: {"nanobot":{"emoji":"🔄","requires":{"bins":["git","python3"]}}}
---

# Git 代码同步

## ⚡ 调用 shell 工具（tool name = "shell"）

**下载代码**：
```json
{"command": "python3 ~/.lingguard/skills/git-sync/scripts/git_download.py --clone <仓库URL>"}
```

**上传代码**：
```json
{"command": "python3 ~/.lingguard/skills/git-sync/scripts/git_upload.py"}
```

---

## ⚠️ 禁止使用 opencode 工具

**git 操作必须调用 shell 工具，绝对禁止调用 opencode 工具！**

```
❌ 错误：调用 opencode 工具（任何 action）
✅ 正确：调用 shell 工具
```

---

## 多任务流程

"下载代码，分析优化，并上库"：
1. git-sync → **shell 工具** 执行下载
2. coding → **opencode 工具** 分析优化
3. git-sync → **shell 工具** 执行上传
