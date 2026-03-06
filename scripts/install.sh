#!/bin/bash
# LingGuard 安装/更新脚本
# 用法: ./scripts/install.sh [--restart]
# 支持: Linux (systemd), macOS (launchd)

set -e

# 配置
PREFIX="${PREFIX:-${HOME}/.local}"
BIN_NAME="lingguard"
SERVICE_NAME="lingguard"
CONFIG_DIR="${HOME}/.lingguard"
SKILLS_DIR="${CONFIG_DIR}/skills"
FORCE_RESTART="${1:-}"

# 检测操作系统
OS="$(uname -s)"
case "$OS" in
    Linux*)  PLATFORM="linux" ;;
    Darwin*) PLATFORM="macos" ;;
    *)       echo "不支持的操作系统: $OS"; exit 1 ;;
esac

# 检查是否以 root 运行（用于系统级安装）
if [ "$EUID" -eq 0 ]; then
    CONFIG_DIR="/root/.lingguard"
    SKILLS_DIR="${CONFIG_DIR}/skills"
fi

# 检测是否为更新模式
IS_UPDATE="false"
if [ -f "${PREFIX}/bin/${BIN_NAME}" ]; then
    IS_UPDATE="true"
fi

# 显示标题
if [ "$IS_UPDATE" = "true" ]; then
    echo "=== LingGuard 更新 ==="
else
    echo "=== LingGuard 安装 ==="
fi
echo "平台: $PLATFORM"
echo "PREFIX: $PREFIX"
echo "CONFIG_DIR: $CONFIG_DIR"
echo "模式: $([ "$IS_UPDATE" = "true" ] && echo "更新" || echo "新安装")"
echo ""

# 1. 安装二进制文件
echo "[1/5] 安装二进制文件..."
mkdir -p "${PREFIX}/bin"

# 查找二进制文件
BIN_FILE=""
if [ -f "lingguard" ] && [ ! -d "lingguard" ]; then
    BIN_FILE="lingguard"
elif [ -f "lingguard/lingguard" ]; then
    BIN_FILE="lingguard/lingguard"
else
    for f in lingguard lingguard.exe; do
        if [ -f "$f" ] && [ ! -d "$f" ]; then
            BIN_FILE="$f"
            break
        fi
    done
fi

if [ -z "$BIN_FILE" ]; then
    echo "错误: 找不到 lingguard 二进制文件"
    echo "请确保在解压后的目录中运行此脚本"
    echo ""
    echo "当前目录内容:"
    ls -la
    exit 1
fi

# 停止服务（更新模式）
if [ "$IS_UPDATE" = "true" ] && [ "$PLATFORM" = "linux" ]; then
    echo "  停止运行中的服务..."
    if [ "$EUID" -eq 0 ]; then
        systemctl stop ${SERVICE_NAME} 2>/dev/null || true
    else
        systemctl --user stop ${SERVICE_NAME} 2>/dev/null || true
    fi
fi

install -m 755 "${BIN_FILE}" "${PREFIX}/bin/${BIN_NAME}"

# macOS: 移除隔离属性
if [ "$PLATFORM" = "macos" ]; then
    xattr -cr "${PREFIX}/bin/${BIN_NAME}" 2>/dev/null || true
fi
echo "  ✓ 已安装到 ${PREFIX}/bin/${BIN_NAME}"

# 2. 创建配置目录
echo "[2/5] 创建配置目录..."
mkdir -p "${CONFIG_DIR}"
mkdir -p "${CONFIG_DIR}/workspace"
mkdir -p "${CONFIG_DIR}/memory"
mkdir -p "${CONFIG_DIR}/cron"
mkdir -p "${CONFIG_DIR}/logs"
mkdir -p "${CONFIG_DIR}/locks"
mkdir -p "${CONFIG_DIR}/moltbook"
rm -f "${CONFIG_DIR}/locks/"*.lock 2>/dev/null || true
echo "  ✓ 已创建 ${CONFIG_DIR}"

# 2.1 安装 Moltbook 凭证（如果存在且不存在）
if [ -f "configs/moltbook/credentials.json" ]; then
    if [ ! -f "${CONFIG_DIR}/moltbook/credentials.json" ]; then
        cp configs/moltbook/credentials.json "${CONFIG_DIR}/moltbook/"
        echo "  ✓ 已安装 Moltbook 凭证"
    else
        echo "  ! Moltbook 凭证已存在，保留"
    fi
fi

# 3. 安装配置文件（仅新安装）
echo "[3/5] 安装配置文件..."
if [ "$IS_UPDATE" = "true" ]; then
    echo "  - 更新模式，跳过配置文件"
elif [ ! -f "${CONFIG_DIR}/config.json" ]; then
    # 新安装：创建配置文件
    if [ -f "configs/config.json" ]; then
        cat configs/config.json | \
            sed "s|\"workspace\": *\"[^\"]*\"|\"workspace\": \"${CONFIG_DIR}/workspace\"|g" | \
            sed "s|\"storePath\": *\"[^\"]*\"|\"storePath\": \"${CONFIG_DIR}/cron/jobs.json\"|g" | \
            sed "s|\"path\": *\"[^\"]*\"|\"path\": \"${CONFIG_DIR}/memory\"|g" | \
            sed "s|\"output\": *\"[^\"]*\"|\"output\": \"${CONFIG_DIR}/logs/lingguard.log\"|g" \
            > "${CONFIG_DIR}/config.json"
        echo "  ✓ 已创建 ${CONFIG_DIR}/config.json"
    else
        echo "  ! configs/config.json 不存在，跳过"
    fi
else
    echo "  ! 配置文件已存在，跳过"
fi

# 3.1 安装 HEARTBEAT.md（仅不存在时）
if [ ! -f "${CONFIG_DIR}/workspace/HEARTBEAT.md" ]; then
    if [ -f "configs/HEARTBEAT.md" ]; then
        cp configs/HEARTBEAT.md "${CONFIG_DIR}/workspace/HEARTBEAT.md"
        echo "  ✓ 已创建 ${CONFIG_DIR}/workspace/HEARTBEAT.md"
    fi
fi

# 4. 更新技能目录
echo "[4/5] 安装技能目录..."
if [ -d "skills" ]; then
    mkdir -p "${SKILLS_DIR}"

    # 更新模式：先备份用户自定义技能
    if [ "$IS_UPDATE" = "true" ]; then
        # 获取内置技能列表
        BUILTIN_SKILLS=""
        if [ -d "skills" ]; then
            BUILTIN_SKILLS=$(ls -1 skills/ 2>/dev/null)
        fi

        # 更新内置技能（覆盖）
        for skill in $BUILTIN_SKILLS; do
            if [ -d "skills/${skill}" ]; then
                rm -rf "${SKILLS_DIR}/${skill}" 2>/dev/null || true
                cp -r "skills/${skill}" "${SKILLS_DIR}/"
            fi
        done
        echo "  ✓ 已更新内置技能"
    else
        # 新安装：直接复制所有技能
        cp -r skills/* "${SKILLS_DIR}/" 2>/dev/null || true
        echo "  ✓ 已安装内置技能到 ${SKILLS_DIR}"
    fi
else
    echo "  ! skills 目录不存在，跳过"
fi

# 5. 安装/更新服务
echo "[5/5] 安装自动启动服务..."

if [ "$PLATFORM" = "macos" ]; then
    # macOS: 使用 launchd
    if [ -f "scripts/com.lingguard.plist" ]; then
        PLIST_DIR="${HOME}/Library/LaunchAgents"
        mkdir -p "${PLIST_DIR}"
        cat scripts/com.lingguard.plist | \
            sed "s|{{HOME}}|${HOME}|g" | \
            sed "s|{{BIN}}|${PREFIX}/bin/lingguard|g" \
            > "${PLIST_DIR}/com.lingguard.plist"

        # 卸载旧服务（如果存在）
        launchctl unload "${PLIST_DIR}/com.lingguard.plist" 2>/dev/null || true

        # 加载服务
        launchctl load "${PLIST_DIR}/com.lingguard.plist"

        # 启动服务
        launchctl kickstart -k "gui/$(id -u)/com.lingguard" 2>/dev/null || true

        if [ "$IS_UPDATE" = "true" ]; then
            echo "  ✓ 已重启 launchd 服务"
        else
            echo "  ✓ 已安装并启动 launchd 服务"
        fi
    else
        echo "  ! scripts/com.lingguard.plist 不存在，跳过"
    fi
else
    # Linux: 使用 systemd
    if [ -f "scripts/lingguard.service" ]; then
        if [ "$EUID" -eq 0 ]; then
            # 系统级安装
            SERVICE_DIR="/etc/systemd/system"
            cat scripts/lingguard.service | \
                sed "s|{{USER}}|root|g" | \
                sed "s|{{HOME}}|/root|g" | \
                sed "s|{{BIN}}|${PREFIX}/bin/lingguard|g" \
                > "${SERVICE_DIR}/${SERVICE_NAME}.service"
            sed -i '/WorkingDirectory/a User=root' "${SERVICE_DIR}/${SERVICE_NAME}.service"

            systemctl daemon-reload
            systemctl enable ${SERVICE_NAME}

            if [ "$IS_UPDATE" = "true" ]; then
                systemctl start ${SERVICE_NAME}
                echo "  ✓ 已重启系统级 systemd 服务"
            else
                systemctl start ${SERVICE_NAME}
                echo "  ✓ 已安装并启动系统级 systemd 服务"
            fi
            echo ""
            systemctl status ${SERVICE_NAME} --no-pager || true
        else
            # 用户级安装
            SERVICE_DIR="${HOME}/.config/systemd/user"
            mkdir -p "${SERVICE_DIR}"
            cat scripts/lingguard.service | \
                sed "s|{{HOME}}|${HOME}|g" | \
                sed "s|{{BIN}}|${PREFIX}/bin/lingguard|g" \
                > "${SERVICE_DIR}/${SERVICE_NAME}.service"

            systemctl --user daemon-reload
            systemctl --user enable ${SERVICE_NAME}

            if [ "$IS_UPDATE" = "true" ]; then
                systemctl --user start ${SERVICE_NAME}
                echo "  ✓ 已重启用户级 systemd 服务"
            else
                systemctl --user start ${SERVICE_NAME}
                echo "  ✓ 已安装并启动用户级 systemd 服务"
            fi
            echo ""
            systemctl --user status ${SERVICE_NAME} --no-pager || true
        fi
    else
        echo "  ! scripts/lingguard.service 不存在，跳过"
    fi
fi

# 完成
echo ""
if [ "$IS_UPDATE" = "true" ]; then
    echo "=== 更新完成 ==="
else
    echo "=== 安装完成 ==="
fi
echo ""
echo "配置文件: ${CONFIG_DIR}/config.json"
echo "工作目录: ${CONFIG_DIR}/workspace"
echo "日志目录: ${CONFIG_DIR}/logs"
echo ""
echo "服务管理:"
if [ "$PLATFORM" = "linux" ]; then
    if [ "$EUID" -eq 0 ]; then
        echo "  systemctl status lingguard    # 查看状态"
        echo "  systemctl restart lingguard   # 重启服务"
        echo "  systemctl stop lingguard      # 停止服务"
        echo "  journalctl -u lingguard -f    # 查看日志"
    else
        echo "  systemctl --user status lingguard    # 查看状态"
        echo "  systemctl --user restart lingguard   # 重启服务"
        echo "  systemctl --user stop lingguard      # 停止服务"
        echo "  journalctl --user -u lingguard -f    # 查看日志"
    fi
elif [ "$PLATFORM" = "macos" ]; then
    echo "  launchctl list | grep lingguard                    # 查看状态"
    echo "  launchctl kickstart -k gui/\$(id -u)/com.lingguard  # 重启服务"
    echo "  launchctl stop com.lingguard                       # 停止服务"
    echo "  tail -f ~/.lingguard/logs/lingguard.log            # 查看日志"
fi
echo ""
echo "其他命令:"
echo "  lingguard agent        # 交互模式"
echo "  lingguard --help       # 查看帮助"
