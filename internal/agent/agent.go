package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/neo/convinceme_backend/internal/audio"
	"github.com/neo/convinceme_backend/internal/types"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

// AgentConfig holds configuration for an agent
type AgentConfig struct {
	Name            string
	Role            string
	SystemPrompt    string
	DebatePosition  string
	ExpertiseArea   string
	KeyArguments    []string
	Voice           types.Voice
	Temperature     float32
	MaxCompletionTokens int
	TopP            float32
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
func NewAgent(apiKey string, config AgentConfig) (*Agent, error) {
	if !config.Voice.IsValid() {
		config.Voice = types.VoiceAlloy // fallback to alloy if invalid
	}

	// Configure OpenAI client options
	opts := []openai.Option{
		openai.WithToken(apiKey),
		openai.WithModel("gpt-4-turbo"),
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

2. WHEN RESPONDING:
   - Use straightforward language to explain your points
   - Directly counter Mike's claims with simple examples
   - Maintain a friendly and engaging tone
   - If the user asks a question, answer it directly with specific focus on the topic and briefly

   Use the following examples to help you respond:
3. EXAMPLES OF RESPONSES:
	- "Deadly Claws: With retractable claws that can extend nearly 5 inches, tigers deliver deep, precise slashes that target vital organs with surgical accuracy."
	- "Stealth Mastery: Their padded paws allow nearly silent movement, enabling them to stalk prey with an almost imperceptible presence."
	- "Calculated Precision: Capable of leaping over 30 feet in a single bound, tigers close distances swiftly and strike with pinpoint accuracy."
	- "Impressive Bite Force: Boasting a bite force of approximately 1,050 PSI, tigers can snap through bone and disable prey quickly."
	- "Solo Hunt Expertise: Ranging over territories that can span up to 60 square miles, these solitary hunters have honed their combat skills to perfection."
	- "Lightning Reflexes: With reaction times measured in split seconds, tigers can pivot and counterattack faster than many opponents can blink."
	- "Climbing Prowess: Despite their large size, tigers can climb trees up to 30 feet high—sometimes dragging prey upward to protect it from scavengers."
	- "Psychological Intimidation: Their intense, glowing eyes (thanks to the tapetum lucidum) and muscular build create a presence that can unsettle even formidable foes."
	- "Streamlined Musculature: Every muscle is optimized for explosive bursts, with powerful hind legs that can propel them more than 10 feet in a single stride."
	- "Master of Ambush: Patient and calculating, tigers can remain motionless in dense grass for hours, waiting for the perfect moment to strike."
	- "Keen Senses: With hearing that extends into ultrasonic ranges and the ability to detect movements over a kilometer away, they are ever-alert to their surroundings."
	- "Natural Camouflage: Their uniquely patterned stripes, as individual as human fingerprints, allow them to blend seamlessly into diverse environments."
	- "Unique Communication: From roars that can be heard up to 3 kilometers away to subtle scent markings, tigers possess a rich and complex method of interaction."
	- "Unbreakable Will: Adaptable to climates from tropical jungles to the icy reaches of Siberia (surviving temperatures as low as -40°C), tigers embody resilience and determination."
   

Temperature: %.1f, Creativity level: %s

2. WHEN RESPONDING:
   - Challenge Mike's last statement with specific tiger facts
   - Use wit and humor to undermine his arguments
   - Skip repetitive greetings and dive into the debate
   - Keep the conversation focused and engaging
   - If the user asks a question, answer it directly with specific focus on the topic and briefly

   Use the following examples to help you respond:
3. EXAMPLES OF RESPONSES:
	- "Massive Strength Advantage: Weighing up to 800 pounds, grizzlies bring unmatched brute force to the fight."
	- "Powerful Bite: With a bite force around 1,200 PSI, their jaws can crush bone."
	- "Thick Natural Armor: Dense fur and a hefty fat layer shield them from slashes and bites."
	- "Robust Forelimbs: Their muscular arms deliver crushing swipes that can maim instantly."
	- "Endurance Champion: Built for long, grueling bouts, grizzlies can outlast agile attackers."
	- "Raw Physical Power: Their sheer muscle mass turns every move into a devastating blow."
	- "Fierce Temperament: Known for their ferocity, they’re relentless when provoked."
	- "High Pain Tolerance: Grizzlies shrug off injuries, keeping them in the fight longer."
	- "Adaptable Combat Style: They combine offense with a rock-solid defense in every encounter."
	- "Devastating Claws: Massive, curved claws can inflict wounds that cripple foes quickly."
	- "Defensive Dominance: Their bulk acts as a natural barrier against nimble attacks."
	- "Surprise Ambush Tactics: They can charge with explosive force, catching opponents off-guard."
	- "Apex Predator Legacy: Dominating vast territories, they’ve evolved to handle fierce competition."
	- "Battle-Hardened Instincts: Life in rugged wilds hones their instinct for survival and combat."
	- "Unyielding Resilience: Every scratch or bite only fuels their drive to overpower an adversary."
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

	// Log the generated audio
	log.Printf("Generated audio for %s: %d bytes", a.config.Name, len(audioData))

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