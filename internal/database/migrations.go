package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Migration represents a database migration
type Migration struct {
	ID        int
	Name      string
	SQL       string
	Timestamp time.Time
}

// MigrationRecord represents a record of a migration that has been applied
type MigrationRecord struct {
	ID        int
	Name      string
	AppliedAt time.Time
}

// MigrationManager handles database migrations
type MigrationManager struct {
	db *sql.DB
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *sql.DB) *MigrationManager {
	return &MigrationManager{
		db: db,
	}
}

// Initialize creates the migrations table if it doesn't exist
func (m *MigrationManager) Initialize() error {
	query := `
	CREATE TABLE IF NOT EXISTS migrations (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := m.db.Exec(query)
	return err
}

// LoadMigrations loads migrations from the specified directory
func (m *MigrationManager) LoadMigrations(migrationsDir string) ([]Migration, error) {
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %v", err)
	}

	var migrations []Migration
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		// Parse migration ID and name from filename
		// Expected format: 001_create_tables.sql
		parts := strings.SplitN(file.Name(), "_", 2)
		if len(parts) != 2 {
			log.Printf("Skipping migration file with invalid name format: %s", file.Name())
			continue
		}

		id := 0
		_, err := fmt.Sscanf(parts[0], "%d", &id)
		if err != nil {
			log.Printf("Skipping migration file with invalid ID: %s", file.Name())
			continue
		}

		name := strings.TrimSuffix(parts[1], ".sql")

		// Read migration SQL
		content, err := os.ReadFile(filepath.Join(migrationsDir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read migration file %s: %v", file.Name(), err)
		}

		migrations = append(migrations, Migration{
			ID:        id,
			Name:      name,
			SQL:       string(content),
			Timestamp: time.Now(),
		})
	}

	// Sort migrations by ID
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].ID < migrations[j].ID
	})

	return migrations, nil
}

// GetAppliedMigrations returns a list of migrations that have been applied
func (m *MigrationManager) GetAppliedMigrations() ([]MigrationRecord, error) {
	rows, err := m.db.Query("SELECT id, name, applied_at FROM migrations ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %v", err)
	}
	defer rows.Close()

	var migrations []MigrationRecord
	for rows.Next() {
		var migration MigrationRecord
		err := rows.Scan(&migration.ID, &migration.Name, &migration.AppliedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan migration row: %v", err)
		}
		migrations = append(migrations, migration)
	}

	return migrations, nil
}

// ApplyMigration applies a single migration
func (m *MigrationManager) ApplyMigration(migration Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	// Apply the migration
	_, err = tx.Exec(migration.SQL)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to apply migration %d_%s: %v", migration.ID, migration.Name, err)
	}

	// Record the migration
	_, err = tx.Exec("INSERT INTO migrations (id, name) VALUES (?, ?)", migration.ID, migration.Name)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to record migration %d_%s: %v", migration.ID, migration.Name, err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// MigrateUp applies all pending migrations
func (m *MigrationManager) MigrateUp(migrationsDir string) error {
	err := m.Initialize()
	if err != nil {
		return fmt.Errorf("failed to initialize migrations table: %v", err)
	}

	migrations, err := m.LoadMigrations(migrationsDir)
	if err != nil {
		return err
	}

	appliedMigrations, err := m.GetAppliedMigrations()
	if err != nil {
		return err
	}

	// Create a map of applied migration IDs for quick lookup
	appliedMap := make(map[int]bool)
	for _, migration := range appliedMigrations {
		appliedMap[migration.ID] = true
	}

	// Apply pending migrations
	for _, migration := range migrations {
		if !appliedMap[migration.ID] {
			log.Printf("Applying migration %d_%s...", migration.ID, migration.Name)
			err := m.ApplyMigration(migration)
			if err != nil {
				return err
			}
			log.Printf("Migration %d_%s applied successfully", migration.ID, migration.Name)
		} else {
			log.Printf("Migration %d_%s already applied, skipping", migration.ID, migration.Name)
		}
	}

	return nil
}
