// Package quota provides API key quota limiting and tracking functionality.
package quota

import (
	"time"
)

// APIKeyConfig represents extended configuration for an API key with quota and model restrictions.
type APIKeyConfig struct {
	// Key is the API key string
	Key string `yaml:"key" json:"key"`

	// WeeklyQuota is the maximum number of tokens allowed per week (resets Sunday 00:00 UTC)
	// Set to 0 for unlimited
	WeeklyQuota int64 `yaml:"weekly-quota" json:"weekly-quota"`

	// AllowedModels is the list of model IDs this API key can access
	// If empty or nil, all models are allowed
	AllowedModels []string `yaml:"allowed-models,omitempty" json:"allowed-models,omitempty"`

	// Name is an optional friendly name for this API key
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
}

// QuotaUsage tracks the current usage for an API key within a quota period.
type QuotaUsage struct {
	// APIKey is the API key this usage applies to
	APIKey string `json:"api_key"`

	// TokensUsed is the number of tokens consumed in the current period
	TokensUsed int64 `json:"tokens_used"`

	// PeriodStart is when the current quota period started (UTC)
	PeriodStart time.Time `json:"period_start"`

	// PeriodEnd is when the current quota period ends (UTC)
	PeriodEnd time.Time `json:"period_end"`

	// LastUpdated is when this usage was last modified
	LastUpdated time.Time `json:"last_updated"`
}

// IsExpired checks if the current quota period has expired.
func (u *QuotaUsage) IsExpired() bool {
	return time.Now().UTC().After(u.PeriodEnd)
}

// GetNextPeriodStart calculates the start of the next quota period (next Sunday 00:00 UTC).
func GetNextPeriodStart(from time.Time) time.Time {
	t := from.UTC()
	// Calculate days until next Sunday
	daysUntilSunday := (7 - int(t.Weekday())) % 7
	if daysUntilSunday == 0 {
		daysUntilSunday = 7
	}
	
	nextSunday := t.AddDate(0, 0, daysUntilSunday)
	// Set to 00:00:00
	return time.Date(nextSunday.Year(), nextSunday.Month(), nextSunday.Day(), 0, 0, 0, 0, time.UTC)
}

// GetCurrentPeriodStart calculates the start of the current quota period (last Sunday 00:00 UTC).
func GetCurrentPeriodStart(now time.Time) time.Time {
	t := now.UTC()
	// Calculate days since last Sunday
	daysSinceSunday := int(t.Weekday())
	if daysSinceSunday == 0 {
		// Today is Sunday, check if we're past midnight
		if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
			daysSinceSunday = 0
		} else {
			daysSinceSunday = 0
		}
	}
	
	lastSunday := t.AddDate(0, 0, -daysSinceSunday)
	// Set to 00:00:00
	return time.Date(lastSunday.Year(), lastSunday.Month(), lastSunday.Day(), 0, 0, 0, 0, time.UTC)
}

// NewQuotaUsage creates a new quota usage record for the current period.
func NewQuotaUsage(apiKey string) *QuotaUsage {
	now := time.Now().UTC()
	periodStart := GetCurrentPeriodStart(now)
	periodEnd := GetNextPeriodStart(now)
	
	return &QuotaUsage{
		APIKey:      apiKey,
		TokensUsed:  0,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		LastUpdated: now,
	}
}
