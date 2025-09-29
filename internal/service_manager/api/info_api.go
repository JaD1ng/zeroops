package api

import (
	"net/http"

	"github.com/fox-gonic/fox"
	"github.com/qiniu/zeroops/internal/service_manager/model"
	"github.com/qiniu/zeroops/internal/service_manager/service"
	"github.com/rs/zerolog/log"
)

// setupInfoRouters 设置服务信息相关路由
func (api *Api) setupInfoRouters(router *fox.Engine) {
	// 服务列表和信息查询
	router.GET("/v1/services", api.GetServices)
	router.GET("/v1/services/:service/activeVersions", api.GetServiceActiveVersions)
	router.GET("/v1/services/:service/availableVersions", api.GetServiceAvailableVersions)
	router.GET("/v1/metrics/:service/:name", api.GetServiceMetricTimeSeries)

	// 服务管理（CRUD）
	router.POST("/v1/services", api.CreateService)
	router.PUT("/v1/services/:service", api.UpdateService)
	router.DELETE("/v1/services/:service", api.DeleteService)
}

// ===== 服务信息查询API =====

// GetServices 获取所有服务列表（GET /v1/services）
func (api *Api) GetServices(c *fox.Context) {
	ctx := c.Request.Context()

	response, err := api.service.GetServicesResponse(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to get services")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to get services",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetServiceActiveVersions 获取服务活跃版本（GET /v1/services/:service/activeVersions）
func (api *Api) GetServiceActiveVersions(c *fox.Context) {
	ctx := c.Request.Context()
	serviceName := c.Param("service")

	if serviceName == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "service name is required",
		})
		return
	}

	activeVersions, err := api.service.GetServiceActiveVersions(ctx, serviceName)
	if err != nil {
		log.Error().Err(err).Str("service", serviceName).Msg("failed to get service active versions")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to get service active versions",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"items": activeVersions,
	})
}

// GetServiceAvailableVersions 获取可用服务版本（GET /v1/services/:service/availableVersions）
func (api *Api) GetServiceAvailableVersions(c *fox.Context) {
	ctx := c.Request.Context()
	serviceName := c.Param("service")
	versionType := c.Query("type")

	if serviceName == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "service name is required",
		})
		return
	}

	versions, err := api.service.GetServiceAvailableVersions(ctx, serviceName, versionType)
	if err != nil {
		log.Error().Err(err).
			Str("service", serviceName).
			Str("type", versionType).
			Msg("failed to get service available versions")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to get service available versions",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"items": versions,
	})
}

// GetServiceMetricTimeSeries 获取服务时序指标数据（GET /v1/metrics/:service/:name）
func (api *Api) GetServiceMetricTimeSeries(c *fox.Context) {
	ctx := c.Request.Context()
	serviceName := c.Param("service")
	metricName := c.Param("name")

	if serviceName == "" || metricName == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "service name and metric name are required",
		})
		return
	}

	// 绑定查询参数
	var query model.MetricTimeSeriesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "invalid query parameters: " + err.Error(),
		})
		return
	}

	// 设置路径参数
	query.Service = serviceName
	query.Name = metricName

	response, err := api.service.GetServiceMetricTimeSeries(ctx, serviceName, metricName, &query)
	if err != nil {
		log.Error().Err(err).
			Str("service", serviceName).
			Str("metric", metricName).
			Msg("failed to get service metric time series")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to get service metric time series",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ===== 服务管理CRUD API =====

// CreateService 创建服务（POST /v1/services）
func (api *Api) CreateService(c *fox.Context) {
	ctx := c.Request.Context()

	var service model.Service
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "invalid request body: " + err.Error(),
		})
		return
	}

	if service.Name == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "service name is required",
		})
		return
	}

	if err := api.service.CreateService(ctx, &service); err != nil {
		log.Error().Err(err).Str("service", service.Name).Msg("failed to create service")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to create service",
		})
		return
	}

	c.JSON(http.StatusCreated, map[string]any{
		"message": "service created successfully",
		"service": service.Name,
	})
}

// UpdateService 更新服务信息（PUT /v1/services/:service）
func (api *Api) UpdateService(c *fox.Context) {
	ctx := c.Request.Context()
	serviceName := c.Param("service")

	if serviceName == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "service name is required",
		})
		return
	}

	var svc model.Service
	if err := c.ShouldBindJSON(&svc); err != nil {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "invalid request body: " + err.Error(),
		})
		return
	}

	// 确保URL参数和请求体中的服务名一致
	svc.Name = serviceName

	if err := api.service.UpdateService(ctx, &svc); err != nil {
		if err == service.ErrServiceNotFound {
			c.JSON(http.StatusNotFound, map[string]any{
				"error":   "not found",
				"message": "service not found",
			})
			return
		}
		log.Error().Err(err).Str("service", serviceName).Msg("failed to update service")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to update service",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"message": "service updated successfully",
		"service": serviceName,
	})
}

// DeleteService 删除服务（DELETE /v1/services/:service）
func (api *Api) DeleteService(c *fox.Context) {
	ctx := c.Request.Context()
	serviceName := c.Param("service")

	if serviceName == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "service name is required",
		})
		return
	}

	if err := api.service.DeleteService(ctx, serviceName); err != nil {
		if err == service.ErrServiceNotFound {
			c.JSON(http.StatusNotFound, map[string]any{
				"error":   "not found",
				"message": "service not found",
			})
			return
		}
		log.Error().Err(err).Str("service", serviceName).Msg("failed to delete service")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to delete service",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"message": "service deleted successfully",
		"service": serviceName,
	})
}
