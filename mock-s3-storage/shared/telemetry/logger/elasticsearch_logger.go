package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"shared/config"
	"time"
)

// ElasticsearchLogger Elasticsearch日志实现
type ElasticsearchLogger struct {
	config      config.ElasticsearchConfig
	httpClient  *http.Client
	indexName   string
	serviceName string
	hostID      string
}

// LogEntry 日志条目结构
type LogEntry struct {
	Timestamp string                 `json:"@timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Service   string                 `json:"service"`
	HostID    string                 `json:"host_id,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Error     string                 `json:"error,omitempty"`
	TraceID   string                 `json:"trace_id,omitempty"`
	SpanID    string                 `json:"span_id,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// NewElasticsearchLogger 创建Elasticsearch日志器
func NewElasticsearchLogger(cfg config.ElasticsearchConfig, serviceName string) *ElasticsearchLogger {
	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 生成索引名称（按日期分片）
	indexName := fmt.Sprintf("%s-%s", cfg.Index, time.Now().Format("2006.01.02"))

	// 获取主机ID
	hostID := GetHostID()

	return &ElasticsearchLogger{
		config:      cfg,
		httpClient:  client,
		indexName:   indexName,
		serviceName: serviceName,
		hostID:      hostID,
	}
}

// Info 记录信息级别日志
func (l *ElasticsearchLogger) Info(ctx context.Context, message string, fields map[string]any) {
	l.log(ctx, "info", message, "", fields)
}

// Error 记录错误级别日志
func (l *ElasticsearchLogger) Error(ctx context.Context, message string, err error, fields map[string]any) {
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	l.log(ctx, "error", message, errMsg, fields)
}

// Debug 记录调试级别日志
func (l *ElasticsearchLogger) Debug(ctx context.Context, message string, fields map[string]any) {
	l.log(ctx, "debug", message, "", fields)
}

// Warn 记录警告级别日志
func (l *ElasticsearchLogger) Warn(ctx context.Context, message string, fields map[string]any) {
	l.log(ctx, "warn", message, "", fields)
}

// GetHostID 获取主机ID
func (l *ElasticsearchLogger) GetHostID() string {
	return l.hostID
}

// log 内部日志记录方法
func (l *ElasticsearchLogger) log(ctx context.Context, level, message, errMsg string, fields map[string]any) {
	// 构建日志条目
	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Service:   l.serviceName,
		HostID:    l.hostID,
		Fields:    fields,
	}

	if errMsg != "" {
		entry.Error = errMsg
	}

	// 从上下文中提取追踪信息（如果有的话）
	if traceID := ctx.Value("trace_id"); traceID != nil {
		if id, ok := traceID.(string); ok {
			entry.TraceID = id
		}
	}
	if spanID := ctx.Value("span_id"); spanID != nil {
		if id, ok := spanID.(string); ok {
			entry.SpanID = id
		}
	}

	// 从上下文中提取request_id
	if requestID := ctx.Value("request_id"); requestID != nil {
		if id, ok := requestID.(string); ok {
			entry.RequestID = id
		}
	}

	// 序列化日志条目
	jsonData, err := json.Marshal(entry)
	if err != nil {
		// 如果序列化失败，回退到控制台输出
		fmt.Printf("日志序列化失败: %v\n", err)
		return
	}

	// 发送到Elasticsearch
	l.sendToElasticsearch(jsonData)
}

// sendToElasticsearch 发送日志到Elasticsearch
func (l *ElasticsearchLogger) sendToElasticsearch(jsonData []byte) {
	// 构建URL
	scheme := "http"
	if l.config.UseSSL {
		scheme = "https"
	}

	url := fmt.Sprintf("%s://%s:%d/%s/_doc", scheme, l.config.Host, l.config.Port, l.indexName)

	// 创建请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("创建HTTP请求失败: %v\n", err)
		return
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	if l.config.Username != "" && l.config.Password != "" {
		req.SetBasicAuth(l.config.Username, l.config.Password)
	}

	// 发送请求（带重试机制）
	var resp *http.Response
	for i := 0; i <= l.config.MaxRetries; i++ {
		resp, err = l.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			break
		}

		if i < l.config.MaxRetries {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	if err != nil {
		fmt.Printf("发送日志到Elasticsearch失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode >= 400 {
		fmt.Printf("Elasticsearch响应错误: %s\n", resp.Status)
	}
}
