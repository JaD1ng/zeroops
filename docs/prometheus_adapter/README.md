# Prometheus Adapter 模块文档

## 概述

Prometheus Adapter 提供从 Prometheus 获取服务指标的 RESTful API 接口。支持按服务名称和版本进行查询。

## 架构设计

### 模块结构

```
internal/prometheus_adapter/
├── server.go           # 服务器主入口，负责初始化和生命周期管理
├── api/                # API 层，处理 HTTP 请求
│   ├── api.go         # API 基础结构和初始化
│   └── metric_api.go  # 指标相关的 API 处理器
├── service/            # 业务逻辑层
│   └── metric_service.go  # 指标查询服务实现
├── client/             # Prometheus 客户端
│   └── prometheus_client.go  # 封装 Prometheus API 调用
└── model/              # 数据模型
    ├── api.go         # API 请求响应模型
    ├── constants.go   # 常量定义（错误码等）
    └── error.go       # 错误类型定义
```

### 层次设计

1. **API 层** (`api/`)
   - 处理 HTTP 请求和响应
   - 参数验证和解析
   - 错误响应格式化

2. **Service 层** (`service/`)
   - 业务逻辑处理
   - 指标和服务存在性验证
   - 数据转换和组装

3. **Client 层** (`client/`)
   - 与 Prometheus API 交互
   - PromQL 查询构建
   - 结果数据转换

4. **Model 层** (`model/`)
   - 统一的数据模型定义
   - 错误类型和错误码
   - 请求响应结构体

### 核心组件

#### PrometheusAdapterServer
主服务器组件，负责：
- 初始化 Prometheus 客户端
- 创建服务实例
- 设置 API 路由
- 管理生命周期

#### PrometheusClient
Prometheus 客户端封装，提供：
- `QueryRange`: 执行时间范围查询
- `GetAvailableMetrics`: 获取所有可用指标
- `CheckMetricExists`: 检查指标是否存在
- `CheckServiceExists`: 检查服务是否存在
- `BuildQuery`: 构建 PromQL 查询语句

#### MetricService
业务逻辑服务，实现：
- 动态指标发现
- 查询参数验证
- 错误处理和转换

## 配置说明

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| PROMETHEUS_ADDRESS | Prometheus 服务器地址 | http://10.210.10.33:9090 |

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