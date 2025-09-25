package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/qiniu/zeroops/internal/prometheus_adapter/client"
	promconfig "github.com/qiniu/zeroops/internal/prometheus_adapter/config"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/model"
	"github.com/rs/zerolog/log"
)

// AlertWebhookService 告警 Webhook 服务
type AlertWebhookService struct {
	promClient      *client.PrometheusClient
	config          *promconfig.PrometheusAdapterConfig
	webhookURL      string
	pollingInterval time.Duration
	httpClient      *http.Client
	alertCache      map[string]*model.PrometheusAlert // 缓存已发送的告警
	cacheMutex      sync.RWMutex
	stopCh          chan struct{}
	running         bool
	runningMutex    sync.Mutex
}

// NewAlertWebhookService 创建告警 Webhook 服务
func NewAlertWebhookService(promClient *client.PrometheusClient, config *promconfig.PrometheusAdapterConfig) *AlertWebhookService {
	return &AlertWebhookService{
		promClient:      promClient,
		config:          config,
		webhookURL:      config.AlertWebhook.URL,
		pollingInterval: config.AlertWebhook.GetPollingInterval(),
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		alertCache:      make(map[string]*model.PrometheusAlert),
		stopCh:          make(chan struct{}),
	}
}

// Start 启动告警轮询服务
func (s *AlertWebhookService) Start() error {
	s.runningMutex.Lock()
	defer s.runningMutex.Unlock()

	if s.running {
		return fmt.Errorf("alert webhook service already running")
	}

	s.running = true
	go s.pollAlerts()

	log.Info().
		Str("webhook_url", s.webhookURL).
		Dur("interval", s.pollingInterval).
		Msg("Alert webhook service started")

	return nil
}

// Stop 停止告警轮询服务
func (s *AlertWebhookService) Stop() {
	s.runningMutex.Lock()
	defer s.runningMutex.Unlock()

	if !s.running {
		return
	}

	close(s.stopCh)
	s.running = false

	log.Info().Msg("Alert webhook service stopped")
}

// pollAlerts 轮询告警
func (s *AlertWebhookService) pollAlerts() {
	ticker := time.NewTicker(s.pollingInterval)
	defer ticker.Stop()

	// 立即执行一次
	s.fetchAndProcessAlerts()

	for {
		select {
		case <-ticker.C:
			s.fetchAndProcessAlerts()
		case <-s.stopCh:
			return
		}
	}
}

// fetchAndProcessAlerts 获取并处理告警
func (s *AlertWebhookService) fetchAndProcessAlerts() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 从 Prometheus 获取告警
	alertsResp, err := s.promClient.GetAlerts(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch alerts from Prometheus")
		return
	}

	// 处理告警
	firingAlerts := []model.PrometheusAlert{}
	resolvedAlerts := []model.PrometheusAlert{}

	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// 分类告警
	currentAlerts := make(map[string]*model.PrometheusAlert)
	for _, alert := range alertsResp.Data.Alerts {
		fingerprint := s.generateFingerprint(alert)
		currentAlerts[fingerprint] = &alert

		// 检查是否是新告警或状态变更
		cachedAlert, exists := s.alertCache[fingerprint]
		if !exists || cachedAlert.State != alert.State {
			if alert.State == "firing" {
				firingAlerts = append(firingAlerts, alert)
			}
		}
	}

	// 检查已恢复的告警
	for fingerprint, cachedAlert := range s.alertCache {
		if _, exists := currentAlerts[fingerprint]; !exists {
			// 告警已恢复
			resolvedAlert := *cachedAlert
			resolvedAlert.State = "resolved"
			resolvedAlerts = append(resolvedAlerts, resolvedAlert)
		}
	}

	// 更新缓存
	s.alertCache = currentAlerts

	// 发送告警
	if len(firingAlerts) > 0 {
		if err := s.sendAlerts(firingAlerts, "firing"); err != nil {
			log.Error().Err(err).Msg("Failed to send firing alerts")
		}
	}

	if len(resolvedAlerts) > 0 {
		if err := s.sendAlerts(resolvedAlerts, "resolved"); err != nil {
			log.Error().Err(err).Msg("Failed to send resolved alerts")
		}
	}
}

// sendAlerts 发送告警到监控模块
func (s *AlertWebhookService) sendAlerts(alerts []model.PrometheusAlert, status string) error {
	webhookAlerts := []model.AlertmanagerWebhookAlert{}

	// 收集所有标签用于 groupLabels 和 commonLabels
	commonLabels := map[string]string{}
	firstAlert := true

	for _, alert := range alerts {
		// 生成 fingerprint
		fingerprint := s.generateFingerprint(alert)

		// 转换时间格式
		startsAt := alert.ActiveAt.Format(time.RFC3339)
		endsAt := "0001-01-01T00:00:00Z"
		if status == "resolved" {
			endsAt = time.Now().Format(time.RFC3339)
		}

		// 构造 GeneratorURL
		generatorURL := fmt.Sprintf("http://prometheus/graph?g0.expr=%s", alert.Labels["alertname"])

		webhookAlert := model.AlertmanagerWebhookAlert{
			Status:       status,
			Labels:       alert.Labels,
			Annotations:  alert.Annotations,
			StartsAt:     startsAt,
			EndsAt:       endsAt,
			GeneratorURL: generatorURL,
			Fingerprint:  fingerprint,
		}
		webhookAlerts = append(webhookAlerts, webhookAlert)

		// 收集公共标签（取第一个告警的标签作为公共标签）
		if firstAlert {
			for k, v := range alert.Labels {
				commonLabels[k] = v
			}
			firstAlert = false
		}
	}

	groupLabels := map[string]string{}
	if alertName, ok := commonLabels["alertname"]; ok {
		groupLabels["alertname"] = alertName
	}

	// 构造请求
	req := model.AlertmanagerWebhookRequest{
		Receiver:     "prometheus_adapter",
		Status:       status,
		Alerts:       webhookAlerts,
		GroupLabels:  groupLabels,
		CommonLabels: commonLabels,
		Version:      "1",
	}

	// 发送请求
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
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
		Str("status", status).
		Int("alert_count", len(alerts)).
		Str("webhook_url", s.webhookURL).
		Msg("Successfully sent alerts to webhook")

	return nil
}

// generateFingerprint 生成告警的唯一标识
func (s *AlertWebhookService) generateFingerprint(alert model.PrometheusAlert) string {
	// 基于标签生成指纹
	labels := ""
	for k, v := range alert.Labels {
		labels += fmt.Sprintf("%s=%s,", k, v)
	}

	h := md5.New()
	h.Write([]byte(labels))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
