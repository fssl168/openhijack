#!/bin/bash

PID_FILE="/tmp/openhijack.pid"

if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo "正在停止 OpenHijack (PID: $PID)..."
        kill "$PID"
        
        i=0
        while [ $i -lt 30 ] && kill -0 "$PID" 2>/dev/null; do
            sleep 1
            i=$((i + 1))
        done
        
        if kill -0 "$PID" 2>/dev/null; then
            echo "强制终止..."
            kill -9 "$PID"
        fi
        
        rm -f "$PID_FILE"
        echo "OpenHijack 已停止"
    else
        echo "OpenHijack 未运行 (清理残留 PID 文件)"
        rm -f "$PID_FILE"
    fi
else
    pkill -f "openhijack.*serve" 2>/dev/null && echo "已终止进程" || echo "OpenHijack 未运行"
fi
