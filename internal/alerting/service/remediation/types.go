package remediation

import (
	"context"
	"encoding/json"
	"time"
)

// HealAction represents a healing action configuration
type HealAction struct {
	ID    string          `json:"id"`
	Desc  string          `json:"desc"`
	Type  string          `json:"type"`
	Rules json.RawMessage `json:"rules"`
}

// HealActionRules represents the rules for a heal action
type HealActionRules struct {
	DeploymentStatus string `json:"deployment_status,omitempty"`
	Action           string `json:"action"`
	Target           string `json:"target,omitempty"`
	Message          string `json:"message,omitempty"`
}

// FaultDomain represents the identified fault domain
type FaultDomain string

const (
	FaultDomainServiceVersion FaultDomain = "service_version_issue"
	FaultDomainUnknown        FaultDomain = "unknown"
)

// HealActionResult represents the result of executing a heal action
type HealActionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ObservationWindow represents the observation period after healing
type ObservationWindow struct {
	Duration  time.Duration `json:"duration"`
	Service   string        `json:"service"`
	Version   string        `json:"version"`
	AlertID   string        `json:"alert_id"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	IsActive  bool          `json:"is_active"`
}

// ObservationWindowManager defines the interface for managing observation windows
type ObservationWindowManager interface {
	StartObservation(ctx context.Context, service, version, alertID string, duration time.Duration) error
	CheckObservation(ctx context.Context, service, version string) (*ObservationWindow, error)
	CompleteObservation(ctx context.Context, service, version string) error
	CancelObservation(ctx context.Context, service, version string) error
}

// HealActionDAO defines the interface for heal action database operations
type HealActionDAO interface {
	GetByType(ctx context.Context, faultType string) (*HealAction, error)
	GetByID(ctx context.Context, id string) (*HealAction, error)
	Create(ctx context.Context, action *HealAction) error
	Update(ctx context.Context, action *HealAction) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]*HealAction, error)
}

// HealActionService defines the interface for heal action business logic
type HealActionService interface {
	IdentifyFaultDomain(labels map[string]string) FaultDomain
	GetHealAction(ctx context.Context, faultDomain FaultDomain) (*HealAction, error)
	ExecuteHealAction(ctx context.Context, action *HealAction, alertID string, labels map[string]string) (*HealActionResult, error)
}
