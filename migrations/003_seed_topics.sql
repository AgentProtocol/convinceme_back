-- Seed the topics table with pre-generated topics

-- Insert pre-generated topics with agent pairings
INSERT INTO topics (title, description, agent1_name, agent1_role, agent2_name, agent2_role, category) VALUES
-- Crypto topics
('Are memecoins net negative or positive for the crypto space?', 
 'A debate about the impact of meme-based cryptocurrencies on the broader crypto ecosystem', 
 '''Fundamentals First'' Bradford', 'Crypto analyst who believes in fundamentals and utility',
 '''Memecoin Supercycle'' Murad', 'Crypto enthusiast who sees value in community-driven tokens',
 'crypto'),

('Is Bitcoin digital gold or a payment system?',
 'A debate about the primary purpose and value proposition of Bitcoin',
 '''Store of Value'' Sarah', 'Bitcoin maximalist who sees BTC primarily as digital gold',
 '''Payments First'' Pablo', 'Bitcoin advocate who believes BTC should focus on being a payment network',
 'crypto'),

('Will Ethereum remain the dominant smart contract platform?',
 'A debate about Ethereum''s future as the leading smart contract platform',
 '''Ethereum Maximalist'' Emma', 'Believes Ethereum''s network effects are unbeatable',
 '''Multi-Chain Advocate'' Marcus', 'Believes multiple blockchains will share the market',
 'crypto'),

-- Technology topics
('Is AI a net positive or negative for humanity?',
 'A debate about the overall impact of artificial intelligence on human society',
 '''Tech Optimist'' Olivia', 'Believes AI will solve more problems than it creates',
 '''Tech Skeptic'' Samuel', 'Concerned about AI''s potential negative impacts',
 'technology'),

('Are open source or proprietary software models better?',
 'A debate about the merits of open source vs. closed source software development',
 '''Open Source Advocate'' Linus', 'Believes in the power of community-driven development',
 '''Proprietary Defender'' Bill', 'Believes commercial software leads to better products',
 'technology'),

-- Computer Science topics
('Segfault vs. Bus Error: Which is worse?',
 'A technical debate about different types of memory access errors in programming',
 '''Segfault Specialist'' Sandra', 'Expert on segmentation faults and their debugging',
 '''Bus Error Analyst'' Brian', 'Specialist in bus errors and hardware interactions',
 'computer science'),

('Tabs vs. Spaces: The ultimate coding style debate',
 'The classic debate about code indentation preferences',
 '''Tab Defender'' Terry', 'Believes tabs are superior for indentation',
 '''Space Advocate'' Sally', 'Believes spaces provide more consistent formatting',
 'computer science'),

-- Animal topics
('Lion vs. Tiger: Who would win in a fight?',
 'A debate about the relative strengths of lions and tigers',
 '''Lion Loyalist'' Leo', 'Expert on lion behavior and hunting techniques',
 '''Tiger Tactician'' Tara', 'Specialist in tiger strength and ambush tactics',
 'animals'),

('Are cats or dogs better pets?',
 'The classic debate about feline vs. canine companionship',
 '''Feline Fanatic'' Felix', 'Cat behavior expert who advocates for cats as ideal pets',
 '''Canine Champion'' Charlie', 'Dog trainer who believes dogs make superior companions',
 'animals');
