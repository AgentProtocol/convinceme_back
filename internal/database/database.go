package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/neo/convinceme_backend/internal/scoring"
)

type Database struct {
	db *sql.DB
}

// Argument represents a player's argument in the database
type Argument struct {
	ID        int64                  `json:"id"`
	PlayerID  string                 `json:"player_id"`
	Topic     string                 `json:"topic"`
	Content   string                 `json:"content"`
	CreatedAt string                 `json:"created_at"`
	Score     *scoring.ArgumentScore `json:"score,omitempty"`
}

// New creates a new database connection and initializes the schema
func New(dataDir string) (*Database, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "arguments.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Read and execute schema
	schemaSQL, err := os.ReadFile("sql/schema.sql")
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %v", err)
	}

	if _, err := db.Exec(string(schemaSQL)); err != nil {
		return nil, fmt.Errorf("failed to create schema: %v", err)
	}

	return &Database{db: db}, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// SaveArgument saves a new argument to the database
func (d *Database) SaveArgument(playerID, topic, content string) (int64, error) {
	query := `INSERT INTO arguments (player_id, topic, content) VALUES (?, ?, ?)`
	result, err := d.db.Exec(query, playerID, topic, content)
	if err != nil {
		return 0, fmt.Errorf("failed to save argument: %v", err)
	}

	return result.LastInsertId()
}

// SaveScore saves a score for an argument
func (d *Database) SaveScore(argumentID int64, score *scoring.ArgumentScore) error {
	query := `INSERT INTO scores (argument_id, strength, relevance, logic, truth, humor, average, explanation)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := d.db.Exec(query, argumentID, score.Strength, score.Relevance, score.Logic,
		score.Truth, score.Humor, score.Average, score.Explanation)

	if err != nil {
		return fmt.Errorf("failed to save score: %v", err)
	}

	return nil
}

// GetArgumentWithScore retrieves an argument and its score by ID
func (d *Database) GetArgumentWithScore(id int64) (*Argument, error) {
	query := `
		SELECT a.id, a.player_id, a.topic, a.content, a.created_at,
			   s.strength, s.relevance, s.logic, s.truth, s.humor, s.average, s.explanation
		FROM arguments a
		LEFT JOIN scores s ON a.id = s.argument_id
		WHERE a.id = ?`

	var arg Argument
	var score scoring.ArgumentScore

	err := d.db.QueryRow(query, id).Scan(
		&arg.ID, &arg.PlayerID, &arg.Topic, &arg.Content, &arg.CreatedAt,
		&score.Strength, &score.Relevance, &score.Logic, &score.Truth, &score.Humor,
		&score.Average, &score.Explanation,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("argument not found")
	} else if err != nil {
		return nil, fmt.Errorf("failed to get argument: %v", err)
	}

	arg.Score = &score
	return &arg, nil
}

// GetAllArguments retrieves all arguments with their scores
func (d *Database) GetAllArguments() ([]*Argument, error) {
	query := `
		SELECT a.id, a.player_id, a.topic, a.content, a.created_at,
			   s.strength, s.relevance, s.logic, s.truth, s.humor, s.average, s.explanation
		FROM arguments a
		LEFT JOIN scores s ON a.id = s.argument_id
		ORDER BY a.created_at DESC
		LIMIT 100`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query arguments: %v", err)
	}
	defer rows.Close()

	var arguments []*Argument
	for rows.Next() {
		var arg Argument
		var score scoring.ArgumentScore

		err := rows.Scan(
			&arg.ID, &arg.PlayerID, &arg.Topic, &arg.Content, &arg.CreatedAt,
			&score.Strength, &score.Relevance, &score.Logic, &score.Truth, &score.Humor,
			&score.Average, &score.Explanation,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan argument: %v", err)
		}

		arg.Score = &score
		arguments = append(arguments, &arg)
	}

	return arguments, nil
}
