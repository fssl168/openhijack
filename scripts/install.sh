#!/bin/bash

set -e

if [ "$(id -u)" -ne 0 ]; then
    echo "需要 root 权限运行安装脚本"
    echo "请使用: sudo bash $0"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
INSTALL_DIR="/usr/local/bin"
SERVICE_DIR="/etc/systemd/system"

echo "=== OpenHijack 安装脚本 ==="
echo ""

cd "$PROJECT_DIR"

echo "[1/4] 编译项目..."
go build -o openhijack ./cmd/openhijack

echo "[2/4] 安装二进制文件..."
cp openhijack "$INSTALL_DIR/openhijack"
chmod +x "$INSTALL_DIR/openhijack"

echo "[3/4] 创建 systemd 服务..."
cat > "$SERVICE_DIR/openhijack.service" << 'EOF'
[Unit]
Description=OpenHijack HTTPS Proxy Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/openhijack serve
Restart=on-failure
RestartSec=5
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=false

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable openhijack.service

echo "[4/4] 设置完成!"
echo ""
echo "安装位置: $INSTALL_DIR/openhijack"
echo "服务状态: 已启用开机自启"
echo ""
echo "常用命令:"
echo "  启动服务:   sudo systemctl start openhijack"
echo "  停止服务:   sudo systemctl stop openhijack"
echo "  查看状态:   sudo systemctl status openhijack"
echo "  查看日志:   sudo journalctl -u openhijack -f"
echo ""
echo "首次使用请先初始化配置:"
echo "  openhijack init"
echo "  openhijack elevate"
