package agent

import (
	"context"
	"fmt"
	"io"

	"github.com/neo/convinceme_backend/internal/audio"
	"github.com/neo/convinceme_backend/internal/types"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
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

// Agent represents an AI agent that can engage in conversation
type Agent struct {
	config AgentConfig
	llm    llms.LLM
	memory []string
	tts    *audio.TTSService
}

// NewAgent creates a new AI agent with the specified configuration
func NewAgent(apiKey string, config AgentConfig) (*Agent, error) {
	if !config.Voice.IsValid() {
		config.Voice = types.VoiceAlloy // fallback to alloy if invalid
	}

	// Configure OpenAI client options
	opts := []openai.Option{
		openai.WithToken(apiKey),
		openai.WithModel("gpt-3.5-turbo"),
	}

	// Create LLM client with configuration
	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM: %v", err)
	}

	tts := audio.NewTTSService(apiKey, config.Voice.String())

	return &Agent{
		config: config,
		llm:    llm,
		memory: make([]string, 0),
		tts:    tts,
	}, nil
}

// GenerateResponse generates a response based on the conversation history and topic
func (a *Agent) GenerateResponse(ctx context.Context, topic string, previousMessage string) (string, error) {
	prompt := fmt.Sprintf(`You are %s with the role of %s. The topic of discussion is: %s. 
Previous message: %s. 
Generate a very brief response (1-2 short sentences max). Be concise and direct. 
Temperature: %.1f, Creativity level: %s:`,
		a.config.Name, a.config.Role, topic, previousMessage,
		a.config.Temperature,
		getCreativityLevel(a.config.Temperature))

	completion, err := a.llm.Call(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %v", err)
	}

	response := completion
	a.memory = append(a.memory, response)

	// Generate speech for the response
	//audioPath := filepath.Join("output", fmt.Sprintf("%s_response_%d.mp3", a.config.Name, len(a.memory)))
	//if err := a.tts.TextToSpeech(ctx, response, audioPath); err != nil {
	//	return "", fmt.Errorf("failed to generate speech: %v", err)
	//}

	return response, nil
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

// GetMemory returns the agent's conversation memory
func (a *Agent) GetMemory() []string {
	return a.memory
}

// GenerateAndStreamAudio generates audio from text and streams it using the provided writer
func (a *Agent) GenerateAndStreamAudio(ctx context.Context, text string, writer io.Writer) error {
	return a.tts.GenerateAndStream(ctx, text, writer)
}
