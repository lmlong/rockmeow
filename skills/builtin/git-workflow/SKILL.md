---
name: git-workflow
description: Git 工作流自动化技能，包含拉取代码、创建 AI 分支、推送代码等操作，内置安全检查防止误推送到主分支
metadata: {"nanobot":{"emoji":"🌿","requires":{"bins":["git","python3"]}}}
---

# Git Workflow

## 功能概述

自动化 Git 工作流程，包括：
- **拉取最新代码**：自动检测主分支（main/master）并拉取最新代码
- **创建/切换分支**：支持自动生成时间戳分支名或使用固定分支
- **推送代码**：安全地将代码推送到远程仓库
- **安全保护**：内置检查，禁止推送到保护分支

## 环境变量配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CODE_PATH` | 当前目录 | Git 仓库路径 |
| `AI_BRANCH` | 空 | 固定使用的 AI 分支名（如 `ai-test`） |
| `MAIN_BRANCH` | 自动检测 | 主分支名称 |
| `PROTECTED_BRANCHES` | `main,master` | 保护分支列表（逗号分隔） |

## 核心功能

### 1. 拉取代码 (git_pull.py)

自动检测主分支并拉取最新代码：

```bash
python3 ./scripts/git_pull.py
```

### 2. 创建/切换分支 (git_branch.py)

**自动生成分支名**（格式：`ai-年月日时分秒`）：
```bash
python3 ./scripts/git_branch.py
# 输出：✅ 已创建并切换到分支 ai-20260214162505
```

**使用固定分支**（推荐用于 AI 开发）：
```bash
AI_BRANCH=ai-test python3 ./scripts/git_branch.py
# 如果 ai-test 不存在，从主分支创建
# 如果已存在，直接切换
```

### 3. 推送代码 (git_push.py)

推送代码到远程仓库（内置安全检查）：

```bash
python3 ./scripts/git_push.py
```

- ✅ 允许推送到：`ai-*`、`feature/*`、`bugfix/*`、`develop` 等非保护分支
- ❌ 禁止推送到：`main`、`master` 等保护分支

## 典型工作流

### AI 开发模式（使用固定分支）

```bash
# 1. 设置环境变量
export AI_BRANCH=ai-test

# 2. 拉取最新代码
python3 ./scripts/git_pull.py

# 3. 切换到 AI 分支
python3 ./scripts/git_branch.py

# 4. 进行代码修改...

# 5. 推送代码
python3 ./scripts/git_push.py
```

### 日常开发模式（自动生成分支名）

```bash
# 1. 拉取最新代码
python3 ./scripts/git_pull.py

# 2. 创建新的 AI 分支
python3 ./scripts/git_branch.py

# 3. 进行代码修改...

# 4. 推送代码
python3 ./scripts/git_push.py
```

## 安全特性

| 检查项 | 说明 |
|--------|------|
| 主分支检测 | 自动检测 main/master/develop |
| 分支保护 | 拒绝推送到 main、master 等保护分支 |
| 工作区检查 | 推送前检查未提交的更改 |
| 交互确认 | 有未提交更改时询问是否继续 |

## 参考文件

- [scripts/git_pull.py](./scripts/git_pull.py) - 拉取代码脚本
- [scripts/git_branch.py](./scripts/git_branch.py) - 创建分支脚本
- [scripts/git_push.py](./scripts/git_push.py) - 推送代码脚本
- [scripts/git_merge.py](./scripts/git_merge.py) - 合并分支脚本
