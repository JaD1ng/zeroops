#!/bin/bash

# Mock S3 服务打包脚本
# 用于打包 mock/s3 目录下的各个服务，采用 Floyd 部署路径

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

# 配置变量
VERSION="${VERSION:-v1.0.0}"
SERVICE_NAME="${1:-storage}"  # 从命令行参数获取服务名，默认为 storage
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MOCK_S3_DIR="$PROJECT_ROOT/mock/s3"
BUILD_DIR=""
PACKAGE_DIR=""

# 获取服务配置文件名
get_config_file() {
    case "$1" in
        metadata)    echo "metadata-config.yaml" ;;
        storage)     echo "storage-config.yaml" ;;
        queue)       echo "queue-config.yaml" ;;
        third-party) echo "third-party-config.yaml" ;;
        mock-error)  echo "mock-error-config.yaml" ;;
        *)           echo "" ;;
    esac
}

# 获取服务二进制名称
get_binary_name() {
    case "$1" in
        metadata)    echo "metadata-service" ;;
        storage)     echo "storage-service" ;;
        queue)       echo "queue-service" ;;
        third-party) echo "third-party-service" ;;
        mock-error)  echo "mock-error-service" ;;
        *)           echo "" ;;
    esac
}

# 验证服务名称
validate_service() {
    local config_file=$(get_config_file "$SERVICE_NAME")

    if [[ -z "$config_file" ]]; then
        log_error "不支持的服务名称: $SERVICE_NAME"
        log_info "支持的服务: metadata, storage, queue, third-party, mock-error"
        exit 1
    fi

    local service_dir="$MOCK_S3_DIR/services/$SERVICE_NAME"
    if [[ ! -d "$service_dir" ]]; then
        log_error "服务目录不存在: $service_dir"
        exit 1
    fi

    log_debug "服务验证通过: $SERVICE_NAME"
}

# 创建构建目录
create_build_dir() {
    log_info "创建构建目录..."
    BUILD_DIR=$(mktemp -d)
    PACKAGE_DIR="$BUILD_DIR/package"

    mkdir -p "$PACKAGE_DIR"

    log_debug "构建目录: $BUILD_DIR"
    log_debug "包目录: $PACKAGE_DIR"
}

# 创建配置文件
create_config() {
    log_info "复制服务配置文件..."

    local config_file=$(get_config_file "$SERVICE_NAME")
    local source_config="$MOCK_S3_DIR/services/$SERVICE_NAME/config/$config_file"

    if [[ ! -f "$source_config" ]]; then
        log_error "配置文件不存在: $source_config"
        exit 1
    fi

    # 复制配置文件并重命名为 config.yaml
    cp "$source_config" "$PACKAGE_DIR/config.yaml"

    # 更新版本号（如果配置中有 version 字段）
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s/version: .*/version: $VERSION/" "$PACKAGE_DIR/config.yaml" 2>/dev/null || true
    else
        sed -i "s/version: .*/version: $VERSION/" "$PACKAGE_DIR/config.yaml" 2>/dev/null || true
    fi

    log_debug "配置文件已复制: $PACKAGE_DIR/config.yaml"
    log_debug "源文件: $source_config"
}

# 创建启动脚本
create_start_script() {
    log_info "创建启动脚本..."

    local binary_name=$(get_binary_name "$SERVICE_NAME")

    cat > "$PACKAGE_DIR/start.sh" << EOF
#!/bin/bash

# 服务启动脚本 - Floyd部署版本
set -e

# Floyd部署路径结构
# 服务目录: /home/qboxserver/$SERVICE_NAME/
# 版本目录: /home/qboxserver/$SERVICE_NAME/_fpkgs/版本号/
# 包目录: /home/qboxserver/$SERVICE_NAME/_package/

# 设置环境变量
export SERVICE_NAME="\${SERVICE_NAME:-$SERVICE_NAME}"
export LOG_LEVEL="\${LOG_LEVEL:-info}"

# Floyd部署的标准路径
FLOYD_SERVICE_DIR="/home/qboxserver/$SERVICE_NAME"
FLOYD_PACKAGE_DIR="/home/qboxserver/$SERVICE_NAME/_package"

# 切换到Floyd包目录
cd "\$FLOYD_PACKAGE_DIR" || { echo "错误: 无法切换到包目录 \$FLOYD_PACKAGE_DIR"; exit 1; }

# 从config.yaml读取端口
if [ -f "config.yaml" ]; then
    SERVICE_PORT=\$(grep "port:" config.yaml | sed 's/.*port:[[:space:]]*\([0-9]*\).*/\1/')
    if [ -z "\$SERVICE_PORT" ]; then
        SERVICE_PORT="8080"
    fi
    echo "从config.yaml读取到端口: \$SERVICE_PORT"
else
    SERVICE_PORT="8080"
    echo "配置文件不存在，使用默认端口: \$SERVICE_PORT"
fi
export SERVICE_PORT

# 检查配置文件
if [ ! -f "config.yaml" ]; then
    echo "错误: 配置文件 config.yaml 不存在"
    exit 1
fi

# 检查可执行文件
if [ ! -f "$binary_name" ]; then
    echo "错误: 可执行文件 $binary_name 不存在"
    exit 1
fi

# 设置权限
chmod +x "$binary_name"

# 启动服务
echo "启动 \$SERVICE_NAME 服务..."
echo "工作目录: \$FLOYD_PACKAGE_DIR"
echo "端口: \$SERVICE_PORT"
echo "日志级别: \$LOG_LEVEL"

# 后台运行服务
nohup ./$binary_name > $SERVICE_NAME.log 2>&1 &

# 保存PID
echo \$! > $SERVICE_NAME.pid

echo "\$SERVICE_NAME 服务已启动，PID: \$(cat $SERVICE_NAME.pid)"

EOF

    chmod +x "$PACKAGE_DIR/start.sh"
    log_debug "启动脚本已创建: $PACKAGE_DIR/start.sh"
}

# 创建停止脚本
create_stop_script() {
    log_info "创建停止脚本..."

    cat > "$PACKAGE_DIR/stop.sh" << EOF
#!/bin/bash

# 服务停止脚本 - Floyd部署版本
set -e

# 设置环境变量
export SERVICE_NAME="\${SERVICE_NAME:-$SERVICE_NAME}"

# Floyd部署的标准路径
FLOYD_PACKAGE_DIR="/home/qboxserver/$SERVICE_NAME/_package"

# 切换到Floyd包目录
cd "\$FLOYD_PACKAGE_DIR" || { echo "错误: 无法切换到包目录 \$FLOYD_PACKAGE_DIR"; exit 1; }

# 检查PID文件
if [ -f "$SERVICE_NAME.pid" ]; then
    PID=\$(cat $SERVICE_NAME.pid)
    if kill -0 "\$PID" 2>/dev/null; then
        echo "停止 \$SERVICE_NAME 服务 (PID: \$PID)..."
        kill "\$PID"

        # 等待进程结束
        for i in {1..10}; do
            if ! kill -0 "\$PID" 2>/dev/null; then
                echo "\$SERVICE_NAME 服务已停止"
                rm -f $SERVICE_NAME.pid
                exit 0
            fi
            sleep 1
        done

        # 强制杀死进程
        echo "强制停止 \$SERVICE_NAME 服务..."
        kill -9 "\$PID" 2>/dev/null || true
        rm -f $SERVICE_NAME.pid
    else
        echo "\$SERVICE_NAME 服务未运行"
        rm -f $SERVICE_NAME.pid
    fi
else
    echo "\$SERVICE_NAME 服务未运行"
fi
EOF

    chmod +x "$PACKAGE_DIR/stop.sh"
    log_debug "停止脚本已创建: $PACKAGE_DIR/stop.sh"
}

# 编译Go服务
build_service() {
    log_info "编译 $SERVICE_NAME 服务..."

    local service_dir="$MOCK_S3_DIR/services/$SERVICE_NAME"
    local binary_name=$(get_binary_name "$SERVICE_NAME")
    local binary_path="$PACKAGE_DIR/$binary_name"

    # 切换到 mock/s3 目录（因为 go.mod 在那里）
    cd "$MOCK_S3_DIR"

    # 编译服务（指定Linux目标平台）
    log_debug "编译命令: CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $binary_path ./services/$SERVICE_NAME/cmd"

    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -o "$binary_path" \
        ./services/$SERVICE_NAME/cmd

    if [ -f "$binary_path" ]; then
        chmod +x "$binary_path"
        log_debug "可执行文件已创建: $binary_path"
        log_debug "文件大小: $(du -h "$binary_path" | cut -f1)"
    else
        log_error "可执行文件创建失败"
        exit 1
    fi

    # 返回原目录
    cd - > /dev/null
}

# 打包服务
package_service() {
    log_info "打包服务..."

    local package_name="$SERVICE_NAME-${VERSION}.tar.gz"
    local output_dir="$PROJECT_ROOT/internal/deploy/packages"
    local package_path="$output_dir/$package_name"

    # 确保packages目录存在
    mkdir -p "$output_dir"

    # 调试信息
    log_debug "项目根目录: $PROJECT_ROOT"
    log_debug "输出目录: $output_dir"
    log_debug "包路径: $package_path"
    log_debug "包目录: $PACKAGE_DIR"

    # 进入包目录
    cd "$PACKAGE_DIR"

    # 创建tar.gz包
    tar -czf "$package_path" .

    if [ -f "$package_path" ]; then
        log_info "服务包已创建: $package_path"
        log_debug "包大小: $(du -h "$package_path" | cut -f1)"

        # 显示包内容
        log_debug "包内容:"
        tar -tzf "$package_path" | while read -r file; do
            log_debug "  - $file"
        done
    else
        log_error "服务包创建失败"
        exit 1
    fi

    # 返回原目录
    cd - > /dev/null
}

# 清理临时文件
cleanup() {
    log_info "清理临时文件..."
    if [ -n "$BUILD_DIR" ] && [ -d "$BUILD_DIR" ]; then
        rm -rf "$BUILD_DIR"
        log_debug "临时目录已清理: $BUILD_DIR"
    fi
}

# 显示使用说明
show_usage() {
    cat << EOF
Mock S3 服务打包脚本

用法:
    $0 [服务名] [选项]

服务名:
    metadata      - 元数据服务 (默认端口: 8081)
    storage       - 存储服务 (默认端口: 8082) [默认]
    queue         - 队列服务 (默认端口: 8083)
    third-party   - 第三方服务 (默认端口: 8084)
    mock-error    - 故障注入服务 (默认端口: 8085)

环境变量:
    VERSION       - 版本号 (默认: v1.0.0)

示例:
    # 打包 storage 服务
    $0 storage

    # 打包 metadata 服务，指定版本
    VERSION=v1.2.0 $0 metadata

    # 打包所有服务
    for svc in metadata storage queue third-party mock-error; do
        $0 \$svc
    done

输出:
    internal/deploy/packages/[服务名]-[版本号].tar.gz

部署包结构:
    ├── config.yaml           # 服务配置文件
    ├── [服务名]-service       # 可执行文件
    ├── start.sh              # 启动脚本
    └── stop.sh               # 停止脚本

Floyd 部署路径:
    /home/qboxserver/[服务名]/_package/

EOF
}

# 主函数
main() {
    # 检查帮助参数
    if [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
        show_usage
        exit 0
    fi

    log_info "=========================================="
    log_info "Mock S3 服务打包工具"
    log_info "=========================================="
    log_info "服务名称: $SERVICE_NAME"
    log_info "版本号: $VERSION"
    log_info "项目根目录: $PROJECT_ROOT"
    log_info "=========================================="

    validate_service
    create_build_dir
    create_config
    create_start_script
    create_stop_script
    build_service
    package_service
    cleanup

    log_info "=========================================="
    log_info "✅ 服务打包完成！"
    log_info "部署包: internal/deploy/packages/$SERVICE_NAME-${VERSION}.tar.gz"
    log_info "=========================================="
}

# 执行主函数
main "$@"
