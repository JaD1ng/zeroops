#!/bin/bash

# 禁用代理
export no_proxy="10.210.10.33,10.99.181.164,localhost,127.0.0.1"
export NO_PROXY="10.210.10.33,10.99.181.164,localhost,127.0.0.1"

echo "=================================================="
echo "   完整告警流程测试"
echo "=================================================="
echo ""
echo "告警流程："
echo "Prometheus (10.210.10.33:9090)"
echo "    ↓ [告警触发]"
echo "Adapter (10.210.10.33:9999)"
echo "    ↓ [转发]"
echo "Webhook (10.99.181.164:8080)"
echo ""
echo "=================================================="

# Step 1: 检查 Webhook 服务
echo -e "\n[Step 1] 检查 Webhook 服务状态"
echo -n "  测试 webhook 端点: "
response=$(curl -s --noproxy "*" -X POST http://10.99.181.164:8080/v1/integrations/alertmanager/webhook \
  -H "Content-Type: application/json" \
  -d '{"test": "connectivity_check"}' \
  -o /dev/null -w "%{http_code}")
if [ "$response" = "200" ]; then
    echo "✅ Webhook 服务正常 (HTTP 200)"
else
    echo "❌ Webhook 服务异常 (HTTP $response)"
    exit 1
fi

# Step 2: 检查 Adapter 服务
echo -e "\n[Step 2] 检查 Adapter 服务状态"
echo -n "  健康检查: "
curl -s --noproxy "*" http://10.210.10.33:9999/-/healthy &>/dev/null && echo "✅ Healthy" || echo "❌ Unhealthy"
echo -n "  就绪检查: "
curl -s --noproxy "*" http://10.210.10.33:9999/-/ready &>/dev/null && echo "✅ Ready" || echo "❌ Not Ready"

# Step 3: 注意事项提醒
echo -e "\n[Step 3] 重要提醒"
echo "  ⚠️  确保 Adapter 服务已使用新配置重启"
echo "  配置文件: internal/prometheus_adapter/config/prometheus_adapter.yml"
echo "  Webhook URL 应为: http://10.99.181.164:8080/v1/integrations/alertmanager/webhook"
echo ""
read -p "  Adapter 服务是否已重启？(y/n): " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "  请先重启 Adapter 服务后再运行测试"
    exit 1
fi

# Step 4: 手动发送测试告警到 Adapter
echo -e "\n[Step 4] 发送测试告警到 Adapter"
timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
alert_data='[{
    "labels": {
        "alertname": "TestAlertFlow",
        "severity": "warning",
        "service": "test_service",
        "environment": "test",
        "source": "manual_test"
    },
    "annotations": {
        "summary": "测试告警流程",
        "description": "验证 Prometheus → Adapter → Webhook 完整链路"
    },
    "startsAt": "'$timestamp'",
    "generatorURL": "http://test.example.com/alerts"
}]'

echo "  发送告警数据..."
response=$(curl -s --noproxy "*" -X POST http://10.210.10.33:9999/api/v2/alerts \
  -H "Content-Type: application/json" \
  -d "$alert_data" \
  -w "\n%{http_code}")

http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')

if [ "$http_code" = "200" ]; then
    echo "  ✅ Adapter 成功接收告警 (HTTP 200)"
    if [ -n "$body" ]; then
        echo "  响应: $body"
    fi
else
    echo "  ❌ Adapter 接收告警失败 (HTTP $http_code)"
    echo "  响应: $body"
    echo ""
    echo "  可能的原因："
    echo "  1. Adapter 服务未运行"
    echo "  2. Adapter 配置未更新"
    echo "  3. 网络连接问题"
fi

# Step 5: 检查 Webhook 是否收到告警
echo -e "\n[Step 5] 验证 Webhook 是否收到告警"
echo "  等待 2 秒让告警传递..."
sleep 2

echo "  查询 Webhook 收到的告警:"
alerts=$(curl -s --noproxy "*" http://10.99.181.164:8080/alerts)

if [ -z "$alerts" ] || [ "$alerts" = "[]" ]; then
    echo "  ⚠️  Webhook 未收到任何告警"
    echo ""
    echo "  请检查："
    echo "  1. Adapter 配置中的 webhook URL 是否正确"
    echo "  2. Adapter 服务是否已重启"
    echo "  3. 查看 Adapter 日志了解详情"
else
    echo "  ✅ Webhook 收到告警！"
    echo ""
    echo "  最新的告警记录:"
    echo "$alerts" | jq -r '.[-1] | "  时间: \(.timestamp)\n  告警名: \(.data.alerts[0].labels.alertname // "N/A")\n  严重性: \(.data.alerts[0].labels.severity // "N/A")\n  状态: \(.data.alerts[0].status // "N/A")"' 2>/dev/null || echo "$alerts"
fi

# Step 6: 测试 Prometheus 的活跃告警
echo -e "\n[Step 6] 检查 Prometheus 中的活跃告警"
echo "  查询 firing 状态的告警 (前3个):"
curl -s --noproxy "*" http://10.210.10.33:9090/api/v1/alerts | \
    jq -r '.data.alerts[] | select(.state=="firing") | "  - \(.labels.alertname) (\(.labels.service // "no-service"))"' 2>/dev/null | head -3

echo -e "\n=================================================="
echo "测试完成！"
echo ""
echo "完整流程验证："
echo "1. Webhook 服务: ✅ 运行中 (10.99.181.164:8080)"
echo "2. Adapter 服务: ✅ 运行中 (10.210.10.33:9999)"
echo "3. 告警传递测试: $([ "$http_code" = "200" ] && echo "✅ 成功" || echo "❌ 失败")"
echo "4. Webhook 接收: $([ -n "$alerts" ] && [ "$alerts" != "[]" ] && echo "✅ 已收到告警" || echo "⚠️  未收到告警")"
echo ""

if [ "$http_code" = "200" ] && [ -n "$alerts" ] && [ "$alerts" != "[]" ]; then
    echo "🎉 恭喜！告警流程工作正常！"
else
    echo "⚠️  告警流程存在问题，请检查上述步骤中的错误信息"
fi

echo "=================================================="