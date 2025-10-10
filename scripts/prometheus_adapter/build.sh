#!/bin/bash

# Prometheus Adapter 打包脚本
# 将编译产物和必要文件打包到 build 目录

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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

# 项目根目录
PROJECT_ROOT=$(cd "$(dirname "$0")"/../.. && pwd)
cd "$PROJECT_ROOT"

# 配置
APP_NAME="prometheus_adapter"
BUILD_DIR="build/${APP_NAME}"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GOOS=${GOOS:-linux}
GOARCH=${GOARCH:-amd64}

log_info "开始构建 ${APP_NAME}"
log_info "版本: ${VERSION}"
log_info "构建时间: ${BUILD_TIME}"
log_info "目标系统: ${GOOS}/${GOARCH}"

# 清理旧的构建目录
if [ -d "$BUILD_DIR" ]; then
    log_warn "清理旧的构建目录..."
    rm -rf "$BUILD_DIR"
fi

# 创建构建目录
log_info "创建构建目录..."
mkdir -p "$BUILD_DIR/bin"
mkdir -p "$BUILD_DIR/config"
mkdir -p "$BUILD_DIR/docs"
mkdir -p "$BUILD_DIR/scripts"
mkdir -p "$BUILD_DIR/rules"

# 编译二进制文件
log_info "编译 ${APP_NAME}..."
LDFLAGS="-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"
CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
    -ldflags "$LDFLAGS" \
    -o "$BUILD_DIR/bin/${APP_NAME}" \
    "./cmd/${APP_NAME}"

if [ $? -ne 0 ]; then
    log_error "编译失败"
    exit 1
fi

# 复制配置文件
log_info "复制配置文件..."
if [ -f "internal/${APP_NAME}/config/prometheus_adapter.yml" ]; then
    cp "internal/${APP_NAME}/config/prometheus_adapter.yml" "$BUILD_DIR/config/"
    log_info "已复制配置文件到 $BUILD_DIR/config/"
else
    log_warn "未找到配置文件，使用默认配置"
fi

# 复制文档
log_info "复制文档..."
if [ -f "docs/${APP_NAME}/README.md" ]; then
    cp "docs/${APP_NAME}/README.md" "$BUILD_DIR/docs/"
fi

# 复制测试脚本
log_info "复制脚本..."
if [ -f "internal/${APP_NAME}/test_alert_update.sh" ]; then
    cp "internal/${APP_NAME}/test_alert_update.sh" "$BUILD_DIR/scripts/"
    chmod +x "$BUILD_DIR/scripts/test_alert_update.sh"
fi

# 复制规则文件
log_info "复制规则文件..."
if [ -d "internal/${APP_NAME}/rules" ]; then
    cp -r "internal/${APP_NAME}/rules/"* "$BUILD_DIR/rules/" 2>/dev/null || true
    log_info "已复制规则文件到 $BUILD_DIR/rules/"
else
    # 如果没有规则文件夹，创建一个空的规则文件
    log_warn "未找到规则目录，创建默认规则文件..."
    cat > "$BUILD_DIR/rules/alert_rules.yml" << 'RULES_EOF'
# Prometheus Alert Rules
# This file is managed by the Prometheus Adapter service
# It will be loaded on startup and saved on shutdown

groups: []
RULES_EOF
fi

# 创建启动脚本
log_info "创建启动脚本..."
cat > "$BUILD_DIR/start.sh" << 'EOF'
#!/bin/bash

# Prometheus Adapter 启动脚本

# 获取脚本所在目录
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
BIN_PATH="$SCRIPT_DIR/bin/prometheus_adapter"
CONFIG_FILE="$SCRIPT_DIR/config/prometheus_adapter.yml"
PID_FILE="$SCRIPT_DIR/prometheus_adapter.pid"
LOG_FILE="$SCRIPT_DIR/prometheus_adapter.log"

# 检查二进制文件
if [ ! -f "$BIN_PATH" ]; then
    echo "错误: 找不到可执行文件 $BIN_PATH"
    exit 1
fi

# 检查是否已在运行
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo "Prometheus Adapter已在运行 (PID: $PID)"
        exit 1
    else
        rm -f "$PID_FILE"
    fi
fi

# 检查配置文件
if [ -f "$CONFIG_FILE" ]; then
    echo "使用配置文件: $CONFIG_FILE"
else
    echo "警告: 找不到配置文件 $CONFIG_FILE，将使用默认配置"
fi

# 环境变量（可选，用于覆盖配置文件）
# export PROMETHEUS_ADDRESS="http://localhost:9090"
# export ALERT_WEBHOOK_URL="http://alert-module:8080/v1/integrations/alertmanager/webhook"
# export ALERT_POLLING_INTERVAL="10s"
# export SERVER_BIND_ADDR="0.0.0.0:9999"

echo "启动 Prometheus Adapter..."

# 切换到脚本目录
cd "$SCRIPT_DIR"

# 后台启动服务
nohup "$BIN_PATH" > "$LOG_FILE" 2>&1 &
PID=$!

# 保存PID
echo $PID > "$PID_FILE"

echo "Prometheus Adapter已启动"
echo "PID: $PID"
echo "日志文件: $LOG_FILE"
echo "PID文件: $PID_FILE"
echo ""
echo "查看日志: tail -f $LOG_FILE"
echo "停止服务: ./stop.sh"
EOF
chmod +x "$BUILD_DIR/start.sh"

# 创建停止脚本
log_info "创建停止脚本..."
cat > "$BUILD_DIR/stop.sh" << 'EOF'
#!/bin/bash

# Prometheus Adapter 停止脚本

# 获取脚本所在目录
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PID_FILE="$SCRIPT_DIR/prometheus_adapter.pid"
APP_NAME="prometheus_adapter"

# 优先从PID文件读取
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE" 2>/dev/null)
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        echo "从PID文件获取进程ID: $PID"
    else
        echo "PID文件中的进程已不存在，清理PID文件"
        rm -f "$PID_FILE"
        PID=""
    fi
else
    PID=""
fi

# 如果PID文件不存在或进程已死，通过进程名查找
if [ -z "$PID" ]; then
    PID=$(ps aux | grep -v grep | grep "$APP_NAME" | awk '{print $2}')
fi

if [ -z "$PID" ]; then
    echo "没有找到运行中的 $APP_NAME 进程"
    exit 0
fi

echo "停止 $APP_NAME (PID: $PID)..."
kill -TERM $PID 2>/dev/null || true

# 等待进程退出
count=0
while [ $count -lt 10 ] && ps -p "$PID" > /dev/null 2>&1; do
    sleep 1
    count=$((count + 1))
done

# 检查是否已退出
if ps -p "$PID" > /dev/null 2>&1; then
    echo "强制停止 $APP_NAME..."
    kill -KILL "$PID" 2>/dev/null || true
fi

# 清理PID文件
if [ -f "$PID_FILE" ]; then
    rm -f "$PID_FILE"
fi

echo "$APP_NAME 已停止"
EOF
chmod +x "$BUILD_DIR/stop.sh"

# 创建版本信息文件
log_info "创建版本信息..."
cat > "$BUILD_DIR/VERSION" << EOF
Application: ${APP_NAME}
Version: ${VERSION}
Build Time: ${BUILD_TIME}
Build OS/Arch: ${GOOS}/${GOARCH}
EOF

# 打包成 tar.gz
ARCHIVE_NAME="${APP_NAME}_${VERSION}_${GOOS}_${GOARCH}.tar.gz"
log_info "创建归档文件: $ARCHIVE_NAME"
cd build
tar -czf "$ARCHIVE_NAME" "$APP_NAME"
cd ..

# 输出构建信息
log_info "构建成功!"
echo ""
echo "构建产物:"
echo "  - 目录: $BUILD_DIR"
echo "  - 归档: build/$ARCHIVE_NAME"
echo ""
echo "文件列表:"
ls -lah "$BUILD_DIR/"
echo ""
echo "归档大小:"
ls -lah "build/$ARCHIVE_NAME"