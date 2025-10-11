# Service Manager 服务管理模块

Service Manager 是 ZeroOps 平台的核心服务管理模块，负责微服务的生命周期管理、部署协调和状态监控。

## 架构设计

### 分层架构

```
┌─────────────────┐
│   HTTP API      │  ← REST API 接口层
├─────────────────┤
│   Service       │  ← 业务逻辑层
├─────────────────┤
│   Database      │  ← 数据访问层
├─────────────────┤
│   PostgreSQL    │  ← 数据存储层
└─────────────────┘
```

- **API层** (`api/`): 处理HTTP请求和响应，参数验证
- **Service层** (`service/`): 核心业务逻辑，事务管理
- **Database层** (`database/`): 数据库操作，SQL查询
- **Model层** (`model/`): 数据模型和类型定义

## 核心功能

### 1. 服务信息管理

- **服务注册**: 创建和注册新的微服务
- **依赖管理**: 维护服务间的依赖关系图
- **版本管理**: 跟踪服务的多个版本
- **健康监控**: 实时监控服务健康状态

### 2. 部署管理

- **部署协调**: 管理服务的部署任务
- **灰度发布**: 支持渐进式部署策略
- **状态控制**: 暂停、继续、回滚部署
- **实例管理**: 跟踪服务实例的分布

### 3. 监控集成

- **指标收集**: 集成时序数据库（Prometheus格式）
- **状态报告**: 服务运行状态实时上报
- **告警处理**: 异常状态检测和告警

## API接口

### 服务管理接口

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/v1/services` | 获取所有服务列表 |
| POST | `/v1/services` | 创建新服务 |
| PUT | `/v1/services/:service` | 更新服务信息 |
| DELETE | `/v1/services/:service` | 删除服务 |
| GET | `/v1/services/:service/activeVersions` | 获取服务详情 |
| GET | `/v1/services/:service/availableVersions` | 获取可用服务版本 |
| GET | `/v1/metrics/:service/:name` | 获取服务监控指标（时序数据） |
| GET | `/v1/metricStats/:service` | 获取服务指标统计（聚合数据） |

### 部署管理接口

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/v1/deployments` | 创建部署任务 |
| GET | `/v1/deployments` | 获取部署任务列表 |
| GET | `/v1/deployments/:service/:version` | 获取部署任务详情 |
| POST | `/v1/deployments/:service/:version` | 更新部署任务 |
| DELETE | `/v1/deployments/:service/:version` | 删除部署任务 |
| POST | `/v1/deployments/:service/:version/pause` | 暂停部署 |
| POST | `/v1/deployments/:service/:version/continue` | 继续部署 |
| POST | `/v1/deployments/:service/:version/rollback` | 回滚部署 |

## 数据模型

### 核心实体

#### Service (服务)
```go
type Service struct {
    Name string   `json:"name"`  // 服务名称（主键）
    Deps []string `json:"deps"`  // 依赖关系列表
}
```

#### ServiceInstance (服务实例)
```go
type ServiceInstance struct {
    ID      string `json:"id"`      // 实例ID（主键）
    Service string `json:"service"` // 关联服务名
    Version string `json:"version"` // 服务版本
}
```

#### ServiceState (服务状态)
- 健康状态等级
- 状态报告时间
- 异常信息

#### DeployTask (部署任务)
- 部署ID
- 目标服务和版本
- 部署状态
- 创建和更新时间

### 数据库设计

使用 PostgreSQL 作为主数据库：

- **services**: 服务基础信息表
- **service_instances**: 服务实例表
- **service_versions**: 服务版本表
- **service_states**: 服务状态表
- **deploy_tasks**: 部署任务表

## 使用示例

### 创建服务

```bash
curl -X POST http://localhost:8080/v1/services \
  -H "Content-Type: application/json" \
  -d '{
    "name": "user-service",
    "deps": ["database-service", "cache-service"]
  }'
```

### 创建部署任务

```bash
curl -X POST http://localhost:8080/v1/deployments \
  -H "Content-Type: application/json" \
  -d '{
    "service": "user-service",
    "version": "v1.2.0",
    "strategy": "rolling"
  }'
```

### 获取服务列表

```bash
curl http://localhost:8080/v1/services
```

响应示例：
```json
{
  "items": [
    {
      "name": "user-service",
      "deployState": "deployed",
      "health": "normal",
      "deps": ["database-service"]
    }
  ],
  "relation": {
    "user-service": ["database-service"]
  }
}
```

### 获取服务指标统计

```bash
curl http://localhost:8080/v1/metricStats/user-service
```

响应示例：
```json
{
  "summary": {
    "metrics": [
      {
        "name": "latency",
        "value": 10
      },
      {
        "name": "traffic",
        "value": 1000
      },
      {
        "name": "errorRatio",
        "value": 10
      },
      {
        "name": "saturation",
        "value": 50
      }
    ]
  },
  "items": [
    {
      "version": "v1.0.1",
      "metrics": [
        {
          "name": "latency",
          "value": 10
        },
        {
          "name": "traffic",
          "value": 1000
        },
        {
          "name": "errorRatio",
          "value": 10
        },
        {
          "name": "saturation",
          "value": 50
        }
      ]
    }
  ]
}
```

## 配置说明

### 数据库配置
```yaml
database:
  host: localhost
  port: 5432
  user: postgres
  password: password
  dbname: zeroops
  sslmode: disable
```

### 服务配置
```yaml
service_manager:
  port: 8080
  log_level: info
```
