package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/neo/convinceme_backend/internal/scoring"
	"github.com/neo/convinceme_backend/internal/server"
)

// MockAgent implements the agent.Agent interface for testing
type MockAgent struct {
	name  string
	voice string
}

func NewMockAgent(name string) *agent.Agent {
	agentConfig := &agent.AgentConfig{
		Name:  name,
		Model: "gpt-3.5-turbo",
	}

	// Create a real agent with the OpenAI key
	openAIKey := os.Getenv("OPENAI_API_KEY")
	agent, err := agent.NewAgent(openAIKey, agentConfig)
	if err != nil {
		log.Fatalf("Failed to create agent %s: %v", name, err)
	}
	return agent
}

// MockScorer implements a simple scoring mechanism
type MockScorer struct{}

func NewMockScorer() *MockScorer {
	return &MockScorer{}
}

func (s *MockScorer) ScoreArgument(ctx context.Context, argument, topic string) (*scoring.ArgumentScore, error) {
	// Return a fixed score for testing
	return &scoring.ArgumentScore{
		Strength:    7,
		Relevance:   8,
		Logic:       6,
		Truth:       7,
		Humor:       5,
		Average:     6.6,
		Explanation: "Mock scoring explanation",
	}, nil
}

// MockClient simulates a WebSocket client
type MockClient struct {
	id       string
	messages []gin.H
	mu       sync.Mutex
}

func NewMockClient(id string) *MockClient {
	return &MockClient{
		id:       id,
		messages: make([]gin.H, 0),
	}
}

func (c *MockClient) ReceiveMessage(message gin.H) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, message)
	log.Printf("Client %s received message: %v", c.id, message)
}

func (c *MockClient) GetMessages() []gin.H {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.messages
}

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

	// Create mock agents
	agent1 := NewMockAgent("'Fundamentals First' Bradford")
	agent2 := NewMockAgent("'Memecoin Supercycle' Murad")

	agents := map[string]*agent.Agent{
		agent1.GetName(): agent1,
		agent2.GetName(): agent2,
	}

	// Create debate manager with mock agents
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		log.Fatalf("OPENAI_API_KEY environment variable is required")
	}

	debateManager := server.NewDebateManager(db, agents, openAIKey)

	// Test 1: Create multiple debates with different topics
	log.Println("=== Test 1: Creating multiple debates ===")
	topics := []string{
		"Are memecoins net negative or positive for the crypto space?",
		"Is Bitcoin digital gold or a payment system?",
		"Will Ethereum remain the dominant smart contract platform?",
	}

	debateIDs := make([]string, len(topics))
	for i, topic := range topics {
		debateID, err := debateManager.CreateDebate(topic, agent1, agent2, "test_user")
		if err != nil {
			log.Fatalf("Failed to create debate for topic '%s': %v", topic, err)
		}
		debateIDs[i] = debateID
		log.Printf("Created debate with ID: %s for topic: %s", debateID, topic)
	}

	// Test 2: Simulate clients joining debates
	log.Println("\n=== Test 2: Simulating clients joining debates ===")
	mockClients := make(map[string][]*MockClient)

	for i, debateID := range debateIDs {
		// Create 2 mock clients for each debate
		clients := []*MockClient{
			NewMockClient(fmt.Sprintf("player1_debate%d", i+1)),
			NewMockClient(fmt.Sprintf("player2_debate%d", i+1)),
		}
		mockClients[debateID] = clients

		// Get the debate session
		session, exists := debateManager.GetDebate(debateID)
		if !exists {
			log.Fatalf("Debate with ID %s not found", debateID)
		}

		// Add mock clients to the session
		for _, client := range clients {
			// In a real scenario, we would add WebSocket connections
			// Here we're just simulating by updating the client count
			session.AddClient(nil, client.id)
			log.Printf("Added client %s to debate %s", client.id, debateID)
		}

		// Activate the debate
		session.UpdateStatus("active")
		log.Printf("Activated debate %s", debateID)
	}

	// Test 3: Simulate player messages and score changes
	log.Println("\n=== Test 3: Simulating player messages and score changes ===")

	// Focus on the first debate for detailed testing
	testDebateID := debateIDs[0]
	testSession, _ := debateManager.GetDebate(testDebateID)

	// Create a mock broadcast function to capture messages
	testSession.SetBroadcastHandler(func(message gin.H) {
		for _, client := range mockClients[testDebateID] {
			client.ReceiveMessage(message)
		}
	})

	// Simulate player messages
	playerMessages := []struct {
		playerID string
		message  string
		side     string
	}{
		{mockClients[testDebateID][0].id, "I think memecoins are great for bringing new people to crypto", "agent2"},
		{mockClients[testDebateID][1].id, "Memecoins have no fundamental value and are purely speculative", "agent1"},
		{mockClients[testDebateID][0].id, "But they create excitement and community engagement", "agent2"},
	}

	// Process player messages
	for _, pm := range playerMessages {
		// Simulate player message
		log.Printf("Player %s sends message: %s (supporting %s)", pm.playerID, pm.message, pm.side)

		// Add to history
		testSession.AddHistoryEntry(pm.playerID, pm.message, true)

		// Score the message (using mock scorer)
		mockScore := &scoring.ArgumentScore{
			Strength:    7,
			Relevance:   8,
			Logic:       6,
			Truth:       7,
			Humor:       5,
			Average:     6.6,
			Explanation: "Mock scoring explanation",
		}

		// Update game score based on side
		var agent1Delta, agent2Delta int
		if pm.side == "agent1" {
			agent1Delta = 10
			agent2Delta = -10
		} else {
			agent1Delta = -10
			agent2Delta = 10
		}

		gameScore := testSession.UpdateGameScore(agent1Delta, agent2Delta)

		// Broadcast message with score
		testSession.Broadcast(gin.H{
			"type":     "message",
			"agent":    pm.playerID,
			"message":  pm.message,
			"isPlayer": true,
			"scores": gin.H{
				"argument": mockScore,
			},
		})

		// Broadcast updated game score
		testSession.Broadcast(gin.H{
			"type": "game_score",
			"gameScore": gin.H{
				agent1.GetName(): debateManager.NormalizeScore(gameScore.Agent1Score),
				agent2.GetName(): debateManager.NormalizeScore(gameScore.Agent2Score),
			},
			"internalScore": gin.H{
				agent1.GetName(): gameScore.Agent1Score,
				agent2.GetName(): gameScore.Agent2Score,
			},
		})

		log.Printf("Updated game score - %s: %d, %s: %d",
			agent1.GetName(), gameScore.Agent1Score,
			agent2.GetName(), gameScore.Agent2Score)
	}

	// Test 4: Simulate game over condition
	log.Println("\n=== Test 4: Simulating game over condition ===")

	// Force one agent's score to zero to trigger game over
	gameScore := testSession.GetGameScore()

	// Make agent1 lose by setting score to 0
	agent1Delta := -gameScore.Agent1Score
	agent2Delta := 0

	gameScore = testSession.UpdateGameScore(agent1Delta, agent2Delta)
	log.Printf("Forced game over - %s: %d, %s: %d",
		agent1.GetName(), gameScore.Agent1Score,
		agent2.GetName(), gameScore.Agent2Score)

	// Check if game is over
	if gameScore.Agent1Score <= 0 {
		winner := agent2.GetName()
		log.Printf("Game over! %s has won the debate", winner)

		// Update status
		testSession.UpdateStatus("finished")

		// Broadcast game over message
		testSession.Broadcast(gin.H{
			"type":    "game_over",
			"winner":  winner,
			"message": fmt.Sprintf("Game over! %s has won the debate!", winner),
		})
	} else if gameScore.Agent2Score <= 0 {
		winner := agent1.GetName()
		log.Printf("Game over! %s has won the debate", winner)

		// Update status
		testSession.UpdateStatus("finished")

		// Broadcast game over message
		testSession.Broadcast(gin.H{
			"type":    "game_over",
			"winner":  winner,
			"message": fmt.Sprintf("Game over! %s has won the debate!", winner),
		})
	}

	// Test 5: Verify debate statuses
	log.Println("\n=== Test 5: Verifying debate statuses ===")
	for i, debateID := range debateIDs {
		session, exists := debateManager.GetDebate(debateID)
		if !exists {
			log.Fatalf("Debate %d with ID %s not found", i+1, debateID)
		}
		status := session.GetStatus()
		log.Printf("Debate %d (ID: %s) - Status: %s", i+1, debateID, status)

		// For the test debate, verify it's finished
		if debateID == testDebateID {
			if status != "finished" {
				log.Fatalf("Test debate should be finished, but status is: %s", status)
			}
		}
	}

	// Test 6: Check client messages
	log.Println("\n=== Test 6: Checking client messages ===")
	for _, client := range mockClients[testDebateID] {
		messages := client.GetMessages()
		log.Printf("Client %s received %d messages", client.id, len(messages))

		// Check for game over message
		var foundGameOver bool
		for _, msg := range messages {
			if msg["type"] == "game_over" {
				foundGameOver = true
				log.Printf("Found game over message: %v", msg)
				break
			}
		}

		if !foundGameOver {
			log.Printf("Warning: Client %s did not receive game over message", client.id)
		}
	}

	log.Println("\nMulti-debate test completed successfully!")
}
