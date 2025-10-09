package ruleset

import (
	"context"
	"time"
)

// AlertRule defines a logical alert rule. This corresponds to a Prometheus alert rule entry
// excluding threshold information, which is managed separately via AlertRuleMeta.
// Name is the business identifier and should align with Prometheus alert: field.
type AlertRule struct {
	Name        string        // unique rule name, typically equals Prometheus alert name
	Description string        // human readable explanation
	Expr        string        // left-hand PromQL expression (e.g. p95 latency expression)
	Op          string        // comparison operator: one of >, <, =, !=
	Severity    string        // severity code such as P0, P1, P2
	WatchTime   time.Duration // watch window; maps to Prometheus rule "for:" at rule level
}

// LabelMap represents a normalized set of label key-value pairs that identify a meta scope.
// Standardization rules are applied before persistence (see normalize.go).
type LabelMap map[string]string

// AlertRuleMeta holds threshold and watch duration for a specific rule under certain labels.
// Threshold is a numeric boundary; WatchTime maps to Prometheus rule "for:" duration.
type AlertRuleMeta struct {
	AlertName string   // foreign key to AlertRule.Name
	Labels    LabelMap // normalized labels; {} means global default
	Threshold float64  // numeric threshold
}

// ChangeLog captures before/after changes for auditing and potential rollback.
type ChangeLog struct {
	ID           string         // external id for de-duplication
	AlertName    string         // rule name
	ChangeType   string         // Create | Update | Delete | Rollback
	Labels       LabelMap       // affected labels
	OldThreshold *float64       // nil if not applicable
	NewThreshold *float64       // nil if not applicable
	OldWatch     *time.Duration // nil if not applicable
	NewWatch     *time.Duration // nil if not applicable
	ChangeTime   time.Time      // when the change happened
}

// Store abstracts persistence operations for rules and metas. Implementations should ensure
// correctness under concurrency via UPSERTs and, if necessary, advisory locks.
type Store interface {
	// Rule CRUD
	CreateRule(ctx context.Context, r *AlertRule) error
	GetRule(ctx context.Context, name string) (*AlertRule, error)
	UpdateRule(ctx context.Context, r *AlertRule) error
	DeleteRule(ctx context.Context, name string) error

	// Meta operations (UPSERT by alert_name + labels)
	UpsertMeta(ctx context.Context, m *AlertRuleMeta) (created bool, err error)
	GetMetas(ctx context.Context, name string, labels LabelMap) ([]*AlertRuleMeta, error)
	DeleteMeta(ctx context.Context, name string, labels LabelMap) error

	// Change logs
	InsertChangeLog(ctx context.Context, log *ChangeLog) error

	// Transaction helper. Implementation must call fn with a transactional Store
	// that respects atomicity for the ops executed within.
	WithTx(ctx context.Context, fn func(Store) error) error
}

// PromSync defines interactions with Prometheus or an exporter responsible for threshold materialization.
// Add/Delete manage the lifecycle of rule files; SyncMeta updates threshold sources.
type PromSync interface {
	AddToPrometheus(ctx context.Context, r *AlertRule) error
	DeleteFromPrometheus(ctx context.Context, name string) error
	SyncMetaToPrometheus(ctx context.Context, m *AlertRuleMeta) error
}

// AlertRuleMgr orchestrates validation, store operations, change logging, and Prometheus sync.
type AlertRuleMgr interface {
	LoadRule(ctx context.Context) error
	UpsertRuleMetas(ctx context.Context, m *AlertRuleMeta) error
	AddAlertRule(ctx context.Context, r *AlertRule) error
	DeleteAlertRule(ctx context.Context, name string) error

	AddToPrometheus(ctx context.Context, r *AlertRule) error
	DeleteFromPrometheus(ctx context.Context, name string) error
	SyncMetaToPrometheus(ctx context.Context, m *AlertRuleMeta) error
	RecordMetaChangeLog(ctx context.Context, oldMeta, newMeta *AlertRuleMeta) error
}
