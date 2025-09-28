package api

import (
	"fmt"
	"net/http"

	"github.com/fox-gonic/fox"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/model"
)

// setupAlertRouters 设置告警相关路由
func (api *Api) setupAlertRouters(router *fox.Engine) {
	router.PUT("/v1/alert-rules/:rule_name", api.UpdateRule)
	router.PUT("/v1/alert-rules-meta/:rule_name", api.UpdateRuleMetas)
	router.DELETE("/v1/alert-rules/:rule_name", api.DeleteRule)
	router.DELETE("/v1/alert-rules-meta/:rule_name", api.DeleteRuleMeta)
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
		WatchTime:   req.WatchTime,
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

// UpdateRuleMetas 批量更新规则元信息
// 通过 rule_name + labels 唯一确定一个元信息记录
func (api *Api) UpdateRuleMetas(c *fox.Context) {
	ruleName := c.Param("rule_name")
	if ruleName == "" {
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			"Rule name is required", nil)
		return
	}

	var req model.UpdateAlertRuleMetaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			"Invalid request body: "+err.Error(), nil)
		return
	}

	if len(req.Metas) == 0 {
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			"At least one meta update is required", nil)
		return
	}

	// 批量更新元信息
	updatedCount := 0
	for _, metaUpdate := range req.Metas {
		// 构建完整的元信息对象
		meta := model.AlertRuleMeta{
			AlertName: ruleName,
			Labels:    metaUpdate.Labels,
			Threshold: metaUpdate.Threshold,
		}

		err := api.alertService.UpdateRuleMeta(meta)
		if err != nil {
			SendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodeInternalError,
				fmt.Sprintf("Failed to update rule meta: %v", err), nil)
			return
		}
		updatedCount++
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"status":        "success",
		"message":       "Rule metas updated and synced to Prometheus",
		"rule_name":     ruleName,
		"updated_count": updatedCount,
	})
}

// DeleteRule 删除单个规则模板及其所有关联的元信息
func (api *Api) DeleteRule(c *fox.Context) {
	ruleName := c.Param("rule_name")
	if ruleName == "" {
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			"Rule name is required", nil)
		return
	}

	// 获取受影响的元信息数量
	affectedCount := api.alertService.GetAffectedMetas(ruleName)

	err := api.alertService.DeleteRule(ruleName)
	if err != nil {
		if err.Error() == fmt.Sprintf("rule '%s' not found", ruleName) {
			SendErrorResponse(c, http.StatusNotFound, model.ErrorCodeInvalidParameter,
				err.Error(), nil)
		} else {
			SendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodeInternalError,
				"Failed to delete rule: "+err.Error(), nil)
		}
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"status":        "success",
		"message":       fmt.Sprintf("Rule '%s' and %d associated metas deleted successfully", ruleName, affectedCount),
		"rule_name":     ruleName,
		"deleted_metas": affectedCount,
	})
}

// DeleteRuleMeta 删除单个规则元信息
func (api *Api) DeleteRuleMeta(c *fox.Context) {
	ruleName := c.Param("rule_name")
	if ruleName == "" {
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			"Rule name is required", nil)
		return
	}

	var req model.DeleteAlertRuleMetaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendErrorResponse(c, http.StatusBadRequest, model.ErrorCodeInvalidParameter,
			"Invalid request body: "+err.Error(), nil)
		return
	}

	err := api.alertService.DeleteRuleMeta(ruleName, req.Labels)
	if err != nil {
		if err.Error() == fmt.Sprintf("rule meta not found for rule '%s' with labels '%s'", ruleName, req.Labels) {
			SendErrorResponse(c, http.StatusNotFound, model.ErrorCodeInvalidParameter,
				err.Error(), nil)
		} else {
			SendErrorResponse(c, http.StatusInternalServerError, model.ErrorCodeInternalError,
				"Failed to delete rule meta: "+err.Error(), nil)
		}
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "success",
		"message":   "Rule meta deleted successfully",
		"rule_name": ruleName,
		"labels":    req.Labels,
	})
}
