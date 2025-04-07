package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// User represents a user in the system
type User struct {
	ID            string     `json:"id"`
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	LastLogin     *time.Time `json:"last_login,omitempty"`
	AccountLocked bool       `json:"account_locked"`
	EmailVerified bool       `json:"email_verified"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Claims represents the JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Config contains authentication configuration
type Config struct {
	JWTSecret                string
	TokenDuration            time.Duration
	RefreshTokenDuration     time.Duration
	RequireEmailVerification bool
	RequireInvitation        bool
}

// GetConfig returns the authentication configuration
func (a *Auth) GetConfig() Config {
	return a.config
}

// Auth handles authentication
type Auth struct {
	config Config
}

// New creates a new Auth instance
func New(config Config) *Auth {
	return &Auth{
		config: config,
	}
}

// TokenPair contains an access token and a refresh token
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// GenerateToken generates a JWT token for a user
func (a *Auth) GenerateToken(user User) (string, error) {
	// Create claims with user information
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.config.TokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "convinceme",
			Subject:   user.ID,
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(a.config.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	return tokenString, nil
}

// GenerateTokenPair generates both an access token and a refresh token
func (a *Auth) GenerateTokenPair(user User) (*TokenPair, error) {
	// Generate access token
	accessToken, err := a.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	// Generate refresh token (a secure random string)
	refreshToken := make([]byte, 32)
	_, err = rand.Read(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %v", err)
	}

	refreshTokenString := base64.URLEncoding.EncodeToString(refreshToken)

	// Calculate expiration time
	expiresAt := time.Now().Add(a.config.TokenDuration)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString,
		ExpiresAt:    expiresAt,
	}, nil
}

// ValidateToken validates a JWT token
func (a *Auth) ValidateToken(tokenString string) (*Claims, error) {
	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(a.config.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	// Check if the token is valid
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("failed to extract claims")
	}

	return claims, nil
}

// GenerateRandomKey generates a random key for JWT signing
func GenerateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// AuthMiddleware returns a middleware that checks for a valid JWT token
func (a *Auth) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// Check if the Authorization header has the correct format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			c.Abort()
			return
		}

		// Validate the token
		tokenString := parts[1]
		claims, err := a.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Invalid token: %v", err)})
			c.Abort()
			return
		}

		// Set user information in the context
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// OptionalAuthMiddleware returns a middleware that checks for a valid JWT token but doesn't require it
func (a *Auth) OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No token provided, continue without user info
			c.Next()
			return
		}

		// Check if the Authorization header has the correct format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			// Invalid format, continue without user info
			c.Next()
			return
		}

		// Validate the token
		tokenString := parts[1]
		claims, err := a.ValidateToken(tokenString)
		if err != nil {
			// Invalid token, continue without user info
			c.Next()
			return
		}

		// Set user information in the context
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// GetUserID gets the user ID from the context
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get("userID")
	if !exists {
		return "", false
	}
	return userID.(string), true
}

// GetUsername gets the username from the context
func GetUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	return username.(string), true
}

// GetUserRole gets the user role from the context
func GetUserRole(c *gin.Context) (string, bool) {
	role, exists := c.Get("role")
	if !exists {
		return "", false
	}
	return role.(string), true
}

// GetUserEmail gets the user email from the context
func GetUserEmail(c *gin.Context) (string, bool) {
	email, exists := c.Get("email")
	if !exists {
		return "", false
	}
	return email.(string), true
}

// RequireRole returns a middleware that requires a specific role
func (a *Auth) RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the user role from the context (set by AuthMiddleware)
		userRole, exists := GetUserRole(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required"})
			c.Abort()
			return
		}

		// Check if the user has the required role
		if userRole != role && userRole != "admin" { // Admins can access everything
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}
