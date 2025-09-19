package service

import (
	"context"
	"encoding/json"
	"fmt"
	"mocks3/shared/models"
	"mocks3/shared/observability"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MockErrorService Mock错误注入服务
type MockErrorService struct {
	config *MockErrorConfig
	logger *observability.Logger

	// 错误规则存储
	rules map[string]*models.MetricAnomalyRule
	mu    sync.RWMutex

	// 统计信息
	stats *ErrorStats
}

// ErrorStats 错误统计信息
type ErrorStats struct {
	TotalRequests  int64     `json:"total_requests"`
	InjectedErrors int64     `json:"injected_errors"`
	ActiveRules    int64     `json:"active_rules"`
	LastUpdated    time.Time `json:"last_updated"`
}

// NewMockErrorService 创建Mock错误注入服务
func NewMockErrorService(config *MockErrorConfig, logger *observability.Logger) *MockErrorService {
	service := &MockErrorService{
		config: config,
		logger: logger,
		rules:  make(map[string]*models.MetricAnomalyRule),
		stats: &ErrorStats{
			LastUpdated: time.Now(),
		},
	}

	// 加载已有的指标异常规则
	if err := service.loadRules(); err != nil {
		logger.Error(context.Background(), "Failed to load metric anomaly rules", observability.Error(err))
	}

	// 启动定期保存协程
	go service.periodicSave()

	return service
}

// CreateRule 创建指标异常规则
func (s *MockErrorService) CreateRule(ctx context.Context, rule *models.MetricAnomalyRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证规则
	if err := s.validateRule(rule); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	// 生成ID（如果未提供）
	if rule.ID == "" {
		rule.ID = generateRuleID()
	}

	// 设置创建时间
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}

	// 保存规则
	s.rules[rule.ID] = rule
	s.stats.ActiveRules = int64(len(s.rules))
	s.stats.LastUpdated = time.Now()

	s.logger.Info(ctx, "Metric anomaly rule created",
		observability.String("rule_id", rule.ID),
		observability.String("service", rule.Service),
		observability.String("metric_name", rule.MetricName),
		observability.String("anomaly_type", rule.AnomalyType))

	return nil
}

// DeleteRule 删除错误规则
func (s *MockErrorService) DeleteRule(ctx context.Context, ruleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.rules[ruleID]; !exists {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	delete(s.rules, ruleID)
	s.stats.ActiveRules = int64(len(s.rules))
	s.stats.LastUpdated = time.Now()

	s.logger.Info(ctx, "Metric anomaly rule deleted",
		observability.String("rule_id", ruleID))

	return nil
}

// ShouldInjectError 判断是否应该注入指标异常
func (s *MockErrorService) ShouldInjectError(ctx context.Context, service, metricName, instance string) (map[string]any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.stats.TotalRequests++
	s.stats.LastUpdated = time.Now()

	for _, rule := range s.rules {
		if !rule.Enabled {
			continue
		}

		// 检查服务匹配
		if rule.Service != "" && rule.Service != service {
			continue
		}
		// 检查实例匹配（如果指定了实例，则必须匹配）
		if rule.Instance != "" && rule.Instance != instance {
			continue
		}

		// 检查指标名称匹配
		if rule.MetricName != "" && rule.MetricName != metricName {
			continue
		}

		// 检查时间范围
		if rule.StartTime != nil && time.Now().Before(*rule.StartTime) {
			continue
		}

		// 检查持续时间
		if rule.Duration > 0 {
			expiryTime := rule.CreatedAt.Add(rule.Duration)
			if time.Now().After(expiryTime) {
				continue
			}
		}

		// 检查触发次数限制
		if rule.MaxTriggers > 0 && rule.Triggered >= rule.MaxTriggers {
			continue
		}

		// 匹配成功，准备异常注入
		rule.Triggered++
		s.stats.InjectedErrors++

		anomaly := map[string]any{
			"anomaly_type": rule.AnomalyType,
			"metric_name":  rule.MetricName,
			"target_value": rule.TargetValue,
			"duration":     rule.Duration.Seconds(),
			"rule_id":      rule.ID,
		}

		s.logger.Info(ctx, "Metric anomaly injected",
			observability.String("rule_id", rule.ID),
			observability.String("service", service),
			observability.String("instance", instance),
			observability.String("metric_name", metricName),
			observability.String("anomaly_type", rule.AnomalyType),
			observability.Float64("target_value", rule.TargetValue),
			observability.Int("triggered_count", rule.Triggered))

		return anomaly, true
	}

	return nil, false
}

// validateRule 验证指标异常规则
func (s *MockErrorService) validateRule(rule *models.MetricAnomalyRule) error {
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}

	if rule.MetricName == "" {
		return fmt.Errorf("metric_name is required")
	}

	if rule.AnomalyType == "" {
		return fmt.Errorf("anomaly_type is required")
	}

	// 验证异常类型是否有效
	validAnomalyTypes := map[string]bool{
		models.AnomalyCPUSpike:     true,
		models.AnomalyMemoryLeak:   true,
		models.AnomalyDiskFull:     true,
		models.AnomalyNetworkFlood: true,
		models.AnomalyMachineDown:  true,
	}

	if !validAnomalyTypes[rule.AnomalyType] {
		return fmt.Errorf("invalid anomaly_type: %s", rule.AnomalyType)
	}

	// 验证目标值
	if rule.TargetValue <= 0 {
		return fmt.Errorf("target_value must be greater than 0")
	}

	// 验证持续时间
	if rule.Duration <= 0 {
		rule.Duration = 300 * time.Second // 默认5分钟
	}

	return nil
}

// loadRules 从文件加载指标异常规则
func (s *MockErrorService) loadRules() error {
	rulesFile := filepath.Join(s.config.Storage.DataDir, "metric_anomaly_rules.json")

	// 确保目录存在
	if err := os.MkdirAll(s.config.Storage.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(rulesFile); os.IsNotExist(err) {
		return nil // 文件不存在，使用空规则
	}

	// 读取文件
	data, err := os.ReadFile(rulesFile)
	if err != nil {
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	// 解析JSON
	var rules []*models.MetricAnomalyRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return fmt.Errorf("failed to parse rules file: %w", err)
	}

	// 加载到内存
	s.rules = make(map[string]*models.MetricAnomalyRule)
	for _, rule := range rules {
		s.rules[rule.ID] = rule
	}

	s.stats.ActiveRules = int64(len(s.rules))

	return nil
}

// saveRules 保存指标异常规则到文件
func (s *MockErrorService) saveRules() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rulesFile := filepath.Join(s.config.Storage.DataDir, "metric_anomaly_rules.json")

	// 转换为切片
	rules := make([]*models.MetricAnomalyRule, 0, len(s.rules))
	for _, rule := range s.rules {
		rules = append(rules, rule)
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(rulesFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write rules file: %w", err)
	}

	return nil
}

// periodicSave 定期保存规则
func (s *MockErrorService) periodicSave() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := s.saveRules(); err != nil {
			s.logger.Error(context.Background(), "Failed to save metric anomaly rules", observability.Error(err))
		}
	}
}

// GetStats 获取统计信息
func (s *MockErrorService) GetStats(ctx context.Context) *ErrorStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本
	statsCopy := *s.stats
	return &statsCopy
}

// generateRuleID 生成规则ID
func generateRuleID() string {
	return uuid.New().String()
}
