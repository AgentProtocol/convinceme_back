package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/neo/convinceme_backend/internal/audio"
	"github.com/neo/convinceme_backend/internal/types"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	maxMemorySize    = 1000 // Maximum number of memory entries to store
	defaultMaxTokens = 150
	minTemperature   = 0.1
	maxTemperature   = 2.0
	minTopP          = 0.1
	maxTopP          = 1.0
)

// AgentConfig holds configuration for an agent
type AgentConfig struct {
	Name        string
	Role        string
	Voice       types.Voice
	Temperature float32
	MaxTokens   int
	TopP        float32
}

// MemoryEntry represents a single memory entry with context
type MemoryEntry struct {
	Message   string    `json:"message"`
	Role      string    `json:"role"`
	Timestamp time.Time `json:"timestamp"`
	Context   struct {
		Emotion    string   `json:"emotion"`
		Topics     []string `json:"topics"`
		Importance float32  `json:"importance"`
	} `json:"context"`
}

// Agent represents an AI agent that can engage in conversation
type Agent struct {
	config AgentConfig
	llm    llms.Model
	memory []MemoryEntry
	tts    *audio.TTSService
}

// validateConfig validates the agent configuration
func validateConfig(config *AgentConfig) error {
	if config.Name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}
	if config.Role == "" {
		return fmt.Errorf("agent role cannot be empty")
	}
	if config.Temperature < minTemperature || config.Temperature > maxTemperature {
		return fmt.Errorf("temperature must be between %.1f and %.1f", minTemperature, maxTemperature)
	}
	if config.TopP < minTopP || config.TopP > maxTopP {
		return fmt.Errorf("topP must be between %.1f and %.1f", minTopP, maxTopP)
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = defaultMaxTokens
	}
	return nil
}

// NewAgent creates a new AI agent with the specified configuration
func NewAgent(apiKey string, config AgentConfig) (*Agent, error) {
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}

	if !config.Voice.IsValid() {
		config.Voice = types.VoiceAlloy // fallback to alloy if invalid
	}

	// Configure OpenAI client options
	opts := []openai.Option{
		openai.WithToken(apiKey),
		openai.WithModel("gpt-4-turbo-preview"),
	}

	// Create LLM client with configuration
	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM: %v", err)
	}

	tts, err := audio.NewTTSService(apiKey, config.Voice.String())
	if err != nil {
		return nil, fmt.Errorf("failed to create TTS service: %v", err)
	}

	return &Agent{
		config: config,
		llm:    llm,
		memory: make([]MemoryEntry, 0),
		tts:    tts,
	}, nil
}

// GenerateResponse generates a response based on the conversation history and topic
func (a *Agent) GenerateResponse(ctx context.Context, topic string, previousMessage string) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("context cancelled: %v", ctx.Err())
	default:
	}

	// Create context from recent memory with more details
	recentContext := a.buildContextFromMemory(10) // Increased from 5 to 10

	prompt := fmt.Sprintf(`You are %s, playing the role of %s. 
Current conversation context:
%s

Topic: %s
Previous message: %s

IMPORTANT INSTRUCTIONS:
1. NEVER start the conversation as if it's new - always acknowledge the ongoing discussion
2. Maintain your character's personality consistently
3. Reference previous points from the conversation
4. Keep responses natural and engaging (2-3 sentences)
5. Stay focused on the current topic while allowing natural transitions
6. Show emotional intelligence and appropriate reactions

Your character traits:
- Name: %s
- Role: %s
- Speaking style: Professional but natural
- Personality: Maintain consistent views and knowledge

Remember: This is an ongoing conversation - do not use generic greetings or act like it's just starting.
Respond in a way that shows you're actively engaged in the current discussion.`,
		a.config.Name, a.config.Role,
		recentContext, topic, previousMessage,
		a.config.Name, a.config.Role)

	completion, err := llms.GenerateFromSinglePrompt(ctx, a.llm, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %v", err)
	}

	// Analyze response for context with proper error handling
	emotionPrompt := fmt.Sprintf("Analyze this response and return one word describing the emotional tone: %s", completion)
	emotion, err := llms.GenerateFromSinglePrompt(ctx, a.llm, emotionPrompt)
	if err != nil {
		log.Printf("Failed to analyze emotion: %v", err)
		emotion = "neutral" // fallback emotion
	}

	// Create memory entry
	entry := MemoryEntry{
		Message:   completion,
		Role:      a.config.Role,
		Timestamp: time.Now(),
	}
	entry.Context.Emotion = emotion
	entry.Context.Topics = []string{topic}
	entry.Context.Importance = 1.0

	// Store in memory with size limit
	a.addMemoryEntry(entry)

	return completion, nil
}

// addMemoryEntry adds a new memory entry while maintaining the size limit
func (a *Agent) addMemoryEntry(entry MemoryEntry) {
	a.memory = append(a.memory, entry)
	if len(a.memory) > maxMemorySize {
		// Remove oldest entries when limit is reached
		a.memory = a.memory[len(a.memory)-maxMemorySize:]
	}
}

// buildContextFromMemory creates a context summary from recent memory entries
func (a *Agent) buildContextFromMemory(n int) string {
	if len(a.memory) == 0 {
		return "No previous context"
	}

	start := len(a.memory) - n
	if start < 0 {
		start = 0
	}

	var context string
	for _, entry := range a.memory[start:] {
		context += fmt.Sprintf("- %s (Emotion: %s, Topics: %v)\n",
			entry.Message, entry.Context.Emotion, entry.Context.Topics)
	}

	return context
}

// getCreativityLevel returns a description of the creativity level based on temperature
func getCreativityLevel(temp float32) string {
	if temp < 0.5 {
		return "conservative"
	} else if temp < 0.8 {
		return "balanced"
	}
	return "creative"
}

// GetName returns the agent's name
func (a *Agent) GetName() string {
	return a.config.Name
}

// GetRole returns the agent's role
func (a *Agent) GetRole() string {
	return a.config.Role
}

// GetMemory returns the agent's conversation memory
func (a *Agent) GetMemory() []MemoryEntry {
	return a.memory
}

// GenerateAndStreamAudio generates audio from text and returns the audio data
func (a *Agent) GenerateAndStreamAudio(ctx context.Context, text string) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled: %v", ctx.Err())
	default:
	}

	audioData, err := a.tts.GenerateAudio(ctx, text)
	if err != nil {
		// Retry once on failure
		log.Printf("First attempt to generate audio failed: %v. Retrying...", err)
		audioData, err = a.tts.GenerateAudio(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate audio after retry: %v", err)
		}
	}

	log.Printf("Generated audio for %s: %d bytes", a.config.Name, len(audioData))
	return audioData, nil
}

// ClearMemory clears the agent's memory
func (a *Agent) ClearMemory() {
	a.memory = make([]MemoryEntry, 0)
}

// SummarizeMemory returns a summary of the agent's memory
func (a *Agent) SummarizeMemory(ctx context.Context) (string, error) {
	if len(a.memory) == 0 {
		return "No memories to summarize", nil
	}

	summaryPrompt := fmt.Sprintf("Summarize this conversation history in 2-3 sentences:\n%s",
		a.buildContextFromMemory(len(a.memory)))

	summary, err := llms.GenerateFromSinglePrompt(ctx, a.llm, summaryPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate memory summary: %v", err)
	}

	return summary, nil
}
