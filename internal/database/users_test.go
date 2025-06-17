package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupUsersTestDB creates a temporary database for testing users
func setupUsersTestDB(t *testing.T) (*Database, string, func()) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "users_test")
	require.NoError(t, err)

	// Create migrations directory
	migrationsDir := filepath.Join(tempDir, "migrations")
	err = os.MkdirAll(migrationsDir, 0755)
	require.NoError(t, err)

	// Create a simple migration file
	migrationContent := `CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		email_verified BOOLEAN NOT NULL DEFAULT 0,
		account_locked BOOLEAN NOT NULL DEFAULT 0,
		verification_token TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS refresh_tokens (
		token TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		expires_at TIMESTAMP NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS password_reset_tokens (
		token TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		expires_at TIMESTAMP NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS invitation_codes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT UNIQUE NOT NULL,
		created_by TEXT NOT NULL,
		email TEXT,
		used BOOLEAN NOT NULL DEFAULT 0,
		used_by TEXT,
		used_at TIMESTAMP,
		expires_at TIMESTAMP NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (used_by) REFERENCES users(id) ON DELETE SET NULL
	);`

	err = os.WriteFile(filepath.Join(migrationsDir, "001_initial_schema.sql"), []byte(migrationContent), 0644)
	require.NoError(t, err)

	// Create a new database
	db, err := New(tempDir)
	require.NoError(t, err)

	// Return cleanup function
	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, tempDir, cleanup
}

// No longer needed as cleanup is handled by the returned function
// func teardownTestDB(tempDir string) {
// 	os.RemoveAll(tempDir)
// }

func TestCreateAndGetUser(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: true,
		AccountLocked: false,
	}

	// Test creating the user
	err := db.CreateUser(user, "password123")
	assert.NoError(t, err)

	// Test getting the user by ID
	retrievedUser, err := db.GetUserByID(user.ID)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, retrievedUser.ID)
	assert.Equal(t, user.Username, retrievedUser.Username)
	assert.Equal(t, user.Email, retrievedUser.Email)
	assert.Equal(t, user.Role, retrievedUser.Role)
	assert.Equal(t, user.EmailVerified, retrievedUser.EmailVerified)
	assert.Equal(t, user.AccountLocked, retrievedUser.AccountLocked)
	assert.NotEmpty(t, retrievedUser.PasswordHash)

	// Test getting the user by username
	retrievedUser, err = db.GetUserByUsername(user.Username)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, retrievedUser.ID)

	// Test getting the user by email
	retrievedUser, err = db.GetUserByEmail(user.Email)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, retrievedUser.ID)

	// Test getting a non-existent user
	_, err = db.GetUserByID("non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateUser(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: false,
		AccountLocked: false,
	}

	// Create the user
	err := db.CreateUser(user, "password123")
	assert.NoError(t, err)

	// Update the user
	user.Username = "updateduser"
	user.Email = "updated@example.com"
	user.Role = RoleModerator
	user.EmailVerified = true

	err = db.UpdateUser(user)
	assert.NoError(t, err)

	// Get the updated user
	retrievedUser, err := db.GetUserByID(user.ID)
	assert.NoError(t, err)
	assert.Equal(t, "updateduser", retrievedUser.Username)
	assert.Equal(t, "updated@example.com", retrievedUser.Email)
	assert.Equal(t, RoleModerator, retrievedUser.Role)
	assert.True(t, retrievedUser.EmailVerified)
}

func TestUpdatePassword(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the user with initial password
	initialPassword := "initialPassword123"
	err := db.CreateUser(user, initialPassword)
	assert.NoError(t, err)

	// Verify the initial password
	_, err = db.VerifyPassword(user.Username, initialPassword)
	assert.NoError(t, err)

	// Update the password
	newPassword := "newPassword456"
	err = db.UpdatePassword(user.ID, newPassword)
	assert.NoError(t, err)

	// Verify the new password works
	_, err = db.VerifyPassword(user.Username, newPassword)
	assert.NoError(t, err)

	// Verify the old password no longer works
	_, err = db.VerifyPassword(user.Username, initialPassword)
	assert.Error(t, err)
}

func TestVerifyPassword(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the user
	password := "correctPassword123"
	err := db.CreateUser(user, password)
	assert.NoError(t, err)

	// Test cases
	testCases := []struct {
		name          string
		username      string
		password      string
		expectedError bool
	}{
		{
			name:          "Correct credentials",
			username:      user.Username,
			password:      password,
			expectedError: false,
		},
		{
			name:          "Incorrect password",
			username:      user.Username,
			password:      "wrongPassword",
			expectedError: true,
		},
		{
			name:          "Non-existent user",
			username:      "nonexistentuser",
			password:      password,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.VerifyPassword(tc.username, tc.password)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the user
	err := db.CreateUser(user, "password123")
	assert.NoError(t, err)

	// Delete the user
	err = db.DeleteUser(user.ID)
	assert.NoError(t, err)

	// Try to get the deleted user
	_, err = db.GetUserByID(user.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRefreshTokenOperations(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the user
	err := db.CreateUser(user, "password123")
	assert.NoError(t, err)

	// Create a refresh token
	token := "test-refresh-token"
	expiresAt := time.Now().Add(24 * time.Hour)
	err = db.CreateRefreshToken(user.ID, token, expiresAt)
	assert.NoError(t, err)

	// Get the refresh token
	refreshToken, err := db.GetRefreshToken(token)
	assert.NoError(t, err)
	assert.Equal(t, token, refreshToken.Token)
	assert.Equal(t, user.ID, refreshToken.UserID)
	assert.True(t, refreshToken.ExpiresAt.After(time.Now()))

	// Delete the refresh token
	err = db.DeleteRefreshToken(token)
	assert.NoError(t, err)

	// Try to get the deleted token
	_, err = db.GetRefreshToken(token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Create multiple tokens for the user
	token1 := "test-token-1"
	token2 := "test-token-2"
	err = db.CreateRefreshToken(user.ID, token1, expiresAt)
	assert.NoError(t, err)
	err = db.CreateRefreshToken(user.ID, token2, expiresAt)
	assert.NoError(t, err)

	// Delete all tokens for the user
	err = db.DeleteUserRefreshTokens(user.ID)
	assert.NoError(t, err)

	// Try to get the deleted tokens
	_, err = db.GetRefreshToken(token1)
	assert.Error(t, err)
	_, err = db.GetRefreshToken(token2)
	assert.Error(t, err)
}

func TestPasswordResetOperations(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the user
	initialPassword := "initialPassword123"
	err := db.CreateUser(user, initialPassword)
	assert.NoError(t, err)

	// Create a password reset token
	resetToken, err := db.CreatePasswordResetToken(user.Email)
	assert.NoError(t, err)
	assert.NotEmpty(t, resetToken)

	// Verify the reset token
	retrievedUser, err := db.VerifyPasswordResetToken(resetToken)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, retrievedUser.ID)

	// Reset the password
	newPassword := "newPassword456"
	err = db.ResetPassword(resetToken, newPassword)
	assert.NoError(t, err)

	// Verify the new password works
	_, err = db.VerifyPassword(user.Username, newPassword)
	assert.NoError(t, err)

	// Verify the old password no longer works
	_, err = db.VerifyPassword(user.Username, initialPassword)
	assert.Error(t, err)

	// Try to use the reset token again (should fail)
	_, err = db.VerifyPasswordResetToken(resetToken)
	assert.Error(t, err)
}

func TestEmailVerification(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user with unverified email
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: false,
	}

	// Create the user
	err := db.CreateUser(user, "password123")
	assert.NoError(t, err)

	// Get the verification token
	retrievedUser, err := db.GetUserByID(user.ID)
	assert.NoError(t, err)
	assert.False(t, retrievedUser.EmailVerified)
	assert.NotNil(t, retrievedUser.VerificationToken)

	// Verify the email
	verificationToken := *retrievedUser.VerificationToken
	err = db.VerifyEmail(verificationToken)
	assert.NoError(t, err)

	// Check that the email is now verified
	retrievedUser, err = db.GetUserByID(user.ID)
	assert.NoError(t, err)
	assert.True(t, retrievedUser.EmailVerified)
	assert.Nil(t, retrievedUser.VerificationToken)

	// Try to verify with an invalid token
	err = db.VerifyEmail("invalid-token")
	assert.Error(t, err)
}

func TestResendVerificationEmail(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user with unverified email
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: false,
	}

	// Create the user
	err := db.CreateUser(user, "password123")
	assert.NoError(t, err)

	// Get the initial verification token
	retrievedUser, err := db.GetUserByID(user.ID)
	assert.NoError(t, err)
	initialToken := *retrievedUser.VerificationToken

	// Resend verification email
	newToken, err := db.ResendVerificationEmail(user.Email)
	assert.NoError(t, err)
	assert.NotEmpty(t, newToken)
	assert.NotEqual(t, initialToken, newToken)

	// Get the user again to check the new token
	retrievedUser, err = db.GetUserByID(user.ID)
	assert.NoError(t, err)
	assert.Equal(t, newToken, *retrievedUser.VerificationToken)

	// Try to resend for a verified email
	retrievedUser.EmailVerified = true
	err = db.UpdateUser(retrievedUser)
	assert.NoError(t, err)

	_, err = db.ResendVerificationEmail(user.Email)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already verified")
}
