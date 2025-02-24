package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// Tool is a tool for the llm agent to interact with different applications.
type Tool interface {
	Name() string
	Description() string
	Call(ctx context.Context, input string) (string, error)
}

// ConvictionMetrics represents the measurement of conviction levels
type ConvictionMetrics struct {
	Agent1Score     float64 `json:"agent1_score"`     // 0-1 conviction score for agent1
	Agent2Score     float64 `json:"agent2_score"`     // 0-1 conviction score for agent2
	Agent1Name      string  `json:"agent1_name"`      // Name of agent1
    Agent2Name      string  `json:"agent2_name"`      // Name of agent2
	OverallTension  float64 `json:"overall_tension"`  // 0-1 tension level between agents
	DominantAgent   string  `json:"dominant_agent"`   // Name of the agent showing more conviction
	AnalysisSummary string  `json:"analysis_summary"` // Brief analysis of the conviction dynamics
}

// ConvictionContext represents the input context for conviction analysis
type ConvictionContext struct {
	Agent1Name      string             `json:"agent1_name"`
	Agent2Name      string             `json:"agent2_name"`
	Conversation    []ConversationEntry `json:"conversation"`
	UserArgument    string             `json:"user_argument"`
	UserScore       *ArgumentScore     `json:"user_score"`
	InitialMetrics  ConvictionMetrics  `json:"initial_metrics"`
}

// ArgumentScore represents the scoring metrics for an argument
type ArgumentScore struct {
	Strength    int     `json:"strength"`    // Support for position (0-100)
	Relevance   int     `json:"relevance"`   // Relevance to discussion (0-100)
	Logic       int     `json:"logic"`       // Logical structure (0-100)
	Truth       int     `json:"truth"`       // Factual accuracy (0-100)
	Humor       int     `json:"humor"`       // Entertainment value (0-100)
	Average     float64 `json:"average"`     // Average of all scores
	Explanation string  `json:"explanation"` // Brief explanation
}

// ConversationEntry represents a single message in the conversation
type ConversationEntry struct {
	Speaker string `json:"speaker"`
	Message string `json:"message"`
}

// ConvictionJudge is a tool that measures conviction levels between AI agents
type ConvictionJudge struct {
	client       *openai.Client
	systemPrompt string
}

var _ Tool = (*ConvictionJudge)(nil) // Verify ConvictionJudge implements Tool interface

// NewConvictionJudge creates a new instance of the ConvictionJudge tool
func NewConvictionJudge(apiKey string) (*ConvictionJudge, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(apiKey)

	systemPrompt := `You are an expert debate judge and behavioral analyst specializing in measuring conviction levels in conversations.
Your role is to:
1. Analyze the conversation between two AI agents
2. Measure their conviction levels on a scale of 0-1 (0 being completely unconvinced, 1 being absolutely convinced)
3. Assess the overall tension in the debate
4. Identify the dominant speaker
5. Provide a brief analysis of the conviction dynamics

Focus on:
- Strength of arguments
- Consistency in positions
- Emotional investment in topics
- Use of persuasive language
- Response to counterarguments
- Overall debate engagement

Your response MUST be a valid JSON object with the following structure:
{
    "agent1_score": float,     // 0-1 conviction score
    "agent2_score": float,     // 0-1 conviction score
    "overall_tension": float,  // 0-1 tension level
    "dominant_agent": string,  // name of dominant speaker
    "analysis_summary": string // brief analysis
}

Be precise, objective, and analytical in your measurements.`

	return &ConvictionJudge{
		client:       client,
		systemPrompt: systemPrompt,
	}, nil
}

// Name returns the name of the tool
func (c *ConvictionJudge) Name() string {
	return "conviction_judge"
}

// Description returns a description of the tool
func (c *ConvictionJudge) Description() string {
	return `Analyzes the conviction levels and debate dynamics between two AI agents in a conversation.
Input should be a JSON string containing the conversation history with format:
{
    "agent1_name": "string",
    "agent2_name": "string",
    "conversation": [
        {"speaker": "agent_name", "message": "string"},
        ...
    ]
}`
}

// Call analyzes the conversation and returns conviction metrics
func (c *ConvictionJudge) Call(ctx context.Context, input string) (string, error) {
	// Parse input JSON
	var conversationData map[string]interface{}
	if err := json.Unmarshal([]byte(input), &conversationData); err != nil {
		return "", fmt.Errorf("invalid input format: %v", err)
	}

	// Create analysis request
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: c.systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: fmt.Sprintf("Analyze this conversation and provide conviction metrics: %s", input),
		},
	}

	// Get analysis from OpenAI
	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       "gpt-4o-mini",
			Messages:    messages,
			Temperature: 0.2, // Low temperature for more consistent analysis
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to analyze conversation: %v", err)
	}

	// Parse the response into metrics to validate format
	metrics := ConvictionMetrics{}
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &metrics); err != nil {
		return "", fmt.Errorf("invalid metrics format from OpenAI: %v", err)
	}

	// Return the raw response as string
	return resp.Choices[0].Message.Content, nil
}
