package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tmc/langchaingo/callbacks"
)

// ConversationMemory represents a single conversation entry in the RAG database
type ConversationMemory struct {
	ID              string    `json:"id"`
	Topic           string    `json:"topic"`
	Timestamp       time.Time `json:"timestamp"`
	AgentNames      []string  `json:"agent_names"`
	Messages        []Message `json:"messages"`
	ConvictionScore float64   `json:"conviction_score"`
	Keywords        []string  `json:"keywords"`
	Summary         string    `json:"summary"`
}

// Message represents a single message in the conversation memory
type Message struct {
	AgentName string    `json:"agent_name"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// MemoryQuery represents a query to search the conversation memory
type MemoryQuery struct {
	Topic    string   `json:"topic"`
	Keywords []string `json:"keywords"`
	Agents   []string `json:"agents,omitempty"`
	Limit    int      `json:"limit,omitempty"`
}

// ConversationRAG is a tool that provides RAG-based conversation memory
type ConversationRAG struct {
	CallbacksHandler callbacks.Handler
	storage          *RAGStorage
}

var _ Tool = (*ConversationRAG)(nil)

// Description returns a string describing the conversation RAG tool
func (c *ConversationRAG) Description() string {
	return `Useful for storing and retrieving conversation history using RAG (Retrieval-Augmented Generation).
	The input should be a JSON string containing either a store request or a query request.
	For storing: {"action": "store", "conversation": {...}}
	For querying: {"action": "query", "query": {...}}`
}

// Name returns the name of the tool
func (c *ConversationRAG) Name() string {
	return "conversation_rag"
}

type RAGRequest struct {
	Action       string              `json:"action"` // "store" or "query"
	Query        *MemoryQuery        `json:"query,omitempty"`
	Conversation *ConversationMemory `json:"conversation,omitempty"`
}

// NewConversationRAG creates a new ConversationRAG instance
func NewConversationRAG(dbPath string, apiKey string) (*ConversationRAG, error) {
	storage, err := NewRAGStorage(dbPath, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %v", err)
	}

	return &ConversationRAG{
		storage: storage,
	}, nil
}

// Call handles storing and retrieving conversation memories
func (c *ConversationRAG) Call(ctx context.Context, input string) (string, error) {
	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolStart(ctx, input)
	}

	var request RAGRequest
	if err := json.Unmarshal([]byte(input), &request); err != nil {
		return fmt.Sprintf("error parsing input: %s", err.Error()), nil
	}

	var result string
	var err error

	switch request.Action {
	case "store":
		err = c.storage.StoreMemory(ctx, request.Conversation)
		if err != nil {
			result = fmt.Sprintf("error storing memory: %s", err.Error())
		} else {
			result = "memory stored successfully"
		}
	case "query":
		memories, err := c.storage.QueryMemories(ctx, request.Query)
		if err != nil {
			result = fmt.Sprintf("error querying memories: %s", err.Error())
		} else {
			resultJSON, err := json.Marshal(memories)
			if err != nil {
				result = fmt.Sprintf("error marshaling results: %s", err.Error())
			} else {
				result = string(resultJSON)
			}
		}
	default:
		result = fmt.Sprintf("invalid action: %s", request.Action)
	}

	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolEnd(ctx, result)
	}

	return result, nil
}

// Close closes the storage connection
func (c *ConversationRAG) Close() error {
	if c.storage != nil {
		return c.storage.Close()
	}
	return nil
}
