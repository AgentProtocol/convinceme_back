package server

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/neo/convinceme_backend/internal/auth"
	"github.com/neo/convinceme_backend/internal/database"
)

// parseIntWithDefault parses a string to an integer with a default value
func parseIntWithDefault(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return val
}

// convertToFeedback converts a request to a database.Feedback object
func convertToFeedback(req *FeedbackRequest, userID string) *database.Feedback {
	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}

	var ratingPtr *int
	if req.Rating > 0 {
		ratingPtr = &req.Rating
	}

	return &database.Feedback{
		UserID:     userIDPtr,
		Type:       database.FeedbackType(req.Type),
		Message:    req.Message,
		Rating:     ratingPtr,
		Path:       req.Path,
		Browser:    req.Browser,
		Device:     req.Device,
		ScreenSize: req.ScreenSize,
		CreatedAt:  time.Now(),
	}
}

// FeedbackRequest represents a feedback submission request
type FeedbackRequest struct {
	Type       string `json:"type" binding:"required"`
	Message    string `json:"message" binding:"required"`
	Rating     int    `json:"rating"`
	Path       string `json:"path"`
	Browser    string `json:"browser"`
	Device     string `json:"device"`
	ScreenSize string `json:"screen_size"`
}

// submitFeedbackHandler handles feedback submission
func (s *Server) submitFeedbackHandler(c *gin.Context) {
	// Parse request
	var req FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Get user ID if authenticated
	var userID string
	id, exists := auth.GetUserID(c)
	if exists {
		userID = id
	}

	// Create feedback
	feedback := convertToFeedback(&req, userID)

	// Store feedback in database
	if err := s.db.SaveFeedback(feedback); err != nil {
		log.Printf("Error saving feedback: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save feedback"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Feedback submitted successfully",
		"feedback": feedback,
	})
}

// getFeedbackHandler gets all feedback with filtering and pagination
func (s *Server) getFeedbackHandler(c *gin.Context) {
	// Get the current user role
	role, exists := auth.GetUserRole(c)
	if !exists || role != string(database.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can view feedback"})
		return
	}

	// Get pagination parameters
	paginationParams := GetPaginationParams(c)

	// Get filter parameters
	filterParams := GetFilterParams(c)

	// Get additional feedback-specific filters
	userID := c.Query("user_id")
	feedbackType := c.Query("type")
	minRating := 0
	if c.Query("min_rating") != "" {
		minRating = parseIntWithDefault(c.Query("min_rating"), 0)
	}
	maxRating := 0
	if c.Query("max_rating") != "" {
		maxRating = parseIntWithDefault(c.Query("max_rating"), 0)
	}

	// Parse date range if provided
	var startDate, endDate time.Time
	if c.Query("start_date") != "" {
		parsedDate, err := time.Parse("2006-01-02", c.Query("start_date"))
		if err == nil {
			startDate = parsedDate
		}
	}
	if c.Query("end_date") != "" {
		parsedDate, err := time.Parse("2006-01-02", c.Query("end_date"))
		if err == nil {
			endDate = parsedDate.Add(24 * time.Hour) // Include the entire end date
		}
	}

	// Create database filter
	filter := database.FeedbackFilter{
		UserID:    userID,
		Type:      feedbackType,
		StartDate: startDate,
		EndDate:   endDate,
		MinRating: minRating,
		MaxRating: maxRating,
		Search:    filterParams.Search,
		SortBy:    filterParams.SortBy,
		SortDir:   filterParams.SortDir,
		Page:      paginationParams.Page,
		PageSize:  paginationParams.PageSize,
	}

	// Get feedback from database
	feedback, total, err := s.db.GetAllFeedback(filter)
	if err != nil {
		log.Printf("Error getting feedback: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get feedback"})
		return
	}

	// Update pagination params with total
	paginationParams.Total = total

	// Return paginated response
	SendPaginatedResponse(c, paginationParams, feedback)
}

// getFeedbackByIDHandler gets feedback by ID
func (s *Server) getFeedbackByIDHandler(c *gin.Context) {
	// Get the current user role
	role, exists := auth.GetUserRole(c)
	if !exists || role != string(database.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can view feedback"})
		return
	}

	// Get feedback ID from path
	id := parseIntWithDefault(c.Param("id"), 0)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid feedback ID"})
		return
	}

	// Get feedback from database
	feedback, err := s.db.GetFeedback(id)
	if err != nil {
		log.Printf("Error getting feedback: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Feedback not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"feedback": feedback})
}

// getFeedbackStatsHandler gets feedback statistics
func (s *Server) getFeedbackStatsHandler(c *gin.Context) {
	// Get the current user role
	role, exists := auth.GetUserRole(c)
	if !exists || role != string(database.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can view feedback statistics"})
		return
	}

	// Get feedback stats from database
	stats, err := s.db.GetFeedbackStats()
	if err != nil {
		log.Printf("Error getting feedback stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get feedback statistics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// deleteFeedbackHandler deletes feedback by ID
func (s *Server) deleteFeedbackHandler(c *gin.Context) {
	// Get the current user role
	role, exists := auth.GetUserRole(c)
	if !exists || role != string(database.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can delete feedback"})
		return
	}

	// Get feedback ID from path
	id := parseIntWithDefault(c.Param("id"), 0)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid feedback ID"})
		return
	}

	// Delete feedback from database
	if err := s.db.DeleteFeedback(id); err != nil {
		log.Printf("Error deleting feedback: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete feedback"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Feedback deleted successfully"})
}

// setupFeedbackRoutes sets up the feedback routes
func (s *Server) setupFeedbackRoutes() {
	// Group all feedback routes under /api/feedback
	feedbackGroup := s.router.Group("/api/feedback")
	{
		// Public route to submit feedback
		feedbackGroup.POST("", s.submitFeedbackHandler)

		// Admin routes (require authentication and admin role)
		adminRoutes := feedbackGroup.Group("/")
		adminRoutes.Use(s.auth.AuthMiddleware())
		adminRoutes.Use(s.auth.RequireRole(string(database.RoleAdmin)))
		{
			// Get all feedback with filtering and pagination
			adminRoutes.GET("", s.getFeedbackHandler)

			// Get feedback statistics
			adminRoutes.GET("/stats", s.getFeedbackStatsHandler)

			// Get feedback by ID
			adminRoutes.GET("/:id", s.getFeedbackByIDHandler)

			// Delete feedback by ID
			adminRoutes.DELETE("/:id", s.deleteFeedbackHandler)
		}
	}
}
