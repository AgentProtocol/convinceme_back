-- Create invitation codes table for closed alpha
CREATE TABLE IF NOT EXISTS invitation_codes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    created_by TEXT,
    email TEXT,
    used BOOLEAN DEFAULT FALSE,
    used_by TEXT,
    used_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (used_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Create index on code for faster lookups
CREATE INDEX IF NOT EXISTS idx_invitation_codes_code ON invitation_codes(code);

-- Create index on email for faster lookups
CREATE INDEX IF NOT EXISTS idx_invitation_codes_email ON invitation_codes(email);

-- Create index on created_by for faster lookups
CREATE INDEX IF NOT EXISTS idx_invitation_codes_created_by ON invitation_codes(created_by);

-- Add invitation_code column to users table
ALTER TABLE users ADD COLUMN invitation_code TEXT;

-- Create index on invitation_code for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_invitation_code ON users(invitation_code);
