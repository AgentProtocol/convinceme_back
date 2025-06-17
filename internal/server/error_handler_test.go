package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorHandler(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Test cases
	testCases := []struct {
		name           string
		setupRouter    func(*gin.Engine)
		expectedStatus int
		expectedError  bool
		appEnv         string
	}{
		{
			name: "No error",
			setupRouter: func(r *gin.Engine) {
				r.GET("/test", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"status": "ok"})
				})
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
			appEnv:         "",
		},
		{
			name: "With error",
			setupRouter: func(r *gin.Engine) {
				r.GET("/test", func(c *gin.Context) {
					c.Error(errors.New("test error"))
					c.Status(http.StatusInternalServerError)
				})
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  true,
			appEnv:         "",
		},
		{
			name: "With error in development mode",
			setupRouter: func(r *gin.Engine) {
				r.GET("/test", func(c *gin.Context) {
					c.Error(errors.New("test error"))
					c.Status(http.StatusInternalServerError)
				})
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  true,
			appEnv:         "development",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set APP_ENV for the test
			if tc.appEnv != "" {
				oldEnv := os.Getenv("APP_ENV")
				os.Setenv("APP_ENV", tc.appEnv)
				defer os.Setenv("APP_ENV", oldEnv)
			}

			// Create a new router with the error handler middleware
			router := gin.New()
			router.Use(RequestIDMiddleware())
			router.Use(ErrorHandler())

			// Set up the router
			tc.setupRouter(router)

			// Create a test request
			req, err := http.NewRequest("GET", "/test", nil)
			require.NoError(t, err)

			// Create a response recorder
			w := httptest.NewRecorder()

			// Perform the request
			router.ServeHTTP(w, req)

			// Check the response
			assert.Equal(t, tc.expectedStatus, w.Code)

			// Check if the response contains the expected fields
			if tc.expectedError {
				assert.Contains(t, w.Body.String(), "error")
				
				// In development mode, should include details
				if tc.appEnv == "development" {
					assert.Contains(t, w.Body.String(), "details")
				}
			} else {
				assert.Contains(t, w.Body.String(), "status")
				assert.Contains(t, w.Body.String(), "ok")
			}

			// Check if the response has the request ID header
			assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
		})
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a new router with the recovery middleware
	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(RecoveryMiddleware())

	// Add a route that panics
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	// Test cases
	testCases := []struct {
		name           string
		path           string
		expectedStatus int
		appEnv         string
	}{
		{
			name:           "No panic",
			path:           "/no-panic",
			expectedStatus: http.StatusNotFound, // Route doesn't exist
			appEnv:         "",
		},
		{
			name:           "With panic",
			path:           "/panic",
			expectedStatus: http.StatusInternalServerError,
			appEnv:         "",
		},
		{
			name:           "With panic in development mode",
			path:           "/panic",
			expectedStatus: http.StatusInternalServerError,
			appEnv:         "development",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set APP_ENV for the test
			if tc.appEnv != "" {
				oldEnv := os.Getenv("APP_ENV")
				os.Setenv("APP_ENV", tc.appEnv)
				defer os.Setenv("APP_ENV", oldEnv)
			}

			// Create a test request
			req, err := http.NewRequest("GET", tc.path, nil)
			require.NoError(t, err)

			// Create a response recorder
			w := httptest.NewRecorder()

			// Perform the request
			router.ServeHTTP(w, req)

			// Check the response
			assert.Equal(t, tc.expectedStatus, w.Code)

			// Check if the response has the request ID header
			assert.NotEmpty(t, w.Header().Get("X-Request-ID"))

			// If it's a panic, check the response body
			if tc.path == "/panic" {
				assert.Contains(t, w.Body.String(), "error")
				
				// In development mode, should include details
				if tc.appEnv == "development" {
					assert.Contains(t, w.Body.String(), "details")
					assert.Contains(t, w.Body.String(), "test panic")
				}
			}
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a new router with the logging middleware
	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(LoggingMiddleware())

	// Add a test route
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Create a test request
	req, err := http.NewRequest("GET", "/test", nil)
	require.NoError(t, err)

	// Create a response recorder
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, http.StatusOK, w.Code)

	// Check if the response has the request ID header
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}
