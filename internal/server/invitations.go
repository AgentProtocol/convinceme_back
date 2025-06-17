package server

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/neo/convinceme_backend/internal/auth"
)

// createInvitationHandler creates a new invitation code
func (s *Server) createInvitationHandler(c *gin.Context) {
	// Get the current user ID
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse request
	var req struct {
		Email string `json:"email" binding:"omitempty,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Create the invitation code (valid for 7 days)
	invitation, err := s.db.CreateInvitationCode(userID, req.Email, 7*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create invitation code: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Invitation code created successfully",
		"invitation": invitation,
	})
}

// listInvitationsHandler lists all invitations created by the current user
func (s *Server) listInvitationsHandler(c *gin.Context) {
	// Get the current user ID
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get invitations
	invitations, err := s.db.GetInvitationsByUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get invitations: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"invitations": invitations,
	})
}

// deleteInvitationHandler deletes an invitation code
func (s *Server) deleteInvitationHandler(c *gin.Context) {
	// Get the current user ID
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get invitation ID from path
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invitation ID"})
		return
	}

	// Delete the invitation
	err = s.db.DeleteInvitationCode(id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete invitation: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Invitation deleted successfully",
	})
}

// validateInvitationHandler validates an invitation code without using it
func (s *Server) validateInvitationHandler(c *gin.Context) {
	// Parse request
	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Validate the invitation code
	invitation, err := s.db.ValidateInvitationCode(req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid invitation code: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Invitation code is valid",
		"invitation": gin.H{
			"code":       invitation.Code,
			"email":      invitation.Email,
			"expires_at": invitation.ExpiresAt,
		},
	})
}

// setupInvitationRoutes sets up the invitation routes
func (s *Server) setupInvitationRoutes() {
	// Group all invitation routes under /api/invitations
	invitationGroup := s.router.Group("/api/invitations")
	{
		// Public routes
		invitationGroup.POST("/validate", s.validateInvitationHandler)

		// Protected routes
		invitationGroup.Use(s.auth.AuthMiddleware())
		invitationGroup.POST("", s.createInvitationHandler)
		invitationGroup.GET("", s.listInvitationsHandler)
		invitationGroup.DELETE("/:id", s.deleteInvitationHandler)
	}
}
