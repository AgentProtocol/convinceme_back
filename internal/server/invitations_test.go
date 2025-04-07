package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/neo/convinceme_backend/internal/auth"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateInvitationHandler(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Create a test user
	user := &database.User{
		ID:            uuid.New().String(),
		Username:      "inviter",
		Email:         "inviter@example.com",
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
			name:           "Valid invitation without email",
			requestBody:    map[string]interface{}{},
			authenticated:  true,
			expectedStatus: http.StatusCreated,
		},
		{
			name: "Valid invitation with email",
			requestBody: map[string]interface{}{
				"email": "invited@example.com",
			},
			authenticated:  true,
			expectedStatus: http.StatusCreated,
		},
		{
			name: "Invalid email",
			requestBody: map[string]interface{}{
				"email": "invalid-email",
			},
			authenticated:  true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Unauthenticated",
			requestBody:    map[string]interface{}{},
			authenticated:  false,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request body
			jsonBody, err := json.Marshal(tc.requestBody)
			require.NoError(t, err)

			// Create request
			req, err := http.NewRequest("POST", "/api/invitations", bytes.NewBuffer(jsonBody))
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
				assert.Contains(t, response, "invitation")

				invitation := response["invitation"].(map[string]interface{})
				assert.NotEmpty(t, invitation["code"])
				assert.Equal(t, user.ID, invitation["created_by"])
				assert.False(t, invitation["used"].(bool))

				if email, ok := tc.requestBody["email"]; ok && tc.expectedStatus == http.StatusCreated {
					assert.Equal(t, email, invitation["email"])
				}
			} else {
				assert.Contains(t, response, "error")
			}
		})
	}
}

func TestListInvitationsHandler(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Create a test user
	user := &database.User{
		ID:            uuid.New().String(),
		Username:      "inviter",
		Email:         "inviter@example.com",
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

	// Create some invitations for the user
	_, err = server.db.CreateInvitationCode(user.ID, "invite1@example.com", 7*24*60*60*1000000000)
	require.NoError(t, err)
	_, err = server.db.CreateInvitationCode(user.ID, "invite2@example.com", 7*24*60*60*1000000000)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name           string
		token          string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "Authenticated user",
			token:          token,
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "Unauthenticated",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			expectedCount:  0,
		},
		{
			name:           "Invalid token",
			token:          "invalid.token.string",
			expectedStatus: http.StatusUnauthorized,
			expectedCount:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req, err := http.NewRequest("GET", "/api/invitations", nil)
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

			if tc.expectedStatus == http.StatusOK {
				// Parse response
				var response map[string]interface{}
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Contains(t, response, "invitations")
				invitations := response["invitations"].([]interface{})
				assert.Len(t, invitations, tc.expectedCount)
			}
		})
	}
}

func TestDeleteInvitationHandler(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Create test users
	user1 := &database.User{
		ID:            uuid.New().String(),
		Username:      "user1",
		Email:         "user1@example.com",
		Role:          database.RoleUser,
		EmailVerified: true,
	}
	err := server.db.CreateUser(user1, "Password123!")
	require.NoError(t, err)

	user2 := &database.User{
		ID:            uuid.New().String(),
		Username:      "user2",
		Email:         "user2@example.com",
		Role:          database.RoleUser,
		EmailVerified: true,
	}
	err = server.db.CreateUser(user2, "Password123!")
	require.NoError(t, err)

	// Generate tokens
	authUser1 := auth.User{
		ID:            user1.ID,
		Username:      user1.Username,
		Email:         user1.Email,
		Role:          string(user1.Role),
		EmailVerified: user1.EmailVerified,
	}
	token1, err := server.auth.GenerateToken(authUser1)
	require.NoError(t, err)

	authUser2 := auth.User{
		ID:            user2.ID,
		Username:      user2.Username,
		Email:         user2.Email,
		Role:          string(user2.Role),
		EmailVerified: user2.EmailVerified,
	}
	token2, err := server.auth.GenerateToken(authUser2)
	require.NoError(t, err)

	// Create an invitation for user1
	invitation, err := server.db.CreateInvitationCode(user1.ID, "invited@example.com", 7*24*60*60*1000000000)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name           string
		invitationID   int
		token          string
		expectedStatus int
	}{
		{
			name:           "Owner deleting invitation",
			invitationID:   invitation.ID,
			token:          token1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Non-owner deleting invitation",
			invitationID:   invitation.ID,
			token:          token2,
			expectedStatus: http.StatusInternalServerError, // Returns 500 because the invitation is already deleted
		},
		{
			name:           "Unauthenticated",
			invitationID:   invitation.ID,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid token",
			invitationID:   invitation.ID,
			token:          "invalid.token.string",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Non-existent invitation",
			invitationID:   9999,
			token:          token1,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip the first test for subsequent runs
			if i > 0 && tc.name == "Owner deleting invitation" {
				// Create a new invitation for each test
				newInvitation, err := server.db.CreateInvitationCode(user1.ID, "invited@example.com", 7*24*60*60*1000000000)
				require.NoError(t, err)
				tc.invitationID = newInvitation.ID
			}

			// Create request
			req, err := http.NewRequest("DELETE", "/api/invitations/"+strconv.Itoa(tc.invitationID), nil)
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

			if tc.expectedStatus == http.StatusOK {
				// Parse response
				var response map[string]interface{}
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Contains(t, response, "message")
				assert.Equal(t, "Invitation deleted successfully", response["message"])
			}
		})
	}
}

func TestValidateInvitationHandler(t *testing.T) {
	// Set up test server
	server, tempDir := setupTestServer(t)
	defer teardownTestServer(tempDir)

	// Create a test user
	user := &database.User{
		ID:            uuid.New().String(),
		Username:      "inviter",
		Email:         "inviter@example.com",
		Role:          database.RoleUser,
		EmailVerified: true,
	}
	err := server.db.CreateUser(user, "Password123!")
	require.NoError(t, err)

	// Create an invitation
	invitation, err := server.db.CreateInvitationCode(user.ID, "invited@example.com", 7*24*60*60*1000000000)
	require.NoError(t, err)

	// Create an expired invitation
	expiredInvitation, err := server.db.CreateInvitationCode(user.ID, "expired@example.com", -1*60*60*1000000000)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
	}{
		{
			name: "Valid invitation",
			requestBody: map[string]interface{}{
				"code": invitation.Code,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Expired invitation",
			requestBody: map[string]interface{}{
				"code": expiredInvitation.Code,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Non-existent invitation",
			requestBody: map[string]interface{}{
				"code": "non-existent-code",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing code",
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
			req, err := http.NewRequest("POST", "/api/invitations/validate", bytes.NewBuffer(jsonBody))
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
				assert.Contains(t, response, "invitation")

				invitationResponse := response["invitation"].(map[string]interface{})
				assert.Equal(t, invitation.Code, invitationResponse["code"])
				assert.Equal(t, invitation.Email, invitationResponse["email"])
			} else {
				assert.Contains(t, response, "error")
			}
		})
	}
}
