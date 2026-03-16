package quota

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// getAuthMetadata retrieves authentication metadata from Gin context.
func getAuthMetadata(c *gin.Context) (apiKey string, metadata map[string]string) {
	if key, exists := c.Get("apiKey"); exists {
		if keyStr, ok := key.(string); ok {
			apiKey = keyStr
		}
	}
	if meta, exists := c.Get("accessMetadata"); exists {
		if metaMap, ok := meta.(map[string]string); ok {
			metadata = metaMap
		}
	}
	return
}

// EnforcementMiddleware creates a middleware that enforces API key quota limits.
func EnforcementMiddleware(tracker *Tracker) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get authentication result from context
		apiKey, metadata := getAuthMetadata(c)
		if apiKey == "" || metadata == nil {
			// No authentication, skip quota check
			c.Next()
			return
		}

		// Check if quota enforcement is enabled for this key
		if metadata["quota-enabled"] == "true" {
			allowed, _ := strconv.ParseBool(metadata["quota-allowed"])
			if !allowed {
				// Quota exceeded
				usage := tracker.GetUsage(apiKey)
				c.Header("X-RateLimit-Limit", metadata["quota-limit"])
				c.Header("X-RateLimit-Remaining", "0")
				c.Header("X-RateLimit-Reset", strconv.FormatInt(usage.PeriodEnd.Unix(), 10))
				
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": gin.H{
						"message": fmt.Sprintf("Weekly quota exceeded. Resets at %s", usage.PeriodEnd.Format(time.RFC3339)),
						"type":    "quota_exceeded",
						"code":    "quota_exceeded",
						"quota_limit": metadata["quota-limit"],
						"quota_used": metadata["quota-limit"],
						"reset_at": usage.PeriodEnd.Format(time.RFC3339),
					},
				})
				c.Abort()
				return
			}

			// Add quota headers to response
			c.Header("X-RateLimit-Limit", metadata["quota-limit"])
			c.Header("X-RateLimit-Remaining", metadata["quota-remaining"])
		}

		c.Next()
	}
}
