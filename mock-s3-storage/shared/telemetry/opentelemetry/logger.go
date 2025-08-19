package opentelemetry

import (
	"context"
	"fmt"
	"log"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// Logger 日志记录器
type Logger struct {
	config Config
	level  LogLevel
}

// NewLogger 创建日志记录器
func NewLogger(config Config) *Logger {
	level := INFO
	if config.Environment == "development" {
		level = DEBUG
	}

	return &Logger{
		config: config,
		level:  level,
	}
}

// Debug 调试日志
func (l *Logger) Debug(ctx context.Context, message string, fields map[string]interface{}) {
	if l.level <= DEBUG {
		l.log(ctx, "DEBUG", message, fields)
	}
}

// Info 信息日志
func (l *Logger) Info(ctx context.Context, message string, fields map[string]interface{}) {
	if l.level <= INFO {
		l.log(ctx, "INFO", message, fields)
	}
}

// Warn 警告日志
func (l *Logger) Warn(ctx context.Context, message string, fields map[string]interface{}) {
	if l.level <= WARN {
		l.log(ctx, "WARN", message, fields)
	}
}

// Error 错误日志
func (l *Logger) Error(ctx context.Context, message string, err error, fields map[string]interface{}) {
	if l.level <= ERROR {
		if fields == nil {
			fields = make(map[string]interface{})
		}
		if err != nil {
			fields["error"] = err.Error()
		}
		l.log(ctx, "ERROR", message, fields)
	}
}

// log 内部日志记录方法
func (l *Logger) log(ctx context.Context, level, message string, fields map[string]interface{}) {
	// 构建日志条目
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     level,
		"message":   message,
		"service":   l.config.ServiceName,
		"version":   l.config.ServiceVersion,
		"env":       l.config.Environment,
	}

	// 添加Request ID（如果可用）
	if requestID := getRequestID(ctx); requestID != "" {
		logEntry["request_id"] = requestID
	}

	// 添加字段
	if fields != nil {
		for k, v := range fields {
			logEntry[k] = v
		}
	}

	// 不添加trace_id和span_id，保持为空
	// 这样日志中就不会有这些字段，或者字段值为空

	// 输出日志（这里可以替换为其他输出方式，如Elasticsearch）
	log.Printf("[%s] %s: %s", level, l.config.ServiceName, message)
	if len(fields) > 0 {
		log.Printf("Fields: %+v", fields)
	}
}

// getRequestID 从上下文中获取request_id
func getRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}
	return ""
}

// getTraceID 从上下文中获取trace_id
func getTraceID(ctx context.Context) string {
	// 这里可以集成OpenTelemetry的trace ID提取
	// 暂时返回空字符串
	return ""
}

// getSpanID 从上下文中获取span_id
func getSpanID(ctx context.Context) string {
	// 这里可以集成OpenTelemetry的span ID提取
	// 暂时返回空字符串
	return ""
}

// LogHTTPRequest 记录HTTP请求日志
func (l *Logger) LogHTTPRequest(ctx context.Context, method, path, status string, duration time.Duration, size int64) {
	fields := map[string]interface{}{
		"method":      method,
		"path":        path,
		"status":      status,
		"duration_ms": duration.Milliseconds(),
		"size":        size,
	}

	if statusCode := getStatusCode(status); statusCode >= 400 {
		l.Error(ctx, fmt.Sprintf("HTTP %s %s", method, path), nil, fields)
	} else {
		l.Info(ctx, fmt.Sprintf("HTTP %s %s", method, path), fields)
	}
}

// LogFileOperation 记录文件操作日志
func (l *Logger) LogFileOperation(ctx context.Context, operation, fileID, status string, size int64) {
	fields := map[string]interface{}{
		"operation": operation,
		"file_id":   fileID,
		"status":    status,
		"size":      size,
	}

	if status == "error" {
		l.Error(ctx, fmt.Sprintf("File %s failed", operation), nil, fields)
	} else {
		l.Info(ctx, fmt.Sprintf("File %s completed", operation), fields)
	}
}

// LogSystemEvent 记录系统事件日志
func (l *Logger) LogSystemEvent(ctx context.Context, event string, fields map[string]interface{}) {
	l.Info(ctx, fmt.Sprintf("System event: %s", event), fields)
}

// LogError 记录错误日志
func (l *Logger) LogError(ctx context.Context, operation string, err error, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["operation"] = operation
	l.Error(ctx, fmt.Sprintf("Operation %s failed", operation), err, fields)
}

// getStatusCode 从状态字符串中提取状态码
func getStatusCode(status string) int {
	// 简单的状态码提取，可以根据需要改进
	switch status {
	case "200":
		return 200
	case "201":
		return 201
	case "400":
		return 400
	case "401":
		return 401
	case "403":
		return 403
	case "404":
		return 404
	case "500":
		return 500
	default:
		return 200 // 默认值
	}
}
