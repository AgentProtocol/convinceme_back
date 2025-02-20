package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/audio"
	"github.com/neo/convinceme_backend/internal/tools"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
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
	rag                *tools.ConversationRAG
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
	Mode    string `json:"mode"` // 'text_only' or 'audio'
}

type audioCache struct {
	data      []byte
	timestamp time.Time
}

type ConvictionMetrics struct {
	Name            string  `json:"name"`
	Confidence      float64 `json:"confidence"`
	Consistency     float64 `json:"consistency"`
	Persuasiveness  float64 `json:"persuasiveness"`
	EmotionalImpact float64 `json:"emotional_impact"`
	Overall         float64 `json:"overall"`
}

type ConvictionUpdate struct {
	Type    string `json:"type"`
	Metrics struct {
		Agent1 ConvictionMetrics `json:"agent1"`
		Agent2 ConvictionMetrics `json:"agent2"`
	} `json:"metrics"`
}

type WebSocketMessage struct {
	Type     string `json:"type"`
	Content  string `json:"content,omitempty"`
	Agent    string `json:"agent,omitempty"`
	AudioURL string `json:"audioUrl,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	EnableCompression: true,
}

func NewServer(agents map[string]*agent.Agent) *Server {
	router := gin.Default()

	// Initialize RAG integration
	dbPath := "data/conversations.db"
	apiKey := os.Getenv("OPENAI_API_KEY")
	rag, err := tools.NewConversationRAG(dbPath, apiKey)
	if err != nil {
		log.Printf("Warning: Failed to initialize RAG: %v", err)
	}

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

	// Add API key middleware for STT endpoint
	router.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/api/stt" {
			apiKey := os.Getenv("OPENAI_API_KEY")
			if apiKey != "" {
				c.Set("openai_api_key", apiKey)
			}
		}
		c.Next()
	})

	server := &Server{
		router:            router,
		agents:            agents,
		audioCache:        make(map[string]audioCache),
		lastPlayerMessage: time.Now(),
		conversationLog:   make([]ConversationEntry, 0),
		rag:               rag,
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

	// Set proper headers for audio streaming
	c.Header("Content-Type", "audio/aac")
	c.Header("Content-Length", fmt.Sprintf("%d", len(cache.data)))
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Expose-Headers", "Content-Length")

	// Stream the audio data
	c.Data(http.StatusOK, "audio/aac", cache.data)
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

	// Set initial deadlines
	ws.SetReadDeadline(time.Now().Add(time.Minute * 5))
	ws.SetWriteDeadline(time.Now().Add(time.Minute * 5))

	// Create a done channel to signal goroutine cleanup
	done := make(chan struct{})
	defer close(done)

	// Create error channel for goroutine errors
	errCh := make(chan error, 1)

	// Start conviction analysis
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				// Get latest conviction metrics
				metrics := s.analyzeCurrentConviction()
				if metrics != nil {
					// Set write deadline for metrics update
					ws.SetWriteDeadline(time.Now().Add(time.Second * 10))
					if err := ws.WriteJSON(metrics); err != nil {
						log.Printf("Failed to send conviction metrics: %v", err)
						errCh <- fmt.Errorf("failed to send conviction metrics: %v", err)
						return
					}
					log.Printf("Sent conviction metrics update")
				}
			}
		}
	}()

	// Handle incoming messages
	for {
		select {
		case err := <-errCh:
			log.Printf("WebSocket error: %v", err)
			return
		default:
			var msg ConversationMessage
			err := ws.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				return
			}

			// Reset read deadline after each successful message
			ws.SetReadDeadline(time.Now().Add(time.Minute * 5))

			// Handle ping messages
			if msg.Type == "ping" {
				ws.SetWriteDeadline(time.Now().Add(time.Second * 10))
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
				if err := s.continueAgentDiscussion(ws); err != nil {
					log.Printf("Failed to continue agent discussion: %v", err)
					return
				}
			} else {
				if err := s.handlePlayerMessage(ws, msg); err != nil {
					log.Printf("Failed to handle player message: %v", err)
					return
				}
			}
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

func (s *Server) handlePlayerMessage(ws *websocket.Conn, msg ConversationMessage) error {
	// Add player message to conversation log
	s.addToConversationLog("Player", msg.Message, true)

	// Get conversation context
	conversationContext := s.getConversationContext()

	// Get historical context from RAG if available
	historicalContext := s.getHistoricalContext(msg)

	// Generate responses from both agents
	ctx := context.Background()
	responses := make(map[string]string)
	var wg sync.WaitGroup
	var responseMutex sync.Mutex
	var responseErrors []error

	for name, a := range s.agents {
		wg.Add(1)
		go func(agentName string, agent *agent.Agent) {
			defer wg.Done()

			// Combine current context with historical context
			fullContext := conversationContext
			if historicalContext != "" {
				fullContext = fmt.Sprintf("%s\n\nBased on previous conversations:\n%s",
					conversationContext, historicalContext)
			}

			prompt := fmt.Sprintf(`Current conversation context:
%s

A player has just said: "%s"

You are %s, with the role of %s.
Generate a concise response that:
1. Addresses the core point in 1-2 sentences
2. Uses precise, impactful language
3. Stays true to your character
4. Maintains conversation flow
5. Avoids unnecessary elaboration
6. Makes every word count
7. References key context when vital`,
				fullContext,
				msg.Message,
				agent.GetName(),
				agent.GetRole())

			response, err := agent.GenerateResponse(ctx, msg.Topic, prompt)
			if err != nil {
				responseMutex.Lock()
				responseErrors = append(responseErrors, fmt.Errorf("failed to generate response for %s: %v", agentName, err))
				responseMutex.Unlock()
				return
			}

			responseMutex.Lock()
			responses[agentName] = response
			responseMutex.Unlock()

			// Store agent's response in RAG immediately
			memory := &tools.ConversationMemory{
				ID:         fmt.Sprintf("resp_%d_%s", time.Now().UnixNano(), agentName),
				Topic:      msg.Topic,
				Timestamp:  time.Now(),
				AgentNames: []string{agentName},
				Messages: []tools.Message{
					{
						AgentName: agentName,
						Content:   response,
						Timestamp: time.Now(),
					},
				},
			}

			// Store in RAG
			request := tools.RAGRequest{
				Action:       "store",
				Conversation: memory,
			}

			requestJSON, err := json.Marshal(request)
			if err == nil {
				if _, err := s.rag.Call(context.Background(), string(requestJSON)); err != nil {
					log.Printf("Warning: Failed to store agent response in RAG: %v", err)
				}
			}
		}(name, a)
	}

	wg.Wait()

	if len(responseErrors) > 0 {
		return fmt.Errorf("errors generating responses: %v", responseErrors)
	}

	// Send responses in sequence
	for name, response := range responses {
		// Store the message in conversation log
		s.addToConversationLog(name, response, false)

		// Send text response
		if err := s.sendWebSocketMessage(ws, WebSocketMessage{
			Type:    "message",
			Content: response,
			Agent:   name,
		}); err != nil {
			return fmt.Errorf("failed to send message: %v", err)
		}

		// Generate and send audio only if in audio mode
		if msg.Mode == "audio" {
			audioData, err := s.agents[name].GenerateAndStreamAudio(ctx, response)
			if err != nil {
				log.Printf("Warning: Failed to generate audio for %s: %v", name, err)
				continue
			}

			// Save audio file and send URL
			audioURL, err := s.saveAndGetAudioURL(name, audioData)
			if err != nil {
				log.Printf("Warning: Failed to save audio for %s: %v", name, err)
				continue
			}

			if err := s.sendWebSocketMessage(ws, WebSocketMessage{
				Type:     "audio",
				AudioURL: audioURL,
				Agent:    name,
				Mode:     "audio",
			}); err != nil {
				log.Printf("Warning: Failed to send audio message for %s: %v", name, err)
			}
		}

		// Add delay between responses
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

func (s *Server) continueAgentDiscussion(ws *websocket.Conn) error {
	// Get conversation context
	conversationContext := s.getConversationContext()

	// Get historical context from RAG if available
	historicalContext := s.getHistoricalContext(ConversationMessage{
		Topic: "General Discussion",
		Type:  "continue",
	})

	// Generate responses from both agents
	ctx := context.Background()
	responses := make(map[string]string)
	var wg sync.WaitGroup
	var responseMutex sync.Mutex
	var responseErrors []error

	for name, a := range s.agents {
		wg.Add(1)
		go func(agentName string, a *agent.Agent) {
			defer wg.Done()

			prompt := fmt.Sprintf(`Current conversation context:
%s%s

You are %s, with the role of %s.
Continue the conversation with a brief response that:
1. Makes a single clear point
2. Uses minimal but impactful words
3. Stays true to your character
4. Maintains dialogue flow
5. Focuses on the essential
6. Builds on previous insights concisely`,
				conversationContext,
				historicalContext,
				a.GetName(),
				a.GetRole())

			response, err := a.GenerateResponse(ctx, "General Discussion", prompt)
			if err != nil {
				responseMutex.Lock()
				responseErrors = append(responseErrors, fmt.Errorf("failed to generate response for %s: %v", agentName, err))
				responseMutex.Unlock()
				return
			}

			responseMutex.Lock()
			responses[agentName] = response
			responseMutex.Unlock()
		}(name, a)
	}

	wg.Wait()

	if len(responseErrors) > 0 {
		return fmt.Errorf("errors generating responses: %v", responseErrors)
	}

	// Send responses in sequence
	for name, response := range responses {
		// Store the message in conversation log
		s.addToConversationLog(name, response, false)

		// Send text response
		if err := s.sendWebSocketMessage(ws, WebSocketMessage{
			Type:    "message",
			Content: response,
			Agent:   name,
		}); err != nil {
			return fmt.Errorf("failed to send message: %v", err)
		}

		// Add a small delay between responses
		time.Sleep(500 * time.Millisecond)
	}

	return nil
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

func (s *Server) Run(addr string, certFile string, keyFile string) error {
	// Ensure we clean up resources when the server stops
	defer s.Close()

	// Create connection state tracking
	var activeConnections sync.Map

	// Configure HTTP server with custom error logging
	srv := &http.Server{
		Addr:     addr,
		Handler:  s.router,
		ErrorLog: log.New(os.Stderr, "server: ", log.LstdFlags|log.Lshortfile),
		ConnState: func(conn net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				activeConnections.Store(conn.RemoteAddr(), time.Now())
				log.Printf("New connection from %s", conn.RemoteAddr())
			case http.StateActive:
				activeConnections.Store(conn.RemoteAddr(), time.Now())
				log.Printf("Connection active: %s", conn.RemoteAddr())
			case http.StateClosed, http.StateHijacked:
				activeConnections.Delete(conn.RemoteAddr())
				log.Printf("Connection ended: %s", conn.RemoteAddr())
			}
		},
	}

	// Update WebSocket upgrader to be more permissive
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true // Allow all origins in development
	}
	upgrader.EnableCompression = true

	// Monitor active connections
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			count := 0
			activeConnections.Range(func(key, value interface{}) bool {
				count++
				return true
			})
			if count > 0 {
				log.Printf("Active connections: %d", count)
			}
		}
	}()

	log.Printf("Server starting on %s (HTTP)", addr)
	log.Printf("⚠️  Development mode: Running in HTTP mode for easier development")
	log.Printf("👉 Access the application:")
	log.Printf("   Open http://localhost%s", addr)

	// Start the HTTP server
	if err := srv.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("server error: %v", err)
	}

	return nil
}

func (s *Server) analyzeCurrentConviction() *ConvictionUpdate {
	s.conversationMutex.RLock()
	defer s.conversationMutex.RUnlock()

	if len(s.conversationLog) == 0 {
		return nil
	}

	// Create dialogue history for each agent
	agent1History := make([]string, 0)
	agent2History := make([]string, 0)
	var agent1Name, agent2Name string

	for _, entry := range s.conversationLog {
		if !entry.IsPlayer {
			if agent1Name == "" {
				agent1Name = entry.Speaker
				agent1History = append(agent1History, entry.Message)
			} else if entry.Speaker == agent1Name {
				agent1History = append(agent1History, entry.Message)
			} else if agent2Name == "" {
				agent2Name = entry.Speaker
				agent2History = append(agent2History, entry.Message)
			} else if entry.Speaker == agent2Name {
				agent2History = append(agent2History, entry.Message)
			}
		}
	}

	// Create conviction meter
	meter := tools.ConvictionMeter{}

	// Analyze agent1
	agent1Input := tools.ConvictionInput{
		AgentName:       agent1Name,
		DialogueHistory: agent1History,
		Topic:           "Current Discussion",
	}
	agent1InputJSON, _ := json.Marshal(agent1Input)
	agent1Result, err := meter.Call(context.Background(), string(agent1InputJSON))
	if err != nil {
		log.Printf("Failed to analyze agent1 conviction: %v", err)
		return nil
	}

	var agent1Metrics tools.ConvictionMetrics
	if err := json.Unmarshal([]byte(agent1Result), &agent1Metrics); err != nil {
		log.Printf("Failed to unmarshal agent1 metrics: %v", err)
		return nil
	}

	// Analyze agent2
	agent2Input := tools.ConvictionInput{
		AgentName:       agent2Name,
		DialogueHistory: agent2History,
		Topic:           "Current Discussion",
	}
	agent2InputJSON, _ := json.Marshal(agent2Input)
	agent2Result, err := meter.Call(context.Background(), string(agent2InputJSON))
	if err != nil {
		log.Printf("Failed to analyze agent2 conviction: %v", err)
		return nil
	}

	var agent2Metrics tools.ConvictionMetrics
	if err := json.Unmarshal([]byte(agent2Result), &agent2Metrics); err != nil {
		log.Printf("Failed to unmarshal agent2 metrics: %v", err)
		return nil
	}

	// Create update
	update := &ConvictionUpdate{
		Type: "conviction",
		Metrics: struct {
			Agent1 ConvictionMetrics `json:"agent1"`
			Agent2 ConvictionMetrics `json:"agent2"`
		}{
			Agent1: ConvictionMetrics{
				Name:            agent1Name,
				Confidence:      agent1Metrics.Confidence,
				Consistency:     agent1Metrics.Consistency,
				Persuasiveness:  agent1Metrics.Persuasiveness,
				EmotionalImpact: agent1Metrics.EmotionalImpact,
				Overall:         agent1Metrics.Overall,
			},
			Agent2: ConvictionMetrics{
				Name:            agent2Name,
				Confidence:      agent2Metrics.Confidence,
				Consistency:     agent2Metrics.Consistency,
				Persuasiveness:  agent2Metrics.Persuasiveness,
				EmotionalImpact: agent2Metrics.EmotionalImpact,
				Overall:         agent2Metrics.Overall,
			},
		},
	}

	return update
}

// Close cleans up resources used by the server
func (s *Server) Close() error {
	if s.rag != nil {
		if err := s.rag.Close(); err != nil {
			return fmt.Errorf("failed to close RAG: %v", err)
		}
	}
	return nil
}

// sendWebSocketMessage sends a message through the WebSocket connection
func (s *Server) sendWebSocketMessage(ws *websocket.Conn, msg WebSocketMessage) error {
	s.wsWriteMutex.Lock()
	defer s.wsWriteMutex.Unlock()

	// Set write deadline for each message
	ws.SetWriteDeadline(time.Now().Add(time.Second * 10))
	return ws.WriteJSON(msg)
}

// getHistoricalContext retrieves relevant historical context from RAG
func (s *Server) getHistoricalContext(msg ConversationMessage) string {
	if s.rag == nil {
		return ""
	}

	// Create memory entry for current conversation
	memory := &tools.ConversationMemory{
		ID:         fmt.Sprintf("conv_%d", time.Now().UnixNano()),
		Topic:      msg.Topic,
		Timestamp:  time.Now(),
		AgentNames: make([]string, 0, len(s.agents)),
		Messages:   make([]tools.Message, 0),
	}

	// Add agent names
	for name := range s.agents {
		memory.AgentNames = append(memory.AgentNames, name)
	}

	// Add messages from conversation log
	s.conversationMutex.RLock()
	for _, entry := range s.conversationLog {
		memory.Messages = append(memory.Messages, tools.Message{
			AgentName: entry.Speaker,
			Content:   entry.Message,
			Timestamp: entry.Time,
		})
	}
	s.conversationMutex.RUnlock()

	// Store in RAG
	request := tools.RAGRequest{
		Action:       "store",
		Conversation: memory,
	}

	requestJSON, err := json.Marshal(request)
	if err == nil {
		if _, err := s.rag.Call(context.Background(), string(requestJSON)); err != nil {
			log.Printf("Warning: Failed to store conversation in RAG: %v", err)
		}
	}

	// Query for relevant history
	queryRequest := tools.RAGRequest{
		Action: "query",
		Query: &tools.MemoryQuery{
			Topic:  msg.Topic,
			Agents: memory.AgentNames,
			Limit:  3,
		},
	}

	queryJSON, err := json.Marshal(queryRequest)
	if err != nil {
		return ""
	}

	result, err := s.rag.Call(context.Background(), string(queryJSON))
	if err != nil {
		return ""
	}

	var memories []tools.ConversationMemory
	if err := json.Unmarshal([]byte(result), &memories); err != nil || len(memories) == 0 {
		return ""
	}

	historicalContext := "\n\nRelevant historical context:\n"
	for i, mem := range memories {
		historicalContext += fmt.Sprintf("%d. %s\n", i+1, mem.Summary)
	}

	return historicalContext
}

// saveAndGetAudioURL saves audio data and returns its URL
func (s *Server) saveAndGetAudioURL(agentName string, audioData []byte) (string, error) {
	audioID := fmt.Sprintf("%s_%d", agentName, time.Now().UnixNano())

	s.cacheMutex.Lock()
	s.audioCache[audioID] = audioCache{
		data:      audioData,
		timestamp: time.Now(),
	}
	s.cacheMutex.Unlock()

	return fmt.Sprintf("/api/audio/%s", audioID), nil
}
