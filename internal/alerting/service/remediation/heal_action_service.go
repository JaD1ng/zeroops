package remediation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

// HealActionServiceImpl implements HealActionService
type HealActionServiceImpl struct {
	dao HealActionDAO
}

// NewHealActionService creates a new heal action service
func NewHealActionService(dao HealActionDAO) *HealActionServiceImpl {
	return &HealActionServiceImpl{dao: dao}
}

// IdentifyFaultDomain identifies the fault domain from alert labels
func (s *HealActionServiceImpl) IdentifyFaultDomain(labels map[string]string) FaultDomain {
	service := labels["service_name"]
	version := labels["version"]

	if service != "" && version != "" {
		return FaultDomainServiceVersion
	}

	// TODO: 可根据更多条件扩展其他故障域
	// - 整体问题：检查是否有全局性指标异常
	// - 单机房问题：检查是否有机房相关标签
	// - 网络问题：检查是否有网络相关标签
	return FaultDomainUnknown
}

// GetHealAction retrieves the appropriate heal action for a fault domain
func (s *HealActionServiceImpl) GetHealAction(ctx context.Context, faultDomain FaultDomain) (*HealAction, error) {
	if faultDomain == FaultDomainUnknown {
		return nil, fmt.Errorf("unknown fault domain, cannot determine heal action")
	}

	action, err := s.dao.GetByType(ctx, string(faultDomain))
	if err != nil {
		return nil, fmt.Errorf("failed to get heal action for domain %s: %w", faultDomain, err)
	}

	return action, nil
}

// ExecuteHealAction executes the heal action based on the rules
func (s *HealActionServiceImpl) ExecuteHealAction(ctx context.Context, action *HealAction, alertID string, labels map[string]string) (*HealActionResult, error) {
	if action == nil {
		return &HealActionResult{
			Success: false,
			Error:   "no heal action provided",
		}, nil
	}

	// Parse the rules
	var rules HealActionRules
	if err := json.Unmarshal(action.Rules, &rules); err != nil {
		return &HealActionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse heal action rules: %v", err),
		}, nil
	}

	// Execute based on action type
	switch rules.Action {
	case "rollback":
		return s.executeRollback(ctx, rules, alertID, labels)
	case "alert":
		return s.executeAlert(rules, alertID, labels)
	default:
		return &HealActionResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported action type: %s", rules.Action),
		}, nil
	}
}

// executeRollback executes a rollback operation
func (s *HealActionServiceImpl) executeRollback(ctx context.Context, rules HealActionRules, alertID string, labels map[string]string) (*HealActionResult, error) {
	_ = ctx // TODO: Use context for HTTP timeout when calling real rollback API
	// Check deployment status if specified
	if rules.DeploymentStatus != "" {
		// TODO: 实际实现中应该查询部署系统获取真实的部署状态
		// 这里暂时模拟检查
		deployStatus := s.getDeploymentStatus(labels)
		if deployStatus != rules.DeploymentStatus {
			return &HealActionResult{
				Success: false,
				Message: fmt.Sprintf("deployment status mismatch: expected %s, got %s", rules.DeploymentStatus, deployStatus),
			}, nil
		}
	}

	// Mock rollback execution
	sleepDur := parseDuration(os.Getenv("REMEDIATION_ROLLBACK_SLEEP"), 30*time.Second)
	log.Info().
		Str("alert_id", alertID).
		Str("target", rules.Target).
		Dur("sleep_duration", sleepDur).
		Msg("executing mock rollback")

	// Simulate rollback time
	time.Sleep(sleepDur)

	// TODO: 实际实现中应该调用真实的回滚接口
	// url := fmt.Sprintf(os.Getenv("REMEDIATION_ROLLBACK_URL"), deriveDeployID(labels))
	// 发起 HTTP POST 请求到回滚接口

	return &HealActionResult{
		Success: true,
		Message: fmt.Sprintf("rollback completed successfully, target: %s", rules.Target),
	}, nil
}

// executeAlert executes an alert-only action (no automatic healing)
func (s *HealActionServiceImpl) executeAlert(rules HealActionRules, alertID string, labels map[string]string) (*HealActionResult, error) {
	_ = labels // TODO: Use labels for context-specific alert messages
	log.Warn().
		Str("alert_id", alertID).
		Str("message", rules.Message).
		Msg("heal action requires manual intervention")

	return &HealActionResult{
		Success: false,
		Message: rules.Message,
	}, nil
}

// getDeploymentStatus gets the deployment status for the given labels
// TODO: 实际实现中应该查询部署系统获取真实的部署状态
func (s *HealActionServiceImpl) getDeploymentStatus(labels map[string]string) string {
	// 这里暂时返回模拟状态
	// 实际实现中应该：
	// 1. 从 labels 中提取 service 和 version
	// 2. 查询部署系统 API 获取当前部署状态
	// 3. 返回 "deploying" 或 "deployed"

	service := labels["service_name"]
	version := labels["version"]

	if service == "" || version == "" {
		return "unknown"
	}

	// 模拟逻辑：如果版本号包含 "dev" 或 "test"，认为是发布中，待确认修改为实际的部署状态区分方式
	if version == "dev" || version == "test" {
		return "deploying"
	}

	return "deployed"
}

// deriveDeployIDFromLabels derives deployment ID from labels
// TODO: Use this function when implementing real rollback API calls
func deriveDeployIDFromLabels(labels map[string]string) string {
	if v := labels["deploy_id"]; v != "" {
		return v
	}
	service := labels["service_name"]
	version := labels["version"]
	if service != "" && version != "" {
		return fmt.Sprintf("%s:%s", service, version)
	}
	return ""
}
