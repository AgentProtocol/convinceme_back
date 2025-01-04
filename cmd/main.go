package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/conversation"
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

	// Create agent configurations
	agent1Config := agent.AgentConfig{
		Name:        "Billionaire, CEO of SpaceX and Tesla, fanatic Trump supporter",
		Role:        "Billionaire, CEO of SpaceX and Tesla, fanatic Trump supporter",
		Voice:       types.VoiceFable,
		Temperature: 1.5, // Higher temperature for more emotional responses
		MaxTokens:   50,
		TopP:        0.9,
	}

	agent2Config := agent.AgentConfig{
		Name:        "Joe Rogan",
		Role:        "Interviewer, host of The Joe Rogan Experience podcast, UFC commentator",
		Voice:       types.VoiceOnyx,
		Temperature: 1.5, // Higher temperature for more emotional responses
		MaxTokens:   50,
		TopP:        0.9,
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

	// Define a common topic for the conversation
	commonTopic := "Political landscape in the United States"

	// Create conversation configuration with the common topic
	convConfig := conversation.ConversationConfig{
		Topic:           commonTopic,
		MaxTurns:        5,
		TurnDelay:       500 * time.Millisecond,
		ResponseStyle:   types.ResponseStyleHumorous,
		MaxTokens:       100,
		TemperatureHigh: true,
	}

	// Create a new conversation with the common topic
	conv := conversation.NewConversation(agent1, agent2, convConfig, inputHandler)

	// Create agents map
	agents := map[string]*agent.Agent{
		agent1Config.Name: agent1,
		agent2Config.Name: agent2,
	}

	// Create and start the server
	srv := server.NewServer(agents)
	logger.Println("Starting HTTPS server with HTTP/3 support on :8080...")
	if err := srv.Run(":8080"); err != nil {
		logger.Fatalf("Server failed: %v", err)
	}

	// Start the conversation
	if err := conv.Start(context.Background()); err != nil {
		logger.Fatalf("Failed to start conversation: %v", err)
	}
}
