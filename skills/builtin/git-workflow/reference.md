---
name: git-workflow
description: Git 工作流详细技术参考
---

# Git Workflow 技术参考

## 功能概述

本文档提供 Git Workflow 技能的详细技术实现说明，包括环境变量、错误处理和安全机制。

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CODE_PATH` | 当前目录 | Git 仓库路径 |
| `AI_BRANCH` | `ai-test` | AI 分支名称 |
| `MAIN_BRANCH` | 自动检测 | 主分支名称（留空自动检测） |

## 脚本详解

### git_download.py

**功能**：切换到 AI 分支并拉取最新代码

**工作流程**：
1. 检查是否是 Git 仓库
2. 自动检测主分支（优先级：master > main > develop）
3. 检查 AI 分支是否存在
   - 不存在：从主分支创建
   - 存在：切换并拉取最新代码

**退出码**：
| 代码 | 含义 |
|------|------|
| 0 | 成功 |
| 1 | 一般错误（不是 Git 仓库、命令失败等） |

### git_upload.py

**功能**：提交所有更改并推送到 AI 分支

**工作流程**：
1. 检查是否是 Git 仓库
2. 检查是否在 AI 分支
3. 检查是否有更改
4. 添加所有更改到暂存区
5. 提交（自动生成提交信息：`AI update: YYYY-MM-DD HH:MM:SS`）
6. 推送到远程

**退出码**：
| 代码 | 含义 |
|------|------|
| 0 | 成功（包括没有更改的情况） |
| 1 | 一般错误（不在 AI 分支、推送失败等） |

## 主分支检测逻辑

```python
def get_main_branch():
    # 1. 尝试从远程 HEAD 获取
    # git symbolic-ref refs/remotes/origin/HEAD
    # 输出格式: refs/remotes/origin/main

    # 2. 尝试常见分支名（按优先级）
    # master > main > develop
    # git rev-parse origin/{branch}
```

## 安全检查机制

### 分支检查

```bash
# 上传时检查当前分支
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

if [[ "$CURRENT_BRANCH" != "ai-test" ]]; then
    echo "错误：当前在 $CURRENT_BRANCH 分支"
    echo "请先切换到 ai-test 分支"
    exit 1
fi
```

### 更改检测

```bash
# 检查是否有更改
CHANGES=$(git status --porcelain)

if [[ -z "$CHANGES" ]]; then
    echo "没有需要提交的更改"
    exit 0
fi
```

## 错误处理

### 常见错误及解决方案

| 错误 | 原因 | 解决方案 |
|------|------|----------|
| `❌ 不是 Git 仓库` | 不在 Git 仓库目录中 | 切换到仓库目录或设置 `CODE_PATH` |
| `❌ 当前在 master 分支` | 上传时不在 AI 分支 | 先运行 `git_download.py` |
| `⚠️ 没有需要提交的更改` | 没有修改任何文件 | 正常情况，无需处理 |
| `❌ 推送失败` | 远程有新提交 | 先运行 `git_download.py` 拉取更新 |

## 完整工作流程

```bash
# 1. 进入项目目录
cd /path/to/project

# 2. 下载代码（首次会从 master 创建 ai-test 分支）
python3 ./scripts/git_download.py

# 3. 进行开发工作...
# ... 编写代码 ...

# 4. 上传代码（自动提交并推送）
python3 ./scripts/git_upload.py
```

## 与 CI/CD 集成

### 示例：自动测试 AI 分支

```yaml
# .gitlab-ci.yml
ai-test:
  stage: test
  script:
    - git checkout ai-test
    - pip install -r requirements.txt
    - pytest tests/
  only:
    - ai-test
```

## Git 配置要求

确保已配置 Git 用户信息：

```bash
# 全局配置
git config --global user.name "Your Name"
git config --global user.email "your.email@example.com"

# 或仓库级别配置
git config --local user.name "Your Name"
git config --local user.email "your.email@example.com"
```
