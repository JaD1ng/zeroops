package api

import (
	"github.com/fox-gonic/fox"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/service"
)

// setupAlertmanagerRouters 设置 Alertmanager 兼容路由
// 这些路由模拟 Alertmanager API，接收 Prometheus 的告警推送
func (api *Api) setupAlertmanagerRouters(router *fox.Engine, alertmanagerService *service.AlertmanagerService) {
	// Alertmanager API v2 告警接收端点
	router.POST("/api/v2/alerts", func(c *fox.Context) {
		alertmanagerService.HandleAlertsV2(c.Writer, c.Request)
	})

	// 健康检查端点
	router.GET("/-/healthy", func(c *fox.Context) {
		alertmanagerService.HandleHealthCheck(c.Writer, c.Request)
	})

	// 就绪检查端点
	router.GET("/-/ready", func(c *fox.Context) {
		alertmanagerService.HandleReadyCheck(c.Writer, c.Request)
	})
}
