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
	config              *config.Config
	promConfig          *promconfig.PrometheusAdapterConfig
	promClient          *client.PrometheusClient
	metricService       *service.MetricService
	alertService        *service.AlertService
	alertWebhookService *service.AlertWebhookService
	api                 *api.Api
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

	// 创建告警 Webhook 服务
	alertWebhookService := service.NewAlertWebhookService(promClient, promConfig)

	server := &PrometheusAdapterServer{
		config:              cfg,
		promConfig:          promConfig,
		promClient:          promClient,
		metricService:       metricService,
		alertService:        alertService,
		alertWebhookService: alertWebhookService,
	}

	// 启动告警 Webhook 服务
	if err := alertWebhookService.Start(); err != nil {
		log.Error().Err(err).Msg("Failed to start alert webhook service")
		// 不返回错误，允许服务继续运行
	}

	log.Info().Str("prometheus_address", promConfig.Prometheus.Address).Msg("Prometheus Adapter initialized successfully")
	return server, nil
}

// UseApi 设置 API 路由
func (s *PrometheusAdapterServer) UseApi(router *fox.Engine) error {
	var err error
	s.api, err = api.NewApi(s.metricService, s.alertService, router)
	if err != nil {
		return fmt.Errorf("failed to initialize API: %w", err)
	}

	return nil
}

// Close 优雅关闭服务器
func (s *PrometheusAdapterServer) Close(ctx context.Context) error {
	log.Info().Msg("Starting shutdown...")

	// 停止告警 Webhook 服务
	if s.alertWebhookService != nil {
		s.alertWebhookService.Stop()
	}

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
