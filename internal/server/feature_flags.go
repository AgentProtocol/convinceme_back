package server

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// FeatureFlags represents the feature flags for the application
type FeatureFlags struct {
	// Authentication features
	RequireEmailVerification bool `json:"require_email_verification"`
	RequireInvitation        bool `json:"require_invitation"`
	AllowPasswordReset       bool `json:"allow_password_reset"`
	AllowSocialLogin         bool `json:"allow_social_login"`

	// Security features
	EnableRateLimiting   bool `json:"enable_rate_limiting"`
	EnableCSRFProtection bool `json:"enable_csrf_protection"`

	// User experience features
	EnableFeedbackCollection bool `json:"enable_feedback_collection"`
	EnableAnalytics          bool `json:"enable_analytics"`

	// Admin features
	EnableAdminDashboard bool `json:"enable_admin_dashboard"`
}

// FeatureFlagManager manages feature flags
type FeatureFlagManager struct {
	flags      FeatureFlags
	configPath string
	mu         sync.RWMutex
}

// NewFeatureFlagManager creates a new feature flag manager
func NewFeatureFlagManager(configPath string) (*FeatureFlagManager, error) {
	manager := &FeatureFlagManager{
		configPath: configPath,
		flags: FeatureFlags{
			// Default values
			RequireEmailVerification: false,
			RequireInvitation:        true, // Default to true for closed alpha
			AllowPasswordReset:       true,
			AllowSocialLogin:         false,
			EnableRateLimiting:       true,
			EnableCSRFProtection:     false, // Disabled for alpha for easier testing
			EnableFeedbackCollection: true,
			EnableAnalytics:          true,
			EnableAdminDashboard:     true,
		},
	}

	// Load configuration from file if it exists
	if _, err := os.Stat(configPath); err == nil {
		err := manager.loadFromFile()
		if err != nil {
			return nil, fmt.Errorf("failed to load feature flags: %v", err)
		}
	} else {
		// Save default configuration
		err := manager.saveToFile()
		if err != nil {
			return nil, fmt.Errorf("failed to save default feature flags: %v", err)
		}
	}

	return manager, nil
}

// GetFlags returns the current feature flags
func (m *FeatureFlagManager) GetFlags() FeatureFlags {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.flags
}

// UpdateFlags updates the feature flags
func (m *FeatureFlagManager) UpdateFlags(flags FeatureFlags) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.flags = flags
	return m.saveToFile()
}

// loadFromFile loads feature flags from a file
func (m *FeatureFlagManager) loadFromFile() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read feature flags file: %v", err)
	}

	var flags FeatureFlags
	err = json.Unmarshal(data, &flags)
	if err != nil {
		return fmt.Errorf("failed to parse feature flags: %v", err)
	}

	m.flags = flags
	return nil
}

// saveToFile saves feature flags to a file
func (m *FeatureFlagManager) saveToFile() error {
	data, err := json.MarshalIndent(m.flags, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal feature flags: %v", err)
	}

	err = os.WriteFile(m.configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write feature flags file: %v", err)
	}

	return nil
}
