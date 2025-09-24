package api

import (
	"fmt"
	"net/http"

	"github.com/fox-gonic/fox"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/model"
)

// setupAlertRouters 设置告警相关路由
func (api *Api) setupAlertRouters(router *fox.Engine) {
	router.POST("/v1/alert-rules/sync", api.SyncRules)
	router.PUT("/v1/alert-rules/:rule_name", api.UpdateRule)
	router.PUT("/v1/alert-rules/meta", api.UpdateRuleMeta)
}

// SyncRules 同步规则到Prometheus
// 接收从监控告警模块发来的规则列表，生成Prometheus规则文件并重载配置
func (api *Api) SyncRules(c *fox.Context) {
	var req model.SyncRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			"Invalid request body: "+err.Error(), nil)
		return
	}

	err := api.alertService.SyncRulesToPrometheus(req.Rules, req.RuleMetas)
	if err != nil {
		SendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodeInternalError,
			"Failed to sync rules to Prometheus: "+err.Error(), nil)
		return
	}

	c.JSON(http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Rules synced to Prometheus",
	})
}

// UpdateRule 更新单个规则模板
// 只更新指定的规则，系统会自动查找所有使用该规则的元信息并重新生成
func (api *Api) UpdateRule(c *fox.Context) {
	ruleName := c.Param("rule_name")
	if ruleName == "" {
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			"Rule name is required", nil)
		return
	}

	var req model.UpdateAlertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			"Invalid request body: "+err.Error(), nil)
		return
	}

	// 构建完整的规则对象
	rule := model.AlertRule{
		Name:        ruleName,
		Description: req.Description,
		Expr:        req.Expr,
		Op:          req.Op,
		Severity:    req.Severity,
	}

	err := api.alertService.UpdateRule(rule)
	if err != nil {
		SendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodeInternalError,
			"Failed to update rule: "+err.Error(), nil)
		return
	}

	// 获取受影响的元信息数量
	affectedCount := api.alertService.GetAffectedMetas(ruleName)

	c.JSON(http.StatusOK, map[string]interface{}{
		"status":         "success",
		"message":        fmt.Sprintf("Rule '%s' updated and synced to Prometheus", ruleName),
		"affected_metas": affectedCount,
	})
}

// UpdateRuleMeta 更新单个规则元信息
// 通过 alert_name + labels 唯一确定一个元信息记录
func (api *Api) UpdateRuleMeta(c *fox.Context) {
	var req model.UpdateAlertRuleMetaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			"Invalid request body: "+err.Error(), nil)
		return
	}

	// alert_name 和 labels 是必填的
	if req.AlertName == "" || req.Labels == "" {
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			"alert_name and labels are required", nil)
		return
	}

	// 构建完整的元信息对象
	meta := model.AlertRuleMeta{
		AlertName: req.AlertName,
		Labels:    req.Labels,
		Threshold: req.Threshold,
		WatchTime: req.WatchTime,
	}

	err := api.alertService.UpdateRuleMeta(meta)
	if err != nil {
		SendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodeInternalError,
			"Failed to update rule meta: "+err.Error(), nil)
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"status":     "success",
		"message":    "Rule meta updated and synced to Prometheus",
		"alert_name": req.AlertName,
		"labels":     req.Labels,
	})
}
