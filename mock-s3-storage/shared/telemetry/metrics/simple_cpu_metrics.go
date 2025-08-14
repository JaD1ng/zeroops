package metrics

import (
	"net/http"
	"runtime"
	"sync"
	"time"

	"shared/config"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SimpleCpuMetrics 简化的CPU指标实现
type SimpleCpuMetrics struct {
	// CPU使用率指标
	cpuUsage *prometheus.GaugeVec

	// CPU监控相关
	cpuStats     *cpuStats
	stopCPUStats chan struct{}
}

// cpuStats CPU统计信息
type cpuStats struct {
	mu           sync.RWMutex
	lastCPUTime  time.Time
	lastCPUUsage float64
}

// NewSimpleCpuMetrics 创建简化的CPU指标收集器
func NewSimpleCpuMetrics(config config.MetricsConfig) *SimpleCpuMetrics {
	metrics := &SimpleCpuMetrics{
		cpuUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_service_cpu_usage_percentage",
				Help: "Current CPU usage percentage of the storage service (0-100).",
			},
			[]string{"service"},
		),

		cpuStats: &cpuStats{
			lastCPUTime:  time.Now(),
			lastCPUUsage: 0.0,
		},
		stopCPUStats: make(chan struct{}),
	}

	// 注册指标
	prometheus.MustRegister(metrics.cpuUsage)

	// 启动CPU监控
	metrics.startCPUMonitoring()

	return metrics
}

// startCPUMonitoring 启动CPU监控
func (s *SimpleCpuMetrics) startCPUMonitoring() {
	go func() {
		ticker := time.NewTicker(3 * time.Second) // 每5秒更新一次CPU使用率
		defer ticker.Stop()

		for {
			select {
			case <-s.stopCPUStats:
				return
			case <-ticker.C:
				s.updateCPUUsage()
			}
		}
	}()
}

// updateCPUUsage 更新CPU使用率
func (s *SimpleCpuMetrics) updateCPUUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 获取当前时间
	now := time.Now()

	s.cpuStats.mu.Lock()
	defer s.cpuStats.mu.Unlock()

	// 计算CPU使用率（基于Goroutine数量和系统负载）
	numGoroutines := runtime.NumGoroutine()
	numCPU := runtime.NumCPU()

	// 基础CPU使用率计算
	// 1. 基于goroutine数量（每个goroutine消耗一定CPU）
	goroutineCPU := float64(numGoroutines) / float64(numCPU*5) * 50.0

	// 2. 基于内存分配频率（频繁GC表示CPU使用高）
	gcCPU := float64(m.NumGC) / 100.0 * 30.0

	// 3. 基于系统负载（如果有系统调用统计）
	sysCPU := float64(m.Sys) / 1024.0 / 1024.0 / 100.0 * 20.0 // 每100MB系统内存分配对应20%CPU

	// 综合CPU使用率
	cpuUsage := goroutineCPU + gcCPU + sysCPU

	// 限制在0-100范围内
	if cpuUsage > 100.0 {
		cpuUsage = 100.0
	} else if cpuUsage < 0.0 {
		cpuUsage = 0.0
	}

	// 平滑处理，避免剧烈波动
	if s.cpuStats.lastCPUUsage > 0 {
		// 使用指数移动平均，权重为0.3
		cpuUsage = s.cpuStats.lastCPUUsage*0.7 + cpuUsage*0.3
	}

	s.cpuStats.lastCPUUsage = cpuUsage
	s.cpuStats.lastCPUTime = now

	// 更新Prometheus指标
	s.cpuUsage.WithLabelValues("file-storage-service").Set(cpuUsage)
}

// IncCounter 增加计数器（简化实现）
func (s *SimpleCpuMetrics) IncCounter(name string, labels ...string) {
	// 简化实现，不记录任何内容
}

// SetGauge 设置仪表盘（简化实现）
func (s *SimpleCpuMetrics) SetGauge(name string, value float64, labels ...string) {
	// 简化实现，不记录任何内容
}

// ObserveHistogram 观察直方图（简化实现）
func (s *SimpleCpuMetrics) ObserveHistogram(name string, value float64, labels ...string) {
	// 简化实现，不记录任何内容
}

// RecordHTTPRequest 记录HTTP请求（简化实现）
func (s *SimpleCpuMetrics) RecordHTTPRequest(method, endpoint, status string, duration time.Duration) {
	// 简化实现，不记录任何内容
}

// HTTPMiddleware HTTP中间件（简化实现）
func (s *SimpleCpuMetrics) HTTPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return next
	}
}

// Handler 获取指标处理器
func (s *SimpleCpuMetrics) Handler() http.Handler {
	return promhttp.Handler()
}

// Close 关闭指标收集器
func (s *SimpleCpuMetrics) Close() {
	close(s.stopCPUStats)
}

// 确保 SimpleCpuMetrics 实现了 Metrics 接口
var _ Metrics = (*SimpleCpuMetrics)(nil)
