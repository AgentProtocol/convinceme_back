-- Replace existing topics with soccer-focused topics

-- First, delete all existing topics
DELETE FROM topics;

-- Reset the auto-increment counter
DELETE FROM sqlite_sequence WHERE name='topics';

-- Insert new soccer topics (only the ones from create_test_debates.go)
INSERT INTO topics (title, description, agent1_name, agent1_role, agent2_name, agent2_role, category) VALUES
-- Soccer GOAT debate
('Who''s the GOAT of football: Messi or Ronaldo?',
 'The ultimate debate about the greatest footballer of all time',
 '''La Pulga Protector'' Pepito', 'Messi devotee and Argentine football evangelist',
 '''Siuuuu Sensei'' Sergio', 'Football analyst and Ronaldo legacy defender',
 'football'),

-- PSG debate
('PSG: Is this CL win the start of a new era or just a one-time success?',
 'A debate about PSG''s recent Champions League victory and future prospects',
 '''PSG Dynasty'' Pierre', 'Believes PSG will dominate European football',
 '''One-Hit Wonder'' Oliver', 'Thinks PSG''s success is temporary',
 'football'),

-- VAR debate
('VAR: Saving the game or ruining the flow?',
 'A debate about Video Assistant Referee technology in football',
 '''VAR Advocate'' Victoria', 'Believes VAR makes football fairer',
 '''Flow Defender'' Frank', 'Thinks VAR disrupts the natural game flow',
 'football'),

-- League comparison
('Premier League or La Liga: Which is the best league in the world?',
 'A debate about the superiority of England''s or Spain''s top football division',
 '''Premier Power'' Paul', 'Believes the Premier League is unmatched',
 '''La Liga Legend'' Luis', 'Argues La Liga has superior technical quality',
 'football'),

-- Manager comparison
('Who''s the better manager: Guardiola or Klopp?',
 'A debate about two of the world''s most successful modern football managers',
 '''Pep Perfectionist'' Patricia', 'Believes Guardiola''s tactical genius is unmatched',
 '''Klopp Crusader'' Klaus', 'Argues Klopp''s passion and results speak louder',
 'football'),

-- Competition prestige
('Is the Champions League more prestigious than the World Cup?',
 'A debate about which competition represents the pinnacle of football',
 '''Champions Champion'' Carlos', 'Believes the Champions League is the ultimate test',
 '''World Cup Warrior'' William', 'Argues the World Cup is football''s greatest stage',
 'football'); 