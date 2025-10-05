package healthcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	adb "github.com/qiniu/zeroops/internal/alerting/database"
	"github.com/qiniu/zeroops/internal/config"
	"github.com/rs/zerolog/log"
)

// PrometheusConfig holds configuration for Prometheus client
type PrometheusConfig struct {
	BaseURL           string
	QueryTimeout      time.Duration
	AnomalyAPIURL     string
	AnomalyAPITimeout time.Duration
}

// NewPrometheusConfigFromEnv creates PrometheusConfig from environment variables
func NewPrometheusConfigFromEnv() *PrometheusConfig {
	return &PrometheusConfig{
		BaseURL:           getEnvOrDefault("PROMETHEUS_URL", "http://localhost:9090"),
		QueryTimeout:      getDurationFromEnv("PROMETHEUS_QUERY_TIMEOUT", 30*time.Second),
		AnomalyAPIURL:     getEnvOrDefault("ANOMALY_DETECTION_API_URL", "http://localhost:8081/api/v1/anomaly/detect"),
		AnomalyAPITimeout: getDurationFromEnv("ANOMALY_DETECTION_API_TIMEOUT", 10*time.Second),
	}
}

// NewPrometheusConfigFromApp converts app config to runtime PrometheusConfig
func NewPrometheusConfigFromApp(c *config.PrometheusConfig) *PrometheusConfig {
	if c == nil {
		return NewPrometheusConfigFromEnv()
	}
	qt, _ := time.ParseDuration(getNonEmpty(c.QueryTimeout, "30s"))
	at, _ := time.ParseDuration(getNonEmpty(c.AnomalyAPITimeout, "10s"))
	return &PrometheusConfig{
		BaseURL:           getNonEmpty(c.URL, "http://localhost:9090"),
		QueryTimeout:      qt,
		AnomalyAPIURL:     getNonEmpty(c.AnomalyAPIURL, "http://localhost:8081/api/v1/anomaly/detect"),
		AnomalyAPITimeout: at,
	}
}

// PrometheusClient handles communication with Prometheus
type PrometheusClient struct {
	config     *PrometheusConfig
	httpClient *http.Client
}

// NewPrometheusClient creates a new Prometheus client
func NewPrometheusClient(config *PrometheusConfig) *PrometheusClient {
	return &PrometheusClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.QueryTimeout,
		},
	}
}

// QueryRange executes a Prometheus query_range request
func (c *PrometheusClient) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*PrometheusResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(start.Unix(), 10))
	params.Set("end", strconv.FormatInt(end.Unix(), 10))
	params.Set("step", strconv.FormatInt(int64(step.Seconds()), 10))

	reqURL := fmt.Sprintf("%s/api/v1/query_range?%s", c.config.BaseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result PrometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus returned error status: %s", result.Status)
	}

	return &result, nil
}

// DetectAnomalies calls the anomaly detection API for each time series individually
func (c *PrometheusClient) DetectAnomalies(ctx context.Context, timeSeries []PrometheusTimeSeries, queries []PrometheusQuery) (*AnomalyDetectionResponse, error) {
	var allAnomalies []Anomaly

	// Process each time series individually
	for i, ts := range timeSeries {
		log.Debug().Int("index", i).Msg("Processing time series for anomaly detection")

		// Find the corresponding query for this time series
		var query *PrometheusQuery
		if i < len(queries) {
			query = &queries[i]
		}

		// Call anomaly detection API for this single time series
		anomalies, err := c.detectAnomaliesForSingleTimeSeries(ctx, ts, query)
		if err != nil {
			log.Error().Err(err).Int("index", i).Msg("Failed to detect anomalies for time series")
			continue // Continue with next time series
		}

		allAnomalies = append(allAnomalies, anomalies...)
	}

	return &AnomalyDetectionResponse{
		Anomalies: allAnomalies,
	}, nil
}

// detectAnomaliesForSingleTimeSeries calls the anomaly detection API for a single time series

func (c *PrometheusClient) detectAnomaliesForSingleTimeSeries(ctx context.Context, timeSeries PrometheusTimeSeries, query *PrometheusQuery) ([]Anomaly, error) {
	// Build request body with metadata and time series data in the new format
	dataPoints := make([]map[string]interface{}, 0, len(timeSeries.Values))
	for _, pair := range timeSeries.Values {
		if len(pair) < 2 {
			continue
		}
		// Parse timestamp (unix seconds, can be float or string)
		var tsInt int64
		switch v := pair[0].(type) {
		case float64:
			tsInt = int64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				tsInt = int64(f)
			}
		}
		if tsInt == 0 {
			continue
		}
		tsRFC3339 := time.Unix(tsInt, 0).UTC().Format(time.RFC3339)

		// Parse value (usually string from Prometheus)
		var valFloat float64
		switch vv := pair[1].(type) {
		case string:
			if f, err := strconv.ParseFloat(vv, 64); err == nil {
				valFloat = f
			} else {
				continue
			}
		case float64:
			valFloat = vv
		default:
			continue
		}

		dataPoints = append(dataPoints, map[string]interface{}{
			"timestamp": tsRFC3339,
			"value":     valFloat,
		})
	}

	requestBody := map[string]interface{}{
		"metadata": map[string]interface{}{
			"alert_name": "",
			"severity":   "",
			"labels":     map[string]string{},
		},
		"data": dataPoints,
	}
	if query != nil {
		requestBody["metadata"] = map[string]interface{}{
			"alert_name": query.AlertName,
			"severity":   query.Severity,
			"labels":     query.Labels,
		}
	}

	reqBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.AnomalyAPIURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: c.config.AnomalyAPITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anomaly detection API failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result AnomalyDetectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Anomalies, nil
}

// BuildPromQL constructs a complete PromQL query from alert rule and meta
func BuildPromQL(rule *AlertRule, meta *AlertRuleMeta) string {
	expr := rule.Expr

	// If the expression already contains label filters, we need to merge them
	// For simplicity, we'll assume the expression has empty label filters like {}
	if strings.Contains(expr, "{}") {
		// Replace {} with the actual label filters
		labelFilters := make([]string, 0, len(meta.Labels))
		for key, value := range meta.Labels {
			labelFilters = append(labelFilters, fmt.Sprintf(`%s="%s"`, key, value))
		}

		if len(labelFilters) > 0 {
			labelStr := strings.Join(labelFilters, ",")
			expr = strings.Replace(expr, "{}", "{"+labelStr+"}", 1)
		}
	}

	return expr
}

// QueryAlertRules fetches all alert rules from database
func QueryAlertRules(ctx context.Context, db *adb.Database) ([]AlertRule, error) {
	if db == nil {
		return []AlertRule{}, nil
	}

	const query = `SELECT name, description, expr, op, severity, watch_time FROM alert_rules`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query alert rules: %w", err)
	}
	defer rows.Close()

	var rules []AlertRule
	for rows.Next() {
		var rule AlertRule
		if err := rows.Scan(&rule.Name, &rule.Description, &rule.Expr, &rule.Op, &rule.Severity, &rule.WatchTime); err != nil {
			return nil, fmt.Errorf("failed to scan alert rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, rows.Err()
}

// QueryAlertRuleMetas fetches all alert rule metas from database
func QueryAlertRuleMetas(ctx context.Context, db *adb.Database) ([]AlertRuleMeta, error) {
	if db == nil {
		return []AlertRuleMeta{}, nil
	}

	const query = `SELECT alert_name, labels, threshold FROM alert_rule_metas`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query alert rule metas: %w", err)
	}
	defer rows.Close()

	var metas []AlertRuleMeta
	for rows.Next() {
		var meta AlertRuleMeta
		var labelsJSON string
		if err := rows.Scan(&meta.AlertName, &labelsJSON, &meta.Threshold); err != nil {
			return nil, fmt.Errorf("failed to scan alert rule meta: %w", err)
		}

		// Parse labels JSON
		if err := json.Unmarshal([]byte(labelsJSON), &meta.Labels); err != nil {
			log.Warn().Err(err).Str("labels", labelsJSON).Msg("failed to parse labels JSON, using empty map")
			meta.Labels = make(map[string]string)
		}

		metas = append(metas, meta)
	}

	return metas, rows.Err()
}

// QueryAlertIssuesForTimeFilter fetches alert issues for time range filtering
func QueryAlertIssuesForTimeFilter(ctx context.Context, db *adb.Database) ([]AlertIssue, error) {
	if db == nil {
		return []AlertIssue{}, nil
	}

	const query = `SELECT id, alert_since, resolved_at, alert_state FROM alert_issues WHERE alert_state IN ('InProcessing', 'Pending')`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query alert issues: %w", err)
	}
	defer rows.Close()

	var issues []AlertIssue
	for rows.Next() {
		var issue AlertIssue
		if err := rows.Scan(&issue.ID, &issue.AlertSince, &issue.ResolvedAt, &issue.AlertState); err != nil {
			return nil, fmt.Errorf("failed to scan alert issue: %w", err)
		}
		issues = append(issues, issue)
	}

	return issues, rows.Err()
}

// FilterAnomaliesByAlertTimeRanges filters anomalies based on existing alert time ranges
func FilterAnomaliesByAlertTimeRanges(anomalies []Anomaly, alertIssues []AlertIssue) []Anomaly {
	var filteredAnomalies []Anomaly

	for _, anomaly := range anomalies {
		anomalyStart := time.Unix(anomaly.Start, 0)
		anomalyEnd := time.Unix(anomaly.End, 0)

		// Check if anomaly time range overlaps with any existing alert
		shouldSkip := false
		for _, issue := range alertIssues {
			// If alert is resolved, check if anomaly is within the alert time range
			if issue.ResolvedAt != nil {
				if anomalyStart.After(issue.AlertSince) && anomalyEnd.Before(*issue.ResolvedAt) {
					shouldSkip = true
					break
				}
			} else {
				// If alert is not resolved, check if anomaly is after alert start
				if anomalyStart.After(issue.AlertSince) {
					shouldSkip = true
					break
				}
			}
		}

		if !shouldSkip {
			filteredAnomalies = append(filteredAnomalies, anomaly)
		}
	}

	return filteredAnomalies
}

// Helper functions
func getEnvOrDefault(key, defaultValue string) string                         { return defaultValue }
func getDurationFromEnv(key string, defaultValue time.Duration) time.Duration { return defaultValue }

func getNonEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
