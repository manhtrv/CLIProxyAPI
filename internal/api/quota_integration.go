package api

import (
	"github.com/gin-gonic/gin"
	configaccess "github.com/router-for-me/CLIProxyAPI/v6/internal/access/config_access"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/quota"
	log "github.com/sirupsen/logrus"
)

// setupQuotaMiddleware adds quota enforcement middleware to a route group if quota tracker is available.
func (s *Server) setupQuotaMiddleware(group *gin.RouterGroup) {
	tracker := configaccess.GetQuotaTracker()
	if tracker != nil {
		group.Use(quota.EnforcementMiddleware(tracker))
		log.Info("✅ Quota enforcement middleware added to routes")
	}
}

// SetupQuotaEnforcement is a legacy method kept for compatibility.
// The quota middleware is now set up directly in setupRoutes().
func (s *Server) SetupQuotaEnforcement() {
	// This is now handled in setupRoutes() via setupQuotaMiddleware()
}
