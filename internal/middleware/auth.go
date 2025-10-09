package middleware

import (
	"github.com/gin-gonic/gin"
)

// Authentication is a placeholder global middleware. It currently allows all requests.
// Per-alerting webhook uses its own path-scoped auth.
func Authentication(c *gin.Context) {
	c.Next()
}
