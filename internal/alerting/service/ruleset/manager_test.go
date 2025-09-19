package ruleset

import (
	"context"
	"testing"
	"time"
)

type memStore struct {
	rules map[string]*AlertRule
	metas map[string]*AlertRuleMeta
	logs  []*ChangeLog
}

func newMemStore() *memStore {
	return &memStore{rules: map[string]*AlertRule{}, metas: map[string]*AlertRuleMeta{}, logs: []*ChangeLog{}}
}

func (m *memStore) CreateRule(ctx context.Context, r *AlertRule) error {
	m.rules[r.Name] = r
	return nil
}
func (m *memStore) GetRule(ctx context.Context, name string) (*AlertRule, error) {
	return m.rules[name], nil
}
func (m *memStore) UpdateRule(ctx context.Context, r *AlertRule) error {
	m.rules[r.Name] = r
	return nil
}
func (m *memStore) DeleteRule(ctx context.Context, name string) error {
	delete(m.rules, name)
	return nil
}
func (m *memStore) UpsertMeta(ctx context.Context, meta *AlertRuleMeta) (bool, error) {
	m.metas[meta.AlertName+"|"+CanonicalLabelKey(meta.Labels)] = meta
	return true, nil
}
func (m *memStore) GetMetas(ctx context.Context, name string, labels LabelMap) ([]*AlertRuleMeta, error) {
	if v, ok := m.metas[name+"|"+CanonicalLabelKey(labels)]; ok {
		return []*AlertRuleMeta{v}, nil
	}
	return nil, nil
}
func (m *memStore) DeleteMeta(ctx context.Context, name string, labels LabelMap) error {
	delete(m.metas, name+"|"+CanonicalLabelKey(labels))
	return nil
}
func (m *memStore) InsertChangeLog(ctx context.Context, log *ChangeLog) error {
	m.logs = append(m.logs, log)
	return nil
}
func (m *memStore) WithTx(ctx context.Context, fn func(Store) error) error { return fn(m) }

func TestManager_UpsertRuleMetas(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	prom := NewExporterSync()
	mgr := NewManager(store, prom, map[string]string{"service_version": "version"})

	meta := &AlertRuleMeta{AlertName: "latency_p95_P0", Labels: LabelMap{"Service": "s3", "service_version": "v1"}, Threshold: 450, WatchTime: 2 * time.Minute}
	if err := mgr.UpsertRuleMetas(ctx, meta); err != nil {
		t.Fatalf("upsert meta: %v", err)
	}
	// verify normalization
	if _, ok := store.metas["latency_p95_P0|service=s3|version=v1"]; !ok {
		t.Fatalf("normalized meta not found in store: %#v", store.metas)
	}
	// verify prom sync
	if th, _, ok := prom.ForTestingGet("latency_p95_P0", LabelMap{"service": "s3", "version": "v1"}); !ok || th != 450 {
		t.Fatalf("prom sync threshold mismatch: th=%v ok=%v", th, ok)
	}
	// verify change log
	if len(store.logs) != 1 || store.logs[0].ChangeType != "Create" {
		t.Fatalf("unexpected change logs: %#v", store.logs)
	}

	// update path
	meta2 := &AlertRuleMeta{AlertName: "latency_p95_P0", Labels: LabelMap{"service": "s3", "version": "v1"}, Threshold: 500, WatchTime: 3 * time.Minute}
	if err := mgr.UpsertRuleMetas(ctx, meta2); err != nil {
		t.Fatalf("upsert meta2: %v", err)
	}
	if len(store.logs) != 2 || store.logs[1].ChangeType != "Update" {
		t.Fatalf("expected update log, got: %#v", store.logs)
	}
}
