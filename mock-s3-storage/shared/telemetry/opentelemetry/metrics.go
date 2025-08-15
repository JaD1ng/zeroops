package opentelemetry

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shirou/gopsutil/v3/cpu"
)

// MetricsCollector 指标收集器
type MetricsCollector struct {
	// 系统指标
	cpuGauge         *prometheus.GaugeVec
	memoryGauge      *prometheus.GaugeVec
	goroutineGauge   *prometheus.GaugeVec
	memoryAllocGauge *prometheus.GaugeVec

	// 业务指标（可以在这里添加更多）
	fileUploadCounter   *prometheus.CounterVec
	fileDownloadCounter *prometheus.CounterVec
	fileDeleteCounter   *prometheus.CounterVec
	fileSizeHistogram   *prometheus.HistogramVec
	fileTypeCounter     *prometheus.CounterVec

	// 性能指标
	requestLatency *prometheus.HistogramVec
	errorCounter   *prometheus.CounterVec

	config Config
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(config Config) *MetricsCollector {
	collector := &MetricsCollector{
		config: config,
	}

	// 初始化所有指标
	collector.initSystemMetrics()
	collector.initBusinessMetrics()
	collector.initPerformanceMetrics()

	// 注册所有指标
	collector.registerMetrics()

	// 启动指标收集
	collector.startCollection()

	return collector
}

// initSystemMetrics 初始化系统指标
func (m *MetricsCollector) initSystemMetrics() {
	// CPU使用率指标
	m.cpuGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_cpu_usage_percentage",
			Help: "Current CPU usage percentage of the service (0-100).",
		},
		[]string{"service"},
	)

	// 内存使用率指标
	m.memoryGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_memory_usage_percentage",
			Help: "Current memory usage percentage of the service (0-100).",
		},
		[]string{"service"},
	)

	// Goroutine数量指标
	m.goroutineGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_goroutine_count",
			Help: "Current number of goroutines.",
		},
		[]string{"service"},
	)

	// 内存分配指标
	m.memoryAllocGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_memory_alloc_bytes",
			Help: "Current memory allocation in bytes.",
		},
		[]string{"service"},
	)
}

// initBusinessMetrics 初始化业务指标
func (m *MetricsCollector) initBusinessMetrics() {
	// 文件上传计数器
	m.fileUploadCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "file_uploads_total",
			Help: "Total number of file uploads.",
		},
		[]string{"service", "status"},
	)

	// 文件下载计数器
	m.fileDownloadCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "file_downloads_total",
			Help: "Total number of file downloads.",
		},
		[]string{"service", "status"},
	)

	// 文件删除计数器
	m.fileDeleteCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "file_deletes_total",
			Help: "Total number of file deletions.",
		},
		[]string{"service", "status"},
	)

	// 文件大小分布
	m.fileSizeHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "file_upload_size_bytes",
			Help:    "File upload size distribution in bytes.",
			Buckets: prometheus.ExponentialBuckets(1024, 2, 10), // 1KB to 1MB
		},
		[]string{"service", "size_range"},
	)

	// 文件类型统计
	m.fileTypeCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "file_uploads_by_type",
			Help: "Total number of file uploads by content type.",
		},
		[]string{"service", "content_type"},
	)
}

// initPerformanceMetrics 初始化性能指标
func (m *MetricsCollector) initPerformanceMetrics() {
	// 请求延迟
	m.requestLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_latency_seconds",
			Help:    "Request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "operation"},
	)

	// 错误计数器
	m.errorCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "errors_total",
			Help: "Total number of errors.",
		},
		[]string{"service", "type"},
	)
}

// registerMetrics 注册所有指标
func (m *MetricsCollector) registerMetrics() {
	// 使用promauto，指标会自动注册到默认registry
	// 只需要注册系统指标，因为业务和性能指标使用promauto
	prometheus.MustRegister(
		m.cpuGauge,
		m.memoryGauge,
		m.goroutineGauge,
		m.memoryAllocGauge,
	)
}

// startCollection 启动指标收集
func (m *MetricsCollector) startCollection() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.collectSystemMetrics()
			}
		}
	}()
}

// collectSystemMetrics 收集系统指标
func (m *MetricsCollector) collectSystemMetrics() {
	// 获取系统指标
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 计算内存使用率
	memoryUsagePercent := float64(memStats.Alloc) / float64(memStats.Sys) * 100
	if memoryUsagePercent > 100 {
		memoryUsagePercent = 100
	}

	// 获取CPU使用率
	cpuPercent, err := cpu.Percent(0, false)
	var cpuUsagePercent float64
	if err == nil && len(cpuPercent) > 0 {
		cpuUsagePercent = cpuPercent[0]
	}

	// 更新指标
	m.cpuGauge.WithLabelValues(m.config.ServiceName).Set(cpuUsagePercent)
	m.memoryGauge.WithLabelValues(m.config.ServiceName).Set(memoryUsagePercent)
	m.goroutineGauge.WithLabelValues(m.config.ServiceName).Set(float64(runtime.NumGoroutine()))
	m.memoryAllocGauge.WithLabelValues(m.config.ServiceName).Set(float64(memStats.Alloc))
}

// GetRegistry 获取Prometheus registry
func (m *MetricsCollector) GetRegistry() *prometheus.Registry {
	return nil // No registry is managed here, so return nil
}

// RecordFileUpload 记录文件上传
func (m *MetricsCollector) RecordFileUpload(status string) {
	m.fileUploadCounter.WithLabelValues(m.config.ServiceName, status).Inc()
}

// RecordFileDownload 记录文件下载
func (m *MetricsCollector) RecordFileDownload(status string) {
	m.fileDownloadCounter.WithLabelValues(m.config.ServiceName, status).Inc()
}

// RecordFileDelete 记录文件删除
func (m *MetricsCollector) RecordFileDelete(status string) {
	m.fileDeleteCounter.WithLabelValues(m.config.ServiceName, status).Inc()
}

// RecordRequestLatency 记录请求延迟
func (m *MetricsCollector) RecordRequestLatency(operation string, duration time.Duration) {
	m.requestLatency.WithLabelValues(m.config.ServiceName, operation).Observe(duration.Seconds())
}

// RecordError 记录错误
func (m *MetricsCollector) RecordError(errorType string) {
	m.errorCounter.WithLabelValues(m.config.ServiceName, errorType).Inc()
}

// RecordFileSize 记录文件大小分布
func (m *MetricsCollector) RecordFileSize(sizeRange string, size int64) {
	m.fileSizeHistogram.WithLabelValues(m.config.ServiceName, sizeRange).Observe(float64(size))
}

// RecordFileType 记录文件类型
func (m *MetricsCollector) RecordFileType(contentType string) {
	m.fileTypeCounter.WithLabelValues(m.config.ServiceName, contentType).Inc()
}
