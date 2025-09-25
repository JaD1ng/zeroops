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
  - 告警规则管理
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
│   └── alert_api.go      # 告警规则管理 API 处理器
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

### 告警规则管理

#### 1. 更新单个规则模板
- 方法与路径：`PUT /v1/alert-rules/:rule_name`
- 功能：更新指定的告警规则模板，系统会自动查找所有使用该规则的元信息并重新生成 Prometheus 规则
- 路径参数：
  - `rule_name`：规则名称（如 `high_cpu_usage`）
- 请求体示例：
```json
{
  "description": "CPU使用率异常告警（更新后）",
  "expr": "avg(system_cpu_usage_percent)",
  "op": ">=",
  "severity": "critical",
  "watch_time": 300
}
```
- 响应示例：
```json
{
  "status": "success",
  "message": "Rule 'high_cpu_usage' updated and synced to Prometheus",
  "affected_metas": 3  // 影响的元信息数量
}
```

#### 2. 批量更新规则元信息
- 方法与路径：`PUT /v1/alert-rules-meta/:rule_name`
- 功能：批量更新指定规则的元信息，系统会根据对应的规则模板重新生成 Prometheus 规则
- 路径参数：
  - `rule_name`：规则名称（如 `high_cpu_usage`）
- 请求体示例：
```json
{
  "metas": [
    {
      "labels": "{\"service\":\"storage-service\",\"version\":\"1.0.0\"}",  // 必填，用于唯一标识
      "threshold": 85
    },
    {
      "labels": "{\"service\":\"storage-service\",\"version\":\"2.0.0\"}",  // 必填，用于唯一标识
      "threshold": 90
    }
  ]
}
```
- 响应示例：
```json
{
  "status": "success",
  "message": "Rule metas updated and synced to Prometheus",
  "rule_name": "high_cpu_usage",
  "updated_count": 2
}
```

#### 规则生成机制
- **规则模板与元信息关联**：通过 `alert_name` 字段关联
  - `AlertRule.name` = `AlertRuleMeta.alert_name`
- **元信息唯一标识**：通过 `alert_name` + `labels` 的组合唯一确定一个元信息记录
- **Prometheus 告警生成**：
  - 所有基于同一规则模板的告警使用相同的 `alert` 名称（即规则模板的 `name`）
  - 通过 `labels` 区分不同的服务实例

#### 字段说明
- **AlertRule（规则模板）**：
  - `name`：规则名称，作为 Prometheus 的 alert 名称
  - `description`：规则描述，可读的 title
  - `expr`：PromQL 表达式，如 `sum(apitime) by (service, version)`，可包含时间范围
  - `op`：比较操作符（`>`, `<`, `=`, `!=`）
  - `severity`：告警等级，通常进入告警的 labels.severity
  - `watch_time`：持续时间（秒），对应 Prometheus 的 `for` 字段
- **AlertRuleMeta（元信息）**：
  - `alert_name`：关联的规则名称（对应 alert_rules.name）
  - `labels`：JSON 格式的标签，用于筛选特定服务（如 `{"service":"s3","version":"v1"}`）
  - `threshold`：告警阈值

#### 增量更新说明
- **增量更新**：新接口支持增量更新，只需传入需要修改的字段
- **自动匹配**：
  - 更新规则模板时，系统自动查找所有 `alert_name` 匹配的元信息并重新生成规则
  - 更新元信息时，系统根据 `alert_name` + `labels` 查找并更新对应的元信息
- **缓存机制**：系统在内存中缓存当前的规则和元信息，支持快速增量更新

## 告警接收 Webhook

- 目标：实现自定义 webhook 服务，主动从 Prometheus 拉取告警并转发到监控告警模块
- 实现方式：
  - 通过 Prometheus Alerts API 获取告警
  - 定期轮询 Prometheus 的 `/api/v1/alerts` 端点
  - 将获取的告警格式化后 POST 到监控告警模块

### Webhook 服务架构
```
┌─────────────────┐
│   Prometheus    │
│  (告警规则引擎)   │
└────────┬────────┘
         │ Pull (轮询)
         │ GET /api/v1/alerts
         ▼
┌─────────────────┐
│  Alert Webhook  │
│   （自定义服务）  │
└────────┬────────┘
         │ Push
         │ POST /v1/integrations/prometheus/alerts
         ▼
┌─────────────────┐
│   监控告警模块    │
│  (告警处理中心)   │
└─────────────────┘
```

### 实现细节
- **轮询机制**：
  - 每 10 秒从 Prometheus 拉取一次活跃告警
  - 通过 `GET http://prometheus:9090/api/v1/alerts` 获取告警列表
  - 维护告警状态缓存，避免重复推送

- **告警格式转换**：
  - 将 Prometheus 告警格式转换为监控告警模块所需格式
  - 包含告警名称、标签、严重程度、开始时间等信息
  - 支持告警恢复状态通知

- **推送目标**：
  - URL: `http://alert-module:8080/v1/integrations/prometheus/alerts`
  - Method: POST
  - Content-Type: application/json

## 支持的服务

当前 mock/s3 环境下：
- `metadata-service`
- `storage-service`
- `queue-service`
- `third-party-service`
- `mock-error-service`

所有服务的版本信息通过标签 `service_version` 暴露。

## 错误码

- `METRIC_NOT_FOUND`：指标不存在
- `SERVICE_NOT_FOUND`：服务不存在
- `INVALID_PARAMETER`：请求参数不合法（如时间格式不正确）
- `INTERNAL_ERROR`：内部服务器错误
- `PROMETHEUS_ERROR`：Prometheus 查询失败
