#!/bin/bash

# 服务打包脚本 - 重写版本
# 专门用于打包storage-service，采用正确的Floyd部署路径

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
VERSION="v1.0.0"
SERVICE_NAME="storage"
BUILD_DIR=""
PACKAGE_DIR=""
TEMP_DIR=""

# 创建构建目录
create_build_dir() {
    log_info "创建构建目录..."
    BUILD_DIR=$(mktemp -d)
    PACKAGE_DIR="$BUILD_DIR/package"
    TEMP_DIR="$BUILD_DIR/temp"
    
    mkdir -p "$PACKAGE_DIR"
    mkdir -p "$TEMP_DIR"
    
    log_debug "构建目录: $BUILD_DIR"
    log_debug "包目录: $PACKAGE_DIR"
    log_debug "临时目录: $TEMP_DIR"
}

# 创建配置文件 - 使用正确的Floyd路径结构
create_config() {
    log_info "复制原版配置文件..."
    
    # 直接复制原版配置文件并重命名为config.yaml
    cp "mock/s3/services/storage/config/storage-config.yaml" "$PACKAGE_DIR/config.yaml"
    
    # 更新版本号（如果需要的话）
    sed -i "s/version: .*/version: $VERSION/" "$PACKAGE_DIR/config.yaml" 2>/dev/null || true
    
    log_debug "配置文件已复制: $PACKAGE_DIR/config.yaml"
}

# 创建启动脚本 - 使用正确的Floyd路径
create_start_script() {
    log_info "创建启动脚本..."
    
    cat > "$PACKAGE_DIR/start.sh" << 'EOF'
#!/bin/bash

# 服务启动脚本 - Floyd部署版本
set -e

# Floyd部署路径结构
# 服务目录: /home/qboxserver/storage/
# 版本目录: /home/qboxserver/storage/_fpkgs/版本号/
# 包目录: /home/qboxserver/storage/_package/

# 设置环境变量
export SERVICE_NAME="${SERVICE_NAME:-storage}"
export LOG_LEVEL="${LOG_LEVEL:-info}"

# Floyd部署的标准路径
FLOYD_SERVICE_DIR="/home/qboxserver/storage"
FLOYD_PACKAGE_DIR="/home/qboxserver/storage/_package"

# 切换到Floyd包目录（Floyd部署时包内容在_package目录）
cd "$FLOYD_PACKAGE_DIR" || { echo "错误: 无法切换到包目录 $FLOYD_PACKAGE_DIR"; exit 1; }

# 从_package目录的config.yaml读取端口
if [ -f "config.yaml" ]; then
    SERVICE_PORT=$(grep "port:" config.yaml | sed 's/.*port:\s*\([0-9]*\).*/\1/')
    if [ -z "$SERVICE_PORT" ]; then
        SERVICE_PORT="8080"
    fi
    echo "从_package/config.yaml读取到端口: $SERVICE_PORT"
else
    SERVICE_PORT="8080"
    echo "配置文件不存在，使用默认端口: $SERVICE_PORT"
fi
export SERVICE_PORT

# 检查配置文件
if [ ! -f "config.yaml" ]; then
    echo "错误: 配置文件 config.yaml 不存在"
    exit 1
fi

# 检查可执行文件
if [ ! -f "storage-service" ]; then
    echo "错误: 可执行文件 storage-service 不存在"
    exit 1
fi

# 设置权限
chmod +x "storage-service"

# 启动服务
echo "启动 $SERVICE_NAME 服务..."
echo "工作目录: $FLOYD_PACKAGE_DIR"
echo "端口: $SERVICE_PORT"
echo "日志级别: $LOG_LEVEL"

# 后台运行服务
nohup ./storage-service \
    --config=config.yaml \
    --port="$SERVICE_PORT" \
    --log-level="$LOG_LEVEL" \
    > storage.log 2>&1 &

# 保存PID
echo $! > storage.pid

echo "$SERVICE_NAME 服务已启动，PID: $(cat storage.pid)"

EOF

    chmod +x "$PACKAGE_DIR/start.sh"
    log_debug "启动脚本已创建: $PACKAGE_DIR/start.sh"
}

# 创建停止脚本
create_stop_script() {
    log_info "创建停止脚本..."
    
    cat > "$PACKAGE_DIR/stop.sh" << 'EOF'
#!/bin/bash

# 服务停止脚本
set -e

# Floyd部署路径结构
# 服务目录: /home/qboxserver/storage/
# 版本目录: /home/qboxserver/storage/_fpkgs/版本号/
# 包目录: /home/qboxserver/storage/_package/

# Floyd部署的标准路径
FLOYD_SERVICE_DIR="/home/qboxserver/storage"
FLOYD_PACKAGE_DIR="/home/qboxserver/storage/_package"

# 设置环境变量
export SERVICE_NAME="${SERVICE_NAME:-storage}"

# 切换到Floyd包目录（Floyd部署时包内容在_package目录）
cd "$FLOYD_PACKAGE_DIR" || { echo "错误: 无法切换到包目录 $FLOYD_PACKAGE_DIR"; exit 1; }

# 检查PID文件
if [ -f "storage.pid" ]; then
    PID=$(cat storage.pid)
    if kill -0 "$PID" 2>/dev/null; then
        echo "停止 $SERVICE_NAME 服务 (PID: $PID)..."
        kill "$PID"
        
        # 等待进程结束
        for i in {1..10}; do
            if ! kill -0 "$PID" 2>/dev/null; then
                echo "$SERVICE_NAME 服务已停止"
                rm -f storage.pid
                exit 0
            fi
            sleep 1
        done
        
        # 强制杀死进程
        echo "强制停止 $SERVICE_NAME 服务..."
        kill -9 "$PID" 2>/dev/null || true
        rm -f storage.pid
    else
        echo "$SERVICE_NAME 服务未运行"
        rm -f storage.pid
    fi
else
    echo "$SERVICE_NAME 服务未运行"
fi
EOF

    chmod +x "$PACKAGE_DIR/stop.sh"
    log_debug "停止脚本已创建: $PACKAGE_DIR/stop.sh"
}

# 创建可执行文件
create_executable() {
    log_info "创建可执行文件..."
    
    local executable_path="$PACKAGE_DIR/storage-service"
    
    # 创建一个简单的Go程序作为可执行文件
    cat > "$TEMP_DIR/main.go" << EOF
package main

import (
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    var (
        configFile = flag.String("config", "config.yaml", "配置文件路径")
        port       = flag.String("port", "8080", "服务端口")
        logLevel   = flag.String("log-level", "info", "日志级别")
    )
    flag.Parse()

    fmt.Printf("启动 storage 服务...\n")
    fmt.Printf("配置文件: %s\n", *configFile)
    fmt.Printf("端口: %s\n", *port)
    fmt.Printf("日志级别: %s\n", *logLevel)

    // 简单的HTTP服务器
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Hello from storage service!"))
    })

    server := &http.Server{
        Addr:    ":" + *port,
        Handler: nil,
    }

    // 启动服务器
    go func() {
        log.Printf("服务器启动在端口 %s", *port)
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("服务器启动失败: %v", err)
        }
    }()

    // 等待中断信号
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    <-c

    log.Println("正在关闭服务器...")
    server.Close()
    log.Println("服务器已关闭")
}
EOF

    # 编译Go程序（指定Linux目标平台）
    cd "$TEMP_DIR"
    go mod init temp 2>/dev/null || true
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$executable_path" main.go
    
    if [ -f "$executable_path" ]; then
        chmod +x "$executable_path"
        log_debug "可执行文件已创建: $executable_path"
    else
        log_error "可执行文件创建失败"
        exit 1
    fi
}

# 打包服务
package_service() {
    log_info "打包服务..."
    
    local package_name="storage-${VERSION}.tar.gz"
    local project_root="/Users/dingnanjia/workspace/mock/zeroops"
    local output_dir="$project_root/internal/deploy/packages"
    local package_path="$output_dir/$package_name"
    
    # 确保packages目录存在
    mkdir -p "$output_dir"
    
    # 调试信息
    log_debug "项目根目录: $project_root"
    log_debug "输出目录: $output_dir"
    log_debug "包路径: $package_path"
    log_debug "包目录: $PACKAGE_DIR"
    
    # 进入包目录
    cd "$PACKAGE_DIR"
    
    # 创建tar.gz包（使用绝对路径）
    tar -czf "$package_path" .
    
    if [ -f "$package_path" ]; then
        log_info "服务包已创建: $package_path"
        log_debug "包大小: $(du -h "$package_path" | cut -f1)"
    else
        log_error "服务包创建失败"
        exit 1
    fi
}

# 清理临时文件
cleanup() {
    log_info "清理临时文件..."
    if [ -n "$BUILD_DIR" ] && [ -d "$BUILD_DIR" ]; then
        rm -rf "$BUILD_DIR"
        log_debug "临时目录已清理: $BUILD_DIR"
    fi
}

# 主函数
main() {
    log_info "开始打包 storage 服务..."
    create_build_dir
    create_config
    create_start_script
    create_stop_script
    create_executable
    package_service
        cleanup
        
        log_info "服务打包完成！"
    log_info "部署包: storage-${VERSION}.tar.gz"
}

# 执行主函数
main "$@"