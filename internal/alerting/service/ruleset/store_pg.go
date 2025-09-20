package ruleset

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	abd "github.com/qiniu/zeroops/internal/alerting/database"
)

// PgStore is a PostgreSQL-backed Store implementation using the alerting database wrapper.
// Note: The current database wrapper does not expose transactions; WithTx acts as a simple wrapper.
// For production-grade atomicity, extend the database wrapper to support sql.Tx and wire it here.
// This implementation uses pgx native types to avoid manual parsing of PostgreSQL interval types.
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
	labelsJSON, err := json.Marshal(m.Labels)
	if err != nil {
		return false, fmt.Errorf("marshal labels: %w", err)
	}

	// Convert time.Duration to pgtype.Interval
	interval := durationToPgInterval(m.WatchTime)

	const q = `
	INSERT INTO alert_rule_metas(alert_name, labels, threshold, watch_time)
	VALUES ($1, $2::jsonb, $3, $4)
	ON CONFLICT (alert_name, labels) DO UPDATE SET
		threshold=EXCLUDED.threshold,
		watch_time=EXCLUDED.watch_time
	`
	_, err = s.DB.ExecContext(ctx, q, m.AlertName, string(labelsJSON), m.Threshold, interval)
	if err != nil {
		return false, fmt.Errorf("upsert meta: %w", err)
	}
	// created flag is not easily observable here without RETURNING clause; return false.
	return false, nil
}

func (s *PgStore) GetMetas(ctx context.Context, name string, labels LabelMap) ([]*AlertRuleMeta, error) {
	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return nil, fmt.Errorf("marshal labels for get: %w", err)
	}
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
		var watch pgtype.Interval
		if err := rows.Scan(&alertName, &labelsRaw, &threshold, &watch); err != nil {
			return nil, fmt.Errorf("scan meta: %w", err)
		}
		lm := LabelMap{}
		if err := json.Unmarshal([]byte(labelsRaw), &lm); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
		meta := &AlertRuleMeta{AlertName: alertName, Labels: lm, Threshold: threshold}

		// Convert pgtype.Interval to time.Duration
		if duration, err := pgIntervalToDuration(watch); err == nil {
			meta.WatchTime = duration
		}
		res = append(res, meta)
	}
	return res, nil
}

func (s *PgStore) DeleteMeta(ctx context.Context, name string, labels LabelMap) error {
	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}
	const q = `DELETE FROM alert_rule_metas WHERE alert_name=$1 AND labels=$2::jsonb`
	_, err = s.DB.ExecContext(ctx, q, name, string(labelsJSON))
	if err != nil {
		return fmt.Errorf("delete meta: %w", err)
	}
	return nil
}

func (s *PgStore) InsertChangeLog(ctx context.Context, log *ChangeLog) error {
	labelsJSON, err := json.Marshal(log.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels for changelog: %w", err)
	}

	// Convert time.Duration to pgtype.Interval for old and new watch times
	var oldWatch, newWatch *pgtype.Interval
	if log.OldWatch != nil {
		interval := durationToPgInterval(*log.OldWatch)
		oldWatch = &interval
	}
	if log.NewWatch != nil {
		interval := durationToPgInterval(*log.NewWatch)
		newWatch = &interval
	}

	const q = `
	INSERT INTO alert_meta_change_logs(id, alert_name, change_type, labels, old_threshold, new_threshold, old_watch, new_watch, change_time)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = s.DB.ExecContext(ctx, q, log.ID, log.AlertName, log.ChangeType, string(labelsJSON), log.OldThreshold, log.NewThreshold, oldWatch, newWatch, log.ChangeTime)
	if err != nil {
		return fmt.Errorf("insert change log: %w", err)
	}
	return nil
}

// durationToPgInterval converts a time.Duration to pgtype.Interval.
// Note: This conversion assumes the duration represents a fixed time period.
// For durations that include months or years, this conversion may not be accurate.
func durationToPgInterval(d time.Duration) pgtype.Interval {
	// Convert to total microseconds first
	totalMicroseconds := d.Microseconds()

	// Calculate days and remaining microseconds
	days := totalMicroseconds / (24 * 60 * 60 * 1000000) // 24 hours * 60 minutes * 60 seconds * 1,000,000 microseconds
	remainingMicroseconds := totalMicroseconds % (24 * 60 * 60 * 1000000)

	return pgtype.Interval{
		Microseconds: remainingMicroseconds,
		Days:         int32(days),
		Months:       0, // Duration doesn't include months
		Valid:        true,
	}
}

// pgIntervalToDuration converts a pgtype.Interval to time.Duration.
// This function returns an error if the interval contains months or years,
// as these cannot be accurately converted to a fixed duration.
func pgIntervalToDuration(interval pgtype.Interval) (time.Duration, error) {
	if !interval.Valid {
		return 0, fmt.Errorf("interval is not valid")
	}

	// Check if the interval contains months or years
	if interval.Months != 0 {
		return 0, fmt.Errorf("cannot convert interval with months to duration: %d months", interval.Months)
	}

	// Convert to duration
	totalMicroseconds := interval.Microseconds + int64(interval.Days)*24*60*60*1000000
	return time.Duration(totalMicroseconds) * time.Microsecond, nil
}
