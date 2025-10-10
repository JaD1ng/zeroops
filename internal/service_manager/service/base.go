package service

import (
	deployModel "github.com/qiniu/zeroops/internal/deploy/model"
	deployService "github.com/qiniu/zeroops/internal/deploy/service"
	"github.com/qiniu/zeroops/internal/service_manager/database"
	"github.com/rs/zerolog/log"
)

type Service struct {
	db              *database.Database
	deployService   deployService.DeployService
	instanceManager InstanceManager
	deployAdapter   *DeployAdapter
}

// InstanceManager 定义实例管理接口
type InstanceManager interface {
	GetServiceInstances(serviceName string, version ...string) ([]*deployModel.InstanceInfo, error)
	GetInstanceVersionHistory(instanceID string) ([]*deployModel.VersionInfo, error)
}

func NewService(db *database.Database) *Service {
	instanceManager := &instanceManagerImpl{}
	service := &Service{
		db:              db,
		deployService:   deployService.NewDeployService(),
		instanceManager: instanceManager,
		deployAdapter:   NewDeployAdapter(instanceManager),
	}

	log.Info().Msg("Service initialized successfully with deploy integration")
	return service
}

// instanceManagerImpl 实现 InstanceManager 接口
type instanceManagerImpl struct{}

func (i *instanceManagerImpl) GetServiceInstances(serviceName string, version ...string) ([]*deployModel.InstanceInfo, error) {
	// TODO: 实现获取服务实例的逻辑
	// 暂时返回空列表，实际应该查询 deploy 模块的数据库
	return []*deployModel.InstanceInfo{}, nil
}

func (i *instanceManagerImpl) GetInstanceVersionHistory(instanceID string) ([]*deployModel.VersionInfo, error) {
	// TODO: 实现获取实例版本历史的逻辑
	return nil, nil
}

func (s *Service) Close() error {
	return nil
}

func (s *Service) GetDatabase() *database.Database {
	return s.db
}
