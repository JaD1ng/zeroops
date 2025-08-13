package faults

import (
	"fmt"
	"net/http"
)

// ErrorType 错误类型枚举
type ErrorType string

const (
	// 客户端错误
	ErrorTypeBadRequest ErrorType = "BAD_REQUEST"
	ErrorTypeNotFound   ErrorType = "NOT_FOUND"
	ErrorTypeConflict   ErrorType = "CONFLICT"

	// 服务端错误
	ErrorTypeInternal      ErrorType = "INTERNAL_ERROR"
	ErrorTypeTimeout       ErrorType = "TIMEOUT"
	ErrorTypeDatabase      ErrorType = "DATABASE_ERROR"
	ErrorTypeStorage       ErrorType = "STORAGE_ERROR"
	ErrorTypeGoroutineLeak ErrorType = "GOROUTINE_LEAK"

	// 业务错误
	ErrorTypeBucketExists   ErrorType = "BUCKET_ALREADY_EXISTS"
	ErrorTypeBucketNotFound ErrorType = "BUCKET_NOT_FOUND"
	ErrorTypeObjectNotFound ErrorType = "OBJECT_NOT_FOUND"
)

// AppError 应用错误
type AppError struct {
	Type      ErrorType `json:"type"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	RequestID string    `json:"request_id,omitempty"`
	Cause     error     `json:"-"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// HTTPStatusCode 返回对应的HTTP状态码
func (e *AppError) HTTPStatusCode() int {
	switch e.Type {
	case ErrorTypeBadRequest:
		return http.StatusBadRequest
	case ErrorTypeNotFound, ErrorTypeBucketNotFound, ErrorTypeObjectNotFound:
		return http.StatusNotFound
	case ErrorTypeConflict, ErrorTypeBucketExists:
		return http.StatusConflict
	case ErrorTypeTimeout:
		return http.StatusRequestTimeout
	default:
		return http.StatusInternalServerError
	}
}

// NewError 创建应用错误
func NewError(errorType ErrorType, code, message string) *AppError {
	return &AppError{
		Type:    errorType,
		Code:    code,
		Message: message,
	}
}
