# Prometheus Adapter API 文档

## 概述

Prometheus Adapter 提供从 Prometheus 获取服务指标的 RESTful API 接口。支持按服务名称和版本进行查询。

## API

### 1. 获取可用指标列表

**GET** `/v1/metrics`

获取所有可用的指标列表。

#### 请求示例
```bash
GET /v1/metrics
```

#### 响应示例
```json
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

### 2. 通用指标查询接口

**GET** `/v1/metrics/:service/:metric`

获取指定服务的任意指标时间序列数据。指标不存在则返回错误。

#### 路径参数
- `service` (string, required): 服务名称
- `metric` (string, required): 指标名称（必须是 Prometheus 中实际存在的指标）

#### 查询参数
- `version` (string, optional): 服务版本，不指定则返回所有版本
- `start` (string, optional): 开始时间 (RFC3339 格式，如: 2024-01-01T00:00:00Z)
- `end` (string, optional): 结束时间 (RFC3339 格式，如: 2024-01-01T01:00:00Z)
- `step` (string, optional): 时间步长 (如: 1m, 5m, 1h)，默认 1m

#### 请求示例

1. **查询 CPU 使用率：**
```bash
GET /v1/metrics/metadata-service/system_cpu_usage_percent?version=1.0.0
```

2. **查询内存使用率：**
```bash
GET /v1/metrics/storage-service/system_memory_usage_percent?version=1.0.0
```

3. **查询 HTTP 请求延迟：**
```bash
GET /v1/metrics/storage-service/http_latency?version=1.0.0
```

4. **查询网络 QPS：**
```bash
GET /v1/metrics/storage-service/system_network_qps?version=1.0.0
```

#### 成功响应示例

**HTTP 200 OK**
```json
{
  "service": "metadata-service",
  "version": "1.0.0",
  "metric": "system_cpu_usage_percent",
  "data": [
    {
      "timestamp": "2024-01-01T00:00:00Z",
      "value": 45.2
    },
    {
      "timestamp": "2024-01-01T00:01:00Z",
      "value": 48.5
    }
  ]
}
```

#### 错误响应示例

**指标不存在时 - HTTP 404 Not Found**
```json
{
  "error": {
    "code": "METRIC_NOT_FOUND",
    "message": "指标 'invalid_metric' 不存在",
    "metric": "invalid_metric"
  }
}
```

**服务不存在时 - HTTP 404 Not Found**
```json
{
  "error": {
    "code": "SERVICE_NOT_FOUND",
    "message": "服务 'invalid-service' 不存在",
    "service": "invalid-service"
  }
}
```

**参数错误时 - HTTP 400 Bad Request**
```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "参数 'start' 格式错误: invalid-time",
    "parameter": "start",
    "value": "invalid-time"
  }
}
```

## 实现说明

### 支持的服务列表

当前 mock/s3 环境中支持的服务：
- `metadata-service` - 元数据管理服务
- `storage-service` - 存储服务
- `queue-service` - 消息队列服务
- `third-party-service` - 第三方集成服务
- `mock-error-service` - 错误模拟服务

所有服务的版本信息通过 `service_version` 标签暴露。