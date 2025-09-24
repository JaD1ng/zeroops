package service

import (
	"fmt"

	"github.com/qiniu/zeroops/internal/deploy/model"
)

// InstanceManager 实例管理接口，负责实例信息查询和状态管理
type InstanceManager interface {
	// GetServiceInstances 获取指定服务的实例详细信息，可选择按版本过滤
	GetServiceInstances(serviceName string, version ...string) ([]*model.InstanceInfo, error)

	// GetInstanceVersionHistory 获取指定实例的版本历史记录
	GetInstanceVersionHistory(instanceID string) ([]*model.VersionInfo, error)
}

type floyInstanceService struct {
}

func NewFloyInstanceService() InstanceManager {
	return &floyInstanceService{}
}

func (f *floyInstanceService) GetServiceInstances(serviceName string, version ...string) ([]*model.InstanceInfo, error) {
	_, err := initDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database connection: %w", err)
	}

	// 根据是否有版本参数选择不同的查询方法
	if len(version) > 0 && version[0] != "" {
		// 有版本参数，按服务名和版本查询
		instances, err := instanceRepo.GetInstanceInfosByServiceNameAndVersion(serviceName, version[0])
		if err != nil {
			return nil, fmt.Errorf("failed to get instances by service name and version: %w", err)
		}
		return instances, nil
	} else {
		// 无版本参数，只按服务名查询
		instances, err := instanceRepo.GetInstanceInfosByServiceName(serviceName)
		if err != nil {
			return nil, fmt.Errorf("failed to get instances by service name: %w", err)
		}
		return instances, nil
	}
}

func (f *floyInstanceService) GetInstanceVersionHistory(instanceID string) ([]*model.VersionInfo, error) {
	return nil, nil
}
