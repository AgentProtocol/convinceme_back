//go:build ignore

package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/neo/convinceme_backend/internal/auth"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDatabase is a mock implementation of the database interface
type MockDatabase struct {
	mock.Mock
	*database.Database // Embed the real database to satisfy the interface
}

// CreateUser mocks the CreateUser method
func (m *MockDatabase) CreateUser(user *database.User, password string) error {
	args := m.Called(user, password)
	return args.Error(0)
}

// GetUserByUsername mocks the GetUserByUsername method
func (m *MockDatabase) GetUserByUsername(username string) (*database.User, error) {
	args := m.Called(username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.User), args.Error(1)
}

// GetUserByEmail mocks the GetUserByEmail method
func (m *MockDatabase) GetUserByEmail(email string) (*database.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.User), args.Error(1)
}

// GetUserByID mocks the GetUserByID method
func (m *MockDatabase) GetUserByID(id string) (*database.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.User), args.Error(1)
}

// VerifyPassword mocks the VerifyPassword method
func (m *MockDatabase) VerifyPassword(username, password string) (*database.User, error) {
	args := m.Called(username, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.User), args.Error(1)
}

// UpdateUser mocks the UpdateUser method
func (m *MockDatabase) UpdateUser(user *database.User) error {
	args := m.Called(user)
	return args.Error(0)
}

// UpdatePassword mocks the UpdatePassword method
func (m *MockDatabase) UpdatePassword(userID, password string) error {
	args := m.Called(userID, password)
	return args.Error(0)
}

// DeleteUser mocks the DeleteUser method
func (m *MockDatabase) DeleteUser(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

// setupTestServer sets up a test server with mock dependencies
func setupTestServer() (*Server, *MockDatabase, *gin.Engine) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create mock database
	mockDB := new(MockDatabase)

	// Create auth handler
	authHandler := auth.New(auth.Config{
		JWTSecret:     "test-secret",
		TokenDuration: 24 * time.Hour,
	})

	// Create server
	server := &Server{
		db:   mockDB,
		auth: authHandler,
	}

	// Create router
	router := gin.New()
	router.Use(gin.Recovery())

	// Add routes
	router.POST("/api/auth/register", server.registerHandler)
	router.POST("/api/auth/login", server.loginHandler)
	router.GET("/api/auth/me", authHandler.AuthMiddleware(), server.meHandler)
	router.PUT("/api/auth/me", authHandler.AuthMiddleware(), server.updateUserHandler)
	router.POST("/api/auth/change-password", authHandler.AuthMiddleware(), server.changePasswordHandler)
	router.DELETE("/api/auth/me", authHandler.AuthMiddleware(), server.deleteUserHandler)

	server.router = router

	return server, mockDB, router
}

// TestRegisterHandler tests the register handler
func TestRegisterHandler(t *testing.T) {
	// Setup
	_, mockDB, router := setupTestServer()

	// Test cases
	testCases := []struct {
		name           string
		requestBody    map[string]interface{}
		setupMock      func()
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "Valid registration",
			requestBody: map[string]interface{}{
				"username": "testuser",
				"email":    "test@example.com",
				"password": "password123",
			},
			setupMock: func() {
				// Mock GetUserByUsername - user doesn't exist
				mockDB.On("GetUserByUsername", "testuser").Return(nil, assert.AnError)
				// Mock GetUserByEmail - email doesn't exist
				mockDB.On("GetUserByEmail", "test@example.com").Return(nil, assert.AnError)
				// Mock CreateUser - success
				mockDB.On("CreateUser", mock.AnythingOfType("*database.User"), "password123").Return(nil)
			},
			expectedStatus: http.StatusCreated,
			expectedBody: map[string]interface{}{
				"message": "User registered successfully",
				"user": map[string]interface{}{
					"username": "testuser",
					"email":    "test@example.com",
				},
				"token": mock.AnythingOfType("string"),
			},
		},
		{
			name: "Username already exists",
			requestBody: map[string]interface{}{
				"username": "existinguser",
				"email":    "new@example.com",
				"password": "password123",
			},
			setupMock: func() {
				// Mock GetUserByUsername - user exists
				existingUser := &database.User{
					ID:       uuid.New().String(),
					Username: "existinguser",
					Email:    "existing@example.com",
				}
				mockDB.On("GetUserByUsername", "existinguser").Return(existingUser, nil)
			},
			expectedStatus: http.StatusConflict,
			expectedBody: map[string]interface{}{
				"error": "Username already exists",
			},
		},
		{
			name: "Email already exists",
			requestBody: map[string]interface{}{
				"username": "newuser",
				"email":    "existing@example.com",
				"password": "password123",
			},
			setupMock: func() {
				// Mock GetUserByUsername - user doesn't exist
				mockDB.On("GetUserByUsername", "newuser").Return(nil, assert.AnError)
				// Mock GetUserByEmail - email exists
				existingUser := &database.User{
					ID:       uuid.New().String(),
					Username: "existinguser",
					Email:    "existing@example.com",
				}
				mockDB.On("GetUserByEmail", "existing@example.com").Return(existingUser, nil)
			},
			expectedStatus: http.StatusConflict,
			expectedBody: map[string]interface{}{
				"error": "Email already exists",
			},
		},
		{
			name: "Invalid request - missing username",
			requestBody: map[string]interface{}{
				"email":    "test@example.com",
				"password": "password123",
			},
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"error": "Invalid request",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock
			tc.setupMock()

			// Create request
			requestBody, _ := json.Marshal(tc.requestBody)
			req, _ := http.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			router.ServeHTTP(w, req)

			// Check status code
			assert.Equal(t, tc.expectedStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Check response
			for key, expectedValue := range tc.expectedBody {
				if key == "user" {
					// Check user object
					user, ok := response["user"].(map[string]interface{})
					assert.True(t, ok)
					expectedUser := expectedValue.(map[string]interface{})
					for userKey, userValue := range expectedUser {
						assert.Equal(t, userValue, user[userKey])
					}
				} else if key == "token" && expectedValue == mock.AnythingOfType("string") {
					// Just check that token exists and is a string
					_, ok := response["token"].(string)
					assert.True(t, ok)
				} else {
					// Check other fields
					assert.Equal(t, expectedValue, response[key])
				}
			}

			// Reset mock
			mockDB.AssertExpectations(t)
			mockDB.ExpectedCalls = nil
			mockDB.Calls = nil
		})
	}
}

// TestLoginHandler tests the login handler
func TestLoginHandler(t *testing.T) {
	// Setup
	_, mockDB, router := setupTestServer()

	// Test cases
	testCases := []struct {
		name           string
		requestBody    map[string]interface{}
		setupMock      func()
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "Valid login",
			requestBody: map[string]interface{}{
				"username": "testuser",
				"password": "password123",
			},
			setupMock: func() {
				// Mock VerifyPassword - success
				user := &database.User{
					ID:        uuid.New().String(),
					Username:  "testuser",
					Email:     "test@example.com",
					CreatedAt: time.Now(),
				}
				mockDB.On("VerifyPassword", "testuser", "password123").Return(user, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": "Login successful",
				"user": map[string]interface{}{
					"username": "testuser",
					"email":    "test@example.com",
				},
				"token": mock.AnythingOfType("string"),
			},
		},
		{
			name: "Invalid credentials",
			requestBody: map[string]interface{}{
				"username": "testuser",
				"password": "wrongpassword",
			},
			setupMock: func() {
				// Mock VerifyPassword - failure
				mockDB.On("VerifyPassword", "testuser", "wrongpassword").Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody: map[string]interface{}{
				"error": "Invalid username or password",
			},
		},
		{
			name: "Invalid request - missing password",
			requestBody: map[string]interface{}{
				"username": "testuser",
			},
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"error": "Invalid request",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock
			tc.setupMock()

			// Create request
			requestBody, _ := json.Marshal(tc.requestBody)
			req, _ := http.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			router.ServeHTTP(w, req)

			// Check status code
			assert.Equal(t, tc.expectedStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Check response
			for key, expectedValue := range tc.expectedBody {
				if key == "user" {
					// Check user object
					user, ok := response["user"].(map[string]interface{})
					assert.True(t, ok)
					expectedUser := expectedValue.(map[string]interface{})
					for userKey, userValue := range expectedUser {
						assert.Equal(t, userValue, user[userKey])
					}
				} else if key == "token" && expectedValue == mock.AnythingOfType("string") {
					// Just check that token exists and is a string
					_, ok := response["token"].(string)
					assert.True(t, ok)
				} else {
					// Check other fields
					assert.Equal(t, expectedValue, response[key])
				}
			}

			// Reset mock
			mockDB.AssertExpectations(t)
			mockDB.ExpectedCalls = nil
			mockDB.Calls = nil
		})
	}
}
