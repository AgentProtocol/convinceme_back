package server

import (
	"context"
	"crypto/tls"
	"fmt"
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
}

type audioCache struct {
	data      []byte
	timestamp time.Time
}

type ConversationMessage struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
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
	}

	router.GET("/ws/conversation", server.handleConversationWebSocket)
	router.GET("/api/audio/:id", server.handleAudioStream)
	router.POST("/api/conversation/start", server.startConversation)

	router.StaticFile("/", "./test.html")
	router.Static("/static", "./static")

	return server
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

	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.playerMessageMutex.Lock()
			if time.Since(s.lastPlayerMessage) >= 2*time.Minute {
				s.continueAgentDiscussion(ws)
			}
			s.playerMessageMutex.Unlock()
		default:
			var msg ConversationMessage
			err := ws.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				return
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
}

func (s *Server) continueAgentDiscussion(ws *websocket.Conn) {
	ctx := context.Background()
	var wg sync.WaitGroup

	for _, agent := range s.agents {
		a := agent
		wg.Add(1)
		go func() {
			defer wg.Done()

			response, err := a.GenerateResponse(ctx, "General Discussion", "")
			if err != nil {
				log.Printf("Failed to generate response: %v", err)
				return
			}

			textMsg := map[string]interface{}{
				"type":     "text",
				"agent":    a.GetName(),
				"message":  response,
				"memories": a.GetMemory(),
			}
			s.wsWriteMutex.Lock()
			err = ws.WriteJSON(textMsg)
			s.wsWriteMutex.Unlock()
			if err != nil {
				log.Printf("Failed to write text message: %v", err)
				return
			}

			audioData, err := a.GenerateAndStreamAudio(ctx, response)
			if err != nil {
				log.Printf("Failed to generate audio: %v", err)
				return
			}

			audioID := fmt.Sprintf("%s_%d", a.GetName(), time.Now().UnixNano())
			s.cacheMutex.Lock()
			s.audioCache[audioID] = audioCache{
				data:      audioData,
				timestamp: time.Now(),
			}
			s.cacheMutex.Unlock()

			audioMsg := map[string]interface{}{
				"type":     "audio",
				"agent":    a.GetName(),
				"audioUrl": fmt.Sprintf("/api/audio/%s", audioID),
			}
			s.wsWriteMutex.Lock()
			err = ws.WriteJSON(audioMsg)
			s.wsWriteMutex.Unlock()
			if err != nil {
				log.Printf("Failed to write audio message: %v", err)
			}
		}()
	}

	wg.Wait()
}

func (s *Server) handlePlayerMessage(ws *websocket.Conn, msg ConversationMessage) {
	ctx := context.Background()
	var wg sync.WaitGroup

	for _, agent := range s.agents {
		a := agent
		wg.Add(1)
		go func() {
			defer wg.Done()

			response, err := a.GenerateResponse(ctx, msg.Topic, msg.Message)
			if err != nil {
				log.Printf("Failed to generate response: %v", err)
				return
			}

			textMsg := map[string]interface{}{
				"type":     "text",
				"agent":    a.GetName(),
				"message":  response,
				"memories": a.GetMemory(),
			}
			s.wsWriteMutex.Lock()
			err = ws.WriteJSON(textMsg)
			s.wsWriteMutex.Unlock()
			if err != nil {
				log.Printf("Failed to write text message: %v", err)
				return
			}

			audioData, err := a.GenerateAndStreamAudio(ctx, response)
			if err != nil {
				log.Printf("Failed to generate audio: %v", err)
				return
			}

			audioID := fmt.Sprintf("%s_%d", a.GetName(), time.Now().UnixNano())
			s.cacheMutex.Lock()
			s.audioCache[audioID] = audioCache{
				data:      audioData,
				timestamp: time.Now(),
			}
			s.cacheMutex.Unlock()

			audioMsg := map[string]interface{}{
				"type":     "audio",
				"agent":    a.GetName(),
				"audioUrl": fmt.Sprintf("/api/audio/%s", audioID),
			}
			s.wsWriteMutex.Lock()
			err = ws.WriteJSON(audioMsg)
			s.wsWriteMutex.Unlock()
			if err != nil {
				log.Printf("Failed to write audio message: %v", err)
			}
		}()
	}

	wg.Wait()
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
			NextProtos: []string{"h2", "http/1.1"},
		},
	}

	return srv.ListenAndServeTLS("cert.pem", "key.pem")
}
