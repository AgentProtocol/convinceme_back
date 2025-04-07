package server

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/neo/convinceme_backend/internal/auth"
	"github.com/neo/convinceme_backend/internal/database"
)

// registerHandler handles user registration
func (s *Server) registerHandler(c *gin.Context) {
	// Parse request
	var req struct {
		Username       string `json:"username" binding:"required,min=3,max=30"`
		Email          string `json:"email" binding:"required,email"`
		Password       string `json:"password" binding:"required,min=8"`
		InvitationCode string `json:"invitation_code" binding:"omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Validate password strength
	if !isStrongPassword(req.Password) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Password is too weak",
			"details": "Password must contain at least one uppercase letter, one lowercase letter, one number, and one special character",
		})
		return
	}

	// Check if registration requires an invitation code
	config := s.auth.GetConfig()
	if config.RequireInvitation && req.InvitationCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation code is required for registration"})
		return
	}

	// Validate invitation code if provided
	var invitation *database.InvitationCode
	if req.InvitationCode != "" {
		var err error
		invitation, err = s.db.ValidateInvitationCode(req.InvitationCode)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid invitation code: %v", err)})
			return
		}

		// If the invitation is for a specific email, check that it matches
		if invitation.Email != "" && invitation.Email != req.Email {
			c.JSON(http.StatusBadRequest, gin.H{"error": "This invitation code is for a different email address"})
			return
		}
	}

	// Check if username already exists
	_, err := s.db.GetUserByUsername(req.Username)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	// Check if email already exists
	_, err = s.db.GetUserByEmail(req.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		return
	}

	// Create user
	user := &database.User{
		ID:             uuid.New().String(),
		Username:       req.Username,
		Email:          req.Email,
		Role:           database.RoleUser,
		EmailVerified:  !config.RequireEmailVerification, // Skip verification if not required
		AccountLocked:  false,
		InvitationCode: req.InvitationCode,
	}

	err = s.db.CreateUser(user, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create user: %v", err)})
		return
	}

	// Mark invitation code as used if provided
	if req.InvitationCode != "" {
		err = s.db.UseInvitationCode(req.InvitationCode, user.ID)
		if err != nil {
			// Log the error but continue
			fmt.Printf("Failed to mark invitation code as used: %v\n", err)
		}
	}

	// Generate token pair
	authUser := auth.User{
		ID:            user.ID,
		Username:      user.Username,
		Email:         user.Email,
		Role:          string(user.Role),
		EmailVerified: user.EmailVerified,
		AccountLocked: user.AccountLocked,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}

	tokenPair, err := s.auth.GenerateTokenPair(authUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate token: %v", err)})
		return
	}

	// Store refresh token in database
	err = s.db.CreateRefreshToken(user.ID, tokenPair.RefreshToken, tokenPair.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to store refresh token: %v", err)})
		return
	}

	// If email verification is required, generate a verification token
	var verificationToken string
	if config.RequireEmailVerification && !user.EmailVerified {
		verificationToken, err = s.db.ResendVerificationEmail(user.Email)
		if err != nil {
			// Log the error but continue
			fmt.Printf("Failed to generate verification token: %v\n", err)
		}

		// TODO: Send verification email
	}

	// Return user and token
	response := gin.H{
		"message": "User registered successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_at":    tokenPair.ExpiresAt,
	}

	// Add verification info if needed
	if config.RequireEmailVerification && !user.EmailVerified {
		response["email_verified"] = false
		response["verification_required"] = true
		// Include verification token in development mode only
		if os.Getenv("APP_ENV") == "development" {
			response["verification_token"] = verificationToken
		}
	}

	c.JSON(http.StatusCreated, response)
}

// loginHandler handles user login
func (s *Server) loginHandler(c *gin.Context) {
	// Parse request
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Verify password
	user, err := s.db.VerifyPassword(req.Username, req.Password)
	if err != nil {
		// Check if the error is about account being locked
		if err.Error() == "account is locked due to too many failed login attempts" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is locked due to too many failed login attempts"})
			return
		}

		// Check if the error is about email verification
		if err.Error() == "email address has not been verified" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Email address has not been verified"})
			return
		}

		// Generic error for invalid credentials
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	// Generate token pair
	authUser := auth.User{
		ID:            user.ID,
		Username:      user.Username,
		Email:         user.Email,
		Role:          string(user.Role),
		EmailVerified: user.EmailVerified,
		AccountLocked: user.AccountLocked,
		LastLogin:     user.LastLogin,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}

	tokenPair, err := s.auth.GenerateTokenPair(authUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate token: %v", err)})
		return
	}

	// Store refresh token in database
	err = s.db.CreateRefreshToken(user.ID, tokenPair.RefreshToken, tokenPair.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to store refresh token: %v", err)})
		return
	}

	// Return user and token
	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user": gin.H{
			"id":             user.ID,
			"username":       user.Username,
			"email":          user.Email,
			"role":           user.Role,
			"email_verified": user.EmailVerified,
			"last_login":     user.LastLogin,
		},
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_at":    tokenPair.ExpiresAt,
	})
}

// meHandler returns the current user
func (s *Server) meHandler(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Get user from database
	user, err := s.db.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user: %v", err)})
		return
	}

	// Return user
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// updateUserHandler updates the current user
func (s *Server) updateUserHandler(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Parse request
	var req struct {
		Username string `json:"username" binding:"omitempty,min=3,max=30"`
		Email    string `json:"email" binding:"omitempty,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Get user from database
	user, err := s.db.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user: %v", err)})
		return
	}

	// Update user fields if provided
	if req.Username != "" && req.Username != user.Username {
		// Check if username already exists
		_, err := s.db.GetUserByUsername(req.Username)
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}
		user.Username = req.Username
	}

	if req.Email != "" && req.Email != user.Email {
		// Check if email already exists
		_, err := s.db.GetUserByEmail(req.Email)
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		user.Email = req.Email
	}

	// Update user in database
	err = s.db.UpdateUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update user: %v", err)})
		return
	}

	// Return updated user
	c.JSON(http.StatusOK, gin.H{
		"message": "User updated successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// changePasswordHandler changes the current user's password
func (s *Server) changePasswordHandler(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Parse request
	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Get user from database
	user, err := s.db.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user: %v", err)})
		return
	}

	// Verify current password
	_, err = s.db.VerifyPassword(user.Username, req.CurrentPassword)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid current password"})
		return
	}

	// Update password
	err = s.db.UpdatePassword(userID, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update password: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password changed successfully",
	})
}

// deleteUserHandler deletes the current user
func (s *Server) deleteUserHandler(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	// Delete user
	err := s.db.DeleteUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete user: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
	})
}
