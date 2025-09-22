# 发布系统设计文档

## 1. 系统概述

### 1.1 设计目标
发布系统是一个专注于执行发布动作的轻量级系统，负责：
- 接收调度系统的发布指令
- 执行具体的发布操作
- 管理服务实例的版本状态
- 提供回滚功能

### 1.2 设计原则
- **单一职责**：只负责发布动作的执行，不涉及调度逻辑
- **简单可靠**：提供稳定、可预测的发布操作
- **状态透明**：实时反馈发布状态和实例版本信息
- **快速回滚**：支持单实例和批量回滚操作

### 1.3 系统边界
- **负责**：发布动作执行、实例状态管理、回滚操作
- **不负责**：发布调度、批次规划、版本状态管理、实例选择、健康检查

## 2. 系统架构

### 2.1 整体架构
```
┌─────────────────┐    方法调用    ┌─────────────────┐    网络通信    ┌─────────────────┐
│   调度系统       │ ──────────────▶ │   发布系统       │ ──────────────▶ │   实例节点       │
│                 │                │                 │                │                 │
│  • 发布指令     │                │  • 发布操作      │                │  • 服务实例       │
│  • 批次规划     │                │  • 实例管理      │                │  • 版本更新      │
│  • 健康检查     │                │  • 回滚操作      │                │  • 状态上报      │
│                 │                │                 │                │                │
└─────────────────┘                └─────────────────┘                └─────────────────┘
                                             │
                                             │ 数据存储
                                             ▼
                                    ┌─────────────────┐
                                    │   数据库         │
                                    │                 │
                                    │  • 实例版本     │
                                    │  • 回滚记录     │
                                    │  • 状态信息     │
                                    └─────────────────┘
```

### 2.2 核心组件

- **发布执行器**: 执行发布操作，更新实例版本
- **回滚执行器**: 执行回滚操作，处理包下载
- **实例管理器**: 感知执行结果，维护实例状态

## 3. 核心功能模块

### 3.1 发布执行模块
负责执行具体的发布操作，包括：
- 接收发布请求（服务名、版本、实例列表）
- 执行发布动作
- 更新实例版本状态
- 记录发布日志

### 3.2 回滚模块
提供回滚功能：
- 接受回滚请求（服务名、回滚的目标版本、实例列表）
- 执行回滚任务
- 更新实例版本状态
- 记录回滚日志

### 3.3 实例管理模块
获取实例状态：
- 获取运行某服务的全部实例（可通过版本号过滤）
- 获取实例的详细信息
- 获取实例历史版本

## 4. 接口设计

发布系统提供以下外部接口：

- **DeployService**: 发布服务接口，负责发布和回滚操作的执行
- **InstanceManager**: 实例管理接口，负责实例信息查询和状态管理

### 4.1 DeployService接口

#### 4.1.1 接口定义

发布服务接口，负责发布和回滚操作的执行。

```go
type DeployService interface {
    DeployNewService(params *DeployNewServiceParams) (*OperationResult, error)
    DeployNewVersion(params *DeployNewVersionParams) (*OperationResult, error)
    ExecuteRollback(params *RollbackParams) (*OperationResult, error)
}
```

#### 4.1.2 方法说明

**DeployNewService方法**: 部署新服务并创建指定数量的实例

**DeployNewVersion方法**: 触发指定服务版本的发布操作

**ExecuteRollback方法**: 对指定实例执行回滚操作，支持单实例或批量实例回滚

### 4.2 InstanceManager接口

#### 4.2.1 接口定义

实例管理接口，负责实例信息查询和状态管理，发布模块和服务管理模块都需要使用。

```go
type InstanceManager interface {
    GetServiceInstances(serviceName string, version ...string) ([]*InstanceInfo, error)
    GetInstanceVersionHistory(instanceID string) ([]*VersionInfo, error)
}
```

#### 4.2.2 方法说明

**GetServiceInstances方法**: 获取指定服务的实例详细信息，可选择按版本过滤

**GetInstanceVersionHistory方法**: 获取指定实例的版本历史记录

## 5. 内部工具函数

### 5.1 **ValidatePackageURL**: 验证包URL的有效性
```go
func ValidatePackageURL(packageURL string) error
```

### 5.2 **GetServiceInstanceIDs**: 根据服务名和版本获取实例ID列表
```go
func GetServiceInstanceIDs(serviceName string, version ...string) ([]string, error)
```

### 5.3 **GetInstanceHost**: 根据实例ID获取实例的IP地址
```go
func GetInstanceHost(instanceID string) (string, error)
```

### 5.4 **GetInstancePort**: 根据实例ID获取实例的端口号
```go
func GetInstancePort(instanceID string) (int, error)
```

### 5.5 **CheckInstanceHealth**: 检查实例的健康状态
```go
func CheckInstanceHealth(instanceID string) (bool, error)
```

### 5.6 **GetHostIp**: 根据主机名获取主机IP地址
```go
func GetHostIp(hostName string) (string, error)
```

### 5.7 **CheckHostHealth**: 检查主机的健康状态
```go
func CheckHostHealth(hostIpAddress string) (bool, error)
```
