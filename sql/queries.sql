-- Check all arguments with their scores (formatted view)
.mode column
.headers on
SELECT 
    a.id,
    a.player_id,
    a.topic,
    a.content,
    s.strength,
    s.relevance,
    s.logic,
    s.truth,
    s.humor,
    s.average,
    substr(s.explanation, 1, 50) || '...' as explanation_preview,
    a.created_at
FROM arguments a
LEFT JOIN scores s ON a.id = s.argument_id
ORDER BY a.created_at DESC;

-- Get full explanation for a specific argument
SELECT 
    a.id,
    a.content,
    s.explanation
FROM arguments a
JOIN scores s ON a.id = s.argument_id
WHERE a.id = 1;  -- Change this number to check different arguments

-- Get average scores by topic
SELECT 
    a.topic,
    COUNT(*) as argument_count,
    AVG(s.strength) as avg_strength,
    AVG(s.relevance) as avg_relevance,
    AVG(s.logic) as avg_logic,
    AVG(s.truth) as avg_truth,
    AVG(s.humor) as avg_humor,
    AVG(s.average) as overall_average
FROM arguments a
JOIN scores s ON a.id = s.argument_id
GROUP BY a.topic;

-- Usage instructions:
-- 1. Run with: sqlite3 data/arguments.db
-- 2. Load this file: .read sql/queries.sql 