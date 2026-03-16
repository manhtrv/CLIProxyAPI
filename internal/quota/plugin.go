package quota

import (
	"context"

	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	log "github.com/sirupsen/logrus"
)

// QuotaPlugin implements coreusage.Plugin to track token usage for quota enforcement.
type QuotaPlugin struct {
	tracker *Tracker
}

// NewQuotaPlugin creates a new quota tracking plugin.
func NewQuotaPlugin(tracker *Tracker) *QuotaPlugin {
	return &QuotaPlugin{tracker: tracker}
}

// HandleUsage implements coreusage.Plugin interface.
// It records token usage for API keys that have quota limits configured.
func (p *QuotaPlugin) HandleUsage(ctx context.Context, record coreusage.Record) {
	if p == nil || p.tracker == nil {
		return
	}

	// Extract API key from record
	apiKey := record.APIKey
	if apiKey == "" {
		return
	}

	// Only track usage for keys with quota configured
	if !p.tracker.HasConfig(apiKey) {
		return
	}

	// Calculate total tokens from the record
	// Use TotalTokens if available, otherwise sum Input + Output
	totalTokens := record.Detail.TotalTokens
	if totalTokens == 0 {
		totalTokens = record.Detail.InputTokens + record.Detail.OutputTokens
	}
	
	if totalTokens == 0 {
		// If no tokens recorded yet, skip
		return
	}

	// Record the usage
	if err := p.tracker.RecordUsage(apiKey, totalTokens); err != nil {
		log.Warnf("Failed to record quota usage for key %s: %v", apiKey, err)
		return
	}

	log.Debugf("Recorded %d tokens for API key %s (quota tracking)", totalTokens, apiKey)
}
