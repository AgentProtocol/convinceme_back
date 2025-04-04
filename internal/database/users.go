package database

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the database
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Don't include in JSON
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateUser creates a new user
func (d *Database) CreateUser(user *User, password string) error {
	// Hash the password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Set the password hash
	user.PasswordHash = string(passwordHash)

	// Insert the user into the database
	query := `INSERT INTO users (id, username, email, password_hash) VALUES (?, ?, ?, ?)`
	_, err = d.db.Exec(query, user.ID, user.Username, user.Email, user.PasswordHash)
	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	return nil
}

// GetUserByID gets a user by ID
func (d *Database) GetUserByID(id string) (*User, error) {
	query := `SELECT id, username, email, password_hash, created_at, updated_at FROM users WHERE id = ?`
	var user User
	err := d.db.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	return &user, nil
}

// GetUserByUsername gets a user by username
func (d *Database) GetUserByUsername(username string) (*User, error) {
	query := `SELECT id, username, email, password_hash, created_at, updated_at FROM users WHERE username = ?`
	var user User
	err := d.db.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with username %s not found", username)
		}
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	return &user, nil
}

// GetUserByEmail gets a user by email
func (d *Database) GetUserByEmail(email string) (*User, error) {
	query := `SELECT id, username, email, password_hash, created_at, updated_at FROM users WHERE email = ?`
	var user User
	err := d.db.QueryRow(query, email).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with email %s not found", email)
		}
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	return &user, nil
}

// UpdateUser updates a user
func (d *Database) UpdateUser(user *User) error {
	query := `UPDATE users SET username = ?, email = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := d.db.Exec(query, user.Username, user.Email, user.ID)
	if err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}

	return nil
}

// UpdatePassword updates a user's password
func (d *Database) UpdatePassword(userID string, password string) error {
	// Hash the password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Update the password hash
	query := `UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err = d.db.Exec(query, string(passwordHash), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %v", err)
	}

	return nil
}

// DeleteUser deletes a user
func (d *Database) DeleteUser(id string) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
	}

	return nil
}

// VerifyPassword verifies a user's password
func (d *Database) VerifyPassword(username, password string) (*User, error) {
	// Get the user
	user, err := d.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}

	// Verify the password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	return user, nil
}
