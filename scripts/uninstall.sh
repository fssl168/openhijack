#!/bin/bash

set -e

if [ "$(id -u)" -ne 0 ]; then
    echo "需要 root 权限运行卸载脚本"
    echo "请使用: sudo bash $0"
    exit 1
fi

INSTALL_DIR="/usr/local/bin"
SERVICE_DIR="/etc/systemd/system"

echo "=== OpenHijack 卸载脚本 ==="
echo ""

echo "[1/3] 停止并移除服务..."
systemctl stop openhijack.service 2>/dev/null || true
systemctl disable openhijack.service 2>/dev/null || true
rm -f "$SERVICE_DIR/openhijack.service"
systemctl daemon-reload

echo "[2/3] 清理证书和 hosts..."
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
"$PROJECT_DIR/openhijack" cleanup 2>/dev/null || true

echo "[3/3] 移除二进制文件..."
rm -f "$INSTALL_DIR/openhijack"

echo ""
echo "卸载完成! 如需完全清理数据目录，请手动删除:"
echo "  rm -rf ~/.local/share/openhijack"
echo "  rm -rf ~/.config/openhijack"
