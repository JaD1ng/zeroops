#!/bin/bash

# 测试增量更新告警规则功能

BASE_URL="http://localhost:8080"

echo "=== 测试增量更新告警规则 ==="

# 1. 先进行全量同步，创建初始规则
echo -e "\n1. 全量同步规则..."
curl -X POST ${BASE_URL}/v1/alert-rules/sync \
  -H "Content-Type: application/json" \
  -d '{
    "rules": [
      {
        "name": "high_cpu_usage",
        "description": "CPU使用率过高告警",
        "expr": "system_cpu_usage_percent",
        "op": ">",
        "severity": "warning"
      },
      {
        "name": "high_memory_usage",
        "description": "内存使用率过高告警",
        "expr": "system_memory_usage_percent",
        "op": ">",
        "severity": "warning"
      }
    ],
    "rule_metas": [
      {
        "alert_name": "high_cpu_usage",
        "labels": "{\"service\":\"storage-service\",\"version\":\"1.0.0\"}",
        "threshold": 80,
        "watch_time": 300
      },
      {
        "alert_name": "high_cpu_usage",
        "labels": "{\"service\":\"metadata-service\",\"version\":\"1.0.0\"}",
        "threshold": 85,
        "watch_time": 300
      },
      {
        "alert_name": "high_memory_usage",
        "labels": "{\"service\":\"storage-service\",\"version\":\"1.0.0\"}",
        "threshold": 90,
        "watch_time": 600
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
    "severity": "critical"
  }' | jq .

sleep 2

# 3. 更新单个规则元信息
echo -e "\n3. 更新规则元信息..."
curl -X PUT ${BASE_URL}/v1/alert-rules/meta \
  -H "Content-Type: application/json" \
  -d '{
    "alert_name": "high_cpu_usage",
    "labels": "{\"service\":\"storage-service\",\"version\":\"1.0.0\"}",
    "threshold": 75,
    "watch_time": 600
  }' | jq .

sleep 2

# 4. 添加新的元信息
echo -e "\n4. 添加新的元信息..."
curl -X PUT ${BASE_URL}/v1/alert-rules/meta \
  -H "Content-Type: application/json" \
  -d '{
    "alert_name": "high_memory_usage",
    "labels": "{\"service\":\"queue-service\",\"version\":\"2.0.0\"}",
    "threshold": 95,
    "watch_time": 300
  }' | jq .

echo -e "\n=== 测试完成 ==="