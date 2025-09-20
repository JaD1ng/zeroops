package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

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
		api.sendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodeInternalError, "获取指标列表失败", nil)
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
	start, end, err := api.parseTimeRange(startStr, endStr)
	if err != nil {
		log.Error().Err(err).Msg("invalid time parameters")
		api.sendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			fmt.Sprintf("参数 'start/end' 格式错误: %s", err.Error()), nil)
		return
	}

	// 解析步长参数
	step := api.parseStep(stepStr)

	// 查询指标
	response, err := api.metricService.QueryMetric(ctx, serviceName, metricName, version, start, end, step)
	if err != nil {
		api.handleQueryError(c, err, serviceName, metricName)
		return
	}

	c.JSON(http.StatusOK, response)
}

// parseTimeRange 解析时间范围参数
func (api *Api) parseTimeRange(startStr, endStr string) (time.Time, time.Time, error) {
	var start, end time.Time
	var err error

	// 如果没有指定开始时间，默认为1小时前
	if startStr == "" {
		start = time.Now().Add(-1 * time.Hour)
	} else {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start time format: %w", err)
		}
	}

	// 如果没有指定结束时间，默认为当前时间
	if endStr == "" {
		end = time.Now()
	} else {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end time format: %w", err)
		}
	}

	// 验证时间范围的合理性
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end time must be after start time")
	}

	return start, end, nil
}

// parseStep 解析步长参数
func (api *Api) parseStep(stepStr string) time.Duration {
	if stepStr == "" {
		return time.Minute // 默认1分钟
	}

	duration, err := time.ParseDuration(stepStr)
	if err != nil {
		log.Warn().Str("step", stepStr).Msg("invalid step format, using default")
		return time.Minute
	}

	return duration
}

// handleQueryError 处理查询错误
func (api *Api) handleQueryError(c *fox.Context, err error, service, metric string) {
	var serviceNotFound *model.ServiceNotFoundError
	var metricNotFound *model.MetricNotFoundError
	var prometheusError *model.PrometheusError

	switch {
	case errors.As(err, &serviceNotFound):
		log.Error().Err(err).Str("service", service).Msg("service not found")
		api.sendErrorResponse(c, http.StatusNotFound, model.ErrorCodeServiceNotFound,
			err.Error(), map[string]string{"service": service})

	case errors.As(err, &metricNotFound):
		log.Error().Err(err).Str("metric", metric).Msg("metric not found")
		api.sendErrorResponse(c, http.StatusNotFound, model.ErrorCodeMetricNotFound,
			err.Error(), map[string]string{"metric": metric})

	case errors.As(err, &prometheusError):
		log.Error().Err(err).Msg("prometheus query error")
		api.sendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodePrometheusError,
			"Prometheus 查询失败", nil)

	default:
		log.Error().Err(err).Msg("unexpected error during metric query")
		api.sendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodeInternalError,
			"内部服务器错误", nil)
	}
}

// sendErrorResponse 发送错误响应
func (api *Api) sendErrorResponse(c *fox.Context, statusCode int, errorCode, message string, extras map[string]string) {
	errorDetail := model.ErrorDetail{
		Code:    errorCode,
		Message: message,
	}

	// 添加额外的字段
	if extras != nil {
		if service, ok := extras["service"]; ok {
			errorDetail.Service = service
		}
		if metric, ok := extras["metric"]; ok {
			errorDetail.Metric = metric
		}
		if parameter, ok := extras["parameter"]; ok {
			errorDetail.Parameter = parameter
		}
		if value, ok := extras["value"]; ok {
			errorDetail.Value = value
		}
	}

	response := model.ErrorResponse{
		Error: errorDetail,
	}

	c.JSON(statusCode, response)
}
