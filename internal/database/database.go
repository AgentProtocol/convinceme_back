package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neo/convinceme_backend/internal/logging"
	"github.com/neo/convinceme_backend/internal/scoring"
)

type Database struct {
	db *sql.DB
}

// Debate represents a debate session in the database
type Debate struct {
	ID         string     `json:"id"`
	Topic      string     `json:"topic"`
	Status     string     `json:"status"`
	Agent1Name string     `json:"agent1_name"`
	Agent2Name string     `json:"agent2_name"`
	CreatedAt  time.Time  `json:"created_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"` // Use pointer for nullable timestamp
	Winner     *string    `json:"winner,omitempty"`   // Use pointer for nullable string
}

// Topic represents a pre-generated debate topic with agent pairings
type Topic struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Agent1Name  string    `json:"agent1_name"`
	Agent1Role  string    `json:"agent1_role"`
	Agent2Name  string    `json:"agent2_name"`
	Agent2Role  string    `json:"agent2_role"`
	Category    string    `json:"category,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Argument represents a player's argument in the database
type Argument struct {
	ID        int64                  `json:"id"`
	PlayerID  string                 `json:"player_id"`
	Topic     string                 `json:"topic"`
	Content   string                 `json:"content"`
	Side      string                 `json:"side"`
	DebateID  *string                `json:"debate_id,omitempty"` // Use pointer for nullable string
	CreatedAt string                 `json:"created_at"`
	Score     *scoring.ArgumentScore `json:"score,omitempty"`
}

// New creates a new database connection and initializes the schema
func New(dataDir string) (*Database, error) {
	logging.Info("Initializing database", map[string]interface{}{
		"data_dir": dataDir,
	})

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logging.Error("Failed to create data directory", map[string]interface{}{
			"error":    err,
			"data_dir": dataDir,
		})
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "arguments.db")
	logging.Debug("Opening database connection", map[string]interface{}{
		"db_path": dbPath,
	})

	// Configure connection pool
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logging.Error("Failed to open database", map[string]interface{}{
			"error":   err,
			"db_path": dbPath,
		})
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)                  // Maximum number of open connections
	db.SetMaxIdleConns(10)                  // Maximum number of idle connections
	db.SetConnMaxLifetime(30 * time.Minute) // Maximum lifetime of a connection
	db.SetConnMaxIdleTime(10 * time.Minute) // Maximum idle time of a connection

	logging.Debug("Database connection pool configured", map[string]interface{}{
		"max_open_conns":    25,
		"max_idle_conns":    10,
		"conn_max_lifetime": "30m",
		"conn_max_idle":     "10m",
	})

	// Verify connection
	if err := db.Ping(); err != nil {
		logging.Error("Failed to ping database", map[string]interface{}{
			"error": err,
		})
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Run migrations
	logging.Info("Running database migrations")
	migrationManager := NewMigrationManager(db)
	err = migrationManager.MigrateUp("migrations")
	if err != nil {
		logging.Error("Failed to run migrations", map[string]interface{}{
			"error": err,
		})
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}
	logging.Info("Database migrations completed successfully")

	logging.Info("Database initialized successfully")
	return &Database{db: db}, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// SaveArgument saves a new argument to the database, linking it to a debate
func (d *Database) SaveArgument(playerID, topic, content, side, debateID string) (int64, error) {
	logging.LogDatabaseEvent("INSERT", "arguments", map[string]interface{}{
		"player_id":      playerID,
		"topic":          topic,
		"side":           side,
		"debate_id":      debateID,
		"content_length": len(content),
	})

	query := `INSERT INTO arguments (player_id, topic, content, side, debate_id) VALUES (?, ?, ?, ?, ?)`
	result, err := d.db.Exec(query, playerID, topic, content, side, debateID)
	if err != nil {
		logging.Error("Failed to save argument", map[string]interface{}{
			"error":     err,
			"player_id": playerID,
			"debate_id": debateID,
		})
		return 0, fmt.Errorf("failed to save argument: %v", err)
	}

	id, _ := result.LastInsertId()
	logging.Debug("Argument saved successfully", map[string]interface{}{
		"argument_id": id,
		"player_id":   playerID,
		"debate_id":   debateID,
	})

	return id, nil
}

// SaveScore saves a score for an argument, linking it to a debate
func (d *Database) SaveScore(argumentID int64, debateID string, score *scoring.ArgumentScore) error {
	logging.LogDatabaseEvent("INSERT", "scores", map[string]interface{}{
		"argument_id": argumentID,
		"debate_id":   debateID,
		"average":     score.Average,
	})

	query := `INSERT INTO scores (argument_id, debate_id, strength, relevance, logic, truth, humor, average, explanation)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := d.db.Exec(query, argumentID, debateID, score.Strength, score.Relevance, score.Logic,
		score.Truth, score.Humor, score.Average, score.Explanation)

	if err != nil {
		logging.Error("Failed to save score", map[string]interface{}{
			"error":       err,
			"argument_id": argumentID,
			"debate_id":   debateID,
		})
		return fmt.Errorf("failed to save score for debate %s: %v", debateID, err)
	}

	logging.Debug("Score saved successfully", map[string]interface{}{
		"argument_id": argumentID,
		"debate_id":   debateID,
		"score":       score.Average,
	})

	return nil
}

// GetArgumentWithScore retrieves an argument and its score by ID
func (d *Database) GetArgumentWithScore(id int64) (*Argument, error) {
	query := `
		SELECT a.id, a.player_id, a.topic, a.content, a.side, a.debate_id, a.created_at,
			   s.strength, s.relevance, s.logic, s.truth, s.humor, s.average, s.explanation
		FROM arguments a
		LEFT JOIN scores s ON a.id = s.argument_id
		WHERE a.id = ?`

	var arg Argument
	var score scoring.ArgumentScore
	// Use sql.NullString for nullable debate_id
	var debateID sql.NullString

	err := d.db.QueryRow(query, id).Scan(
		&arg.ID, &arg.PlayerID, &arg.Topic, &arg.Content, &arg.Side, &debateID, &arg.CreatedAt,
		&score.Strength, &score.Relevance, &score.Logic, &score.Truth, &score.Humor,
		&score.Average, &score.Explanation,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("argument %d not found", id)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get argument: %v", err)
	}

	if debateID.Valid {
		arg.DebateID = &debateID.String
	}

	arg.Score = &score
	return &arg, nil
}

// GetAllArguments retrieves the last 100 arguments with their scores
// Consider adding filtering by debate_id if needed later
func (d *Database) GetAllArguments() ([]*Argument, error) {
	query := `
		SELECT a.id, a.player_id, a.topic, a.content, a.side, a.debate_id, a.created_at,
			   s.strength, s.relevance, s.logic, s.truth, s.humor, s.average, s.explanation
		FROM arguments a
		LEFT JOIN scores s ON a.id = s.argument_id
		ORDER BY a.created_at DESC
		LIMIT 100`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all arguments: %v", err)
	}
	defer rows.Close()

	var arguments []*Argument
	for rows.Next() {
		var arg Argument
		var score scoring.ArgumentScore
		var debateID sql.NullString

		err := rows.Scan(
			&arg.ID, &arg.PlayerID, &arg.Topic, &arg.Content, &arg.Side, &debateID, &arg.CreatedAt,
			&score.Strength, &score.Relevance, &score.Logic, &score.Truth, &score.Humor,
			&score.Average, &score.Explanation,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan argument row: %v", err)
		}

		if debateID.Valid {
			arg.DebateID = &debateID.String
		}
		arg.Score = &score
		arguments = append(arguments, &arg)
	}

	return arguments, nil
}

// GetLeaderboard retrieves the top-scoring arguments for a specific debate
func (d *Database) GetLeaderboard(debateID string, limit int) ([]*Argument, error) {
	query := `
		SELECT a.id, a.player_id, a.topic, a.content, a.side, a.debate_id, a.created_at,
			   s.strength, s.relevance, s.logic, s.truth, s.humor, s.average, s.explanation
		FROM arguments a
		INNER JOIN scores s ON a.id = s.argument_id
		WHERE a.debate_id = ?
		ORDER BY s.average DESC, a.created_at ASC
		LIMIT ?`

	rows, err := d.db.Query(query, debateID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query leaderboard for debate %s: %v", debateID, err)
	}
	defer rows.Close()

	var arguments []*Argument
	for rows.Next() {
		var arg Argument
		var score scoring.ArgumentScore
		var debateIDResult sql.NullString

		err := rows.Scan(
			&arg.ID, &arg.PlayerID, &arg.Topic, &arg.Content, &arg.Side, &debateIDResult, &arg.CreatedAt,
			&score.Strength, &score.Relevance, &score.Logic, &score.Truth, &score.Humor,
			&score.Average, &score.Explanation,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan leaderboard argument row: %v", err)
		}

		if debateIDResult.Valid {
			arg.DebateID = &debateIDResult.String
		}
		arg.Score = &score
		arguments = append(arguments, &arg)
	}

	return arguments, nil
}

// --- Debate Management Functions ---

// CreateDebate adds a new debate session to the database
func (d *Database) CreateDebate(id, topic, status, agent1Name, agent2Name string) error {
	query := `INSERT INTO debates (id, topic, status, agent1_name, agent2_name) VALUES (?, ?, ?, ?, ?)`
	_, err := d.db.Exec(query, id, topic, status, agent1Name, agent2Name)
	if err != nil {
		return fmt.Errorf("failed to create debate %s: %v", id, err)
	}
	return nil
}

// UpdateDebateStatus updates the status of a specific debate
func (d *Database) UpdateDebateStatus(id, status string) error {
	query := `UPDATE debates SET status = ? WHERE id = ?`
	result, err := d.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update status for debate %s: %v", id, err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("debate %s not found for status update", id)
	}
	return nil
}

// UpdateDebateEnd marks a debate as finished, setting the end time and winner
func (d *Database) UpdateDebateEnd(id, status, winner string) error {
	query := `UPDATE debates SET status = ?, ended_at = CURRENT_TIMESTAMP, winner = ? WHERE id = ?`
	result, err := d.db.Exec(query, status, winner, id)
	if err != nil {
		return fmt.Errorf("failed to end debate %s: %v", id, err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("debate %s not found for ending", id)
	}
	return nil
}

// GetDebate retrieves a specific debate by its ID
func (d *Database) GetDebate(id string) (*Debate, error) {
	query := `SELECT id, topic, status, agent1_name, agent2_name, created_at, ended_at, winner FROM debates WHERE id = ?`
	var debate Debate
	var endedAt sql.NullTime
	var winner sql.NullString

	err := d.db.QueryRow(query, id).Scan(
		&debate.ID, &debate.Topic, &debate.Status, &debate.Agent1Name, &debate.Agent2Name,
		&debate.CreatedAt, &endedAt, &winner,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("debate %s not found", id)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get debate %s: %v", id, err)
	}

	if endedAt.Valid {
		debate.EndedAt = &endedAt.Time
	}
	if winner.Valid {
		debate.Winner = &winner.String
	}

	return &debate, nil
}

// DebateFilter contains filter parameters for debates
type DebateFilter struct {
	Status  string
	Search  string
	SortBy  string
	SortDir string
	Offset  int
	Limit   int
}

// ListDebates retrieves debates with pagination and filtering
func (d *Database) ListDebates(filter DebateFilter) ([]*Debate, int, error) {
	// Build the WHERE clause based on filters
	whereClause := ""
	args := []any{}

	// Add status filter if provided
	if filter.Status != "" {
		whereClause = "WHERE status = ?"
		args = append(args, filter.Status)
	}

	// Add search filter if provided
	if filter.Search != "" {
		if whereClause != "" {
			whereClause += " AND "
		} else {
			whereClause = "WHERE "
		}
		// Search in topic
		whereClause += "topic LIKE ?"
		searchTerm := "%" + filter.Search + "%"
		args = append(args, searchTerm)
	}

	// Build the ORDER BY clause
	orderClause := "ORDER BY "
	if filter.SortBy != "" {
		orderClause += filter.SortBy
	} else {
		orderClause += "created_at"
	}

	if filter.SortDir != "" && filter.SortDir == "desc" {
		orderClause += " DESC"
	} else {
		orderClause += " ASC"
	}

	// Count total records for pagination
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM debates %s", whereClause)
	var total int
	err := d.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count debates: %v", err)
	}

	// Build the main query with pagination
	query := fmt.Sprintf(
		`SELECT id, topic, status, agent1_name, agent2_name, created_at, ended_at, winner
		FROM debates %s %s LIMIT ? OFFSET ?`,
		whereClause, orderClause,
	)

	// Add pagination parameters
	args = append(args, filter.Limit, filter.Offset)

	// Execute the query
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list debates: %v", err)
	}
	defer rows.Close()

	var debates []*Debate
	for rows.Next() {
		var debate Debate
		var endedAt, winner sql.NullString
		err := rows.Scan(
			&debate.ID, &debate.Topic, &debate.Status, &debate.Agent1Name, &debate.Agent2Name,
			&debate.CreatedAt, &endedAt, &winner,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan debate row: %v", err)
		}

		if endedAt.Valid {
			endTime, err := time.Parse(time.RFC3339, endedAt.String)
			if err == nil {
				debate.EndedAt = &endTime
			}
		}

		if winner.Valid {
			debate.Winner = &winner.String
		}

		debates = append(debates, &debate)
	}

	return debates, total, nil
}

// ListActiveDebates retrieves debates that are currently 'waiting' or 'active'
func (d *Database) ListActiveDebates() ([]*Debate, error) {
	// Note: We could use the DebateFilter here, but for now we're using a custom query
	// that specifically looks for both 'waiting' and 'active' statuses

	// Custom query for active debates (includes 'waiting' status)
	query := `SELECT id, topic, status, agent1_name, agent2_name, created_at FROM debates WHERE status = 'waiting' OR status = 'active' ORDER BY created_at DESC`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list active debates: %v", err)
	}
	defer rows.Close()

	var debates []*Debate
	for rows.Next() {
		var debate Debate
		err := rows.Scan(
			&debate.ID, &debate.Topic, &debate.Status, &debate.Agent1Name, &debate.Agent2Name, &debate.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan active debate row: %v", err)
		}
		debates = append(debates, &debate)
	}

	return debates, nil
}

// TopicFilter contains filter parameters for topics
type TopicFilter struct {
	Category string
	Search   string
	SortBy   string
	SortDir  string
	Offset   int
	Limit    int
}

// GetTopics retrieves all available pre-generated topics with pagination and filtering
func (d *Database) GetTopics(filter TopicFilter) ([]*Topic, int, error) {
	// Build the WHERE clause based on filters
	whereClause := ""
	args := []any{}

	// Add category filter if provided
	if filter.Category != "" {
		whereClause = "WHERE category = ?"
		args = append(args, filter.Category)
	}

	// Add search filter if provided
	if filter.Search != "" {
		if whereClause != "" {
			whereClause += " AND "
		} else {
			whereClause = "WHERE "
		}
		// Search in title or description
		whereClause += "(title LIKE ? OR description LIKE ?)"
		searchTerm := "%" + filter.Search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	// Build the ORDER BY clause
	orderClause := "ORDER BY "
	if filter.SortBy != "" {
		orderClause += filter.SortBy
	} else {
		orderClause += "id"
	}

	if filter.SortDir != "" && filter.SortDir == "desc" {
		orderClause += " DESC"
	} else {
		orderClause += " ASC"
	}

	// Count total records for pagination
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM topics %s", whereClause)
	var total int
	err := d.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count topics: %v", err)
	}

	// Build the main query with pagination
	query := fmt.Sprintf(
		`SELECT id, title, description, agent1_name, agent1_role, agent2_name, agent2_role, category, created_at
		FROM topics %s %s LIMIT ? OFFSET ?`,
		whereClause, orderClause,
	)

	// Add pagination parameters
	args = append(args, filter.Limit, filter.Offset)

	// Execute the query
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list topics: %v", err)
	}
	defer rows.Close()

	var topics []*Topic
	for rows.Next() {
		var topic Topic
		var description, category sql.NullString
		err := rows.Scan(
			&topic.ID, &topic.Title, &description, &topic.Agent1Name, &topic.Agent1Role,
			&topic.Agent2Name, &topic.Agent2Role, &category, &topic.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan topic row: %v", err)
		}

		if description.Valid {
			topic.Description = description.String
		}
		if category.Valid {
			topic.Category = category.String
		}

		topics = append(topics, &topic)
	}

	return topics, total, nil
}

// GetTopicsByCategory retrieves topics filtered by category
func (d *Database) GetTopicsByCategory(category string) ([]*Topic, error) {
	query := `SELECT id, title, description, agent1_name, agent1_role, agent2_name, agent2_role, category, created_at
			FROM topics WHERE category = ? ORDER BY id`
	rows, err := d.db.Query(query, category)
	if err != nil {
		return nil, fmt.Errorf("failed to list topics by category: %v", err)
	}
	defer rows.Close()

	var topics []*Topic
	for rows.Next() {
		var topic Topic
		var description, category sql.NullString
		err := rows.Scan(
			&topic.ID, &topic.Title, &description, &topic.Agent1Name, &topic.Agent1Role,
			&topic.Agent2Name, &topic.Agent2Role, &category, &topic.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan topic row: %v", err)
		}

		if description.Valid {
			topic.Description = description.String
		}
		if category.Valid {
			topic.Category = category.String
		}

		topics = append(topics, &topic)
	}

	return topics, nil
}

// GetTopic retrieves a specific topic by ID
func (d *Database) GetTopic(id int) (*Topic, error) {
	query := `SELECT id, title, description, agent1_name, agent1_role, agent2_name, agent2_role, category, created_at
			FROM topics WHERE id = ?`
	var topic Topic
	var description, category sql.NullString

	err := d.db.QueryRow(query, id).Scan(
		&topic.ID, &topic.Title, &description, &topic.Agent1Name, &topic.Agent1Role,
		&topic.Agent2Name, &topic.Agent2Role, &category, &topic.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("topic with ID %d not found", id)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get topic: %v", err)
	}

	if description.Valid {
		topic.Description = description.String
	}
	if category.Valid {
		topic.Category = category.String
	}

	return &topic, nil
}
