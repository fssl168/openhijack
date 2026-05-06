#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BINARY_NAME="openhijack"
BINARY_PATH="$PROJECT_DIR/build/$BINARY_NAME"

if [ ! -f "$BINARY_PATH" ]; then
    BINARY_PATH="$PROJECT_DIR/$BINARY_NAME"
fi

if [ ! -f "$BINARY_PATH" ]; then
    echo "错误: 未找到编译后的二进制文件"
    echo "请先运行: go build -o $BINARY_NAME ./cmd/openhijack"
    exit 1
fi

PID_FILE="/tmp/openhijack.pid"

run_with_privilege() {
    local mode="$1"
    shift
    
    if [ "$(id -u)" -eq 0 ]; then
        exec "$BINARY_PATH" serve "$@"
        return
    fi

    if "$BINARY_PATH" elevate "$@" 2>/dev/null; then
        return
    fi

    echo "elevate 不可用，尝试使用 sudo..."
    exec sudo -E "$BINARY_PATH" serve "$@"
}

case "${1:-start}" in
    init)
        echo "初始化配置..."
        "$BINARY_PATH" init "${@:2}"
        ;;
    
    start|serve)
        if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
            echo "OpenHijack 已在运行 (PID: $(cat "$PID_FILE"))"
            exit 0
        fi
        
        echo "启动 OpenHijack 代理服务器..."
        run_with_privilege "serve" "${@:2}"
        ;;
    
    elevate)
        exec "$BINARY_PATH" elevate "${@:2}"
        ;;
    
    debug)
        echo "调试模式启动 OpenHijack..."
        run_with_privilege "debug" --debug "${@:2}"
        ;;
    
    http)
        echo "HTTP 模式启动 OpenHijack (无 TLS)..."
        exec "$BINARY_PATH" serve --http "${@:2}"
        ;;
    
    status)
        if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
            echo "OpenHijack 运行中 (PID: $(cat "$PID_FILE"))"
        else
            echo "OpenHijack 未运行"
            rm -f "$PID_FILE"
        fi
        ;;
    
    paths)
        "$BINARY_PATH" paths
        ;;
    
    *)
        echo "用法: $0 {init|start|serve|elevate|debug|http|status|paths} [选项]"
        echo ""
        echo "命令:"
        echo "  init       初始化配置文件"
        echo "  start      启动代理服务器 (默认)"
        echo "  serve      启动代理服务器"
        echo "  elevate    权限提升模式启动"
        echo "  debug      调试模式启动"
        echo "  http       HTTP 模式 (无 TLS)"
        echo "  status     查看运行状态"
        echo "  paths      显示数据路径"
        echo ""
        echo "示例:"
        echo "  $0 init                    # 初始化配置"
        echo "  $0 start                   # 启动服务"
        echo "  $0 start --port 8443       # 指定端口启动"
        echo "  $0 debug                   # 调试模式"
        ;;
esac
