package player

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// InputType represents the type of input received from the player
type InputType string

const (
	InputTypeText  InputType = "text"
	InputTypeVoice InputType = "voice"
)

// PlayerInput represents a single input from the player
type PlayerInput struct {
	Type      InputType  `json:"type"`
	Content   string     `json:"content"`
	Timestamp time.Time  `json:"timestamp"`
}

// InputHandler manages player inputs and their processing
type InputHandler struct {
	inputs     []PlayerInput
	mu         sync.RWMutex
	processors []InputProcessor
	logger     *log.Logger
}

// InputProcessor is an interface for components that need to process player input
type InputProcessor interface {
	ProcessInput(ctx context.Context, input PlayerInput) error
}

// NewInputHandler creates a new input handler
func NewInputHandler(logger *log.Logger) *InputHandler {
	return &InputHandler{
		inputs:     make([]PlayerInput, 0),
		processors: make([]InputProcessor, 0),
		logger:     logger,
	}
}

// RegisterProcessor adds a new input processor
func (h *InputHandler) RegisterProcessor(processor InputProcessor) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.processors = append(h.processors, processor)
}

// HandleInput processes a new input from the player
func (h *InputHandler) HandleInput(ctx context.Context, inputType InputType, content string) error {
	input := PlayerInput{
		Type:      inputType,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Log the input
	inputJSON, _ := json.Marshal(input)
	h.logger.Printf("Received player input: %s", string(inputJSON))

	// Store the input
	h.mu.Lock()
	h.inputs = append(h.inputs, input)
	h.mu.Unlock()

	// Process the input through all registered processors
	for _, processor := range h.processors {
		if err := processor.ProcessInput(ctx, input); err != nil {
			h.logger.Printf("Error processing input: %v", err)
			return err
		}
	}

	return nil
}

// GetRecentInputs returns the n most recent inputs
func (h *InputHandler) GetRecentInputs(n int) []PlayerInput {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.inputs) <= n {
		return h.inputs
	}
	return h.inputs[len(h.inputs)-n:]
}
