package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// UserRole defines the role of a user
type UserRole string

// User roles
const (
	RoleAdmin     UserRole = "admin"
	RoleModerator UserRole = "moderator"
	RoleUser      UserRole = "user"
)

// User represents a user in the database
type User struct {
	ID                  string     `json:"id"`
	Username            string     `json:"username"`
	Email               string     `json:"email"`
	PasswordHash        string     `json:"-"` // Don't include in JSON
	Role                UserRole   `json:"role"`
	LastLogin           *time.Time `json:"last_login,omitempty"`
	FailedLoginAttempts int        `json:"-"`
	AccountLocked       bool       `json:"account_locked"`
	EmailVerified       bool       `json:"email_verified"`
	VerificationToken   *string    `json:"-"`
	ResetToken          *string    `json:"-"`
	ResetTokenExpires   *time.Time `json:"-"`
	InvitationCode      string     `json:"invitation_code,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// RefreshToken represents a JWT refresh token in the database
type RefreshToken struct {
	ID        int       `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// generateRandomToken generates a secure random token of the specified length
func generateRandomToken(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		// In case of error, return a fallback (though this should never happen)
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.URLEncoding.EncodeToString(b)
}

// CreateUser creates a new user
func (d *Database) CreateUser(user *User, password string) error {
	// Hash the password with appropriate cost factor
	costFactor := 12 // Higher cost factor for better security
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), costFactor)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Set the password hash
	user.PasswordHash = string(passwordHash)

	// Generate verification token if email is not verified
	if !user.EmailVerified {
		token := generateRandomToken(32)
		user.VerificationToken = &token
	}

	// Set default role if not specified
	if user.Role == "" {
		user.Role = RoleUser
	}

	// Insert the user into the database
	query := `INSERT INTO users (
		id, username, email, password_hash, role, account_locked,
		email_verified, verification_token, failed_login_attempts, invitation_code
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = d.db.Exec(
		query,
		user.ID,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.AccountLocked,
		user.EmailVerified,
		user.VerificationToken,
		user.FailedLoginAttempts,
		user.InvitationCode,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	return nil
}

// GetUserByID gets a user by ID
func (d *Database) GetUserByID(id string) (*User, error) {
	query := `SELECT
		id, username, email, password_hash, role, last_login,
		failed_login_attempts, account_locked, email_verified,
		verification_token, reset_token, reset_token_expires,
		invitation_code, created_at, updated_at
	FROM users WHERE id = ?`

	var user User
	var lastLogin, resetTokenExpires sql.NullTime
	var verificationToken, resetToken, invitationCode sql.NullString
	var role string

	err := d.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&role,
		&lastLogin,
		&user.FailedLoginAttempts,
		&user.AccountLocked,
		&user.EmailVerified,
		&verificationToken,
		&resetToken,
		&resetTokenExpires,
		&invitationCode,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	// Convert role string to UserRole
	user.Role = UserRole(role)

	// Handle nullable fields
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	if verificationToken.Valid {
		user.VerificationToken = &verificationToken.String
	}

	if resetToken.Valid {
		user.ResetToken = &resetToken.String
	}

	if resetTokenExpires.Valid {
		user.ResetTokenExpires = &resetTokenExpires.Time
	}

	if invitationCode.Valid {
		user.InvitationCode = invitationCode.String
	}

	return &user, nil
}

// GetUserByUsername gets a user by username
func (d *Database) GetUserByUsername(username string) (*User, error) {
	// Reuse the same query structure as GetUserByID but with a different WHERE clause
	query := `SELECT
		id, username, email, password_hash, role, last_login,
		failed_login_attempts, account_locked, email_verified,
		verification_token, reset_token, reset_token_expires,
		invitation_code, created_at, updated_at
	FROM users WHERE username = ?`

	var user User
	var lastLogin, resetTokenExpires sql.NullTime
	var verificationToken, resetToken, invitationCode sql.NullString
	var role string

	err := d.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&role,
		&lastLogin,
		&user.FailedLoginAttempts,
		&user.AccountLocked,
		&user.EmailVerified,
		&verificationToken,
		&resetToken,
		&resetTokenExpires,
		&invitationCode,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with username %s not found", username)
		}
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	// Convert role string to UserRole
	user.Role = UserRole(role)

	// Handle nullable fields
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	if verificationToken.Valid {
		user.VerificationToken = &verificationToken.String
	}

	if resetToken.Valid {
		user.ResetToken = &resetToken.String
	}

	if resetTokenExpires.Valid {
		user.ResetTokenExpires = &resetTokenExpires.Time
	}

	if invitationCode.Valid {
		user.InvitationCode = invitationCode.String
	}

	return &user, nil
}

// GetUserByEmail gets a user by email
func (d *Database) GetUserByEmail(email string) (*User, error) {
	// Reuse the same query structure as GetUserByID but with a different WHERE clause
	query := `SELECT
		id, username, email, password_hash, role, last_login,
		failed_login_attempts, account_locked, email_verified,
		verification_token, reset_token, reset_token_expires,
		invitation_code, created_at, updated_at
	FROM users WHERE email = ?`

	var user User
	var lastLogin, resetTokenExpires sql.NullTime
	var verificationToken, resetToken, invitationCode sql.NullString
	var role string

	err := d.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&role,
		&lastLogin,
		&user.FailedLoginAttempts,
		&user.AccountLocked,
		&user.EmailVerified,
		&verificationToken,
		&resetToken,
		&resetTokenExpires,
		&invitationCode,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with email %s not found", email)
		}
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	// Convert role string to UserRole
	user.Role = UserRole(role)

	// Handle nullable fields
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	if verificationToken.Valid {
		user.VerificationToken = &verificationToken.String
	}

	if resetToken.Valid {
		user.ResetToken = &resetToken.String
	}

	if resetTokenExpires.Valid {
		user.ResetTokenExpires = &resetTokenExpires.Time
	}

	if invitationCode.Valid {
		user.InvitationCode = invitationCode.String
	}

	return &user, nil
}

// UpdateUser updates a user
func (d *Database) UpdateUser(user *User) error {
	query := `UPDATE users SET
		username = ?,
		email = ?,
		role = ?,
		account_locked = ?,
		email_verified = ?,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?`

	_, err := d.db.Exec(
		query,
		user.Username,
		user.Email,
		user.Role,
		user.AccountLocked,
		user.EmailVerified,
		user.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}

	return nil
}

// UpdatePassword updates a user's password
func (d *Database) UpdatePassword(userID string, password string) error {
	// Hash the password with appropriate cost factor
	costFactor := 12 // Higher cost factor for better security
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), costFactor)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Update the password hash and reset failed login attempts
	query := `UPDATE users SET
		password_hash = ?,
		failed_login_attempts = 0,
		account_locked = FALSE,
		reset_token = NULL,
		reset_token_expires = NULL,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?`

	_, err = d.db.Exec(query, string(passwordHash), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %v", err)
	}

	return nil
}

// DeleteUser deletes a user
func (d *Database) DeleteUser(id string) error {
	// Start a transaction to ensure both operations succeed or fail together
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	// Delete refresh tokens first (due to foreign key constraint)
	_, err = tx.Exec("DELETE FROM refresh_tokens WHERE user_id = ?", id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete refresh tokens: %v", err)
	}

	// Delete the user
	_, err = tx.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete user: %v", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// CreateRefreshToken creates a new refresh token for a user
func (d *Database) CreateRefreshToken(userID string, token string, expiresAt time.Time) error {
	query := `INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES (?, ?, ?)`
	_, err := d.db.Exec(query, userID, token, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to create refresh token: %v", err)
	}

	return nil
}

// GetRefreshToken gets a refresh token by token string
func (d *Database) GetRefreshToken(token string) (*RefreshToken, error) {
	query := `SELECT id, user_id, token, expires_at, created_at FROM refresh_tokens WHERE token = ?`
	var refreshToken RefreshToken

	err := d.db.QueryRow(query, token).Scan(
		&refreshToken.ID,
		&refreshToken.UserID,
		&refreshToken.Token,
		&refreshToken.ExpiresAt,
		&refreshToken.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("refresh token not found")
		}
		return nil, fmt.Errorf("failed to get refresh token: %v", err)
	}

	// Check if token is expired
	if time.Now().After(refreshToken.ExpiresAt) {
		// Delete the expired token
		d.DeleteRefreshToken(token)
		return nil, fmt.Errorf("refresh token has expired")
	}

	return &refreshToken, nil
}

// DeleteRefreshToken deletes a refresh token
func (d *Database) DeleteRefreshToken(token string) error {
	query := `DELETE FROM refresh_tokens WHERE token = ?`
	_, err := d.db.Exec(query, token)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token: %v", err)
	}

	return nil
}

// DeleteUserRefreshTokens deletes all refresh tokens for a user
func (d *Database) DeleteUserRefreshTokens(userID string) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = ?`
	_, err := d.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user refresh tokens: %v", err)
	}

	return nil
}

// CleanupExpiredTokens removes all expired refresh tokens
func (d *Database) CleanupExpiredTokens() error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < ?`
	_, err := d.db.Exec(query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup expired tokens: %v", err)
	}

	return nil
}

// CreatePasswordResetToken creates a password reset token for a user
func (d *Database) CreatePasswordResetToken(email string) (string, error) {
	// Get the user by email
	user, err := d.GetUserByEmail(email)
	if err != nil {
		return "", err
	}

	// Generate a reset token
	resetToken := generateRandomToken(32)

	// Set expiration time (24 hours from now)
	expiresAt := time.Now().Add(24 * time.Hour)

	// Update the user record
	query := `UPDATE users SET
		reset_token = ?,
		reset_token_expires = ?,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?`

	_, err = d.db.Exec(query, resetToken, expiresAt, user.ID)
	if err != nil {
		return "", fmt.Errorf("failed to create password reset token: %v", err)
	}

	return resetToken, nil
}

// VerifyPasswordResetToken verifies a password reset token
func (d *Database) VerifyPasswordResetToken(token string) (*User, error) {
	// Find user with this reset token
	query := `SELECT
		id, username, email, password_hash, role, last_login,
		failed_login_attempts, account_locked, email_verified,
		verification_token, reset_token, reset_token_expires,
		created_at, updated_at
	FROM users WHERE reset_token = ?`

	var user User
	var lastLogin, resetTokenExpires sql.NullTime
	var verificationToken, resetToken sql.NullString
	var role string

	err := d.db.QueryRow(query, token).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&role,
		&lastLogin,
		&user.FailedLoginAttempts,
		&user.AccountLocked,
		&user.EmailVerified,
		&verificationToken,
		&resetToken,
		&resetTokenExpires,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid reset token")
		}
		return nil, fmt.Errorf("failed to verify reset token: %v", err)
	}

	// Convert role string to UserRole
	user.Role = UserRole(role)

	// Handle nullable fields
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	if verificationToken.Valid {
		user.VerificationToken = &verificationToken.String
	}

	if resetToken.Valid {
		user.ResetToken = &resetToken.String
	}

	if resetTokenExpires.Valid {
		user.ResetTokenExpires = &resetTokenExpires.Time
	}

	// Check if token has expired
	if user.ResetTokenExpires == nil || time.Now().After(*user.ResetTokenExpires) {
		// Clear the expired token
		clearQuery := `UPDATE users SET
			reset_token = NULL,
			reset_token_expires = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`

		_, err = d.db.Exec(clearQuery, user.ID)
		if err != nil {
			// Just log the error
			fmt.Printf("Failed to clear expired reset token: %v\n", err)
		}

		return nil, fmt.Errorf("reset token has expired")
	}

	return &user, nil
}

// ResetPassword resets a user's password using a reset token
func (d *Database) ResetPassword(token, newPassword string) error {
	// Verify the token first
	user, err := d.VerifyPasswordResetToken(token)
	if err != nil {
		return err
	}

	// Update the password
	err = d.UpdatePassword(user.ID, newPassword)
	if err != nil {
		return err
	}

	// Clear the reset token
	clearQuery := `UPDATE users SET
		reset_token = NULL,
		reset_token_expires = NULL,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?`

	_, err = d.db.Exec(clearQuery, user.ID)
	if err != nil {
		// Just log the error
		fmt.Printf("Failed to clear reset token: %v\n", err)
	}

	return nil
}

// VerifyEmail verifies a user's email address using a verification token
func (d *Database) VerifyEmail(token string) error {
	// Find user with this verification token
	query := `SELECT id FROM users WHERE verification_token = ?`

	var userID string
	err := d.db.QueryRow(query, token).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("invalid verification token")
		}
		return fmt.Errorf("failed to verify email: %v", err)
	}

	// Update the user record
	updateQuery := `UPDATE users SET
		email_verified = TRUE,
		verification_token = NULL,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?`

	_, err = d.db.Exec(updateQuery, userID)
	if err != nil {
		return fmt.Errorf("failed to update email verification status: %v", err)
	}

	return nil
}

// ResendVerificationEmail generates a new verification token for a user
func (d *Database) ResendVerificationEmail(email string) (string, error) {
	// Get the user by email
	user, err := d.GetUserByEmail(email)
	if err != nil {
		return "", err
	}

	// Check if email is already verified
	if user.EmailVerified {
		return "", fmt.Errorf("email is already verified")
	}

	// Generate a new verification token
	token := generateRandomToken(32)

	// Update the user record
	query := `UPDATE users SET
		verification_token = ?,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?`

	_, err = d.db.Exec(query, token, user.ID)
	if err != nil {
		return "", fmt.Errorf("failed to update verification token: %v", err)
	}

	return token, nil
}

// VerifyPassword verifies a user's password
func (d *Database) VerifyPassword(username, password string) (*User, error) {
	// Get the user
	user, err := d.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}

	// Check if account is locked
	if user.AccountLocked {
		return nil, fmt.Errorf("account is locked due to too many failed login attempts")
	}

	// Check if email is verified (if required)
	if !user.EmailVerified {
		return nil, fmt.Errorf("email address has not been verified")
	}

	// Verify the password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		// Increment failed login attempts
		user.FailedLoginAttempts++

		// Lock account after 5 failed attempts
		if user.FailedLoginAttempts >= 5 {
			user.AccountLocked = true
		}

		// Update the user record
		updateQuery := `UPDATE users SET
			failed_login_attempts = ?,
			account_locked = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`

		_, updateErr := d.db.Exec(
			updateQuery,
			user.FailedLoginAttempts,
			user.AccountLocked,
			user.ID,
		)

		if updateErr != nil {
			// Log the error but don't return it to avoid leaking information
			fmt.Printf("Failed to update login attempts: %v\n", updateErr)
		}

		return nil, fmt.Errorf("invalid password")
	}

	// Password is correct, reset failed login attempts and update last login time
	now := time.Now()
	user.LastLogin = &now
	user.FailedLoginAttempts = 0

	updateQuery := `UPDATE users SET
		failed_login_attempts = 0,
		last_login = ?,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?`

	_, updateErr := d.db.Exec(updateQuery, now, user.ID)
	if updateErr != nil {
		// Log the error but don't return it
		fmt.Printf("Failed to update last login: %v\n", updateErr)
	}

	return user, nil
}
