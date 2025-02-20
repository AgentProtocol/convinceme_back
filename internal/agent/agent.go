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
	"github.com/neo/convinceme_backend/internal/tools"
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
	tts    *audio.TTSService
	rag    *tools.ConversationRAG
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

	// Initialize RAG storage
	rag, err := tools.NewConversationRAG("data/conversations.db", apiKey)
	if err != nil {
		log.Printf("Warning: Failed to initialize RAG storage: %v", err)
	}

	return &Agent{
		config: config,
		llm:    llm,
		tts:    tts,
		rag:    rag,
	}, nil
}

// GenerateResponse generates a response based on the conversation history and topic
func (a *Agent) GenerateResponse(ctx context.Context, topic string, prompt string) (string, error) {
	// Add explicit instruction for 2 sentences
	prompt = fmt.Sprintf("%s\n\nIMPORTANT: Respond with EXACTLY 2 sentences, no more and no less.", prompt)

	response, err := llms.GenerateFromSinglePrompt(ctx, a.llm, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %v", err)
	}

	// Ensure response is exactly 2 sentences
	response = a.limitToTwoSentences(response)

	// Calculate conviction score based on response analysis
	convictionScore := a.calculateConvictionScore(response)

	// Store response in RAG
	memory := &tools.ConversationMemory{
		ID:         fmt.Sprintf("resp_%d_%s", time.Now().UnixNano(), a.config.Name),
		Topic:      topic,
		Timestamp:  time.Now(),
		AgentNames: []string{a.config.Name},
		Messages: []tools.Message{
			{
				AgentName: a.config.Name,
				Content:   response,
				Timestamp: time.Now(),
			},
		},
		ConvictionScore: convictionScore,
		Keywords:        a.extractKeywords(topic, response),
		Summary:         response,
	}

	// Convert memory to RAG request
	request := tools.RAGRequest{
		Action:       "store",
		Conversation: memory,
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal RAG request: %v", err)
	}

	if _, err := a.rag.Call(ctx, string(requestJSON)); err != nil {
		return "", fmt.Errorf("failed to store memory: %v", err)
	}

	return response, nil
}

// limitToTwoSentences ensures the response is exactly 2 sentences
func (a *Agent) limitToTwoSentences(text string) string {
	// Split into sentences (handling common sentence endings)
	sentences := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})

	// Clean sentences and remove empty ones
	var cleanSentences []string
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s != "" {
			cleanSentences = append(cleanSentences, s)
		}
	}

	// If we have less than 2 sentences, return as is
	if len(cleanSentences) <= 1 {
		return text
	}

	// Take exactly 2 sentences and restore punctuation
	result := cleanSentences[0] + "." + " " + cleanSentences[1] + "."
	return result
}

// calculateConvictionScore analyzes response content to determine conviction level
func (a *Agent) calculateConvictionScore(response string) float64 {
	// Initialize base score
	score := 0.5

	// Keywords indicating strong conviction
	strongConvictionPhrases := []string{
		// Certainty expressions
		"I am certain", "I strongly believe", "clearly", "definitely", "without doubt",
		"must", "always", "never", "absolutely", "undoubtedly", "unquestionably",
		"I am convinced", "I am sure", "I am positive", "I am confident",
		"it is evident", "it is clear", "it is obvious", "it is undeniable",
		"there is no doubt", "without question", "beyond doubt", "indisputably",
		"categorically", "unmistakably", "undeniably", "incontrovertibly",

		// Strong assertions
		"I know for a fact", "I can assure you", "I guarantee", "I am adamant",
		"it is essential", "it is crucial", "it is vital", "it is imperative",
		"necessarily", "invariably", "consistently", "inevitably", "indubitably",
		"fundamentally", "inherently", "intrinsically", "by definition",

		// Absolute statements
		"in every case", "without exception", "in all instances", "every time",
		"under all circumstances", "in all cases", "universally", "eternally",
		"permanently", "conclusively", "decisively", "definitively", "irrefutably",

		// Emphatic expressions
		"indeed", "absolutely true", "precisely", "exactly", "certainly",
		"emphatically", "unequivocally", "resolutely", "firmly", "steadfastly",
		"without hesitation", "without reservation", "beyond any doubt",
		"beyond question", "beyond dispute", "proven fact", "established fact",
	}

	// Keywords indicating uncertainty
	uncertaintyPhrases := []string{
		// Doubt expressions
		"perhaps", "maybe", "might", "could", "possibly", "potentially",
		"I think", "seems", "appears", "uncertain", "not sure", "unsure",
		"I guess", "I suppose", "I assume", "presumably", "probably",
		"conceivably", "feasibly", "perchance", "per chance", "plausibly",

		// Hesitation markers
		"somewhat", "sort of", "kind of", "in a way", "to some extent",
		"to a degree", "more or less", "rather", "fairly", "relatively",
		"comparatively", "approximately", "roughly", "about", "around",
		"tentatively", "provisionally", "conditionally",

		// Qualifying statements
		"it depends", "it varies", "it may be", "it could be", "it might be",
		"as far as I know", "to my knowledge", "from what I understand",
		"if I'm not mistaken", "if I remember correctly", "correct me if I'm wrong",
		"I may be wrong", "I could be wrong", "I might be mistaken",

		// Ambiguity markers
		"unclear", "ambiguous", "vague", "debatable", "questionable",
		"disputable", "controversial", "arguable", "contestable",
		"open to interpretation", "open to debate", "open to question",
		"not necessarily", "not always", "not entirely", "not exactly",

		// Hedging expressions
		"generally", "typically", "usually", "often", "sometimes",
		"occasionally", "frequently", "rarely", "seldom", "hardly",
		"barely", "scarcely", "in most cases", "in some cases",
		"in certain cases", "under certain circumstances",
		"to some degree", "more or less", "give or take",

		// Speculative language
		"hypothetically", "theoretically", "in theory", "supposedly",
		"allegedly", "reportedly", "apparently", "ostensibly",
		"reputedly", "purportedly", "it is said", "it is believed",
		"it is thought", "it is considered", "it is assumed",
	}

	responseLower := strings.ToLower(response)

	// Adjust score based on conviction phrases
	for _, phrase := range strongConvictionPhrases {
		if strings.Contains(responseLower, strings.ToLower(phrase)) {
			score += 0.1
		}
	}

	// Adjust score based on uncertainty phrases
	for _, phrase := range uncertaintyPhrases {
		if strings.Contains(responseLower, strings.ToLower(phrase)) {
			score -= 0.1
		}
	}

	// Ensure score stays within 0-1 range
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// extractKeywords extracts relevant keywords from topic and response
func (a *Agent) extractKeywords(topic string, response string) []string {
	keywords := make(map[string]bool)

	// Add topic as a keyword
	keywords[topic] = true

	// Analyze response sentiment/tone
	responseLower := strings.ToLower(response)

	// Add tone-based keywords
	if strings.Contains(responseLower, "think") || strings.Contains(responseLower, "believe") || strings.Contains(responseLower, "consider") {
		keywords["Reflective"] = true
	}
	if strings.Contains(responseLower, "must") || strings.Contains(responseLower, "should") || strings.Contains(responseLower, "need to") {
		keywords["Assertive"] = true
	}
	if strings.Contains(responseLower, "inspire") || strings.Contains(responseLower, "encourage") || strings.Contains(responseLower, "potential") {
		keywords["Inspirational"] = true
	}
	if strings.Contains(responseLower, "question") || strings.Contains(responseLower, "wonder") || strings.Contains(responseLower, "curious") {
		keywords["Inquisitive"] = true
	}

	// Convert map to slice
	var result []string
	for k := range keywords {
		result = append(result, k)
	}

	return result
}

// GetName returns the agent's name
func (a *Agent) GetName() string {
	return a.config.Name
}

// GetRole returns the agent's role
func (a *Agent) GetRole() string {
	return a.config.Role
}

// ClearMemory clears the agent's memory by closing and reinitializing RAG
func (a *Agent) ClearMemory() {
	if a.rag != nil {
		a.rag.Close()
		// Reinitialize RAG
		if rag, err := tools.NewConversationRAG("data/conversations.db", os.Getenv("OPENAI_API_KEY")); err == nil {
			a.rag = rag
		}
	}
}

// SummarizeMemory returns a summary of the agent's memory from RAG
func (a *Agent) SummarizeMemory(ctx context.Context) (string, error) {
	if a.rag == nil {
		return "No memories to summarize", nil
	}

	request := tools.RAGRequest{
		Action: "query",
		Query: &tools.MemoryQuery{
			Topic:  "all",
			Agents: []string{a.config.Name},
			Limit:  10,
		},
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal query request: %v", err)
	}

	result, err := a.rag.Call(ctx, string(requestJSON))
	if err != nil {
		return "", fmt.Errorf("failed to query memories: %v", err)
	}

	var memories []tools.ConversationMemory
	if err := json.Unmarshal([]byte(result), &memories); err != nil {
		return "", fmt.Errorf("failed to unmarshal memories: %v", err)
	}

	if len(memories) == 0 {
		return "No memories found", nil
	}

	// Create a summary prompt from the memories
	var memoryText string
	for _, memory := range memories {
		for _, msg := range memory.Messages {
			memoryText += fmt.Sprintf("%s: %s\n", msg.AgentName, msg.Content)
		}
	}

	summaryPrompt := fmt.Sprintf("Summarize this conversation history in 2-3 sentences:\n%s", memoryText)
	summary, err := llms.GenerateFromSinglePrompt(ctx, a.llm, summaryPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate memory summary: %v", err)
	}

	return summary, nil
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
