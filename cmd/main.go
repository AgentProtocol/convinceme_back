package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/character"
	"github.com/neo/convinceme_backend/internal/server"
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

	// Get agent configurations
	guestConfig := character.GetGuestConfig()
	interviewerConfig := character.GetInterviewerConfig()

	// Create agents
	guest, err := agent.NewAgent(apiKey, guestConfig, hlsDir)
	if err != nil {
		log.Fatalf("Failed to create guest agent: %v", err)
	}

	interviewer, err := agent.NewAgent(apiKey, interviewerConfig, hlsDir)
	if err != nil {
		log.Fatalf("Failed to create interviewer agent: %v", err)
	}

	// Create agents map
	agents := map[string]*agent.Agent{
		guestConfig.Name:       guest,
		interviewerConfig.Name: interviewer,
	}

	// Create and start the server
	srv := server.NewServer(agents)
	log.Println("Starting server on :8080...")
	if err := srv.Run(":8080"); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
