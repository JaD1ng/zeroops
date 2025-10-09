#!/bin/bash

# ZeroOps 部署测试脚本
# 用于测试 storage 服务的部署功能

set -e

# 配置参数
SERVICE_NAME="storage"
BASE_URL="http://localhost:8080"
PACKAGE_PATH="/Users/dingnanjia/workspace/mock/zeroops/internal/deploy/packages"

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

# 检查服务是否运行
check_service() {
    log_info "检查 ZeroOps 服务状态..."
    if curl -s "${BASE_URL}/v1/services" > /dev/null; then
        log_info "ZeroOps 服务运行正常"
    else
        log_error "ZeroOps 服务未运行，请先启动服务"
        exit 1
    fi
}

# 部署服务
deploy_service() {
    local version="$1"
    local package_file="$2"
    local instance_count="${3:-2}"

    log_info "开始部署 ${SERVICE_NAME} 版本 ${version}..."

    # 检查包文件是否存在
    if [[ ! -f "$package_file" ]]; then
        log_error "包文件不存在: $package_file"
        return 1
    fi

    # 获取当前时间作为调度时间
    local schedule_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # 构建请求数据
    local request_data=$(cat <<EOF
{
    "service": "${SERVICE_NAME}",
    "version": "${version}",
    "scheduleTime": "${schedule_time}",
    "packageURL": "${package_file}",
    "instanceCount": ${instance_count}
}
EOF
)

    log_info "发送部署请求..."
    echo "请求数据: $request_data"

    # 发送部署请求
    local response=$(curl -s -X POST "${BASE_URL}/v1/deployments" \
        -H "Content-Type: application/json" \
        -d "$request_data")

    if [[ $? -eq 0 ]]; then
        log_info "部署请求发送成功"
        echo "响应: $response"
        return 0
    else
        log_error "部署请求发送失败"
        return 1
    fi
}

# 查看部署状态
check_deployment_status() {
    local service="$1"
    local version="$2"

    log_info "查询部署状态: ${service} ${version}"

    local response=$(curl -s -X GET "${BASE_URL}/v1/deployments/${service}/${version}")

    if [[ $? -eq 0 ]]; then
        echo "部署状态: $response"
    else
        log_error "查询部署状态失败"
    fi
}

# 列出所有部署
list_all_deployments() {
    log_info "查询所有部署任务..."

    local response=$(curl -s -X GET "${BASE_URL}/v1/deployments")

    if [[ $? -eq 0 ]]; then
        echo "所有部署: $response" | jq '.' 2>/dev/null || echo "$response"
    else
        log_error "查询部署列表失败"
    fi
}

# 帮助信息
show_help() {
    echo "用法: $0 [命令] [参数]"
    echo ""
    echo "命令:"
    echo "  deploy <version> [instance_count]  - 部署指定版本 (默认2个实例)"
    echo "  status <version>                   - 查看指定版本的部署状态"
    echo "  list                              - 列出所有部署任务"
    echo "  check                             - 检查服务状态"
    echo ""
    echo "示例:"
    echo "  $0 deploy v1.0.2                 - 部署 v1.0.2 版本，2个实例"
    echo "  $0 deploy v1.0.3 3               - 部署 v1.0.3 版本，3个实例"
    echo "  $0 status v1.0.2                 - 查看 v1.0.2 的部署状态"
    echo "  $0 list                           - 查看所有部署"
    echo ""
    echo "注意: 目前只有 storage-v1.0.0.tar.gz 包文件可用"
}

# 主函数
main() {
    case "${1:-help}" in
        "deploy")
            if [[ -z "$2" ]]; then
                log_error "请指定版本号"
                show_help
                exit 1
            fi
            check_service
            # 注意：目前只使用 v1.0.0 的包文件
            deploy_service "$2" "${PACKAGE_PATH}/storage-v1.0.0.tar.gz" "$3"
            ;;
        "status")
            if [[ -z "$2" ]]; then
                log_error "请指定版本号"
                exit 1
            fi
            check_deployment_status "${SERVICE_NAME}" "$2"
            ;;
        "list")
            list_all_deployments
            ;;
        "check")
            check_service
            ;;
        "help"|*)
            show_help
            ;;
    esac
}

# 运行主函数
main "$@"