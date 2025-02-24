package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/neo/convinceme_backend/internal/audio"
	"github.com/quic-go/quic-go/http3"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/neo/convinceme_backend/internal/scoring"
	"github.com/neo/convinceme_backend/internal/tools"
)

type Server struct {
	router             *gin.Engine
	agents             map[string]*agent.Agent
	wsWriteMutex       sync.Mutex
	audioCache         map[string]audioCache
	cacheMutex         sync.RWMutex
	lastPlayerMessage  time.Time
	playerMessageMutex sync.Mutex
	lastSpeakingAgent  string
	agentMutex         sync.RWMutex
	conversationLog    []ConversationEntry
	conversationMutex  sync.RWMutex
	judge              *tools.ConvictionJudge
	useHTTPS           bool
	config             *Config
	scorer             *scoring.Scorer
	db                 *database.Database
}

type ConversationEntry struct {
	Speaker  string    `json:"speaker"`
	Message  string    `json:"message"`
	Time     time.Time `json:"time"`
	IsPlayer bool      `json:"is_player"`
	Topic    string    `json:"topic"`
}

type ConversationMessage struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
	Type    string `json:"type"`
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

func NewServer(agents map[string]*agent.Agent, db *database.Database, apiKey string, useHTTPS bool, config *Config) *Server {
	judge, err := tools.NewConvictionJudge(apiKey)
	if err != nil {
		log.Printf("Warning: Failed to create conviction judge: %v", err)
	}

	scorer, err := scoring.NewScorer(apiKey)
	if err != nil {
		log.Printf("Warning: Failed to initialize scorer: %v", err)
	}

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

	server := &Server{
		router:            router,
		agents:            agents,
		audioCache:        make(map[string]audioCache),
		lastPlayerMessage: time.Now(),
		conversationLog:   make([]ConversationEntry, 0),
		judge:             judge,
		useHTTPS:          useHTTPS,
		config:            config,
		scorer:            scorer,
		db:                db,
	}

	router.GET("/ws/conversation", server.handleConversationWebSocket)
	router.GET("/api/audio/:id", server.handleAudioStream)
	router.POST("/api/conversation/start", server.startConversation)
	router.POST("/api/stt", audio.HandleSTT)
	router.GET("/api/agents", server.listAgents)
	router.GET("/api/arguments", server.getArguments)
	router.GET("/api/arguments/:id", server.getArgument)

	router.StaticFile("/", "./test.html")
	router.Static("/static", "./static")
	router.Static("/hls", "./static/hls")

	router.Use(func(c *gin.Context) {
		if c.Request.URL.Path[:4] == "/hls" {
			if len(c.Request.URL.Path) > 5 && c.Request.URL.Path[len(c.Request.URL.Path)-5:] == ".aac" {
				c.Header("Content-Type", "audio/aac")
			}
		}
	})

	return server
}

func (s *Server) addToConversationLog(speaker string, message string, isPlayer bool, topic string) {
	s.conversationMutex.Lock()
	defer s.conversationMutex.Unlock()

	entry := ConversationEntry{
		Speaker:  speaker,
		Message:  message,
		Time:     time.Now(),
		IsPlayer: isPlayer,
		Topic:    topic,
	}
	s.conversationLog = append(s.conversationLog, entry)
}

func (s *Server) getConversationContext() string {
	s.conversationMutex.RLock()
	defer s.conversationMutex.RUnlock()

	var context string
	// Get last 5 entries or all if less than 5
	startIdx := len(s.conversationLog) - 5
	if startIdx < 0 {
		startIdx = 0
	}

	for _, entry := range s.conversationLog[startIdx:] {
		speakerType := "Agent"
		if entry.IsPlayer {
			speakerType = "Player"
		}
		context += fmt.Sprintf("%s (%s): %s\n", entry.Speaker, speakerType, entry.Message)
	}

	return context
}

func (s *Server) handleAudioStream(c *gin.Context) {
	audioID := c.Param("id")

	s.cacheMutex.RLock()
	cache, exists := s.audioCache[audioID]
	s.cacheMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Audio not found"})
		return
	}

	c.Header("Content-Type", "audio/aac")
	c.Header("Content-Length", fmt.Sprintf("%d", len(cache.data)))
	c.Header("Cache-Control", "public, max-age=31536000")

	c.Data(http.StatusOK, "audio/aac", cache.data)

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

func (s *Server) handleConversationWebSocket(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer ws.Close()

	// Set read deadline for initial connection
	ws.SetReadDeadline(time.Now().Add(time.Second * 60))

	// Create a done channel to signal goroutine cleanup
	done := make(chan struct{})
	defer close(done)

	// Start ping handler
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := ws.WriteJSON(gin.H{"type": "ping"}); err != nil {
					log.Printf("Failed to send ping: %v", err)
					return
				}
			}
		}
	}()

	// Handle incoming messages
	for {
		var msg ConversationMessage
		err := ws.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			return
		}

		// Reset read deadline after each successful message
		ws.SetReadDeadline(time.Now().Add(time.Second * 60))

		// Handle ping messages
		if msg.Type == "ping" {
			if err := ws.WriteJSON(gin.H{"type": "pong"}); err != nil {
				log.Printf("Failed to send pong: %v", err)
				return
			}
			continue
		}

		s.playerMessageMutex.Lock()
		s.lastPlayerMessage = time.Now()
		s.playerMessageMutex.Unlock()

		if msg.Message == "" {
			s.continueAgentDiscussion(ws)
		} else {
			s.handlePlayerMessage(ws, msg)
		}
	}
}

func (s *Server) getNextAgent() *agent.Agent {
	s.agentMutex.RLock()
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

	return nil
}

func (s *Server) handlePlayerMessage(ws *websocket.Conn, msg ConversationMessage) {
	ctx := context.Background()

	// Score the user's message first
	if s.scorer != nil {
		log.Printf("\n=== Scoring User Message ===\n")
		score, err := s.scorer.ScoreArgument(ctx, msg.Message, msg.Topic)
		if err != nil {
			log.Printf("Failed to score user message: %v", err)
		} else {
			// Save argument and score to database
			argID, err := s.db.SaveArgument("player1", msg.Topic, msg.Message)
			if err != nil {
				log.Printf("Failed to save argument: %v", err)
			} else {
				if err := s.db.SaveScore(argID, score); err != nil {
					log.Printf("Failed to save score: %v", err)
				}
			}

			// Send score back to user
			if err := ws.WriteJSON(gin.H{
				"type": "score",
				"message": fmt.Sprintf("Argument score:\n"+
					"Strength: %d/100\n"+
					"Relevance: %d/100\n"+
					"Logic: %d/100\n"+
					"Truth: %d/100\n"+
					"Humor: %d/100\n"+
					"Average: %.1f/100\n",
					score.Strength,
					score.Relevance,
					score.Logic,
					score.Truth,
					score.Humor,
					score.Average),
			}); err != nil {
				log.Printf("Failed to send score to user: %v", err)
			}
		}
	}

	// Process agent responses
	responses := make(map[string]string)
	// scores := make(map[string]*scoring.ArgumentScore)
	var wg sync.WaitGroup
	var responseMutex sync.Mutex

	// Generate responses from agents
	for name, a := range s.agents {
		wg.Add(1)
		go func(agentName string, agent *agent.Agent) {
			defer wg.Done()

			topic := "General Discussion" // Default topic
			s.conversationMutex.RLock()
			if len(s.conversationLog) > 0 {
				topic = s.conversationLog[0].Topic // Use the topic from conversation
			}
			s.conversationMutex.RUnlock()

			prompt := fmt.Sprintf(`Current debate context:
%s

You are %s, with the role of %s.

Your task is to:
1. Present compelling arguments supporting your species' superiority as a predator
2. Use scientific evidence and documented research
3. Counter opposing arguments with facts and examples
4. Stay focused on the comparison of bears and tigers
5. Be assertive but professional in your responses

Current topic: %s

Generate a response that demonstrates your expertise while directly addressing the points in the debate.`,
				s.getConversationContext(),
				agent.GetName(),
				agent.GetRole(),
				topic)
			response, err := agent.GenerateResponse(ctx, topic, prompt)
			if err != nil {
				log.Printf("Failed to generate response for %s: %v", agentName, err)
				return
			}

			// Score agent's response
			// var argScore *scoring.ArgumentScore
			// if s.scorer != nil {
			// 	if score, err := s.scorer.ScoreArgument(ctx, response, msg.Topic); err == nil {
			// 		argScore = score
			// 	}
			// }

			responseMutex.Lock()
			responses[agentName] = response
			// scores[agentName] = argScore
			responseMutex.Unlock()
		}(name, a)
	}

	wg.Wait()

	// Send responses in sequence
	for name, response := range responses {
		// score := scores[name]
		message := gin.H{
			"type":    "text",
			"message": response,
			"agent":   name,
		}

		// if score != nil {
		// 	message["scores"] = gin.H{
		// 		"argument": score,
		// 	}
		// }

		if err := ws.WriteJSON(message); err != nil {
			log.Printf("Failed to send response for %s: %v", name, err)
			continue
		}

		// Generate and send audio for the response
		agent := s.agents[name]
		audioData, err := agent.GenerateAndStreamAudio(ctx, response)
		if err != nil {
			log.Printf("Failed to generate audio for %s: %v", name, err)
			continue
		}

		audioID := fmt.Sprintf("%s_%d", name, time.Now().UnixNano())
		s.cacheMutex.Lock()
		s.audioCache[audioID] = audioCache{
			data:      audioData,
			timestamp: time.Now(),
		}
		s.cacheMutex.Unlock()

		if err := ws.WriteJSON(gin.H{
			"type":     "audio",
			"audioUrl": fmt.Sprintf("/api/audio/%s", audioID),
			"agent":    name,
		}); err != nil {
			log.Printf("Failed to send audio URL for %s: %v", name, err)
		}

		time.Sleep(time.Duration(s.config.ResponseDelay) * time.Millisecond)
	}

	// After processing the message and getting agent response
	s.analyzeConviction(context.Background(), ws)
}

func (s *Server) continueAgentDiscussion(ws *websocket.Conn) {
	// Get conversation context
	conversationContext := s.getConversationContext()

	// Get the next agent to speak
	agent := s.getNextAgent()
	if agent == nil {
		log.Printf("No agents available")
		return
	}

	ctx := context.Background()
	topic := "General Discussion" // Default topic
	s.conversationMutex.RLock()
	if len(s.conversationLog) > 0 {
		topic = s.conversationLog[0].Topic // Use the topic from conversation
	}
	s.conversationMutex.RUnlock()

	prompt := fmt.Sprintf(`Current debate context:
%s

You are %s, with the role of %s.

Your task is to:
1. Present compelling arguments supporting your species' superiority as a predator
2. Use scientific evidence and documented research
3. Counter opposing arguments with facts and examples
4. Stay focused on the comparison of bears and tigers
5. Be assertive but professional in your responses

Current topic: %s

Generate a response that demonstrates your expertise while directly addressing the points in the debate.`,
		conversationContext,
		agent.GetName(),
		agent.GetRole(),
		topic)

	response, err := agent.GenerateResponse(ctx, topic, prompt)
	if err != nil {
		log.Printf("Failed to generate response: %v", err)
		return
	}

	// Add response to conversation log
	s.addToConversationLog(agent.GetName(), response, false, topic)

	// Send text response
	if err := ws.WriteJSON(gin.H{
		"type":    "text",
		"message": response,
		"agent":   agent.GetName(),
	}); err != nil {
		log.Printf("Failed to send text response: %v", err)
		return
	}

	// Generate and send audio
	audioData, err := agent.GenerateAndStreamAudio(ctx, response)
	if err != nil {
		log.Printf("Failed to generate audio: %v", err)
		return
	}

	audioID := fmt.Sprintf("%s_%d", agent.GetName(), time.Now().UnixNano())
	s.cacheMutex.Lock()
	s.audioCache[audioID] = audioCache{
		data:      audioData,
		timestamp: time.Now(),
	}
	s.cacheMutex.Unlock()

	if err := ws.WriteJSON(gin.H{
		"type":     "audio",
		"audioUrl": fmt.Sprintf("/api/audio/%s", audioID),
		"agent":    agent.GetName(),
	}); err != nil {
		log.Printf("Failed to send audio URL: %v", err)
	}

	// After each agent response
	s.analyzeConviction(context.Background(), ws)
}

func (s *Server) startConversation(c *gin.Context) {
	var msg ConversationMessage
	if err := c.BindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Conversation started",
		"topic":   msg.Topic,
	})
}

func (s *Server) listAgents(c *gin.Context) {
	agents := make([]map[string]interface{}, 0)
	for _, a := range s.agents {
		agents = append(agents, map[string]interface{}{
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

func (s *Server) analyzeConviction(ctx context.Context, ws *websocket.Conn) {
	if s.judge == nil {
		return
	}

	s.conversationMutex.RLock()
	if len(s.conversationLog) < 2 {
		s.conversationMutex.RUnlock()
		return
	}

	// Convert server conversation log to judge's format
	judgeConversation := make([]tools.ConversationEntry, 0, len(s.conversationLog))
	for _, entry := range s.conversationLog {
		if !entry.IsPlayer { // Only include agent messages
			judgeConversation = append(judgeConversation, tools.ConversationEntry{
				Speaker: entry.Speaker,
				Message: entry.Message,
			})
		}
	}

	// Get agent names
	var agent1Name, agent2Name string
	for name := range s.agents {
		if agent1Name == "" {
			agent1Name = name
		} else {
			agent2Name = name
			break
		}
	}
	s.conversationMutex.RUnlock()

	conversationData := map[string]interface{}{
		"agent1_name":  agent1Name,
		"agent2_name":  agent2Name,
		"conversation": judgeConversation,
	}

	conversationJSON, err := json.Marshal(conversationData)
	if err != nil {
		log.Printf("Warning: Failed to marshal conversation data: %v", err)
		return
	}

	metricsJSON, err := s.judge.Call(ctx, string(conversationJSON))
	if err != nil {
		log.Printf("Warning: Failed to analyze conviction: %v", err)
		return
	}

	var metrics tools.ConvictionMetrics
	if err := json.Unmarshal([]byte(metricsJSON), &metrics); err != nil {
		log.Printf("Warning: Failed to parse conviction metrics: %v", err)
		return
	}

	analysisText := fmt.Sprintf("\n=== Conviction Analysis ===\n"+
		"%s Conviction: %.2f\n"+
		"%s Conviction: %.2f\n"+
		"Overall Tension: %.2f\n"+
		"Dominant Speaker: %s\n"+
		"Analysis: %s\n"+
		"========================\n",
		agent1Name, metrics.Agent1Score,
		agent2Name, metrics.Agent2Score,
		metrics.OverallTension,
		metrics.DominantAgent,
		metrics.AnalysisSummary)

	// Log to server console
	log.Print(analysisText)

	// Send to frontend through WebSocket
	if ws != nil {
		if err := ws.WriteJSON(gin.H{
			"type":    "conviction",
			"message": analysisText,
		}); err != nil {
			log.Printf("Failed to send conviction analysis: %v", err)
		}
	}
}
