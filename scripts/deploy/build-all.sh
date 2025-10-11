#!/bin/bash

# Mock S3 批量打包脚本
# 用于一次性打包所有服务

set -e

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_SCRIPT="$SCRIPT_DIR/build.sh"

# 服务列表
SERVICES=("metadata" "storage" "queue" "third-party" "mock-error")

# 版本号
VERSION="${VERSION:-v1.0.0}"

echo -e "${GREEN}=========================================="
echo -e "Mock S3 批量打包工具"
echo -e "=========================================="
echo -e "版本号: $VERSION"
echo -e "打包服务: ${SERVICES[*]}"
echo -e "==========================================${NC}"
echo ""

# 记录开始时间
START_TIME=$(date +%s)

# 打包所有服务
SUCCESS_COUNT=0
FAIL_COUNT=0
FAILED_SERVICES=()

for service in "${SERVICES[@]}"; do
    echo -e "${BLUE}正在打包服务: $service${NC}"

    if VERSION="$VERSION" "$BUILD_SCRIPT" "$service"; then
        ((SUCCESS_COUNT++))
        echo -e "${GREEN}✓ $service 打包成功${NC}"
    else
        ((FAIL_COUNT++))
        FAILED_SERVICES+=("$service")
        echo -e "\033[0;31m✗ $service 打包失败${NC}"
    fi
    echo ""
done

# 记录结束时间
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# 输出统计信息
echo -e "${GREEN}=========================================="
echo -e "打包完成！"
echo -e "=========================================="
echo -e "总计: ${#SERVICES[@]} 个服务"
echo -e "成功: $SUCCESS_COUNT 个"
echo -e "失败: $FAIL_COUNT 个"
echo -e "耗时: ${DURATION}秒"

if [ $FAIL_COUNT -gt 0 ]; then
    echo -e "\033[0;31m失败的服务: ${FAILED_SERVICES[*]}${NC}"
fi

echo -e "==========================================${NC}"
echo ""

# 列出生成的包
if [ $SUCCESS_COUNT -gt 0 ]; then
    echo -e "${BLUE}生成的部署包:${NC}"
    ls -lh internal/deploy/packages/*-${VERSION}.tar.gz 2>/dev/null || true
fi

# 退出码
if [ $FAIL_COUNT -gt 0 ]; then
    exit 1
else
    exit 0
fi
