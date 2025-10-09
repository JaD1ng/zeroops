#!/bin/bash

# ZeroOps 部署脚本
set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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

# 检查Docker是否安装
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    if ! command -v docker compose &> /dev/null; then
        log_error "Docker Compose is not installed. Please install Docker Compose first."
        exit 1
    fi
    
    log_info "Docker and Docker Compose are installed."
}

# 检查环境变量文件
check_env() {
    if [ ! -f ".env.prod" ]; then
        log_warn ".env.prod file not found. Creating from template..."
        cat > .env.prod << EOF
# 数据库配置
DB_PASSWORD=your_secure_database_password

# 日志级别
LOG_LEVEL=info

# Prometheus配置
PROMETHEUS_URL=http://10.210.10.33:9090

# Ruleset API配置
RULESET_API_BASE=http://10.210.10.33:9999

# Webhook认证配置
WEBHOOK_USER=alert
WEBHOOK_PASS=your_secure_webhook_password
EOF
        log_warn "Please edit .env.prod file with your actual configuration before deploying."
        exit 1
    fi
    log_info "Environment file found."
}

# 构建镜像
build_images() {
    log_info "Building Docker images..."
    docker compose -f docker-compose.prod.yml build --no-cache
    log_info "Images built successfully."
}

# 启动服务
start_services() {
    log_info "Starting services..."
    docker compose -f docker-compose.prod.yml --env-file .env.prod up -d
    log_info "Services started successfully."
}

# 检查服务状态
check_services() {
    log_info "Checking service status..."
    docker compose -f docker-compose.prod.yml ps
    
    log_info "Waiting for services to be ready..."
    sleep 10
    
    # 检查健康状态
    if docker compose -f docker-compose.prod.yml ps | grep -q "unhealthy"; then
        log_error "Some services are unhealthy. Check logs with: docker compose -f docker-compose.prod.yml logs"
        exit 1
    fi
    
    log_info "All services are healthy."
}

# 显示访问信息
show_access_info() {
    log_info "Deployment completed successfully!"
    echo ""
    echo "Service URLs:"
    echo "  - ZeroOps API: http://localhost:8080"
    echo "  - Alerting ML: http://localhost:8081"
    echo "  - Redis: localhost:6379"
    echo "  - PostgreSQL: localhost:5432"
    echo ""
    echo "Useful commands:"
    echo "  - View logs: docker compose -f docker-compose.prod.yml logs -f"
    echo "  - Stop services: docker compose -f docker-compose.prod.yml down"
    echo "  - Restart services: docker compose -f docker-compose.prod.yml restart"
    echo "  - Update services: docker compose -f docker-compose.prod.yml pull && docker compose -f docker-compose.prod.yml up -d"
}

# 主函数
main() {
    log_info "Starting ZeroOps deployment..."
    
    check_docker
    check_env
    build_images
    start_services
    check_services
    show_access_info
}

# 脚本参数处理
case "${1:-}" in
    "build")
        check_docker
        build_images
        ;;
    "start")
        check_docker
        check_env
        start_services
        check_services
        ;;
    "stop")
        log_info "Stopping services..."
        docker compose -f docker-compose.prod.yml down
        ;;
    "restart")
        log_info "Restarting services..."
        docker compose -f docker-compose.prod.yml restart
        ;;
    "logs")
        docker compose -f docker-compose.prod.yml logs -f
        ;;
    "status")
        docker compose -f docker-compose.prod.yml ps
        ;;
    *)
        main
        ;;
esac
