-- Add topics table for pre-generated debate topics

-- Topics table stores pre-generated debate topics with agent pairings
CREATE TABLE IF NOT EXISTS topics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,                  -- The debate topic/question
    description TEXT,                     -- Optional longer description
    agent1_name TEXT NOT NULL,            -- Name for the first agent
    agent1_role TEXT NOT NULL,            -- Role/position for the first agent
    agent2_name TEXT NOT NULL,            -- Name for the second agent
    agent2_role TEXT NOT NULL,            -- Role/position for the second agent
    category TEXT,                        -- Optional category (e.g., "crypto", "technology", "animals")
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
