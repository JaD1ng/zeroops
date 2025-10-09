package api

import (
	"fmt"
	"time"

	"github.com/fox-gonic/fox"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/model"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/service"
	"github.com/rs/zerolog/log"
)

// Api Prometheus Adapter API
type Api struct {
	metricService       *service.MetricService
	alertService        *service.AlertService
	alertmanagerService *service.AlertmanagerService
	router              *fox.Engine
}

// NewApi 创建新的 API
func NewApi(metricService *service.MetricService, alertService *service.AlertService, alertmanagerService *service.AlertmanagerService, router *fox.Engine) (*Api, error) {
	api := &Api{
		metricService:       metricService,
		alertService:        alertService,
		alertmanagerService: alertmanagerService,
		router:              router,
	}

	api.setupRouters(router)
	return api, nil
}

// setupRouters 设置路由
func (api *Api) setupRouters(router *fox.Engine) {
	// 指标相关路由
	api.setupMetricRouters(router)
	// 告警相关路由
	api.setupAlertRouters(router)
	// Alertmanager 兼容路由
	api.setupAlertmanagerRouters(router, api.alertmanagerService)
}

// ========== 通用辅助方法 ==========

// SendErrorResponse 发送错误响应（可被其他API模块使用）
func SendErrorResponse(c *fox.Context, statusCode int, errorCode, message string, extras map[string]string) {
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

// ParseTimeRange 解析时间范围参数
func ParseTimeRange(startStr, endStr string) (time.Time, time.Time, error) {
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

// ParseStep 解析步长参数
func ParseStep(stepStr string) time.Duration {
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
