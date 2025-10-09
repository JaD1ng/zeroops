package receiver

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

var (
	cfgBasicUser string
	cfgBasicPass string
	cfgBearer    string
)

// ConfigureAuth sets credentials for webhook auth (config-driven). Empty values disable that auth type.
func ConfigureAuth(user, pass, bearer string) {
	cfgBasicUser = user
	cfgBasicPass = pass
	cfgBearer = bearer
}

func authEnabled() bool {
	// prefer config
	if cfgBasicUser != "" || cfgBasicPass != "" || cfgBearer != "" {
		return true
	}
	// fallback to env for backward compatibility
	return os.Getenv("ALERT_WEBHOOK_BASIC_USER") != "" ||
		os.Getenv("ALERT_WEBHOOK_BASIC_PASS") != "" ||
		os.Getenv("ALERT_WEBHOOK_BEARER") != ""
}

// AuthMiddleware returns false if unauthorized and writes a 401 response.
func AuthMiddleware(c *gin.Context) bool {
	if !authEnabled() {
		return true
	}

	// prefer config
	user := cfgBasicUser
	pass := cfgBasicPass
	bearer := cfgBearer
	// fallback to env if not set in config
	if user == "" && pass == "" && bearer == "" {
		user = os.Getenv("ALERT_WEBHOOK_BASIC_USER")
		pass = os.Getenv("ALERT_WEBHOOK_BASIC_PASS")
		bearer = os.Getenv("ALERT_WEBHOOK_BEARER")
	}

	if user != "" || pass != "" {
		u, p, ok := c.Request.BasicAuth()
		if !ok || u != user || p != pass {
			c.JSON(http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
			return false
		}
		return true
	}

	if bearer != "" {
		if c.GetHeader("Authorization") != "Bearer "+bearer {
			c.JSON(http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
			return false
		}
	}
	return true
}
