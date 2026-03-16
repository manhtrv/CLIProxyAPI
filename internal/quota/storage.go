package quota

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// Storage handles persistent storage of quota usage data.
type Storage struct {
	filePath string
	mu       sync.RWMutex
	data     map[string]*QuotaUsage // apiKey -> usage
}

// NewStorage creates a new storage instance.
func NewStorage(dataDir string) (*Storage, error) {
	if dataDir == "" {
		// Default to /CLIProxyAPI directory
		dataDir = "/CLIProxyAPI"
	}

	// Expand ~ if present
	if len(dataDir) > 0 && dataDir[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			dataDir = filepath.Join(home, dataDir[1:])
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	filePath := filepath.Join(dataDir, "quotas.json")
	s := &Storage{
		filePath: filePath,
		data:     make(map[string]*QuotaUsage),
	}

	// Load existing data
	if err := s.load(); err != nil {
		log.Warnf("Failed to load quota data from %s: %v (starting fresh)", filePath, err)
	}

	return s, nil
}

// Get retrieves quota usage for an API key.
func (s *Storage) Get(apiKey string) *QuotaUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	usage, ok := s.data[apiKey]
	if !ok {
		return nil
	}
	
	// Check if period expired
	if usage.IsExpired() {
		return nil
	}
	
	return usage
}

// Set updates quota usage for an API key.
func (s *Storage) Set(apiKey string, usage *QuotaUsage) error {
	s.mu.Lock()
	usage.LastUpdated = time.Now().UTC()
	s.data[apiKey] = usage
	s.mu.Unlock()
	
	return s.save()
}

// AddUsage increments token usage for an API key.
func (s *Storage) AddUsage(apiKey string, tokens int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	usage, ok := s.data[apiKey]
	if !ok || usage.IsExpired() {
		usage = NewQuotaUsage(apiKey)
		s.data[apiKey] = usage
	}
	
	usage.TokensUsed += tokens
	usage.LastUpdated = time.Now().UTC()
	
	return s.save()
}

// GetAll returns all quota usage records.
func (s *Storage) GetAll() map[string]*QuotaUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make(map[string]*QuotaUsage, len(s.data))
	for k, v := range s.data {
		result[k] = v
	}
	return result
}

// Reset clears quota usage for an API key.
func (s *Storage) Reset(apiKey string) error {
	s.mu.Lock()
	delete(s.data, apiKey)
	s.mu.Unlock()
	
	return s.save()
}

// load reads quota data from disk.
func (s *Storage) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's OK
		}
		return err
	}

	var stored map[string]*QuotaUsage
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}

	s.data = stored
	return nil
}

// save writes quota data to disk.
func (s *Storage) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// Cleanup removes expired quota records.
func (s *Storage) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for key, usage := range s.data {
		if usage.IsExpired() {
			// Keep for historical purposes but could be cleaned up
			// For now, we'll keep expired records
			log.Debugf("Quota period expired for API key: %s (expired at %s)", key, usage.PeriodEnd.Format(time.RFC3339))
		}
	}
	
	// Save after cleanup
	_ = s.save()
}

// StartCleanupTask starts a background task to periodically clean up expired quota records.
func (s *Storage) StartCleanupTask(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			s.Cleanup()
		}
	}()
}
