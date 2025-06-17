package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin" // Add missing gin import
	"github.com/google/uuid"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/conversation"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/neo/convinceme_backend/internal/scoring"
)

// DebateManager handles the creation, tracking, and cleanup of debate sessions
type DebateManager struct {
	db           database.DatabaseInterface
	agents       map[string]*agent.Agent
	debates      map[string]*conversation.DebateSession
	debatesMutex sync.RWMutex
	apiKey       string
	scorer       *scoring.Scorer
}

// NewDebateManager creates a new debate manager
func NewDebateManager(db database.DatabaseInterface, agents map[string]*agent.Agent, apiKey string) *DebateManager {
	scorer, err := scoring.NewScorer(apiKey)
	if err != nil {
		log.Printf("Warning: Failed to initialize scorer in DebateManager: %v", err)
	}

	return &DebateManager{
		db:      db,
		agents:  agents,
		debates: make(map[string]*conversation.DebateSession),
		apiKey:  apiKey,
		scorer:  scorer,
	}
}

// CreateDebate creates a new debate with the given topic and agents
func (m *DebateManager) CreateDebate(topic string, agent1, agent2 *agent.Agent, createdBy string) (string, error) {
	// Generate a unique ID for the debate
	debateID := uuid.New().String()

	// Create debate config
	config := conversation.DefaultConfig()
	config.Topic = topic

	// Create a new debate session
	session, err := conversation.NewDebateSession(debateID, agent1, agent2, config, m.apiKey)
	if err != nil {
		return "", fmt.Errorf("failed to create debate session: %v", err)
	}

	// Store debate in database
	err = m.db.CreateDebate(debateID, topic, "waiting", agent1.GetName(), agent2.GetName())
	if err != nil {
		return "", fmt.Errorf("failed to store debate in database: %v", err)
	}

	// Store session in memory
	m.debatesMutex.Lock()
	m.debates[debateID] = session
	m.debatesMutex.Unlock()

	log.Printf("Created new debate %s on topic '%s' with agents %s vs %s",
		debateID, topic, agent1.GetName(), agent2.GetName())

	return debateID, nil
}

// GetDebate retrieves a debate session by ID
func (m *DebateManager) GetDebate(debateID string) (*conversation.DebateSession, bool) {
	m.debatesMutex.RLock()
	defer m.debatesMutex.RUnlock()

	session, exists := m.debates[debateID]
	return session, exists
}

// StartDebateLoop starts the debate loop for a session
func (m *DebateManager) StartDebateLoop(session *conversation.DebateSession) {
	// Start the debate loop in a goroutine
	go func() {
		ctx := context.Background()
		debateID := session.DebateID

		log.Printf("Starting debate loop for debate %s", debateID)

		// Generate initial message
		initialMessage := fmt.Sprintf("Welcome to the debate on: %s", session.Config.Topic)
		session.Broadcast(gin.H{
			"type":    "system",
			"message": initialMessage,
		})

		// Add a slight delay before first agent speaks
		time.Sleep(2 * time.Second)

		// Main debate loop
		for turn := 0; turn < session.Config.MaxTurns; turn++ {
			// Check if debate should continue
			status := session.GetStatus()
			if status != "active" {
				log.Printf("Debate %s is no longer active (status: %s). Ending loop.", debateID, status)
				break
			}

			// Add a small delay to allow for player interruptions
			time.Sleep(1 * time.Second)

			// Get next agent to speak
			agent := session.GetNextAgent()
			agentName := agent.GetName()

			// Get context from recent history
			recentHistory := session.GetRecentHistory(5)
			var contextStr string
			for _, entry := range recentHistory {
				contextStr += fmt.Sprintf("%s: %s\n", entry.Speaker, entry.Message)
			}

			// Generate response
			prompt := getPrompt(contextStr, "", agentName, "Debate Participant", session.Config.Topic)
			response, err := agent.GenerateResponse(ctx, session.Config.Topic, prompt)
			if err != nil {
				log.Printf("Error generating response for %s in debate %s: %v", agentName, debateID, err)
				continue
			}

			// Add to history
			session.AddHistoryEntry(agentName, response, false)

			// Score the argument
			score, err := m.scorer.ScoreArgument(ctx, response, session.Config.Topic)
			if err != nil {
				log.Printf("Error scoring response for %s in debate %s: %v", agentName, debateID, err)
				// Create a default score rather than skipping scoring entirely
				score = &scoring.ArgumentScore{
					Strength:    5,
					Relevance:   5,
					Logic:       5,
					Truth:       5,
					Humor:       5,
					Average:     5.0,
					Explanation: "Failed to calculate score",
				}
			}

			// Update game score based on agent and score
			var agent1Delta, agent2Delta int
			scoreDelta := int(score.Average)

			if agentName == session.Agent1.GetName() {
				agent1Delta = scoreDelta
				agent2Delta = -scoreDelta
			} else {
				agent1Delta = -scoreDelta
				agent2Delta = scoreDelta
			}

			gameScore := session.UpdateGameScore(agent1Delta, agent2Delta)

			// Check for game over condition
			var gameOver bool
			var winner string

			if gameScore.Agent1Score <= 0 {
				gameOver = true
				winner = session.Agent2.GetName()
			} else if gameScore.Agent2Score <= 0 {
				gameOver = true
				winner = session.Agent1.GetName()
			}

			// Broadcast response with score
			session.Broadcast(gin.H{
				"type":    "message",
				"agent":   agentName,
				"message": response,
				"scores": gin.H{
					"argument": score,
				},
			})

			// Broadcast updated game score
			session.Broadcast(gin.H{
				"type": "game_score",
				"gameScore": gin.H{
					session.Agent1.GetName(): m.NormalizeScore(gameScore.Agent1Score),
					session.Agent2.GetName(): m.NormalizeScore(gameScore.Agent2Score),
				},
				"internalScore": gin.H{
					session.Agent1.GetName(): gameScore.Agent1Score,
					session.Agent2.GetName(): gameScore.Agent2Score,
				},
			})

			// If game over, end debate
			if gameOver {
				log.Printf("Game over in debate %s. Winner: %s", debateID, winner)

				// Update status in memory
				session.UpdateStatus("finished")

				// Update database
				err := m.db.UpdateDebateEnd(debateID, "finished", winner)
				if err != nil {
					log.Printf("Error updating debate end in database: %v", err)
				}

				// Broadcast game over message
				session.Broadcast(gin.H{
					"type":    "game_over",
					"winner":  winner,
					"message": fmt.Sprintf("Game over! %s has won the debate!", winner),
				})

				break
			}

			// Pause between turns
			time.Sleep(session.Config.TurnDelay)
		}

		// If we reached max turns without a winner
		if session.GetStatus() == "active" {
			log.Printf("Debate %s reached maximum turns without a winner", debateID)
			session.UpdateStatus("finished")
			m.db.UpdateDebateEnd(debateID, "finished", "")
		}
	}()
}

// NormalizeScore normalizes a score to a 0-10 scale for display
func (m *DebateManager) NormalizeScore(score int) float64 {
	maxScore := 200.0
	normalized := float64(score) / maxScore * 10.0
	if normalized < 0 {
		return 0
	}
	if normalized > 10 {
		return 10
	}
	return normalized
}

// RemoveDebate removes a debate from the manager
func (m *DebateManager) RemoveDebate(debateID string) {
	m.debatesMutex.Lock()
	defer m.debatesMutex.Unlock()

	delete(m.debates, debateID)
	log.Printf("Removed debate %s from manager", debateID)
}

// Add a periodic cleanup method to run in the server
func (m *DebateManager) StartPeriodicCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			m.CleanupInactiveDebates()
		}
	}()
}

// CleanupInactiveDebates removes finished debates that have been inactive for a certain period
func (m *DebateManager) CleanupInactiveDebates() {
	m.debatesMutex.Lock()
	defer m.debatesMutex.Unlock()

	// Define cutoff time (24 hours ago)
	//cutoff := time.Now().Add(-24 * time.Hour)
	var toRemove []string

	for id, session := range m.debates {
		if session.GetStatus() == "finished" {
			// TODO: Currently removing all finished debates without checking inactivity period
			// This could be enhanced by adding a "finished_at" timestamp to DebateSession
			// and comparing: if finishedAt.Before(cutoff) { ... }

			// For now, let's assume all finished debates are older than cutoff
			toRemove = append(toRemove, id)

			// Alternatively, we could fetch the debate from the database to check ended_at
			// debate, err := m.db.GetDebate(id)
			// if err == nil && debate.EndedAt != nil && debate.EndedAt.Before(cutoff) {
			//     toRemove = append(toRemove, id)
			// }
		}
	}

	for _, id := range toRemove {
		delete(m.debates, id)
		log.Printf("Cleaned up inactive debate %s", id)
	}
}
