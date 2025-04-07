package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// FeedbackType represents the type of feedback
type FeedbackType string

const (
	FeedbackTypeAuth       FeedbackType = "auth"
	FeedbackTypeUI         FeedbackType = "ui"
	FeedbackTypePerformance FeedbackType = "performance"
	FeedbackTypeFeature    FeedbackType = "feature"
	FeedbackTypeBug        FeedbackType = "bug"
	FeedbackTypeOther      FeedbackType = "other"
)

// Feedback represents user feedback
type Feedback struct {
	ID         int         `json:"id"`
	UserID     *string     `json:"user_id,omitempty"`
	Type       FeedbackType `json:"type"`
	Message    string      `json:"message"`
	Rating     *int        `json:"rating,omitempty"`
	Path       string      `json:"path,omitempty"`
	Browser    string      `json:"browser,omitempty"`
	Device     string      `json:"device,omitempty"`
	ScreenSize string      `json:"screen_size,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
}

// FeedbackFilter represents filter parameters for feedback queries
type FeedbackFilter struct {
	UserID    string
	Type      string
	StartDate time.Time
	EndDate   time.Time
	MinRating int
	MaxRating int
	Search    string
	SortBy    string
	SortDir   string
	Page      int
	PageSize  int
}

// SaveFeedback saves feedback to the database
func (d *Database) SaveFeedback(feedback *Feedback) error {
	query := `
		INSERT INTO feedback (
			user_id, type, message, rating, path, browser, device, screen_size, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var userID sql.NullString
	if feedback.UserID != nil {
		userID = sql.NullString{String: *feedback.UserID, Valid: true}
	}

	var rating sql.NullInt64
	if feedback.Rating != nil {
		rating = sql.NullInt64{Int64: int64(*feedback.Rating), Valid: true}
	}

	result, err := d.db.Exec(
		query,
		userID,
		string(feedback.Type),
		feedback.Message,
		rating,
		feedback.Path,
		feedback.Browser,
		feedback.Device,
		feedback.ScreenSize,
		feedback.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save feedback: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}

	feedback.ID = int(id)
	return nil
}

// GetFeedback gets feedback by ID
func (d *Database) GetFeedback(id int) (*Feedback, error) {
	query := `
		SELECT id, user_id, type, message, rating, path, browser, device, screen_size, created_at
		FROM feedback
		WHERE id = ?
	`

	var feedback Feedback
	var userID sql.NullString
	var rating sql.NullInt64
	var createdAt string

	err := d.db.QueryRow(query, id).Scan(
		&feedback.ID,
		&userID,
		&feedback.Type,
		&feedback.Message,
		&rating,
		&feedback.Path,
		&feedback.Browser,
		&feedback.Device,
		&feedback.ScreenSize,
		&createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("feedback not found")
		}
		return nil, fmt.Errorf("failed to get feedback: %w", err)
	}

	if userID.Valid {
		feedback.UserID = &userID.String
	}

	if rating.Valid {
		ratingInt := int(rating.Int64)
		feedback.Rating = &ratingInt
	}

	feedback.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}

	return &feedback, nil
}

// GetFeedbackByUser gets all feedback submitted by a user
func (d *Database) GetFeedbackByUser(userID string) ([]*Feedback, error) {
	query := `
		SELECT id, user_id, type, message, rating, path, browser, device, screen_size, created_at
		FROM feedback
		WHERE user_id = ?
		ORDER BY created_at DESC
	`

	rows, err := d.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get feedback: %w", err)
	}
	defer rows.Close()

	var feedbackList []*Feedback
	for rows.Next() {
		var feedback Feedback
		var userIDNullable sql.NullString
		var rating sql.NullInt64
		var createdAt string

		err := rows.Scan(
			&feedback.ID,
			&userIDNullable,
			&feedback.Type,
			&feedback.Message,
			&rating,
			&feedback.Path,
			&feedback.Browser,
			&feedback.Device,
			&feedback.ScreenSize,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan feedback: %w", err)
		}

		if userIDNullable.Valid {
			feedback.UserID = &userIDNullable.String
		}

		if rating.Valid {
			ratingInt := int(rating.Int64)
			feedback.Rating = &ratingInt
		}

		feedback.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		feedbackList = append(feedbackList, &feedback)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating feedback rows: %w", err)
	}

	return feedbackList, nil
}

// GetAllFeedback gets all feedback with filtering and pagination
func (d *Database) GetAllFeedback(filter FeedbackFilter) ([]*Feedback, int, error) {
	// Build the query
	baseQuery := `
		SELECT id, user_id, type, message, rating, path, browser, device, screen_size, created_at
		FROM feedback
	`
	countQuery := `
		SELECT COUNT(*)
		FROM feedback
	`

	// Build WHERE clause
	var conditions []string
	var args []interface{}

	if filter.UserID != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, filter.UserID)
	}

	if filter.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, filter.Type)
	}

	if !filter.StartDate.IsZero() {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, filter.StartDate.Format(time.RFC3339))
	}

	if !filter.EndDate.IsZero() {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, filter.EndDate.Format(time.RFC3339))
	}

	if filter.MinRating > 0 {
		conditions = append(conditions, "rating >= ?")
		args = append(args, filter.MinRating)
	}

	if filter.MaxRating > 0 {
		conditions = append(conditions, "rating <= ?")
		args = append(args, filter.MaxRating)
	}

	if filter.Search != "" {
		conditions = append(conditions, "(message LIKE ? OR path LIKE ?)")
		searchTerm := "%" + filter.Search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	// Add WHERE clause if conditions exist
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Add ORDER BY clause
	orderClause := " ORDER BY created_at DESC"
	if filter.SortBy != "" {
		orderClause = " ORDER BY " + filter.SortBy
		if filter.SortDir == "asc" {
			orderClause += " ASC"
		} else {
			orderClause += " DESC"
		}
	}

	// Add pagination
	limitOffset := ""
	if filter.Page > 0 && filter.PageSize > 0 {
		limitOffset = fmt.Sprintf(" LIMIT %d OFFSET %d", filter.PageSize, (filter.Page-1)*filter.PageSize)
	}

	// Execute count query
	var total int
	err := d.db.QueryRow(countQuery+whereClause, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count feedback: %w", err)
	}

	// Execute main query
	rows, err := d.db.Query(baseQuery+whereClause+orderClause+limitOffset, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get feedback: %w", err)
	}
	defer rows.Close()

	var feedbackList []*Feedback
	for rows.Next() {
		var feedback Feedback
		var userIDNullable sql.NullString
		var rating sql.NullInt64
		var createdAt string

		err := rows.Scan(
			&feedback.ID,
			&userIDNullable,
			&feedback.Type,
			&feedback.Message,
			&rating,
			&feedback.Path,
			&feedback.Browser,
			&feedback.Device,
			&feedback.ScreenSize,
			&createdAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan feedback: %w", err)
		}

		if userIDNullable.Valid {
			feedback.UserID = &userIDNullable.String
		}

		if rating.Valid {
			ratingInt := int(rating.Int64)
			feedback.Rating = &ratingInt
		}

		feedback.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse created_at: %w", err)
		}

		feedbackList = append(feedbackList, &feedback)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating feedback rows: %w", err)
	}

	return feedbackList, total, nil
}

// GetFeedbackStats gets statistics about feedback
func (d *Database) GetFeedbackStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get total count
	var totalCount int
	err := d.db.QueryRow("SELECT COUNT(*) FROM feedback").Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}
	stats["total_count"] = totalCount

	// Get count by type
	rows, err := d.db.Query("SELECT type, COUNT(*) FROM feedback GROUP BY type")
	if err != nil {
		return nil, fmt.Errorf("failed to get count by type: %w", err)
	}
	defer rows.Close()

	countByType := make(map[string]int)
	for rows.Next() {
		var feedbackType string
		var count int
		if err := rows.Scan(&feedbackType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan count by type: %w", err)
		}
		countByType[feedbackType] = count
	}
	stats["count_by_type"] = countByType

	// Get average rating
	var avgRating sql.NullFloat64
	err = d.db.QueryRow("SELECT AVG(rating) FROM feedback WHERE rating IS NOT NULL").Scan(&avgRating)
	if err != nil {
		return nil, fmt.Errorf("failed to get average rating: %w", err)
	}
	if avgRating.Valid {
		stats["average_rating"] = avgRating.Float64
	} else {
		stats["average_rating"] = 0
	}

	// Get rating distribution
	rows, err = d.db.Query("SELECT rating, COUNT(*) FROM feedback WHERE rating IS NOT NULL GROUP BY rating ORDER BY rating")
	if err != nil {
		return nil, fmt.Errorf("failed to get rating distribution: %w", err)
	}
	defer rows.Close()

	ratingDistribution := make(map[int]int)
	for rows.Next() {
		var rating int
		var count int
		if err := rows.Scan(&rating, &count); err != nil {
			return nil, fmt.Errorf("failed to scan rating distribution: %w", err)
		}
		ratingDistribution[rating] = count
	}
	stats["rating_distribution"] = ratingDistribution

	// Get feedback over time (last 30 days)
	query := `
		SELECT DATE(created_at) as date, COUNT(*) as count
		FROM feedback
		WHERE created_at >= date('now', '-30 days')
		GROUP BY DATE(created_at)
		ORDER BY date
	`
	rows, err = d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get feedback over time: %w", err)
	}
	defer rows.Close()

	feedbackOverTime := make(map[string]int)
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, fmt.Errorf("failed to scan feedback over time: %w", err)
		}
		feedbackOverTime[date] = count
	}
	stats["feedback_over_time"] = feedbackOverTime

	return stats, nil
}

// DeleteFeedback deletes feedback by ID
func (d *Database) DeleteFeedback(id int) error {
	_, err := d.db.Exec("DELETE FROM feedback WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete feedback: %w", err)
	}
	return nil
}
