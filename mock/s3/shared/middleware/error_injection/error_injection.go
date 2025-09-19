package error_injection

import (
	"context"
	"fmt"
	"mocks3/shared/client"
	"mocks3/shared/models"
	"mocks3/shared/observability"
	"mocks3/shared/utils"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// MetricInjectorConfig 指标异常注入器配置
type MetricInjectorConfig struct {
	MockErrorService MockErrorServiceConfig `yaml:"mock_error_service"`
	Cache            CacheConfig            `yaml:"cache"`
}

// MockErrorServiceConfig Mock Error Service配置
type MockErrorServiceConfig struct {
	URL     string        `yaml:"url"`
	Timeout time.Duration `yaml:"timeout"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	TTL time.Duration `yaml:"ttl"`
}

// MetricInjector 指标异常注入器
type MetricInjector struct {
	mockErrorClient *client.BaseHTTPClient
	serviceName     string
	logger          *observability.Logger

	// 缓存
	cache    map[string]*CachedAnomaly
	cacheMu  sync.RWMutex
	cacheTTL time.Duration

	// 真实资源注入器
	cpuInjector     *CPUSpikeInjector
	memoryInjector  *MemoryLeakInjector
	diskInjector    *DiskFullInjector
	networkInjector *NetworkFloodInjector
	machineInjector *MachineDownInjector
}

// CachedAnomaly 缓存的异常配置
type CachedAnomaly struct {
	Anomaly   map[string]any
	ExpiresAt time.Time
}

// NewMetricInjector 从YAML配置创建指标异常注入器
func NewMetricInjector(configPath string, serviceName string, logger *observability.Logger) (*MetricInjector, error) {
	// 加载配置文件
	var config MetricInjectorConfig
	if err := utils.LoadConfig(configPath, &config); err != nil {
		return nil, fmt.Errorf("failed to load metric injector config: %w", err)
	}

	// 设置默认值
	if config.MockErrorService.URL == "" {
		config.MockErrorService.URL = "http://mock-error-service:8085" // 默认Mock Error Service地址
	}
	if config.MockErrorService.Timeout == 0 {
		config.MockErrorService.Timeout = 5 * time.Second // 默认5秒超时
	}
	if config.Cache.TTL == 0 {
		config.Cache.TTL = 30 * time.Second // 默认30秒缓存TTL
	}

	client := client.NewBaseHTTPClient(
		config.MockErrorService.URL,
		config.MockErrorService.Timeout,
		"metric-injector",
		logger,
	)

	injector := &MetricInjector{
		mockErrorClient: client,
		serviceName:     serviceName,
		logger:          logger,
		cache:           make(map[string]*CachedAnomaly),
		cacheTTL:        config.Cache.TTL,
		// 初始化真实资源注入器
		cpuInjector:     NewCPUSpikeInjector(logger),
		memoryInjector:  NewMemoryLeakInjector(logger),
		diskInjector:    NewDiskFullInjector(logger, ""),
		networkInjector: NewNetworkFloodInjector(logger),
		machineInjector: NewMachineDownInjector(logger),
	}

	// 启动缓存清理协程
	injector.StartCacheCleanup()

	return injector, nil
}

// NewMetricInjectorWithDefaults 使用默认配置创建指标异常注入器
func NewMetricInjectorWithDefaults(mockErrorServiceURL string, serviceName string, logger *observability.Logger) *MetricInjector {
	client := client.NewBaseHTTPClient(mockErrorServiceURL, 5*time.Second, "metric-injector", logger)

	injector := &MetricInjector{
		mockErrorClient: client,
		serviceName:     serviceName,
		logger:          logger,
		cache:           make(map[string]*CachedAnomaly),
		cacheTTL:        30 * time.Second,
		// 初始化真实资源注入器
		cpuInjector:     NewCPUSpikeInjector(logger),
		memoryInjector:  NewMemoryLeakInjector(logger),
		diskInjector:    NewDiskFullInjector(logger, ""),
		networkInjector: NewNetworkFloodInjector(logger),
		machineInjector: NewMachineDownInjector(logger),
	}

	injector.StartCacheCleanup()
	return injector
}

// InjectMetricAnomaly 检查并注入指标异常
func (mi *MetricInjector) InjectMetricAnomaly(ctx context.Context, metricName string, originalValue float64) float64 {
	// 计算实例标识，用于实例级注入与缓存
	instanceID := utils.GetInstanceID(mi.serviceName)

	// 检查缓存（加入实例维度）
	cacheKey := mi.serviceName + ":" + instanceID + ":" + metricName
	mi.cacheMu.RLock()
	if cached, exists := mi.cache[cacheKey]; exists && time.Now().Before(cached.ExpiresAt) {
		mi.cacheMu.RUnlock()
		if cached.Anomaly != nil {
			return mi.applyAnomaly(ctx, cached.Anomaly, originalValue, metricName)
		}
		return originalValue
	}
	mi.cacheMu.RUnlock()

	// 查询Mock Error Service获取异常规则
	request := map[string]string{
		"service":     mi.serviceName,
		"metric_name": metricName,
		"instance":    instanceID,
	}

	var response struct {
		ShouldInject bool           `json:"should_inject"`
		Service      string         `json:"service"`
		MetricName   string         `json:"metric_name"`
		Instance     string         `json:"instance"`
		Anomaly      map[string]any `json:"anomaly,omitempty"`
	}

	// 使用较短的超时时间避免影响正常指标收集
	opts := client.RequestOptions{
		Method: "POST",
		Path:   "/api/v1/metric-inject/check",
		Body:   request,
	}

	err := mi.mockErrorClient.DoRequestWithJSON(ctx, opts, &response)
	if err != nil {
		mi.logger.Debug(ctx, "Failed to check metric injection",
			observability.Error(err),
			observability.String("metric_name", metricName))
		// 失败时缓存空结果，避免频繁请求
		mi.updateCache(cacheKey, nil)
		return originalValue
	}

	// 更新缓存
	var anomaly map[string]any
	if response.ShouldInject {
		anomaly = response.Anomaly
	}
	mi.updateCache(cacheKey, anomaly)

	// 启动真实资源消耗注入异常
	if response.ShouldInject && response.Anomaly != nil {
		return mi.applyAnomaly(ctx, response.Anomaly, originalValue, metricName)
	}

	return originalValue
}

// updateCache 更新缓存
func (mi *MetricInjector) updateCache(key string, anomaly map[string]any) {
	mi.cacheMu.Lock()
	defer mi.cacheMu.Unlock()

	mi.cache[key] = &CachedAnomaly{
		Anomaly:   anomaly,
		ExpiresAt: time.Now().Add(mi.cacheTTL),
	}
}

// applyAnomaly 应用指标异常
func (mi *MetricInjector) applyAnomaly(ctx context.Context, anomaly map[string]any, originalValue float64, metricName string) float64 {
	anomalyType, ok := anomaly["anomaly_type"].(string)
	if !ok {
		return originalValue
	}

	targetValueRaw, ok := anomaly["target_value"]
	if !ok {
		return originalValue
	}

	var targetValue float64
	switch v := targetValueRaw.(type) {
	case float64:
		targetValue = v
	case float32:
		targetValue = float64(v)
	case int:
		targetValue = float64(v)
	case int64:
		targetValue = float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			targetValue = parsed
		} else {
			return originalValue
		}
	default:
		return originalValue
	}

	// 获取持续时间，默认为30秒
	duration := 30 * time.Second
	if durationRaw, exists := anomaly["duration"]; exists {
		if durationStr, ok := durationRaw.(string); ok {
			if parsed, err := time.ParseDuration(durationStr); err == nil {
				duration = parsed
			}
		}
	}

	ruleID, _ := anomaly["rule_id"].(string)

	mi.logger.Info(ctx, "Starting real resource consumption for anomaly",
		observability.String("metric_name", metricName),
		observability.String("anomaly_type", anomalyType),
		observability.Float64("original_value", originalValue),
		observability.Float64("target_value", targetValue),
		observability.String("duration", duration.String()),
		observability.String("rule_id", ruleID))

	// 启动真实资源消耗
	switch anomalyType {
	case models.AnomalyCPUSpike:
		// 启动CPU峰值注入
		if !mi.cpuInjector.IsActive() {
			mi.cpuInjector.StartCPUSpike(ctx, targetValue, duration)
		}
		// CPU异常返回真实测量的CPU使用率
		return mi.cpuInjector.GetCurrentCPUUsage()

	case models.AnomalyMemoryLeak:
		// 启动内存泄露注入
		if !mi.memoryInjector.IsActive() {
			mi.memoryInjector.StartMemoryLeak(ctx, int64(targetValue), duration)
		}
		// 内存泄露返回当前已分配的内存量
		currentMB := mi.memoryInjector.GetCurrentMemoryMB()
		if currentMB > 0 {
			return float64(currentMB)
		}
		return originalValue

	case models.AnomalyDiskFull:
		// 启动磁盘满载注入
		if !mi.diskInjector.IsActive() {
			mi.diskInjector.StartDiskFull(ctx, targetValue, duration)
		}
		// 磁盘满载返回当前真实磁盘使用率
		return mi.diskInjector.GetCurrentDiskUsage()

	case models.AnomalyNetworkFlood:
		// 启动网络风暴注入
		if !mi.networkInjector.IsActive() {
			mi.networkInjector.StartNetworkFlood(ctx, int(targetValue), duration)
		}
		// 网络风暴返回当前连接数
		currentConns := mi.networkInjector.GetCurrentConnections()
		if currentConns > 0 {
			return float64(currentConns)
		}
		return originalValue

	case models.AnomalyMachineDown:
		// 启动机器宕机模拟
		if !mi.machineInjector.IsActive() {
			simulationType := "service_hang" // 默认模拟类型
			if simTypeRaw, exists := anomaly["simulation_type"]; exists {
				if simType, ok := simTypeRaw.(string); ok {
					simulationType = simType
				}
			}
			mi.machineInjector.StartMachineDown(ctx, simulationType, duration)
		}
		// 机器宕机返回0表示服务不可用
		return 0

	default:
		mi.logger.Warn(ctx, "Unknown anomaly type, returning original value",
			observability.String("anomaly_type", anomalyType))
		return originalValue
	}
}

// HTTPMiddleware HTTP中间件 - 用于HTTP请求级别的异常注入
func (mi *MetricInjector) HTTPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 可以在这里添加HTTP级别的错误注入逻辑
			// 例如：延迟响应、返回错误状态码、断开连接等
			// 目前主要通过InjectMetricAnomaly方法在指标收集时注入异常
			next.ServeHTTP(w, r)
		})
	}
}

// CleanupCache 清理过期缓存
func (mi *MetricInjector) CleanupCache() {
	mi.cacheMu.Lock()
	defer mi.cacheMu.Unlock()

	now := time.Now()
	for key, cached := range mi.cache {
		if now.After(cached.ExpiresAt) {
			delete(mi.cache, key)
		}
	}
}

// StartCacheCleanup 启动缓存清理协程
func (mi *MetricInjector) StartCacheCleanup() {
	go func() {
		ticker := time.NewTicker(mi.cacheTTL)
		defer ticker.Stop()

		for range ticker.C {
			mi.CleanupCache()
		}
	}()
}

// Cleanup 清理所有资源
func (mi *MetricInjector) Cleanup() {
	mi.logger.Info(context.Background(), "Cleaning up MetricInjector resources")

	// 清理所有真实资源注入器
	if mi.cpuInjector != nil {
		mi.cpuInjector.Cleanup()
	}
	if mi.memoryInjector != nil {
		mi.memoryInjector.Cleanup()
	}
	if mi.diskInjector != nil {
		mi.diskInjector.Cleanup()
	}
	if mi.networkInjector != nil {
		mi.networkInjector.Cleanup()
	}
	if mi.machineInjector != nil {
		mi.machineInjector.Cleanup()
	}

	// 清理缓存
	mi.CleanupCache()
}

// GetAnomalyStatus 获取当前异常状态信息
func (mi *MetricInjector) GetAnomalyStatus(ctx context.Context) map[string]any {
	status := make(map[string]any)

	// CPU异常状态
	if mi.cpuInjector != nil {
		status["cpu_spike_active"] = mi.cpuInjector.IsActive()
	}

	// 内存异常状态
	if mi.memoryInjector != nil {
		status["memory_leak_active"] = mi.memoryInjector.IsActive()
		status["current_memory_mb"] = mi.memoryInjector.GetCurrentMemoryMB()
	}

	// 磁盘异常状态
	if mi.diskInjector != nil {
		status["disk_full_active"] = mi.diskInjector.IsActive()
		status["current_disk_usage_percent"] = mi.diskInjector.GetCurrentDiskUsage()
	}

	// 网络异常状态
	if mi.networkInjector != nil {
		status["network_flood_active"] = mi.networkInjector.IsActive()
		status["current_connections"] = mi.networkInjector.GetCurrentConnections()
	}

	// 机器异常状态
	if mi.machineInjector != nil {
		status["machine_down_active"] = mi.machineInjector.IsActive()
	}

	return status
}

// StopAllAnomalies 停止所有当前活跃的异常注入
func (mi *MetricInjector) StopAllAnomalies(ctx context.Context) {
	mi.logger.Info(ctx, "Stopping all active anomaly injections")

	// 停止CPU异常
	if mi.cpuInjector != nil && mi.cpuInjector.IsActive() {
		mi.cpuInjector.StopCPUSpike(ctx)
	}

	// 停止内存异常
	if mi.memoryInjector != nil && mi.memoryInjector.IsActive() {
		mi.memoryInjector.StopMemoryLeak(ctx)
	}

	// 停止磁盘异常
	if mi.diskInjector != nil && mi.diskInjector.IsActive() {
		mi.diskInjector.StopDiskFull(ctx)
	}

	// 停止网络异常
	if mi.networkInjector != nil && mi.networkInjector.IsActive() {
		mi.networkInjector.StopNetworkFlood(ctx)
	}

	// 停止机器异常
	if mi.machineInjector != nil && mi.machineInjector.IsActive() {
		mi.machineInjector.StopMachineDown(ctx)
	}
}
