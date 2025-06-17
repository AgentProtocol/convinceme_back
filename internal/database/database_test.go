package database

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	// Skip this test for now as it requires migrations
	t.Skip("Skipping test that requires migrations")

	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "database_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test creating a new database
	db, err := New(tempDir)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Check if the database file was created
	dbPath := filepath.Join(tempDir, "arguments.db")
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestTopicFilter(t *testing.T) {
	// Skip this test for now as it requires migrations
	t.Skip("Skipping test that requires migrations")

	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "database_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a new database
	db, err := New(tempDir)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Create a test filter
	filter := TopicFilter{
		Category: "test",
		Search:   "search",
		SortBy:   "title",
		SortDir:  "asc",
		Offset:   10,
		Limit:    20,
	}

	// Test the filter
	topics, total, err := db.GetTopics(filter)
	assert.NoError(t, err)
	assert.NotNil(t, topics)
	assert.GreaterOrEqual(t, total, 0)
}

func TestDebateFilter(t *testing.T) {
	// Skip this test for now as it requires migrations
	t.Skip("Skipping test that requires migrations")

	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "database_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a new database
	db, err := New(tempDir)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Create a test filter
	filter := DebateFilter{
		Status:  "active",
		Search:  "search",
		SortBy:  "created_at",
		SortDir: "desc",
		Offset:  0,
		Limit:   10,
	}

	// Test the filter
	debates, total, err := db.ListDebates(filter)
	assert.NoError(t, err)
	assert.NotNil(t, debates)
	assert.GreaterOrEqual(t, total, 0)
}
