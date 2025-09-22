package service

import "github.com/qiniu/zeroops/internal/deploy/model"

// InstanceManager 实例管理接口，负责实例信息查询和状态管理
type InstanceManager interface {
	// GetServiceInstances 获取指定服务的实例详细信息，可选择按版本过滤
	GetServiceInstances(serviceName string, version ...string) ([]*model.InstanceInfo, error)

	// GetInstanceVersionHistory 获取指定实例的版本历史记录
	GetInstanceVersionHistory(instanceID string) ([]*model.VersionInfo, error)
}
