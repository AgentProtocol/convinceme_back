package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/conversation" // Keep this for DebateConfig/NewDebateSession
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/neo/convinceme_backend/internal/logging"

	// "github.com/neo/convinceme_backend/internal/player" // Removed unused import
	"github.com/neo/convinceme_backend/internal/server"
	"github.com/neo/convinceme_backend/internal/types"
)

func main() {
	// Initialize the comprehensive logging system
	logLevel := logging.INFO
	if os.Getenv("DEBUG") == "true" {
		logLevel = logging.DEBUG
	}

	logConfig := logging.Config{
		Level:       logLevel,
		Prefix:      "ConvinceMe",
		Colored:     true,
		LogToFile:   true,
		LogFilePath: "logs/app.log",
	}

	if err := logging.InitDefaultLogger(logConfig); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	logging.Info("Starting ConvinceMe Backend", map[string]interface{}{
		"version": "1.0.0",
		"env":     os.Getenv("ENV"),
	})

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logging.Fatal("Error loading .env file", map[string]interface{}{"error": err})
	}
	logging.Info("Environment variables loaded successfully")

	// Get both API keys
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		logging.Fatal("OPENAI_API_KEY is not set in the environment variables")
	}

	elevenLabsKey := os.Getenv("ELEVENLABS_API_KEY")
	if elevenLabsKey == "" {
		logging.Fatal("ELEVENLABS_API_KEY is not set in the environment variables")
	}

	// Check which TTS provider to use (defaults to ElevenLabs if not set)
	// Set to "openai" to use OpenAI's TTS service instead of ElevenLabs
	ttsProvider := os.Getenv("TTS_PROVIDER")
	logging.Info("TTS Configuration", map[string]interface{}{
		"provider": ttsProvider,
	})

	// Check if HTTPS should be used
	useHTTPS := os.Getenv("USE_HTTPS") == "true"
	logging.Info("Server Configuration", map[string]interface{}{
		"https_enabled": useHTTPS,
	})

	// Initialize database
	logging.Info("Initializing database...")
	db, err := database.New("data")
	if err != nil {
		logging.Fatal("Failed to initialize database", map[string]interface{}{"error": err})
	}
	logging.Info("Database initialized successfully")
	defer db.Close()

	// Ensure HLS directory exists
	logging.Info("Setting up HLS directory...")
	hlsDir := filepath.Join("static", "hls")
	if err := os.MkdirAll(hlsDir, 0755); err != nil {
		logging.Fatal("Failed to create HLS directory", map[string]interface{}{"error": err, "path": hlsDir})
	}
	logging.Info("HLS directory ready", map[string]interface{}{"path": hlsDir})

	// Load agent configurations from JSON files
	logging.Info("Loading agent configurations...")
	agent1Config, err := agent.LoadAgentConfig("internal/agent/degenerate.json")
	if err != nil {
		logging.Fatal("Failed to load degenerate config", map[string]interface{}{"error": err})
	}

	agent2Config, err := agent.LoadAgentConfig("internal/agent/midcurver.json")
	if err != nil {
		logging.Fatal("Failed to load midcurver config", map[string]interface{}{"error": err})
	}
	logging.Info("Agent configurations loaded", map[string]interface{}{
		"agent1": agent1Config.Name,
		"agent2": agent2Config.Name,
	})

	// Create agents with OpenAI API key
	logging.Info("Creating AI agents...")
	agent1, err := agent.NewAgent(openAIKey, agent1Config)
	if err != nil {
		logging.Fatal("Failed to create agent1", map[string]interface{}{"error": err, "agent": agent1Config.Name})
	}

	agent2, err := agent.NewAgent(openAIKey, agent2Config)
	if err != nil {
		logging.Fatal("Failed to create agent2", map[string]interface{}{"error": err, "agent": agent2Config.Name})
	}
	logging.Info("AI agents created successfully")

	// Create input handler (Removed as it's unused)
	// inputHandler := player.NewInputHandler(logger)

	// Define the debate topic with explicit initial context
	commonTopic := "Are memecoins net negative or positive for the crypto space?"

	// Create debate configuration using the renamed struct and DefaultConfig
	logging.Info("Setting up debate configuration", map[string]interface{}{
		"topic": commonTopic,
	})
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
	logging.Info("Creating main debate session...")
	_, err = conversation.NewDebateSession("main_debate", agent1, agent2, debateConfig, openAIKey)
	if err != nil {
		logging.Fatal("Failed to create main debate session", map[string]interface{}{"error": err})
	}
	logging.Info("Main debate session created successfully")
	// conv := conversation.NewDebateSession(agent1, agent2, debateConfig, openAIKey) // Old call

	// Create agents map (remains the same)
	agents := map[string]*agent.Agent{
		agent1Config.Name: agent1,
		agent2Config.Name: agent2,
	}

	// Get JWT secret from environment variables
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default_secret_key_for_development" // Default for development
		logging.Warn("JWT_SECRET not set, using default value for development")
	}

	// Check if email verification is required
	requireEmailVerification := os.Getenv("REQUIRE_EMAIL_VERIFICATION") == "true"

	// Check if invitation codes are required for registration
	requireInvitation := os.Getenv("REQUIRE_INVITATION") == "true"

	logging.Info("Authentication Configuration", map[string]interface{}{
		"email_verification_required": requireEmailVerification,
		"invitation_required":         requireInvitation,
	})

	// Get port from environment, default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	// Ensure port has colon prefix
	if port[0] != ':' {
		port = ":" + port
	}

	// Update server config to include both API keys
	serverConfig := &server.Config{
		Port:                     port,
		OpenAIKey:                openAIKey,
		ElevenLabsKey:            elevenLabsKey, // Use ElevenLabs key
		ResponseDelay:            500,
		JWTSecret:                jwtSecret,
		RequireEmailVerification: requireEmailVerification,
		RequireInvitation:        requireInvitation,
	}

	// Create and start the server
	srv := server.NewServer(agents, db, openAIKey, useHTTPS, serverConfig)
	logging.Info("Starting server", map[string]interface{}{
		"port":  serverConfig.Port,
		"https": useHTTPS,
	})
	if err := srv.Run(serverConfig.Port); err != nil {
		logging.Fatal("Server failed", map[string]interface{}{"error": err})
	}

	// Start the conversation (This logic is removed as the server/manager will handle starting sessions)
	// if err := conv.Start(context.Background()); err != nil {
	// 	logger.Fatalf("Failed to start conversation: %v", err)
	// }
}
