package server

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/neo/convinceme_backend/internal/agent"
)

type Server struct {
	router       *gin.Engine
	agents       map[string]*agent.Agent
	wsWriteMutex sync.Mutex
}

type ConversationMessage struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
	EnableCompression: true,
}

// NewServer creates a new HTTP server with WebSocket support
func NewServer(agents map[string]*agent.Agent) *Server {
	router := gin.Default()

	// Add CORS middleware
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
		router: router,
		agents: agents,
	}

	// Setup routes
	router.GET("/ws/conversation", server.handleConversationWebSocket)
	router.POST("/api/conversation/start", server.startConversation)
	router.GET("/api/agents", server.listAgents)

	// Serve static files with custom headers for HLS
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

func (s *Server) handleConversationWebSocket(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer ws.Close()

	// Handle incoming messages
	for {
		var msg ConversationMessage
		err := ws.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Process conversation in a goroutine
		go func() {
			ctx := context.Background()
			var wg sync.WaitGroup

			for _, agent := range s.agents {
				a := agent // Create a local copy of the agent
				wg.Add(1)
				go func() {
					defer wg.Done()

					// Generate response
					response, err := a.GenerateResponse(ctx, msg.Topic, msg.Message)
					if err != nil {
						log.Printf("Failed to generate response: %v", err)
						return
					}

					// Send text response
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

					// Generate and stream audio
					audioPath, err := a.GenerateAndStreamAudio(ctx, response)
					if err != nil {
						log.Printf("Failed to generate audio: %v", err)
						return
					}

					// Send audio URL
					audioMsg := map[string]interface{}{
						"type":      "audio",
						"agent":     a.GetName(),
						"audioPath": audioPath,
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
		}()
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
	return s.router.Run(addr)
}
