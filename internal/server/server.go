package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/neo/convinceme_backend/internal/audio"
	"github.com/neo/convinceme_backend/internal/conversation"
	"github.com/neo/convinceme_backend/internal/player"
	"github.com/quic-go/quic-go/http3"
	"github.com/tcolgate/mp3"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/neo/convinceme_backend/internal/scoring"
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
	useHTTPS           bool
	config             *Config
	scorer             *scoring.Scorer
	db                 *database.Database
	// Connection-related fields with single mutex
	connectionMutex     sync.RWMutex
	clients             map[*websocket.Conn]string // Map of WebSocket connections to player IDs
	conversationStarted bool
	stopDiscussion      chan struct{}
	// Add new fields for tracking conversation state
	conversationID int
	conversationWg sync.WaitGroup
	// Add a dedicated lock for conversation state transitions
	conversationStateMutex sync.Mutex
	// Other fields
	userMessages           chan string
	lastAudioStart         time.Time
	requiredDelay          time.Duration
	pendingUserMessage     string
	pendingUserMessageLock sync.RWMutex
	playerQueue            map[string]int // Map of player IDs to their queue numbers
	playerQueueMux         sync.RWMutex
	nextQueueNum           int // Next available queue number
	sharedConversation     *conversation.Conversation
}

type ConversationEntry struct {
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
	duration  time.Duration
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

var agent1Score int = MAX_SCORE / 2
var agent2Score int = MAX_SCORE / 2

func NewServer(agents map[string]*agent.Agent, db *database.Database, apiKey string, useHTTPS bool, config *Config) *Server {
	// Initialize player queue tracking
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

	// Create input handler for the shared conversation
	inputHandler := player.NewInputHandler(log.Default())

	// Get the first two agents for the conversation
	var agent1, agent2 *agent.Agent
	for _, a := range agents {
		if agent1 == nil {
			agent1 = a
		} else if agent2 == nil {
			agent2 = a
			break
		}
	}

	server := &Server{
		router:            router,
		agents:            agents,
		audioCache:        make(map[string]audioCache),
		lastPlayerMessage: time.Now(),
		conversationLog:   make([]ConversationEntry, 0),
		useHTTPS:          useHTTPS,
		config:            config,
		scorer:            scorer,
		db:                db,
		// Initialize new fields
		userMessages:   make(chan string, 10),
		stopDiscussion: make(chan struct{}),
		lastAudioStart: time.Now(),
		requiredDelay:  0,
		// Initialize clients map and player queue tracking
		clients:      make(map[*websocket.Conn]string),
		playerQueue:  make(map[string]int),
		nextQueueNum: 1,
		// Initialize shared conversation
		sharedConversation: conversation.NewConversation(
			agent1,
			agent2,
			conversation.DefaultConfig(),
			inputHandler,
			apiKey,
		),
		conversationStarted: false,
		// Initialize conversation tracking fields
		conversationID:         0,
		conversationWg:         sync.WaitGroup{},
		conversationStateMutex: sync.Mutex{},
	}

	router.GET("/ws/conversation", server.handleConversationWebSocket)
	router.GET("/api/audio/:id", server.handleAudioStream)
	router.POST("/api/conversation/start", server.startConversation)
	router.POST("/api/stt", audio.HandleSTT)
	router.GET("/api/agents", server.listAgents)
	router.GET("/api/arguments", server.getArguments)
	router.GET("/api/arguments/:id", server.getArgument)
	router.GET("/api/gameScore", server.getGameScore)

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

	log.Printf("Server initialized with %d agents", len(agents))
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

func (s *Server) handleConversationWebSocket(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer ws.Close()

	// Lock the conversation state to check if we can start a new one
	s.conversationStateMutex.Lock()

	// Add client to map first (always do this)
	s.connectionMutex.Lock()
	s.clients[ws] = "" // Initially empty player ID
	s.connectionMutex.Unlock()

	// Check if conversation needs to be started
	var startConversation bool
	var currentConversationID int

	if !s.conversationStarted {
		s.conversationStarted = true
		startConversation = true
		// Increment conversation ID to ensure unique identification
		s.conversationID++
		currentConversationID = s.conversationID
		// Create a new stop channel
		s.stopDiscussion = make(chan struct{})
		log.Printf("Starting new conversation with ID: %d", currentConversationID)
	} else {
		currentConversationID = s.conversationID
		log.Printf("Client joined existing conversation with ID: %d", currentConversationID)
	}

	// Start conversation outside the locks if needed
	if startConversation {
		s.conversationWg.Add(1)
		go func() {
			defer s.conversationWg.Done()
			s.continueAgentDiscussion(ws, currentConversationID)
		}()
	}

	// Release the state lock now that we've decided what to do
	s.conversationStateMutex.Unlock()

	// Ensure cleanup on disconnect
	defer func() {
		// Lock to prevent race conditions during cleanup
		s.conversationStateMutex.Lock()
		defer s.conversationStateMutex.Unlock()

		s.connectionMutex.Lock()
		delete(s.clients, ws)
		remaining := len(s.clients)
		s.connectionMutex.Unlock()

		// Only stop conversation if this was the last client
		if remaining == 0 && s.conversationStarted {
			log.Printf("Last client disconnected. Stopping conversation ID: %d", s.conversationID)
			s.conversationStarted = false
			close(s.stopDiscussion)

			// Wait for conversation goroutine to fully terminate
			// This happens while holding the state lock to prevent new conversations from starting
			log.Printf("Waiting for conversation ID %d to terminate...", s.conversationID)
			s.conversationWg.Wait()
			log.Printf("Conversation ID %d fully terminated", s.conversationID)
		}
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

		// Update player ID in clients map and assign queue number if not set
		if msg.PlayerID != "" {
			s.connectionMutex.Lock()
			s.clients[ws] = msg.PlayerID
			s.connectionMutex.Unlock()

			// Assign queue number if not already assigned
			s.playerQueueMux.Lock()
			if _, exists := s.playerQueue[msg.PlayerID]; !exists {
				s.playerQueue[msg.PlayerID] = s.nextQueueNum
				s.nextQueueNum++
			}
			s.playerQueueMux.Unlock()
		}

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

		// Broadcast message to all connected clients with queue number
		s.playerQueueMux.Lock()
		queueNum := s.playerQueue[msg.PlayerID]
		s.playerQueueMux.Unlock()

		s.broadcastMessage(gin.H{
			"type":        "message",
			"content":     msg.Message,
			"agent":       msg.PlayerID,
			"queueNumber": queueNum,
		})

		s.handlePlayerMessage(ws, msg)

		log.Printf("User message received: %s", msg.Message)
	}
}

// Add a new method to broadcast messages to all connected clients
func (s *Server) broadcastMessage(message interface{}) {
	s.connectionMutex.RLock()
	defer s.connectionMutex.RUnlock()

	// Iterate through all connected clients and send the message
	for client := range s.clients {
		if err := client.WriteJSON(message); err != nil {
			log.Printf("Error broadcasting message: %v", err)
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
	log.Printf("Received WebSocket message: %+v", msg)
	s.addToConversationLog("Player", msg.Message, true, msg.Topic)

	// Generate responses from both agents
	ctx := context.Background()

	// Get agent names
	agent1Name, agent2Name := s.GetOrderedAgentNames()

	log.Printf("\n-------------->agent1Name is %s, agent2Name is %s\n", agent1Name, agent2Name)

	// Score the user's message first
	if s.scorer != nil {
		log.Printf("\n=== Scoring User Message ===\n")
		score, err := s.scorer.ScoreArgument(ctx, msg.Message, msg.Topic)
		if err != nil {
			log.Printf("Failed to score user message: %v", err)
		} else {
			// Save argument and score to database
			// Determine which side the argument supports based on agent support scores

			log.Printf("Argument supports: %s", msg.Side)

			argID, err := s.db.SaveArgument(msg.PlayerID, msg.Topic, msg.Message, msg.Side)
			if err != nil {
				log.Printf("Failed to save argument: %v", err)
			} else {
				if err := s.db.SaveScore(argID, score); err != nil {
					log.Printf("Failed to save score: %v", err)
				}

				// Send the complete argument with score through WebSocket
				argument := database.Argument{
					ID:        argID,
					PlayerID:  msg.PlayerID,
					Topic:     msg.Topic,
					Content:   msg.Message,
					Side:      msg.Side,
					Score:     score,
					CreatedAt: time.Now().Format(time.RFC3339),
				}

				log.Printf("Sending argument data through WebSocket: %+v", argument)

				s.broadcastMessage(gin.H{
					"type":     "argument",
					"argument": argument,
				})
			}

			// Send score back to user
			s.broadcastMessage(gin.H{
				"type": "score",
				"message": fmt.Sprintf("Argument score:\n"+
					"Strength: %d/100\n"+
					"Relevance: %d/100\n"+
					"Logic: %d/100\n"+
					"Truth: %d/100\n"+
					"Humor: %d/100\n"+
					"Average: %.1f/100\n"+
					"Explanation: %s\n",
					score.Strength,
					score.Relevance,
					score.Logic,
					score.Truth,
					score.Humor,
					score.Average,
					score.Explanation),
			})
			log.Printf("Raw Jason values in server.go are:\n")

			log.Printf("Argument score:\n"+
				"Strength: %d/100\n"+
				"Relevance: %d/100\n"+
				"Logic: %d/100\n"+
				"Truth: %d/100\n"+
				"Humor: %d/100\n"+
				"Average: %.1f/100\n"+
				"Explanation: %s\n",
				score.Strength,
				score.Relevance,
				score.Logic,
				score.Truth,
				score.Humor,
				score.Average,
				score.Explanation)
		}

		if msg.Side == agent1Name {
			agent1Score = agent1Score + int(score.Average)
			agent2Score = agent2Score - int(score.Average)
		} else {
			agent2Score = agent2Score + int(score.Average)
			agent1Score = agent1Score - int(score.Average)
		}
		log.Printf("%s scored %d points ", msg.Side, int(score.Average))

		s.broadcastMessage(gin.H{
			"type": "game_score",
			"gameScore": gin.H{
				agent1Name: agent1Score,
				agent2Name: agent2Score,
			},
		})
	}
}

// This is the initial prompt when there's no player message (empty string)
func getPrompt(conversationContext string, playerMessage string, agentName string, agentRole string) string {
	switch playerMessage {
	case "":
		return fmt.Sprintf(`Current conversation context: %s

You are %s, with the role of %s.
Topic: Are memecoins net negative or positive for the crypto space?

CRITICAL INSTRUCTIONS
1. You MUST respond with EXACTLY 1-2 SHORT sentences
2. DIRECTLY ADDRESS the previous speaker's point before making your counter-argument
3. Use the sample arguments as your core message, but adapt them slightly to maintain natural conversation flow
4. Never use emojis or smileys
5. Never repeat arguments that have already been used in the conversation. This is a hard requirement.

RESPONSE PRIORITY ORDER:
1. Briefly acknowledge or counter the previous point
2. Then deliver your argument using one of the sample arguments below
3. Keep it engaging and confrontational, but natural

Generate a response that:
1. Focuses on one specific argument about memecoin impact
3. Directly addresses previous points when relevant
5. Keeps responses concise (1-2 sentences maximum)
6. Do not use emojis or smileys!

DEBATE GUIDELINES:
1. Make it engaging and fun
2. Use crypto slang and terminology where appropriate

CRITICAL ROLE ENFORCEMENT:
- If you are 'Memecoin Supercycle' Murad: You MUST use PRO-MEMECOIN arguments ONLY
- If you are 'Fundamentals First' Bradford: You MUST use ANTI-MEMECOIN arguments ONLY
- Never switch sides or contradict your assigned position
- Never repeat an argument you've already used in the conversation

CONVERSATION STYLE:
Murad should be:
- Ultra confident and dismissive
- Uses phrases like "NGMI", "ser", "aping in", "moon"
- Brags about gains and mocks traditional thinking
- Proud of being a degen and embraces it

Bradford should be:
- Sarcastically analytical
- Uses precise numbers to mock memecoin failures
- Points out absurdities in memecoin logic
- Condescending about "innovative" memecoin features

PRO-MEMECOIN ARGUMENTS (MURAD ONLY):
- "Memecoins democratize crypto gains - while VCs get all the good presale allocations for 'serious' projects, ANY retail trader can catch the next PEPE pump!"
- "NGMI if you can't see how memecoins are pure GALAXY BRAIN marketing ser - when $DOGE mooned so hard it put Elon on SNL, every normie's uncle was suddenly aping in!
- "Memecoins are literally the GREATEST MARKETING TOOL crypto has ever seen - when DOGE gets on SNL, millions of normies learn about crypto whether they want to or not!"
- "Memecoins are the perfect gateway drug to DeFi - you start with DOGE, then you learn about DEXes, then suddenly you're yield farming like a pro!"
- "One viral Wojak meme brings more eyeballs than 9000 GitHub commits fam - while you're reading documentation, I'm building generational wealth on $BONK!"
- "The massive liquidity memecoins bring to DEXes helps the ENTIRE ecosystem - when people come for FARTCOIN, they stay for everything else!"
- "Have fun staying poor, but I'm out here making bank!"
- "Imagine being so smol brain you can't see memecoins are the ultimate bull market indicator - while you're waiting for TradFi signals, we're already in Lambo territory!"
- "I'm not a financial advisor, but if I were to invest $100 in crypto, I'd put it all in memecoins - they're the only ones that make sense!"
- "I'm molded by the trenches you wouldn't survive a day in. I'm a degenerate, I'm a hustler, and I'm not gon stop."
- "Getting rugged by ElonCumRocket69Inu teaches more about DYOR than your fancy blockchain course ser - school of hard knocks is now on-chain!"
- "I'm only a 100x away from finally coming down from the closet on Thanksgiving."
- "We invented token burns because number go up technology is LITERALLY the future - cope and seethe while we innovate our way to the moon!"

SAMPLE ANTI-MEMECOIN ARGUMENTS (BRADFORD ONLY):
- "Many memecoins traders went absolutely bankrupt and they are never coming back."
- "The SEC cited memecoin manipulation as a key reason for rejecting spot ETF applications, directly harming legitimate crypto projects."
- "Last quarter's data shows memecoin speculation consumed 40% of Ethereum's gas, making the network unusable for legitimate DeFi applications."
- "While serious teams build MEV-resistant protocols and implement veTokenomics, memecoin 'developers' are literally copy-pasting contracts and adding 'Inu' to the name - stellar contribution to the space"
"The average memecoin loses 99.8% of its value within 30 days of launch according to DeFiLlama data - meanwhile, real DeFi protocols with actual revenue-sharing mechanisms continue building regardless of market conditions"
"Your memecoin's liquidity is thinner than the developer's moral compass - but I'm sure those 'locked' tokens are totally safe behind that 24-hour timelock"
"The combined code quality of every memecoin launched this year has fewer security features than a MySpace page from 2006 - but at least the dog logo is cute"
"The average memecoin dev's GitHub activity looks like a flatline EKG - copy-pasting SafeMoon's code and changing the emoji doesn't count as 'innovative tokenomics', ser"
"Your token's buy tax is higher than the collective IQ of its Telegram group - but sure, tell me more about how it's 'democratizing finance'"
"Your memecoin's roadmap has more red flags than a Soviet military parade - but I'm sure 'Phase 4: Moon' is thoroughly planned out"
"The average memecoin holder's portfolio duration is shorter than a TikTok attention span - speedrunning from FOMO to food stamps"


Keep responses focused on the core debate about memecoin impact on crypto.`, conversationContext, agentName, agentRole)
		// This is the prompt when there's a player message
	default:
		return fmt.Sprintf(`Current conversation context: %s

You are %s, with the role of %s.
Generate a response that:
1. Shows you understand the full conversation context
2. Acknowledges the player's message if there is one
3. Stays in character
4. Maintains natural conversation flow
5. Is brief but engaging
6. Interacts with the other agent's previous messages when relevant
7. Keep it short and concise and only use maximally two short sentences
8. Use your character's specific crypto slang and terminology

REMEMBER:
1. Be PASSIONATE about your stance on memecoins!
2. Challenge the other person's viewpoint (but keep it playful)
3. Use crypto-specific metaphors and comparisons
4. Reference actual protocols, metrics, or memes depending on your character
5. Use your character's signature language style
6. Make your arguments memorable and punchy
7. Keep it short - 2 sentences maximum. This is a MUST obey condition.
8. Ideally try to limit it to one powerful statement.

Examples of the tone we want:
For the Degen:
- "Ser, while you're reading whitepapers, my $PEPE bag just did a 100x - this is what mass adoption looks like!"
- "NGMI with that boomer mentality, memecoins are literally onboarding more users than your precious L2s!"

For the Analyst:
- "Your 'community-driven' memecoin just rugged faster than you can say 'sustainable tokenomics'"
- "While you're chasing pumps, actual DeFi protocols generated $69M in real revenue last month"
- "Memecoins are a scam and a waste of time and literally bankrupted thousands of people"

Keep it spicy, keep it authentic to your character, but make your points count!`, conversationContext, playerMessage, agentName, agentRole)
	}
}

func (s *Server) continueAgentDiscussion(ws *websocket.Conn, conversationID int) {
	log.Printf("Starting agent discussion for conversation ID: %d", conversationID)

	// Check if conversation is still active
	s.connectionMutex.RLock()
	active := s.conversationStarted && conversationID == s.conversationID
	hasClients := len(s.clients) > 0
	s.connectionMutex.RUnlock()

	if !active || !hasClients {
		log.Printf("Conversation %d not active or no clients. Exiting.", conversationID)
		return
	}

	log.Printf("Conversation %d is active with %d clients", conversationID, len(s.clients))
	isfirstmessage := true

	for {
		// Recheck conversation status at the start of each loop
		s.connectionMutex.RLock()
		active := s.conversationStarted && conversationID == s.conversationID
		hasClients := len(s.clients) > 0
		clientCount := len(s.clients)
		s.connectionMutex.RUnlock()

		if !active || !hasClients {
			log.Printf("Conversation %d no longer active or no clients (%d). Exiting loop.", conversationID, clientCount)
			return
		}

		select {
		case <-s.stopDiscussion:
			log.Printf("Stop signal received for conversation %d", conversationID)
			return
		default:
			// Check if conversation is still the current one
			s.connectionMutex.RLock()
			isCurrentConversation := conversationID == s.conversationID && s.conversationStarted
			s.connectionMutex.RUnlock()

			if !isCurrentConversation {
				log.Printf("Conversation %d is no longer the current conversation (current is %d). Exiting.",
					conversationID, s.conversationID)
				return
			}

			// Check if enough time has passed since last audio
			timeSinceLastAudio := time.Since(s.lastAudioStart)

			if timeSinceLastAudio < s.requiredDelay {
				isfirstmessage = false
				waitTime := s.requiredDelay - timeSinceLastAudio
				log.Printf("Conversation %d: Waiting %v before next message", conversationID, waitTime)

				// Instead of a single long sleep, break it into smaller sleeps
				// and check if we should continue after each one
				sleepIncrement := 500 * time.Millisecond
				for timeLeft := waitTime; timeLeft > 0; timeLeft -= sleepIncrement {
					sleepDuration := sleepIncrement
					if timeLeft < sleepIncrement {
						sleepDuration = timeLeft
					}
					time.Sleep(sleepDuration)

					// Check if we should exit during sleep
					select {
					case <-s.stopDiscussion:
						log.Printf("Stop signal received for conversation %d during wait", conversationID)
						return
					default:
						// Continue waiting
					}

					// Check if conversation is still valid
					s.connectionMutex.RLock()
					stillValid := conversationID == s.conversationID && s.conversationStarted
					stillHasClients := len(s.clients) > 0
					s.connectionMutex.RUnlock()

					if !stillValid || !stillHasClients {
						log.Printf("Conversation %d is no longer valid during wait. Exiting.", conversationID)
						return
					}
				}
			} else {
				isfirstmessage = true
			}

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

			ctx := context.Background()
			prompt := getPrompt(conversationContext, pendingMessage, agent.GetName(), agent.GetRole())

			// Time the response generation
			responseStart := time.Now()
			response, err := agent.GenerateResponse(ctx, "Are memecoins net negative or positive for the crypto space?", prompt)
			responseGenerationTime := time.Since(responseStart)
			if err != nil {
				log.Printf("Failed to generate response: %v", err)
				continue
			}

			// Add response to conversation log
			s.addToConversationLog(agent.GetName(), response, false, "Are memecoins net negative or positive for the crypto space?")

			// Broadcast text response to all clients
			s.broadcastMessage(gin.H{
				"type":    "text",
				"message": response,
				"agent":   agent.GetName(),
			})

			// Time the audio generation
			audioStart := time.Now()
			audioData, err := agent.GenerateAndStreamAudio(ctx, response)
			audioGenerationTime := time.Since(audioStart)
			if err != nil {
				log.Printf("Failed to generate audio server: %v", err)
				s.lastAudioStart = time.Now()
				s.requiredDelay = 5 * time.Second
				continue
			}

			audioDuration := getAudioDuration(audioData)
			go func() {
				// Wait for the audio to finish
				time.Sleep(audioDuration)
				
				// Now score the response
				score, err := s.scorer.ScoreArgument(ctx, response, "Bear vs Tiger: Who would win in a fight?")
				if err != nil {
					log.Printf("Error scoring response: %v", err)
					return
				}
				// Agents arguments are two times less impactful as compared to the player's
				if (agent.GetName() == TIGER_AGENT) {
					agent1Score = agent1Score + int(score.Average)/2
					agent2Score = agent2Score - int(score.Average)/2
				} else {
					agent2Score = agent2Score + int(score.Average)/2
					agent1Score = agent1Score - int(score.Average)/2
				}
				log.Printf("------------------>%s scored %d points <---------------", agent.GetName(), int(score.Average))
		
				s.broadcastMessage(gin.H{
					"type": "game_score",
					"gameScore": gin.H{
						TIGER_AGENT: agent1Score,
						BEAR_AGENT: agent2Score,
					},
				})
				
			
				// Handle the score result here
				log.Printf("Score calculated after audio delay: %v", score)
			}()

			audioID := fmt.Sprintf("%s_%d", agent.GetName(), time.Now().UnixNano())

			s.cacheMutex.Lock()
			s.audioCache[audioID] = audioCache{
				data:      audioData,
				timestamp: time.Now(),
				duration:  audioDuration,
			}
			s.cacheMutex.Unlock()

			// Broadcast audio URL to all clients
			s.broadcastMessage(gin.H{
				"type":     "audio",
				"audioUrl": fmt.Sprintf("/api/audio/%s", audioID),
				"agent":    agent.GetName(),
				"duration": audioDuration.Seconds(),
			})

			// Calculate total generation time for this message
			totalGenerationTime := time.Since(generationStart)

			// Log timing information
			log.Printf("Generation timing for %s: Response=%v, Audio=%v, Total=%v",
				agent.GetName(),
				responseGenerationTime,
				audioGenerationTime,
				totalGenerationTime)
			log.Printf("Is first message: %v", isfirstmessage)
			// Calculate delay
			// buffer := time.Duration(math.Min(audioDuration.Seconds()*0.2, 1.0)) * time.Second
			// remainingDelay := audioDuration + buffer
			remainingDelay := audioDuration
			if isfirstmessage {
				remainingDelay = remainingDelay - totalGenerationTime
				isfirstmessage = false
			}
			log.Printf("Remaining delay: %v", remainingDelay)
			log.Printf("Total generation time: %v", totalGenerationTime)
			// After sending audio URL, update timing information
			s.lastAudioStart = time.Now()
			s.requiredDelay = remainingDelay
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

func (s *Server) getGameScore(c *gin.Context) {
	agent1Name, agent2Name := s.GetOrderedAgentNames()
	c.JSON(http.StatusOK, gin.H{
		agent1Name: agent1Score,
		agent2Name: agent2Score,
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

// getAudioDuration calculates the duration of MP3 audio data by parsing MP3 frames
func getAudioDuration(audioData []byte) time.Duration {
	reader := bytes.NewReader(audioData)
	decoder := mp3.NewDecoder(reader)

	var duration time.Duration
	var frame mp3.Frame
	skipped := 0

	for {
		err := decoder.Decode(&frame, &skipped)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error decoding MP3 frame: %v", err)
			break
		}
		duration += frame.Duration()
	}

	return duration
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

// Function to decide player vote and calculate delta
func decidePlayerVote(agent1Support, agent2Support int, agent1Name, agent2Name string) (int, string) {
	// Calculate the delta
	delta := abs(agent1Support - agent2Support)

	// Determine the winner
	var winner string
	if agent1Support > agent2Support {
		winner = agent1Name
	} else if agent2Support > agent1Support {
		winner = agent2Name
	} else {
		winner = ""
	}

	return delta, winner
}

// Helper function to calculate absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
