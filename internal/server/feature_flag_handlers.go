package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/neo/convinceme_backend/internal/auth"
	"github.com/neo/convinceme_backend/internal/database"
)

// getFeatureFlagsHandler returns the current feature flags
func (s *Server) getFeatureFlagsHandler(c *gin.Context) {
	flags := s.featureFlags.GetFlags()
	c.JSON(http.StatusOK, gin.H{
		"feature_flags": flags,
	})
}

// updateFeatureFlagsHandler updates the feature flags
func (s *Server) updateFeatureFlagsHandler(c *gin.Context) {
	// Get the current user role
	role, exists := auth.GetUserRole(c)
	if !exists || role != string(database.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can update feature flags"})
		return
	}

	// Parse request
	var req FeatureFlags
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Update flags
	err := s.featureFlags.UpdateFlags(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update feature flags", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Feature flags updated successfully",
		"feature_flags": req,
	})
}

// setupFeatureFlagRoutes sets up the feature flag routes
func (s *Server) setupFeatureFlagRoutes() {
	// Group all feature flag routes under /api/features
	featureGroup := s.router.Group("/api/features")
	{
		// Public route to get feature flags
		featureGroup.GET("", s.getFeatureFlagsHandler)
		
		// Admin route to update feature flags
		featureGroup.Use(s.auth.AuthMiddleware())
		featureGroup.Use(s.auth.RequireRole(string(database.RoleAdmin)))
		featureGroup.PUT("", s.updateFeatureFlagsHandler)
	}
}
