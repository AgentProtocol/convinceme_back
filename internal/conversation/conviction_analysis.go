package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/neo/convinceme_backend/internal/tools"
)

// ConvictionAnalysis holds conviction metrics for both agents
type ConvictionAnalysis struct {
	InterviewerMetrics tools.ConvictionMetrics `json:"interviewer_metrics"`
	GuestMetrics       tools.ConvictionMetrics `json:"guest_metrics"`
	Topic              string                  `json:"topic"`
}

// AnalyzeConviction measures the conviction rates of both agents in the conversation
func (c *Conversation) AnalyzeConviction(ctx context.Context) (*ConvictionAnalysis, error) {
	// Create conviction meter tool
	meter := tools.ConvictionMeter{}

	// Separate dialogue history by agent
	interviewerHistory := make([]string, 0)
	guestHistory := make([]string, 0)

	// Get dialogue history from conversation
	c.mu.RLock()
	messages := c.messages // Use the messages field directly
	c.mu.RUnlock()

	// Separate messages by agent
	for _, msg := range messages {
		if msg.Agent == c.agent1 {
			interviewerHistory = append(interviewerHistory, msg.Content)
		} else if msg.Agent == c.agent2 {
			guestHistory = append(guestHistory, msg.Content)
		}
	}

	// Analyze interviewer conviction
	interviewerInput := tools.ConvictionInput{
		AgentName:       c.agent1.GetName(),
		DialogueHistory: interviewerHistory,
		Topic:           c.config.Topic,
	}

	interviewerInputJSON, err := json.Marshal(interviewerInput)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal interviewer input: %v", err)
	}

	interviewerResult, err := meter.Call(ctx, string(interviewerInputJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to analyze interviewer conviction: %v", err)
	}

	var interviewerMetrics tools.ConvictionMetrics
	if err := json.Unmarshal([]byte(interviewerResult), &interviewerMetrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal interviewer metrics: %v", err)
	}

	// Analyze guest conviction
	guestInput := tools.ConvictionInput{
		AgentName:       c.agent2.GetName(),
		DialogueHistory: guestHistory,
		Topic:           c.config.Topic,
	}

	guestInputJSON, err := json.Marshal(guestInput)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal guest input: %v", err)
	}

	guestResult, err := meter.Call(ctx, string(guestInputJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to analyze guest conviction: %v", err)
	}

	var guestMetrics tools.ConvictionMetrics
	if err := json.Unmarshal([]byte(guestResult), &guestMetrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal guest metrics: %v", err)
	}

	// Create analysis result
	analysis := &ConvictionAnalysis{
		InterviewerMetrics: interviewerMetrics,
		GuestMetrics:       guestMetrics,
		Topic:              c.config.Topic,
	}

	// Log the analysis results
	log.Printf("Conviction Analysis for topic '%s':\n", c.config.Topic)
	log.Printf("Interviewer (%s) metrics: %+v\n", c.agent1.GetName(), interviewerMetrics)
	log.Printf("Guest (%s) metrics: %+v\n", c.agent2.GetName(), guestMetrics)

	return analysis, nil
}
