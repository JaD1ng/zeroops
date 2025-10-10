package receiver

import (
	"github.com/gin-gonic/gin"
)

// ConfigureAuth is a no-op; authentication disabled at source.
func ConfigureAuth(user, pass, bearer string) {}

// AuthMiddleware returns false if unauthorized and writes a 401 response.
func AuthMiddleware(c *gin.Context) bool { return true }
