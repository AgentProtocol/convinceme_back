-- Add missing indexes to improve query performance

-- Add indexes to arguments table
CREATE INDEX IF NOT EXISTS idx_arguments_debate_id ON arguments(debate_id);
CREATE INDEX IF NOT EXISTS idx_arguments_player_id ON arguments(player_id);

-- Add indexes to scores table
CREATE INDEX IF NOT EXISTS idx_scores_debate_id ON scores(debate_id);

-- Add indexes to topics table
CREATE INDEX IF NOT EXISTS idx_topics_category ON topics(category);

-- Add indexes to debates table
CREATE INDEX IF NOT EXISTS idx_debates_status ON debates(status);
