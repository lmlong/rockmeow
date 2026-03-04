---
name: git-sync
description: Git 代码同步（下载/上传）。触发词：下载代码、上传代码、git clone、git push、上库
metadata: {"nanobot":{"emoji":"🔄","requires":{"bins":["git","python3"]}}}
---

# Git 代码同步

## 下载代码

```json
{"command": "python3 ~/.lingguard/skills/git-sync/scripts/git_download.py --clone <仓库URL>"}
```

## 上传代码

```json
{"command": "python3 ~/.lingguard/skills/git-sync/scripts/git_upload.py"}
```

## 多任务流程

"下载代码，分析优化，并上库"：
1. **git-sync** → 下载
2. **coding** → 分析优化
3. **git-sync** → 上传
