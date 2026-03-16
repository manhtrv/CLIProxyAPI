package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	configaccess "github.com/router-for-me/CLIProxyAPI/v6/internal/access/config_access"
)

// GetQuotaStatus returns quota status for all API keys with extended configuration.
func (h *Handler) GetQuotaStatus(c *gin.Context) {
	tracker := configaccess.GetQuotaTracker()
	if tracker == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"message": "Quota tracking is not enabled",
		})
		return
	}

	allUsage := tracker.GetAllUsage()
	statuses := make([]map[string]interface{}, 0, len(allUsage))
	
	for apiKey := range allUsage {
		status := tracker.GetQuotaStatus(apiKey)
		// Mask API key for security (show only last 4 characters)
		if len(apiKey) > 4 {
			status["api_key_masked"] = "***" + apiKey[len(apiKey)-4:]
			delete(status, "api_key")
		}
		statuses = append(statuses, status)
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": true,
		"quotas":  statuses,
	})
}

// GetQuotaStatusByKey returns quota status for a specific API key.
func (h *Handler) GetQuotaStatusByKey(c *gin.Context) {
	apiKey := c.Query("key")
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "API key parameter 'key' is required",
		})
		return
	}

	tracker := configaccess.GetQuotaTracker()
	if tracker == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"message": "Quota tracking is not enabled",
		})
		return
	}

	if !tracker.HasConfig(apiKey) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "API key not found or has no quota configuration",
		})
		return
	}

	status := tracker.GetQuotaStatus(apiKey)
	c.JSON(http.StatusOK, status)
}

// ResetQuota resets quota usage for a specific API key.
func (h *Handler) ResetQuota(c *gin.Context) {
	apiKey := c.Query("key")
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "API key parameter 'key' is required",
		})
		return
	}

	tracker := configaccess.GetQuotaTracker()
	if tracker == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Quota tracking is not enabled",
		})
		return
	}

	if !tracker.HasConfig(apiKey) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "API key not found or has no quota configuration",
		})
		return
	}

	if err := tracker.ResetUsage(apiKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to reset quota: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Quota reset successfully",
		"status":  tracker.GetQuotaStatus(apiKey),
	})
}

// GetExtendedAPIKeys returns list of API keys with extended configuration.
func (h *Handler) GetExtendedAPIKeys(c *gin.Context) {
	if h.cfg == nil || len(h.cfg.APIKeysExtended) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"keys":    []interface{}{},
		})
		return
	}

	keys := make([]map[string]interface{}, 0, len(h.cfg.APIKeysExtended))
	for _, keyConfig := range h.cfg.APIKeysExtended {
		// Mask API key for security
		maskedKey := keyConfig.Key
		if len(maskedKey) > 4 {
			maskedKey = "***" + maskedKey[len(maskedKey)-4:]
		}

		keyInfo := map[string]interface{}{
			"key_masked":     maskedKey,
			"name":           keyConfig.Name,
			"weekly_quota":   keyConfig.WeeklyQuota,
			"allowed_models": keyConfig.AllowedModels,
		}
		keys = append(keys, keyInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": true,
		"count":   len(keys),
		"keys":    keys,
	})
}
