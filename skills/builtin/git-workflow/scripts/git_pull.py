#!/usr/bin/env python3
"""
Git Pull 脚本
用途：切换到主分支并拉取最新代码
"""

import subprocess
import sys
import os


# 代码路径（优先使用环境变量，否则使用当前目录）
CODE_PATH = os.getenv('CODE_PATH', os.getcwd())

# 主分支名称（优先使用环境变量，默认 main）
MAIN_BRANCH = os.getenv('MAIN_BRANCH', 'main')


class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    NC = '\033[0m'


def run_command(cmd: list) -> None:
    """执行 shell 命令"""
    try:
        subprocess.run(cmd, check=True, text=True, cwd=CODE_PATH)
    except subprocess.CalledProcessError:
        print(f"{Colors.RED}命令执行失败: {' '.join(cmd)}{Colors.NC}")
        sys.exit(1)


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
            # 输出格式: refs/remotes/origin/main
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

    # 切换到主分支
    print_info(f"正在切换到 {main_branch} 分支...")
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

    # 拉取最新代码
    print_info(f"正在拉取 {main_branch} 最新代码...")
    result = subprocess.run(['git', 'pull', 'origin', main_branch],
                           cwd=CODE_PATH)
    if result.returncode == 0:
        print_success(f"{main_branch} 代码已更新")
    else:
        print(f"{Colors.YELLOW}⚠️ 拉取代码可能有问题，请检查{Colors.NC}")


if __name__ == '__main__':
    main()
