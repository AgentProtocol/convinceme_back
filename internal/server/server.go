package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"strings"

	// "math" // Removed unused import
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/neo/convinceme_backend/internal/audio"
	"github.com/neo/convinceme_backend/internal/conversation"
	"github.com/quic-go/quic-go/http3"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/neo/convinceme_backend/internal/scoring"
)

type Server struct {
	router        *gin.Engine
	agents        map[string]*agent.Agent
	audioCache    map[string]audioCache
	cacheMutex    sync.RWMutex
	useHTTPS      bool
	config        *Config
	scorer        *scoring.Scorer
	db            *database.Database
	debateManager *DebateManager // Manages all debate sessions
}

// DebateEntry struct remains here for now, might move if logging moves entirely
type DebateEntry struct {
	Speaker  string    `json:"speaker"`
	Message  string    `json:"message"`
	Time     time.Time `json:"time"`
	IsPlayer bool      `json:"is_player"`
	Topic    string    `json:"topic"`
}

type ConversationMessage struct {
	PlayerID string `json:"player_id"`
	Topic    string `json:"topic"`
	Message  string `json:"message"`
	Type     string `json:"type"`
	Side     string `json:"side"`
}

type audioCache struct {
	data      []byte
	timestamp time.Time
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	EnableCompression: true,
}

// Define constants for agent roles
const (
	TIGER_AGENT = "'Fundamentals First' Bradford"
	BEAR_AGENT  = "'Memecoin Supercycle' Murad"
	MAX_SCORE   = 200
)

func NewServer(agents map[string]*agent.Agent, db *database.Database, apiKey string, useHTTPS bool, config *Config) *Server {
	// Initialize player queue tracking (Scorer remains part of Server for now)
	scorer, err := scoring.NewScorer(apiKey)
	if err != nil {
		log.Printf("Warning: Failed to initialize scorer: %v", err)
	}

	// Initialize Debate Manager
	debateManager := NewDebateManager(db, agents, apiKey)

	router := gin.Default()

	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		c.Writer.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Range")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, HEAD")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// inputHandler := player.NewInputHandler(log.Default()) // Removed unused variable

	// Get the first two agents for the conversation (This logic moves to DebateManager.CreateNewDebate)
	// var agent1, agent2 *agent.Agent
	// for _, a := range agents { ... }

	server := &Server{
		router:        router,
		agents:        agents, // Keep agents map for reference if needed by manager/server
		audioCache:    make(map[string]audioCache),
		useHTTPS:      useHTTPS,
		config:        config,
		scorer:        scorer, // Scorer might be passed to sessions later
		db:            db,
		debateManager: debateManager, // Assign the manager
		// Removed initialization of conversation-specific fields
	}

	// --- Update Routes ---
	// router.GET("/ws/conversation", server.handleConversationWebSocket) // Old route
	router.GET("/ws/debate/:debateID", server.handleDebateWebSocket) // New route
	router.GET("/api/audio/:id", server.handleAudioStream)           // Remains mostly the same
	// router.POST("/api/conversation/start", server.startConversation) // To be replaced or modified
	router.POST("/api/debates", server.createDebateHandler) // New endpoint to create debates
	router.POST("/api/stt", audio.HandleSTT)
	router.GET("/api/agents", server.listAgents)
	router.GET("/api/arguments", server.getArguments)             // May need debateID filter later
	router.GET("/api/arguments/:id", server.getArgument)          // May need debateID context later
	router.GET("/api/debates", server.listDebatesHandler)         // New endpoint to list debates
	router.GET("/api/debates/:debateID", server.getDebateHandler) // New endpoint to get specific debate details
	// router.GET("/api/gameScore", server.getGameScore) // Game score is now per-debate

	// Topic-related endpoints
	router.GET("/api/topics", server.listTopicsHandler)                              // List all available topics
	router.GET("/api/topics/category/:category", server.listTopicsByCategoryHandler) // List topics by category
	router.GET("/api/topics/:id", server.getTopicHandler)                            // Get specific topic details

	// Update static file routes
	router.StaticFile("/", "./static/lobby.html") // Use lobby as main page
	router.StaticFile("/lobby.html", "./static/lobby.html")
	router.StaticFile("/debate.html", "./static/debate.html")
	router.StaticFile("/test.html", "./test.html") // Add test.html for testing
	router.Static("/static", "./static")
	router.Static("/hls", "./static/hls")

	router.Use(func(c *gin.Context) {
		if c.Request.URL.Path[:4] == "/hls" {
			if len(c.Request.URL.Path) > 5 && c.Request.URL.Path[len(c.Request.URL.Path)-5:] == ".aac" {
				c.Header("Content-Type", "audio/aac")
			}
		}
	})

	log.Printf("Server initialized with %d agents", len(agents))
	return server
}

// --- New API Handlers (Stubs) ---

func (s *Server) createDebateHandler(c *gin.Context) {
	// Extract request data
	var req struct {
		Topic     string `json:"topic" binding:"required"`
		Agent1    string `json:"agent1" binding:"required"`
		Agent2    string `json:"agent2" binding:"required"`
		CreatedBy string `json:"created_by"`
		TopicID   int    `json:"topic_id"` // Optional: Use a pre-generated topic
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// If a topic ID is provided, use the pre-generated topic
	if req.TopicID > 0 {
		topic, err := s.db.GetTopic(req.TopicID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Topic with ID %d not found: %v", req.TopicID, err)})
			return
		}

		// Override the request with the pre-generated topic details
		req.Topic = topic.Title
		req.Agent1 = topic.Agent1Name
		req.Agent2 = topic.Agent2Name
	}

	// Validate agents exist
	agent1, exists := s.agents[req.Agent1]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Agent '%s' not found", req.Agent1)})
		return
	}

	agent2, exists := s.agents[req.Agent2]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Agent '%s' not found", req.Agent2)})
		return
	}

	if req.Agent1 == req.Agent2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create debate with the same agent on both sides"})
		return
	}

	// Create debate via manager
	debateID, err := s.debateManager.CreateDebate(req.Topic, agent1, agent2, req.CreatedBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create debate: %v", err)})
		return
	}

	// Return debate info
	debate, err := s.db.GetDebate(debateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Debate created but failed to retrieve details: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Debate created successfully",
		"debate":  debate,
	})
}

func (s *Server) listDebatesHandler(c *gin.Context) {
	// Get query parameters for filtering
	status := c.DefaultQuery("status", "")

	var debates []*database.Debate
	var err error

	if status == "active" || status == "waiting" {
		// Only fetch active debates
		debates, err = s.db.ListActiveDebates()
	} else {
		// TODO: Implement fetching all debates with pagination
		// For now, just fetch active ones as fallback
		debates, err = s.db.ListActiveDebates()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list debates", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"debates": debates,
		"count":   len(debates),
	})
}

func (s *Server) getDebateHandler(c *gin.Context) {
	debateID := c.Param("debateID")
	if debateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Debate ID is required"})
		return
	}

	debate, err := s.db.GetDebate(debateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Debate not found", "details": err.Error()})
		return
	}

	// Get additional real-time information from active session if available
	session, exists := s.debateManager.GetDebate(debateID)

	response := gin.H{"debate": debate}

	if exists {
		// Add real-time information
		gameScore := session.GetGameScore()
		status := session.GetStatus()
		_, clientCount := session.CheckStatusAndClients()

		response["real_time"] = gin.H{
			"game_score": gin.H{
				debate.Agent1Name: gameScore.Agent1Score,
				debate.Agent2Name: gameScore.Agent2Score,
			},
			"status":       status,
			"client_count": clientCount,
		}
	}

	c.JSON(http.StatusOK, response)
}

// --- WebSocket Handler ---

func (s *Server) handleDebateWebSocket(c *gin.Context) {
	debateID := c.Param("debateID")
	log.Printf("WebSocket connection attempt for debate ID: %s", debateID)

	// 1. Get DebateSession from manager
	session, exists := s.debateManager.GetDebate(debateID)
	if !exists {
		log.Printf("Debate session %s not found", debateID)
		// Optionally send an error back before closing if possible,
		// but Upgrade might fail anyway if we write before upgrading.
		// For simplicity, just return. The client will see a failed connection.
		return
	}

	// 2. Upgrade connection
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection for debate %s: %v", debateID, err)
		return
	}
	defer ws.Close()
	log.Printf("WebSocket connection upgraded for debate %s", debateID)

	// Generate a unique Player ID for this connection
	// TODO: Replace with actual user authentication/ID if available
	playerID := fmt.Sprintf("player_%s", uuid.New().String()[:8])

	// 3. Add client to session
	session.AddClient(ws, playerID)

	// 4. If first client for a 'waiting' debate, start the debate loop
	if session.GetStatus() == "waiting" {
		session.UpdateStatus("active")
		// Update DB status as well
		err := s.db.UpdateDebateStatus(debateID, "active")
		if err != nil {
			log.Printf("Error updating debate %s status to active in DB: %v", debateID, err)
			// Handle error - maybe close connection?
		}
		s.debateManager.StartDebateLoop(session)
	}

	// Ensure client is removed on disconnect
	defer func() {
		_, remainingClients := session.RemoveClient(ws)
		// If last client leaves an active debate, consider stopping it
		if remainingClients == 0 && session.GetStatus() == "active" {
			log.Printf("Last client left debate %s. Stopping loop.", debateID)
			// Signal the debate loop to stop (implementation needed in DebateSession/Manager)
			// For now, just update status
			session.UpdateStatus("finished") // Or maybe "waiting"?
			err := s.db.UpdateDebateStatus(debateID, session.GetStatus())
			if err != nil {
				log.Printf("Error updating debate %s status to %s in DB on disconnect: %v", debateID, session.GetStatus(), err)
			}
			// Consider removing from manager's active map if truly finished
			// s.debateManager.RemoveDebate(debateID)
		}
	}()

	// 5. Handle incoming messages for this client/session
	for {
		var msg ConversationMessage // Use the existing message struct for now
		err := ws.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for player %s in debate %s: %v", playerID, debateID, err)
			} else {
				log.Printf("WebSocket closed for player %s in debate %s", playerID, debateID)
			}
			break // Exit loop on error/close
		}

		log.Printf("Received message from player %s in debate %s: %s", playerID, debateID, msg.Message)

		// Process the player message
		if msg.Message == "" {
			continue // Skip empty messages
		}

		// 1. Handle player interruption
		session.HandlePlayerInterruption(playerID, msg.Message)

		// 2. Score the argument
		score, err := s.scorer.ScoreArgument(context.Background(), msg.Message, session.Config.Topic)
		if err != nil {
			log.Printf("Error scoring player argument in debate %s: %v", debateID, err)
			// Create a default score
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

		// 3. Save argument to database with debate ID
		argumentID, err := s.db.SaveArgument(playerID, session.Config.Topic, msg.Message, msg.Side, debateID)
		if err != nil {
			log.Printf("Error saving player argument to database: %v", err)
		} else {
			// Save score to database
			err = s.db.SaveScore(argumentID, debateID, score)
			if err != nil {
				log.Printf("Error saving argument score to database: %v", err)
			}
		}

		// 4. Update game score based on player message
		// Determine which agent the player is supporting/opposing
		var agent1Delta, agent2Delta int
		scoreDelta := int(score.Average)

		// If player explicitly chose a side, use that
		if msg.Side == "agent1" || strings.Contains(strings.ToLower(msg.Message), strings.ToLower(session.Agent1.GetName())) {
			// Player is supporting Agent1
			agent1Delta = scoreDelta
			agent2Delta = -scoreDelta
		} else if msg.Side == "agent2" || strings.Contains(strings.ToLower(msg.Message), strings.ToLower(session.Agent2.GetName())) {
			// Player is supporting Agent2
			agent1Delta = -scoreDelta
			agent2Delta = scoreDelta
		} else {
			// No clear side, use a smaller impact
			agent1Delta = scoreDelta / 2
			agent2Delta = -scoreDelta / 2
		}

		gameScore := session.UpdateGameScore(agent1Delta, agent2Delta)

		// 5. Broadcast the player message with score
		session.Broadcast(gin.H{
			"type":     "message",
			"agent":    playerID,
			"message":  msg.Message,
			"isPlayer": true,
			"scores": gin.H{
				"argument": score,
			},
		})

		// 6. Broadcast updated game score
		session.Broadcast(gin.H{
			"type": "game_score",
			"gameScore": gin.H{
				session.Agent1.GetName(): s.debateManager.NormalizeScore(gameScore.Agent1Score),
				session.Agent2.GetName(): s.debateManager.NormalizeScore(gameScore.Agent2Score),
			},
			"internalScore": gin.H{
				session.Agent1.GetName(): gameScore.Agent1Score,
				session.Agent2.GetName(): gameScore.Agent2Score,
			},
		})

		// 7. Check for game over condition
		if gameScore.Agent1Score <= 0 {
			winner := session.Agent2.GetName()
			handleGameOver(s, session, debateID, winner)
		} else if gameScore.Agent2Score <= 0 {
			winner := session.Agent1.GetName()
			handleGameOver(s, session, debateID, winner)
		}
	}
}

// --- Refactored/Commented/Removed Methods ---

func (s *Server) handleAudioStream(c *gin.Context) {
	// This function remains largely the same, as audio caching might stay global for simplicity
	audioID := c.Param("id")

	s.cacheMutex.RLock()
	cache, exists := s.audioCache[audioID]
	s.cacheMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Audio not found"})
		return
	}

	c.Header("Content-Type", "audio/mpeg")
	c.Header("Content-Length", fmt.Sprintf("%d", len(cache.data)))
	c.Header("Cache-Control", "public, max-age=31536000")

	c.Data(http.StatusOK, "audio/mpeg", cache.data)

	go s.cleanupCache()
}

func (s *Server) cleanupCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	threshold := time.Now().Add(-1 * time.Hour)
	for id, cache := range s.audioCache {
		if cache.timestamp.Before(threshold) {
			delete(s.audioCache, id)
		}
	}
}

// handleConversationWebSocket is removed - replaced by handleDebateWebSocket
/*
func (s *Server) handleConversationWebSocket(c *gin.Context) {
	// ... (old logic relying on global state) ...
}
*/

// broadcastMessage is removed - replaced by DebateSession.Broadcast
/*
func (s *Server) broadcastMessage(message interface{}) {
	// ... (old logic relying on global s.clients) ...
}
*/

// getNextAgent is removed - replaced by DebateSession.GetNextAgent
/*
func (s *Server) getNextAgent() *agent.Agent {
	s.agentMutex.RLock() // This mutex might be removed if lastSpeakingAgent moves entirely
	lastSpeaker := s.lastSpeakingAgent
	s.agentMutex.RUnlock()

	// Choose the agent that hasn't spoken last
	for name, agent := range s.agents {
		if name != lastSpeaker {
			s.agentMutex.Lock()
			s.lastSpeakingAgent = name
			s.agentMutex.Unlock()
			return agent
		}
	}

	// If no last speaker (first message), pick any agent
	for name, agent := range s.agents {
		s.agentMutex.Lock()
		s.lastSpeakingAgent = name
		s.agentMutex.Unlock()
		return agent
	}

}
*/

// handlePlayerMessage logic needs to move to DebateManager or DebateSession
// It will need access to the specific session, scorer, db, etc.
/*
func (s *Server) handlePlayerMessage(ws *websocket.Conn, msg ConversationMessage) {
	// ... (old logic relying on global state and broadcasting) ...
}
*/

// getPrompt might be moved to conversation/DebateSession or kept as a helper if needed globally
func getPrompt(conversationContext string, playerMessage string, agentName string, agentRole string, topic string) string {
	switch playerMessage {
	case "":
		return fmt.Sprintf(`Current conversation context: %s

You are %s, with the role of %s.
Topic: %s

CRITICAL INSTRUCTIONS
1. You MUST respond with EXACTLY 1-2 SHORT sentences
2. DIRECTLY ADDRESS the previous speaker's point before making your counter-argument
3. Adapt your arguments to maintain natural conversation flow
4. Never use emojis or smileys
5. Never repeat arguments that have already been used in the conversation. This is a hard requirement.

RESPONSE PRIORITY ORDER:
1. Briefly acknowledge or counter the previous point
2. Then deliver your argument in a way that supports your position
3. Keep it engaging and confrontational, but natural

Generate a response that:
1. Focuses on one specific argument about the topic
2. Directly addresses previous points when relevant
3. Keeps responses concise (1-2 sentences maximum)
4. Do not use emojis or smileys!

DEBATE GUIDELINES:
1. Make it engaging and fun
2. Use terminology appropriate to the topic
3. Stay in character based on your role

CRITICAL ROLE ENFORCEMENT:
- You are %s with the role of %s
- Always argue from your assigned position
- Never switch sides or contradict your assigned position
- Never repeat an argument you've already used in the conversation

Keep responses focused on the core debate about the topic.`, conversationContext, agentName, agentRole, topic, agentName, agentRole)
		// This is the prompt when there's a player message
	default:
		return fmt.Sprintf(`Current conversation context: %s

You are %s, with the role of %s.
Topic: %s

CRITICAL INSTRUCTIONS
1. You MUST respond with EXACTLY 1-2 SHORT sentences
2. DIRECTLY ADDRESS the player's message: "%s"
3. Adapt your arguments to maintain natural conversation flow
4. Never use emojis or smileys
5. Never repeat arguments that have already been used in the conversation. This is a hard requirement.

RESPONSE PRIORITY ORDER:
1. Briefly acknowledge or counter the player's point
2. Then deliver your argument in a way that supports your position
3. Keep it engaging and confrontational, but natural

Generate a response that:
1. Focuses on one specific argument about the topic
2. Directly addresses the player's message
3. Keeps responses concise (1-2 sentences maximum)
4. Do not use emojis or smileys!

DEBATE GUIDELINES:
1. Make it engaging and fun
2. Use terminology appropriate to the topic
3. Stay in character based on your role

CRITICAL ROLE ENFORCEMENT:
- You are %s with the role of %s
- Always argue from your assigned position
- Never switch sides or contradict your assigned position
- Never repeat an argument you've already used in the conversation

Keep responses focused on the core debate about the topic.`, conversationContext, agentName, agentRole, topic, playerMessage, agentName, agentRole)
	}
}

// continueAgentDiscussion logic needs to move to DebateManager.StartDebateLoop
/*
func (s *Server) continueAgentDiscussion(ws *websocket.Conn, conversationID int) {
	// ... (old logic relying on global state, broadcasting, etc.) ...
}
*/

// startConversation is removed - replaced by createDebateHandler
/*
func (s *Server) startConversation(c *gin.Context) {
	// ... (old logic) ...
}
*/

func (s *Server) listAgents(c *gin.Context) {
	// This can likely remain as it lists globally available agents
	agents := make([]map[string]any, 0)
	for _, a := range s.agents {
		agents = append(agents, map[string]any{
			"name": a.GetName(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
	})
}

func (s *Server) getArguments(c *gin.Context) {
	arguments, err := s.db.GetAllArguments()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"arguments": arguments})
}

func (s *Server) getArgument(c *gin.Context) {
	// This might need modification later if arguments are strictly tied to debates
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid argument ID"})
		return
	}

	argument, err := s.db.GetArgumentWithScore(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, argument)
}

// getGameScore is removed - score is now per-debate, fetched via getDebateHandler or WebSocket
/*
func (s *Server) getGameScore(c *gin.Context) {
	// ... (old logic using global scores) ...
}
*/

func (s *Server) Run(addr string) error {
	if s.useHTTPS {
		return s.runHTTPS(addr)
	}
	return s.runHTTP(addr)
}

func (s *Server) runHTTP(addr string) error {
	log.Printf("Starting HTTP server on %s...", addr)
	return s.router.Run(addr)
}

func (s *Server) runHTTPS(addr string) error {
	log.Printf("Starting HTTPS server with HTTP/3 support on %s...", addr)
	srv := &http.Server{
		Addr:    addr,
		Handler: s.router,
		TLSConfig: &tls.Config{
			NextProtos: []string{"h3", "http/1.1"},
		},
	}

	// Create an HTTP/3 server
	http3Srv := &http3.Server{
		Addr:      addr,
		Handler:   s.router,
		TLSConfig: srv.TLSConfig,
	}

	// Start the HTTP/3 server
	go func() {
		if err := http3Srv.ListenAndServeTLS("cert.pem", "key.pem"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP/3 server failed: %v", err)
		}
	}()

	// Start the HTTP/1.1 and HTTP/2 server
	return srv.ListenAndServeTLS("cert.pem", "key.pem")
}

// GetOrderedAgentNames returns agent names in a consistent order
func (s *Server) GetOrderedAgentNames() (agent1Name, agent2Name string) {
	// First look for Tiger Agent, then Bear Agent
	for name, agent := range s.agents {
		switch agent.GetName() {
		case TIGER_AGENT:
			agent1Name = name
		case BEAR_AGENT:
			agent2Name = name
		}
	}

	// Optional validation
	if agent1Name == "" || agent2Name == "" {
		log.Printf("Warning: Could not find both agents. Tiger: %s, Bear: %s", agent1Name, agent2Name)
	}

	return agent1Name, agent2Name
}

// handleGameOver handles the game over condition for a debate
func handleGameOver(s *Server, session *conversation.DebateSession, debateID string, winner string) {
	log.Printf("Game over in debate %s. Winner: %s", debateID, winner)

	// Update status in memory
	session.UpdateStatus("finished")

	// Update database
	err := s.db.UpdateDebateEnd(debateID, "finished", winner)
	if err != nil {
		log.Printf("Error updating debate end in database: %v", err)
	}

	// Broadcast game over message
	session.Broadcast(gin.H{
		"type":    "game_over",
		"winner":  winner,
		"message": fmt.Sprintf("Game over! %s has won the debate!", winner),
	})
}

// listTopicsHandler returns a list of all available pre-generated topics
func (s *Server) listTopicsHandler(c *gin.Context) {
	topics, err := s.db.GetTopics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get topics: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"topics": topics})
}

// listTopicsByCategoryHandler returns topics filtered by category
func (s *Server) listTopicsByCategoryHandler(c *gin.Context) {
	category := c.Param("category")
	if category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category parameter is required"})
		return
	}

	topics, err := s.db.GetTopicsByCategory(category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get topics: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"topics": topics, "category": category})
}

// getTopicHandler returns details for a specific topic
func (s *Server) getTopicHandler(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid topic ID"})
		return
	}

	topic, err := s.db.GetTopic(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Topic with ID %d not found", id)})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get topic: %v", err)})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"topic": topic})
}
