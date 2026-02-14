#!/usr/bin/env python3
"""
Git Push 脚本
用途：推送代码到远程仓库（带安全检查）
"""

import subprocess
import sys
import os


# 代码路径（优先使用环境变量，否则使用当前目录）
CODE_PATH = os.getenv('CODE_PATH', os.getcwd())

# 保护分支列表
PROTECTED_BRANCHES = os.getenv('PROTECTED_BRANCHES', 'main,master').split(',')


class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    NC = '\033[0m'


def run_command_quiet(cmd: list, cwd: str = None) -> subprocess.CompletedProcess:
    """执行 shell 命令（静默，返回结果）"""
    try:
        return subprocess.run(cmd, check=True, capture_output=True, text=True, cwd=cwd or CODE_PATH)
    except subprocess.CalledProcessError:
        print(f"{Colors.RED}命令执行失败: {' '.join(cmd)}{Colors.NC}")
        sys.exit(1)


def get_default_branch() -> str:
    """自动检测默认分支"""
    try:
        result = subprocess.run(
            ['git', 'symbolic-ref', 'refs/remotes/origin/HEAD'],
            capture_output=True, text=True, cwd=CODE_PATH
        )
        if result.returncode == 0:
            return result.stdout.strip().split('/')[-1]
    except Exception:
        pass

    for branch in ['main', 'master', 'develop']:
        try:
            subprocess.run(
                ['git', 'rev-parse', f'origin/{branch}'],
                capture_output=True, check=True, cwd=CODE_PATH
            )
            return branch
        except subprocess.CalledProcessError:
            continue

    return 'main'


def print_success(msg: str) -> None:
    print(f"{Colors.GREEN}✅ {msg}{Colors.NC}")


def print_error(msg: str) -> None:
    print(f"{Colors.RED}❌ {msg}{Colors.NC}")


def print_warning(msg: str) -> None:
    print(f"{Colors.YELLOW}⚠️  {msg}{Colors.NC}")


def print_info(msg: str) -> None:
    print(f"{Colors.YELLOW}📌 {msg}{Colors.NC}")


def main() -> None:
    # 检测仓库目录
    repo_dir = None
    try:
        result = subprocess.run(['git', 'rev-parse', '--git-dir'],
                               capture_output=True, text=True)
        if result.returncode == 0:
            repo_dir = os.getcwd()
    except Exception:
        pass

    if not repo_dir:
        try:
            result = subprocess.run(['git', 'rev-parse', '--git-dir'],
                                   capture_output=True, text=True, cwd=CODE_PATH)
            if result.returncode == 0:
                repo_dir = CODE_PATH
        except Exception:
            pass

    if not repo_dir:
        print_error("当前目录和 CODE_PATH 都不是 Git 仓库")
        sys.exit(1)

    print_info(f"使用仓库目录: {repo_dir}")

    # 获取当前分支
    result = run_command_quiet(['git', 'rev-parse', '--abbrev-ref', 'HEAD'], cwd=repo_dir)
    current_branch = result.stdout.strip()

    # 动态检测保护分支
    default_branch = get_default_branch()
    protected = PROTECTED_BRANCHES + [default_branch]

    # 检查是否为保护分支
    if current_branch in protected:
        print_error(f"禁止推送到保护分支 '{current_branch}'")
        print_warning("请切换到 ai-*、feature/* 或其他开发分支")
        sys.exit(1)

    # 检查工作区状态
    result = run_command_quiet(['git', 'status', '--porcelain'], cwd=repo_dir)
    if result.stdout.strip():
        print_warning("警告：工作区有未提交的更改")
        result = run_command_quiet(['git', 'status', '--short'], cwd=repo_dir)
        print(result.stdout)
        response = input("仍然继续推送？[y/N]: ")
        if response.lower() != 'y':
            print_warning("推送已取消")
            sys.exit(0)

    # 推送
    print_info(f"正在推送到远程仓库: {current_branch}")
    result = subprocess.run(['git', 'push', '-u', 'origin', current_branch], cwd=repo_dir)
    if result.returncode == 0:
        print_success("推送成功")
    else:
        print_error("推送失败")
        sys.exit(1)


if __name__ == '__main__':
    main()
