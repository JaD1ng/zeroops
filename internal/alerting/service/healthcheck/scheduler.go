package healthcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	adb "github.com/qiniu/zeroops/internal/alerting/database"
	"github.com/qiniu/zeroops/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type Deps struct {
	DB       *adb.Database
	Redis    *redis.Client
	AlertCh  chan<- AlertMessage
	Batch    int
	Interval time.Duration
}

// PrometheusDeps holds dependencies for Prometheus anomaly detection task
type PrometheusDeps struct {
	DB                  *adb.Database
	AnomalyDetectClient *AnomalyDetectClient
	Interval            time.Duration
	QueryStep           time.Duration
	QueryRange          time.Duration
	// ruleset API integration
	RulesetBase    string
	RulesetTimeout time.Duration
}

// NewRedisClientFromEnv constructs a redis client from env.
func NewRedisClientFromEnv() *redis.Client { return nil }

// NewRedisClientFromConfig constructs a redis client from app config.
func NewRedisClientFromConfig(c *config.RedisConfig) *redis.Client {
	if c == nil {
		return nil
	}
	return redis.NewClient(&redis.Options{
		Addr:     c.Addr,
		Password: c.Password,
		DB:       c.DB,
	})
}

func StartScheduler(ctx context.Context, deps Deps) {
	if deps.Interval <= 0 {
		deps.Interval = 10 * time.Second
	}
	if deps.Batch <= 0 {
		deps.Batch = 200
	}
	t := time.NewTicker(deps.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := runOnce(ctx, deps.DB, deps.Redis, deps.AlertCh, deps.Batch); err != nil {
				log.Error().Err(err).Msg("healthcheck runOnce failed")
			}
		}
	}
}

// StartPrometheusScheduler starts the Prometheus anomaly detection scheduler
func StartPrometheusScheduler(ctx context.Context, deps PrometheusDeps) {
	if deps.Interval <= 0 {
		deps.Interval = 5 * time.Minute // Default 6 hours
	}
	if deps.QueryStep <= 0 {
		deps.QueryStep = 1 * time.Minute // Default 1 minute step
	}
	if deps.QueryRange <= 0 {
		deps.QueryRange = 6 * time.Hour // Default 6 hours range
	}

	t := time.NewTicker(deps.Interval)
	defer t.Stop()

	// Run once immediately on startup
	if err := runPrometheusAnomalyDetection(ctx, deps); err != nil {
		log.Error().Err(err).Msg("prometheus anomaly detection failed on startup")
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := runPrometheusAnomalyDetection(ctx, deps); err != nil {
				log.Error().Err(err).Msg("prometheus anomaly detection failed")
			}
		}
	}
}

type pendingRow struct {
	ID         string
	Level      string
	Title      string
	AlertSince time.Time
	LabelsJSON string
}

func runOnce(ctx context.Context, db *adb.Database, rdb *redis.Client, ch chan<- AlertMessage, batch int) error {
	rows, err := queryPendingFromDB(ctx, db, batch)
	if err != nil {
		return err
	}
	for _, it := range rows {
		labels := parseLabels(it.LabelsJSON)
		svc := labels["service"]
		ver := labels["service_version"]
		// 1) publish to channel (non-blocking)
		if ch != nil {
			select {
			case ch <- AlertMessage{ID: it.ID, Service: svc, Version: ver, Level: it.Level, Title: it.Title, AlertSince: it.AlertSince, Labels: labels}:
			default:
				// channel full, skip state change
				continue
			}
		}
		// 2) alert state CAS: Pending -> InProcessing
		_ = alertStateCAS(ctx, rdb, it.ID, "Pending", "InProcessing")
		// 3) service state CAS by derived level
		if svc != "" {
			target := deriveHealth(it.Level)
			_ = serviceStateCAS(ctx, rdb, svc, ver, target)
		}
	}
	return nil
}

func queryPendingFromDB(ctx context.Context, db *adb.Database, limit int) ([]pendingRow, error) {
	if db == nil {
		return []pendingRow{}, nil
	}
	const q = `SELECT id, level, title, labels, alert_since
FROM alert_issues
WHERE alert_state = 'Pending'
ORDER BY alert_since ASC
LIMIT $1`
	rows, err := db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]pendingRow, 0, limit)
	for rows.Next() {
		var it pendingRow
		if err := rows.Scan(&it.ID, &it.Level, &it.Title, &it.LabelsJSON, &it.AlertSince); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func alertStateCAS(ctx context.Context, rdb *redis.Client, id, expected, next string) error {
	if rdb == nil {
		return nil
	}
	key := "alert:issue:" + id
	script := redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if not v then return 0 end
local obj = cjson.decode(v)
if obj.alertState ~= ARGV[1] then return -1 end
obj.alertState = ARGV[2]
redis.call('SET', KEYS[1], cjson.encode(obj), 'KEEPTTL')
redis.call('SREM', KEYS[2], ARGV[3])
redis.call('SADD', KEYS[3], ARGV[3])
return 1
`)
	_, _ = script.Run(ctx, rdb, []string{key, "alert:index:alert_state:Pending", "alert:index:alert_state:InProcessing"}, expected, next, id).Result()
	return nil
}

func serviceStateCAS(ctx context.Context, rdb *redis.Client, service, version, target string) error {
	if rdb == nil {
		return nil
	}
	key := "service_state:" + service + ":" + version
	script := redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if not v then v = '{}'; end
local obj = cjson.decode(v)
obj.health_state = ARGV[2]
redis.call('SET', KEYS[1], cjson.encode(obj), 'KEEPTTL')
if ARGV[2] ~= '' then redis.call('SADD', KEYS[3], KEYS[1]) end
return 1
`)
	_, _ = script.Run(ctx, rdb, []string{key, "", "service_state:index:health:" + target}, "", target, key).Result()
	return nil
}

// parseLabels supports either flat map {"k":"v"} or array [{"key":"k","value":"v"}]
func parseLabels(s string) map[string]string {
	m := map[string]string{}
	if s == "" {
		return m
	}
	// try map form
	if json.Unmarshal([]byte(s), &m) == nil && len(m) > 0 {
		return m
	}
	// try array form
	var arr []struct{ Key, Value string }
	if json.Unmarshal([]byte(s), &arr) == nil {
		out := make(map[string]string, len(arr))
		for _, kv := range arr {
			out[kv.Key] = kv.Value
		}
		return out
	}
	return map[string]string{}
}

// runPrometheusAnomalyDetection executes the Prometheus anomaly detection task
func runPrometheusAnomalyDetection(ctx context.Context, deps PrometheusDeps) error {
	log.Info().Msg("Starting Prometheus anomaly detection task")

	// 1. Fetch alert rules and metas from database
	rules, err := QueryAlertRules(ctx, deps.DB)
	if err != nil {
		return fmt.Errorf("failed to query alert rules: %w", err)
	}

	metas, err := QueryAlertRuleMetas(ctx, deps.DB)
	if err != nil {
		return fmt.Errorf("failed to query alert rule metas: %w", err)
	}
	log.Debug().Int("meta_count", len(metas)).Msg("fetched alert rule metas")

	// 2. Build PromQL queries by combining rules and metas
	var queries []PrometheusQuery
	for _, rule := range rules {
		for _, meta := range metas {
			if meta.AlertName == rule.Name {
				promQL := BuildPromQL(&rule, &meta)
				queries = append(queries, PrometheusQuery{
					AlertName: rule.Name,
					Expr:      promQL,
					Labels:    meta.Labels,
					Threshold: meta.Threshold,
					Severity:  rule.Severity,
				})
				log.Debug().Str("alert_name", rule.Name).Str("severity", rule.Severity).Str("expr", promQL).Msg("built promql")
			}
		}
	}

	log.Info().Int("query_count", len(queries)).Msg("Built PromQL queries")

	// 3. Calculate time range for queries
	end := time.Now()
	start := end.Add(-deps.QueryRange)
	log.Debug().Time("start", start).Time("end", end).Dur("step", deps.QueryStep).Dur("range", deps.QueryRange).Msg("time range computed")

	// 4. Execute queries and collect time series data
	var allTimeSeries []PrometheusTimeSeries
	var allQueries []PrometheusQuery
	for _, query := range queries {
		log.Debug().Str("query", query.Expr).Interface("labels", query.Labels).Msg("executing promql")

		resp, err := deps.AnomalyDetectClient.QueryRange(ctx, query.Expr, start, end, deps.QueryStep)
		if err != nil {
			log.Error().Err(err).Str("query", query.Expr).Msg("Failed to execute PromQL query")
			continue
		}

		// Add time series data and corresponding query info
		for range resp.Data.Result {
			allQueries = append(allQueries, query)
		}
		allTimeSeries = append(allTimeSeries, resp.Data.Result...)

		for _, ts := range resp.Data.Result {
			log.Debug().Interface("metric", ts.Metric).Int("value", len(ts.Values)).Msg("promql result series appended")
		}
		// no file exports
	}

	log.Info().Int("time_series_count", len(allTimeSeries)).Msg("Collected time series data")
	// 5. For each series, detect anomalies and handle per-series filtered anomalies
	rulesetBase := strings.TrimSuffix(strings.TrimSpace(deps.RulesetBase), "/")
	rulesetTimeout := deps.RulesetTimeout
	if rulesetTimeout <= 0 {
		rulesetTimeout = 10 * time.Second
	}
	httpClient := &http.Client{Timeout: rulesetTimeout}

	// Query existing alert issues once for filtering
	alertIssues, err := QueryAlertIssuesForTimeFilter(ctx, deps.DB)
	if err != nil {
		return fmt.Errorf("failed to query alert issues for filtering: %w", err)
	}

	totalDetected := 0
	totalFiltered := 0

	for i, ts := range allTimeSeries {
		if i >= len(allQueries) {
			break
		}
		q := allQueries[i]
		log.Debug().Any("q", q).Msg("per-series anomaly detection query")
		// per-series anomaly detection
		anomalies, derr := deps.AnomalyDetectClient.detectAnomaliesForSingleTimeSeries(ctx, ts, &q)
		if derr != nil {
			log.Error().Err(derr).Msg("per-series anomaly detection failed")
			continue
		}
		log.Debug().Any("anomalies", anomalies).Msg("per-series anomaly detection completed")
		totalDetected += len(anomalies)

		// filter by existing alert time ranges
		filtered := FilterAnomaliesByAlertTimeRanges(anomalies, alertIssues)
		log.Debug().Any("filtered", filtered).Msg("per-series anomaly detection filtered")
		totalFiltered += len(filtered)
		if len(filtered) == 0 {
			log.Debug().Msg("no anomalies filtered")
			continue
		}

		// Adjust thresholds: derive new threshold from exact service+version meta, then apply across versions for this service
		service := q.Labels["service"]
		if service == "" || q.AlertName == "" {
			log.Debug().Msg("skip threshold update due to missing service or alert_name")
			continue
		}
		log.Debug().Str("alert_name", q.AlertName).Str("service", service).Msg("adjusting thresholds")
		versionKey, version := detectVersionFromLabels(q.Labels)
		log.Debug().Str("alert_name", q.AlertName).Str("service", service).Str("version_key", versionKey).Str("version", version).Msg("detect version from labels")
		if version == "" {
			log.Debug().Str("alert_name", q.AlertName).Str("service", service).Msg("skip update: version is empty in labels")
			continue
		}

		baseTh, ok, terr := fetchExactThreshold(ctx, deps.DB, q.AlertName, service, versionKey, version)
		log.Debug().Str("alert_name", q.AlertName).Str("service", service).Str("version", version).Float64("base_threshold", baseTh).Bool("ok", ok).Err(terr).Msg("fetch exact threshold")
		if terr != nil {
			log.Error().Err(terr).Str("alert_name", q.AlertName).Str("service", service).Str("version", version).Msg("fetch exact threshold failed")
			continue
		}
		if !ok {
			log.Debug().Str("alert_name", q.AlertName).Str("service", service).Str("version", version).Msg("no exact meta threshold found; skip")
			continue
		}
		newThreshold := baseTh * 0.99

		// fetch all metas for this service (across versions)
		metas, ferr := fetchMetasForService(ctx, deps.DB, q.AlertName, service)
		log.Debug().Str("alert_name", q.AlertName).Str("service", service).Any("metas", metas).Msg("fetch metas for service")
		if ferr != nil {
			log.Error().Err(ferr).Str("alert_name", q.AlertName).Str("service", service).Msg("fetch metas failed")
			continue
		}
		if len(metas) == 0 {
			log.Debug().Str("alert_name", q.AlertName).Str("service", service).Msg("no metas matched for service")
			continue
		}

		// Build PUT body metas: include all versions; only exact version uses newThreshold
		const eps = 1e-9
		updates := make([]ruleMetaUpdate, 0, len(metas))
		changed := make([]struct {
			LabelsJSON string
			Old        float64
			New        float64
		}, 0, 1)
		targetMatched := false
		for _, m := range metas {
			isTarget := strings.Contains(m.LabelsJSON, fmt.Sprintf(`"%s":"%s"`, versionKey, version))
			log.Debug().Str("labels", m.LabelsJSON).Bool("is_target", isTarget).Msg("metas appended")
			if isTarget {
				targetMatched = true
				updates = append(updates, ruleMetaUpdate{Labels: m.LabelsJSON, Threshold: newThreshold})
				log.Debug().Str("labels", m.LabelsJSON).Float64("1.threshold", m.Threshold).Float64("new_threshold", newThreshold).Msg("metas appended")
				if math.Abs(m.Threshold-newThreshold) > eps {
					changed = append(changed, struct {
						LabelsJSON string
						Old        float64
						New        float64
					}{LabelsJSON: m.LabelsJSON, Old: m.Threshold, New: newThreshold})
				}
			} else {
				updates = append(updates, ruleMetaUpdate{Labels: m.LabelsJSON, Threshold: m.Threshold})
				log.Debug().Str("labels", m.LabelsJSON).Float64("2.threshold", m.Threshold).Float64("new_threshold", newThreshold).Msg("metas appended")
			}
			log.Debug().Str("labels", m.LabelsJSON).Float64("3.threshold", m.Threshold).Float64("new_threshold", newThreshold).Msg("metas appended")
		}
		if !targetMatched || len(changed) == 0 {
			log.Debug().Str("alert_name", q.AlertName).Str("service", service).Str("version", version).Bool("target_matched", targetMatched).Int("changed_count", len(changed)).Msg("no threshold change for exact version; skip")
			continue
		}

		if rulesetBase != "" {
			if err := putRuleMetas(ctx, httpClient, rulesetBase, q.AlertName, updates); err != nil {
				log.Error().Err(err).Str("alert_name", q.AlertName).Str("service", service).Msg("ruleset meta PUT failed")
			} else {
				log.Info().Str("alert_name", q.AlertName).Str("service", service).Int("meta_count", len(updates)).Msg("ruleset meta updated")
				// Log change records only after successful external update, only for changed metas
				for _, c := range changed {
					_ = insertMetaChangeLog(ctx, deps.DB, "Update", q.AlertName, c.LabelsJSON, c.Old, c.New)
				}
			}
		} else {
			log.Warn().Msg("RULESET_API_BASE not set; skip PUT /v1/alert-rule-metas/{rule_name}")
		}
	}

	log.Info().Int("anomaly_count", totalDetected).Int("filtered_anomaly_count", totalFiltered).Msg("Completed anomaly detection for all time series")
	log.Info().Msg("Completed Prometheus anomaly detection task")
	return nil
}

// fetchMetasForService returns all metas for a given rule and service across versions
func fetchMetasForService(ctx context.Context, db *adb.Database, alertName, service string) ([]struct {
	LabelsJSON string
	Threshold  float64
}, error) {
	out := []struct {
		LabelsJSON string
		Threshold  float64
	}{}
	if db == nil {
		return out, nil
	}
	const q = `SELECT labels::text, threshold FROM alert_rule_metas WHERE alert_name = $1 AND labels @> $2::jsonb`
	rows, err := db.QueryContext(ctx, q, alertName, fmt.Sprintf(`{"service":"%s"}`, service))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var labelsText string
		var th float64
		if err := rows.Scan(&labelsText, &th); err != nil {
			return nil, err
		}
		out = append(out, struct {
			LabelsJSON string
			Threshold  float64
		}{LabelsJSON: labelsText, Threshold: th})
	}
	return out, rows.Err()
}

// fetchExactThreshold returns (threshold, found, error) for a specific service+version meta
func fetchExactThreshold(ctx context.Context, db *adb.Database, alertName, service, versionKey, version string) (float64, bool, error) {
	if db == nil {
		return 0, false, nil
	}
	const q = `SELECT threshold FROM alert_rule_metas WHERE alert_name=$1 AND labels = $2::jsonb`
	labelsJSON := fmt.Sprintf(`{"service":"%s","%s":"%s"}`, service, versionKey, version)
	rows, err := db.QueryContext(ctx, q, alertName, labelsJSON)
	if err != nil {
		return 0, false, nil
	}
	defer rows.Close()
	if rows.Next() {
		var th float64
		if err := rows.Scan(&th); err == nil {
			return th, true, nil
		}
	}
	return 0, false, nil
}

// detectVersionFromLabels tries common keys for version and returns (key, value)
func detectVersionFromLabels(labels map[string]string) (string, string) {
	if v := labels["service_version"]; v != "" {
		log.Debug().Str("labels", fmt.Sprintf("%v", labels)).Str("service_version", v).Msg("detect version from labels")
		return "service_version", v
	}

	if v := labels["version"]; v != "" {
		log.Debug().Str("labels", fmt.Sprintf("%v", labels)).Str("version", v).Msg("detect version from labels")
		return "version", v
	}
	return "", ""
}

type ruleMetaUpdate struct {
	Labels    string  `json:"labels"`
	Threshold float64 `json:"threshold"`
}

type ruleMetaPutReq struct {
	RuleName string           `json:"rule_name"`
	Metas    []ruleMetaUpdate `json:"metas"`
}

// putRuleMetas calls PUT /v1/alert-rule-metas/{rule_name}
func putRuleMetas(ctx context.Context, client *http.Client, base, ruleName string, metas []ruleMetaUpdate) error {
	if client == nil {
		client = http.DefaultClient
	}
	endpoint := strings.TrimSuffix(base, "/") + "/v1/alert-rule-metas/" + url.PathEscape(ruleName)
	body, _ := json.Marshal(ruleMetaPutReq{RuleName: ruleName, Metas: metas})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ruleset api status %d", resp.StatusCode)
	}
	return nil
}

// insertMetaChangeLog writes a change record to alert_meta_change_logs (best-effort)
func insertMetaChangeLog(ctx context.Context, db *adb.Database, changeType, alertName, labels string, oldTh, newTh float64) error {
	if db == nil {
		return nil
	}
	const q = `INSERT INTO alert_meta_change_logs (id, change_type, change_time, alert_name, labels, old_threshold, new_threshold)
               VALUES ($1, $2, NOW(), $3, $4, $5, $6)`
	id := fmt.Sprintf("%s-%d", alertName, time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, q, id, changeType, alertName, labels, oldTh, newTh); err != nil {
		log.Warn().Err(err).Msg("insert alert_meta_change_logs failed")
		return err
	}
	return nil
}
