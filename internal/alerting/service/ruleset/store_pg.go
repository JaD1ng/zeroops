package ruleset

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	abd "github.com/qiniu/zeroops/internal/alerting/database"
)

// PgStore is a PostgreSQL-backed Store implementation using the alerting database wrapper.
// Note: The current database wrapper does not expose transactions; WithTx acts as a simple wrapper.
// For production-grade atomicity, extend the database wrapper to support sql.Tx and wire it here.
type PgStore struct {
	DB *abd.Database
}

func NewPgStore(db *abd.Database) *PgStore { return &PgStore{DB: db} }

func (s *PgStore) WithTx(ctx context.Context, fn func(Store) error) error {
	// Fallback: invoke fn directly. Replace with real transactional context when available.
	return fn(s)
}

func (s *PgStore) CreateRule(ctx context.Context, r *AlertRule) error {
	const q = `
	INSERT INTO alert_rules(name, description, expr, op, severity)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (name) DO UPDATE SET
		description = EXCLUDED.description,
		expr = EXCLUDED.expr,
		op = EXCLUDED.op,
		severity = EXCLUDED.severity
	`
	_, err := s.DB.ExecContext(ctx, q, r.Name, r.Description, r.Expr, r.Op, r.Severity)
	if err != nil {
		return fmt.Errorf("create rule: %w", err)
	}
	return nil
}

func (s *PgStore) GetRule(ctx context.Context, name string) (*AlertRule, error) {
	const q = `SELECT name, description, expr, op, severity FROM alert_rules WHERE name = $1`
	rows, err := s.DB.QueryContext(ctx, q, name)
	if err != nil {
		return nil, fmt.Errorf("get rule: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var r AlertRule
		if err := rows.Scan(&r.Name, &r.Description, &r.Expr, &r.Op, &r.Severity); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		return &r, nil
	}
	return nil, fmt.Errorf("rule not found: %s", name)
}

func (s *PgStore) UpdateRule(ctx context.Context, r *AlertRule) error {
	const q = `UPDATE alert_rules SET description=$2, expr=$3, op=$4, severity=$5 WHERE name=$1`
	_, err := s.DB.ExecContext(ctx, q, r.Name, r.Description, r.Expr, r.Op, r.Severity)
	if err != nil {
		return fmt.Errorf("update rule: %w", err)
	}
	return nil
}

func (s *PgStore) DeleteRule(ctx context.Context, name string) error {
	const q = `DELETE FROM alert_rules WHERE name=$1`
	_, err := s.DB.ExecContext(ctx, q, name)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}
	return nil
}

func (s *PgStore) UpsertMeta(ctx context.Context, m *AlertRuleMeta) (bool, error) {
	labelsJSON, _ := json.Marshal(m.Labels)
	const q = `
	INSERT INTO alert_rule_metas(alert_name, labels, threshold, watch_time)
	VALUES ($1, $2::jsonb, $3, $4)
	ON CONFLICT (alert_name, labels) DO UPDATE SET
		threshold=EXCLUDED.threshold,
		watch_time=EXCLUDED.watch_time,
		updated_at=now()
	`
	_, err := s.DB.ExecContext(ctx, q, m.AlertName, string(labelsJSON), m.Threshold, m.WatchTime)
	if err != nil {
		return false, fmt.Errorf("upsert meta: %w", err)
	}
	// created flag is not easily observable here without RETURNING clause; return false.
	return false, nil
}

func (s *PgStore) GetMetas(ctx context.Context, name string, labels LabelMap) ([]*AlertRuleMeta, error) {
	labelsJSON, _ := json.Marshal(labels)
	const q = `
	SELECT alert_name, labels, threshold, watch_time
	FROM alert_rule_metas
	WHERE alert_name = $1 AND labels = $2::jsonb
	`
	rows, err := s.DB.QueryContext(ctx, q, name, string(labelsJSON))
	if err != nil {
		return nil, fmt.Errorf("get metas: %w", err)
	}
	defer rows.Close()
	var res []*AlertRuleMeta
	for rows.Next() {
		var alertName string
		var labelsRaw string
		var threshold float64
		var watch any
		if err := rows.Scan(&alertName, &labelsRaw, &threshold, &watch); err != nil {
			return nil, fmt.Errorf("scan meta: %w", err)
		}
		lm := LabelMap{}
		_ = json.Unmarshal([]byte(labelsRaw), &lm)
		meta := &AlertRuleMeta{AlertName: alertName, Labels: lm, Threshold: threshold}
		// best-effort: watch_time may come back as string or duration; we try string -> duration
		switch v := watch.(type) {
		case string:
			if d, err := timeParseDurationPG(v); err == nil {
				meta.WatchTime = d
			}
		}
		res = append(res, meta)
	}
	return res, nil
}

func (s *PgStore) DeleteMeta(ctx context.Context, name string, labels LabelMap) error {
	labelsJSON, _ := json.Marshal(labels)
	const q = `DELETE FROM alert_rule_metas WHERE alert_name=$1 AND labels=$2::jsonb`
	_, err := s.DB.ExecContext(ctx, q, name, string(labelsJSON))
	if err != nil {
		return fmt.Errorf("delete meta: %w", err)
	}
	return nil
}

func (s *PgStore) InsertChangeLog(ctx context.Context, log *ChangeLog) error {
	labelsJSON, _ := json.Marshal(log.Labels)
	const q = `
	INSERT INTO alert_meta_change_logs(id, alert_name, change_type, labels, old_threshold, new_threshold, old_watch, new_watch, change_time)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := s.DB.ExecContext(ctx, q, log.ID, log.AlertName, log.ChangeType, string(labelsJSON), log.OldThreshold, log.NewThreshold, log.OldWatch, log.NewWatch, log.ChangeTime)
	if err != nil {
		return fmt.Errorf("insert change log: %w", err)
	}
	return nil
}

// timeParseDurationPG parses a small subset of PostgreSQL interval text output into time.Duration.
// Supported examples: "01:02:03", "02:03", "3600 seconds". Best-effort only.
func timeParseDurationPG(s string) (time.Duration, error) {
	// HH:MM:SS
	var h, m int
	var sec float64
	if n, _ := fmt.Sscanf(s, "%d:%d:%f", &h, &m, &sec); n >= 2 {
		d := time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec*float64(time.Second))
		return d, nil
	}
	var seconds float64
	if n, _ := fmt.Sscanf(s, "%f seconds", &seconds); n == 1 {
		return time.Duration(seconds * float64(time.Second)), nil
	}
	return 0, fmt.Errorf("unsupported interval format: %s", s)
}
