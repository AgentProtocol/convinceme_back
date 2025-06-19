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
	"github.com/neo/convinceme_backend/internal/logging"
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
	server       *Server // Reference to the server for audio caching
}

// NewDebateManager creates a new debate manager
func NewDebateManager(db database.DatabaseInterface, agents map[string]*agent.Agent, apiKey string, server *Server) *DebateManager {
	scorer, err := scoring.NewScorer(apiKey)
	if err != nil {
		log.Printf("Warning: Failed to initialize scorer in DebateManager: %v", err)
	}

	manager := &DebateManager{
		db:      db,
		agents:  agents,
		debates: make(map[string]*conversation.DebateSession),
		apiKey:  apiKey,
		scorer:  scorer,
		server:  server,
	}

	// Load active debates from database into memory
	err = manager.LoadActiveDebates()
	if err != nil {
		log.Printf("Warning: Failed to load active debates: %v", err)
	}

	return manager
}

// CreateDebate creates a new debate with the given topic and agents
func (m *DebateManager) CreateDebate(topic string, agent1, agent2 *agent.Agent, createdBy string) (string, error) {
	// Generate a unique ID for the debate
	debateID := uuid.New().String()

	logging.LogDebateEvent("debate_creation_start", debateID, map[string]interface{}{
		"topic":      topic,
		"agent1":     agent1.GetName(),
		"agent2":     agent2.GetName(),
		"created_by": createdBy,
	})

	// Create debate config
	config := conversation.DefaultConfig()
	config.Topic = topic

	// Create a new debate session
	session, err := conversation.NewDebateSession(debateID, agent1, agent2, config, m.apiKey)
	if err != nil {
		logging.LogDebateEvent("debate_session_creation_failed", debateID, map[string]interface{}{
			"error": err,
			"topic": topic,
		})
		return "", fmt.Errorf("failed to create debate session: %v", err)
	}

	// Store debate in database
	err = m.db.CreateDebate(debateID, topic, "waiting", agent1.GetName(), agent2.GetName())
	if err != nil {
		logging.LogDebateEvent("debate_db_creation_failed", debateID, map[string]interface{}{
			"error": err,
			"topic": topic,
		})
		return "", fmt.Errorf("failed to store debate in database: %v", err)
	}

	// Store session in memory
	m.debatesMutex.Lock()
	m.debates[debateID] = session
	m.debatesMutex.Unlock()

	logging.LogDebateEvent("debate_created_successfully", debateID, map[string]interface{}{
		"topic":  topic,
		"agent1": agent1.GetName(),
		"agent2": agent2.GetName(),
		"status": "waiting",
	})

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
		// Add panic recovery to prevent the debate loop from crashing silently
		defer func() {
			if r := recover(); r != nil {
				logging.Error("Panic in debate loop", map[string]interface{}{
					"debate_id": session.DebateID,
					"panic":     r,
				})
				// Set debate status to finished if it panicked
				session.UpdateStatus("finished")
				session.Broadcast(gin.H{
					"type":    "error",
					"message": "Internal error occurred in debate. Debate has ended.",
				})
			}
		}()

		ctx := context.Background()
		debateID := session.DebateID

		logging.Info("Starting debate loop", map[string]interface{}{
			"debate_id": debateID,
		})

		// Generate initial message
		initialMessage := fmt.Sprintf("Welcome to the debate on: %s", session.Config.Topic)
		session.Broadcast(gin.H{
			"type":    "system",
			"message": initialMessage,
		})

		// Add a slight delay before first agent speaks
		time.Sleep(2 * time.Second)

		// Set debate timeout to 15 minutes
		debateTimeout := time.NewTimer(15 * time.Minute)
		defer debateTimeout.Stop()

		// Add heartbeat to monitor debate progress
		lastActivityTime := time.Now()
		maxInactivityDuration := 5 * time.Minute // If no progress for 5 minutes, something is wrong

		// Main debate loop - continue until winner or timeout
		agentTurnCount := 0 // Add counter to track agent turns

		for {
			// Check for timeout
			select {
			case <-debateTimeout.C:
				logging.Info("Debate timed out", map[string]interface{}{
					"debate_id":        debateID,
					"timeout_duration": "15m",
				})
				session.UpdateStatus("finished")
				session.Broadcast(gin.H{
					"type":    "timeout",
					"message": "Debate timed out after 15 minutes. No winner determined.",
				})
				return
			default:
				// Continue with debate logic
			}

			// Check for inactivity (debate stuck)
			if time.Since(lastActivityTime) > maxInactivityDuration {
				logging.Error("Debate appears stuck - no progress for too long", map[string]interface{}{
					"debate_id":           debateID,
					"last_activity":       lastActivityTime,
					"inactivity_duration": time.Since(lastActivityTime),
					"max_allowed":         maxInactivityDuration,
				})
				session.UpdateStatus("finished")
				session.Broadcast(gin.H{
					"type":    "error",
					"message": "Debate ended due to inactivity. No progress detected for 5 minutes.",
				})
				return
			}

			// Check if debate should continue
			status := session.GetStatus()
			if status != "active" {
				logging.Info("Debate no longer active, ending loop", map[string]interface{}{
					"debate_id": debateID,
					"status":    status,
				})
				break
			}

			// Increment agent turn counter
			agentTurnCount++
			logging.Info("Starting agent turn", map[string]interface{}{
				"debate_id": debateID,
				"turn":      agentTurnCount,
			})

			// Add a small delay to allow for player interruptions
			time.Sleep(1 * time.Second)

			// Get next agent to speak
			agent := session.GetNextAgent()
			agentName := agent.GetName()
			logging.Info("Agent will speak", map[string]interface{}{
				"debate_id":  debateID,
				"agent_name": agentName,
				"turn":       agentTurnCount,
			})

			// Get context from recent history
			recentHistory := session.GetRecentHistory(5)
			var contextStr string
			for _, entry := range recentHistory {
				contextStr += fmt.Sprintf("%s: %s\n", entry.Speaker, entry.Message)
			}
			logging.Debug("Context for agent", map[string]interface{}{
				"debate_id": debateID,
				"turn":      agentTurnCount,
				"context":   contextStr,
			})

			// Generate response
			prompt := getPrompt(contextStr, "", agentName, "Debate Participant", session.Config.Topic)
			logging.Info("Calling agent.GenerateResponse", map[string]interface{}{
				"debate_id":  debateID,
				"agent_name": agentName,
				"turn":       agentTurnCount,
			})
			response, err := agent.GenerateResponse(ctx, session.Config.Topic, prompt)
			if err != nil {
				logging.Error("Error generating response", map[string]interface{}{
					"debate_id":  debateID,
					"agent_name": agentName,
					"turn":       agentTurnCount,
					"error":      err.Error(),
				})
				continue
			}
			logging.Info("Successfully generated response", map[string]interface{}{
				"debate_id":  debateID,
				"agent_name": agentName,
				"turn":       agentTurnCount,
				"response":   response,
			})

			// Update activity time - we made progress!
			lastActivityTime = time.Now()

			// Add to history - scoring will be done later
			session.AddHistoryEntry(agentName, response, false)
			logging.Info("Added response to history", map[string]interface{}{
				"debate_id":  debateID,
				"agent_name": agentName,
				"turn":       agentTurnCount,
			})

			// Generate audio for the response
			logging.Info("Generating audio", map[string]interface{}{
				"debate_id":  debateID,
				"agent_name": agentName,
				"turn":       agentTurnCount,
			})
			audioData, err := agent.GenerateAndStreamAudio(ctx, response)
			var audioURL string
			if err != nil {
				logging.Error("Error generating audio", map[string]interface{}{
					"debate_id":  debateID,
					"agent_name": agentName,
					"turn":       agentTurnCount,
					"error":      err.Error(),
				})
			} else {
				// Store audio in cache and get URL
				audioURL = m.server.CacheAudio(audioData)
				logging.Info("Generated audio", map[string]interface{}{
					"debate_id":  debateID,
					"agent_name": agentName,
					"turn":       agentTurnCount,
					"audio_url":  audioURL,
				})
			}

			// Score the argument
			logging.Info("Scoring argument", map[string]interface{}{
				"debate_id":  debateID,
				"agent_name": agentName,
				"turn":       agentTurnCount,
			})
			score, err := m.scorer.ScoreArgument(ctx, response, session.Config.Topic)
			if err != nil {
				logging.Error("Error scoring response", map[string]interface{}{
					"debate_id":  debateID,
					"agent_name": agentName,
					"turn":       agentTurnCount,
					"error":      err.Error(),
				})
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
			} else {
				logging.Info("Successfully scored argument", map[string]interface{}{
					"debate_id":  debateID,
					"agent_name": agentName,
					"turn":       agentTurnCount,
					"score":      score.Average,
				})
			}

			// Update the history entry with the score
			session.UpdateLastHistoryEntryScore(score.Average)

			// Update game score based on comparative performance
			// Compare current agent's score with opponent's recent average score
			currentAgentScore := score.Average

			// Get opponent's name and recent score
			var opponentName string
			if agentName == session.Agent1.GetName() {
				opponentName = session.Agent2.GetName()
			} else {
				opponentName = session.Agent1.GetName()
			}

			opponentRecentScore := session.GetRecentAgentScore(opponentName, 3) // Get opponent's recent performance

			// Calculate performance difference
			scoreDifference := currentAgentScore - opponentRecentScore

			// Determine HP changes based on who performed worse
			var hpLoss int
			var weakerSide string

			if scoreDifference > 0.5 {
				// Current agent performed significantly better - opponent loses HP
				hpLoss = int(scoreDifference * 4) // Scale the difference
				if hpLoss > 20 {
					hpLoss = 20 // Cap maximum loss
				}
				if hpLoss < 3 {
					hpLoss = 3 // Minimum meaningful loss
				}

				if agentName == session.Agent1.GetName() {
					weakerSide = session.Agent2.GetName()
				} else {
					weakerSide = session.Agent1.GetName()
				}
			} else if scoreDifference < -0.5 {
				// Current agent performed significantly worse - current agent loses HP
				hpLoss = int(-scoreDifference * 4) // Scale the difference
				if hpLoss > 20 {
					hpLoss = 20 // Cap maximum loss
				}
				if hpLoss < 3 {
					hpLoss = 3 // Minimum meaningful loss
				}
				weakerSide = agentName
			} else {
				// Performance is roughly equal - no HP loss
				hpLoss = 0
				weakerSide = ""
			}

			logging.Info("Calculated HP loss based on comparative performance", map[string]interface{}{
				"debate_id":             debateID,
				"current_agent":         agentName,
				"current_score":         currentAgentScore,
				"opponent_recent_score": opponentRecentScore,
				"score_difference":      scoreDifference,
				"hp_loss":               hpLoss,
				"weaker_side":           weakerSide,
			})

			// Apply HP loss to the weaker performing side
			var agent1Delta, agent2Delta int
			if hpLoss > 0 && weakerSide != "" {
				if weakerSide == session.Agent1.GetName() {
					agent1Delta = -hpLoss // Agent1 loses HP for weaker argument
					agent2Delta = 0       // Agent2 unchanged
				} else {
					agent1Delta = 0       // Agent1 unchanged
					agent2Delta = -hpLoss // Agent2 loses HP for weaker argument
				}
			} else {
				// No HP changes for equal performance
				agent1Delta = 0
				agent2Delta = 0
			}

			gameScore := session.UpdateGameScore(agent1Delta, agent2Delta)
			logging.Info("Updated game score", map[string]interface{}{
				"debate_id":    debateID,
				"turn":         agentTurnCount,
				"agent1_score": gameScore.Agent1Score,
				"agent2_score": gameScore.Agent2Score,
				"agent1_delta": agent1Delta,
				"agent2_delta": agent2Delta,
			})

			// Check for game over condition
			var gameOver bool
			var winner string

			if gameScore.Agent1Score <= 0 {
				gameOver = true
				winner = session.Agent2.GetName()
				logging.Info("Game over - Agent1 health depleted", map[string]interface{}{
					"debate_id":    debateID,
					"winner":       winner,
					"agent1_score": gameScore.Agent1Score,
					"agent2_score": gameScore.Agent2Score,
					"turn":         agentTurnCount,
				})
			} else if gameScore.Agent2Score <= 0 {
				gameOver = true
				winner = session.Agent1.GetName()
				logging.Info("Game over - Agent2 health depleted", map[string]interface{}{
					"debate_id":    debateID,
					"winner":       winner,
					"agent1_score": gameScore.Agent1Score,
					"agent2_score": gameScore.Agent2Score,
					"turn":         agentTurnCount,
				})
			}

			// Broadcast response with score
			message := gin.H{
				"type":    "message",
				"agent":   agentName,
				"content": response, // Changed from "message" to "content" to match frontend
				"scores": gin.H{
					"argument": score,
				},
			}

			// Add audio URL if available
			if audioURL != "" {
				message["audioUrl"] = audioURL
			}

			session.Broadcast(message)

			// Also broadcast separate audio message for frontend audio player
			if audioURL != "" {
				session.Broadcast(gin.H{
					"type":     "audio",
					"audioUrl": audioURL,
					"agent":    agentName,
				})
			}

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

		// Debate ended due to status change (winner determined, timeout, or error)
		logging.Info("Debate loop ended", map[string]interface{}{
			"debate_id":    debateID,
			"final_status": session.GetStatus(),
			"total_turns":  agentTurnCount,
		})
	}()
}

// NormalizeScore normalizes a score to a 0-100 scale for display
func (m *DebateManager) NormalizeScore(score int) float64 {
	// Since we start at 100 HP and use sum of parameters, keep original scale
	// Just ensure they stay within reasonable bounds
	if score < 0 {
		return 0
	}
	if score > 200 {
		return 200
	}
	return float64(score)
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

// LoadActiveDebates loads active debates from the database into memory
func (m *DebateManager) LoadActiveDebates() error {
	m.debatesMutex.Lock()
	defer m.debatesMutex.Unlock()

	// Get active debates from database
	debates, err := m.db.ListActiveDebates()
	if err != nil {
		return fmt.Errorf("failed to list active debates: %v", err)
	}

	log.Printf("Loading %d active debates into memory", len(debates))

	for _, debate := range debates {
		// Get agents for this debate
		agent1, exists1 := m.agents[debate.Agent1Name]
		agent2, exists2 := m.agents[debate.Agent2Name]

		if !exists1 || !exists2 {
			log.Printf("Warning: Skipping debate %s - missing agents (Agent1: %s exists: %v, Agent2: %s exists: %v)",
				debate.ID, debate.Agent1Name, exists1, debate.Agent2Name, exists2)
			continue
		}

		// Create debate config
		config := conversation.DefaultConfig()
		config.Topic = debate.Topic

		// Create new debate session
		session, err := conversation.NewDebateSession(debate.ID, agent1, agent2, config, m.apiKey)
		if err != nil {
			log.Printf("Warning: Failed to create session for debate %s: %v", debate.ID, err)
			continue
		}

		// Set the correct status from database
		session.UpdateStatus(debate.Status)

		// Store in memory
		m.debates[debate.ID] = session

		log.Printf("Loaded debate %s (%s) into memory with status: %s", debate.ID, debate.Topic, debate.Status)
	}

	log.Printf("Successfully loaded %d debates into memory", len(m.debates))
	return nil
}

// GetDebateInfo returns comprehensive information about a debate for reconnecting clients
func (m *DebateManager) GetDebateInfo(debateID string) (map[string]interface{}, error) {
	m.debatesMutex.RLock()
	defer m.debatesMutex.RUnlock()

	session, exists := m.debates[debateID]
	if !exists {
		return nil, fmt.Errorf("debate not found")
	}

	// Get current game scores
	gameScore := session.GetGameScore()

	// Get recent history for catch-up
	recentHistory := session.GetRecentHistory(10)

	// Convert history to a format suitable for frontend
	historyData := make([]map[string]interface{}, 0, len(recentHistory))
	for _, entry := range recentHistory {
		historyData = append(historyData, map[string]interface{}{
			"speaker":   entry.Speaker,
			"message":   entry.Message,
			"time":      entry.Time,
			"is_player": entry.IsPlayer,
		})
	}

	debateInfo := map[string]interface{}{
		"debate_id": debateID,
		"status":    session.GetStatus(),
		"topic":     session.Config.Topic,
		"agent1":    session.Agent1.GetName(),
		"agent2":    session.Agent2.GetName(),
		"game_score": map[string]interface{}{
			session.Agent1.GetName(): m.NormalizeScore(gameScore.Agent1Score),
			session.Agent2.GetName(): m.NormalizeScore(gameScore.Agent2Score),
		},
		"internal_score": map[string]interface{}{
			session.Agent1.GetName(): gameScore.Agent1Score,
			session.Agent2.GetName(): gameScore.Agent2Score,
		},
		"history":      historyData,
		"client_count": len(session.Clients),
		"is_active":    session.GetStatus() == "active",
	}

	return debateInfo, nil
}
