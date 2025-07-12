-- Add wallet address support to users table

-- Add wallet_address column to users table
ALTER TABLE users ADD COLUMN wallet_address TEXT;

-- Add wallet_verified column to track wallet verification status
ALTER TABLE users ADD COLUMN wallet_verified BOOLEAN DEFAULT FALSE;

-- Add wallet_connected_at timestamp to track when wallet was connected
ALTER TABLE users ADD COLUMN wallet_connected_at TIMESTAMP;

-- Create index on wallet_address for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_wallet_address ON users(wallet_address);

-- Add unique constraint on wallet_address to prevent duplicate wallets
-- Note: Allow NULL values since not all users may have wallets
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_wallet_address_unique 
ON users(wallet_address) WHERE wallet_address IS NOT NULL;

-- Update arguments table to optionally track wallet addresses
ALTER TABLE arguments ADD COLUMN wallet_address TEXT;
CREATE INDEX IF NOT EXISTS idx_arguments_wallet_address ON arguments(wallet_address);

-- Update debates table to optionally track creator's wallet
ALTER TABLE debates ADD COLUMN creator_wallet_address TEXT;
CREATE INDEX IF NOT EXISTS idx_debates_creator_wallet ON debates(creator_wallet_address); 