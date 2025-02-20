package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// RAGStorage handles persistent storage for conversation memories
type RAGStorage struct {
	db            *sql.DB
	vectorService *VectorService
}

// NewRAGStorage creates a new RAG storage instance
func NewRAGStorage(dbPath string, apiKey string) (*RAGStorage, error) {
	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	// Open database with WAL mode for better concurrency
	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_synchronous=NORMAL&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Set pragmas for better performance
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA mmap_size=30000000000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA auto_vacuum=INCREMENTAL",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma '%s': %v", pragma, err)
		}
	}

	// Create tables with proper indexes
	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	// Initialize vector service
	vectorService := NewVectorService(apiKey)

	// Test database connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	log.Printf("Successfully initialized RAG database at %s", dbPath)

	return &RAGStorage{
		db:            db,
		vectorService: vectorService,
	}, nil
}

// createTables creates the necessary database tables
func createTables(db *sql.DB) error {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Create conversations table with vector embedding support
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			topic TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			agent_names TEXT NOT NULL,
			conviction_score REAL NOT NULL,
			keywords TEXT NOT NULL,
			summary TEXT NOT NULL,
			embedding BLOB,
			UNIQUE(id)
		);
		
		CREATE INDEX IF NOT EXISTS idx_conversations_topic ON conversations(topic);
		CREATE INDEX IF NOT EXISTS idx_conversations_timestamp ON conversations(timestamp);
		CREATE INDEX IF NOT EXISTS idx_conversations_embedding ON conversations(embedding);
	`)
	if err != nil {
		return fmt.Errorf("failed to create conversations table: %v", err)
	}

	// Create messages table with better indexing
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			agent_name TEXT NOT NULL,
			content TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			embedding BLOB,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		);
		
		CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
		CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
		CREATE INDEX IF NOT EXISTS idx_messages_embedding ON messages(embedding);
	`)
	if err != nil {
		return fmt.Errorf("failed to create messages table: %v", err)
	}

	return tx.Commit()
}

// StoreMemory stores a conversation memory with vector embedding
func (s *RAGStorage) StoreMemory(ctx context.Context, memory *ConversationMemory) error {
	// Check for existing conversation with same content
	var existingID string
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM conversations 
		WHERE topic = ? AND agent_names = ? AND timestamp >= datetime('now', '-1 minute')
	`, memory.Topic, memory.AgentNames).Scan(&existingID)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check for existing conversation: %v", err)
	}

	if existingID != "" {
		// Skip if duplicate found within last minute
		return nil
	}

	// Generate embedding for the conversation
	text := fmt.Sprintf("%s %s", memory.Topic, memory.Summary)
	for _, msg := range memory.Messages {
		text += " " + msg.Content
	}

	embedding, err := s.vectorService.GetEmbedding(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %v", err)
	}

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback()

	// Store conversation
	agentNamesJSON, err := json.Marshal(memory.AgentNames)
	if err != nil {
		return fmt.Errorf("failed to marshal agent names: %v", err)
	}

	keywordsJSON, err := json.Marshal(memory.Keywords)
	if err != nil {
		return fmt.Errorf("failed to marshal keywords: %v", err)
	}

	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %v", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO conversations (
			id, topic, timestamp, agent_names, conviction_score,
			keywords, summary, embedding
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		memory.ID, memory.Topic, memory.Timestamp, string(agentNamesJSON),
		memory.ConvictionScore, string(keywordsJSON), memory.Summary, embeddingJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to insert conversation: %v", err)
	}

	// Store messages with duplicate check
	for _, msg := range memory.Messages {
		// Check for duplicate message
		var exists bool
		err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM messages 
				WHERE conversation_id = ? AND agent_name = ? AND content = ? AND timestamp >= datetime('now', '-1 minute')
			)
		`, memory.ID, msg.AgentName, msg.Content).Scan(&exists)

		if err != nil {
			return fmt.Errorf("failed to check for duplicate message: %v", err)
		}

		if !exists {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO messages (
					conversation_id, agent_name, content, timestamp
				) VALUES (?, ?, ?, ?)`,
				memory.ID, msg.AgentName, msg.Content, msg.Timestamp,
			)
			if err != nil {
				return fmt.Errorf("failed to insert message: %v", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// QueryMemories searches for relevant memories using vector similarity
func (s *RAGStorage) QueryMemories(ctx context.Context, query *MemoryQuery) ([]ConversationMemory, error) {
	// Generate embedding for the query
	queryText := query.Topic
	if len(query.Keywords) > 0 {
		queryText += " " + strings.Join(query.Keywords, " ")
	}

	queryEmbedding, err := s.vectorService.GetEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %v", err)
	}

	// Query conversations with vector similarity
	rows, err := s.db.QueryContext(ctx, `
		SELECT 
			c.id, c.topic, c.timestamp, c.agent_names, c.conviction_score,
			c.keywords, c.summary, c.embedding
		FROM conversations c
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %v", err)
	}
	defer rows.Close()

	var memories []ConversationMemory
	for rows.Next() {
		var memory ConversationMemory
		var agentNamesJSON, keywordsJSON, embeddingJSON string

		err := rows.Scan(
			&memory.ID, &memory.Topic, &memory.Timestamp,
			&agentNamesJSON, &memory.ConvictionScore,
			&keywordsJSON, &memory.Summary, &embeddingJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %v", err)
		}

		// Parse JSON fields
		if err := json.Unmarshal([]byte(agentNamesJSON), &memory.AgentNames); err != nil {
			return nil, fmt.Errorf("failed to unmarshal agent names: %v", err)
		}
		if err := json.Unmarshal([]byte(keywordsJSON), &memory.Keywords); err != nil {
			return nil, fmt.Errorf("failed to unmarshal keywords: %v", err)
		}

		var embedding []float32
		if err := json.Unmarshal([]byte(embeddingJSON), &embedding); err != nil {
			return nil, fmt.Errorf("failed to unmarshal embedding: %v", err)
		}

		// Calculate similarity score
		similarity := s.vectorService.CosineSimilarity(queryEmbedding, embedding)

		// Filter by agents if specified
		if len(query.Agents) > 0 {
			agentMatch := false
			for _, queryAgent := range query.Agents {
				for _, memoryAgent := range memory.AgentNames {
					if strings.EqualFold(queryAgent, memoryAgent) {
						agentMatch = true
						break
					}
				}
				if agentMatch {
					break
				}
			}
			if !agentMatch {
				continue
			}
		}

		// Load messages for this conversation
		messages, err := s.getConversationMessages(ctx, memory.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get conversation messages: %v", err)
		}
		memory.Messages = messages

		// Add to results if similarity is above threshold
		if similarity > 0.7 { // Adjust threshold as needed
			memories = append(memories, memory)
		}
	}

	// Sort by similarity and conviction score
	sort.Slice(memories, func(i, j int) bool {
		return memories[i].ConvictionScore > memories[j].ConvictionScore
	})

	// Apply limit if specified
	if query.Limit > 0 && len(memories) > query.Limit {
		memories = memories[:query.Limit]
	}

	return memories, nil
}

// getConversationMessages retrieves all messages for a conversation
func (s *RAGStorage) getConversationMessages(ctx context.Context, conversationID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT agent_name, content, timestamp
		FROM messages
		WHERE conversation_id = ?
		ORDER BY timestamp ASC`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %v", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		err := rows.Scan(&msg.AgentName, &msg.Content, &msg.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %v", err)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// Close closes the database connection
func (s *RAGStorage) Close() error {
	return s.db.Close()
}
