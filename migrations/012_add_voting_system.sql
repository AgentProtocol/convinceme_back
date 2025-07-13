-- Add voting system for arguments

-- Create votes table to track user votes on arguments
CREATE TABLE IF NOT EXISTS votes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    argument_id INTEGER NOT NULL,
    debate_id TEXT NOT NULL,
    vote_type TEXT NOT NULL CHECK (vote_type IN ('upvote', 'downvote')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (argument_id) REFERENCES arguments(id) ON DELETE CASCADE,
    FOREIGN KEY (debate_id) REFERENCES debates(id) ON DELETE CASCADE
);

-- Ensure a user can only vote once per argument
CREATE UNIQUE INDEX IF NOT EXISTS idx_votes_user_argument ON votes(user_id, argument_id);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_votes_user_debate ON votes(user_id, debate_id);
CREATE INDEX IF NOT EXISTS idx_votes_argument ON votes(argument_id);
CREATE INDEX IF NOT EXISTS idx_votes_debate ON votes(debate_id);

-- Add vote tracking columns to arguments table
ALTER TABLE arguments ADD COLUMN upvotes INTEGER DEFAULT 0;
ALTER TABLE arguments ADD COLUMN downvotes INTEGER DEFAULT 0;
ALTER TABLE arguments ADD COLUMN vote_score REAL DEFAULT 0.0; -- Net impact on argument score

-- Create indexes for vote columns
CREATE INDEX IF NOT EXISTS idx_arguments_vote_score ON arguments(vote_score);

-- Create a view for user vote counts per debate to enforce 3-vote limit
CREATE VIEW IF NOT EXISTS user_vote_counts AS
SELECT 
    user_id,
    debate_id,
    COUNT(*) as vote_count
FROM votes
GROUP BY user_id, debate_id; 