---
name: git-sync
description: Git 代码同步（下载/上传）。当用户说"下载代码"、"上传代码"、"同步代码"、"拉取代码"、"推送代码"、"git clone"、"git push"、"上库"、"提交代码"、"克隆仓库"时使用，固定使用 ai-test 分支
metadata: {"nanobot":{"emoji":"🔄","requires":{"bins":["git","python3"]}}}
---

# Git 代码同步

## ⚠️ 重要规则

- **只使用 shell 工具**执行所有 git 操作
- **不要使用 MCP 文件系统工具**（mcp_filesystem_*）
- 固定使用 `ai-test` 分支进行所有操作
- 工作目录：`~/.lingguard/workspace`

## 功能

简化的 Git 同步操作，固定使用 `ai-test` 分支：

- **克隆仓库**：克隆新仓库到 workspace
- **下载代码**：切换到 `ai-test` 分支并拉取最新代码
- **上传代码**：提交所有更改并推送到 `ai-test` 分支

## 触发场景

- "下载代码"、"git clone"、"克隆仓库"
- "上传代码"、"git push"、"上库"、"提交代码"
- "同步代码"、"拉取代码"、"推送代码"

## 用法

### 克隆新仓库

**当用户给出 git 仓库 URL 时使用**：

```bash
cd ~/.lingguard/workspace && git clone ssh://git@gitlab.example.com:9022/group/repo.git
```

然后进入仓库目录，切换到 ai-test 分支：

```bash
cd ~/.lingguard/workspace/repo && git checkout -b ai-test && git push -u origin ai-test
```

### 下载代码（已存在的仓库）

```bash
cd ~/.lingguard/workspace/repo && python3 ./scripts/git_download.py
```

工作流程：
1. 检测主分支（优先 master，其次 main）
2. 检查 `ai-test` 分支是否存在
   - 不存在：从主分支创建 `ai-test`
   - 存在：切换到 `ai-test` 并拉取最新代码

### 上传代码

```bash
cd ~/.lingguard/workspace/repo && python3 ./scripts/git_upload.py
```

工作流程：
1. 检查是否在 `ai-test` 分支
2. 检查是否有更改
3. 添加所有更改到暂存区
4. 提交（自动生成提交信息）
5. 推送到远程 `ai-test` 分支

## 典型流程

```bash
# 1. 克隆新仓库
cd ~/.lingguard/workspace && git clone ssh://git@xxx/repo.git

# 2. 进入仓库并创建 ai-test 分支
cd repo && git checkout -b ai-test && git push -u origin ai-test

# 3. 进行代码修改...

# 4. 上传代码
python3 ./scripts/git_upload.py
```

## 脚本文件

- [scripts/git_download.py](./scripts/git_download.py) - 下载代码
- [scripts/git_upload.py](./scripts/git_upload.py) - 上传代码
