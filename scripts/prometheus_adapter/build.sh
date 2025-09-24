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
mkdir -p "$BUILD_DIR/docs"
mkdir -p "$BUILD_DIR/scripts"

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

# 创建启动脚本
log_info "创建启动脚本..."
cat > "$BUILD_DIR/start.sh" << 'EOF'
#!/bin/bash

# Prometheus Adapter 启动脚本

# 默认配置
PROMETHEUS_URL=${PROMETHEUS_URL:-"http://localhost:9090"}
PORT=${PORT:-8080}
LOG_LEVEL=${LOG_LEVEL:-"info"}

# 获取脚本所在目录
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
BIN_PATH="$SCRIPT_DIR/bin/prometheus_adapter"

# 检查二进制文件
if [ ! -f "$BIN_PATH" ]; then
    echo "错误: 找不到可执行文件 $BIN_PATH"
    exit 1
fi

# 启动参数
ARGS=""
ARGS="$ARGS --prometheus-url=$PROMETHEUS_URL"
ARGS="$ARGS --port=$PORT"
ARGS="$ARGS --log-level=$LOG_LEVEL"

echo "启动 Prometheus Adapter..."
echo "Prometheus URL: $PROMETHEUS_URL"
echo "监听端口: $PORT"
echo "日志级别: $LOG_LEVEL"

# 启动服务
exec "$BIN_PATH" $ARGS
EOF
chmod +x "$BUILD_DIR/start.sh"

# 创建停止脚本
log_info "创建停止脚本..."
cat > "$BUILD_DIR/stop.sh" << 'EOF'
#!/bin/bash

# Prometheus Adapter 停止脚本

APP_NAME="prometheus_adapter"

# 查找进程
PID=$(ps aux | grep -v grep | grep "$APP_NAME" | awk '{print $2}')

if [ -z "$PID" ]; then
    echo "没有找到运行中的 $APP_NAME 进程"
    exit 0
fi

echo "停止 $APP_NAME (PID: $PID)..."
kill -TERM $PID

# 等待进程退出
sleep 2

# 检查是否还在运行
if ps -p $PID > /dev/null 2>&1; then
    echo "强制停止进程..."
    kill -KILL $PID
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