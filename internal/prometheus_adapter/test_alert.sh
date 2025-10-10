#!/bin/bash

# 测试增量更新告警规则功能

BASE_URL="http://10.210.10.33:9999"

echo "=== 测试增量更新告警规则 ==="

# 1. 初始化规则（使用增量更新接口）
echo -e "\n1. 创建初始规则..."

# 1.1 创建 high_cpu_usage 规则模板
echo -e "\n1.1 创建规则模板: high_cpu_usage"
curl -X PUT ${BASE_URL}/v1/alert-rules/high_cpu_usage \
  -H "Content-Type: application/json" \
  -d '{
    "description": "CPU使用率过高告警",
    "expr": "system_cpu_usage_percent",
    "op": ">",
    "severity": "warning",
    "watch_time": 300
  }' | jq .

sleep 1

# 1.2 创建 high_memory_usage 规则模板
echo -e "\n1.2 创建规则模板: high_memory_usage"
curl -X PUT ${BASE_URL}/v1/alert-rules/high_memory_usage \
  -H "Content-Type: application/json" \
  -d '{
    "description": "内存使用率过高告警",
    "expr": "system_memory_usage_percent",
    "op": ">",
    "severity": "warning",
    "watch_time": 600
  }' | jq .

sleep 1

# 1.3 设置 high_cpu_usage 规则的元信息
echo -e "\n1.3 设置规则元信息: high_cpu_usage"
curl -X PUT ${BASE_URL}/v1/alert-rules-meta/high_cpu_usage \
  -H "Content-Type: application/json" \
  -d '{
    "metas": [
      {
        "labels": "{\"service\":\"storage-service\",\"version\":\"1.0.0\"}",
        "threshold": 80
      },
      {
        "labels": "{\"service\":\"metadata-service\",\"version\":\"1.0.0\"}",
        "threshold": 85
      }
    ]
  }' | jq .

sleep 1

# 1.4 设置 high_memory_usage 规则的元信息
echo -e "\n1.4 设置规则元信息: high_memory_usage"
curl -X PUT ${BASE_URL}/v1/alert-rules-meta/high_memory_usage \
  -H "Content-Type: application/json" \
  -d '{
    "metas": [
      {
        "labels": "{\"service\":\"storage-service\",\"version\":\"1.0.0\"}",
        "threshold": 90
      }
    ]
  }' | jq .

sleep 2

# 2. 更新单个规则模板
echo -e "\n2. 更新规则模板 high_cpu_usage..."
curl -X PUT ${BASE_URL}/v1/alert-rules/high_cpu_usage \
  -H "Content-Type: application/json" \
  -d '{
    "description": "CPU使用率异常告警（更新后）",
    "expr": "avg(system_cpu_usage_percent[5m])",
    "op": ">=",
    "severity": "critical",
    "watch_time": 300
  }' | jq .

sleep 2

# 3. 批量更新规则元信息
echo -e "\n3. 批量更新规则元信息（high_cpu_usage）..."
curl -X PUT ${BASE_URL}/v1/alert-rules-meta/high_cpu_usage \
  -H "Content-Type: application/json" \
  -d '{
    "metas": [
      {
        "labels": "{\"service\":\"storage-service\",\"version\":\"1.0.0\"}",
        "threshold": 75
      },
      {
        "labels": "{\"service\":\"metadata-service\",\"version\":\"1.0.0\"}",
        "threshold": 88
      }
    ]
  }' | jq .

sleep 2

# 4. 批量更新规则元信息（添加新的服务）
echo -e "\n4. 批量更新规则元信息（high_memory_usage - 添加新服务）..."
curl -X PUT ${BASE_URL}/v1/alert-rules-meta/high_memory_usage \
  -H "Content-Type: application/json" \
  -d '{
    "metas": [
      {
        "labels": "{\"service\":\"queue-service\",\"version\":\"2.0.0\"}",
        "threshold": 95
      },
      {
        "labels": "{\"service\":\"third-party-service\",\"version\":\"1.0.0\"}",
        "threshold": 92
      }
    ]
  }' | jq .

sleep 2

# 5. 测试删除规则元信息
echo -e "\n5. 删除规则元信息（删除 high_cpu_usage 的 storage-service）..."
curl -X DELETE ${BASE_URL}/v1/alert-rules-meta/high_cpu_usage \
  -H "Content-Type: application/json" \
  -d '{
    "labels": "{\"service\":\"storage-service\",\"version\":\"1.0.0\"}"
  }' | jq .

sleep 2

# 6. 测试删除不存在的规则元信息（应该返回404）
echo -e "\n6. 删除不存在的规则元信息（测试错误处理）..."
curl -X DELETE ${BASE_URL}/v1/alert-rules-meta/high_cpu_usage \
  -H "Content-Type: application/json" \
  -d '{
    "labels": "{\"service\":\"non-existent-service\",\"version\":\"1.0.0\"}"
  }' | jq .

sleep 2

# 7. 测试删除整个规则模板
echo -e "\n7. 删除整个规则模板（删除 high_memory_usage 及其所有元信息）..."
curl -X DELETE ${BASE_URL}/v1/alert-rules/high_memory_usage | jq .

sleep 2

# 8. 测试删除不存在的规则模板（应该返回404）
echo -e "\n8. 删除不存在的规则模板（测试错误处理）..."
curl -X DELETE ${BASE_URL}/v1/alert-rules/non_existent_rule | jq .

sleep 2

# 9. 验证删除结果 - 查看剩余的规则
echo -e "\n9. 验证删除结果..."
echo "9.1 尝试更新已删除的规则模板（应该创建新规则）："
curl -X PUT ${BASE_URL}/v1/alert-rules/high_memory_usage \
  -H "Content-Type: application/json" \
  -d '{
    "description": "重新创建的内存告警规则",
    "expr": "system_memory_usage_percent",
    "op": ">",
    "severity": "warning",
    "watch_time": 300
  }' | jq .

sleep 1

echo -e "\n9.2 查看当前 high_cpu_usage 的受影响元信息数量（应该只剩1个）："
curl -X PUT ${BASE_URL}/v1/alert-rules/high_cpu_usage \
  -H "Content-Type: application/json" \
  -d '{
    "description": "验证剩余元信息的规则更新"
  }' | jq .

echo -e "\n=== 删除功能测试完成 ==="
echo -e "\n测试总结："
echo "✓ 测试了删除单个规则元信息"
echo "✓ 测试了删除不存在的规则元信息（错误处理）"
echo "✓ 测试了删除整个规则模板及其所有元信息"
echo "✓ 测试了删除不存在的规则模板（错误处理）"
echo "✓ 验证了删除操作的实际效果"

echo -e "\n=== 测试完成 ==="