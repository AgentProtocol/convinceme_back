package server

import (
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/neo/convinceme_backend/internal/agent"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
	EnableCompression: true,
}

type Server struct {
	router       *gin.Engine
	agents       map[string]*agent.Agent
	wsWriteMutex sync.Mutex
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
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

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

	// Setup routes with detailed logging
	router.GET("/ws/conversation", func(c *gin.Context) {
		log.Printf("Received WebSocket connection request from: %s", c.Request.RemoteAddr)
		server.handleConversationWebSocket(c)
	})
	router.POST("/api/conversation/start", server.startConversation)
	router.GET("/api/agents", server.listAgents)

	return server
}

// Run starts the HTTP server
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

type ConversationMessage struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
}

func (s *Server) handleConversationWebSocket(c *gin.Context) {
	log.Printf("Starting WebSocket upgrade process...")

	// Log headers for debugging
	log.Printf("Request Headers: %v", c.Request.Header)

	upgrader.Subprotocols = []string{"binary"} // Add support for binary subprotocol
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		log.Printf("Response headers: %v", c.Writer.Header())
		return
	}
	log.Printf("WebSocket connection successfully established")
	defer ws.Close()

	// Set read deadline to help detect connection issues
	ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start a ping ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			s.wsWriteMutex.Lock()
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.wsWriteMutex.Unlock()
				return
			}
			s.wsWriteMutex.Unlock()
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

					// Send text response with mutex protection
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

					// Create a pipe for streaming audio
					pr, pw := io.Pipe()
					go func() {
						defer pw.Close()
						if err := a.GenerateAndStreamAudio(ctx, response, pw); err != nil {
							log.Printf("Failed to generate audio: %v", err)
						}
					}()

					// Stream the audio data in chunks
					buf := make([]byte, 32*1024) // 32KB chunks
					for {
						n, err := pr.Read(buf)
						if err == io.EOF {
							break
						}
						if err != nil {
							log.Printf("Failed to read audio chunk: %v", err)
							break
						}

						// Send audio chunk as binary message
						s.wsWriteMutex.Lock()
						err = ws.WriteMessage(websocket.BinaryMessage, buf[:n])
						s.wsWriteMutex.Unlock()
						if err != nil {
							log.Printf("Failed to write audio chunk: %v", err)
							break
						}
					}

					// Send end-of-stream marker
					s.wsWriteMutex.Lock()
					err = ws.WriteJSON(map[string]interface{}{
						"type":  "audio_end",
						"agent": a.GetName(),
					})
					s.wsWriteMutex.Unlock()
					if err != nil {
						log.Printf("Failed to write end marker: %v", err)
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
