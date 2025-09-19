package interfaces

import (
	"context"
	"mocks3/shared/models"
	"net/http"
)

// MetricAnomalyService 指标异常注入服务接口
type MetricAnomalyService interface {
	// 异常规则管理
	CreateRule(ctx context.Context, rule *models.MetricAnomalyRule) error
	DeleteRule(ctx context.Context, ruleID string) error
	GetRule(ctx context.Context, ruleID string) (*models.MetricAnomalyRule, error)
	ListRules(ctx context.Context) ([]*models.MetricAnomalyRule, error)

	// 指标异常注入核心功能
	ShouldInjectError(ctx context.Context, service, metricName, instance string) (map[string]any, bool)
}

// MetricInjector HTTP指标异常注入器接口
type MetricInjector interface {
	// HTTP 中间件 - 核心功能
	HTTPMiddleware() func(http.Handler) http.Handler

	// 指标异常注入 - 统一入口
	InjectMetricAnomaly(ctx context.Context, metricName string, originalValue float64) float64
}
