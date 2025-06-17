package server

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/neo/convinceme_backend/internal/auth"
	"github.com/neo/convinceme_backend/internal/database"
)

// refreshTokenHandler handles token refresh
func (s *Server) refreshTokenHandler(c *gin.Context) {
	// Parse request
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Verify refresh token
	refreshToken, err := s.db.GetRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	// Get the user
	user, err := s.db.GetUserByID(refreshToken.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Check if user is locked or email is not verified
	if user.AccountLocked {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is locked"})
		return
	}

	config := s.auth.GetConfig()
	if config.RequireEmailVerification && !user.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "Email address has not been verified"})
		return
	}

	// Generate new token pair
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

	// Delete old refresh token
	err = s.db.DeleteRefreshToken(req.RefreshToken)
	if err != nil {
		// Log the error but continue
		fmt.Printf("Failed to delete old refresh token: %v\n", err)
	}

	// Store new refresh token
	err = s.db.CreateRefreshToken(user.ID, tokenPair.RefreshToken, tokenPair.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to store refresh token: %v", err)})
		return
	}

	// Return new tokens
	c.JSON(http.StatusOK, gin.H{
		"message":       "Token refreshed successfully",
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_at":    tokenPair.ExpiresAt,
	})
}

// verifyEmailHandler verifies a user's email address
func (s *Server) verifyEmailHandler(c *gin.Context) {
	// Get token from query parameter
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification token is required"})
		return
	}

	// Verify the token
	err := s.db.VerifyEmail(token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to verify email: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully"})
}

// forgotPasswordHandler initiates the password reset process
func (s *Server) forgotPasswordHandler(c *gin.Context) {
	// Parse request
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Generate reset token
	resetToken, err := s.db.CreatePasswordResetToken(req.Email)
	if err != nil {
		// Don't reveal if the email exists or not
		c.JSON(http.StatusOK, gin.H{"message": "If your email is registered, you will receive a password reset link"})
		return
	}

	// TODO: Send password reset email
	
	// In development mode, return the token
	response := gin.H{"message": "If your email is registered, you will receive a password reset link"}
	if os.Getenv("APP_ENV") == "development" {
		response["reset_token"] = resetToken
	}

	c.JSON(http.StatusOK, response)
}

// resetPasswordHandler resets a user's password
func (s *Server) resetPasswordHandler(c *gin.Context) {
	// Parse request
	var req struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Validate password strength
	if !isStrongPassword(req.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Password is too weak",
			"details": "Password must contain at least one uppercase letter, one lowercase letter, one number, and one special character",
		})
		return
	}

	// Reset the password
	err := s.db.ResetPassword(req.Token, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to reset password: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

// resendVerificationHandler resends the verification email
func (s *Server) resendVerificationHandler(c *gin.Context) {
	// Parse request
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Resend verification email
	verificationToken, err := s.db.ResendVerificationEmail(req.Email)
	if err != nil {
		// Don't reveal if the email exists or not
		c.JSON(http.StatusOK, gin.H{"message": "If your email is registered and not verified, you will receive a verification email"})
		return
	}

	// TODO: Send verification email
	
	// In development mode, return the token
	response := gin.H{"message": "If your email is registered and not verified, you will receive a verification email"}
	if os.Getenv("APP_ENV") == "development" {
		response["verification_token"] = verificationToken
	}

	c.JSON(http.StatusOK, response)
}

// setupAuthRoutes sets up the authentication routes
func (s *Server) setupAuthRoutes() {
	authGroup := s.router.Group("/api/auth")
	{
		// Public routes
		authGroup.POST("/register", s.registerHandler)
		authGroup.POST("/login", s.loginHandler)
		authGroup.POST("/refresh", s.refreshTokenHandler)
		authGroup.POST("/forgot-password", s.forgotPasswordHandler)
		authGroup.POST("/reset-password", s.resetPasswordHandler)
		authGroup.GET("/verify-email", s.verifyEmailHandler)
		authGroup.POST("/resend-verification", s.resendVerificationHandler)
		
		// Protected routes
		authGroup.Use(s.auth.AuthMiddleware())
		authGroup.GET("/me", s.meHandler)
		authGroup.PUT("/me", s.updateUserHandler)
		authGroup.POST("/change-password", s.changePasswordHandler)
		authGroup.DELETE("/me", s.deleteUserHandler)
		
		// Admin routes
		adminGroup := authGroup.Group("/admin")
		adminGroup.Use(s.auth.RequireRole(string(database.RoleAdmin)))
		// TODO: Add admin routes
	}
}
