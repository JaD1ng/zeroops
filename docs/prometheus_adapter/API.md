# Prometheus Adapter API 文档

## 概述

Prometheus Adapter 提供从 Prometheus 获取服务 QPS 和平均时延指标的 RESTful API 接口。支持按服务名称和版本进行查询。

> **当前状态**：
> - QPS 指标：已实现，使用 `system_network_qps` 指标（基于网络包统计）
> - 时延指标：已实现，使用 `http.server.request.duration_seconds` 指标（HTTP 请求真实时延）

## API

### 1. 获取服务 QPS 指标

**GET** `/v1/metrics/:service/qps`

获取指定服务的 QPS（每秒请求数）指标数据。

#### 路径参数
- `service` (string, required): 服务名称

#### 查询参数
- `version` (string, optional): 服务版本，不指定则返回所有版本
- `start` (string, optional): 开始时间 (RFC3339 格式，如: 2024-01-01T00:00:00Z)
- `end` (string, optional): 结束时间 (RFC3339 格式，如: 2024-01-01T01:00:00Z)
- `step` (string, optional): 时间步长 (如: 1m, 5m, 1h)，默认 1m

#### 请求示例
```bash
GET /v1/metrics/metadata-service/qps?version=1.0.0&start=2024-01-01T00:00:00Z&end=2024-01-01T01:00:00Z&step=1m
```

#### 响应示例
```json
{
  "service": "metadata-service",
  "version": "1.0.0",
  "metric_type": "qps",
  "data": [
    {
      "timestamp": "2024-01-01T00:00:00Z",
      "value": 150.5
    },
    {
      "timestamp": "2024-01-01T00:01:00Z",
      "value": 148.2
    }
  ],
  "summary": {
    "min": 120.1,
    "max": 180.3,
    "avg": 152.8,
    "total_points": 60
  }
}
```

### 2. 获取服务平均时延指标

**GET** `/v1/metrics/:service/latency`

获取指定服务的平均响应时延指标数据（单位：秒）。

#### 路径参数
- `service` (string, required): 服务名称

#### 查询参数
- `version` (string, optional): 服务版本，不指定则返回所有版本
- `start` (string, optional): 开始时间 (RFC3339 格式)
- `end` (string, optional): 结束时间 (RFC3339 格式)
- `step` (string, optional): 时间步长，默认 1m
- `percentile` (string, optional): 百分位数 (p50, p95, p99)，默认 p50

#### 请求示例
```bash
GET /v1/metrics/storage-service/latency?version=1.0.0&percentile=p95&start=2024-01-01T00:00:00Z&end=2024-01-01T01:00:00Z
```

#### 响应示例
```json
{
  "service": "storage-service",
  "version": "1.0.0",
  "metric_type": "latency",
  "percentile": "p95",
  "data": [
    {
      "timestamp": "2024-01-01T00:00:00Z",
      "value": 125.8
    },
    {
      "timestamp": "2024-01-01T00:01:00Z",
      "value": 132.1
    }
  ],
  "summary": {
    "min": 98.5,
    "max": 201.2,
    "avg": 128.9,
    "total_points": 60
  }
}
```

### 3. 获取服务综合指标

**GET** `/v1/metrics/:service/overview`

同时获取指定服务的 QPS 和时延指标概览。

#### 路径参数
- `service` (string, required): 服务名称

#### 查询参数
- `version` (string, optional): 服务版本
- `start` (string, optional): 开始时间 (RFC3339 格式)
- `end` (string, optional): 结束时间 (RFC3339 格式)

#### 响应示例
```json
{
  "service": "queue-service",
  "version": "1.0.0",
  "time_range": {
    "start": "2024-01-01T00:00:00Z",
    "end": "2024-01-01T01:00:00Z"
  },
  "metrics": {
    "qps": {
      "current": 152.8,
      "avg": 148.5,
      "max": 180.3,
      "min": 120.1
    },
    "latency": {
      "p50": 85.2,
      "p95": 128.9,
      "p99": 201.2
    }
  }
}
```

### 4. 获取可用服务列表

**GET** `/v1/services`

获取 Prometheus 中可监控的服务列表。

#### 查询参数
- `prefix` (string, optional): 服务名前缀过滤

#### 响应示例
```json
{
  "services": [
    {
      "name": "metadata-service",
      "versions": ["1.0.0"],
      "active_versions": ["1.0.0"],
      "last_updated": "2024-01-01T01:00:00Z"
    },
    {
      "name": "storage-service",
      "versions": ["1.0.0"],
      "active_versions": ["1.0.0"],
      "last_updated": "2024-01-01T00:45:00Z"
    },
    {
      "name": "queue-service",
      "versions": ["1.0.0"],
      "active_versions": ["1.0.0"],
      "last_updated": "2024-01-01T00:30:00Z"
    },
    {
      "name": "third-party-service",
      "versions": ["1.0.0"],
      "active_versions": ["1.0.0"],
      "last_updated": "2024-01-01T00:20:00Z"
    },
    {
      "name": "mock-error-service",
      "versions": ["1.0.0"],
      "active_versions": ["1.0.0"],
      "last_updated": "2024-01-01T00:15:00Z"
    }
  ],
  "total": 5
}
```

## 错误响应

所有 API 在出错时返回统一的错误格式：

```json
{
  "error": "error_code",
  "message": "详细错误描述",
  "details": {
    "field": "具体错误字段"
  }
}
```

### 常见错误码

- `400 Bad Request`: 请求参数错误
- `404 Not Found`: 服务或版本不存在
- `500 Internal Server Error`: 内部服务器错误
- `503 Service Unavailable`: Prometheus 连接失败

## 实现说明

### Prometheus 查询语法

API 内部使用的 Prometheus 查询示例：

#### QPS 查询
```promql
# 网络包 QPS（当前实现）
system_network_qps{exported_job="metadata-service",service_version="1.0.0"}

# 计算5分钟平均 QPS
rate(system_network_qps{exported_job="metadata-service",service_version="1.0.0"}[5m])
```

#### 平均时延查询
```promql
# P95 时延（95分位数）
histogram_quantile(0.95, rate(http.server.request.duration_seconds_bucket{exported_job="metadata-service",service_version="1.0.0"}[5m]))

# P50 时延（中位数）
histogram_quantile(0.50, rate(http.server.request.duration_seconds_bucket{exported_job="metadata-service",service_version="1.0.0"}[5m]))

# P99 时延（99分位数）
histogram_quantile(0.99, rate(http.server.request.duration_seconds_bucket{exported_job="metadata-service",service_version="1.0.0"}[5m]))

# 平均时延
rate(http.server.request.duration_seconds_sum{exported_job="metadata-service",service_version="1.0.0"}[5m])
/
rate(http.server.request.duration_seconds_count{exported_job="metadata-service",service_version="1.0.0"}[5m])
```

### 配置要求

需要在配置文件中指定：
- Prometheus 服务器地址：`http://10.210.10.33:9090`
- 查询超时时间：30秒
- 默认时间范围：最近1小时
- 服务标签映射：
  - 服务名：`exported_job`（在指标中作为标签）
  - 版本号：`service_version`（在指标中作为标签）
  - 实例标识：通过 OpenTelemetry 的 `service.instance.id` 属性设置

### 支持的服务列表

当前 mock/s3 环境中支持的服务：
- `metadata-service` - 元数据管理服务（版本：1.0.0）
- `storage-service` - 存储服务（版本：1.0.0）
- `queue-service` - 消息队列服务（版本：1.0.0）
- `third-party-service` - 第三方集成服务（版本：1.0.0）
- `mock-error-service` - 错误模拟服务（版本：1.0.0）

所有服务的版本信息通过 `service_version` 标签暴露。

### 缓存策略

- 指标数据缓存时间：30秒
- 服务列表缓存时间：5分钟
- 支持 ETag 缓存验证