package ruleset

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrInvalidMeta indicates provided meta is incomplete or invalid.
	ErrInvalidMeta = errors.New("invalid alert rule meta")
)

// Manager implements AlertRuleMgr, coordinating store and Prometheus sync.
type Manager struct {
	store    Store
	prom     PromSync
	aliasMap map[string]string
}

func NewManager(store Store, prom PromSync, aliasMap map[string]string) *Manager {
	if aliasMap == nil {
		aliasMap = map[string]string{}
	}
	return &Manager{store: store, prom: prom, aliasMap: aliasMap}
}

func (m *Manager) LoadRule(ctx context.Context) error { return nil }

func (m *Manager) AddAlertRule(ctx context.Context, r *AlertRule) error {
	if r == nil || r.Name == "" {
		return fmt.Errorf("invalid rule")
	}
	// First ensure the rule is added to Prometheus successfully
	// This guarantees Prometheus has the correct data even if DB write fails
	if err := m.prom.AddToPrometheus(ctx, r); err != nil {
		return fmt.Errorf("failed to add rule to Prometheus: %w", err)
	}
	// Then persist to database
	// If this fails, the rule will still be in Prometheus, which is better than
	// having it in DB but not in Prometheus (which would cause missing alerts)
	if err := m.store.CreateRule(ctx, r); err != nil {
		return fmt.Errorf("failed to create rule in database: %w", err)
	}
	return nil
}

func (m *Manager) DeleteAlertRule(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("invalid name")
	}
	// First remove from Prometheus to stop alerting immediately
	// This prevents false alerts if DB deletion fails
	if err := m.prom.DeleteFromPrometheus(ctx, name); err != nil {
		return fmt.Errorf("failed to delete rule from Prometheus: %w", err)
	}
	// Then remove from database
	// If this fails, the rule is already removed from Prometheus (no false alerts)
	if err := m.store.DeleteRule(ctx, name); err != nil {
		return fmt.Errorf("failed to delete rule from database: %w", err)
	}
	return nil
}

func (m *Manager) AddToPrometheus(ctx context.Context, r *AlertRule) error {
	return m.prom.AddToPrometheus(ctx, r)
}
func (m *Manager) DeleteFromPrometheus(ctx context.Context, name string) error {
	return m.prom.DeleteFromPrometheus(ctx, name)
}
func (m *Manager) SyncMetaToPrometheus(ctx context.Context, meta *AlertRuleMeta) error {
	return m.prom.SyncMetaToPrometheus(ctx, meta)
}

func (m *Manager) UpsertRuleMetas(ctx context.Context, meta *AlertRuleMeta) error {
	if meta == nil {
		return ErrInvalidMeta
	}
	meta.Labels = NormalizeLabels(meta.Labels, m.aliasMap)
	if err := validateMeta(meta); err != nil {
		return err
	}

	// First, get the old meta for change logging
	oldList, err := m.store.GetMetas(ctx, meta.AlertName, meta.Labels)
	if err != nil {
		return err
	}
	var old *AlertRuleMeta
	if len(oldList) > 0 {
		old = oldList[0]
	}

	// Prepare change log parameters outside of transaction to minimize lock time
	var changeLog *ChangeLog
	if old != nil || meta != nil {
		changeLog = m.prepareChangeLog(old, meta)
	}

	// First ensure the meta is synced to Prometheus successfully
	// This guarantees Prometheus has the correct threshold data even if DB write fails
	if err := m.prom.SyncMetaToPrometheus(ctx, meta); err != nil {
		return fmt.Errorf("failed to sync meta to Prometheus: %w", err)
	}

	// Then persist to database within a transaction
	// If this fails, the meta will still be in Prometheus, which is better than
	// having it in DB but not in Prometheus (which would cause incorrect thresholds)
	return m.store.WithTx(ctx, func(tx Store) error {
		_, err = tx.UpsertMeta(ctx, meta)
		if err != nil {
			return err
		}
		// Insert pre-prepared change log
		if changeLog != nil {
			if err := tx.InsertChangeLog(ctx, changeLog); err != nil {
				return err
			}
		}
		return nil
	})
}

// prepareChangeLog prepares change log parameters outside of transaction to minimize lock time
func (m *Manager) prepareChangeLog(oldMeta, newMeta *AlertRuleMeta) *ChangeLog {
	if newMeta == nil {
		return nil
	}

	// Prepare all parameters outside of transaction
	var oldTh, newTh *float64
	var oldW, newW *time.Duration
	if oldMeta != nil {
		oldTh = &oldMeta.Threshold
		oldW = &oldMeta.WatchTime
	}
	if newMeta != nil {
		newTh = &newMeta.Threshold
		newW = &newMeta.WatchTime
	}

	// Generate ID and timestamp outside of transaction
	now := time.Now()
	changeTime := now.UTC()
	id := fmt.Sprintf("%s-%s-%d", newMeta.AlertName, CanonicalLabelKey(newMeta.Labels), now.UnixNano())

	return &ChangeLog{
		ID:           id,
		AlertName:    newMeta.AlertName,
		ChangeType:   classifyChange(oldMeta, newMeta),
		Labels:       newMeta.Labels,
		OldThreshold: oldTh,
		NewThreshold: newTh,
		OldWatch:     oldW,
		NewWatch:     newW,
		ChangeTime:   changeTime,
	}
}

func (m *Manager) RecordMetaChangeLog(ctx context.Context, oldMeta, newMeta *AlertRuleMeta) error {
	changeLog := m.prepareChangeLog(oldMeta, newMeta)
	if changeLog == nil {
		return nil
	}
	return m.store.InsertChangeLog(ctx, changeLog)
}

func classifyChange(oldMeta, newMeta *AlertRuleMeta) string {
	if oldMeta == nil && newMeta != nil {
		return "Create"
	}
	if oldMeta != nil && newMeta == nil {
		return "Delete"
	}
	return "Update"
}

func validateMeta(m *AlertRuleMeta) error {
	if m.AlertName == "" {
		return ErrInvalidMeta
	}
	if !isFinite(m.Threshold) {
		return ErrInvalidMeta
	}
	if m.WatchTime < 0 {
		return ErrInvalidMeta
	}
	return nil
}

func isFinite(f float64) bool { return !((f != f) || (f > 1e308) || (f < -1e308)) }
