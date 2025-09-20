package remediation

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHealActionServiceImpl_IdentifyFaultDomain(t *testing.T) {
	service := &HealActionServiceImpl{}

	tests := []struct {
		name     string
		labels   map[string]string
		expected FaultDomain
	}{
		{
			name: "service_version_issue",
			labels: map[string]string{
				"service_name": "test-service",
				"version":      "v1.0.0",
			},
			expected: FaultDomainServiceVersion,
		},
		{
			name: "missing_service_name",
			labels: map[string]string{
				"version": "v1.0.0",
			},
			expected: FaultDomainUnknown,
		},
		{
			name: "missing_version",
			labels: map[string]string{
				"service_name": "test-service",
			},
			expected: FaultDomainUnknown,
		},
		{
			name:     "empty_labels",
			labels:   map[string]string{},
			expected: FaultDomainUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.IdentifyFaultDomain(tt.labels)
			if result != tt.expected {
				t.Errorf("IdentifyFaultDomain() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHealActionServiceImpl_ExecuteHealAction(t *testing.T) {
	service := &HealActionServiceImpl{}

	tests := []struct {
		name        string
		action      *HealAction
		alertID     string
		labels      map[string]string
		expectError bool
	}{
		{
			name: "rollback_action",
			action: &HealAction{
				ID:   "test-rollback",
				Desc: "Test rollback action",
				Type: "service_version_issue",
				Rules: json.RawMessage(`{
					"deployment_status": "deploying",
					"action": "rollback",
					"target": "previous_version"
				}`),
			},
			alertID: "test-alert-1",
			labels: map[string]string{
				"service_name": "test-service",
				"version":      "dev",
			},
			expectError: false,
		},
		{
			name: "alert_action",
			action: &HealAction{
				ID:   "test-alert",
				Desc: "Test alert action",
				Type: "service_version_issue",
				Rules: json.RawMessage(`{
					"action": "alert",
					"message": "Version already deployed, manual intervention required"
				}`),
			},
			alertID: "test-alert-2",
			labels: map[string]string{
				"service_name": "test-service",
				"version":      "v1.0.0",
			},
			expectError: false,
		},
		{
			name:        "nil_action",
			action:      nil,
			alertID:     "test-alert-3",
			labels:      map[string]string{},
			expectError: false, // Should not error, but return failure result
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ExecuteHealAction(context.Background(), tt.action, tt.alertID, tt.labels)

			if tt.expectError && err == nil {
				t.Errorf("ExecuteHealAction() expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("ExecuteHealAction() unexpected error: %v", err)
			}

			if result == nil {
				t.Errorf("ExecuteHealAction() returned nil result")
			}
		})
	}
}

func TestHealActionServiceImpl_getDeploymentStatus(t *testing.T) {
	service := &HealActionServiceImpl{}

	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name: "deploying_version",
			labels: map[string]string{
				"service_name": "test-service",
				"version":      "dev",
			},
			expected: "deploying",
		},
		{
			name: "deployed_version",
			labels: map[string]string{
				"service_name": "test-service",
				"version":      "v1.0.0",
			},
			expected: "deployed",
		},
		{
			name: "missing_service_name",
			labels: map[string]string{
				"version": "v1.0.0",
			},
			expected: "unknown",
		},
		{
			name: "missing_version",
			labels: map[string]string{
				"service_name": "test-service",
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.getDeploymentStatus(tt.labels)
			if result != tt.expected {
				t.Errorf("getDeploymentStatus() = %v, want %v", result, tt.expected)
			}
		})
	}
}
