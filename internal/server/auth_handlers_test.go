package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/neo/convinceme_backend/internal/auth"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestServer creates a test server with a temporary database
func setupTestServer(t *testing.T) (*Server, string) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "auth_handlers_test")
	require.NoError(t, err)

	// Create a mock database
	db := &TestMockDB{}

	// Create a test config
	config := &Config{
		JWTSecret:                "test_secret",
		RequireEmailVerification: false,
		RequireInvitation:        false,
	}

	// Create a test auth handler
	authHandler := auth.New(auth.Config{
		JWTSecret:                config.JWTSecret,
		TokenDuration:            time.Hour,
		RefreshTokenDuration:     24 * time.Hour,
		RequireEmailVerification: config.RequireEmailVerification,
		RequireInvitation:        config.RequireInvitation,
	})

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test router
	router := gin.New()

	// Create a test server
	server := &Server{
		router:       router,
		db:           db,
		auth:         authHandler,
		config:       config,
		featureFlags: createTestFeatureFlags(),
	}

	// Set up routes
	server.setupAuthRoutes()
	server.setupInvitationRoutes()
	server.setupFeatureFlagRoutes()
	server.setupFeedbackRoutes()

	return server, tempDir
}

// createTestFeatureFlags creates a test feature flag manager
func createTestFeatureFlags() *FeatureFlagManager {
	return &FeatureFlagManager{
		configPath: "",
		flags: FeatureFlags{
			RequireEmailVerification: false,
			RequireInvitation:        false,
			AllowPasswordReset:       true,
			AllowSocialLogin:         false,
			EnableRateLimiting:       false,
			EnableCSRFProtection:     false,
			EnableFeedbackCollection: true,
			EnableAnalytics:          false,
			EnableAdminDashboard:     true,
		},
	}
}

// teardownTestServer cleans up the test server
func teardownTestServer(tempDir string) {
	os.RemoveAll(tempDir)
}

func TestRegisterHandler(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Test cases
	testCases := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "Valid registration",
			requestBody: map[string]interface{}{
				"username": "testuser",
				"email":    "test@example.com",
				"password": "Password123!",
			},
			expectedStatus: http.StatusCreated,
			expectedError:  false,
		},
		{
			name: "Missing username",
			requestBody: map[string]interface{}{
				"email":    "test@example.com",
				"password": "Password123!",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name: "Missing email",
			requestBody: map[string]interface{}{
				"username": "testuser2",
				"password": "Password123!",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name: "Missing password",
			requestBody: map[string]interface{}{
				"username": "testuser3",
				"email":    "test3@example.com",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name: "Invalid email",
			requestBody: map[string]interface{}{
				"username": "testuser4",
				"email":    "invalid-email",
				"password": "Password123!",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name: "Weak password",
			requestBody: map[string]interface{}{
				"username": "testuser5",
				"email":    "test5@example.com",
				"password": "password",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name: "Username too short",
			requestBody: map[string]interface{}{
				"username": "te",
				"email":    "test6@example.com",
				"password": "Password123!",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request body
			jsonBody, err := json.Marshal(tc.requestBody)
			require.NoError(t, err)

			// Create request
			req, err := http.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			server.router.ServeHTTP(w, req)

			// Check response
			assert.Equal(t, tc.expectedStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tc.expectedError {
				assert.Contains(t, response, "error")
			} else {
				assert.Contains(t, response, "message")
				assert.Contains(t, response, "user")
				assert.Contains(t, response, "access_token")
				assert.Contains(t, response, "refresh_token")
			}
		})
	}

	// Test duplicate username
	t.Run("Duplicate username", func(t *testing.T) {
		// Create first user
		jsonBody, _ := json.Marshal(map[string]interface{}{
			"username": "duplicate",
			"email":    "duplicate1@example.com",
			"password": "Password123!",
		})
		req, _ := http.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// Try to create second user with same username
		jsonBody, _ = json.Marshal(map[string]interface{}{
			"username": "duplicate",
			"email":    "duplicate2@example.com",
			"password": "Password123!",
		})
		req, _ = http.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	// Test duplicate email
	t.Run("Duplicate email", func(t *testing.T) {
		// Create first user
		jsonBody, _ := json.Marshal(map[string]interface{}{
			"username": "email1",
			"email":    "same@example.com",
			"password": "Password123!",
		})
		req, _ := http.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// Try to create second user with same email
		jsonBody, _ = json.Marshal(map[string]interface{}{
			"username": "email2",
			"email":    "same@example.com",
			"password": "Password123!",
		})
		req, _ = http.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusConflict, w.Code)
	})
}

func TestLoginHandler(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Create a test user
	username := "logintest"
	email := "login@example.com"
	password := "Password123!"
	user := &database.User{
		ID:            uuid.New().String(),
		Username:      username,
		Email:         email,
		Role:          database.RoleUser,
		EmailVerified: true,
	}
	err := server.db.CreateUser(user, password)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "Valid login",
			requestBody: map[string]interface{}{
				"username": username,
				"password": password,
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name: "Invalid username",
			requestBody: map[string]interface{}{
				"username": "nonexistent",
				"password": password,
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
		{
			name: "Invalid password",
			requestBody: map[string]interface{}{
				"username": username,
				"password": "wrongpassword",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
		{
			name: "Missing username",
			requestBody: map[string]interface{}{
				"password": password,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name: "Missing password",
			requestBody: map[string]interface{}{
				"username": username,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request body
			jsonBody, err := json.Marshal(tc.requestBody)
			require.NoError(t, err)

			// Create request
			req, err := http.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			server.router.ServeHTTP(w, req)

			// Check response
			assert.Equal(t, tc.expectedStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tc.expectedError {
				assert.Contains(t, response, "error")
			} else {
				assert.Contains(t, response, "message")
				assert.Contains(t, response, "user")
				assert.Contains(t, response, "access_token")
				assert.Contains(t, response, "refresh_token")
			}
		})
	}
}

func TestRefreshTokenHandler(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Create a test user
	user := &database.User{
		ID:            uuid.New().String(),
		Username:      "refreshtest",
		Email:         "refresh@example.com",
		Role:          database.RoleUser,
		EmailVerified: true,
	}
	err := server.db.CreateUser(user, "Password123!")
	require.NoError(t, err)

	// Generate a token pair
	authUser := auth.User{
		ID:            user.ID,
		Username:      user.Username,
		Email:         user.Email,
		Role:          string(user.Role),
		EmailVerified: user.EmailVerified,
	}
	tokenPair, err := server.auth.GenerateTokenPair(authUser)
	require.NoError(t, err)

	// Store the refresh token
	err = server.db.CreateRefreshToken(user.ID, tokenPair.RefreshToken, tokenPair.ExpiresAt)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "Valid refresh token",
			requestBody: map[string]interface{}{
				"refresh_token": tokenPair.RefreshToken,
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name: "Invalid refresh token",
			requestBody: map[string]interface{}{
				"refresh_token": "invalid-token",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
		{
			name:           "Missing refresh token",
			requestBody:    map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request body
			jsonBody, err := json.Marshal(tc.requestBody)
			require.NoError(t, err)

			// Create request
			req, err := http.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			server.router.ServeHTTP(w, req)

			// Check response
			assert.Equal(t, tc.expectedStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tc.expectedError {
				assert.Contains(t, response, "error")
			} else {
				assert.Contains(t, response, "message")
				assert.Contains(t, response, "access_token")
				assert.Contains(t, response, "refresh_token")
				assert.Contains(t, response, "expires_at")
			}
		})
	}
}

func TestForgotPasswordHandler(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Create a test user
	user := &database.User{
		ID:            uuid.New().String(),
		Username:      "forgottest",
		Email:         "forgot@example.com",
		Role:          database.RoleUser,
		EmailVerified: true,
	}
	err := server.db.CreateUser(user, "Password123!")
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
	}{
		{
			name: "Valid email",
			requestBody: map[string]interface{}{
				"email": user.Email,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Non-existent email",
			requestBody: map[string]interface{}{
				"email": "nonexistent@example.com",
			},
			expectedStatus: http.StatusOK, // Should still return OK for security
		},
		{
			name: "Invalid email format",
			requestBody: map[string]interface{}{
				"email": "invalid-email",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing email",
			requestBody:    map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request body
			jsonBody, err := json.Marshal(tc.requestBody)
			require.NoError(t, err)

			// Create request
			req, err := http.NewRequest("POST", "/api/auth/forgot-password", bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			server.router.ServeHTTP(w, req)

			// Check response
			assert.Equal(t, tc.expectedStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Should always contain a message
			assert.Contains(t, response, "message")

			// In development mode, should contain reset_token for valid emails
			if tc.expectedStatus == http.StatusOK && tc.requestBody["email"] == user.Email {
				// Set APP_ENV to development for this test
				oldEnv := os.Getenv("APP_ENV")
				os.Setenv("APP_ENV", "development")
				defer os.Setenv("APP_ENV", oldEnv)

				// Perform request again
				w = httptest.NewRecorder()
				server.router.ServeHTTP(w, req)

				// Parse response
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				// Should contain reset_token
				assert.Contains(t, response, "reset_token")
			}
		})
	}
}

func TestResetPasswordHandler(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Create a test user
	user := &database.User{
		ID:            uuid.New().String(),
		Username:      "resettest",
		Email:         "reset@example.com",
		Role:          database.RoleUser,
		EmailVerified: true,
	}
	err := server.db.CreateUser(user, "OldPassword123!")
	require.NoError(t, err)

	// Create a reset token
	resetToken, err := server.db.CreatePasswordResetToken(user.Email)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
	}{
		{
			name: "Valid reset",
			requestBody: map[string]interface{}{
				"token":        resetToken,
				"new_password": "NewPassword123!",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Invalid token",
			requestBody: map[string]interface{}{
				"token":        "invalid-token",
				"new_password": "NewPassword123!",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Weak password",
			requestBody: map[string]interface{}{
				"token":        resetToken,
				"new_password": "password",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Missing token",
			requestBody: map[string]interface{}{
				"new_password": "NewPassword123!",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Missing password",
			requestBody: map[string]interface{}{
				"token": resetToken,
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip the first test case for subsequent runs
			if i > 0 && tc.name == "Valid reset" {
				// Create a new reset token for each test
				resetToken, err = server.db.CreatePasswordResetToken(user.Email)
				require.NoError(t, err)
				tc.requestBody["token"] = resetToken
			}

			// Create request body
			jsonBody, err := json.Marshal(tc.requestBody)
			require.NoError(t, err)

			// Create request
			req, err := http.NewRequest("POST", "/api/auth/reset-password", bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			server.router.ServeHTTP(w, req)

			// Check response
			assert.Equal(t, tc.expectedStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tc.expectedStatus == http.StatusOK {
				assert.Contains(t, response, "message")
			} else {
				assert.Contains(t, response, "error")
			}
		})
	}
}

func TestProtectedRoutes(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Create test users with different roles
	adminUser := &database.User{
		ID:            uuid.New().String(),
		Username:      "admin",
		Email:         "admin@example.com",
		Role:          database.RoleAdmin,
		EmailVerified: true,
	}
	err := server.db.CreateUser(adminUser, "Password123!")
	require.NoError(t, err)

	regularUser := &database.User{
		ID:            uuid.New().String(),
		Username:      "user",
		Email:         "user@example.com",
		Role:          database.RoleUser,
		EmailVerified: true,
	}
	err = server.db.CreateUser(regularUser, "Password123!")
	require.NoError(t, err)

	// Generate tokens
	adminAuthUser := auth.User{
		ID:            adminUser.ID,
		Username:      adminUser.Username,
		Email:         adminUser.Email,
		Role:          string(adminUser.Role),
		EmailVerified: adminUser.EmailVerified,
	}
	adminToken, err := server.auth.GenerateToken(adminAuthUser)
	require.NoError(t, err)

	userAuthUser := auth.User{
		ID:            regularUser.ID,
		Username:      regularUser.Username,
		Email:         regularUser.Email,
		Role:          string(regularUser.Role),
		EmailVerified: regularUser.EmailVerified,
	}
	userToken, err := server.auth.GenerateToken(userAuthUser)
	require.NoError(t, err)

	// Test cases for protected routes
	protectedRoutes := []struct {
		name           string
		method         string
		path           string
		adminOnly      bool
		expectedStatus map[string]int // token type -> expected status
	}{
		{
			name:      "Get current user",
			method:    "GET",
			path:      "/api/auth/me",
			adminOnly: false,
			expectedStatus: map[string]int{
				"admin":   http.StatusOK,
				"user":    http.StatusOK,
				"none":    http.StatusUnauthorized,
				"invalid": http.StatusUnauthorized,
			},
		},
		{
			name:      "Update current user",
			method:    "PUT",
			path:      "/api/auth/me",
			adminOnly: false,
			expectedStatus: map[string]int{
				"admin":   http.StatusOK,
				"user":    http.StatusOK,
				"none":    http.StatusUnauthorized,
				"invalid": http.StatusUnauthorized,
			},
		},
		{
			name:      "Change password",
			method:    "POST",
			path:      "/api/auth/change-password",
			adminOnly: false,
			expectedStatus: map[string]int{
				"admin":   http.StatusOK,
				"user":    http.StatusOK,
				"none":    http.StatusUnauthorized,
				"invalid": http.StatusUnauthorized,
			},
		},
		{
			name:      "Delete current user",
			method:    "DELETE",
			path:      "/api/auth/me",
			adminOnly: false,
			expectedStatus: map[string]int{
				"admin":   http.StatusOK,
				"user":    http.StatusOK,
				"none":    http.StatusUnauthorized,
				"invalid": http.StatusUnauthorized,
			},
		},
		{
			name:      "Create invitation",
			method:    "POST",
			path:      "/api/invitations",
			adminOnly: false,
			expectedStatus: map[string]int{
				"admin":   http.StatusCreated,
				"user":    http.StatusCreated,
				"none":    http.StatusUnauthorized,
				"invalid": http.StatusUnauthorized,
			},
		},
		{
			name:      "List invitations",
			method:    "GET",
			path:      "/api/invitations",
			adminOnly: false,
			expectedStatus: map[string]int{
				"admin":   http.StatusOK,
				"user":    http.StatusOK,
				"none":    http.StatusUnauthorized,
				"invalid": http.StatusUnauthorized,
			},
		},
		{
			name:      "Update feature flags",
			method:    "PUT",
			path:      "/api/features",
			adminOnly: true,
			expectedStatus: map[string]int{
				"admin":   http.StatusOK,
				"user":    http.StatusForbidden,
				"none":    http.StatusUnauthorized,
				"invalid": http.StatusUnauthorized,
			},
		},
	}

	for _, route := range protectedRoutes {
		t.Run(route.name, func(t *testing.T) {
			// Test with different token types
			tokenTypes := map[string]string{
				"admin":   adminToken,
				"user":    userToken,
				"none":    "",
				"invalid": "invalid.token.string",
			}

			for tokenType, token := range tokenTypes {
				// Create request
				var req *http.Request
				var err error

				if route.method == "GET" {
					req, err = http.NewRequest(route.method, route.path, nil)
				} else {
					// For non-GET requests, add a simple JSON body
					jsonBody, _ := json.Marshal(map[string]interface{}{
						"email": "test@example.com",
					})
					req, err = http.NewRequest(route.method, route.path, bytes.NewBuffer(jsonBody))
					req.Header.Set("Content-Type", "application/json")
				}
				require.NoError(t, err)

				// Add authorization header if token is provided
				if token != "" {
					req.Header.Set("Authorization", "Bearer "+token)
				}

				// Create response recorder
				w := httptest.NewRecorder()

				// Perform request
				server.router.ServeHTTP(w, req)

				// Check response
				expectedStatus := route.expectedStatus[tokenType]
				assert.Equal(t, expectedStatus, w.Code, "Failed for token type: %s", tokenType)
			}
		})
	}
}
