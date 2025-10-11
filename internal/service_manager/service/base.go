package service

import (
	deployService "github.com/qiniu/zeroops/internal/deploy/service"
	promClient "github.com/qiniu/zeroops/internal/prometheus_adapter/client"
	"github.com/qiniu/zeroops/internal/service_manager/database"
	"github.com/rs/zerolog/log"
)

type Service struct {
	db               *database.Database
	deployService    deployService.DeployService
	instanceManager  deployService.InstanceManager
	deployAdapter    *DeployAdapter
	prometheusClient *promClient.PrometheusClient
}

func NewService(db *database.Database) *Service {
	// 使用 deploy 模块提供的真实实例管理器实现
	instanceManager := deployService.NewFloyInstanceService()

	// 初始化 Prometheus 客户端
	promClient, err := promClient.NewPrometheusClient("http://10.210.10.33:9090")
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Prometheus client, metrics will be unavailable")
	}

	service := &Service{
		db:               db,
		deployService:    deployService.NewDeployService(),
		instanceManager:  instanceManager,
		deployAdapter:    NewDeployAdapter(instanceManager),
		prometheusClient: promClient,
	}

	log.Info().Msg("Service initialized successfully with deploy integration")
	return service
}

func (s *Service) Close() error {
	return nil
}

func (s *Service) GetDatabase() *database.Database {
	return s.db
}
