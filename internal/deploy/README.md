# 发布系统模块

## 目录结构说明

```
internal/deploy/
├── README.md                    # 本文档
├── model/                       # 数据模型
│   ├── deploy_models.go        # 发布相关数据结构（DeployParams、RollbackParams、OperationResult）
│   ├── instance_models.go      # 实例相关数据结构（InstanceInfo、VersionInfo）
│   └── types.go                # 通用类型定义和常量
└── service/                     # 业务服务层
    ├── deploy_service.go       # DeployService接口定义和实现
    ├── instance_service.go     # InstanceManager接口定义和实现
    └── internal_utils.go       # 内部工具函数实现
```

## 模块职责

### model/ - 数据模型
- **deploy_models.go**: 定义 `DeployParams`、`RollbackParams`、`OperationResult` 等结构体
- **instance_models.go**: 定义 `InstanceInfo`、`VersionInfo` 等结构体
- **types.go**: 定义通用的类型、常量和枚举

### service/ - 业务服务层
- **deploy_service.go**: `DeployService` 接口定义和实现，提供发布和回滚操作
- **instance_service.go**: `InstanceManager` 接口定义和实现，提供实例查询操作
- **internal_utils.go**: 内部工具函数实现，包含所有辅助方法

## 设计原则

1. **简洁高效**: 最小化目录层级，专注核心功能
2. **单一职责**: 每个文件都有明确的职责边界
3. **接口导向**: 接口定义和实现在同一个文件中，便于维护
4. **可测试性**: 每个模块都可以独立进行单元测试

## 接口和函数映射

### 外部接口
- **DeployService**: `service/deploy_service.go`
  - `ExecuteDeployment()` - 执行发布操作
  - `ExecuteRollback()` - 执行回滚操作

- **InstanceManager**: `service/instance_service.go`
  - `GetServiceInstances()` - 获取服务实例列表
  - `GetInstanceVersionHistory()` - 获取实例版本历史

### 内部工具函数
- **内部辅助方法**: `service/internal_utils.go`
  - `ValidatePackageURL()` - 验证包URL
  - `GetServiceInstanceIDs()` - 获取实例ID列表
  - `GetInstanceHost()` - 获取实例IP地址
  - `GetInstancePort()` - 获取实例端口号
  - `CheckInstanceHealth()` - 检查实例健康状态
