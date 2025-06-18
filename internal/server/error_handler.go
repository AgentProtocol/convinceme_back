package server

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/neo/convinceme_backend/internal/logging"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Status     int       `json:"status"`
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	Path       string    `json:"path"`
	Timestamp  time.Time `json:"timestamp"`
	RequestID  string    `json:"request_id,omitempty"`
	ErrorCode  string    `json:"error_code,omitempty"`
	HelpURL    string    `json:"help_url,omitempty"`
	DevMessage string    `json:"-"` // For logging only, not sent to client
}

// ErrorHandler middleware for handling errors
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// If there are errors, handle them
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			status := c.Writer.Status()
			if status < 400 {
				status = http.StatusInternalServerError
			}

			// Create error response
			errorResponse := ErrorResponse{
				Status:    status,
				Message:   "An error occurred while processing your request",
				Path:      c.Request.URL.Path,
				Timestamp: time.Now(),
				RequestID: c.GetString("RequestID"),
			}

			// Add details in development mode
			if os.Getenv("APP_ENV") == "development" {
				errorResponse.Details = err.Error()
				errorResponse.DevMessage = string(debug.Stack())
			}

			// Log the error
			fmt.Printf("[ERROR] %s - %s - %s\n", errorResponse.Timestamp.Format(time.RFC3339),
				errorResponse.Path, err.Error())

			// Return error response
			c.JSON(status, gin.H{"error": errorResponse})
		}
	}
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate a unique request ID
		requestID := fmt.Sprintf("%d", time.Now().UnixNano())
		c.Set("RequestID", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// LoggingMiddleware logs all requests
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		userID, _ := c.Get("userID")
		requestID, _ := c.Get("RequestID")

		// Determine log level based on status code
		if status >= 500 {
			logging.LogHTTPRequest(method, path, status, latency, map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"error":      "server_error",
			})
		} else if status >= 400 {
			logging.LogHTTPRequest(method, path, status, latency, map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"error":      "client_error",
			})
		} else {
			logging.LogHTTPRequest(method, path, status, latency, map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
			})
		}
	}
}

// RecoveryMiddleware recovers from panics
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID, _ := c.Get("RequestID")

				// Log the panic with our logging system
				logging.Error("Server panic occurred", map[string]interface{}{
					"request_id": requestID,
					"path":       c.Request.URL.Path,
					"method":     c.Request.Method,
					"error":      err,
					"stack":      string(debug.Stack()),
				})

				// Create error response
				errorResponse := ErrorResponse{
					Status:    http.StatusInternalServerError,
					Message:   "An unexpected error occurred",
					Path:      c.Request.URL.Path,
					Timestamp: time.Now(),
					RequestID: c.GetString("RequestID"),
				}

				// Add details in development mode
				if os.Getenv("APP_ENV") == "development" {
					errorResponse.Details = fmt.Sprintf("%v", err)
					errorResponse.DevMessage = string(debug.Stack())
				}

				// Return error response
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": errorResponse})
			}
		}()
		c.Next()
	}
}
