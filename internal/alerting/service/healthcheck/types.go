package healthcheck

import (
	"encoding/json"
	"strconv"
	"time"
)

// AlertMessage is the payload sent to downstream processors.
type AlertMessage struct {
	ID         string            `json:"id"`
	Service    string            `json:"service"`
	Version    string            `json:"version,omitempty"`
	Level      string            `json:"level"`
	Title      string            `json:"title"`
	AlertSince time.Time         `json:"alert_since"`
	Labels     map[string]string `json:"labels"`
}

// PrometheusQuery represents a PromQL query with its metadata
type PrometheusQuery struct {
	AlertName string            `json:"alert_name"`
	Expr      string            `json:"expr"`
	Labels    map[string]string `json:"labels"`
	Threshold float64           `json:"threshold"`
	Severity  string            `json:"severity"`
}

// PrometheusTimeSeries represents time series data from Prometheus
type PrometheusTimeSeries struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"` // [timestamp, value] pairs
}

// PrometheusResponse represents the response from Prometheus query_range API
type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string                 `json:"resultType"`
		Result     []PrometheusTimeSeries `json:"result"`
	} `json:"data"`
}

// Anomaly represents an anomaly detected in time series data
type Anomaly struct {
	Start int64 `json:"start"` // unix timestamp
	End   int64 `json:"end"`   // unix timestamp
}

// UnmarshalJSON supports start/end as RFC3339 string or unix seconds (number/string)
func (a *Anomaly) UnmarshalJSON(data []byte) error {
	type rawAnomaly struct {
		Start interface{} `json:"start"`
		End   interface{} `json:"end"`
	}
	var r rawAnomaly
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	a.Start = parseFlexibleTimeToUnix(r.Start)
	a.End = parseFlexibleTimeToUnix(r.End)
	return nil
}

func parseFlexibleTimeToUnix(v interface{}) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case string:
		// Try RFC3339 first
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return ts.Unix()
		}
		// Try numeric string
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return int64(f)
		}
		return 0
	default:
		return 0
	}
}

// AnomalyDetectionResponse represents the response from anomaly detection API
type AnomalyDetectionResponse struct {
	Metadata  *AnomalyMetadata `json:"metadata,omitempty"`
	Anomalies []Anomaly        `json:"anomalies"`
}

// AnomalyMetadata contains metadata about the anomaly detection request
type AnomalyMetadata struct {
	AlertName string            `json:"alert_name"`
	Severity  string            `json:"severity"`
	Labels    map[string]string `json:"labels"`
}

// AlertRule represents a row from alert_rules table
type AlertRule struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Expr        string `json:"expr"`
	Op          string `json:"op"`
	Severity    string `json:"severity"`
	WatchTime   string `json:"watch_time"`
}

// AlertRuleMeta represents a row from alert_rule_metas table
type AlertRuleMeta struct {
	AlertName string            `json:"alert_name"`
	Labels    map[string]string `json:"labels"`
	Threshold float64           `json:"threshold"`
}

// AlertIssue represents a row from alert_issues table for time filtering
type AlertIssue struct {
	ID         string     `json:"id"`
	AlertSince time.Time  `json:"alert_since"`
	ResolvedAt *time.Time `json:"resolved_at"`
	AlertState string     `json:"alert_state"`
}

// deriveHealth maps alert level to service health state.
func deriveHealth(level string) string {
	switch level {
	case "P0":
		return "Error"
	case "P1", "P2":
		return "Warning"
	default:
		return "Warning"
	}
}
