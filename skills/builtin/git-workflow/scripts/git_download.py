#!/usr/bin/env python3
"""
Git Download 脚本
用途：切换到 ai-test 分支并拉取最新代码
- 如果 ai-test 不存在，从主分支创建
"""

import subprocess
import sys
import os

# 配置
CODE_PATH = os.getenv('CODE_PATH', os.getcwd())
AI_BRANCH = os.getenv('AI_BRANCH', 'ai-test')
MAIN_BRANCH = os.getenv('MAIN_BRANCH', '')  # 留空自动检测

def get_main_branch():
    """自动检测主分支"""
    # 尝试从远程 HEAD 获取
    result = subprocess.run(
        ['git', 'symbolic-ref', 'refs/remotes/origin/HEAD'],
        capture_output=True, text=True, cwd=CODE_PATH
    )
    if result.returncode == 0:
        return result.stdout.strip().split('/')[-1]

    # 尝试常见分支名
    for branch in ['main', 'master', 'develop']:
        result = subprocess.run(
            ['git', 'rev-parse', f'origin/{branch}'],
            capture_output=True, cwd=CODE_PATH
        )
        if result.returncode == 0:
            return branch

    return 'main'

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

    # 获取主分支名称
    main_branch = MAIN_BRANCH or get_main_branch()
    print(f"{Colors.YELLOW}📌 检测到主分支: {main_branch}{Colors.NC}")

    # 检查 ai-test 分支是否存在
    result = run_cmd(['git', 'rev-parse', '--verify', AI_BRANCH], check=False)

    if result.returncode != 0:
        # 分支不存在，从主分支创建
        print(f"{Colors.YELLOW}📌 分支 {AI_BRANCH} 不存在，从 {main_branch} 创建...{Colors.NC}")

        # 先拉取主分支最新代码
        run_cmd(['git', 'fetch', 'origin', main_branch])

        # 从主分支创建 ai-test 分支
        run_cmd(['git', 'checkout', '-b', AI_BRANCH, f'origin/{main_branch}'])
        print(f"{Colors.GREEN}✅ 已创建并切换到分支 {AI_BRANCH}{Colors.NC}")
    else:
        # 分支存在，切换并拉取
        print(f"{Colors.YELLOW}📌 切换到分支 {AI_BRANCH}...{Colors.NC}")
        run_cmd(['git', 'checkout', AI_BRANCH])

        print(f"{Colors.YELLOW}📌 拉取最新代码...{Colors.NC}")
        result = run_cmd(['git', 'pull', 'origin', AI_BRANCH], check=False)
        if result.returncode == 0:
            print(f"{Colors.GREEN}✅ 代码已更新{Colors.NC}")
        else:
            print(f"{Colors.YELLOW}⚠️ 远程分支可能不存在，这是正常的{Colors.NC}")
            print(f"{Colors.GREEN}✅ 已切换到分支 {AI_BRANCH}{Colors.NC}")

if __name__ == '__main__':
    main()
