package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/zeroops/internal/service_manager/model"
	"github.com/qiniu/zeroops/internal/service_manager/service"
	"github.com/rs/zerolog/log"
)

// setupDeployRouters 设置部署管理相关路由
func (api *Api) setupDeployRouters(router *gin.Engine) {
	// 部署任务基本操作
	router.POST("/v1/deployments", api.CreateDeployment)
	router.GET("/v1/deployments", api.GetDeployments)
	router.GET("/v1/deployments/:deployID", api.GetDeploymentByID)
	router.POST("/v1/deployments/:deployID", api.UpdateDeployment)
	router.DELETE("/v1/deployments/:deployID", api.DeleteDeployment)

	// 部署任务控制操作
	router.POST("/v1/deployments/:deployID/pause", api.PauseDeployment)
	router.POST("/v1/deployments/:deployID/continue", api.ContinueDeployment)
	router.POST("/v1/deployments/:deployID/rollback", api.RollbackDeployment)
}

// ===== 部署管理相关API =====

// CreateDeployment 创建发布任务（POST /v1/deployments）
func (api *Api) CreateDeployment(c *gin.Context) {
	ctx := c.Request.Context()

	var req model.CreateDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "invalid request body: " + err.Error(),
		})
		return
	}

	if req.Service == "" || req.Version == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "service and version are required",
		})
		return
	}

	deployID, err := api.service.CreateDeployment(ctx, &req)
	if err != nil {
		if err == service.ErrServiceNotFound {
			c.JSON(http.StatusBadRequest, map[string]any{
				"error":   "bad request",
				"message": "service not found",
			})
			return
		}
		if err == service.ErrDeploymentConflict {
			c.JSON(http.StatusConflict, map[string]any{
				"error":   "conflict",
				"message": "deployment conflict: service version already in deployment",
			})
			return
		}
		log.Error().Err(err).
			Str("service", req.Service).
			Str("version", req.Version).
			Msg("failed to create deployment")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to create deployment",
		})
		return
	}

	c.JSON(http.StatusCreated, map[string]any{
		"id":      deployID,
		"message": "deployment created successfully",
	})
}

// GetDeploymentByID 获取发布任务详情（GET /v1/deployments/:deployID）
func (api *Api) GetDeploymentByID(c *gin.Context) {
	ctx := c.Request.Context()
	deployID := c.Param("deployID")

	if deployID == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "deployment ID is required",
		})
		return
	}

	deployment, err := api.service.GetDeploymentByID(ctx, deployID)
	if err != nil {
		if err == service.ErrDeploymentNotFound {
			c.JSON(http.StatusNotFound, map[string]any{
				"error":   "not found",
				"message": "deployment not found",
			})
			return
		}
		log.Error().Err(err).Str("deployID", deployID).Msg("failed to get deployment")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to get deployment",
		})
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// GetDeployments 获取发布任务列表（GET /v1/deployments）
func (api *Api) GetDeployments(c *gin.Context) {
	ctx := c.Request.Context()

	query := &model.DeploymentQuery{
		Type:    model.DeployState(c.Query("type")),
		Service: c.Query("service"),
		Start:   c.Query("start"),
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			query.Limit = limit
		}
	}

	deployments, err := api.service.GetDeployments(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("failed to get deployments")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to get deployments",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"items": deployments,
	})
}

// UpdateDeployment 修改发布任务（POST /v1/deployments/:deployID）
func (api *Api) UpdateDeployment(c *gin.Context) {
	ctx := c.Request.Context()
	deployID := c.Param("deployID")

	if deployID == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "deployment ID is required",
		})
		return
	}

	var req model.UpdateDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "invalid request body: " + err.Error(),
		})
		return
	}

	err := api.service.UpdateDeployment(ctx, deployID, &req)
	if err != nil {
		if err == service.ErrDeploymentNotFound {
			c.JSON(http.StatusNotFound, map[string]any{
				"error":   "not found",
				"message": "deployment not found",
			})
			return
		}
		if err == service.ErrInvalidDeployState {
			c.JSON(http.StatusBadRequest, map[string]any{
				"error":   "bad request",
				"message": "invalid deployment state for update",
			})
			return
		}
		log.Error().Err(err).Str("deployID", deployID).Msg("failed to update deployment")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to update deployment",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"message": "deployment updated successfully",
	})
}

// DeleteDeployment 删除发布任务（DELETE /v1/deployments/:deployID）
func (api *Api) DeleteDeployment(c *gin.Context) {
	ctx := c.Request.Context()
	deployID := c.Param("deployID")

	if deployID == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "deployment ID is required",
		})
		return
	}

	err := api.service.DeleteDeployment(ctx, deployID)
	if err != nil {
		if err == service.ErrDeploymentNotFound {
			c.JSON(http.StatusNotFound, map[string]any{
				"error":   "not found",
				"message": "deployment not found",
			})
			return
		}
		if err == service.ErrInvalidDeployState {
			c.JSON(http.StatusBadRequest, map[string]any{
				"error":   "bad request",
				"message": "invalid deployment state for deletion",
			})
			return
		}
		log.Error().Err(err).Str("deployID", deployID).Msg("failed to delete deployment")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to delete deployment",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"message": "deployment deleted successfully",
	})
}

// PauseDeployment 暂停发布任务（POST /v1/deployments/:deployID/pause）
func (api *Api) PauseDeployment(c *gin.Context) {
	ctx := c.Request.Context()
	deployID := c.Param("deployID")

	if deployID == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "deployment ID is required",
		})
		return
	}

	err := api.service.PauseDeployment(ctx, deployID)
	if err != nil {
		if err == service.ErrDeploymentNotFound {
			c.JSON(http.StatusNotFound, map[string]any{
				"error":   "not found",
				"message": "deployment not found",
			})
			return
		}
		if err == service.ErrInvalidDeployState {
			c.JSON(http.StatusBadRequest, map[string]any{
				"error":   "bad request",
				"message": "deployment cannot be paused in current state",
			})
			return
		}
		log.Error().Err(err).Str("deployID", deployID).Msg("failed to pause deployment")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to pause deployment",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"message": "deployment paused successfully",
	})
}

// ContinueDeployment 继续发布任务（POST /v1/deployments/:deployID/continue）
func (api *Api) ContinueDeployment(c *gin.Context) {
	ctx := c.Request.Context()
	deployID := c.Param("deployID")

	if deployID == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "deployment ID is required",
		})
		return
	}

	err := api.service.ContinueDeployment(ctx, deployID)
	if err != nil {
		if err == service.ErrDeploymentNotFound {
			c.JSON(http.StatusNotFound, map[string]any{
				"error":   "not found",
				"message": "deployment not found",
			})
			return
		}
		if err == service.ErrInvalidDeployState {
			c.JSON(http.StatusBadRequest, map[string]any{
				"error":   "bad request",
				"message": "deployment cannot be continued in current state",
			})
			return
		}
		log.Error().Err(err).Str("deployID", deployID).Msg("failed to continue deployment")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to continue deployment",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"message": "deployment continued successfully",
	})
}

// RollbackDeployment 回滚发布任务（POST /v1/deployments/:deployID/rollback）
func (api *Api) RollbackDeployment(c *gin.Context) {
	ctx := c.Request.Context()
	deployID := c.Param("deployID")

	if deployID == "" {
		c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "bad request",
			"message": "deployment ID is required",
		})
		return
	}

	err := api.service.RollbackDeployment(ctx, deployID)
	if err != nil {
		if err == service.ErrDeploymentNotFound {
			c.JSON(http.StatusNotFound, map[string]any{
				"error":   "not found",
				"message": "deployment not found",
			})
			return
		}
		if err == service.ErrInvalidDeployState {
			c.JSON(http.StatusBadRequest, map[string]any{
				"error":   "bad request",
				"message": "deployment cannot be rolled back in current state",
			})
			return
		}
		log.Error().Err(err).Str("deployID", deployID).Msg("failed to rollback deployment")
		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "internal server error",
			"message": "failed to rollback deployment",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"message": "deployment rolled back successfully",
	})
}
