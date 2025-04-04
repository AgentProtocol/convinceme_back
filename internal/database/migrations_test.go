package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) (*sql.DB, string, func()) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "migrations_test")
	assert.NoError(t, err)

	// Create a test database
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(t, err)

	// Return the database, path, and cleanup function
	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, tempDir, cleanup
}

func TestMigrationManagerInitialize(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a migration manager
	manager := NewMigrationManager(db)

	// Initialize the migrations table
	err := manager.Initialize()
	assert.NoError(t, err)

	// Check if the migrations table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='migrations'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestLoadMigrations(t *testing.T) {
	db, tempDir, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a migrations directory
	migrationsDir := filepath.Join(tempDir, "migrations")
	err := os.Mkdir(migrationsDir, 0755)
	assert.NoError(t, err)

	// Create test migration files
	testMigrations := []struct {
		filename string
		content  string
	}{
		{"001_create_test_table.sql", "CREATE TABLE test (id INTEGER PRIMARY KEY);"},
		{"002_add_test_column.sql", "ALTER TABLE test ADD COLUMN name TEXT;"},
		{"invalid_migration.sql", "This should be skipped"},
	}

	for _, m := range testMigrations {
		err := os.WriteFile(filepath.Join(migrationsDir, m.filename), []byte(m.content), 0644)
		assert.NoError(t, err)
	}

	// Create a migration manager
	manager := NewMigrationManager(db)

	// Load migrations
	migrations, err := manager.LoadMigrations(migrationsDir)
	assert.NoError(t, err)

	// Check if the correct migrations were loaded
	assert.Equal(t, 2, len(migrations))
	assert.Equal(t, 1, migrations[0].ID)
	assert.Equal(t, "create_test_table", migrations[0].Name)
	assert.Equal(t, "CREATE TABLE test (id INTEGER PRIMARY KEY);", migrations[0].SQL)
	assert.Equal(t, 2, migrations[1].ID)
	assert.Equal(t, "add_test_column", migrations[1].Name)
	assert.Equal(t, "ALTER TABLE test ADD COLUMN name TEXT;", migrations[1].SQL)
}

func TestGetAppliedMigrations(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a migration manager
	manager := NewMigrationManager(db)

	// Initialize the migrations table
	err := manager.Initialize()
	assert.NoError(t, err)

	// Insert some test migrations
	_, err = db.Exec("INSERT INTO migrations (id, name) VALUES (1, 'create_test_table')")
	assert.NoError(t, err)
	_, err = db.Exec("INSERT INTO migrations (id, name) VALUES (2, 'add_test_column')")
	assert.NoError(t, err)

	// Get applied migrations
	migrations, err := manager.GetAppliedMigrations()
	assert.NoError(t, err)

	// Check if the correct migrations were returned
	assert.Equal(t, 2, len(migrations))
	assert.Equal(t, 1, migrations[0].ID)
	assert.Equal(t, "create_test_table", migrations[0].Name)
	assert.Equal(t, 2, migrations[1].ID)
	assert.Equal(t, "add_test_column", migrations[1].Name)
}

func TestApplyMigration(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a migration manager
	manager := NewMigrationManager(db)

	// Initialize the migrations table
	err := manager.Initialize()
	assert.NoError(t, err)

	// Create a test migration
	migration := Migration{
		ID:        1,
		Name:      "create_test_table",
		SQL:       "CREATE TABLE test (id INTEGER PRIMARY KEY);",
		Timestamp: time.Now(),
	}

	// Apply the migration
	err = manager.ApplyMigration(migration)
	assert.NoError(t, err)

	// Check if the migration was applied
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Check if the migration was recorded
	var id int
	var name string
	err = db.QueryRow("SELECT id, name FROM migrations WHERE id = 1").Scan(&id, &name)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	assert.Equal(t, "create_test_table", name)
}

func TestMigrateUp(t *testing.T) {
	db, tempDir, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a migrations directory
	migrationsDir := filepath.Join(tempDir, "migrations")
	err := os.Mkdir(migrationsDir, 0755)
	assert.NoError(t, err)

	// Create test migration files
	testMigrations := []struct {
		filename string
		content  string
	}{
		{"001_create_test_table.sql", "CREATE TABLE test (id INTEGER PRIMARY KEY);"},
		{"002_add_test_column.sql", "ALTER TABLE test ADD COLUMN name TEXT;"},
	}

	for _, m := range testMigrations {
		err := os.WriteFile(filepath.Join(migrationsDir, m.filename), []byte(m.content), 0644)
		assert.NoError(t, err)
	}

	// Create a migration manager
	manager := NewMigrationManager(db)

	// Run migrations
	err = manager.MigrateUp(migrationsDir)
	assert.NoError(t, err)

	// Check if the migrations were applied
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Check if the column was added
	var columnCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('test') WHERE name='name'").Scan(&columnCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, columnCount)

	// Check if the migrations were recorded
	var migrationCount int
	err = db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&migrationCount)
	assert.NoError(t, err)
	assert.Equal(t, 2, migrationCount)
}
