#!/usr/bin/env python3
"""
Git Upload 脚本
用途：提交所有更改并推送到 ai-test 分支
"""

import subprocess
import sys
import os
from datetime import datetime

# 配置
CODE_PATH = os.getenv('CODE_PATH', os.getcwd())
AI_BRANCH = os.getenv('AI_BRANCH', 'ai-test')

class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    NC = '\033[0m'

def run_cmd(cmd, check=True):
    """执行命令"""
    result = subprocess.run(cmd, capture_output=True, text=True, cwd=CODE_PATH)
    if check and result.returncode != 0:
        print(f"{Colors.RED}❌ 命令失败: {' '.join(cmd)}{Colors.NC}")
        print(result.stderr)
        sys.exit(1)
    return result

def main():
    # 检查是否是 Git 仓库
    if run_cmd(['git', 'rev-parse', '--git-dir'], check=False).returncode != 0:
        print(f"{Colors.RED}❌ {CODE_PATH} 不是 Git 仓库{Colors.NC}")
        sys.exit(1)

    # 获取当前分支
    result = run_cmd(['git', 'rev-parse', '--abbrev-ref', 'HEAD'])
    current_branch = result.stdout.strip()

    # 检查是否在 ai-test 分支
    if current_branch != AI_BRANCH:
        print(f"{Colors.RED}❌ 当前在 {current_branch} 分支，请先切换到 {AI_BRANCH} 分支{Colors.NC}")
        print(f"{Colors.YELLOW}💡 运行: python3 git_download.py{Colors.NC}")
        sys.exit(1)

    # 检查是否有更改
    result = run_cmd(['git', 'status', '--porcelain'])
    if not result.stdout.strip():
        print(f"{Colors.YELLOW}⚠️ 没有需要提交的更改{Colors.NC}")
        sys.exit(0)

    # 显示更改
    print(f"{Colors.YELLOW}📌 检测到以下更改:{Colors.NC}")
    result = run_cmd(['git', 'status', '--short'])
    print(result.stdout)

    # 添加所有更改
    print(f"{Colors.YELLOW}📌 添加更改到暂存区...{Colors.NC}")
    run_cmd(['git', 'add', '-A'])

    # 生成提交信息
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    commit_msg = f"AI update: {timestamp}"

    # 提交
    print(f"{Colors.YELLOW}📌 提交更改...{Colors.NC}")
    run_cmd(['git', 'commit', '-m', commit_msg])
    print(f"{Colors.GREEN}✅ 已提交: {commit_msg}{Colors.NC}")

    # 推送
    print(f"{Colors.YELLOW}📌 推送到 {AI_BRANCH}...{Colors.NC}")
    result = run_cmd(['git', 'push', '-u', 'origin', AI_BRANCH], check=False)
    if result.returncode == 0:
        print(f"{Colors.GREEN}✅ 推送成功{Colors.NC}")
    else:
        # 可能是远程分支不存在，尝试推送
        print(f"{Colors.YELLOW}⚠️ 尝试推送新分支...{Colors.NC}")
        result = run_cmd(['git', 'push', 'origin', AI_BRANCH], check=False)
        if result.returncode == 0:
            print(f"{Colors.GREEN}✅ 推送成功{Colors.NC}")
        else:
            print(f"{Colors.RED}❌ 推送失败{Colors.NC}")
            print(result.stderr)
            sys.exit(1)

if __name__ == '__main__':
    main()
