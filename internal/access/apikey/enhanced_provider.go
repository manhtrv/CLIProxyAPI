// Package apikey provides an enhanced API key authentication provider
// with quota and model filtering capabilities.
package apikey

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	sdkaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/quota"
)

// provider implements the authentication provider interface with quota support.
type provider struct {
	name         string
	keys         map[string]struct{}
	quotaTracker *quota.Tracker
}

// newProvider creates a new enhanced API key provider.
func newProvider(name string, keys []string, quotaTracker *quota.Tracker) *provider {
	providerName := strings.TrimSpace(name)
	if providerName == "" {
		providerName = sdkaccess.DefaultAccessProviderName
	}
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}
	return &provider{
		name:         providerName,
		keys:         keySet,
		quotaTracker: quotaTracker,
	}
}

// Identifier returns the provider identifier.
func (p *provider) Identifier() string {
	if p == nil || p.name == "" {
		return sdkaccess.DefaultAccessProviderName
	}
	return p.name
}

// Authenticate performs API key authentication and enriches result with quota metadata.
func (p *provider) Authenticate(_ context.Context, r *http.Request) (*sdkaccess.Result, *sdkaccess.AuthError) {
	if p == nil {
		return nil, sdkaccess.NewNotHandledError()
	}
	if len(p.keys) == 0 {
		return nil, sdkaccess.NewNotHandledError()
	}

	authHeader := r.Header.Get("Authorization")
	authHeaderGoogle := r.Header.Get("X-Goog-Api-Key")
	authHeaderAnthropic := r.Header.Get("X-Api-Key")
	queryKey := ""
	queryAuthToken := ""
	if r.URL != nil {
		queryKey = r.URL.Query().Get("key")
		queryAuthToken = r.URL.Query().Get("auth_token")
	}
	if authHeader == "" && authHeaderGoogle == "" && authHeaderAnthropic == "" && queryKey == "" && queryAuthToken == "" {
		return nil, sdkaccess.NewNoCredentialsError()
	}

	apiKey := extractBearerToken(authHeader)

	candidates := []struct {
		value  string
		source string
	}{
		{apiKey, "authorization"},
		{authHeaderGoogle, "x-goog-api-key"},
		{authHeaderAnthropic, "x-api-key"},
		{queryKey, "query-key"},
		{queryAuthToken, "query-auth-token"},
	}

	for _, candidate := range candidates {
		if candidate.value == "" {
			continue
		}
		if _, ok := p.keys[candidate.value]; ok {
			result := &sdkaccess.Result{
				Provider:  p.Identifier(),
				Principal: candidate.value,
				Metadata: map[string]string{
					"source": candidate.source,
				},
			}

			// Enrich result with quota metadata if available
			p.enrichWithQuotaMetadata(result)

			return result, nil
		}
	}

	return nil, sdkaccess.NewInvalidCredentialError()
}

// enrichWithQuotaMetadata adds quota and model filtering metadata to the authentication result.
func (p *provider) enrichWithQuotaMetadata(result *sdkaccess.Result) {
	if p.quotaTracker == nil || !p.quotaTracker.HasConfig(result.Principal) {
		return
	}

	config := p.quotaTracker.GetConfig(result.Principal)

	// Add quota information to metadata
	if config.WeeklyQuota > 0 {
		allowed, remaining, _ := p.quotaTracker.CheckQuota(result.Principal)
		result.Metadata["quota-enabled"] = "true"
		result.Metadata["quota-limit"] = fmt.Sprintf("%d", config.WeeklyQuota)
		result.Metadata["quota-remaining"] = fmt.Sprintf("%d", remaining)
		result.Metadata["quota-allowed"] = fmt.Sprintf("%t", allowed)
	}

	// Add allowed models to metadata
	if len(config.AllowedModels) > 0 {
		result.Metadata["allowed-models"] = strings.Join(config.AllowedModels, ",")
	}

	// Add key name if set
	if config.Name != "" {
		result.Metadata["key-name"] = config.Name
	}
}

func extractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return header
	}
	if strings.ToLower(parts[0]) != "bearer" {
		return header
	}
	return strings.TrimSpace(parts[1])
}

func normalizeKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		if _, exists := seen[trimmedKey]; exists {
			continue
		}
		seen[trimmedKey] = struct{}{}
		normalized = append(normalized, trimmedKey)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// Register registers the enhanced API key provider with quota tracking.
func Register(apiKeys []string, extendedKeys []*quota.APIKeyConfig, quotaTracker *quota.Tracker) {
	// Combine simple keys with extended keys
	allKeys := make([]string, 0, len(apiKeys)+len(extendedKeys))
	allKeys = append(allKeys, apiKeys...)
	for _, cfg := range extendedKeys {
		if cfg.Key != "" {
			allKeys = append(allKeys, cfg.Key)
		}
	}

	keys := normalizeKeys(allKeys)
	if len(keys) == 0 {
		sdkaccess.UnregisterProvider(sdkaccess.AccessProviderTypeConfigAPIKey)
		return
	}

	sdkaccess.RegisterProvider(
		sdkaccess.AccessProviderTypeConfigAPIKey,
		newProvider(sdkaccess.DefaultAccessProviderName, keys, quotaTracker),
	)
}
