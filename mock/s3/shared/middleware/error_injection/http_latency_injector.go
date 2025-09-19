package error_injection

import (
	"context"
	"mocks3/shared/client"
	"mocks3/shared/observability"
	"sync"
	"time"
)

// HTTPLatencyInjector HTTP请求延迟注入器
type HTTPLatencyInjector struct {
	mockErrorClient *client.BaseHTTPClient
	serviceName     string
	serviceVersion  string
	logger          *observability.Logger

	// 缓存
	cache    map[string]*CachedLatencyConfig
	cacheMu  sync.RWMutex
	cacheTTL time.Duration
}

// CachedLatencyConfig 缓存的延迟配置
type CachedLatencyConfig struct {
	Config    *LatencyConfig
	ExpiresAt time.Time
}

// LatencyConfig 延迟配置
type LatencyConfig struct {
	ShouldInject bool          `json:"should_inject"`
	Latency      time.Duration `json:"latency"`     // 注入的延迟时间
	Probability  float64       `json:"probability"` // 注入概率 (0-1)
	Pattern      string        `json:"pattern"`     // 路径匹配模式（可选）
}

// NewHTTPLatencyInjector 创建HTTP延迟注入器
func NewHTTPLatencyInjector(mockErrorServiceURL string, serviceName, serviceVersion string, logger *observability.Logger) *HTTPLatencyInjector {
	client := client.NewBaseHTTPClient(mockErrorServiceURL, 5*time.Second, "latency-injector", logger)

	injector := &HTTPLatencyInjector{
		mockErrorClient: client,
		serviceName:     serviceName,
		serviceVersion:  serviceVersion,
		logger:          logger,
		cache:           make(map[string]*CachedLatencyConfig),
		cacheTTL:        30 * time.Second,
	}

	// 启动缓存清理
	go injector.cleanupCache()

	return injector
}

// GetLatencyConfig 获取延迟配置
func (h *HTTPLatencyInjector) GetLatencyConfig(ctx context.Context, path string) (*LatencyConfig, error) {
	// 构建缓存键（基于版本）
	cacheKey := h.serviceName + ":" + h.serviceVersion + ":" + path

	// 检查缓存
	h.cacheMu.RLock()
	if cached, exists := h.cache[cacheKey]; exists && time.Now().Before(cached.ExpiresAt) {
		h.cacheMu.RUnlock()
		return cached.Config, nil
	}
	h.cacheMu.RUnlock()

	// 查询Mock Error Service获取延迟配置
	request := map[string]string{
		"service": h.serviceName,
		"version": h.serviceVersion,
		"path":    path,
		"type":    "http_latency",
	}

	var response struct {
		ShouldInject bool    `json:"should_inject"`
		Latency      int64   `json:"latency_ms"` // 毫秒
		Probability  float64 `json:"probability"`
		Pattern      string  `json:"pattern"`
	}

	opts := client.RequestOptions{
		Method: "POST",
		Path:   "/api/v1/latency-inject/check",
		Body:   request,
	}

	err := h.mockErrorClient.DoRequestWithJSON(ctx, opts, &response)
	if err != nil {
		h.logger.Debug(ctx, "Failed to check latency injection",
			observability.Error(err),
			observability.String("path", path))
		// 失败时缓存空结果
		h.updateCache(cacheKey, nil)
		return nil, nil
	}

	// 构建配置
	var config *LatencyConfig
	if response.ShouldInject {
		config = &LatencyConfig{
			ShouldInject: true,
			Latency:      time.Duration(response.Latency) * time.Millisecond,
			Probability:  response.Probability,
			Pattern:      response.Pattern,
		}
	}

	// 更新缓存
	h.updateCache(cacheKey, config)

	return config, nil
}

// InjectLatency 注入延迟（如果需要）
func (h *HTTPLatencyInjector) InjectLatency(ctx context.Context, path string) time.Duration {
	config, err := h.GetLatencyConfig(ctx, path)
	if err != nil || config == nil || !config.ShouldInject {
		return 0
	}

	// 基于概率决定是否注入
	if config.Probability < 1.0 {
		// 简单的概率实现（生产环境应使用更好的随机数）
		if time.Now().UnixNano()%100 >= int64(config.Probability*100) {
			return 0
		}
	}

	// 执行真实的延迟
	if config.Latency > 0 {
		h.logger.Info(ctx, "Injecting HTTP latency",
			observability.String("service", h.serviceName),
			observability.String("version", h.serviceVersion),
			observability.String("path", path),
			observability.Duration("latency", config.Latency))

		// 真实的延迟注入
		time.Sleep(config.Latency)

		return config.Latency
	}

	return 0
}

// updateCache 更新缓存
func (h *HTTPLatencyInjector) updateCache(key string, config *LatencyConfig) {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()

	h.cache[key] = &CachedLatencyConfig{
		Config:    config,
		ExpiresAt: time.Now().Add(h.cacheTTL),
	}
}

// cleanupCache 定期清理过期缓存
func (h *HTTPLatencyInjector) cleanupCache() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		h.cacheMu.Lock()
		now := time.Now()
		for key, cached := range h.cache {
			if now.After(cached.ExpiresAt) {
				delete(h.cache, key)
			}
		}
		h.cacheMu.Unlock()
	}
}

// Cleanup 清理资源
func (h *HTTPLatencyInjector) Cleanup() {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	h.cache = make(map[string]*CachedLatencyConfig)
}
