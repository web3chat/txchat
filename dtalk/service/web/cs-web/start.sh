#!/bin/bash

# 前台 ./start.sh：关终端通常会让进程退出。
# 后台 ./start.sh daemon：nohup 脱离终端，关 SSH/窗口后仍运行。

DAEMON=0
case "${1:-}" in
    daemon|-d|--daemon) DAEMON=1 ;;
    -h|--help)
        echo "用法:"
        echo "  $0              前台启动（关闭终端一般会停止服务）"
        echo "  $0 daemon       后台启动（关闭终端仍运行；日志 cs-web-dev.log，PID 写入 cs-web-dev.pid）"
        exit 0
        ;;
esac

echo "启动客服工作台Web界面..."
echo

# Rollup 通过可选依赖安装平台二进制；若 node_modules 来自其它操作系统或未装好可选依赖，Linux 上会缺少 @rollup/rollup-linux-*。
# 返回当前 Linux 环境对应的 Rollup 可选包目录名（相对于 node_modules），非 Linux 或无法识别时输出空。
expected_rollup_linux_pkg() {
    [ "$(uname -s)" = "Linux" ] || return 0
    local musl=0
    if command -v ldd >/dev/null 2>&1 && [ -x /bin/sh ] && ldd /bin/sh 2>/dev/null | grep -q musl; then
        musl=1
    fi
    case "$(uname -m)" in
        x86_64)
            if [ "$musl" -eq 1 ]; then echo "@rollup/rollup-linux-x64-musl"; else echo "@rollup/rollup-linux-x64-gnu"; fi
            ;;
        aarch64|arm64)
            if [ "$musl" -eq 1 ]; then echo "@rollup/rollup-linux-arm64-musl"; else echo "@rollup/rollup-linux-arm64-gnu"; fi
            ;;
        armv7l)
            if [ "$musl" -eq 1 ]; then echo "@rollup/rollup-linux-arm-musleabihf"; else echo "@rollup/rollup-linux-arm-gnueabihf"; fi
            ;;
        *)
            ;;
    esac
}

ensure_node_deps() {
    local rollup_pkg
    rollup_pkg="$(expected_rollup_linux_pkg)"

    if [ ! -d "node_modules" ]; then
        echo "首次运行，安装依赖..."
        npm install
        echo
        return
    fi

    if [ -n "$rollup_pkg" ] && [ ! -d "node_modules/$rollup_pkg" ]; then
        echo "检测到 Rollup 平台原生模块缺失（目录: $rollup_pkg）。"
        echo "常见原因：从 macOS/Windows 复制了 node_modules，或 npm 可选依赖未正确安装。"
        echo "将删除 node_modules 并在本机重新执行 npm install ..."
        rm -rf node_modules
        npm install
        echo
        if [ ! -d "node_modules/$rollup_pkg" ]; then
            echo "仍未安装到 $rollup_pkg，请在本目录手动执行："
            echo "  rm -rf node_modules package-lock.json && npm install"
            echo "若使用 Alpine(musl)，请确认网络可访问 registry 以下载对应 musl 包。"
            exit 1
        fi
    fi
}

# 检查Node.js是否安装
if ! command -v node &> /dev/null; then
    echo "Node.js 未安装，请先安装 Node.js (https://nodejs.org/)"
    echo
    read -p "按任意键继续..."
    exit 1
fi

# 检查npm是否安装
if ! command -v npm &> /dev/null; then
    echo "npm 未安装，请先安装 npm"
    echo
    read -p "按任意键继续..."
    exit 1
fi

ensure_node_deps

PID_FILE="cs-web-dev.pid"
LOG_FILE="cs-web-dev.log"

run_daemon() {
    if [ -f "$PID_FILE" ]; then
        oldpid="$(cat "$PID_FILE" 2>/dev/null)"
        if [ -n "$oldpid" ] && kill -0 "$oldpid" 2>/dev/null; then
            echo "开发服务已在运行 (PID $oldpid)。重启请先执行:"
            echo "  kill $oldpid && rm -f $PID_FILE && $0 daemon"
            exit 1
        fi
        rm -f "$PID_FILE"
    fi
    echo "后台启动开发服务器（关闭本窗口不会停止）..."
    echo "日志文件: $(pwd)/$LOG_FILE"
    nohup npm run dev >>"$LOG_FILE" 2>&1 &
    echo $! >"$PID_FILE"
    echo "PID: $(cat "$PID_FILE")"
    echo "访问: http://localhost:3000 （或 http://<本机IP>:3000）"
    echo "查看日志: tail -f $LOG_FILE"
    echo "停止服务: kill \$(cat $PID_FILE) && rm -f $PID_FILE"
}

echo "启动开发服务器..."
echo "访问地址: http://localhost:3000 （或 http://<本机IP>:3000，需放行防火墙/安全组）"
echo "（可选）AI 客服：在 cs-web 目录创建 .env.local，写入 DEEPSEEK_API_KEY=你的密钥"

if [ "$DAEMON" -eq 1 ]; then
    run_daemon
    exit 0
fi

echo "前台运行：关闭本终端窗口通常会停止服务；若需常驻请使用: $0 daemon"
echo "按 Ctrl+C 停止服务器"
echo
npm run dev
