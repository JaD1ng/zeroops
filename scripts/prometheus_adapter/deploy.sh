#!/bin/bash

# Prometheus Adapter 部署脚本
# 将打包好的文件解压并部署到指定目录

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

# 显示使用帮助
show_usage() {
    cat << EOF
使用方法:
    $0 [选项] <归档文件>

选项:
    -d, --deploy-dir DIR    指定部署目录 (默认: /home/qboxserver/zeroops_prometheus_adapter)
    -b, --backup            部署前备份现有目录
    -s, --start             部署后自动启动服务
    -r, --restart           如果服务已运行则重启
    -f, --force             强制部署，不询问确认
    -h, --help              显示此帮助信息

示例:
    $0 prometheus_adapter_v1.0.0_linux_amd64.tar.gz
    $0 -d /opt/prometheus_adapter -b -s prometheus_adapter.tar.gz
    $0 --backup --restart prometheus_adapter.tar.gz

EOF
    exit 0
}

# 默认配置
DEPLOY_DIR="/home/qboxserver/zeroops_prometheus_adapter"
BACKUP=false
START_SERVICE=false
RESTART_SERVICE=false
FORCE_DEPLOY=false
ARCHIVE_FILE=""

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -d|--deploy-dir)
            DEPLOY_DIR="$2"
            shift 2
            ;;
        -b|--backup)
            BACKUP=true
            shift
            ;;
        -s|--start)
            START_SERVICE=true
            shift
            ;;
        -r|--restart)
            RESTART_SERVICE=true
            shift
            ;;
        -f|--force)
            FORCE_DEPLOY=true
            shift
            ;;
        -h|--help)
            show_usage
            ;;
        *)
            if [ -z "$ARCHIVE_FILE" ]; then
                ARCHIVE_FILE="$1"
            else
                log_error "未知参数: $1"
                show_usage
            fi
            shift
            ;;
    esac
done

# 检查归档文件参数
if [ -z "$ARCHIVE_FILE" ]; then
    log_error "请指定要部署的归档文件"
    show_usage
fi

# 检查归档文件是否存在
if [ ! -f "$ARCHIVE_FILE" ]; then
    log_error "找不到归档文件: $ARCHIVE_FILE"
    exit 1
fi

# 获取归档文件的绝对路径
ARCHIVE_FILE=$(realpath "$ARCHIVE_FILE")

log_info "部署配置:"
log_info "  归档文件: $ARCHIVE_FILE"
log_info "  部署目录: $DEPLOY_DIR"
log_info "  备份现有: $BACKUP"
log_info "  自动启动: $START_SERVICE"
log_info "  重启服务: $RESTART_SERVICE"

# 确认部署
if [ "$FORCE_DEPLOY" = false ]; then
    echo -n "确认部署? (y/N): "
    read -r CONFIRM
    if [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ]; then
        log_warn "部署已取消"
        exit 0
    fi
fi

# 检查是否有运行中的服务
check_running_service() {
    # 优先从PID文件读取
    if [ -f "$DEPLOY_DIR/prometheus_adapter.pid" ]; then
        local pid=$(cat "$DEPLOY_DIR/prometheus_adapter.pid" 2>/dev/null)
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            echo "$pid"
            return
        fi
    fi

    # 如果PID文件不存在或进程已死，通过进程名查找
    local pid=$(ps aux | grep -v grep | grep "prometheus_adapter" | grep -v "$0" | awk '{print $2}')
    if [ -n "$pid" ]; then
        echo "$pid"
    fi
}

# 停止运行中的服务
stop_service() {
    local pid=$1
    if [ -n "$pid" ]; then
        log_warn "停止运行中的服务 (PID: $pid)..."
        kill -TERM "$pid" 2>/dev/null || true

        # 等待进程退出
        local count=0
        while [ $count -lt 10 ] && ps -p "$pid" > /dev/null 2>&1; do
            sleep 1
            count=$((count + 1))
        done

        # 如果还没退出，强制停止
        if ps -p "$pid" > /dev/null 2>&1; then
            log_warn "强制停止进程..."
            kill -KILL "$pid" 2>/dev/null || true
        fi

        # 清理PID文件
        if [ -f "$DEPLOY_DIR/prometheus_adapter.pid" ]; then
            rm -f "$DEPLOY_DIR/prometheus_adapter.pid"
        fi

        log_info "服务已停止"
    fi
}

# 检查运行中的服务
RUNNING_PID=$(check_running_service)
if [ -n "$RUNNING_PID" ]; then
    log_warn "检测到运行中的 prometheus_adapter 服务 (PID: $RUNNING_PID)"
    if [ "$RESTART_SERVICE" = true ] || [ "$FORCE_DEPLOY" = true ]; then
        stop_service "$RUNNING_PID"
    else
        log_error "服务正在运行，请先停止服务或使用 -r/--restart 选项"
        exit 1
    fi
fi

# 备份现有目录
if [ "$BACKUP" = true ] && [ -d "$DEPLOY_DIR" ]; then
    BACKUP_DIR="${DEPLOY_DIR}_backup_$(date +%Y%m%d_%H%M%S)"
    log_info "备份现有目录到: $BACKUP_DIR"

    # 需要sudo权限
    if [ -w "$(dirname "$DEPLOY_DIR")" ]; then
        mv "$DEPLOY_DIR" "$BACKUP_DIR"
    else
        log_warn "需要管理员权限来备份目录"
        sudo mv "$DEPLOY_DIR" "$BACKUP_DIR"
    fi
fi

# 创建临时解压目录
TEMP_DIR=$(mktemp -d)
log_info "创建临时目录: $TEMP_DIR"

# 解压归档文件
log_info "解压归档文件..."
tar -xzf "$ARCHIVE_FILE" -C "$TEMP_DIR"

# 查找解压后的目录
EXTRACTED_DIR=$(find "$TEMP_DIR" -maxdepth 1 -type d -name "prometheus_adapter" | head -n 1)
if [ -z "$EXTRACTED_DIR" ]; then
    log_error "解压失败：找不到 prometheus_adapter 目录"
    rm -rf "$TEMP_DIR"
    exit 1
fi

# 创建部署目录（如果需要sudo）
log_info "创建部署目录..."
if [ -w "$(dirname "$DEPLOY_DIR")" ]; then
    mkdir -p "$(dirname "$DEPLOY_DIR")"
else
    log_warn "需要管理员权限来创建部署目录"
    sudo mkdir -p "$(dirname "$DEPLOY_DIR")"
fi

# 移动到部署目录
log_info "部署到: $DEPLOY_DIR"
if [ -w "$(dirname "$DEPLOY_DIR")" ]; then
    if [ -d "$DEPLOY_DIR" ]; then
        rm -rf "$DEPLOY_DIR"
    fi
    mv "$EXTRACTED_DIR" "$DEPLOY_DIR"
else
    log_warn "需要管理员权限来部署"
    if [ -d "$DEPLOY_DIR" ]; then
        sudo rm -rf "$DEPLOY_DIR"
    fi
    sudo mv "$EXTRACTED_DIR" "$DEPLOY_DIR"
fi

# 设置权限
log_info "设置文件权限..."
if [ -w "$DEPLOY_DIR" ]; then
    chmod +x "$DEPLOY_DIR/bin/prometheus_adapter"
    chmod +x "$DEPLOY_DIR/start.sh"
    chmod +x "$DEPLOY_DIR/stop.sh"
    [ -f "$DEPLOY_DIR/scripts/test_alert_update.sh" ] && chmod +x "$DEPLOY_DIR/scripts/test_alert_update.sh"
    # 确保 config 目录和配置文件可读
    chmod 755 "$DEPLOY_DIR/config"
    [ -f "$DEPLOY_DIR/config/prometheus_adapter.yml" ] && chmod 644 "$DEPLOY_DIR/config/prometheus_adapter.yml"
    # 确保 rules 目录可写
    chmod 755 "$DEPLOY_DIR/rules"
    [ -f "$DEPLOY_DIR/rules/alert_rules.yml" ] && chmod 644 "$DEPLOY_DIR/rules/alert_rules.yml"
else
    sudo chmod +x "$DEPLOY_DIR/bin/prometheus_adapter"
    sudo chmod +x "$DEPLOY_DIR/start.sh"
    sudo chmod +x "$DEPLOY_DIR/stop.sh"
    [ -f "$DEPLOY_DIR/scripts/test_alert_update.sh" ] && sudo chmod +x "$DEPLOY_DIR/scripts/test_alert_update.sh"
    # 确保 config 目录和配置文件可读
    sudo chmod 755 "$DEPLOY_DIR/config"
    [ -f "$DEPLOY_DIR/config/prometheus_adapter.yml" ] && sudo chmod 644 "$DEPLOY_DIR/config/prometheus_adapter.yml"
    # 确保 rules 目录可写
    sudo chmod 755 "$DEPLOY_DIR/rules"
    [ -f "$DEPLOY_DIR/rules/alert_rules.yml" ] && sudo chmod 644 "$DEPLOY_DIR/rules/alert_rules.yml"
    # 设置 rules 目录的所有者为服务运行用户
    sudo chown -R qboxserver:qboxserver "$DEPLOY_DIR/rules"
    # 确保配置文件也可以被服务用户读取
    sudo chown qboxserver:qboxserver "$DEPLOY_DIR/config/prometheus_adapter.yml"
fi

# 清理临时目录
rm -rf "$TEMP_DIR"

# 显示部署信息
log_info "部署成功!"
echo ""
echo "部署信息:"
echo "  目录: $DEPLOY_DIR"
echo ""
echo "版本信息:"
if [ -f "$DEPLOY_DIR/VERSION" ]; then
    cat "$DEPLOY_DIR/VERSION"
else
    echo "  无版本信息"
fi
echo ""
echo "文件列表:"
ls -lah "$DEPLOY_DIR/"

# 创建systemd服务文件（可选）
create_systemd_service() {
    local service_name="prometheus-adapter"
    local service_file="/etc/systemd/system/${service_name}.service"

    log_info "创建 systemd 服务..."

    cat << EOF | sudo tee "$service_file" > /dev/null
[Unit]
Description=Prometheus Adapter Service
After=network.target

[Service]
Type=simple
User=qboxserver
Group=qboxserver
WorkingDirectory=$DEPLOY_DIR
# 可选：通过环境变量覆盖配置
#Environment="PROMETHEUS_ADDRESS=http://localhost:9090"
#Environment="ALERT_WEBHOOK_URL=http://alert-module:8080/v1/integrations/alertmanager/webhook"
#Environment="ALERT_POLLING_INTERVAL=10s"
#Environment="SERVER_BIND_ADDR=0.0.0.0:9999"
ExecStart=$DEPLOY_DIR/bin/prometheus_adapter
ExecStop=$DEPLOY_DIR/stop.sh
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    log_info "Systemd 服务已创建: ${service_name}.service"
    echo ""
    echo "可以使用以下命令管理服务:"
    echo "  启动: sudo systemctl start ${service_name}"
    echo "  停止: sudo systemctl stop ${service_name}"
    echo "  重启: sudo systemctl restart ${service_name}"
    echo "  状态: sudo systemctl status ${service_name}"
    echo "  开机自启: sudo systemctl enable ${service_name}"
}

# 询问是否创建systemd服务
if [ "$FORCE_DEPLOY" = false ]; then
    echo ""
    echo -n "是否创建 systemd 服务? (y/N): "
    read -r CREATE_SERVICE
    if [ "$CREATE_SERVICE" = "y" ] || [ "$CREATE_SERVICE" = "Y" ]; then
        create_systemd_service
    fi
fi

# 启动服务
if [ "$START_SERVICE" = true ] || [ "$RESTART_SERVICE" = true ]; then
    log_info "启动服务..."

    # 设置环境变量
    export PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"
    export PORT="${PORT:-8080}"
    export LOG_LEVEL="${LOG_LEVEL:-info}"

    # 启动服务
    cd "$DEPLOY_DIR"

    # 直接启动二进制文件而不是通过start.sh脚本
    nohup ./bin/prometheus_adapter > prometheus_adapter.log 2>&1 &
    PID=$!

    # 保存PID到文件
    echo $PID > prometheus_adapter.pid

    log_info "服务已启动 (PID: $PID)"
    echo "PID文件: $DEPLOY_DIR/prometheus_adapter.pid"
    echo "日志文件: $DEPLOY_DIR/prometheus_adapter.log"

    # 等待服务启动
    sleep 2

    # 检查是否启动成功
    if kill -0 "$PID" 2>/dev/null; then
        log_info "服务启动成功，正在运行"
        echo ""
        echo "查看日志: tail -f $DEPLOY_DIR/prometheus_adapter.log"
        echo "停止服务: kill \$(cat $DEPLOY_DIR/prometheus_adapter.pid)"
    else
        log_error "服务启动失败，请检查日志"
        exit 1
    fi
else
    echo ""
    echo "手动启动服务:"
    echo "  cd $DEPLOY_DIR"
    echo "  nohup ./bin/prometheus_adapter > prometheus_adapter.log 2>&1 &"
    echo "  echo \$! > prometheus_adapter.pid"
    echo ""
    echo "停止服务:"
    echo "  kill \$(cat prometheus_adapter.pid)"
fi

log_info "部署完成!"