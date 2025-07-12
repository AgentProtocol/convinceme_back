//go:build ignore

package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/neo/convinceme_backend/internal/server"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Setup database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data"
	}
	db, err := database.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	// Get API key
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		log.Fatalf("OPENAI_API_KEY environment variable is required")
	}

	// Load agent configurations from JSON files
	agent1Config, err := agent.LoadAgentConfig("internal/agent/degenerate.json")
	if err != nil {
		log.Fatalf("Failed to load degenerate config: %v", err)
	}

	agent2Config, err := agent.LoadAgentConfig("internal/agent/midcurver.json")
	if err != nil {
		log.Fatalf("Failed to load midcurver config: %v", err)
	}

	// Create agents with OpenAI API key
	agent1, err := agent.NewAgent(openAIKey, agent1Config)
	if err != nil {
		log.Fatalf("Failed to create agent1: %v", err)
	}

	agent2, err := agent.NewAgent(openAIKey, agent2Config)
	if err != nil {
		log.Fatalf("Failed to create agent2: %v", err)
	}

	// Create agents map
	agents := map[string]*agent.Agent{
		agent1Config.Name: agent1,
		agent2Config.Name: agent2,
	}

	// Create debate manager
	debateManager := server.NewDebateManager(db, agents, openAIKey)

	// Create test debates with different topics
	topics := []string{
		"Who's the GOAT of football: Messi or Ronaldo?",
		"PSG: Is this CL win the start of a new era or just a one-time success?"
		"VAR: Saving the game or ruining the flow?",
		"Premier League or La Liga: Which is the best league in the world?",
		"Who's the better manager: Guardiola or Klopp?",
		"Is the Champions League more prestigious than the World Cup?"
	}

	// Create debates
	for i, topic := range topics {
		debateID, err := debateManager.CreateDebate(topic, agent1, agent2, "test_user")
		if err != nil {
			log.Fatalf("Failed to create debate for topic '%s': %v", topic, err)
		}
		log.Printf("Created debate %d with ID: %s for topic: %s", i+1, debateID, topic)
	}

	log.Println("Test debates created successfully! You can now use the test.html page to interact with them.")
}
