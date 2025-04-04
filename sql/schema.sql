-- Arguments table stores player arguments
CREATE TABLE IF NOT EXISTS arguments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,  -- Identifier for the player
    topic TEXT NOT NULL,      -- Topic of the argument
    content TEXT NOT NULL,    -- The actual argument text
    side TEXT NOT NULL,       -- Which side the argument supports
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Scores table stores argument scores
CREATE TABLE IF NOT EXISTS scores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    argument_id INTEGER NOT NULL,
    strength INTEGER NOT NULL CHECK (strength BETWEEN 0 AND 100),
    relevance INTEGER NOT NULL CHECK (relevance BETWEEN 0 AND 100),
    logic INTEGER NOT NULL CHECK (logic BETWEEN 0 AND 100),
    truth INTEGER NOT NULL CHECK (truth BETWEEN 0 AND 100),
    humor INTEGER NOT NULL CHECK (humor BETWEEN 0 AND 100),
    average REAL NOT NULL,
    explanation TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (argument_id) REFERENCES arguments(id) ON DELETE CASCADE
);

-- Debates table stores information about each debate session
CREATE TABLE IF NOT EXISTS debates (
    id TEXT PRIMARY KEY,          -- Unique identifier (e.g., UUID)
    topic TEXT NOT NULL,
    status TEXT NOT NULL,         -- e.g., 'waiting', 'active', 'finished'
    agent1_name TEXT NOT NULL,
    agent2_name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP NULL,
    winner TEXT NULL              -- Name of the winning agent/side
);

-- Add debate_id column to arguments table
-- Note: Adding foreign key constraint might require more complex migration steps
-- if data already exists. For simplicity, we'll omit the constraint for now.
ALTER TABLE arguments ADD COLUMN debate_id TEXT;

-- Add debate_id column to scores table
ALTER TABLE scores ADD COLUMN debate_id TEXT;
