#!/usr/bin/env python3
"""
Git Branch 脚本
用途：从主分支创建新的 AI 分支，或切换到指定分支
"""

import subprocess
import sys
import os
from datetime import datetime


# 代码路径（优先使用环境变量，否则使用当前目录）
CODE_PATH = os.getenv('CODE_PATH', os.getcwd())

# 主分支名称（优先使用环境变量，默认 main）
MAIN_BRANCH = os.getenv('MAIN_BRANCH', 'main')

# 固定分支名（如果设置，则切换到此分支而不是创建新分支）
AI_BRANCH = os.getenv('AI_BRANCH', '')


class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    NC = '\033[0m'


def run_command_quiet(cmd: list) -> subprocess.CompletedProcess:
    """执行 shell 命令（静默，返回结果）"""
    try:
        return subprocess.run(cmd, check=True, capture_output=True, text=True, cwd=CODE_PATH)
    except subprocess.CalledProcessError:
        print(f"{Colors.RED}命令执行失败: {' '.join(cmd)}{Colors.NC}")
        sys.exit(1)


def get_default_branch() -> str:
    """自动检测默认分支"""
    # 先尝试从远程获取
    try:
        result = subprocess.run(
            ['git', 'symbolic-ref', 'refs/remotes/origin/HEAD'],
            capture_output=True, text=True, cwd=CODE_PATH
        )
        if result.returncode == 0:
            return result.stdout.strip().split('/')[-1]
    except Exception:
        pass

    # 尝试常见的默认分支名
    for branch in ['main', 'master', 'develop']:
        try:
            subprocess.run(
                ['git', 'rev-parse', f'origin/{branch}'],
                capture_output=True, check=True, cwd=CODE_PATH
            )
            return branch
        except subprocess.CalledProcessError:
            continue

    return MAIN_BRANCH


def print_success(msg: str) -> None:
    print(f"{Colors.GREEN}✅ {msg}{Colors.NC}")


def print_info(msg: str) -> None:
    print(f"{Colors.YELLOW}📌 {msg}{Colors.NC}")


def get_ai_branch_name() -> str:
    """生成基于时间戳的 AI 分支名"""
    timestamp = datetime.now().strftime('%Y%m%d%H%M%S')
    return f"ai-{timestamp}"


def main() -> None:
    # 检查是否是 Git 仓库
    try:
        subprocess.run(['git', 'rev-parse', '--git-dir'],
                      capture_output=True, check=True, cwd=CODE_PATH)
    except subprocess.CalledProcessError:
        print(f"{Colors.RED}❌ {CODE_PATH} 不是 Git 仓库{Colors.NC}")
        sys.exit(1)

    # 获取默认分支
    main_branch = get_default_branch()
    print_info(f"检测到主分支: {main_branch}")

    # 确定要使用的分支名
    if AI_BRANCH:
        # 使用固定分支名
        branch_name = AI_BRANCH
        print_info(f"使用固定 AI 分支: {branch_name}")

        # 检查分支是否存在
        result = subprocess.run(['git', 'rev-parse', '--verify', branch_name],
                               capture_output=True, cwd=CODE_PATH)
        if result.returncode == 0:
            # 分支存在，直接切换
            print_info(f"切换到已存在的分支: {branch_name}")
            try:
                subprocess.run(['git', 'checkout', branch_name],
                              capture_output=True, check=True, cwd=CODE_PATH)
                print_success(f"已切换到分支 {branch_name}")
            except subprocess.CalledProcessError:
                print(f"{Colors.RED}❌ 切换到分支 {branch_name} 失败{Colors.NC}")
                sys.exit(1)
        else:
            # 分支不存在，从主分支创建
            print_info(f"分支 {branch_name} 不存在，从 {main_branch} 创建...")
            # 切换到主分支
            try:
                subprocess.run(['git', 'checkout', main_branch],
                              capture_output=True, check=True, cwd=CODE_PATH)
            except subprocess.CalledProcessError:
                print(f"{Colors.RED}❌ 切换到 {main_branch} 分支失败{Colors.NC}")
                sys.exit(1)

            # 创建新分支
            try:
                subprocess.run(['git', 'checkout', '-b', branch_name],
                              capture_output=True, check=True, cwd=CODE_PATH)
                print_success(f"已创建并切换到分支 {branch_name}")
            except subprocess.CalledProcessError:
                print(f"{Colors.RED}❌ 创建分支失败{Colors.NC}")
                sys.exit(1)
    else:
        # 生成分支名
        branch_name = get_ai_branch_name()
        print_info(f"正在从 {main_branch} 创建 AI 分支: {branch_name}")

        # 切换到主分支
        print_info(f"切换到 {main_branch} 分支...")
        try:
            subprocess.run(['git', 'checkout', main_branch],
                          capture_output=True, check=True, cwd=CODE_PATH)
        except subprocess.CalledProcessError:
            try:
                subprocess.run(['git', 'switch', main_branch],
                              capture_output=True, check=True, cwd=CODE_PATH)
            except subprocess.CalledProcessError:
                print(f"{Colors.RED}❌ 切换到 {main_branch} 分支失败{Colors.NC}")
                sys.exit(1)

        # 创建新分支
        try:
            subprocess.run(['git', 'checkout', '-b', branch_name],
                          capture_output=True, check=True, cwd=CODE_PATH)
            print_success(f"已创建并切换到分支 {branch_name}")
        except subprocess.CalledProcessError:
            try:
                subprocess.run(['git', 'switch', '-c', branch_name],
                              capture_output=True, check=True, cwd=CODE_PATH)
                print_success(f"已创建并切换到分支 {branch_name}")
            except subprocess.CalledProcessError:
                print(f"{Colors.RED}❌ 创建分支失败{Colors.NC}")
                sys.exit(1)


if __name__ == '__main__':
    main()
