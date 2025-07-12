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
	"github.com/neo/convinceme_backend/internal/auth"
	"github.com/neo/convinceme_backend/internal/conversation"
	"github.com/neo/convinceme_backend/internal/logging"
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
	db            database.DatabaseInterface
	debateManager *DebateManager      // Manages all debate sessions
	auth          *auth.Auth          // Authentication handler
	featureFlags  *FeatureFlagManager // Feature flag manager
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
	TIGER_AGENT = "'La Pulga Protector' Pepito"
	BEAR_AGENT  = "'Siuuuu Sensei' Sergio"
	MAX_SCORE   = 10
)

func NewServer(agents map[string]*agent.Agent, db *database.Database, apiKey string, useHTTPS bool, config *Config) *Server {
	// Initialize player queue tracking (Scorer remains part of Server for now)
	scorer, err := scoring.NewScorer(apiKey)
	if err != nil {
		log.Printf("Warning: Failed to initialize scorer: %v", err)
	}

	// Initialize Auth handler
	authHandler := auth.New(auth.Config{
		JWTSecret:                config.JWTSecret,
		TokenDuration:            24 * time.Hour,     // Access tokens valid for 24 hours
		RefreshTokenDuration:     7 * 24 * time.Hour, // Refresh tokens valid for 7 days
		RequireEmailVerification: config.RequireEmailVerification,
		RequireInvitation:        config.RequireInvitation,
	})

	// Create a new router without default middleware
	router := gin.New()

	// Add custom middleware
	router.Use(RequestIDMiddleware())
	router.Use(LoggingMiddleware())
	router.Use(RecoveryMiddleware())
	router.Use(ErrorHandler())

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

	// Initialize feature flags
	featureFlags, err := NewFeatureFlagManager("config/feature_flags.json")
	if err != nil {
		// Log the error but continue with default flags
		log.Printf("Failed to initialize feature flags: %v\n", err)
		// Create a default feature flag manager
		featureFlags = &FeatureFlagManager{
			configPath: "config/feature_flags.json",
			flags: FeatureFlags{
				RequireEmailVerification: config.RequireEmailVerification,
				RequireInvitation:        config.RequireInvitation,
				AllowPasswordReset:       true,
				AllowSocialLogin:         false,
				EnableRateLimiting:       true,
				EnableCSRFProtection:     false,
				EnableFeedbackCollection: true,
				EnableAnalytics:          true,
				EnableAdminDashboard:     true,
			},
			mu: sync.RWMutex{},
		}
	}

	server := &Server{
		router:       router,
		agents:       agents, // Keep agents map for reference if needed by manager/server
		audioCache:   make(map[string]audioCache),
		useHTTPS:     useHTTPS,
		config:       config,
		scorer:       scorer, // Scorer might be passed to sessions later
		db:           db,
		auth:         authHandler,  // Authentication handler
		featureFlags: featureFlags, // Feature flag manager
		// Removed initialization of conversation-specific fields
	}

	// Initialize Debate Manager with server reference
	debateManager := NewDebateManager(db, agents, apiKey, server)
	server.debateManager = debateManager

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

	// Setup authentication routes
	server.setupAuthRoutes()

	// Setup invitation routes
	server.setupInvitationRoutes()

	// Setup feature flag routes
	server.setupFeatureFlagRoutes()

	// Setup feedback routes
	server.setupFeedbackRoutes()

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
		Topic     string `json:"topic"`
		Agent1    string `json:"agent1"`
		Agent2    string `json:"agent2"`
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
	// Get pagination parameters
	paginationParams := GetPaginationParams(c)

	// Get filter parameters
	filterParams := GetFilterParams(c)

	// Special case for 'active' status to include 'waiting' debates
	if filterParams.Status == "active" || filterParams.Status == "waiting" {
		// Use the existing method that handles both 'active' and 'waiting' statuses
		debates, err := s.db.ListActiveDebates()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list debates: %v", err)})
			return
		}

		// Since we're not using pagination here, just return all debates
		c.JSON(http.StatusOK, gin.H{
			"debates": debates,
			"count":   len(debates),
		})
		return
	}

	// Convert to database filter
	filter := database.DebateFilter{
		Status:  filterParams.Status,
		Search:  filterParams.Search,
		SortBy:  filterParams.SortBy,
		SortDir: filterParams.SortDir,
		Offset:  paginationParams.CalculateOffset(),
		Limit:   paginationParams.PageSize,
	}

	// Get debates with pagination and filtering
	debates, total, err := s.db.ListDebates(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list debates: %v", err)})
		return
	}

	// Update pagination with total count
	paginationParams.Total = total

	// Send paginated response
	SendPaginatedResponse(c, paginationParams, debates)
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
	clientIP := c.ClientIP()

	logging.LogWebSocketEvent("connection_attempt", debateID, "", map[string]interface{}{
		"client_ip":  clientIP,
		"user_agent": c.GetHeader("User-Agent"),
	})

	// 1. Get DebateSession from manager
	session, exists := s.debateManager.GetDebate(debateID)
	if !exists {
		logging.LogWebSocketEvent("debate_not_found", debateID, "", map[string]interface{}{
			"client_ip": clientIP,
		})
		// Optionally send an error back before closing if possible,
		// but Upgrade might fail anyway if we write before upgrading.
		// For simplicity, just return. The client will see a failed connection.
		return
	}

	// 2. Upgrade connection
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logging.LogWebSocketEvent("upgrade_failed", debateID, "", map[string]interface{}{
			"error":     err,
			"client_ip": clientIP,
		})
		return
	}
	defer ws.Close()

	// Generate a unique Player ID for this connection
	// TODO: Replace with actual user authentication/ID if available
	playerID := fmt.Sprintf("player_%s", uuid.New().String()[:8])

	logging.LogWebSocketEvent("connection_established", debateID, playerID, map[string]interface{}{
		"client_ip": clientIP,
	})

	// 3. Add client to session
	session.AddClient(ws, playerID)

	// 4. Send current debate state to new client (for reconnections)
	status := session.GetStatus()
	gameScore := session.GetGameScore()
	recentHistory := session.GetRecentHistory(10) // Send last 10 messages for context

	// Send welcome message with current state
	welcomeMsg := gin.H{
		"type":   "welcome",
		"status": status,
		"gameScore": gin.H{
			session.Agent1.GetName(): float64(gameScore.Agent1Score),
			session.Agent2.GetName(): float64(gameScore.Agent2Score),
		},
		"debate_id": debateID,
		"player_id": playerID,
	}

	if err := ws.WriteJSON(welcomeMsg); err != nil {
		logging.Error("Failed to send welcome message", map[string]interface{}{
			"error":     err,
			"debate_id": debateID,
			"player_id": playerID,
		})
	}

	// Send recent history to help client catch up
	for _, entry := range recentHistory {
		historyMsg := gin.H{
			"type":      "message",
			"agent":     entry.Speaker,
			"content":   entry.Message,
			"timestamp": entry.Time,
			"isPlayer":  entry.IsPlayer,
			"isHistory": true, // Mark as historical message
		}
		if err := ws.WriteJSON(historyMsg); err != nil {
			logging.Error("Failed to send history message", map[string]interface{}{
				"error":     err,
				"debate_id": debateID,
				"player_id": playerID,
			})
			break // Stop sending history if there's an error
		}
	}

	// 5. If first client for a 'waiting' debate, start the debate loop
	if session.GetStatus() == "waiting" {
		logging.LogDebateEvent("status_change", debateID, map[string]interface{}{
			"from_status":  "waiting",
			"to_status":    "active",
			"triggered_by": playerID,
		})

		session.UpdateStatus("active")
		// Update DB status as well
		err := s.db.UpdateDebateStatus(debateID, "active")
		if err != nil {
			logging.Error("Failed to update debate status in database", map[string]interface{}{
				"error":     err,
				"debate_id": debateID,
				"status":    "active",
			})
			// Handle error - maybe close connection?
		}
		s.debateManager.StartDebateLoop(session)
	}

	// Ensure client is removed on disconnect
	defer func() {
		_, remainingClients := session.RemoveClient(ws)
		logging.LogWebSocketEvent("client_disconnected", debateID, playerID, map[string]interface{}{
			"remaining_clients": remainingClients,
		})

		// IMPORTANT: Never stop debates due to client disconnections!
		// Debates should continue running even without observers to allow:
		// 1. Clients to reconnect and catch up on the debate
		// 2. New clients to join ongoing debates
		// 3. Robust handling of network issues

		if remainingClients == 0 && session.GetStatus() == "active" {
			logging.Info("Debate continues without observers", map[string]interface{}{
				"debate_id": debateID,
				"status":    "active_unobserved",
			})
			// Debates only stop when:
			// 1. A winner is determined (HP reaches 0)
			// 2. Timeout occurs (15 minutes)
			// 3. Manual intervention
			// 4. Critical errors in the debate loop
		}
	}()

	// 6. Handle incoming messages for this client/session with better error recovery
	for {
		var msg ConversationMessage

		// Set read deadline to detect connection issues
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))

		err := ws.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logging.Error("Unexpected WebSocket error", map[string]interface{}{
					"error":     err,
					"player_id": playerID,
					"debate_id": debateID,
				})
			} else {
				logging.Info("WebSocket connection closed", map[string]interface{}{
					"player_id": playerID,
					"debate_id": debateID,
					"reason":    "normal_close",
				})
			}
			break // Exit loop on error/close
		}

		// Reset read deadline after successful read
		ws.SetReadDeadline(time.Time{})

		logging.Info("Received player message", map[string]interface{}{
			"player_id": playerID,
			"debate_id": debateID,
			"message":   msg.Message,
			"type":      msg.Type,
		})

		// Handle special message types for state synchronization
		if msg.Type == "get_state" {
			debateInfo, err := s.debateManager.GetDebateInfo(debateID)
			if err != nil {
				logging.Error("Failed to get debate info", map[string]interface{}{
					"error":     err,
					"debate_id": debateID,
					"player_id": playerID,
				})
				continue
			}

			stateMsg := gin.H{
				"type":        "state_update",
				"debate_info": debateInfo,
			}

			if err := ws.WriteJSON(stateMsg); err != nil {
				logging.Error("Failed to send state update", map[string]interface{}{
					"error":     err,
					"debate_id": debateID,
					"player_id": playerID,
				})
			}
			continue // Don't process as regular message
		}

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

		// 4. Update game score based on player message using comparative performance
		// Calculate player's average score (same scale as agents: 1-10)
		playerAverageScore := float64(score.Strength+score.Relevance+score.Logic+score.Truth+score.Humor) / 5.0

		// Determine which side the player is supporting first
		var supportedAgent, opposedAgent string

		if msg.Side == "agent1" || strings.Contains(strings.ToLower(msg.Message), strings.ToLower(session.Agent1.GetName())) {
			supportedAgent = session.Agent1.GetName()
			opposedAgent = session.Agent2.GetName()
		} else if msg.Side == "agent2" || strings.Contains(strings.ToLower(msg.Message), strings.ToLower(session.Agent2.GetName())) {
			supportedAgent = session.Agent2.GetName()
			opposedAgent = session.Agent1.GetName()
		} else {
			// No clear side - no HP changes for neutral comments
			supportedAgent = ""
			opposedAgent = ""
		}

		var agent1Delta, agent2Delta int

		if supportedAgent != "" && opposedAgent != "" {
			// Get recent scores for both agents to see which is performing worse
			supportedAgentScore := session.GetRecentAgentScore(supportedAgent, 3)
			opposedAgentScore := session.GetRecentAgentScore(opposedAgent, 3)

			// Compare player's argument quality with agent performance
			// If player's argument is much stronger than the opposed agent's recent performance,
			// the opposed agent loses HP
			scoreDifferenceVsOpposed := playerAverageScore - opposedAgentScore

			var hpLoss int
			if scoreDifferenceVsOpposed > 1.0 {
				// Player's argument is significantly better than opposed agent - opposed agent loses HP
				hpLoss = int(scoreDifferenceVsOpposed * 3) // 3x multiplier for good player intervention
				if hpLoss > 15 {
					hpLoss = 15 // Cap at 15 HP loss
				}
				if hpLoss < 3 {
					hpLoss = 3 // Minimum meaningful loss
				}

				// Apply HP loss to the opposed agent
				if opposedAgent == session.Agent1.GetName() {
					agent1Delta = -hpLoss
					agent2Delta = 0
				} else {
					agent1Delta = 0
					agent2Delta = -hpLoss
				}
			} else {
				// Player's argument is not significantly better - no HP changes
				agent1Delta = 0
				agent2Delta = 0
			}

			log.Printf("Player comparative scoring - Player: %.2f, Supported agent: %.2f, Opposed agent: %.2f, Score diff vs opposed: %.2f, HP loss: %d",
				playerAverageScore, supportedAgentScore, opposedAgentScore, scoreDifferenceVsOpposed, hpLoss)
		} else {
			// Neutral comment - no HP changes
			agent1Delta = 0
			agent2Delta = 0
		}

		gameScore := session.UpdateGameScore(agent1Delta, agent2Delta)

		// 5. Broadcast the player message with score
		session.Broadcast(gin.H{
			"type":     "message",
			"agent":    playerID, // Show full player ID
			"content":  msg.Message,
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

// CacheAudio stores audio data in the cache and returns the URL to access it
func (s *Server) CacheAudio(audioData []byte) string {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// Generate unique ID for the audio
	audioID := uuid.New().String()

	// Store in cache
	s.audioCache[audioID] = audioCache{
		data:      audioData,
		timestamp: time.Now(),
	}

	// Return the URL path
	return fmt.Sprintf("/api/audio/%s", audioID)
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

// listTopicsHandler returns a list of all available pre-generated topics with pagination and filtering
func (s *Server) listTopicsHandler(c *gin.Context) {
	// Get pagination parameters
	paginationParams := GetPaginationParams(c)

	// Get filter parameters
	filterParams := GetFilterParams(c)

	// Convert to database filter
	filter := database.TopicFilter{
		Category: filterParams.Category,
		Search:   filterParams.Search,
		SortBy:   filterParams.SortBy,
		SortDir:  filterParams.SortDir,
		Offset:   paginationParams.CalculateOffset(),
		Limit:    paginationParams.PageSize,
	}

	// Get topics with pagination and filtering
	topics, total, err := s.db.GetTopics(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get topics: %v", err)})
		return
	}

	// Update pagination with total count
	paginationParams.Total = total

	// Send paginated response
	SendPaginatedResponse(c, paginationParams, topics)
}

// listTopicsByCategoryHandler returns topics filtered by category with pagination
func (s *Server) listTopicsByCategoryHandler(c *gin.Context) {
	category := c.Param("category")
	if category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category parameter is required"})
		return
	}

	// Get pagination parameters
	paginationParams := GetPaginationParams(c)

	// Get filter parameters
	filterParams := GetFilterParams(c)

	// Override category with the path parameter
	filterParams.Category = category

	// Convert to database filter
	filter := database.TopicFilter{
		Category: category,
		Search:   filterParams.Search,
		SortBy:   filterParams.SortBy,
		SortDir:  filterParams.SortDir,
		Offset:   paginationParams.CalculateOffset(),
		Limit:    paginationParams.PageSize,
	}

	// Get topics with pagination and filtering
	topics, total, err := s.db.GetTopics(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get topics: %v", err)})
		return
	}

	// Update pagination with total count
	paginationParams.Total = total

	// Send paginated response with category info
	response := BuildPaginationResponse(c, paginationParams, topics)
	response["category"] = category
	c.JSON(http.StatusOK, response)
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
