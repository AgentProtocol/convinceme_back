package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/neo/convinceme_backend/internal/auth"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitFeedbackHandler(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Create a test user
	user := &database.User{
		ID:            uuid.New().String(),
		Username:      "feedbackuser",
		Email:         "feedback@example.com",
		Role:          database.RoleUser,
		EmailVerified: true,
	}
	err := server.db.CreateUser(user, "Password123!")
	require.NoError(t, err)

	// Generate a token
	authUser := auth.User{
		ID:            user.ID,
		Username:      user.Username,
		Email:         user.Email,
		Role:          string(user.Role),
		EmailVerified: user.EmailVerified,
	}
	token, err := server.auth.GenerateToken(authUser)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name           string
		requestBody    map[string]interface{}
		authenticated  bool
		expectedStatus int
	}{
		{
			name: "Valid feedback (authenticated)",
			requestBody: map[string]interface{}{
				"type":    "auth",
				"message": "The login process is great!",
				"rating":  5,
			},
			authenticated:  true,
			expectedStatus: http.StatusCreated,
		},
		{
			name: "Valid feedback (unauthenticated)",
			requestBody: map[string]interface{}{
				"type":    "auth",
				"message": "I had trouble logging in",
				"rating":  2,
			},
			authenticated:  false,
			expectedStatus: http.StatusCreated,
		},
		{
			name: "Missing type",
			requestBody: map[string]interface{}{
				"message": "Feedback without type",
				"rating":  3,
			},
			authenticated:  false,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Missing message",
			requestBody: map[string]interface{}{
				"type":   "auth",
				"rating": 4,
			},
			authenticated:  false,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "With browser info",
			requestBody: map[string]interface{}{
				"type":        "auth",
				"message":     "Feedback with browser info",
				"rating":      5,
				"browser":     "Chrome",
				"device":      "Desktop",
				"screen_size": "1920x1080",
			},
			authenticated:  true,
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request body
			jsonBody, err := json.Marshal(tc.requestBody)
			require.NoError(t, err)

			// Create request
			req, err := http.NewRequest("POST", "/api/feedback", bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			// Add authorization if authenticated
			if tc.authenticated {
				req.Header.Set("Authorization", "Bearer "+token)
			}

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

			if tc.expectedStatus == http.StatusCreated {
				assert.Contains(t, response, "message")
				assert.Contains(t, response, "feedback")

				feedback := response["feedback"].(map[string]interface{})
				assert.Equal(t, tc.requestBody["type"], feedback["type"])
				assert.Equal(t, tc.requestBody["message"], feedback["message"])

				if tc.authenticated {
					assert.Equal(t, user.ID, feedback["user_id"])
				} else {
					assert.Empty(t, feedback["user_id"])
				}

				if rating, ok := tc.requestBody["rating"]; ok {
					assert.Equal(t, float64(rating.(int)), feedback["rating"])
				}

				if browser, ok := tc.requestBody["browser"]; ok {
					assert.Equal(t, browser, feedback["browser"])
				}

				if device, ok := tc.requestBody["device"]; ok {
					assert.Equal(t, device, feedback["device"])
				}

				if screenSize, ok := tc.requestBody["screen_size"]; ok {
					assert.Equal(t, screenSize, feedback["screen_size"])
				}
			} else {
				assert.Contains(t, response, "error")
			}
		})
	}
}

func TestGetFeedbackHandler(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Create an admin user
	adminUser := &database.User{
		ID:            uuid.New().String(),
		Username:      "adminuser",
		Email:         "admin@example.com",
		Role:          database.RoleAdmin,
		EmailVerified: true,
	}
	err := server.db.CreateUser(adminUser, "Password123!")
	require.NoError(t, err)

	// Create a regular user
	regularUser := &database.User{
		ID:            uuid.New().String(),
		Username:      "regularuser",
		Email:         "regular@example.com",
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

	regularAuthUser := auth.User{
		ID:            regularUser.ID,
		Username:      regularUser.Username,
		Email:         regularUser.Email,
		Role:          string(regularUser.Role),
		EmailVerified: regularUser.EmailVerified,
	}
	regularToken, err := server.auth.GenerateToken(regularAuthUser)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{
			name:           "Admin user",
			token:          adminToken,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Regular user",
			token:          regularToken,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "No token",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid token",
			token:          "invalid.token.string",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req, err := http.NewRequest("GET", "/api/feedback", nil)
			require.NoError(t, err)

			// Add authorization if token is provided
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}

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
				assert.Contains(t, response, "feedback")
				// Currently returns empty array since we don't have a feedback table yet
				assert.IsType(t, []interface{}{}, response["feedback"])
			} else {
				assert.Contains(t, response, "error")
			}
		})
	}
}
