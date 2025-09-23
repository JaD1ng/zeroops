# Prometheus Adapter

基于 Prometheus 的指标查询与告警规则同步适配层，提供统一的 REST API：
- 按服务与版本查询任意 Prometheus 指标
- 同步告警规则到 Prometheus 并触发重载

目录
- 概述
- 快速开始
- 架构设计
- API 参考
  - 指标查询
  - 告警规则同步
- Alertmanager 集成
- 支持的服务
- 错误码

## 概述

Prometheus Adapter 作为内部系统与 Prometheus 之间的适配层：
- 向上暴露简洁、统一的 HTTP API
- 向下负责 PromQL 查询与 Prometheus 规则文件管理

## 架构设计

- 分层设计
  - API 层（`internal/prometheus_adapter/api`）：HTTP 请求处理、参数校验、错误格式化
  - Service 层（`internal/prometheus_adapter/service`）：业务逻辑、指标与服务存在性校验、数据装配
  - Client 层（`internal/prometheus_adapter/client`）：与 Prometheus API 交互、PromQL 构建、结果转换
  - Model 层（`internal/prometheus_adapter/model`）：统一数据模型、错误类型、常量

- 目录结构
```
internal/prometheus_adapter/
├── server.go              # 服务器主入口，负责初始化和生命周期管理
├── api/                   # API 层，处理 HTTP 请求
│   ├── api.go            # API 基础结构和初始化
│   ├── metric_api.go     # 指标相关的 API 处理器
│   └── alert_api.go      # 告警规则同步 API 处理器
├── service/               # 业务逻辑层
│   ├── metric_service.go # 指标查询服务实现
│   └── alert_service.go  # 告警规则同步服务实现
├── client/                # Prometheus 客户端
│   └── prometheus_client.go  # 封装 Prometheus API 调用
└── model/                 # 数据模型
    ├── api.go            # API 请求响应模型
    ├── alert.go          # 告警规则模型
    ├── constants.go      # 常量定义（错误码等）
    ├── error.go          # 错误类型定义
    └── prometheus.go     # Prometheus 规则文件模型
```

- 核心组件
  - PrometheusAdapterServer：初始化客户端与路由，管理服务生命周期
  - PrometheusClient：`QueryRange`、`GetAvailableMetrics`、`CheckMetricExists`、`CheckServiceExists`、`BuildQuery`
  - MetricService：参数校验、动态指标发现、错误转换
  - AlertService：告警规则同步、Prometheus 规则文件生成、配置重载

## API 

### 指标查询

1) 获取可用指标列表
- 方法与路径：`GET /v1/metrics`
- 用途：列出当前可查询的所有指标名称
- 响应示例：
```
{
  "metrics": [
    "system_cpu_usage_percent",
    "system_memory_usage_percent",
    "system_disk_usage_percent",
    "system_network_qps",
    "system_machine_online_status",
    "http_latency"
  ]
}
```

2) 查询指定服务的指标时间序列
- 方法与路径：`GET /v1/metrics/{service}/{metric}`
- 路径参数：
  - `service`：服务名（必填）
  - `metric`：指标名（必填，需为 Prometheus 中存在的指标）
- 查询参数：
  - `version`：服务版本（选填；不传则返回所有版本）
  - `start`：开始时间（选填，RFC3339）
  - `end`：结束时间（选填，RFC3339）
  - `step`：步长（选填，如 `1m`、`5m`、`1h`；默认 `1m`）
- 请求示例：
  - `GET /v1/metrics/metadata-service/system_cpu_usage_percent?version=1.0.0`
  - `GET /v1/metrics/storage-service/system_memory_usage_percent?version=1.0.0`
  - `GET /v1/metrics/storage-service/http_latency?version=1.0.0`
  - `GET /v1/metrics/storage-service/system_network_qps?version=1.0.0`
- 成功响应示例：
```
{
  "service": "metadata-service",
  "version": "1.0.0",
  "metric": "system_cpu_usage_percent",
  "data": [
    { "timestamp": "2024-01-01T00:00:00Z", "value": 45.2 },
    { "timestamp": "2024-01-01T00:01:00Z", "value": 48.5 }
  ]
}
```
- 错误响应示例：
  - 指标不存在（404）：
```
{
  "error": {
    "code": "METRIC_NOT_FOUND",
    "message": "指标 'invalid_metric' 不存在",
    "metric": "invalid_metric"
  }
}
```
  - 服务不存在（404）：
```
{
  "error": {
    "code": "SERVICE_NOT_FOUND",
    "message": "服务 'invalid-service' 不存在",
    "service": "invalid-service"
  }
}
```
  - 参数错误（400）：
```
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "参数 'start' 格式错误: invalid-time",
    "parameter": "start",
    "value": "invalid-time"
  }
}
```

### 告警规则同步

- 方法与路径：`POST /v1/alert-rules/sync`
- 功能：接收监控告警模块发送的完整规则列表，生成 Prometheus 规则文件并触发重载（全量同步）
- 请求体示例：
```
{
  "rules": [
    {
      "name": "high_cpu_usage",
      "description": "CPU使用率过高告警",
      "expr": "system_cpu_usage_percent",
      "op": ">",
      "severity": "warning"
    }
  ],
  "rule_metas": [
    {
      "alert_name": "high_cpu_usage_storage_v1",
      "labels": "{\"service\":\"storage-service\",\"version\":\"1.0.0\"}",
      "threshold": 90,
      "watch_time": 300,
      "match_time": "5m"
    }
  ]
}
```
- 响应示例：
```
{
  "status": "success",
  "message": "Rules synced to Prometheus"
}
```

## Alertmanager 集成

- 目标：将 Prometheus 触发的告警通过 Alertmanager 转发到监控告警模块
- `alertmanager.yml` 配置示例：
```yaml
global:
  resolve_timeout: 5m

route:
  group_by: ['alertname', 'cluster', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'zeroops-alert-webhook'

receivers:
  - name: 'zeroops-alert-webhook'
    webhook_configs:
      - url: 'http://alert-module:8080/v1/integrations/alertmanager/webhook'
        send_resolved: true
```
- 说明：
  - `url`：监控告警模块的 webhook 地址（按实际部署修改主机与端口）
  - `send_resolved`：为 `true` 时，告警恢复也会通知

## 支持的服务

当前 mock/s3 环境下：
- `metadata-service`
- `storage-service`
- `queue-service`
- `third-party-service`（原文为 third-party-servrice，已更正）
- `mock-error-service`

所有服务的版本信息通过标签 `service_version` 暴露。

## 错误码

- `METRIC_NOT_FOUND`：指标不存在
- `SERVICE_NOT_FOUND`：服务不存在
- `INVALID_PARAMETER`：请求参数不合法（如时间格式不正确）
- `INTERNAL_ERROR`：内部服务器错误
- `PROMETHEUS_ERROR`：Prometheus 查询失败
