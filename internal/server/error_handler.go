package server

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
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

		// Log request
		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		userID, _ := c.Get("userID")
		requestID, _ := c.Get("RequestID")

		// Log format: timestamp - request_id - method - path - status - latency - user_id
		fmt.Printf("[%s] %s - %s - %s - %d - %v - %v\n",
			time.Now().Format(time.RFC3339),
			requestID,
			method,
			path,
			status,
			latency,
			userID,
		)
	}
}

// RecoveryMiddleware recovers from panics
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the error
				fmt.Printf("[PANIC] %s - %s - %v\n%s\n", 
					time.Now().Format(time.RFC3339),
					c.Request.URL.Path,
					err,
					string(debug.Stack()),
				)

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
