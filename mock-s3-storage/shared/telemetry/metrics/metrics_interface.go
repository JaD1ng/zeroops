package metrics

import (
	"net/http"
	"shared/config"
	"time"
)

// Metrics 指标接口
type Metrics interface {
	// Counter 计数器
	IncCounter(name string, labels ...string)

	// Gauge 仪表盘
	SetGauge(name string, value float64, labels ...string)

	// Histogram 直方图
	ObserveHistogram(name string, value float64, labels ...string)

	// HTTP指标
	RecordHTTPRequest(method, endpoint, status string, duration time.Duration)

	// HTTP中间件
	HTTPMiddleware() func(http.Handler) http.Handler

	// 获取指标处理器
	Handler() http.Handler

	// Close 关闭指标收集器
	Close()
}

// NewMetrics 创建指标收集器
func NewMetrics(config config.MetricsConfig) Metrics {
	return NewSimpleCpuMetrics(config)
}
