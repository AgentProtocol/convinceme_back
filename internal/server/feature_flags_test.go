package server

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureFlagManager(t *testing.T) {
	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "feature_flags_test.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	// Write default flags to the file
	defaultFlags := FeatureFlags{
		RequireEmailVerification: false,
		RequireInvitation:        true,
		AllowPasswordReset:       true,
		AllowSocialLogin:         false,
		EnableRateLimiting:       true,
		EnableCSRFProtection:     false,
		EnableFeedbackCollection: true,
		EnableAnalytics:          true,
		EnableAdminDashboard:     true,
	}
	data, err := json.Marshal(defaultFlags)
	require.NoError(t, err)
	_, err = tempFile.Write(data)
	require.NoError(t, err)
	tempFile.Close()

	// Create a new feature flag manager
	manager, err := NewFeatureFlagManager(tempFile.Name())
	require.NoError(t, err)
	assert.NotNil(t, manager)

	// Check default values
	flags := manager.GetFlags()
	assert.False(t, flags.RequireEmailVerification)
	assert.True(t, flags.RequireInvitation) // Default to true for closed alpha
	assert.True(t, flags.AllowPasswordReset)
	assert.False(t, flags.AllowSocialLogin)
	assert.True(t, flags.EnableRateLimiting)
	assert.False(t, flags.EnableCSRFProtection)
	assert.True(t, flags.EnableFeedbackCollection)
	assert.True(t, flags.EnableAnalytics)
	assert.True(t, flags.EnableAdminDashboard)

	// Update flags
	updatedFlags := FeatureFlags{
		RequireEmailVerification: true,
		RequireInvitation:        false,
		AllowPasswordReset:       false,
		AllowSocialLogin:         true,
		EnableRateLimiting:       false,
		EnableCSRFProtection:     true,
		EnableFeedbackCollection: false,
		EnableAnalytics:          false,
		EnableAdminDashboard:     false,
	}
	err = manager.UpdateFlags(updatedFlags)
	require.NoError(t, err)

	// Check updated values
	flags = manager.GetFlags()
	assert.True(t, flags.RequireEmailVerification)
	assert.False(t, flags.RequireInvitation)
	assert.False(t, flags.AllowPasswordReset)
	assert.True(t, flags.AllowSocialLogin)
	assert.False(t, flags.EnableRateLimiting)
	assert.True(t, flags.EnableCSRFProtection)
	assert.False(t, flags.EnableFeedbackCollection)
	assert.False(t, flags.EnableAnalytics)
	assert.False(t, flags.EnableAdminDashboard)

	// Check that the file was saved correctly
	fileData, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	var savedFlags FeatureFlags
	err = json.Unmarshal(fileData, &savedFlags)
	require.NoError(t, err)
	assert.Equal(t, updatedFlags, savedFlags)

	// Create a new manager with the same file
	newManager, err := NewFeatureFlagManager(tempFile.Name())
	require.NoError(t, err)
	assert.NotNil(t, newManager)

	// Check that the flags were loaded correctly
	flags = newManager.GetFlags()
	assert.Equal(t, updatedFlags, flags)
}

func TestFeatureFlagManagerWithInvalidFile(t *testing.T) {
	// Create a temporary file with invalid JSON
	tempFile, err := os.CreateTemp("", "invalid_feature_flags_test.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	_, err = tempFile.Write([]byte("invalid json"))
	require.NoError(t, err)
	tempFile.Close()

	// Create a new feature flag manager
	manager, err := NewFeatureFlagManager(tempFile.Name())
	assert.Error(t, err)
	assert.Nil(t, manager)
}

func TestFeatureFlagManagerWithNonExistentFile(t *testing.T) {
	// Create a new feature flag manager with a non-existent file
	manager, err := NewFeatureFlagManager("non_existent_file.json")
	require.NoError(t, err)
	assert.NotNil(t, manager)

	// Check default values
	flags := manager.GetFlags()
	assert.False(t, flags.RequireEmailVerification)
	assert.True(t, flags.RequireInvitation)
	assert.True(t, flags.AllowPasswordReset)
	assert.False(t, flags.AllowSocialLogin)
	assert.True(t, flags.EnableRateLimiting)
	assert.False(t, flags.EnableCSRFProtection)
	assert.True(t, flags.EnableFeedbackCollection)
	assert.True(t, flags.EnableAnalytics)
	assert.True(t, flags.EnableAdminDashboard)

	// Check that the file was created
	_, err = os.Stat("non_existent_file.json")
	require.NoError(t, err)

	// Clean up
	os.Remove("non_existent_file.json")
}
