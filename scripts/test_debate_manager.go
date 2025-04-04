package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/neo/convinceme_backend/internal/server"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Setup database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/arguments.db"
	}
	db, err := database.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	// Create test agents
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		log.Fatalf("OPENAI_API_KEY environment variable is required")
	}

	agent1Config := &agent.Config{
		Name:        "Test Agent 1",
		Model:       "gpt-3.5-turbo",
		Temperature: 0.7,
	}
	agent2Config := &agent.Config{
		Name:        "Test Agent 2",
		Model:       "gpt-3.5-turbo",
		Temperature: 0.7,
	}

	agent1, err := agent.NewAgent(openAIKey, agent1Config)
	if err != nil {
		log.Fatalf("Failed to create agent1: %v", err)
	}

	agent2, err := agent.NewAgent(openAIKey, agent2Config)
	if err != nil {
		log.Fatalf("Failed to create agent2: %v", err)
	}

	agents := map[string]*agent.Agent{
		agent1Config.Name: agent1,
		agent2Config.Name: agent2,
	}

	// Create debate manager
	debateManager := server.NewDebateManager(db, agents, openAIKey)

	// Test 1: Create debates
	log.Println("=== Test 1: Creating debates ===")
	debateIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		topic := fmt.Sprintf("Test Topic %d", i+1)
		debateID, err := debateManager.CreateDebate(topic, agent1, agent2, "test_user")
		if err != nil {
			log.Fatalf("Failed to create debate %d: %v", i+1, err)
		}
		debateIDs[i] = debateID
		log.Printf("Created debate %d with ID: %s", i+1, debateID)
	}

	// Test 2: Check debates exist
	log.Println("\n=== Test 2: Verifying debates exist ===")
	for i, debateID := range debateIDs {
		session, exists := debateManager.GetDebate(debateID)
		if !exists {
			log.Fatalf("Debate %d with ID %s not found", i+1, debateID)
		}
		log.Printf("Debate %d (ID: %s) found - Status: %s", i+1, debateID, session.GetStatus())
	}

	// Test 3: Simulate setting debates to different states
	log.Println("\n=== Test 3: Simulating state changes ===")

	// Set first debate to active
	session1, _ := debateManager.GetDebate(debateIDs[0])
	session1.UpdateStatus("active")
	log.Printf("Updated debate 1 status to active, current status: %s", session1.GetStatus())

	// Set second debate to finished
	session2, _ := debateManager.GetDebate(debateIDs[1])
	session2.UpdateStatus("finished")
	log.Printf("Updated debate 2 status to finished, current status: %s", session2.GetStatus())

	// Leave third debate in waiting state
	session3, _ := debateManager.GetDebate(debateIDs[2])
	log.Printf("Debate 3 status remains: %s", session3.GetStatus())

	// Test 4: Run cleanup and check what was removed
	log.Println("\n=== Test 4: Testing cleanup functionality ===")
	log.Println("Before cleanup:")
	printDebateStatuses(debateManager, debateIDs)

	debateManager.CleanupInactiveDebates()

	log.Println("\nAfter cleanup:")
	printDebateStatuses(debateManager, debateIDs)

	// Test 5: Set up a periodic cleanup
	log.Println("\n=== Test 5: Testing periodic cleanup ===")
	debateManager.StartPeriodicCleanup(2 * time.Second)
	log.Println("Started periodic cleanup with 2-second interval")

	// Wait a bit and check again
	time.Sleep(5 * time.Second)
	log.Println("\nAfter waiting for periodic cleanup:")
	printDebateStatuses(debateManager, debateIDs)

	log.Println("\nTests completed!")
}

// Helper function to print debate statuses
func printDebateStatuses(manager *server.DebateManager, debateIDs []string) {
	for i, debateID := range debateIDs {
		session, exists := manager.GetDebate(debateID)
		status := "removed"
		if exists {
			status = session.GetStatus()
		}
		log.Printf("Debate %d (ID: %s) - Status: %s, Exists: %t", i+1, debateID, status, exists)
	}
}
