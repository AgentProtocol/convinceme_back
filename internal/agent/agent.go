package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/neo/convinceme_backend/internal/audio"
	"github.com/neo/convinceme_backend/internal/types"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

// AgentConfig holds configuration for an agent
type AgentConfig struct {
	Name                string
	Role                string
	SystemPrompt        string
	DebatePosition      string
	ExpertiseArea       string
	KeyArguments        []string
	Voice               types.Voice
	Temperature         float32
	MaxCompletionTokens int
	TopP                float32
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
	llm    llms.LLM
	memory []MemoryEntry
	tts    *audio.TTSService
}

// NewAgent creates a new AI agent with the specified configuration
func NewAgent(openAIKey string, config AgentConfig) (*Agent, error) {
	if !config.Voice.IsValid() {
		config.Voice = types.VoiceMark // fallback to alloy if invalid
	}

	// Configure OpenAI client options
	opts := []openai.Option{
		openai.WithToken(openAIKey),
		openai.WithModel("gpt-4o-mini"),
	}

	// Create LLM client with configuration
	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM: %v", err)
	}

	// Create TTS service - API keys are loaded from environment variables
	tts, err := audio.NewTTSService(config.Voice.String())
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
	// Create context from recent memory
	recentContext := a.buildContextFromMemory(5) // Get context from last 5 interactions

	prompt := fmt.Sprintf(`You are %s with the role of %s. 
Recent conversation context: %s
Current topic of discussion: %s
Previous message: %s

Generate a response that:
1. Shows understanding of the conversation context
2. Maintains natural flow
3. Is brief (1-2 short sentences)
4. Shows appropriate emotional response
5. Stays relevant to the topic while allowing for natural topic transitions

Temperature: %.1f, Creativity level: %s

WHEN RESPONDING:	
   - Use straightforward language to explain your points
   - Maintain a friendly and engaging tone
   - If the user asks a question, answer it directly with specific focus on the topic and briefly
   - Use wit and humor to undermine the other agent's arguments

`,
		a.config.Name, a.config.Role, recentContext, topic, previousMessage,
		a.config.Temperature, getCreativityLevel(a.config.Temperature))

	completion, err := a.llm.Call(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %v", err)
	}

	// Analyze response for context
	emotionPrompt := fmt.Sprintf("Analyze this response and return one word describing the emotional tone: %s", completion)
	emotion, _ := a.llm.Call(ctx, emotionPrompt)

	// Create memory entry
	entry := MemoryEntry{
		Message:   completion,
		Role:      a.config.Role,
		Timestamp: time.Now(),
	}
	entry.Context.Emotion = emotion
	entry.Context.Topics = []string{topic}
	entry.Context.Importance = 1.0 // Can be adjusted based on content analysis

	// Store in memory
	a.memory = append(a.memory, entry)

	// Log the generated response
	log.Printf("Generated response by %s: %s", a.config.Name, completion)

	return completion, nil
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
	audioData, err := a.tts.GenerateAudio(ctx, text)
	if err != nil {
		return nil, err
	}

	return audioData, nil
}

// LoadAgentConfig loads an agent configuration from a JSON file
func LoadAgentConfig(configPath string) (AgentConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return AgentConfig{}, err
	}

	var config AgentConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return AgentConfig{}, err
	}

	return config, nil
}

// Add a new method to check if a message is directed to this agent
func (a *Agent) IsAddressed(message string) bool {
	// Check for common name variations
	nameLower := strings.ToLower(a.config.Name)
	messageLower := strings.ToLower(message)

	return strings.Contains(messageLower, nameLower) ||
		strings.Contains(messageLower, strings.ToLower(strings.Split(a.config.Name, " ")[0])) // First name check
}
