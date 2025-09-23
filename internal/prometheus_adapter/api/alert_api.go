package api

import (
	"net/http"

	"github.com/fox-gonic/fox"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/model"
)

// setupAlertRouters 设置告警相关路由
func (api *Api) setupAlertRouters(router *fox.Engine) {
	router.POST("/v1/alert-rules/sync", api.SyncRules)
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
