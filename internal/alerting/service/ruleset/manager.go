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
	if err := m.store.CreateRule(ctx, r); err != nil {
		return err
	}
	return m.prom.AddToPrometheus(ctx, r)
}

func (m *Manager) DeleteAlertRule(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("invalid name")
	}
	if err := m.store.DeleteRule(ctx, name); err != nil {
		return err
	}
	return m.prom.DeleteFromPrometheus(ctx, name)
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
	return m.store.WithTx(ctx, func(tx Store) error {
		oldList, err := tx.GetMetas(ctx, meta.AlertName, meta.Labels)
		if err != nil {
			return err
		}
		var old *AlertRuleMeta
		if len(oldList) > 0 {
			old = oldList[0]
		}
		_, err = tx.UpsertMeta(ctx, meta)
		if err != nil {
			return err
		}
		if err := m.RecordMetaChangeLog(ctx, old, meta); err != nil {
			return err
		}
		if err := m.prom.SyncMetaToPrometheus(ctx, meta); err != nil {
			return err
		}
		return nil
	})
}

func (m *Manager) RecordMetaChangeLog(ctx context.Context, oldMeta, newMeta *AlertRuleMeta) error {
	if newMeta == nil {
		return nil
	}
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
	log := &ChangeLog{
		ID:           fmt.Sprintf("%s-%s-%d", newMeta.AlertName, CanonicalLabelKey(newMeta.Labels), time.Now().UnixNano()),
		AlertName:    newMeta.AlertName,
		ChangeType:   classifyChange(oldMeta, newMeta),
		Labels:       newMeta.Labels,
		OldThreshold: oldTh,
		NewThreshold: newTh,
		OldWatch:     oldW,
		NewWatch:     newW,
		ChangeTime:   time.Now().UTC(),
	}
	return m.store.InsertChangeLog(ctx, log)
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
