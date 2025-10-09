package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/fox-gonic/fox"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/model"
	"github.com/rs/zerolog/log"
)

// setupMetricRouters 设置指标相关路由
func (api *Api) setupMetricRouters(router *fox.Engine) {
	router.GET("/v1/metrics", api.GetMetrics)
	router.GET("/v1/metrics/:service/:metric", api.QueryMetric)
}

// GetMetrics 获取可用指标列表（GET /v1/metrics）
func (api *Api) GetMetrics(c *fox.Context) {
	ctx := c.Request.Context()

	response, err := api.metricService.GetAvailableMetrics(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to get available metrics")
		SendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodeInternalError, "获取指标列表失败", nil)
		return
	}

	c.JSON(http.StatusOK, response)
}

// QueryMetric 查询指标数据（GET /v1/metrics/:service/:metric）
func (api *Api) QueryMetric(c *fox.Context) {
	ctx := c.Request.Context()

	// 获取路径参数
	serviceName := c.Param("service")
	metricName := c.Param("metric")

	// 获取查询参数
	version := c.Query("version")
	startStr := c.Query("start")
	endStr := c.Query("end")
	stepStr := c.Query("step")

	// 解析时间参数
	start, end, err := ParseTimeRange(startStr, endStr)
	if err != nil {
		log.Error().Err(err).Msg("invalid time parameters")
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			fmt.Sprintf("参数 'start/end' 格式错误: %s", err.Error()), nil)
		return
	}

	// 解析步长参数
	step := ParseStep(stepStr)

	// 查询指标
	response, err := api.metricService.QueryMetric(ctx, serviceName, metricName, version, start, end, step)
	if err != nil {
		api.handleQueryError(c, err, serviceName, metricName)
		return
	}

	c.JSON(http.StatusOK, response)
}

// handleQueryError 处理查询错误
func (api *Api) handleQueryError(c *fox.Context, err error, service, metric string) {
	var serviceNotFound *model.ServiceNotFoundError
	var metricNotFound *model.MetricNotFoundError
	var prometheusError *model.PrometheusError

	switch {
	case errors.As(err, &serviceNotFound):
		log.Error().Err(err).Str("service", service).Msg("service not found")
		SendErrorResponse(c, http.StatusNotFound, model.ErrorCodeServiceNotFound,
			err.Error(), map[string]string{"service": service})

	case errors.As(err, &metricNotFound):
		log.Error().Err(err).Str("metric", metric).Msg("metric not found")
		SendErrorResponse(c, http.StatusNotFound, model.ErrorCodeMetricNotFound,
			err.Error(), map[string]string{"metric": metric})

	case errors.As(err, &prometheusError):
		log.Error().Err(err).Msg("prometheus query error")
		SendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodePrometheusError,
			"Prometheus 查询失败", nil)

	default:
		log.Error().Err(err).Msg("unexpected error during metric query")
		SendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodeInternalError,
			"内部服务器错误", nil)
	}
}
