package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/conversation"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/neo/convinceme_backend/internal/player"
	"github.com/neo/convinceme_backend/internal/server"
	"github.com/neo/convinceme_backend/internal/types"
)

func main() {
	// Set up logging
	logger := log.New(os.Stdout, "[ConvinceMe] ", log.LstdFlags|log.Lshortfile)

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Fatalf("Error loading .env file: %v", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		logger.Fatalf("OPENAI_API_KEY is not set in the environment variables")
	}

	// Check if HTTPS should be used
	useHTTPS := os.Getenv("USE_HTTPS") == "true"

	// Initialize database
	db, err := database.New("data")
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Ensure HLS directory exists
	hlsDir := filepath.Join("static", "hls")
	if err := os.MkdirAll(hlsDir, 0755); err != nil {
		logger.Fatalf("Failed to create HLS directory: %v", err)
	}

	// Load agent configurations from JSON files
	agent1Config, err := agent.LoadAgentConfig("internal/agent/degenerate.json")
	if err != nil {
		logger.Fatalf("Failed to load degenerate config: %v", err)
	}

	agent2Config, err := agent.LoadAgentConfig("internal/agent/midcurver.json")
	if err != nil {
		logger.Fatalf("Failed to load midcurver config: %v", err)
	}

	// Create agents
	agent1, err := agent.NewAgent(apiKey, agent1Config)
	if err != nil {
		logger.Fatalf("Failed to create agent1: %v", err)
	}

	agent2, err := agent.NewAgent(apiKey, agent2Config)
	if err != nil {
		logger.Fatalf("Failed to create agent2: %v", err)
	}

	// Create input handler
	inputHandler := player.NewInputHandler(logger)

	// Define the debate topic with explicit initial context
	commonTopic := "Are memecoins net negative or positive for the crypto space?"

	// Create conversation configuration
	convConfig := conversation.ConversationConfig{
		Topic:           commonTopic,
		MaxTurns:        10,
		TurnDelay:       500 * time.Millisecond,
		ResponseStyle:   types.ResponseStyleDebate, // Changed to debate style
		MaxCompletionTokens:       150,                       // Increased tokens for more detailed responses
		TemperatureHigh: true,
	}

	// Create a new conversation with the common topic
	conv := conversation.NewConversation(agent1, agent2, convConfig, inputHandler, apiKey)

	// Create agents map
	agents := map[string]*agent.Agent{
		agent1Config.Name: agent1,
		agent2Config.Name: agent2,
	}

	// Create server configuration
	serverConfig := &server.Config{
		Port:          ":8080",
		OpenAIKey:     apiKey,
		ResponseDelay: 500,
	}

	// Create and start the server
	srv := server.NewServer(agents, db, apiKey, useHTTPS, serverConfig)
	logger.Printf("Starting server on %s...", serverConfig.Port)
	if err := srv.Run(serverConfig.Port); err != nil {
		logger.Fatalf("Server failed: %v", err)
	}

	// Start the conversation
	if err := conv.Start(context.Background()); err != nil {
		logger.Fatalf("Failed to start conversation: %v", err)
	}
}
