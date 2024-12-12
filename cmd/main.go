package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/server"
	"github.com/neo/convinceme_backend/internal/types"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	// Ensure HLS directory exists
	hlsDir := filepath.Join("static", "hls")
	if err := os.MkdirAll(hlsDir, 0755); err != nil {
		log.Fatalf("Failed to create HLS directory: %v", err)
	}

	// Create agent configurations
	agent1Config := agent.AgentConfig{
		Name:        "Ilon Musk",
		Role:        "Technology Expert",
		Voice:       types.VoiceShimmer,
		Temperature: 1.5,
		MaxTokens:   50,
		TopP:        0.9,
	}

	agent2Config := agent.AgentConfig{
		Name:        "Joe Rogan",
		Role:        "Interviewer",
		Voice:       types.VoiceFable,
		Temperature: 1.5,
		MaxTokens:   50,
		TopP:        0.9,
	}

	// Create agents
	agent1, err := agent.NewAgent(apiKey, agent1Config, hlsDir)
	if err != nil {
		log.Fatalf("Failed to create agent1: %v", err)
	}

	agent2, err := agent.NewAgent(apiKey, agent2Config, hlsDir)
	if err != nil {
		log.Fatalf("Failed to create agent2: %v", err)
	}

	// Create agents map
	agents := map[string]*agent.Agent{
		agent1Config.Name: agent1,
		agent2Config.Name: agent2,
	}

	// Create and start the server
	srv := server.NewServer(agents)
	log.Println("Starting server on :8080...")
	if err := srv.Run(":8080"); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
