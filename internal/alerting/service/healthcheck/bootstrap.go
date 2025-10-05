package healthcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	adb "github.com/qiniu/zeroops/internal/alerting/database"
	"github.com/qiniu/zeroops/internal/config"
	"github.com/rs/zerolog/log"
)

// RuleConfigFile represents the structure of the rules config file
type RuleConfigFile struct {
	Rules []RuleConfigItem `json:"rules"`
}

type RuleConfigItem struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Expr        string           `json:"expr"`
	Op          string           `json:"op"`
	Severity    string           `json:"severity"`
	WatchTime   string           `json:"watch_time"`
	Metas       []RuleConfigMeta `json:"metas"`
}

type RuleConfigMeta struct {
	Labels    map[string]string `json:"labels"`
	Threshold float64           `json:"threshold"`
}

// BootstrapRulesFromConfig loads a rules config JSON file, compares with DB, and for missing rules:
// 1) PUT /v1/alert-rules/{rule_name}
// 2) PUT /v1/alert-rules-meta/{rule_name}
// 3) Insert into local DB: alert_rules and alert_rule_metas (on conflict do nothing)
func BootstrapRulesFromConfig(ctx context.Context, db *adb.Database) error { return nil }

// BootstrapRulesFromConfigWithApp allows injecting app config to avoid env usage.
func BootstrapRulesFromConfigWithApp(ctx context.Context, db *adb.Database, c *config.RulesetConfig) error {
	if c == nil || strings.TrimSpace(c.ConfigFile) == "" {
		return nil
	}
	base := strings.TrimSuffix(strings.TrimSpace(c.APIBase), "/")
	data, err := os.ReadFile(c.ConfigFile)
	if err != nil {
		return fmt.Errorf("read rules config: %w", err)
	}
	var cfg RuleConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse rules config: %w", err)
	}
	if len(cfg.Rules) == 0 {
		return nil
	}
	existing := map[string]struct{}{}
	if rules, err := QueryAlertRules(ctx, db); err == nil {
		for _, r := range rules {
			existing[r.Name] = struct{}{}
		}
	}
	tout := 10 * time.Second
	if d, err := time.ParseDuration(strings.TrimSpace(c.APITimeout)); err == nil {
		tout = d
	}
	client := &http.Client{Timeout: tout}
	for _, r := range cfg.Rules {
		if _, ok := existing[r.Name]; ok {
			continue
		}
		if base != "" {
			if err := putRule(ctx, client, base, &r); err != nil {
				log.Error().Err(err).Str("rule", r.Name).Msg("external PUT alert rule failed")
				continue
			}
			metaUpdates := make([]ruleMetaUpdate, 0, len(r.Metas))
			for _, m := range r.Metas {
				labelsJSON, _ := json.Marshal(m.Labels)
				metaUpdates = append(metaUpdates, ruleMetaUpdate{Labels: string(labelsJSON), Threshold: m.Threshold})
			}
			if len(metaUpdates) > 0 {
				if err := putRuleMetasBootstrap(ctx, client, base, r.Name, metaUpdates); err != nil {
					log.Error().Err(err).Str("rule", r.Name).Msg("external PUT rule metas failed")
					continue
				}
			}
		}
		_ = insertAlertRule(ctx, db, &r)
		for _, m := range r.Metas {
			_ = insertAlertRuleMeta(ctx, db, r.Name, m)
		}
	}
	return nil
}

func putRule(ctx context.Context, client *http.Client, base string, r *RuleConfigItem) error {
	endpoint := base + "/v1/alert-rules/" + url.PathEscape(r.Name)

	// Convert watch_time from string (e.g., "5 minutes") to seconds
	watchTimeSeconds, err := parseWatchTimeToSeconds(r.WatchTime)
	if err != nil {
		return fmt.Errorf("parse watch_time %s: %w", r.WatchTime, err)
	}

	body := map[string]interface{}{
		"description": r.Description,
		"expr":        r.Expr,
		"op":          r.Op,
		"severity":    r.Severity,
		"watch_time":  watchTimeSeconds,
	}
	bs, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, io.NopCloser(strings.NewReader(string(bs))))
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
		return fmt.Errorf("ruleset PUT rule status %d", resp.StatusCode)
	}
	return nil
}

// parseWatchTimeToSeconds converts watch_time string to seconds
// e.g., "5 minutes" -> 300, "4 minutes" -> 240
func parseWatchTimeToSeconds(watchTime string) (int, error) {
	if watchTime == "" {
		return 0, fmt.Errorf("empty watch_time")
	}

	duration, err := time.ParseDuration(watchTime)
	if err != nil {
		return 0, err
	}

	return int(duration.Seconds()), nil
}

// putRuleMetasBootstrap calls PUT /v1/alert-rules-meta/{rule_name} with the correct format
func putRuleMetasBootstrap(ctx context.Context, client *http.Client, base, ruleName string, metas []ruleMetaUpdate) error {
	endpoint := base + "/v1/alert-rules-meta/" + url.PathEscape(ruleName)
	body := map[string]interface{}{
		"metas": metas,
	}
	bs, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, io.NopCloser(strings.NewReader(string(bs))))
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
		return fmt.Errorf("ruleset PUT rule metas status %d", resp.StatusCode)
	}
	return nil
}

func insertAlertRule(ctx context.Context, db *adb.Database, r *RuleConfigItem) error {
	if db == nil {
		return nil
	}
	const q = `INSERT INTO alert_rules (name, description, expr, op, severity, watch_time)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (name) DO NOTHING`
	_, err := db.ExecContext(ctx, q, r.Name, r.Description, r.Expr, r.Op, r.Severity, r.WatchTime)
	return err
}

func insertAlertRuleMeta(ctx context.Context, db *adb.Database, ruleName string, m RuleConfigMeta) error {
	if db == nil {
		return nil
	}
	labelsJSON, _ := json.Marshal(m.Labels)
	const q = `INSERT INTO alert_rule_metas (alert_name, labels, threshold)
	VALUES ($1, $2::jsonb, $3)
	ON CONFLICT (alert_name, labels) DO NOTHING`
	_, err := db.ExecContext(ctx, q, ruleName, string(labelsJSON), m.Threshold)
	return err
}
