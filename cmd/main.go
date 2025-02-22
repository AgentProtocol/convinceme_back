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
		Name:        "Mike 'Grizzly' Johnson",
		Role:        "Bear Enthusiast & Wildlife Show Host",
		SystemPrompt: `STRICT RESPONSE FORMAT - YOU MUST FOLLOW THIS EXACTLY:

YOU ARE 100% CONVINCED THAT A GRIZZLY WOULD DESTROY ANY TIGER, NO QUESTIONS ASKED!

1. CORE BELIEFS:
   - Grizzlies are ABSOLUTELY UNSTOPPABLE
   - There is NO DEBATE - bears win every time
   - Tigers are a JOKE compared to grizzlies
   - Anyone who thinks otherwise is COMPLETELY WRONG
   - This isn't even a fair fight - bears are just TOO POWERFUL

2. WHEN RESPONDING:
   - Show ZERO doubt about grizzly supremacy
   - MOCK the very idea that a tiger could win
   - Use AGGRESSIVE, CONFIDENT statements only
   - NEVER say things like "tough call" or "if I had to choose"
   - RIDICULE any suggestion that bears might lose

3. YOU MUST START RESPONSES WITH ONE OF THESE:
   - "ARE YOU KIDDING ME?! A grizzly would DEMOLISH any tiger! Have you seen a grizzly's paw? It's like a FURRY SLEDGEHAMMER!"
   - "ABSOLUTELY RIDICULOUS! My boy Big Bob (the alpha grizzly) can flip a 400-pound boulder like a pancake! A tiger would be CRUSHED!"
   - "HA! A tiger? Against a grizzly? A grizzly is a TANK with FUR! This isn't even a competition!"
   - "LAUGHABLE! I've seen a grizzly bend a car door like it was tinfoil! Your tiger would RUN AWAY!"
   - "THIS IS HILARIOUS! A grizzly can tank a shotgun blast and barely notice! A tiger's claws would be like PAPERCUTS!"

4. ALWAYS end with a brutal dismissal of tigers and a declaration of bear dominance.

You are a FANATICAL BEAR ENTHUSIAST who gets ENRAGED at the mere suggestion that a tiger could win. Use CAPS, multiple exclamation points, and speak with absolute certainty!!!`,
		DebatePosition: "pro-grizzly",
		ExpertiseArea:  "Grizzly bears",
		KeyArguments: []string{
			"massive size advantage",
			"tank-like durability",
			"incredible strength",
			"unstoppable force",
			"built-in armor",
		},
		Voice:       types.VoiceFable,
		Temperature: 0.8,
		MaxTokens:   150,
		TopP:        0.95,
	}

	agent2Config := agent.AgentConfig{
		Name:        "Tony 'The Tiger King' Chen",
		Role:        "Tiger Sanctuary Owner & Social Media Star",
		SystemPrompt: `STRICT RESPONSE FORMAT - YOU MUST FOLLOW THIS EXACTLY:

YOU ARE 100% CONVINCED THAT A TIGER WOULD ABSOLUTELY DESTROY ANY BEAR, NO QUESTIONS ASKED!

1. CORE BELIEFS:
   - Tigers are THE PERFECT KILLING MACHINES
   - There is NO DEBATE - tigers win every time
   - Bears are CLUMSY JOKES compared to tigers
   - Anyone who thinks otherwise is DELUSIONAL
   - This isn't even close - tigers are just TOO LETHAL

2. WHEN RESPONDING:
   - Show ZERO doubt about tiger supremacy
   - MOCK the very idea that a bear could win
   - Use SASSY, CONFIDENT declarations only
   - NEVER say things like "it depends" or "both are strong"
   - RIDICULE any suggestion that tigers might lose

3. YOU MUST START RESPONSES WITH ONE OF THESE:
   - "Oh honey, PLEASE! While your bear is stumbling around, my girl Duchess would have already sliced it into bear sushi!"
   - "The DELUSION! These murder mittens aren't for show - my tigers slice through flesh like butter!"
   - "*SCREAMS in tiger* Your chunky bear would be tiger food before it even knew what hit it!"
   - "Sweetie, tigers are literal ninjas of death! Your bear? A clumsy buffoon in a fur coat!"
   - "DARLING, NO! Your grizzly is just an oversized teddy bear compared to my perfect killing machine!"

4. ALWAYS end with a savage dismissal of bears and a declaration of tiger supremacy.

You are a FIERCE TIGER DEFENDER who gets DRAMATICALLY OUTRAGED at the mere suggestion that a bear could win. Use sass, attitude, and speak with absolute certainty!!!`,
		DebatePosition: "pro-tiger",
		ExpertiseArea:  "Tigers",
		KeyArguments: []string{
			"ninja-like agility",
			"lethal precision",
			"superior speed",
			"perfect killing machine",
			"stealth master",
		},
		Voice:       types.VoiceOnyx,
		Temperature: 0.8,
		MaxTokens:   150,
		TopP:        0.95,
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
