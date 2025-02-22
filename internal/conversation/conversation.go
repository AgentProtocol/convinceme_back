package conversation

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/player"
	"github.com/neo/convinceme_backend/internal/types"
)

// ConversationConfig holds configuration for the conversation
type ConversationConfig struct {
	Topic           string
	MaxTurns        int
	TurnDelay       time.Duration
	ResponseStyle   types.ResponseStyle
	MaxTokens       int
	TemperatureHigh bool
}

// DefaultConfig returns a default configuration
func DefaultConfig() ConversationConfig {
	return ConversationConfig{
		Topic:           "Live radio podcast interview about AI advancements",
		MaxTurns:        5,
		TurnDelay:       500 * time.Millisecond,      // Reduced delay to 500 milliseconds
		ResponseStyle:   types.ResponseStyleHumorous, // Set to humorous for more emotional responses
		MaxTokens:       100,
		TemperatureHigh: true,
	}
}

// Conversation manages the dialogue between two agents
type Conversation struct {
	agent1       *agent.Agent
	agent2       *agent.Agent
	config       ConversationConfig
	inputHandler *player.InputHandler
	isActive     bool
	mu           sync.RWMutex
}

// NewConversation creates a new conversation between two agents
func NewConversation(agent1, agent2 *agent.Agent, config ConversationConfig, inputHandler *player.InputHandler) *Conversation {
	if !config.ResponseStyle.IsValid() {
		config.ResponseStyle = types.ResponseStyleCasual // fallback to casual if invalid
	}
	return &Conversation{
		agent1:       agent1,
		agent2:       agent2,
		config:       config,
		inputHandler: inputHandler,
	}
}

// Start begins the conversation between the agents
func (c *Conversation) Start(ctx context.Context) error {
	c.mu.Lock()
	c.isActive = true
	c.mu.Unlock()

	var lastMessage string
	interviewer := c.agent1
	guest := c.agent2
	currentAgent := interviewer
	otherAgent := guest

	fmt.Printf("Starting conversation on topic: %s\n", c.config.Topic)
	fmt.Printf("Style: %s\n", c.config.ResponseStyle)
	fmt.Printf("Between %s (Interviewer) and %s (Guest)\n\n", interviewer.GetName(), guest.GetName())

	stylePrompt := c.getPromptStyle()
	lastMessage = fmt.Sprintf("Let's start discussing about %s. %s", c.config.Topic, stylePrompt)

	for turn := 0; turn < c.config.MaxTurns; turn++ {
		response, err := currentAgent.GenerateResponse(ctx, c.config.Topic, lastMessage)
		if err != nil {
			return fmt.Errorf("failed to generate response: %v", err)
		}

		fmt.Printf("AGENT-%d: %s\n", getAgentNumber(currentAgent, interviewer), response)
		lastMessage = response

		// Generate audio
		audioData, err := currentAgent.GenerateAndStreamAudio(ctx, response)
		if err != nil {
			return fmt.Errorf("failed to generate audio: %v", err)
		}

		// Log the generated response and audio
		log.Printf("Generated response by %s: %s", currentAgent.GetName(), response)
		log.Printf("Generated audio for %s: %d bytes", currentAgent.GetName(), len(audioData))

		// Switch agents for the next turn
		currentAgent, otherAgent = otherAgent, currentAgent

		// Wait before next turn
		time.Sleep(c.config.TurnDelay)
	}

	c.mu.Lock()
	c.isActive = false
	c.mu.Unlock()

	return nil
}

// handlePlayerInterrupt processes a player interruption
func (c *Conversation) handlePlayerInterrupt(ctx context.Context, input player.PlayerInput, currentAgent *agent.Agent) error {
	// Create a prompt that includes the player's input
	prompt := fmt.Sprintf(`A player has just interrupted with: "%s"
Please acknowledge their input and incorporate it naturally into the conversation.
Be brief but engaging.`, input.Content)

	response, err := currentAgent.GenerateResponse(ctx, c.config.Topic, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate interrupt response: %v", err)
	}

	fmt.Printf("AGENT-%d (responding to player): %s\n", getAgentNumber(currentAgent, c.agent1), response)
	return nil
}

// playerInputProcessor implements the InputProcessor interface
type playerInputProcessor struct {
	conversation *Conversation
	interruptCh  chan player.PlayerInput
}

func (p *playerInputProcessor) ProcessInput(ctx context.Context, input player.PlayerInput) error {
	// Only process input if conversation is active
	p.conversation.mu.RLock()
	isActive := p.conversation.isActive
	p.conversation.mu.RUnlock()

	if !isActive {
		return nil
	}

	// Send input to interrupt channel
	select {
	case p.interruptCh <- input:
		return nil
	default:
		return fmt.Errorf("interrupt channel is full")
	}
}

// getPromptStyle returns the prompt modification based on response style
func (c *Conversation) getPromptStyle() string {
	switch c.config.ResponseStyle {
	case types.ResponseStyleFormal:
		return "Maintain a formal and professional tone."
	case types.ResponseStyleCasual:
		return "Keep the tone casual and friendly."
	case types.ResponseStyleTechnical:
		return "Use technical language and precise terminology."
	case types.ResponseStyleDebate:
		return "Use persuasive and argumentative language."
	case types.ResponseStyleHumorous:
		return "Keep the tone light and humorous."
	default:
		return "Keep the tone casual and friendly."
	}
}

// getAgentNumber returns 1 for agent1 and 2 for agent2
func getAgentNumber(current, agent1 *agent.Agent) int {
	if current == agent1 {
		return 1
	}
	return 2
}
