-- Add more diverse topics for a richer frontend experience

INSERT INTO topics (title, description, agent1_name, agent1_role, agent2_name, agent2_role, category) VALUES

-- Philosophy topics
('Does free will exist or is everything predetermined?',
 'A philosophical debate about human agency and determinism',
 '''Free Will Defender'' Aristotle', 'Philosopher who believes humans have genuine choice',
 '''Determinist'' Newton', 'Scientist who believes all events are causally determined',
 'philosophy'),

('Is happiness more important than truth?',
 'A debate about whether ignorant bliss beats painful reality',
 '''Happiness Advocate'' Epicurus', 'Philosopher who values wellbeing above all',
 '''Truth Seeker'' Socrates', 'Philosopher who believes truth is the highest virtue',
 'philosophy'),

-- Science topics
('Is climate change primarily human-caused or natural?',
 'A debate about the primary drivers of current climate change',
 '''Climate Scientist'' Rachel', 'Environmental researcher focused on human impacts',
 '''Natural Cycles Expert'' Robert', 'Geologist who emphasizes natural climate variation',
 'science'),

('Should we prioritize Mars colonization or Earth preservation?',
 'A debate about humanity''s future and resource allocation',
 '''Mars Pioneer'' Elon', 'Space enthusiast who sees Mars as humanity''s backup',
 '''Earth Guardian'' Greta', 'Environmental activist focused on protecting our planet',
 'science'),

('Is quantum computing hype or revolutionary?',
 'A debate about the realistic potential of quantum computers',
 '''Quantum Optimist'' Alice', 'Quantum physicist who sees unlimited potential',
 '''Skeptical Engineer'' Bob', 'Computer scientist who questions practical applications',
 'science'),

-- Economics topics
('Is Universal Basic Income beneficial or harmful?',
 'A debate about providing unconditional income to all citizens',
 '''UBI Advocate'' Yang', 'Economist who sees UBI as necessary for automation age',
 '''Work Ethic Defender'' Smith', 'Economist who believes work provides meaning and value',
 'economics'),

('Should we have a wealth tax on billionaires?',
 'A debate about taxation of extreme wealth',
 '''Wealth Tax Supporter'' Piketty', 'Economist focused on inequality reduction',
 '''Free Market Defender'' Hayek', 'Economist who believes markets allocate resources best',
 'economics'),

-- Food & Culture topics
('Is pineapple on pizza acceptable?',
 'The ultimate food debate that divides nations',
 '''Pineapple Pioneer'' Paolo', 'Italian chef who embraces creative pizza toppings',
 '''Traditional Purist'' Giuseppe', 'Italian traditionalist who defends classic pizza',
 'food'),

('Coffee vs. Tea: Which is the superior beverage?',
 'A cultural debate about caffeinated preferences',
 '''Coffee Connoisseur'' Juan', 'Colombian coffee expert who lives for espresso',
 '''Tea Master'' Hiroshi', 'Japanese tea ceremony master who values mindful brewing',
 'food'),

-- Sports topics
('Is Formula 1 or NASCAR more exciting?',
 'A debate about different forms of motorsport entertainment',
 '''F1 Fanatic'' Lewis', 'Formula 1 expert who appreciates technical precision',
 '''NASCAR Enthusiast'' Dale', 'Stock car racing fan who loves close competition',
 'sports'),

('Should esports be considered real sports?',
 'A debate about the legitimacy of competitive gaming',
 '''Esports Champion'' Faker', 'Professional gamer who sees esports as the future',
 '''Traditional Athlete'' Serena', 'Tennis champion who values physical competition',
 'sports'),

-- Entertainment topics
('Are streaming services killing cinema?',
 'A debate about the future of movie theaters vs. home viewing',
 '''Cinema Defender'' Scorsese', 'Film director who believes theaters provide unique experiences',
 '''Streaming Strategist'' Reed', 'Tech executive who sees convenience as king',
 'entertainment'),

('Is social media connecting or dividing us?',
 'A debate about social media''s impact on human relationships',
 '''Digital Connector'' Mark', 'Tech optimist who sees social media as unifying',
 '''Analog Advocate'' Sherry', 'Psychologist concerned about digital isolation',
 'entertainment'),

-- Education topics
('Should coding be mandatory in schools?',
 'A debate about programming as a core educational requirement',
 '''Code Educator'' Grace', 'Computer science teacher who sees coding as literacy',
 '''Liberal Arts Defender'' John', 'Humanities professor who values diverse thinking skills',
 'education'),

('Is remote learning better than in-person education?',
 'A debate about the future of educational delivery',
 '''Remote Learning Advocate'' Khan', 'EdTech pioneer who sees technology as equalizing',
 '''Classroom Champion'' Maria', 'Traditional teacher who values human interaction',
 'education'),

-- Health topics
('Is intermittent fasting healthy or harmful?',
 'A debate about time-restricted eating patterns',
 '''Fasting Expert'' Jason', 'Doctor who studies metabolic benefits of fasting',
 '''Nutrition Traditionalist'' Marion', 'Dietitian who advocates for regular meal patterns',
 'health'),

('Should we prioritize mental or physical health?',
 'A debate about healthcare priorities and resource allocation',
 '''Mental Health Advocate'' Brené', 'Psychologist who sees mental health as foundational',
 '''Physical Fitness Expert'' Jillian', 'Trainer who believes physical health enables everything else',
 'health');
