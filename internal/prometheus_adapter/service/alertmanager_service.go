package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	promconfig "github.com/qiniu/zeroops/internal/prometheus_adapter/config"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/model"
	"github.com/rs/zerolog/log"
)

// AlertmanagerService Alertmanager 服务
// 接收 Prometheus 的告警推送并转发到监控告警模块
type AlertmanagerService struct {
	config         *promconfig.PrometheusAdapterConfig
	webhookURL     string
	httpClient     *http.Client
	resolveTimeout time.Duration
}

// NewAlertmanagerProxyService 创建新的 Alertmanager 代理服务
func NewAlertmanagerProxyService(config *promconfig.PrometheusAdapterConfig) *AlertmanagerService {
	return &AlertmanagerService{
		config:         config,
		webhookURL:     config.AlertWebhook.URL,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		resolveTimeout: 5 * time.Minute, // 默认 resolve_timeout
	}
}

// HandleAlertsV2 处理 Prometheus 推送的告警
// 实现 POST /api/v2/alerts 接口
func (s *AlertmanagerService) HandleAlertsV2(w http.ResponseWriter, r *http.Request) {
	// 检查 Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "" {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	// 解析 Prometheus 发送的告警
	var alerts []model.AlertmanagerAlert
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &alerts); err != nil {
		log.Error().
			Err(err).
			Str("body", string(body)).
			Msg("Failed to unmarshal alerts")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 处理时间戳：如果缺失则设置默认值
	now := time.Now()
	for i := range alerts {
		// 如果 startsAt 缺失，设置为当前时间
		if alerts[i].StartsAt == "" {
			alerts[i].StartsAt = now.Format(time.RFC3339)
		}
		// 如果 endsAt 缺失，设置为当前时间 + resolve_timeout
		if alerts[i].EndsAt == "" {
			alerts[i].EndsAt = now.Add(s.resolveTimeout).Format(time.RFC3339)
		}
	}

	log.Info().
		Int("alert_count", len(alerts)).
		Msg("Received alerts from Prometheus")

	// 转发告警到监控模块
	if err := s.forwardAlertsV2(alerts); err != nil {
		log.Error().Err(err).Msg("Failed to forward alerts")
		// 返回 500 让 Prometheus 重试
		http.Error(w, "Failed to forward alerts", http.StatusInternalServerError)
		return
	}

	// 返回成功响应（Alertmanager API v2 返回空 JSON）
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}"))
}

// HandleHealthCheck 健康检查接口
// 实现 GET /-/healthy
func (s *AlertmanagerService) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// HandleReadyCheck 就绪检查接口
// 实现 GET /-/ready
func (s *AlertmanagerService) HandleReadyCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// forwardAlertsV2 转发告警到监控告警模块
func (s *AlertmanagerService) forwardAlertsV2(alerts []model.AlertmanagerAlert) error {
	// 转换为 Alertmanager webhook 格式
	webhookAlerts := []model.AlertmanagerWebhookAlert{}
	commonLabels := map[string]string{}
	groupLabels := map[string]string{}

	// 统计告警状态，用于确定总体状态
	hasFiring := false

	for _, alert := range alerts {
		// 确定告警状态：通过比较 endsAt 和当前时间
		status := "firing"
		if alert.EndsAt != "" {
			endsAtTime, err := time.Parse(time.RFC3339, alert.EndsAt)
			if err == nil && endsAtTime.Before(time.Now()) {
				status = "resolved"
			} else {
				hasFiring = true
			}
		} else {
			hasFiring = true
		}

		// 生成 fingerprint
		fingerprint := s.generateFingerprint(alert.Labels)

		// 构造 GeneratorURL
		generatorURL := alert.GeneratorURL
		if generatorURL == "" && alert.Labels["alertname"] != "" {
			generatorURL = fmt.Sprintf("http://prometheus/graph?g0.expr=%s", alert.Labels["alertname"])
		}

		webhookAlert := model.AlertmanagerWebhookAlert{
			Status:       status,
			Labels:       alert.Labels,
			Annotations:  alert.Annotations,
			StartsAt:     alert.StartsAt, // 已经是 RFC3339 格式
			EndsAt:       alert.EndsAt,   // 已经是 RFC3339 格式
			GeneratorURL: generatorURL,
			Fingerprint:  fingerprint,
		}
		webhookAlerts = append(webhookAlerts, webhookAlert)

		// 收集公共标签
		if len(commonLabels) == 0 {
			for k, v := range alert.Labels {
				commonLabels[k] = v
			}
		}
	}

	// 设置 groupLabels
	if alertName, ok := commonLabels["alertname"]; ok {
		groupLabels["alertname"] = alertName
	}

	// 确定总体状态：如果有任何 firing 的告警，总体状态为 firing，否则为 resolved
	overallStatus := "resolved"
	if hasFiring {
		overallStatus = "firing"
	}

	// 构造 webhook 请求
	req := model.AlertmanagerWebhookRequest{
		Receiver:     "prometheus_adapter",
		Status:       overallStatus, // 根据告警实际状态设置
		Alerts:       webhookAlerts,
		GroupLabels:  groupLabels,
		CommonLabels: commonLabels,
		Alert:        "REDACTED",
		Version:      "1",
	}

	// 发送到监控告警模块
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook request: %w", err)
	}

	resp, err := s.httpClient.Post(s.webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Info().
		Int("alert_count", len(alerts)).
		Str("webhook_url", s.webhookURL).
		Msg("Successfully forwarded alerts to monitoring module")

	return nil
}

// generateFingerprint 生成告警指纹
func (s *AlertmanagerService) generateFingerprint(labels map[string]string) string {
	// 简化版指纹生成
	result := ""
	for k, v := range labels {
		result += fmt.Sprintf("%s:%s,", k, v)
	}
	return fmt.Sprintf("%x", result)[:16]
}
