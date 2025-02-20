package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/neo/convinceme_backend/internal/tools"
)

// RAGIntegration handles integration with the conversation RAG system
type RAGIntegration struct {
	rag *tools.ConversationRAG
}

// NewRAGIntegration creates a new RAG integration
func NewRAGIntegration() (*RAGIntegration, error) {
	// Get OpenAI API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	// Create data directory if it doesn't exist
	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	// Initialize RAG with SQLite database
	dbPath := filepath.Join(dataDir, "conversations.db")
	rag, err := tools.NewConversationRAG(dbPath, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create RAG: %v", err)
	}

	return &RAGIntegration{
		rag: rag,
	}, nil
}

// StoreConversation stores a conversation in the RAG database
func (r *RAGIntegration) StoreConversation(ctx context.Context, conv *Conversation, analysis *ConvictionAnalysis) error {
	// Convert conversation messages to RAG format
	messages := make([]tools.Message, 0, len(conv.messages))
	for _, msg := range conv.messages {
		agentName := ""
		if msg.Agent == conv.agent1 {
			agentName = conv.agent1.GetName()
		} else if msg.Agent == conv.agent2 {
			agentName = conv.agent2.GetName()
		}

		messages = append(messages, tools.Message{
			AgentName: agentName,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		})
	}

	// Calculate overall conviction score
	overallScore := 0.0
	if analysis != nil {
		overallScore = (analysis.InterviewerMetrics.Overall + analysis.GuestMetrics.Overall) / 2.0
	}

	// Extract keywords from messages
	keywords := extractKeywords(conv.config.Topic, messages)

	// Generate summary
	summary := generateSummary(conv.config.Topic, conv.agent1.GetName(), conv.agent2.GetName(), messages, overallScore)

	// Create memory entry
	memory := &tools.ConversationMemory{
		ID:              uuid.New().String(),
		Topic:           conv.config.Topic,
		Timestamp:       time.Now(),
		AgentNames:      []string{conv.agent1.GetName(), conv.agent2.GetName()},
		Messages:        messages,
		ConvictionScore: overallScore,
		Keywords:        keywords,
		Summary:         summary,
	}

	// Create store request
	request := tools.RAGRequest{
		Action:       "store",
		Conversation: memory,
	}

	// Convert request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal store request: %v", err)
	}

	// Store in RAG database
	result, err := r.rag.Call(ctx, string(requestJSON))
	if err != nil {
		return fmt.Errorf("failed to store conversation: %v", err)
	}

	if result != "memory stored successfully" {
		return fmt.Errorf("failed to store conversation: %s", result)
	}

	return nil
}

// extractKeywords extracts relevant keywords from the conversation
func extractKeywords(topic string, messages []tools.Message) []string {
	keywords := make(map[string]struct{})

	// Add topic words as keywords
	topicWords := strings.Fields(topic)
	for _, word := range topicWords {
		word = strings.ToLower(strings.Trim(word, ".,!?"))
		if len(word) > 3 { // Skip short words
			keywords[word] = struct{}{}
		}
	}

	// Extract keywords from messages
	for _, msg := range messages {
		words := strings.Fields(msg.Content)
		for _, word := range words {
			word = strings.ToLower(strings.Trim(word, ".,!?"))
			if len(word) > 3 { // Skip short words
				keywords[word] = struct{}{}
			}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(keywords))
	for kw := range keywords {
		result = append(result, kw)
	}

	// Sort keywords for consistency
	sort.Strings(result)
	return result
}

// generateSummary generates a summary of the conversation
func generateSummary(topic, agent1Name, agent2Name string, messages []tools.Message, convictionScore float64) string {
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("Conversation about %s between ", topic))
	summary.WriteString(fmt.Sprintf("%s and %s", agent1Name, agent2Name))
	summary.WriteString(fmt.Sprintf(". Contains %d messages", len(messages)))

	if convictionScore > 0 {
		summary.WriteString(fmt.Sprintf(" with conviction score %.2f", convictionScore))
	}

	return summary.String()
}

// QueryRelevantHistory queries the RAG database for relevant conversation history
func (r *RAGIntegration) QueryRelevantHistory(ctx context.Context, topic string, agentNames []string, limit int) ([]tools.ConversationMemory, error) {
	// Create query request
	request := tools.RAGRequest{
		Action: "query",
		Query: &tools.MemoryQuery{
			Topic:  topic,
			Agents: agentNames,
			Limit:  limit,
		},
	}

	// Convert request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query request: %v", err)
	}

	// Query RAG database
	result, err := r.rag.Call(ctx, string(requestJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %v", err)
	}

	// Parse results
	var memories []tools.ConversationMemory
	if err := json.Unmarshal([]byte(result), &memories); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query results: %v", err)
	}

	return memories, nil
}

// GenerateContextFromHistory generates a context string from relevant conversation history
func (r *RAGIntegration) GenerateContextFromHistory(ctx context.Context, topic string, agentNames []string) (string, error) {
	// Query relevant memories
	memories, err := r.QueryRelevantHistory(ctx, topic, agentNames, 3) // Get top 3 relevant conversations
	if err != nil {
		return "", fmt.Errorf("failed to query relevant history: %v", err)
	}

	if len(memories) == 0 {
		return "", nil // No relevant history found
	}

	// Generate context string
	var context string
	context = "Based on previous conversations:\n\n"

	for i, memory := range memories {
		context += fmt.Sprintf("%d. %s\n", i+1, memory.Summary)
		context += "Key points:\n"

		// Add a few representative messages
		messageCount := 0
		for _, msg := range memory.Messages {
			if messageCount >= 2 { // Limit to 2 messages per conversation
				break
			}
			context += fmt.Sprintf("- %s: %s\n", msg.AgentName, msg.Content)
			messageCount++
		}
		context += "\n"
	}

	return context, nil
}

// Close closes the RAG database connection
func (r *RAGIntegration) Close() error {
	if r.rag != nil {
		return r.rag.Close()
	}
	return nil
}
