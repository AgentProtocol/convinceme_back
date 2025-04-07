-- Enhance users table with additional security and role features

-- Add role column to users table
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';

-- Add last_login column to track login activity
ALTER TABLE users ADD COLUMN last_login TIMESTAMP;

-- Add failed_login_attempts for security
ALTER TABLE users ADD COLUMN failed_login_attempts INTEGER DEFAULT 0;

-- Add account_locked column for security
ALTER TABLE users ADD COLUMN account_locked BOOLEAN DEFAULT FALSE;

-- Add email_verified column for email verification
ALTER TABLE users ADD COLUMN email_verified BOOLEAN DEFAULT FALSE;

-- Add verification_token for email verification
ALTER TABLE users ADD COLUMN verification_token TEXT;

-- Add reset_token for password reset
ALTER TABLE users ADD COLUMN reset_token TEXT;

-- Add reset_token_expires for password reset token expiration
ALTER TABLE users ADD COLUMN reset_token_expires TIMESTAMP;

-- Create index on role for faster role-based queries
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

-- Create refresh_tokens table for JWT refresh tokens
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create index on token for faster lookups
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token ON refresh_tokens(token);

-- Create index on user_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
