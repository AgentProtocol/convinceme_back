package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/conversation" // Keep this for DebateConfig/NewDebateSession
	"github.com/neo/convinceme_backend/internal/database"

	// "github.com/neo/convinceme_backend/internal/player" // Removed unused import
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

	// Get both API keys
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		logger.Fatalf("OPENAI_API_KEY is not set in the environment variables")
	}

	elevenLabsKey := os.Getenv("ELEVENLABS_API_KEY")
	if elevenLabsKey == "" {
		logger.Fatalf("ELEVENLABS_API_KEY is not set in the environment variables")
	}

	// Check which TTS provider to use (defaults to ElevenLabs if not set)
	// Set to "openai" to use OpenAI's TTS service instead of ElevenLabs
	ttsProvider := os.Getenv("TTS_PROVIDER")
	logger.Printf("Using TTS provider: %s", ttsProvider)

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

	// Create agents with OpenAI API key
	agent1, err := agent.NewAgent(openAIKey, agent1Config)
	if err != nil {
		logger.Fatalf("Failed to create agent1: %v", err)
	}

	agent2, err := agent.NewAgent(openAIKey, agent2Config)
	if err != nil {
		logger.Fatalf("Failed to create agent2: %v", err)
	}

	// Create input handler (Removed as it's unused)
	// inputHandler := player.NewInputHandler(logger)

	// Define the debate topic with explicit initial context
	commonTopic := "Are memecoins net negative or positive for the crypto space?"

	// Create debate configuration using the renamed struct and DefaultConfig
	debateConfig := conversation.DefaultConfig() // Use the new default
	debateConfig.Topic = commonTopic             // Override topic if needed
	debateConfig.ResponseStyle = types.ResponseStyleDebate
	debateConfig.MaxCompletionTokens = 150
	// Adjust other fields from DefaultConfig if necessary
	// convConfig := conversation.DebateConfig{ // Old manual config
	// 	Topic:               commonTopic,
	// 	MaxTurns:            10,
	// 	TurnDelay:           500 * time.Millisecond,
	// }

	// Create a new debate session (placeholder - this logic will move to manager)
	// Note: inputHandler is currently unused in server.go and might be removed entirely
	// For now, passing nil or the existing handler. The DebateSession constructor doesn't take it.
	// Using a placeholder ID "main_debate"
	_, err = conversation.NewDebateSession("main_debate", agent1, agent2, debateConfig, openAIKey)
	if err != nil {
		logger.Fatalf("Failed to create main debate session: %v", err)
	}
	// conv := conversation.NewDebateSession(agent1, agent2, debateConfig, openAIKey) // Old call

	// Create agents map (remains the same)
	agents := map[string]*agent.Agent{
		agent1Config.Name: agent1,
		agent2Config.Name: agent2,
	}

	// Update server config to include both API keys
	serverConfig := &server.Config{
		Port:          ":8080",
		OpenAIKey:     openAIKey,
		ElevenLabsKey: elevenLabsKey, // Use ElevenLabs key
		ResponseDelay: 500,
	}

	// Create and start the server
	srv := server.NewServer(agents, db, openAIKey, useHTTPS, serverConfig)
	logger.Printf("Starting server on %s...", serverConfig.Port)
	if err := srv.Run(serverConfig.Port); err != nil {
		logger.Fatalf("Server failed: %v", err)
	}

	// Start the conversation (This logic is removed as the server/manager will handle starting sessions)
	// if err := conv.Start(context.Background()); err != nil {
	// 	logger.Fatalf("Failed to start conversation: %v", err)
	// }
}
