package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// Test creating a new Auth instance
	config := Config{
		JWTSecret:                "test_secret",
		TokenDuration:            time.Hour,
		RefreshTokenDuration:     24 * time.Hour,
		RequireEmailVerification: false,
		RequireInvitation:        false,
	}

	auth := New(config)
	assert.NotNil(t, auth)
	assert.Equal(t, config, auth.GetConfig())
}

func TestGenerateToken(t *testing.T) {
	// Create a test Auth instance
	config := Config{
		JWTSecret:     "test_secret",
		TokenDuration: time.Hour,
	}
	auth := New(config)

	// Create a test user
	user := User{
		ID:       "test_id",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
	}

	// Generate a token
	token, err := auth.GenerateToken(user)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the token
	claims, err := auth.ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.Username, claims.Username)
	assert.Equal(t, user.Email, claims.Email)
	assert.Equal(t, user.Role, claims.Role)
}

func TestGenerateTokenPair(t *testing.T) {
	// Create a test Auth instance
	config := Config{
		JWTSecret:            "test_secret",
		TokenDuration:        time.Hour,
		RefreshTokenDuration: 24 * time.Hour,
	}
	auth := New(config)

	// Create a test user
	user := User{
		ID:       "test_id",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
	}

	// Generate a token pair
	tokenPair, err := auth.GenerateTokenPair(user)
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.NotEmpty(t, tokenPair.RefreshToken)
	assert.True(t, tokenPair.ExpiresAt.After(time.Now()))
	assert.True(t, tokenPair.ExpiresAt.Before(time.Now().Add(time.Hour+time.Minute)))

	// Validate the access token
	claims, err := auth.ValidateToken(tokenPair.AccessToken)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
}

func TestValidateToken(t *testing.T) {
	// Create a test Auth instance
	config := Config{
		JWTSecret:     "test_secret",
		TokenDuration: time.Hour,
	}
	auth := New(config)

	// Create a test user
	user := User{
		ID:       "test_id",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
	}

	// Generate a token
	token, err := auth.GenerateToken(user)
	assert.NoError(t, err)

	// Test cases
	testCases := []struct {
		name          string
		token         string
		expectedError bool
	}{
		{
			name:          "Valid token",
			token:         token,
			expectedError: false,
		},
		{
			name:          "Empty token",
			token:         "",
			expectedError: true,
		},
		{
			name:          "Invalid token",
			token:         "invalid.token.string",
			expectedError: true,
		},
		{
			name:          "Malformed token",
			token:         "malformedtoken",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			claims, err := auth.ValidateToken(tc.token)
			if tc.expectedError {
				assert.Error(t, err)
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, user.ID, claims.UserID)
			}
		})
	}
}

func TestExpiredToken(t *testing.T) {
	// Create a test Auth instance with a very short token duration
	config := Config{
		JWTSecret:     "test_secret",
		TokenDuration: time.Millisecond, // Very short duration
	}
	auth := New(config)

	// Create a test user
	user := User{
		ID:       "test_id",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
	}

	// Generate a token
	token, err := auth.GenerateToken(user)
	assert.NoError(t, err)

	// Wait for the token to expire
	time.Sleep(10 * time.Millisecond)

	// Validate the token
	claims, err := auth.ValidateToken(token)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "token is expired")
}

func TestAuthMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test Auth instance
	config := Config{
		JWTSecret:     "test_secret",
		TokenDuration: time.Hour,
	}
	auth := New(config)

	// Create a test user
	user := User{
		ID:       "test_id",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
	}

	// Generate a token
	token, err := auth.GenerateToken(user)
	assert.NoError(t, err)

	// Create a test router with the auth middleware
	router := gin.New()
	router.Use(auth.AuthMiddleware())
	router.GET("/protected", func(c *gin.Context) {
		userID, exists := GetUserID(c)
		assert.True(t, exists)
		assert.Equal(t, user.ID, userID)

		username, exists := GetUsername(c)
		assert.True(t, exists)
		assert.Equal(t, user.Username, username)

		email, exists := GetUserEmail(c)
		assert.True(t, exists)
		assert.Equal(t, user.Email, email)

		role, exists := GetUserRole(c)
		assert.True(t, exists)
		assert.Equal(t, user.Role, role)

		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// Test cases
	testCases := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "Valid token",
			authHeader:     "Bearer " + token,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "No token",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid token format",
			authHeader:     "Bearer invalid.token.string",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Wrong auth format",
			authHeader:     "Basic " + token,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Token without Bearer prefix",
			authHeader:     token,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test request
			req := httptest.NewRequest("GET", "/protected", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			// Create a response recorder
			w := httptest.NewRecorder()

			// Perform the request
			router.ServeHTTP(w, req)

			// Check the response
			assert.Equal(t, tc.expectedStatus, w.Code)
		})
	}
}

func TestOptionalAuthMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test Auth instance
	config := Config{
		JWTSecret:     "test_secret",
		TokenDuration: time.Hour,
	}
	auth := New(config)

	// Create a test user
	user := User{
		ID:       "test_id",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
	}

	// Generate a token
	token, err := auth.GenerateToken(user)
	assert.NoError(t, err)

	// Create a test router with the optional auth middleware
	router := gin.New()
	router.Use(auth.OptionalAuthMiddleware())
	router.GET("/optional", func(c *gin.Context) {
		userID, exists := GetUserID(c)
		if exists {
			assert.Equal(t, user.ID, userID)
		}
		c.JSON(http.StatusOK, gin.H{"authenticated": exists})
	})

	// Test cases
	testCases := []struct {
		name               string
		authHeader         string
		expectedStatus     int
		expectedAuthStatus bool
	}{
		{
			name:               "Valid token",
			authHeader:         "Bearer " + token,
			expectedStatus:     http.StatusOK,
			expectedAuthStatus: true,
		},
		{
			name:               "No token",
			authHeader:         "",
			expectedStatus:     http.StatusOK,
			expectedAuthStatus: false,
		},
		{
			name:               "Invalid token",
			authHeader:         "Bearer invalid.token.string",
			expectedStatus:     http.StatusOK,
			expectedAuthStatus: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test request
			req := httptest.NewRequest("GET", "/optional", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			// Create a response recorder
			w := httptest.NewRecorder()

			// Perform the request
			router.ServeHTTP(w, req)

			// Check the response
			assert.Equal(t, tc.expectedStatus, w.Code)
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedAuthStatus, response["authenticated"])
		})
	}
}

func TestRequireRole(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test Auth instance
	config := Config{
		JWTSecret:     "test_secret",
		TokenDuration: time.Hour,
	}
	auth := New(config)

	// Create test users with different roles
	adminUser := User{
		ID:       "admin_id",
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "admin",
	}

	moderatorUser := User{
		ID:       "mod_id",
		Username: "moderator",
		Email:    "mod@example.com",
		Role:     "moderator",
	}

	regularUser := User{
		ID:       "user_id",
		Username: "user",
		Email:    "user@example.com",
		Role:     "user",
	}

	// Generate tokens
	adminToken, _ := auth.GenerateToken(adminUser)
	modToken, _ := auth.GenerateToken(moderatorUser)
	userToken, _ := auth.GenerateToken(regularUser)

	// Create a test router with role-based middleware
	router := gin.New()

	// Admin-only route
	adminGroup := router.Group("/admin")
	adminGroup.Use(auth.AuthMiddleware(), auth.RequireRole("admin"))
	adminGroup.GET("", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "admin access granted"})
	})

	// Moderator or admin route
	modGroup := router.Group("/moderator")
	modGroup.Use(auth.AuthMiddleware(), auth.RequireRole("moderator"))
	modGroup.GET("", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "moderator access granted"})
	})

	// Test cases
	testCases := []struct {
		name           string
		path           string
		token          string
		expectedStatus int
	}{
		{
			name:           "Admin accessing admin route",
			path:           "/admin",
			token:          adminToken,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Moderator accessing admin route",
			path:           "/admin",
			token:          modToken,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "User accessing admin route",
			path:           "/admin",
			token:          userToken,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Admin accessing moderator route",
			path:           "/moderator",
			token:          adminToken,
			expectedStatus: http.StatusOK, // Admin can access moderator routes
		},
		{
			name:           "Moderator accessing moderator route",
			path:           "/moderator",
			token:          modToken,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "User accessing moderator route",
			path:           "/moderator",
			token:          userToken,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test request
			req := httptest.NewRequest("GET", tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)

			// Create a response recorder
			w := httptest.NewRecorder()

			// Perform the request
			router.ServeHTTP(w, req)

			// Check the response
			assert.Equal(t, tc.expectedStatus, w.Code)
		})
	}
}

func TestCreateToken(t *testing.T) {
	// Create a test Auth instance
	config := Config{
		JWTSecret:     "test_secret",
		TokenDuration: time.Hour,
	}
	auth := New(config)

	// Create a test user
	user := User{
		ID:       "test_id",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
	}

	// We'll just use the standard token generation since createToken is private

	// Generate the token
	token, err := auth.GenerateToken(user)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the token
	parsedClaims, err := auth.ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, parsedClaims.UserID)
	assert.Equal(t, user.Username, parsedClaims.Username)
	assert.Equal(t, user.Email, parsedClaims.Email)
	assert.Equal(t, user.Role, parsedClaims.Role)
	assert.Equal(t, "convinceme", parsedClaims.Issuer)
}
