package prometheusadapter

import (
	"fmt"
	"os"

	"github.com/fox-gonic/fox"
	"github.com/qiniu/zeroops/internal/config"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/api"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/client"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/service"
	"github.com/rs/zerolog/log"
)

// PrometheusAdapterServer Prometheus Adapter 服务器
type PrometheusAdapterServer struct {
	config        *config.Config
	promClient    *client.PrometheusClient
	metricService *service.MetricService
	api           *api.Api
}

// NewPrometheusAdapterServer 创建新的 Prometheus Adapter 服务器
func NewPrometheusAdapterServer(cfg *config.Config) (*PrometheusAdapterServer, error) {
	// 使用环境变量或默认值获取 Prometheus 地址
	prometheusAddr := os.Getenv("PROMETHEUS_ADDRESS")
	if prometheusAddr == "" {
		prometheusAddr = "http://localhost:9090"
	}

	// 创建 Prometheus 客户端
	promClient, err := client.NewPrometheusClient(prometheusAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	// 创建指标服务
	metricService := service.NewMetricService(promClient)

	server := &PrometheusAdapterServer{
		config:        cfg,
		promClient:    promClient,
		metricService: metricService,
	}

	log.Info().Str("prometheus_address", prometheusAddr).Msg("Prometheus Adapter initialized successfully")
	return server, nil
}

// UseApi 设置 API 路由
func (s *PrometheusAdapterServer) UseApi(router *fox.Engine) error {
	var err error
	s.api, err = api.NewApi(s.metricService, router)
	if err != nil {
		return fmt.Errorf("failed to initialize API: %w", err)
	}
	return nil
}

// Close 关闭服务器
func (s *PrometheusAdapterServer) Close() error {
	// 当前没有需要关闭的资源
	log.Info().Msg("Prometheus Adapter server closed")
	return nil
}
