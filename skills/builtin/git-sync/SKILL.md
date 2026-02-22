---
name: git-sync
description: Git 代码同步（下载/上传）。当用户说"下载代码"、"上传代码"、"同步代码"、"拉取代码"、"推送代码"时使用，固定使用 ai-test 分支
metadata: {"nanobot":{"emoji":"🔄","requires":{"bins":["git","python3"]}}}
---

# Git 代码同步

## 功能

简化的 Git 同步操作，固定使用 `ai-test` 分支：

- **下载代码**：切换到 `ai-test` 分支并拉取最新代码
- **上传代码**：提交所有更改并推送到 `ai-test` 分支

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CODE_PATH` | 当前目录 | Git 仓库路径 |
| `AI_BRANCH` | `ai-test` | AI 分支名称 |

## 用法

### 下载代码

```bash
python3 ./scripts/git_download.py
```

工作流程：
1. 检测主分支（优先 master，其次 main）
2. 检查 `ai-test` 分支是否存在
   - 不存在：从主分支创建 `ai-test`
   - 存在：切换到 `ai-test` 并拉取最新代码

### 上传代码

```bash
python3 ./scripts/git_upload.py
```

工作流程：
1. 检查是否在 `ai-test` 分支
2. 检查是否有更改
3. 添加所有更改到暂存区
4. 提交（自动生成提交信息）
5. 推送到远程 `ai-test` 分支

## 典型流程

```bash
# 1. 下载代码
python3 ./scripts/git_download.py

# 2. 进行代码修改...

# 3. 上传代码
python3 ./scripts/git_upload.py
```

## 脚本文件

- [scripts/git_download.py](./scripts/git_download.py) - 下载代码
- [scripts/git_upload.py](./scripts/git_upload.py) - 上传代码
