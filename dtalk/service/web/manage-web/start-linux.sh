#!/bin/bash

# 群组管理系统 Web 界面 - Linux 启动脚本
# 支持后台运行、日志记录、进程管理

set -e

# 配置
PORT=${PORT:-8000}
HOST=${HOST:-0.0.0.0}
LOG_FILE="web.log"
PID_FILE="web.pid"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 Python 版本
check_python() {
    if ! command -v python3 &> /dev/null; then
        log_error "Python3 未安装，请先安装 Python 3.6+"
        exit 1
    fi
    
    PYTHON_VERSION=$(python3 -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')")
    log_info "检测到 Python 版本: $PYTHON_VERSION"
    
    if ! python3 -c "import sys; exit(0 if sys.version_info >= (3, 6) else 1)" 2>/dev/null; then
        log_error "Python 版本过低，需要 3.6+，当前版本: $PYTHON_VERSION"
        exit 1
    fi
}

# 检查端口是否被占用
check_port() {
    if lsof -i :$PORT &> /dev/null; then
        log_warning "端口 $PORT 已被占用"
        if [ -f "$PID_FILE" ]; then
            OLD_PID=$(cat "$PID_FILE")
            if kill -0 $OLD_PID 2>/dev/null; then
                log_info "发现旧进程 PID: $OLD_PID，正在停止..."
                kill -9 $OLD_PID 2>/dev/null || true
                rm -f "$PID_FILE"
                sleep 2
            else
                rm -f "$PID_FILE"
            fi
        fi
    fi
}

# 启动服务
start_server() {
    log_info "启动群组管理系统 Web 界面..."
    log_info "端口: $PORT"
    log_info "主机: $HOST"
    log_info "日志文件: $LOG_FILE"
    
    # 检查必要文件
    if [ ! -f "index.html" ]; then
        log_error "未找到 index.html，请确保在正确的目录中运行此脚本"
        exit 1
    fi
    
    # 启动 Python HTTP 服务器
    nohup python3 -m http.server $PORT --bind $HOST > "$LOG_FILE" 2>&1 &
    SERVER_PID=$!
    
    # 保存 PID
    echo $SERVER_PID > "$PID_FILE"
    
    # 等待服务启动
    sleep 2
    
    # 检查服务是否正常启动
    if kill -0 $SERVER_PID 2>/dev/null; then
        log_success "Web 服务启动成功！"
        log_success "访问地址: http://$HOST:$PORT"
        log_success "进程 PID: $SERVER_PID"
        log_info "使用 './start-linux.sh stop' 停止服务"
        log_info "使用 './start-linux.sh status' 查看状态"
        log_info "使用 './start-linux.sh logs' 查看日志"
    else
        log_error "Web 服务启动失败"
        rm -f "$PID_FILE"
        exit 1
    fi
}

# 停止服务
stop_server() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 $PID 2>/dev/null; then
            log_info "正在停止 Web 服务 (PID: $PID)..."
            kill -TERM $PID 2>/dev/null || true
            sleep 2
            if kill -0 $PID 2>/dev/null; then
                log_warning "强制停止进程..."
                kill -9 $PID 2>/dev/null || true
            fi
            log_success "Web 服务已停止"
        else
            log_warning "进程不存在或已停止"
        fi
        rm -f "$PID_FILE"
    else
        log_warning "未找到 PID 文件，尝试查找并停止相关进程..."
        PIDS=$(pgrep -f "python3 -m http.server $PORT" || true)
        if [ -n "$PIDS" ]; then
            echo $PIDS | xargs kill -9 2>/dev/null || true
            log_success "已停止相关进程"
        else
            log_warning "未找到相关进程"
        fi
    fi
}

# 查看状态
show_status() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 $PID 2>/dev/null; then
            log_success "Web 服务正在运行 (PID: $PID)"
            log_info "端口: $PORT"
            log_info "访问地址: http://$HOST:$PORT"
            
            # 显示进程信息
            ps -p $PID -o pid,ppid,cmd,etime,pcpu,pmem
        else
            log_warning "PID 文件存在但进程未运行"
            rm -f "$PID_FILE"
        fi
    else
        log_warning "Web 服务未运行"
    fi
}

# 查看日志
show_logs() {
    if [ -f "$LOG_FILE" ]; then
        log_info "显示最近 50 行日志:"
        tail -n 50 "$LOG_FILE"
    else
        log_warning "日志文件不存在: $LOG_FILE"
    fi
}

# 重启服务
restart_server() {
    log_info "重启 Web 服务..."
    stop_server
    sleep 1
    start_server
}

# 显示帮助
show_help() {
    echo "群组管理系统 Web 界面启动脚本"
    echo ""
    echo "用法: $0 [命令]"
    echo ""
    echo "命令:"
    echo "  start     启动服务 (默认)"
    echo "  stop      停止服务"
    echo "  restart   重启服务"
    echo "  status    查看状态"
    echo "  logs      查看日志"
    echo "  help      显示帮助"
    echo ""
    echo "环境变量:"
    echo "  PORT      端口号 (默认: 8000)"
    echo "  HOST      绑定地址 (默认: 0.0.0.0)"
    echo ""
    echo "示例:"
    echo "  $0                    # 启动服务"
    echo "  $0 start              # 启动服务"
    echo "  $0 stop               # 停止服务"
    echo "  PORT=9000 $0 start    # 在端口 9000 启动"
}

# 主逻辑
main() {
    case "${1:-start}" in
        start)
            check_python
            check_port
            start_server
            ;;
        stop)
            stop_server
            ;;
        restart)
            restart_server
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "未知命令: $1"
            show_help
            exit 1
            ;;
    esac
}

# 捕获中断信号
trap 'log_info "收到中断信号，正在停止服务..."; stop_server; exit 0' INT TERM

# 执行主逻辑
main "$@"
