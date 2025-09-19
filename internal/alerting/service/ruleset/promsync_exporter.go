package ruleset

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ExporterSync is an in-memory PromSync implementation that maintains threshold/watch values
// for each (rule, labels) pair. It is intended for unit tests and simple deployments where
// another component exposes these as metrics.
type ExporterSync struct {
	mu         sync.RWMutex
	thresholds map[string]float64
	watchTimes map[string]time.Duration
}

func NewExporterSync() *ExporterSync {
	return &ExporterSync{
		thresholds: make(map[string]float64),
		watchTimes: make(map[string]time.Duration),
	}
}

// keyFor builds a stable key for the given rule and labels.
func (e *ExporterSync) keyFor(rule string, labels LabelMap) string {
	return fmt.Sprintf("%s|%s", rule, CanonicalLabelKey(labels))
}

func (e *ExporterSync) AddToPrometheus(ctx context.Context, r *AlertRule) error { return nil }

func (e *ExporterSync) DeleteFromPrometheus(ctx context.Context, name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	// delete all entries for the rule
	prefix := name + "|"
	for k := range e.thresholds {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(e.thresholds, k)
			delete(e.watchTimes, k)
		}
	}
	return nil
}

func (e *ExporterSync) SyncMetaToPrometheus(ctx context.Context, m *AlertRuleMeta) error {
	if m == nil || m.AlertName == "" {
		return fmt.Errorf("invalid meta: missing alert name")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	key := e.keyFor(m.AlertName, m.Labels)
	e.thresholds[key] = m.Threshold
	e.watchTimes[key] = m.WatchTime
	return nil
}

// ForTestingGet exposes current values for assertions in unit tests.
func (e *ExporterSync) ForTestingGet(rule string, labels LabelMap) (threshold float64, watch time.Duration, ok bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	key := e.keyFor(rule, labels)
	v, ok1 := e.thresholds[key]
	w, ok2 := e.watchTimes[key]
	return v, w, ok1 && ok2
}
