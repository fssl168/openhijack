#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BINARY_NAME="openhijack-gui"

BINARY_PATHS=(
    "$PROJECT_DIR/gui/build/bin/$BINARY_NAME"
    "$PROJECT_DIR/gui/$BINARY_NAME"
    "$PROJECT_DIR/build/$BINARY_NAME"
    "$BINARY_NAME"
)

BINARY_PATH=""
for path in "${BINARY_PATHS[@]}"; do
    if [ -f "$path" ]; then
        BINARY_PATH="$path"
        break
    fi
done

if [ -z "$BINARY_PATH" ]; then
    echo "错误: 未找到 GUI 二进制文件"
    echo ""
    echo "搜索路径:"
    for path in "${BINARY_PATHS[@]}"; do
        echo "  - $path"
    done
    echo ""
    echo "请先使用 wails 构建 GUI:"
    echo "  cd $PROJECT_DIR/gui && wails build"
    exit 1
fi

echo "找到 GUI 二进制: $BINARY_PATH"

check_display_env() {
    if [ -z "$DISPLAY" ]; then
        echo "错误: DISPLAY 环境变量未设置"
        echo "请确保在图形桌面环境中运行此脚本"
        exit 1
    fi
    
    if [ -z "$XAUTHORITY" ] && [ -f "$HOME/.Xauthority" ]; then
        export XAUTHORITY="$HOME/.Xauthority"
        echo "已设置 XAUTHORITY=$XAUTHORITY"
    fi
    
    echo "显示环境:"
    echo "  DISPLAY=$DISPLAY"
    echo "  XAUTHORITY=${XAUTHORITY:-未设置}"
    echo "  USER=$USER"
}

run_gui_with_privilege() {
    if [ "$(id -u)" -eq 0 ]; then
        echo "以 root 身份运行 GUI..."
        check_display_env
        exec "$BINARY_PATH"
        return
    fi

    echo "请求提升权限以启动 GUI..."

    SUDO_USER="$USER"
    SUDO_HOME="$(eval echo ~$USER)"

    if [ -z "$DISPLAY" ]; then
        echo "错误: 无法获取 DISPLAY 环境变量"
        exit 1
    fi

    export DISPLAY="$DISPLAY"

    if [ -f "$SUDO_HOME/.Xauthority" ]; then
        export XAUTHORITY="$SUDO_HOME/.Xauthority"
    fi

    config_dir=""
    if [ -d "$SUDO_HOME/.config/openhijack" ]; then
        config_dir="$SUDO_HOME/.config/openhijack"
    elif [ -d "$SUDO_HOME/.openhijack" ]; then
        config_dir="$SUDO_HOME/.openhijack"
    fi

    if [ -n "$config_dir" ] && [ -f "$config_dir/config.toml" ]; then
        export OPENHIJACK_CONFIG="$config_dir/config.toml"
        echo "配置文件: $OPENHIJACK_CONFIG"
    fi

    echo "启动 GUI 应用 (sudo 模式)..."
    exec sudo -E \
        -u root \
        --preserve-env=DISPLAY \
        --preserve-env=XAUTHORITY \
        --preserve-env=HOME \
        --preserve-env=USER \
        --preserve-env=SUDO_USER \
        --preserve-env=OPENHIJACK_CONFIG \
        -H \
        bash -c "
            export DISPLAY='$DISPLAY'
            export XAUTHORITY='${XAUTHORITY:-}'
            export HOME='$SUDO_HOME'
            export SUDO_USER='$SUDO_USER'
            export OPENHIJACK_CONFIG='${OPENHIJACK_CONFIG:-}'
            echo '=== OpenHijack GUI (sudo 模式) ==='
            echo 'DISPLAY=' \$DISPLAY
            echo 'XAUTHORITY=' \${XAUTHORITY:-未设置}
            echo 'SUDO_USER=' \$SUDO_USER
            echo 'HOME=' \$HOME
            exec '$BINARY_PATH'
        "
}

run_elevate_mode() {
    echo "使用 elevate 服务模式启动（无 GUI）..."

    SUDO_USER="$USER"
    SUDO_HOME="$(eval echo ~$USER)"

    config_dir=""
    if [ -d "$SUDO_HOME/.config/openhijack" ]; then
        config_dir="$SUDO_HOME/.config/openhijack"
    elif [ -d "$SUDO_HOME/.openhijack" ]; then
        config_dir="$SUDO_HOME/.openhijack"
    fi

    if [ -n "$config_dir" ] && [ -f "$config_dir/config.toml" ]; then
        export OPENHIJACK_CONFIG="$config_dir/config.toml"
        echo "配置文件: $OPENHIJACK_CONFIG"
    fi

    if [ "$(id -u)" -eq 0 ]; then
        echo "已是 root 用户，直接启动 elevate 服务模式..."
        exec "$BINARY_PATH" elevate "${@:2}"
    fi

    exec sudo -E \
        --preserve-env=DISPLAY \
        --preserve-env=XAUTHORITY \
        --preserve-env=HOME \
        --preserve-env=SUDO_USER \
        --preserve-env=OPENHIJACK_CONFIG \
        -H \
        "$BINARY_PATH" elevate "${@:2}"
}

case "${1:-start}" in
    start|gui)
        run_gui_with_privilege "$@"
        ;;

    elevate)
        run_elevate_mode "${@:2}"
        ;;

    serve)
        echo "服务模式启动 (elevate)..."
        run_elevate_mode "${@:2}"
        ;;

    normal)
        echo "普通模式启动（无权限提升）..."
        check_display_env
        exec "$BINARY_PATH"
        ;;

    debug)
        echo "调试模式启动..."
        check_display_env

        if [ "$(id -u)" -ne 0 ]; then
            echo "提示: 使用 sudo 运行可获得完整功能"
        fi

        exec "$BINARY_PATH"
        ;;

    *)
        echo "用法: $0 {start|gui|elevate|serve|normal|debug}"
        echo ""
        echo "命令:"
        echo "  start     启动 GUI 图形界面（默认，推荐）"
        echo "  gui       同 start"
        echo "  elevate   服务模式（无 GUI，仅代理服务器）"
        echo "  serve     同 elevate"
        echo "  normal    普通用户启动（无 sudo）"
        echo "  debug     调试模式启动"
        echo ""
        echo "示例:"
        echo "  $0 start                # 启动 GUI（推荐）✅"
        echo "  $0 elevate              # 纯服务模式（无界面）"
        echo "  $0 serve --port 8443    # 服务模式指定端口"
        echo "  $0 normal               # 普通用户启动"
        echo "  $0 debug                # 调试模式"
        ;;
esac
