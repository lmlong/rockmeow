---
name: git-workflow
description: Git 工作流使用示例
---

# Git Workflow 使用示例

## 基础示例

### 示例 1：完整工作流程

```bash
# 步骤 1：下载代码（自动切换到 ai-test 分支，不存在则从 master 创建）
python3 ./scripts/git_download.py
# 输出：
#   📌 检测到主分支: master
#   📌 分支 ai-test 不存在，从 master 创建...
#   ✅ 已创建并切换到分支 ai-test

# 步骤 2：查看当前分支
git branch
# 输出：
#   * ai-test
#     master

# 步骤 3：进行更改...

# 步骤 4：上传代码（自动提交并推送）
python3 ./scripts/git_upload.py
# 输出：
#   📌 检测到以下更改:
#    M README.md
#   📌 添加更改到暂存区...
#   📌 提交更改...
#   ✅ 已提交: AI update: 2026-02-14 16:57:29
#   ✅ 推送成功
```

### 示例 2：第二次使用（分支已存在）

```bash
# 下载代码（ai-test 已存在，直接切换并拉取）
python3 ./scripts/git_download.py
# 输出：
#   📌 检测到主分支: master
#   📌 切换到分支 ai-test...
#   📌 拉取最新代码...
#   ✅ 代码已更新

# 进行开发...
vim README.md

# 上传代码
python3 ./scripts/git_upload.py
```

## 高级示例

### 示例 3：自定义分支名称

```bash
# 使用环境变量自定义 AI 分支名
AI_BRANCH=ai-feature python3 ./scripts/git_download.py

# 进行开发...

# 上传时也需要指定相同的分支名
AI_BRANCH=ai-feature python3 ./scripts/git_upload.py
```

### 示例 4：指定工作目录

```bash
# 指定仓库路径
CODE_PATH=/path/to/repo python3 ./scripts/git_download.py

# 进行开发...

CODE_PATH=/path/to/repo python3 ./scripts/git_upload.py
```

### 示例 5：批量操作多个仓库

```bash
#!/bin/bash
# multi-repo-update.sh

REPOS=(
    "/path/to/repo1"
    "/path/to/repo2"
    "/path/to/repo3"
)

SCRIPTS_DIR="/path/to/git-workflow/scripts"

for repo in "${REPOS[@]}"; do
    echo "处理仓库: $repo"

    # 下载代码
    CODE_PATH="$repo" python3 "$SCRIPTS_DIR/git_download.py"

    # 进行一些修改...
    # ...

    # 上传代码
    CODE_PATH="$repo" python3 "$SCRIPTS_DIR/git_upload.py"
done
```

## 场景示例

### 场景 1：AI 辅助修复 Bug

```bash
# 1. 下载最新代码
python3 ./scripts/git_download.py

# 2. AI 分析并修复代码
# ... AI 工作过程 ...

# 3. 上传修复
python3 ./scripts/git_upload.py
# 输出：
#   ✅ 已提交: AI update: 2026-02-14 16:57:29
#   ✅ 推送成功
```

### 场景 2：AI 生成新功能

```bash
# 1. 下载代码
python3 ./scripts/git_download.py

# 2. AI 生成新功能代码
# 多个文件被修改...

# 3. 查看变更
git status
# 输出：
# modified:   src/api/user.py
# modified:   src/models/user.py
# new file:   tests/test_user_api.py

# 4. 上传所有更改
python3 ./scripts/git_upload.py
```

### 场景 3：紧急修复流程

```bash
# 快速下载代码
python3 ./scripts/git_download.py

# 快速修复...
vim src/critical.py

# 快速上传
python3 ./scripts/git_upload.py
```

## 故障排除示例

### 问题：推送被拒绝

```bash
$ python3 ./scripts/git_upload.py
❌ 推送失败
! [rejected]        ai-test -> ai-test (fetch first)

# 解决方案：先拉取远程更新
python3 ./scripts/git_download.py
python3 ./scripts/git_upload.py
```

### 问题：不在 ai-test 分支

```bash
$ python3 ./scripts/git_upload.py
❌ 当前在 master 分支，请先切换到 ai-test 分支
💡 运行: python3 git_download.py

# 解决方案：先运行下载脚本
python3 ./scripts/git_download.py
python3 ./scripts/git_upload.py
```

### 问题：没有更改需要提交

```bash
$ python3 ./scripts/git_upload.py
⚠️ 没有需要提交的更改

# 这是正常的，说明没有文件被修改
```
