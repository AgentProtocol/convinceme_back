package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/neo/convinceme_backend/internal/audio"
	"github.com/quic-go/quic-go/http3"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/neo/convinceme_backend/internal/agent"
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
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	EnableCompression: true,
}

func NewServer(agents map[string]*agent.Agent) *Server {
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
	// Add player message to conversation log
	s.addToConversationLog("Player", msg.Message, true)

	// Get conversation context
	conversationContext := s.getConversationContext()

	// Generate responses from both agents
	ctx := context.Background()
	responses := make(map[string]string)
	var wg sync.WaitGroup
	var responseMutex sync.Mutex

	for name, a := range s.agents {
		wg.Add(1)
		go func(agentName string, agent *agent.Agent) {
			defer wg.Done()

			prompt := fmt.Sprintf(`Current conversation context:
%s

A player has just said: "%s"

You are %s, with the role of %s.
Generate a response that:
1. Shows you understand the full conversation context
2. Acknowledges the player's message
3. Stays in character
4. Maintains natural conversation flow
5. Is brief but engaging
6. Interacts with the other agent's previous messages when relevant`,
				conversationContext,
				msg.Message,
				agent.GetName(),
				agent.GetRole())

			response, err := agent.GenerateResponse(ctx, msg.Topic, prompt)
			if err != nil {
				log.Printf("Failed to generate response for %s: %v", agentName, err)
				return
			}

			responseMutex.Lock()
			responses[agentName] = response
			responseMutex.Unlock()
		}(name, a)
	}

	wg.Wait()

	// Send responses in sequence with a delay
	for name, response := range responses {
		agent := s.agents[name]

		// Add agent response to conversation log
		s.addToConversationLog(name, response, false)

		// Send text response
		if err := ws.WriteJSON(gin.H{
			"type":    "text",
			"message": response,
			"agent":   name,
		}); err != nil {
			log.Printf("Failed to send text response for %s: %v", name, err)
			continue
		}

		// Generate and send audio
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

		// Add delay between agent responses
		time.Sleep(2 * time.Second)
	}
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
	prompt := fmt.Sprintf(`Current conversation context:
%s

You are %s, with the role of %s.
Generate a response that:
1. Shows you understand the full conversation context
2. Stays in character
3. Maintains natural conversation flow
4. Is brief but engaging
5. Builds on previous messages when relevant`,
		conversationContext,
		agent.GetName(),
		agent.GetRole())

	response, err := agent.GenerateResponse(ctx, "General Discussion", prompt)
	if err != nil {
		log.Printf("Failed to generate response: %v", err)
		return
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
			log.Fatalf("HTTP/3 server failed: %v", err)
		}
	}()

	// Start the HTTP/1.1 and HTTP/2 server
	return srv.ListenAndServeTLS("cert.pem", "key.pem")
}
