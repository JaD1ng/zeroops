package model

import "fmt"

// ===== 错误响应结构体 =====

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Service   string `json:"service,omitempty"`
	Metric    string `json:"metric,omitempty"`
	Parameter string `json:"parameter,omitempty"`
	Value     string `json:"value,omitempty"`
}

// ===== 自定义错误类型 =====

// ServiceNotFoundError 服务不存在错误
type ServiceNotFoundError struct {
	Service string
}

func (e *ServiceNotFoundError) Error() string {
	return fmt.Sprintf("服务 '%s' 不存在", e.Service)
}

// MetricNotFoundError 指标不存在错误
type MetricNotFoundError struct {
	Metric string
}

func (e *MetricNotFoundError) Error() string {
	return fmt.Sprintf("指标 '%s' 不存在", e.Metric)
}

// PrometheusError Prometheus 查询错误
type PrometheusError struct {
	Message string
}

func (e *PrometheusError) Error() string {
	return fmt.Sprintf("Prometheus 查询错误: %s", e.Message)
}
