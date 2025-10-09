package receiver

import "github.com/gin-gonic/gin"

func RegisterReceiverRoutes(r *gin.Engine, h *Handler) {
	r.POST("/v1/integrations/alertmanager/webhook", h.AlertmanagerWebhook)
}
