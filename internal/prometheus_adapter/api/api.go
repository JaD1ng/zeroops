package api

import (
	"github.com/fox-gonic/fox"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/service"
)

// Api Prometheus Adapter API
type Api struct {
	metricService *service.MetricService
	router        *fox.Engine
}

// NewApi 创建新的 API
func NewApi(metricService *service.MetricService, router *fox.Engine) (*Api, error) {
	api := &Api{
		metricService: metricService,
		router:        router,
	}

	api.setupRouters(router)
	return api, nil
}

// setupRouters 设置路由
func (api *Api) setupRouters(router *fox.Engine) {
	// 指标相关路由
	api.setupMetricRouters(router)
}
