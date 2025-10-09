package receiver

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	dao   AlertIssueDAO
	cache AlertIssueCache
}

// NewHandler keeps backward compatibility and uses a NoopCache by default.
func NewHandler(dao AlertIssueDAO) *Handler { return &Handler{dao: dao, cache: NoopCache{}} }

// NewHandlerWithCache allows injecting a real cache implementation.
func NewHandlerWithCache(dao AlertIssueDAO, cache AlertIssueCache) *Handler {
	if cache == nil {
		cache = NoopCache{}
	}
	return &Handler{dao: dao, cache: cache}
}

func (h *Handler) AlertmanagerWebhook(c *gin.Context) {
	log.Info().Msg("AlertmanagerWebhook: starting webhook processing")

	if !AuthMiddleware(c) {
		log.Warn().Msg("AlertmanagerWebhook: authentication failed")
		return
	}
	log.Debug().Msg("AlertmanagerWebhook: authentication successful")

	var req AMWebhook
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error().Err(err).Msg("AlertmanagerWebhook: failed to parse JSON request")
		c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid JSON"})
		return
	}
	log.Debug().Int("alert_count", len(req.Alerts)).Str("status", req.Status).Msg("AlertmanagerWebhook: JSON parsed successfully")

	if err := ValidateAMWebhook(&req); err != nil {
		log.Error().Err(err).Msg("AlertmanagerWebhook: webhook validation failed")
		c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	log.Debug().Msg("AlertmanagerWebhook: webhook validation passed")

	if strings.ToLower(req.Status) != "firing" {
		log.Info().Str("status", req.Status).Msg("AlertmanagerWebhook: ignoring non-firing alert")
		c.JSON(http.StatusOK, map[string]any{"ok": true, "msg": "ignored (not firing)"})
		return
	}
	log.Info().Int("alert_count", len(req.Alerts)).Msg("AlertmanagerWebhook: processing firing alerts")

	created := 0
	for i, a := range req.Alerts {
		log.Debug().Int("alert_index", i).Str("alert_name", a.Labels["alertname"]).Msg("AlertmanagerWebhook: processing alert")

		key := BuildIdempotencyKey(a)
		// Distributed idempotency (best-effort). If key exists, skip.
		if ok, _ := h.cache.TryMarkIdempotent(c.Request.Context(), a); !ok {
			log.Debug().Str("idempotency_key", key).Msg("AlertmanagerWebhook: alert already processed (distributed cache)")
			continue
		}
		if AlreadySeen(key) {
			log.Debug().Str("idempotency_key", key).Msg("AlertmanagerWebhook: alert already processed (local cache)")
			continue
		}

		row, err := MapToAlertIssueRow(&req, &a)
		if err != nil {
			log.Error().Err(err).Str("alert_name", a.Labels["alertname"]).Msg("AlertmanagerWebhook: failed to map alert to issue row")
			continue
		}
		log.Debug().Str("issue_id", row.ID).Str("level", row.Level).Msg("AlertmanagerWebhook: alert mapped to issue row")

		if err := h.dao.InsertAlertIssue(c.Request.Context(), row); err != nil {
			log.Error().Err(err).Str("issue_id", row.ID).Msg("AlertmanagerWebhook: failed to insert alert issue to database")
			continue
		}
		log.Info().Str("issue_id", row.ID).Str("level", row.Level).Msg("AlertmanagerWebhook: alert issue inserted to database")

		if w, ok := h.dao.(ServiceStateWriter); ok {
			service := strings.TrimSpace(a.Labels["service"])
			version := strings.TrimSpace(a.Labels["service_version"]) // optional
			if service != "" {
				derived := "Warning"
				if row.Level == "P0" {
					derived = "Error"
				} else if row.Level == "P1" || row.Level == "P2" {
					derived = "Warning"
				}
				log.Debug().Str("service", service).Str("version", version).Str("derived_state", derived).Msg("AlertmanagerWebhook: updating service state")

				if err := w.UpsertServiceState(c.Request.Context(), service, version, nil, derived, row.ID); err != nil {
					log.Error().Err(err).Str("service", service).Str("version", version).Msg("AlertmanagerWebhook: failed to upsert service state")
				} else {
					log.Info().Str("service", service).Str("version", version).Str("state", derived).Msg("AlertmanagerWebhook: service state updated")
				}

				if err := h.cache.WriteServiceState(c.Request.Context(), service, version, time.Time{}, derived); err != nil {
					log.Error().Err(err).Str("service", service).Str("version", version).Msg("AlertmanagerWebhook: failed to write service state to cache")
				} else {
					log.Debug().Str("service", service).Str("version", version).Msg("AlertmanagerWebhook: service state written to cache")
				}
			} else {
				log.Debug().Msg("AlertmanagerWebhook: no service label found, skipping service state update")
			}
		}

		// Write-through to cache. Errors are ignored to avoid impacting webhook ack.
		if err := h.cache.WriteIssue(c.Request.Context(), row, a); err != nil {
			log.Error().Err(err).Str("issue_id", row.ID).Msg("AlertmanagerWebhook: failed to write issue to cache")
		} else {
			log.Debug().Str("issue_id", row.ID).Msg("AlertmanagerWebhook: issue written to cache")
		}

		MarkSeen(key)
		created++
		log.Info().Str("issue_id", row.ID).Int("total_created", created).Msg("AlertmanagerWebhook: alert processing completed")
	}

	log.Info().Int("total_alerts", len(req.Alerts)).Int("created_issues", created).Msg("AlertmanagerWebhook: webhook processing completed")
	c.JSON(http.StatusOK, map[string]any{"ok": true, "created": created})
}
