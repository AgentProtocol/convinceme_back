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

	// Check if HTTPS should be used
	useHTTPS := os.Getenv("USE_HTTPS") == "true"

	// Create agent configurations
	agent1Config := agent.AgentConfig{
		Name:        "Bear Expert",
		Role:        "Wildlife biologist specializing in bears, passionate advocate for bears' superiority. Expert in ursine behavior, physiology, and hunting patterns. Has spent 15 years studying bears in their natural habitat and strongly believes they are the ultimate apex predators.",
		Voice:       types.VoiceFable,
		Temperature: 1.5, // Higher temperature for more emotional responses
		MaxTokens:   150,
		TopP:        0.9,
	}

	agent2Config := agent.AgentConfig{
		Name:        "Tiger Specialist",
		Role:        "Big cat researcher and tiger conservation expert. Has studied tigers across Asia for 20 years, documenting their hunting techniques and physical capabilities. Firmly believes tigers are nature's perfect predators.",
		Voice:       types.VoiceOnyx,
		Temperature: 1.5, // Higher temperature for more emotional responses
		MaxTokens:   150,
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

	// Define the debate topic with explicit initial context
	commonTopic := `Bear vs Tiger: Who is the superior predator?

Key points to debate:
- Physical strength and combat abilities
- Hunting success rates and techniques
- Territorial dominance
- Survival skills and adaptability
- Historical encounters and documented fights
- Biological advantages and disadvantages

The debate should focus on factual evidence while acknowledging the passion each expert has for their respective species.`

	// Create conversation configuration with the common topic
	convConfig := conversation.ConversationConfig{
		Topic:           commonTopic,
		MaxTurns:        10, // Increased turns for more detailed debate
		TurnDelay:       500 * time.Millisecond,
		ResponseStyle:   types.ResponseStyleDebate, // Changed to debate style
		MaxTokens:       150,                       // Increased tokens for more detailed responses
		TemperatureHigh: true,
	}

	// Create a new conversation with the common topic
	conv := conversation.NewConversation(agent1, agent2, convConfig, inputHandler, apiKey)

	// Create server config with proper initialization
	serverConfig := &server.Config{
		OpenAIKey:     apiKey,
		Port:          ":8080",
		ResponseDelay: 500 * time.Millisecond,
		Agents: map[string]server.AgentConfig{
			agent1Config.Name: {
				Name:           agent1Config.Name,
				Role:           agent1Config.Role,
				Model:          "gpt-4-turbo-preview",
				Voice:          agent1Config.Voice.String(),
				DebatePosition: "Bear Expert",
			},
			agent2Config.Name: {
				Name:           agent2Config.Name,
				Role:           agent2Config.Role,
				Model:          "gpt-4-turbo-preview",
				Voice:          agent2Config.Voice.String(),
				DebatePosition: "Tiger Specialist",
			},
		},
	}

	// Create agents map
	agents := map[string]*agent.Agent{
		agent1Config.Name: agent1,
		agent2Config.Name: agent2,
	}

	// Create and start the server with proper config
	srv := server.NewServer(agents, apiKey, useHTTPS, serverConfig)
	if useHTTPS {
		logger.Println("Starting HTTPS server with HTTP/3 support on :8080...")
	} else {
		logger.Println("Starting HTTP server on :8080...")
	}

	// Start server with error handling
	if err := srv.Run(serverConfig.Port); err != nil {
		logger.Fatalf("Server failed: %v", err)
	}

	// Start the conversation
	if err := conv.Start(context.Background()); err != nil {
		logger.Fatalf("Failed to start conversation: %v", err)
	}
}
