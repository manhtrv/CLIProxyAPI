package quota

import (
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// Tracker manages API key quotas and usage tracking.
type Tracker struct {
	configs map[string]*APIKeyConfig // apiKey -> config
	storage *Storage
	mu      sync.RWMutex
}

// NewTracker creates a new quota tracker.
func NewTracker(configs []*APIKeyConfig, dataDir string) (*Tracker, error) {
	storage, err := NewStorage(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create quota storage: %w", err)
	}

	configMap := make(map[string]*APIKeyConfig, len(configs))
	for _, cfg := range configs {
		if cfg.Key != "" {
			configMap[cfg.Key] = cfg
		}
	}

	tracker := &Tracker{
		configs: configMap,
		storage: storage,
	}

	// Start cleanup task (runs every 6 hours)
	storage.StartCleanupTask(6 * time.Hour)

	log.Infof("Quota tracker initialized with %d API keys", len(configMap))
	return tracker, nil
}

// GetConfig returns the configuration for an API key.
func (t *Tracker) GetConfig(apiKey string) *APIKeyConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.configs[apiKey]
}

// HasConfig checks if an API key has quota configuration.
func (t *Tracker) HasConfig(apiKey string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.configs[apiKey]
	return ok
}

// CheckQuota verifies if an API key has sufficient quota remaining.
// Returns (allowed bool, remaining tokens, error)
func (t *Tracker) CheckQuota(apiKey string) (bool, int64, error) {
	t.mu.RLock()
	config, ok := t.configs[apiKey]
	t.mu.RUnlock()

	if !ok {
		// No quota config = unlimited
		return true, -1, nil
	}

	if config.WeeklyQuota == 0 {
		// Quota set to 0 = unlimited
		return true, -1, nil
	}

	usage := t.storage.Get(apiKey)
	if usage == nil {
		// No usage yet = full quota available
		return true, config.WeeklyQuota, nil
	}

	remaining := config.WeeklyQuota - usage.TokensUsed
	if remaining <= 0 {
		return false, 0, nil
	}

	return true, remaining, nil
}

// RecordUsage records token usage for an API key.
func (t *Tracker) RecordUsage(apiKey string, tokens int64) error {
	t.mu.RLock()
	_, hasConfig := t.configs[apiKey]
	t.mu.RUnlock()

	if !hasConfig {
		// No quota config, no need to track
		return nil
	}

	return t.storage.AddUsage(apiKey, tokens)
}

// GetUsage returns current usage for an API key.
func (t *Tracker) GetUsage(apiKey string) *QuotaUsage {
	usage := t.storage.Get(apiKey)
	if usage == nil {
		// Return a fresh usage record
		return NewQuotaUsage(apiKey)
	}
	return usage
}

// GetAllUsage returns usage for all tracked API keys.
func (t *Tracker) GetAllUsage() map[string]*QuotaUsage {
	return t.storage.GetAll()
}

// ResetUsage resets quota usage for an API key.
func (t *Tracker) ResetUsage(apiKey string) error {
	return t.storage.Reset(apiKey)
}

// IsModelAllowed checks if a model is allowed for an API key.
func (t *Tracker) IsModelAllowed(apiKey, modelID string) bool {
	t.mu.RLock()
	config, ok := t.configs[apiKey]
	t.mu.RUnlock()

	if !ok {
		// No config = all models allowed
		return true
	}

	if len(config.AllowedModels) == 0 {
		// Empty allowed list = all models allowed
		return true
	}

	// Check if model is in allowed list
	for _, allowed := range config.AllowedModels {
		if matchModel(modelID, allowed) {
			return true
		}
	}

	return false
}

// GetAllowedModels returns the list of allowed models for an API key.
// Returns nil if all models are allowed.
func (t *Tracker) GetAllowedModels(apiKey string) []string {
	t.mu.RLock()
	config, ok := t.configs[apiKey]
	t.mu.RUnlock()

	if !ok || len(config.AllowedModels) == 0 {
		return nil
	}

	return config.AllowedModels
}

// matchModel checks if a model ID matches an allowed pattern.
// Supports wildcards: "gemini-*", "*-pro", "*flash*"
func matchModel(modelID, pattern string) bool {
	// Exact match
	if modelID == pattern {
		return true
	}

	// Wildcard matching
	if strings.Contains(pattern, "*") {
		if pattern == "*" {
			return true
		}

		// Prefix match: "gemini-*"
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			return strings.HasPrefix(modelID, prefix)
		}

		// Suffix match: "*-pro"
		if strings.HasPrefix(pattern, "*") {
			suffix := strings.TrimPrefix(pattern, "*")
			return strings.HasSuffix(modelID, suffix)
		}

		// Substring match: "*flash*"
		if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
			substring := strings.Trim(pattern, "*")
			return strings.Contains(modelID, substring)
		}
	}

	return false
}

// GetQuotaStatus returns a summary of quota status for an API key.
func (t *Tracker) GetQuotaStatus(apiKey string) map[string]interface{} {
	t.mu.RLock()
	config, ok := t.configs[apiKey]
	t.mu.RUnlock()

	status := map[string]interface{}{
		"api_key": apiKey,
		"has_quota_limit": ok && config.WeeklyQuota > 0,
	}

	if !ok {
		status["quota_limit"] = "unlimited"
		status["tokens_used"] = 0
		status["tokens_remaining"] = "unlimited"
		return status
	}

	if config.WeeklyQuota == 0 {
		status["quota_limit"] = "unlimited"
		status["tokens_used"] = 0
		status["tokens_remaining"] = "unlimited"
	} else {
		usage := t.storage.Get(apiKey)
		if usage == nil {
			usage = NewQuotaUsage(apiKey)
		}

		status["quota_limit"] = config.WeeklyQuota
		status["tokens_used"] = usage.TokensUsed
		status["tokens_remaining"] = config.WeeklyQuota - usage.TokensUsed
		status["period_start"] = usage.PeriodStart.Format(time.RFC3339)
		status["period_end"] = usage.PeriodEnd.Format(time.RFC3339)
		status["last_updated"] = usage.LastUpdated.Format(time.RFC3339)
	}

	if len(config.AllowedModels) > 0 {
		status["allowed_models"] = config.AllowedModels
	} else {
		status["allowed_models"] = "all"
	}

	if config.Name != "" {
		status["name"] = config.Name
	}

	return status
}
