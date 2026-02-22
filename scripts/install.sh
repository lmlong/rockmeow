#!/bin/bash
# LingGuard 安装脚本
# 用法: make install 或 sudo ./scripts/install.sh
# 支持: Linux (systemd), macOS (launchd)

set -e

# 配置
PREFIX="${PREFIX:-${HOME}/.local}"
BIN_NAME="lingguard"
SERVICE_NAME="lingguard"
CONFIG_DIR="${HOME}/.lingguard"
SKILLS_DIR="${CONFIG_DIR}/skills"

# 检测操作系统
OS="$(uname -s)"
case "$OS" in
    Linux*)  PLATFORM="linux" ;;
    Darwin*) PLATFORM="macos" ;;
    *)       echo "不支持的操作系统: $OS"; exit 1 ;;
esac

echo "=== LingGuard 安装 ==="
echo "平台: $PLATFORM"
echo "PREFIX: $PREFIX"
echo "CONFIG_DIR: $CONFIG_DIR"
echo ""

# 检查是否以 root 运行（用于系统级安装）
if [ "$EUID" -eq 0 ]; then
    # root 用户，使用 /root 作为 HOME
    CONFIG_DIR="/root/.lingguard"
    SKILLS_DIR="${CONFIG_DIR}/skills"
    echo "检测到 root 用户，配置目录: $CONFIG_DIR"
fi

# 1. 安装二进制文件
echo "[1/5] 安装二进制文件..."
mkdir -p "${PREFIX}/bin"
install -m 755 lingguard "${PREFIX}/bin/${BIN_NAME}"

# macOS: 移除隔离属性，允许运行未签名应用
if [ "$PLATFORM" = "macos" ]; then
    xattr -cr "${PREFIX}/bin/${BIN_NAME}" 2>/dev/null || true
    echo "  ✓ 已安装到 ${PREFIX}/bin/${BIN_NAME} (已移除隔离属性)"
else
    echo "  ✓ 已安装到 ${PREFIX}/bin/${BIN_NAME}"
fi

# 2. 创建配置目录
echo "[2/5] 创建配置目录..."
mkdir -p "${CONFIG_DIR}"
mkdir -p "${CONFIG_DIR}/workspace"
mkdir -p "${CONFIG_DIR}/memory"
mkdir -p "${CONFIG_DIR}/cron"
mkdir -p "${CONFIG_DIR}/logs"
mkdir -p "${CONFIG_DIR}/locks"
mkdir -p "${CONFIG_DIR}/moltbook"
# 清理旧锁文件
rm -f "${CONFIG_DIR}/locks/"*.lock 2>/dev/null || true
echo "  ✓ 已创建 ${CONFIG_DIR}"

# 2.1 安装 Moltbook 凭证（如果存在）
if [ -f "configs/moltbook/credentials.json" ]; then
    if [ ! -f "${CONFIG_DIR}/moltbook/credentials.json" ]; then
        cp configs/moltbook/credentials.json "${CONFIG_DIR}/moltbook/"
        echo "  ✓ 已安装 Moltbook 凭证到 ${CONFIG_DIR}/moltbook/credentials.json"
    else
        echo "  ! ${CONFIG_DIR}/moltbook/credentials.json 已存在，保留现有凭证"
    fi
fi

# 3. 安装配置文件（如果不存在）
echo "[3/5] 安装配置文件..."
if [ ! -f "${CONFIG_DIR}/config.json" ]; then
    # 使用 configs/config.json 作为模板
    if [ -f "configs/config.json" ]; then
        # 复制并调整配置
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
    echo "  ! ${CONFIG_DIR}/config.json 已存在，保留现有配置"
    # 检查是否需要更新 opencode 配置
    if ! grep -q '"opencode"' "${CONFIG_DIR}/config.json" 2>/dev/null; then
        echo "  ! 建议手动添加 opencode 配置到 tools 部分"
    fi
fi

# 4. 安装技能目录
echo "[4/5] 安装技能目录..."
if [ -d "skills/builtin" ]; then
    mkdir -p "${SKILLS_DIR}/builtin"
    cp -r skills/builtin/* "${SKILLS_DIR}/builtin/" 2>/dev/null || true
    echo "  ✓ 已安装内置技能到 ${SKILLS_DIR}/builtin"
else
    echo "  ! skills/builtin 目录不存在，跳过"
fi

# 5. 安装服务（可选）
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
        echo "  ✓ 已安装 launchd 服务"
        echo ""
        echo "启用并启动服务:"
        echo "  launchctl load ${PLIST_DIR}/com.lingguard.plist"
        echo ""
        echo "停止服务:"
        echo "  launchctl unload ${PLIST_DIR}/com.lingguard.plist"
        echo ""
        echo "查看状态:"
        echo "  launchctl list | grep lingguard"
    else
        echo "  ! scripts/com.lingguard.plist 不存在，跳过"
    fi
else
    # Linux: 使用 systemd
    if [ -f "scripts/lingguard.service" ]; then
        # 创建服务文件，替换配置路径
        if [ "$EUID" -eq 0 ]; then
            # 系统级安装
            SERVICE_DIR="/etc/systemd/system"
            cat scripts/lingguard.service | \
                sed "s|{{USER}}|root|g" | \
                sed "s|{{HOME}}|/root|g" | \
                sed "s|{{BIN}}|${PREFIX}/bin/lingguard|g" \
                > "${SERVICE_DIR}/${SERVICE_NAME}.service"
            # 系统级服务需要添加 User 字段
            sed -i '/WorkingDirectory/a User=root' "${SERVICE_DIR}/${SERVICE_NAME}.service"
            systemctl daemon-reload
            echo "  ✓ 已安装系统级 systemd 服务"
            echo ""
            echo "启用并启动服务:"
            echo "  sudo systemctl enable lingguard"
            echo "  sudo systemctl start lingguard"
        else
            # 用户级安装
            SERVICE_DIR="${HOME}/.config/systemd/user"
            mkdir -p "${SERVICE_DIR}"
            cat scripts/lingguard.service | \
                sed "s|{{HOME}}|${HOME}|g" | \
                sed "s|{{BIN}}|${PREFIX}/bin/lingguard|g" \
                > "${SERVICE_DIR}/${SERVICE_NAME}.service"
            echo "  ✓ 已安装用户级 systemd 服务"
            echo ""
            echo "注意: 用户服务需要启用 linger 才能在登录前运行"
            echo "  loginctl enable-linger \$USER"
            echo ""
            echo "启用并启动服务:"
            echo "  systemctl --user daemon-reload"
            echo "  systemctl --user enable lingguard"
            echo "  systemctl --user start lingguard"
        fi
    else
        echo "  ! scripts/lingguard.service 不存在，跳过"
    fi
fi

echo ""
echo "=== 安装完成 ==="
echo ""
echo "配置文件: ${CONFIG_DIR}/config.json"
echo "工作目录: ${CONFIG_DIR}/workspace"
echo "日志目录: ${CONFIG_DIR}/logs"
echo ""
echo "快速开始:"
echo "  lingguard agent        # 交互模式"
echo "  lingguard gateway      # 启动网关"
echo "  lingguard --help       # 查看帮助"
