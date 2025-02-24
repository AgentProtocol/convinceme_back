package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/neo/convinceme_backend/internal/audio"
	"github.com/quic-go/quic-go/http3"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/neo/convinceme_backend/internal/agent"
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
	// New fields for continuous discussion
	isUserConnected bool
	connectedMutex  sync.RWMutex
	userMessages    chan string
	stopDiscussion  chan struct{}
	// Field to track pending user message
	pendingUserMessage     string
	pendingUserMessageLock sync.RWMutex
}

type ConversationEntry struct {
	Speaker  string    `json:"speaker"`
	Message  string    `json:"message"`
	Time     time.Time `json:"time"`
	IsPlayer bool      `json:"is_player"`
}

type ConversationMessage struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

type audioCache struct {
	data      []byte
	timestamp time.Time
	duration  time.Duration
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	EnableCompression: true,
}

func NewServer(agents map[string]*agent.Agent, apiKey string, useHTTPS bool) *Server {
	judge, err := tools.NewConvictionJudge(apiKey)
	if err != nil {
		log.Printf("Warning: Failed to create conviction judge: %v", err)
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
		// Initialize new fields
		userMessages:   make(chan string, 10),
		stopDiscussion: make(chan struct{}),
	}

	router.GET("/ws/conversation", server.handleConversationWebSocket)
	router.GET("/api/audio/:id", server.handleAudioStream)
	router.POST("/api/conversation/start", server.startConversation)
	router.POST("/api/stt", audio.HandleSTT)

	router.StaticFile("/", "./test.html")
	router.Static("/static", "./static")

	return server
}

func (s *Server) addToConversationLog(speaker string, message string, isPlayer bool) {
	s.conversationMutex.Lock()
	defer s.conversationMutex.Unlock()

	entry := ConversationEntry{
		Speaker:  speaker,
		Message:  message,
		Time:     time.Now(),
		IsPlayer: isPlayer,
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

	// Set user as connected
	s.connectedMutex.Lock()
	s.isUserConnected = true
	s.connectedMutex.Unlock()

	// Create a new stop channel for this connection
	s.stopDiscussion = make(chan struct{})

	// Ensure cleanup on disconnect
	defer func() {
		s.connectedMutex.Lock()
		s.isUserConnected = false
		s.connectedMutex.Unlock()
		close(s.stopDiscussion)
	}()

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

	// Start continuous discussion in a separate goroutine
	go s.continueAgentDiscussion(ws)

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

		// Update pending user message
		s.pendingUserMessageLock.Lock()
		s.pendingUserMessage = msg.Message
		s.pendingUserMessageLock.Unlock()

		// Add to conversation log
		s.addToConversationLog("Player", msg.Message, true)

		log.Printf("User message received: %s", msg.Message)
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

func getPrompt(promptType string) string {
	switch promptType {
	case "player_message":
		return `You are participating in a structured debate about whether bears or tigers are the superior predator.

Current conversation context:
%s

You are %s, with the role of %s.
Remember:
1. Stay strictly focused on the bear vs tiger debate
2. Use your expertise to support your position
3. Reference specific facts and studies about your species
5. Never deviate from the debate topic
6. Be passionate but factual about your position
7. The latest message is from the player
8. You should respond to that Player message by repeating the points made in the message and then addressing them

Generate a response that maintains the debate focus and supports your position.`
	case "continue_debate":
		return `You are participating in a structured debate about whether bears or tigers are the superior predator.

Current conversation context:
%s

You are %s, with the role of %s.

Your task is to:
1. Continue the debate about bears vs tigers as superior predators
2. Use your expertise to present new arguments or expand on previous points
3. Reference specific facts, studies, or observations about your species
4. Challenge or address points made about the opposing predator
5. Stay strictly focused on the debate topic
6. Be passionate but factual in your arguments

Key debate points to consider:
- Physical strength and combat abilities
- Hunting success rates and techniques
- Territorial dominance
- Survival skills and adaptability
- Historical encounters and documented fights
- Biological advantages and disadvantages

Generate a response that advances the debate while maintaining scientific credibility.`
	default:
		return ""
	}
}

func (s *Server) continueAgentDiscussion(ws *websocket.Conn) {
	// Check if user is connected
	s.connectedMutex.RLock()
	isConnected := s.isUserConnected
	s.connectedMutex.RUnlock()

	if !isConnected {
		return
	}

	isFirstMessage := true

	for {
		select {
		case <-s.stopDiscussion:
			return
		default:
			// Start timing the entire generation process
			generationStart := time.Now()

			// Get conversation context
			conversationContext := s.getConversationContext()

			// Get the next agent to speak
			agent := s.getNextAgent()
			if agent == nil {
				log.Printf("No agents available")
				return
			}

			// Check for pending user message
			s.pendingUserMessageLock.RLock()
			pendingMessage := s.pendingUserMessage
			// Clear pending user message
			s.pendingUserMessage = ""
			s.pendingUserMessageLock.RUnlock()

			promptType := "continue_debate"
			if pendingMessage != "" {
				promptType = "player_message"
			}

			ctx := context.Background()
			prompt := fmt.Sprintf(getPrompt(promptType),
				conversationContext,
				agent.GetName(),
				agent.GetRole())

			// Time the response generation
			responseStart := time.Now()
			response, err := agent.GenerateResponse(ctx, "Bear vs Tiger Debate", prompt)
			responseGenerationTime := time.Since(responseStart)
			if err != nil {
				log.Printf("Failed to generate response: %v", err)
				continue
			}

			// Add response to conversation log
			s.addToConversationLog(agent.GetName(), response, false)

			// Send text response
			if err := ws.WriteJSON(gin.H{
				"type":    "text",
				"message": response,
				"agent":   agent.GetName(),
			}); err != nil {
				log.Printf("Failed to send text response: %v", err)
				return
			}

			// After each agent response
			s.analyzeConviction(context.Background(), ws)

			// Time the audio generation
			audioStart := time.Now()
			audioData, err := agent.GenerateAndStreamAudio(ctx, response)
			audioGenerationTime := time.Since(audioStart)
			if err != nil {
				log.Printf("Failed to generate audio: %v", err)
				continue
			}

			audioDuration := getAudioDuration(audioData)
			audioID := fmt.Sprintf("%s_%d", agent.GetName(), time.Now().UnixNano())

			s.cacheMutex.Lock()
			s.audioCache[audioID] = audioCache{
				data:      audioData,
				timestamp: time.Now(),
				duration:  audioDuration,
			}
			s.cacheMutex.Unlock()

			if err := ws.WriteJSON(gin.H{
				"type":     "audio",
				"audioUrl": fmt.Sprintf("/api/audio/%s", audioID),
				"agent":    agent.GetName(),
				"duration": audioDuration.Seconds(),
			}); err != nil {
				log.Printf("Failed to send audio URL: %v", err)
				return
			}

			// Calculate total generation time for this message
			totalGenerationTime := time.Since(generationStart)

			// Log timing information
			log.Printf("Generation timing for %s: Response=%v, Audio=%v, Total=%v",
				agent.GetName(),
				responseGenerationTime,
				audioGenerationTime,
				totalGenerationTime)

			// Calculate delay
			buffer := time.Duration(math.Min(audioDuration.Seconds()*0.2, 0.5) * float64(time.Second))
			remainingDelay := audioDuration + buffer
			if isFirstMessage {
				remainingDelay -= totalGenerationTime
				isFirstMessage = false
			}
			// Only sleep if we need to wait more
			if remainingDelay > 0 {
				time.Sleep(remainingDelay)
				log.Printf("Waited for %v before next message", remainingDelay)
			} else {
				log.Printf("No delay needed, generation difference exceeds audio duration")
			}
		}
	}
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
			"message": metricsJSON,
		}); err != nil {
			log.Printf("Failed to send conviction analysis: %v", err)
		}
	}
}

// getAudioDuration calculates the duration of AAC audio data
// OpenAI TTS uses AAC-LC with 64kbps bitrate
func getAudioDuration(audioData []byte) time.Duration {
	// AAC-LC 64kbps = 8000 bytes per second
	bytesPerSecond := 8000
	seconds := float64(len(audioData)) / float64(bytesPerSecond)
	return time.Duration(seconds * float64(time.Second))
}
