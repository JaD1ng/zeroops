package prometheusadapter

import (
	"context"
	"fmt"

	"github.com/fox-gonic/fox"
	"github.com/qiniu/zeroops/internal/config"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/api"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/client"
	promconfig "github.com/qiniu/zeroops/internal/prometheus_adapter/config"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/service"
	"github.com/rs/zerolog/log"
)

// PrometheusAdapterServer Prometheus Adapter 服务器
type PrometheusAdapterServer struct {
	config                   *config.Config
	promConfig               *promconfig.PrometheusAdapterConfig
	promClient               *client.PrometheusClient
	metricService            *service.MetricService
	alertService             *service.AlertService
	alertmanagerProxyService *service.AlertmanagerService
	api                      *api.Api
}

// NewPrometheusAdapterServer 创建新的 Prometheus Adapter 服务器
func NewPrometheusAdapterServer(cfg *config.Config) (*PrometheusAdapterServer, error) {
	// 加载 Prometheus Adapter 配置
	promConfig, err := promconfig.LoadConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to load prometheus adapter config: %w", err)
	}

	// 创建 Prometheus 客户端
	promClient, err := client.NewPrometheusClient(promConfig.Prometheus.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	// 创建指标服务
	metricService := service.NewMetricService(promClient)

	// 创建告警服务
	alertService := service.NewAlertService(promClient, promConfig)

	// 创建 Alertmanager 代理服务
	alertmanagerProxyService := service.NewAlertmanagerProxyService(promConfig)

	server := &PrometheusAdapterServer{
		config:                   cfg,
		promConfig:               promConfig,
		promClient:               promClient,
		metricService:            metricService,
		alertService:             alertService,
		alertmanagerProxyService: alertmanagerProxyService,
	}

	log.Info().Str("prometheus_address", promConfig.Prometheus.Address).Msg("Prometheus Adapter initialized successfully")
	return server, nil
}

// GetBindAddr 获取配置文件中的绑定地址
func (s *PrometheusAdapterServer) GetBindAddr() string {
	if s.promConfig != nil && s.promConfig.Server.BindAddr != "" {
		return s.promConfig.Server.BindAddr
	}
	return ""
}

// UseApi 设置 API 路由
func (s *PrometheusAdapterServer) UseApi(router *fox.Engine) error {
	var err error
	s.api, err = api.NewApi(s.metricService, s.alertService, s.alertmanagerProxyService, router)
	if err != nil {
		return fmt.Errorf("failed to initialize API: %w", err)
	}

	log.Info().Msg("All API endpoints registered")

	return nil
}

// Close 优雅关闭服务器
func (s *PrometheusAdapterServer) Close(ctx context.Context) error {
	log.Info().Msg("Starting shutdown...")

	// 调用 alertService 的 Shutdown 方法保存规则
	if s.alertService != nil {
		if err := s.alertService.Shutdown(); err != nil {
			log.Error().Err(err).Msg("Failed to shutdown alert service")
			return err
		}
	}

	log.Info().Msg("Prometheus Adapter server shut down")
	return nil
}
