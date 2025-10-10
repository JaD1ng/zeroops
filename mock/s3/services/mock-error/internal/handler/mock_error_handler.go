package handler

import (
	"mocks3/services/mock-error/internal/service"
	"mocks3/shared/models"
	"mocks3/shared/observability"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// MockErrorHandler Mock错误注入HTTP处理器
type MockErrorHandler struct {
	errorService *service.MockErrorService
	logger       *observability.Logger
}

// NewMockErrorHandler 创建Mock错误注入HTTP处理器
func NewMockErrorHandler(errorService *service.MockErrorService, logger *observability.Logger) *MockErrorHandler {
	return &MockErrorHandler{
		errorService: errorService,
		logger:       logger,
	}
}

// SetupRoutes 设置路由
func (h *MockErrorHandler) SetupRoutes(router *gin.Engine) {
	// 监控异常注入API
	api := router.Group("/api/v1")
	{
		api.POST("/metric-anomaly", h.createMetricAnomaly)       // 创建指标异常规则
		api.DELETE("/metric-anomaly/:id", h.deleteMetricAnomaly) // 删除异常规则
		api.POST("/metric-inject/check", h.checkMetricInjection) // 检查是否注入指标异常
		api.GET("/stats", h.getStats)                            // 获取统计信息
	}

	// 健康检查
	router.GET("/health", h.healthCheck)
}

// createMetricAnomaly 创建指标异常规则
func (h *MockErrorHandler) createMetricAnomaly(c *gin.Context) {
	ctx := c.Request.Context()

	var rule models.MetricAnomalyRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		h.logger.Error(ctx, "Failed to bind metric anomaly rule", observability.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.errorService.CreateRule(ctx, &rule); err != nil {
		h.logger.Error(ctx, "Failed to create metric anomaly rule", observability.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info(ctx, "Metric anomaly rule created successfully",
		observability.String("rule_id", rule.ID),
		observability.String("anomaly_type", rule.AnomalyType),
		observability.String("metric_name", rule.MetricName))

	c.JSON(http.StatusCreated, rule)
}

// deleteMetricAnomaly 删除指标异常规则
func (h *MockErrorHandler) deleteMetricAnomaly(c *gin.Context) {
	ctx := c.Request.Context()
	ruleID := c.Param("id")

	if err := h.errorService.DeleteRule(ctx, ruleID); err != nil {
		h.logger.Error(ctx, "Failed to delete metric anomaly rule",
			observability.String("rule_id", ruleID),
			observability.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info(ctx, "Metric anomaly rule deleted successfully",
		observability.String("rule_id", ruleID))

	c.JSON(http.StatusOK, gin.H{"message": "Metric anomaly rule deleted successfully"})
}

// checkMetricInjection 检查是否应该注入指标异常
func (h *MockErrorHandler) checkMetricInjection(c *gin.Context) {
	ctx := c.Request.Context()

	var request struct {
		Service    string `json:"service" binding:"required"`
		MetricName string `json:"metric_name" binding:"required"`
		Instance   string `json:"instance"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Error(ctx, "Failed to bind metric injection check request", observability.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	anomaly, shouldInject := h.errorService.ShouldInjectError(ctx, request.Service, request.MetricName, request.Instance)

	response := gin.H{
		"should_inject": shouldInject,
		"service":       request.Service,
		"metric_name":   request.MetricName,
		"instance":      request.Instance,
	}

	if shouldInject {
		response["anomaly"] = anomaly
	}

	c.JSON(http.StatusOK, response)
}

// getStats 获取统计信息
func (h *MockErrorHandler) getStats(c *gin.Context) {
	ctx := c.Request.Context()

	stats := h.errorService.GetStats(ctx)

	c.JSON(http.StatusOK, stats)
}

// healthCheck 健康检查
func (h *MockErrorHandler) healthCheck(c *gin.Context) {
	ctx := c.Request.Context()

	h.logger.Info(ctx, "Health check requested")

	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "mock-error-service",
		"timestamp": time.Now(),
	})
}
