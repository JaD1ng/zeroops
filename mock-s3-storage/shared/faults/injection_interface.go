package faults

import "context"

// InjectionType 注入类型
type InjectionType string

const (
	InjectionTypeHTTPError     InjectionType = "http_error"     // HTTP错误注入
	InjectionTypeHTTPLatency   InjectionType = "http_latency"   // HTTP延迟注入
	InjectionTypeDBError       InjectionType = "db_error"       // 数据库错误
	InjectionTypeDBSlow        InjectionType = "db_slow"        // 数据库慢查询
	InjectionTypeStorageError  InjectionType = "storage_error"  // 存储错误
	InjectionTypeGoroutineLeak InjectionType = "goroutine_leak" // Goroutine泄漏
)

// InjectionRule 注入规则
type InjectionRule struct {
	ID       string         `json:"id"`
	Type     InjectionType  `json:"type"`
	Service  string         `json:"service"`
	Endpoint string         `json:"endpoint"`
	Rate     float64        `json:"rate"` // 注入比例 (0.0-1.0)
	Enabled  bool           `json:"enabled"`
	Config   map[string]any `json:"config"` // 配置参数

	// 要注入的错误信息
	ErrorType ErrorType `json:"error_type,omitempty"` // 对应err_types.go中的ErrorType
	ErrorCode string    `json:"error_code,omitempty"`
	ErrorMsg  string    `json:"error_message,omitempty"`
}

// InjectionEngine 注入引擎接口
type InjectionEngine interface {
	// ShouldInject 判断是否应该注入
	ShouldInject(ctx context.Context, service, endpoint string) bool

	// GetInjectionRule 获取匹配的注入规则
	GetInjectionRule(service, endpoint string) *InjectionRule

	// CreateError 根据注入规则创建对应的AppError
	CreateError(rule *InjectionRule) *AppError

	// AddRule 添加规则
	AddRule(rule *InjectionRule) error

	// RemoveRule 移除规则
	RemoveRule(ruleID string) error
}

// NewInjectionEngine 创建注入引擎
func NewInjectionEngine() InjectionEngine {
	// TODO: 实现注入引擎
	return nil
}
