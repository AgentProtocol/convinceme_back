package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket" // Added for Clients map
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/logging"
	"github.com/neo/convinceme_backend/internal/player"
	"github.com/neo/convinceme_backend/internal/tools"
	"github.com/neo/convinceme_backend/internal/types"
)

// DebateConfig holds configuration for the debate session
type DebateConfig struct {
	Topic               string
	MaxTurns            int           // Might be less relevant if debates run until a winner or manually stopped
	TurnDelay           time.Duration // Delay between agent turns
	ResponseStyle       types.ResponseStyle
	MaxCompletionTokens int
	TemperatureHigh     bool
}

// DefaultConfig returns a default configuration for a debate
func DefaultConfig() DebateConfig {
	return DebateConfig{
		Topic:               "Are memecoins net negative or positive for the crypto space?",
		MaxTurns:            20, // Increased from 10 to allow for longer debates
		TurnDelay:           500 * time.Millisecond,
		ResponseStyle:       types.ResponseStyleDebate,
		MaxCompletionTokens: 150,
		TemperatureHigh:     true,
	}
}

// DebateEntry represents a single message in the debate history
type DebateEntry struct {
	Speaker  string    `json:"speaker"` // Agent name or Player ID
	Message  string    `json:"message"`
	Time     time.Time `json:"time"`
	IsPlayer bool      `json:"is_player"`
}

// GameScore tracks the scores within a debate session
type GameScore struct {
	Agent1Score int
	Agent2Score int
	// Add agent names if needed, or get from DebateSession.agent1/2
}

// DebateSession manages the state and logic for a single debate instance
type DebateSession struct {
	DebateID    string                     `json:"debate_id"`
	Agent1      *agent.Agent               `json:"-"` // Exclude agents from JSON serialization
	Agent2      *agent.Agent               `json:"-"`
	Config      DebateConfig               `json:"config"`
	Status      string                     `json:"status"` // e.g., "waiting", "active", "finished"
	Clients     map[*websocket.Conn]string `json:"-"`      // Map of client connections to Player IDs for this debate
	History     []DebateEntry              `json:"history"`
	GameScore   GameScore                  `json:"game_score"`
	Judge       *tools.ConvictionJudge     `json:"-"` // Optional: Judge for analysis
	debateMutex sync.RWMutex               // Mutex for protecting session state (Clients, History, Status, GameScore)
	// Add other necessary fields like stopChannel, lastSpeaker, etc.
	stopChannel chan struct{}
	lastSpeaker string
}

// NewDebateSession creates a new debate session
func NewDebateSession(id string, agent1, agent2 *agent.Agent, config DebateConfig, apiKey string) (*DebateSession, error) {
	if !config.ResponseStyle.IsValid() {
		config.ResponseStyle = types.ResponseStyleDebate // Default to debate style
	}

	// Optional: Initialize judge if needed for this session
	judge, err := tools.NewConvictionJudge(apiKey)
	if err != nil {
		log.Printf("Warning: Failed to create conviction judge for debate %s: %v", id, err)
		// Decide if judge is critical or optional
	}

	// Initialize GameScore (starting at 100 HP each)
	initialScore := 100 // Start with 100 HP for both agents

	return &DebateSession{
		DebateID:    id,
		Agent1:      agent1,
		Agent2:      agent2,
		Config:      config,
		Status:      "waiting", // Initial status
		Clients:     make(map[*websocket.Conn]string),
		History:     make([]DebateEntry, 0),
		GameScore:   GameScore{Agent1Score: initialScore, Agent2Score: initialScore},
		Judge:       judge,
		stopChannel: make(chan struct{}),
	}, nil
}

// Start method might be removed or repurposed.
// The core agent discussion loop will likely be managed by the server/manager
// and triggered when the status becomes 'active'.
// func (d *DebateSession) Start(ctx context.Context) error { ... }

// AddClient adds a WebSocket client to the session
func (d *DebateSession) AddClient(conn *websocket.Conn, playerID string) {
	d.debateMutex.Lock()
	defer d.debateMutex.Unlock()
	d.Clients[conn] = playerID
	log.Printf("Player %s joined debate %s. Total clients: %d", playerID, d.DebateID, len(d.Clients))
}

// RemoveClient removes a WebSocket client from the session
func (d *DebateSession) RemoveClient(conn *websocket.Conn) (playerID string, remaining int) {
	d.debateMutex.Lock()
	defer d.debateMutex.Unlock()
	playerID = d.Clients[conn]
	delete(d.Clients, conn)
	remaining = len(d.Clients)
	log.Printf("Player %s left debate %s. Remaining clients: %d", playerID, d.DebateID, remaining)
	return playerID, remaining
}

// Broadcast sends a message to all clients in this debate session
func (d *DebateSession) Broadcast(message interface{}) {
	d.debateMutex.RLock()
	defer d.debateMutex.RUnlock()

	clientCount := len(d.Clients)

	logging.LogWebSocketEvent("broadcast_start", d.DebateID, "", map[string]interface{}{
		"client_count": clientCount,
		"message_type": fmt.Sprintf("%T", message),
	})

	if clientCount == 0 {
		logging.LogWebSocketEvent("broadcast_no_clients", d.DebateID, "", map[string]interface{}{})
		return
	}

	successCount := 0
	errorCount := 0

	for client := range d.Clients {
		// Write synchronously to avoid concurrent writes to the same connection
		// Each WebSocket connection can only have one writer at a time
		if err := client.WriteJSON(message); err != nil {
			errorCount++
			logging.LogWebSocketEvent("broadcast_client_error", d.DebateID, "", map[string]interface{}{
				"error": err,
			})
			// Optional: Consider removing the client if write fails repeatedly
		} else {
			successCount++
		}
	}

	logging.LogWebSocketEvent("broadcast_completed", d.DebateID, "", map[string]interface{}{
		"success_count": successCount,
		"error_count":   errorCount,
	})
}

// AddHistoryEntry adds a message to the debate history safely
func (d *DebateSession) AddHistoryEntry(speaker string, message string, isPlayer bool) {
	d.debateMutex.Lock()
	defer d.debateMutex.Unlock()
	entry := DebateEntry{
		Speaker:  speaker,
		Message:  message,
		Time:     time.Now(),
		IsPlayer: isPlayer,
	}
	d.History = append(d.History, entry)
	// Optional: Limit history size if needed
}

// GetRecentHistory retrieves the last N entries safely
func (d *DebateSession) GetRecentHistory(n int) []DebateEntry {
	d.debateMutex.RLock()
	defer d.debateMutex.RUnlock()
	start := len(d.History) - n
	if start < 0 {
		start = 0
	}
	// Return a copy to avoid race conditions if the caller modifies it
	historyCopy := make([]DebateEntry, len(d.History[start:]))
	copy(historyCopy, d.History[start:])
	return historyCopy
}

// UpdateStatus updates the debate status safely
func (d *DebateSession) UpdateStatus(newStatus string) {
	d.debateMutex.Lock()
	defer d.debateMutex.Unlock()
	if d.Status != newStatus {
		log.Printf("Debate %s status changed from %s to %s", d.DebateID, d.Status, newStatus)
		d.Status = newStatus
	}
}

// GetStatus retrieves the current status safely
func (d *DebateSession) GetStatus() string {
	d.debateMutex.RLock()
	defer d.debateMutex.RUnlock()
	return d.Status
}

// UpdateGameScore updates the scores safely
func (d *DebateSession) UpdateGameScore(agent1Delta, agent2Delta int) GameScore {
	d.debateMutex.Lock()
	defer d.debateMutex.Unlock()
	d.GameScore.Agent1Score += agent1Delta
	d.GameScore.Agent2Score += agent2Delta
	// Optional: Add score clamping logic (e.g., min/max scores)
	log.Printf("Debate %s score updated: Agent1=%d, Agent2=%d", d.DebateID, d.GameScore.Agent1Score, d.GameScore.Agent2Score)
	return d.GameScore // Return updated score
}

// GetGameScore retrieves the current scores safely
func (d *DebateSession) GetGameScore() GameScore {
	d.debateMutex.RLock()
	defer d.debateMutex.RUnlock()
	return d.GameScore
}

// GetStopChannel returns the channel used to signal the debate loop to stop
func (d *DebateSession) GetStopChannel() chan struct{} {
	// No lock needed as the channel itself is generally safe for reading
	return d.stopChannel
}

// CheckStatusAndClients returns the current status and client count safely
func (d *DebateSession) CheckStatusAndClients() (status string, clientCount int) {
	d.debateMutex.RLock()
	defer d.debateMutex.RUnlock()
	return d.Status, len(d.Clients)
}

// GetNextAgent determines which agent should speak next
func (d *DebateSession) GetNextAgent() *agent.Agent {
	d.debateMutex.Lock() // Lock needed to safely read and write lastSpeaker
	defer d.debateMutex.Unlock()

	if d.lastSpeaker == d.Agent1.GetName() {
		d.lastSpeaker = d.Agent2.GetName()
		return d.Agent2
	}
	// Default to Agent1 if no last speaker or if Agent2 spoke last
	d.lastSpeaker = d.Agent1.GetName()
	return d.Agent1
}

// HandlePlayerInterruption processes a player message and determines if it should interrupt the agent conversation
func (d *DebateSession) HandlePlayerInterruption(playerID, message string) bool {
	// Add the player message to history
	d.AddHistoryEntry(playerID, message, true)

	// For now, we'll consider any player message as an interruption
	// In the future, this could be more sophisticated, e.g., checking if the message
	// is directed at a specific agent or contains a question
	return true
}

// --- Methods below might be removed or significantly changed ---

// Start begins the conversation between the agents (Likely to be removed/refactored)
func (d *DebateSession) Start(ctx context.Context) error {
	d.debateMutex.Lock()
	if d.Status != "waiting" {
		d.debateMutex.Unlock()
		return fmt.Errorf("debate %s already started or finished", d.DebateID)
	}
	d.Status = "active"
	d.debateMutex.Unlock()

	log.Printf("Starting debate %s on topic: %s", d.DebateID, d.Config.Topic)

	// The actual agent discussion loop will likely be managed elsewhere (e.g., DebateManager)
	// This method might just set the status and let the manager handle the loop.

	// Example placeholder for the old logic structure:
	// var lastMessage string = fmt.Sprintf("Let's start discussing: %s", d.Config.Topic) // Removed unused variable
	// Placeholder for the loop - actual loop logic will move
	for turn := 0; turn < d.Config.MaxTurns; turn++ {
		// This loop needs complete refactoring to fit the new manager/session model
		log.Printf("Debate %s Turn %d (Placeholder)", d.DebateID, turn+1)
		time.Sleep(1 * time.Second) // Simulate work

		// Check if stopped
		select {
		case <-d.stopChannel:
			log.Printf("Debate %s received stop signal during placeholder loop.", d.DebateID)
			d.UpdateStatus("finished") // Or another appropriate status
			return nil
		default:
			// Continue
		}
	}

	d.UpdateStatus("finished") // Mark as finished after placeholder turns
	log.Printf("Debate %s finished (Placeholder)", d.DebateID)
	return nil
}

// analyzeConviction uses the ConvictionJudge to analyze the current debate (Refactored)
func (d *DebateSession) analyzeConviction(ctx context.Context) error {
	if d.Judge == nil {
		return fmt.Errorf("conviction judge not initialized for debate %s", d.DebateID)
	}

	d.debateMutex.RLock()
	historyCopy := make([]DebateEntry, len(d.History))
	copy(historyCopy, d.History)
	agent1Name := d.Agent1.GetName()
	agent2Name := d.Agent2.GetName()
	d.debateMutex.RUnlock()

	// Convert DebateEntry to the format expected by the judge tool if necessary
	// Assuming the judge tool expects []ConversationEntry format for now
	judgeHistory := make([]tools.ConversationEntry, 0, len(historyCopy))
	for _, entry := range historyCopy {
		judgeHistory = append(judgeHistory, tools.ConversationEntry{
			Speaker: entry.Speaker,
			Message: entry.Message,
			// Time/IsPlayer might not be needed by the judge tool
		})
	}

	conversationData := map[string]interface{}{
		"agent1_name":  agent1Name,
		"agent2_name":  agent2Name,
		"conversation": judgeHistory, // Use the converted history
	}

	conversationJSON, err := json.Marshal(conversationData)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation data for debate %s: %v", d.DebateID, err)
	}

	metricsJSON, err := d.Judge.Call(ctx, string(conversationJSON))
	if err != nil {
		return fmt.Errorf("failed to analyze conviction for debate %s: %v", d.DebateID, err)
	}

	var metrics tools.ConvictionMetrics
	if err := json.Unmarshal([]byte(metricsJSON), &metrics); err != nil {
		return fmt.Errorf("failed to parse conviction metrics for debate %s: %v", d.DebateID, err)
	}

	log.Printf("\n=== Conviction Analysis (Debate: %s) ===\n"+
		"%s Conviction: %.2f\n"+
		"%s Conviction: %.2f\n"+
		"Overall Tension: %.2f\n"+
		"Dominant Speaker: %s\n"+
		"Analysis: %s\n"+
		"========================\n",
		d.DebateID,
		agent1Name, metrics.Agent1Score,
		agent2Name, metrics.Agent2Score,
		metrics.OverallTension,
		metrics.DominantAgent,
		metrics.AnalysisSummary)

	// Optional: Store metrics or update debate state based on analysis
	return nil
}

// handlePlayerInput processes player input within the context of this debate session
// This logic will likely move to the DebateManager or be called by it.
func (d *DebateSession) handlePlayerInput(ctx context.Context, playerID string, input player.PlayerInput) error { // Added playerID parameter
	log.Printf("Handling player input from %s for debate %s: %s", playerID, d.DebateID, input.Content)

	// 1. Add player message to history
	d.AddHistoryEntry(playerID, input.Content, true) // Use passed-in playerID

	// 2. Score the player's argument (requires scorer instance)
	// scorer := ... // Need access to a scorer, maybe passed in or part of DebateSession
	// score, err := scorer.ScoreArgument(ctx, input.Content, d.Config.Topic)
	// if err != nil { ... }
	// d.db.SaveArgument(...) // Need access to db
	// d.db.SaveScore(...)

	// 3. Update Game Score based on player's argument score and side
	// side := ... // Determine which agent the player supported
	// scoreDelta := int(score.Average) // Or some calculation
	// if side == d.Agent1.GetName() {
	//     d.UpdateGameScore(scoreDelta, -scoreDelta)
	// } else {
	//     d.UpdateGameScore(-scoreDelta, scoreDelta)
	// }

	// 4. Broadcast updated score and argument analysis
	// d.Broadcast(...)

	// 5. Potentially trigger the next agent's response based on player input
	// This depends on the desired interaction flow.

	return fmt.Errorf("handlePlayerInput logic needs further implementation")
}

// getPromptStyle returns the prompt modification based on response style (Refactored)
func (d *DebateSession) getPromptStyle() string {
	switch d.Config.ResponseStyle {
	case types.ResponseStyleFormal:
		return "Maintain a formal and professional tone."
	case types.ResponseStyleCasual:
		return "Keep the tone casual and friendly."
	case types.ResponseStyleTechnical:
		return "Use technical language and precise terminology."
	case types.ResponseStyleDebate:
		return "Use persuasive and argumentative language."
	case types.ResponseStyleHumorous:
		return "Keep the tone light and humorous."
	default:
		return "Keep the tone casual and friendly." // Default fallback
	}
}

// Note: playerInputProcessor is removed as input handling will be managed differently.
// Note: getAgentNumber is removed as agent identification is handled within the session.
