package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/tmc/langchaingo/callbacks"
)

// ConvictionMetrics represents the metrics for measuring conviction
type ConvictionMetrics struct {
	Confidence      float64 `json:"confidence"`       // How confident the agent sounds
	Consistency     float64 `json:"consistency"`      // How consistent the agent's arguments are
	Persuasiveness  float64 `json:"persuasiveness"`   // How persuasive the arguments are
	EmotionalImpact float64 `json:"emotional_impact"` // Emotional impact of the arguments
	Overall         float64 `json:"overall"`          // Overall conviction score
}

// ConvictionMeter is a tool that measures the conviction rate of AI agents
type ConvictionMeter struct {
	CallbacksHandler callbacks.Handler
}

var _ Tool = ConvictionMeter{}

// Description returns a string describing the conviction meter tool
func (c ConvictionMeter) Description() string {
	return `Useful for measuring the conviction rate of AI agents in a conversation.
	The input should be a JSON string containing the agent's dialogue history and relevant context.
	Returns metrics about confidence, consistency, persuasiveness, and overall conviction rate.`
}

// Name returns the name of the tool
func (c ConvictionMeter) Name() string {
	return "conviction_meter"
}

type ConvictionInput struct {
	AgentName       string   `json:"agent_name"`
	DialogueHistory []string `json:"dialogue_history"`
	Topic           string   `json:"topic"`
}

// Call analyzes the input dialogue and returns conviction metrics
func (c ConvictionMeter) Call(ctx context.Context, input string) (string, error) {
	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolStart(ctx, input)
	}

	// Parse input JSON
	var convInput ConvictionInput
	if err := json.Unmarshal([]byte(input), &convInput); err != nil {
		return fmt.Sprintf("error parsing input: %s", err.Error()), nil
	}

	// Calculate metrics
	metrics := c.calculateMetrics(convInput)

	// Convert metrics to JSON string
	result, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Sprintf("error marshaling metrics: %s", err.Error()), nil
	}

	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolEnd(ctx, string(result))
	}

	return string(result), nil
}

// calculateMetrics analyzes the dialogue and calculates conviction metrics
func (c ConvictionMeter) calculateMetrics(input ConvictionInput) ConvictionMetrics {
	metrics := ConvictionMetrics{}

	totalMessages := float64(len(input.DialogueHistory))
	if totalMessages == 0 {
		return metrics
	}

	// Calculate confidence based on strong statements and evidence
	confidenceScore := 0.0
	confidenceIndicators := map[string]float64{
		"evidence shows":     1.0,
		"research indicates": 1.0,
		"clearly":            0.8,
		"definitely":         0.8,
		"proven":             1.0,
		"according to":       0.9,
		"studies show":       1.0,
		"data suggests":      0.9,
		"statistics":         0.9,
		"fact":               0.8,
		"i believe":          0.6,
		"in my experience":   0.7,
		"i think":            0.5,
	}

	for _, msg := range input.DialogueHistory {
		msg = strings.ToLower(msg)
		messageScore := 0.0
		for indicator, weight := range confidenceIndicators {
			if strings.Contains(msg, indicator) {
				messageScore += weight
			}
		}
		// Cap the message score at 1.0
		if messageScore > 1.0 {
			messageScore = 1.0
		}
		confidenceScore += messageScore
	}
	metrics.Confidence = normalizeScore(confidenceScore / totalMessages)

	// Calculate consistency
	consistencyScore := 1.0
	contradictions := 0.0
	for i := 1; i < len(input.DialogueHistory); i++ {
		if c.hasContradiction(input.DialogueHistory[i-1], input.DialogueHistory[i]) {
			contradictions++
		}
	}
	if totalMessages > 1 {
		consistencyScore = 1.0 - (contradictions / (totalMessages - 1))
	}
	metrics.Consistency = normalizeScore(consistencyScore)

	// Calculate persuasiveness
	persuasiveScore := 0.0
	persuasiveIndicators := map[string]float64{
		"because":          0.8,
		"therefore":        0.9,
		"consequently":     0.9,
		"this means":       0.7,
		"imagine":          0.6,
		"consider":         0.6,
		"think about":      0.6,
		"for example":      0.8,
		"specifically":     0.7,
		"importantly":      0.7,
		"let me explain":   0.8,
		"to put it simply": 0.7,
	}

	for _, msg := range input.DialogueHistory {
		msg = strings.ToLower(msg)
		messageScore := 0.0
		for indicator, weight := range persuasiveIndicators {
			if strings.Contains(msg, indicator) {
				messageScore += weight
			}
		}
		// Topic relevance bonus
		if strings.Contains(msg, strings.ToLower(input.Topic)) {
			messageScore += 0.3
		}
		// Cap the message score at 1.0
		if messageScore > 1.0 {
			messageScore = 1.0
		}
		persuasiveScore += messageScore
	}
	metrics.Persuasiveness = normalizeScore(persuasiveScore / totalMessages)

	// Calculate emotional impact
	emotionalScore := 0.0
	emotionalIndicators := map[string]float64{
		"passionate": 0.9,
		"crucial":    0.8,
		"essential":  0.8,
		"important":  0.7,
		"critical":   0.8,
		"we":         0.3,
		"our":        0.3,
		"together":   0.5,
		"community":  0.6,
		"future":     0.5,
		"believe":    0.6,
		"feel":       0.7,
		"care":       0.7,
		"hope":       0.6,
		"concerned":  0.7,
		"excited":    0.8,
		"worried":    0.7,
	}

	for _, msg := range input.DialogueHistory {
		msg = strings.ToLower(msg)
		messageScore := 0.0
		for indicator, weight := range emotionalIndicators {
			if strings.Contains(msg, indicator) {
				messageScore += weight
			}
		}
		// Cap the message score at 1.0
		if messageScore > 1.0 {
			messageScore = 1.0
		}
		emotionalScore += messageScore
	}
	metrics.EmotionalImpact = normalizeScore(emotionalScore / totalMessages)

	// Calculate overall score with weighted average
	metrics.Overall = normalizeScore(
		(metrics.Confidence * 0.3) +
			(metrics.Consistency * 0.2) +
			(metrics.Persuasiveness * 0.3) +
			(metrics.EmotionalImpact * 0.2),
	)

	return metrics
}

func normalizeScore(score float64) float64 {
	// Ensure score is between 0 and 1
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	// Apply a sigmoid-like normalization to spread out the values more evenly
	// This helps prevent all scores from clustering around the same value
	normalizedScore := (1.0 / (1.0 + math.Exp(-5.0*(score-0.5))))
	return normalizedScore
}

// measureConfidence analyzes confidence indicators in dialogue
func (c ConvictionMeter) measureConfidence(history []string) float64 {
	confidenceIndicators := []string{
		"certainly", "definitely", "absolutely", "clearly",
		"I am confident", "I know", "without doubt",
	}

	score := 0.0
	totalMessages := float64(len(history))

	for _, message := range history {
		message = strings.ToLower(message)
		for _, indicator := range confidenceIndicators {
			if strings.Contains(message, indicator) {
				score += 1.0
			}
		}
	}

	if totalMessages > 0 {
		return score / totalMessages
	}
	return 0.0
}

// measureConsistency checks for argument consistency
func (c ConvictionMeter) measureConsistency(history []string) float64 {
	// Simplified implementation - could be enhanced with more sophisticated NLP
	if len(history) < 2 {
		return 1.0 // Perfect consistency for single message
	}

	// Check for contradictions and consistency in arguments
	contradictions := 0.0
	for i := 1; i < len(history); i++ {
		if c.hasContradiction(history[i-1], history[i]) {
			contradictions++
		}
	}

	return 1.0 - (contradictions / float64(len(history)-1))
}

// measurePersuasiveness analyzes persuasive elements
func (c ConvictionMeter) measurePersuasiveness(history []string, topic string) float64 {
	persuasiveElements := []string{
		"because", "therefore", "consequently", "research shows",
		"evidence suggests", "studies indicate", "experts agree",
	}

	score := 0.0
	totalMessages := float64(len(history))

	for _, message := range history {
		message = strings.ToLower(message)
		for _, element := range persuasiveElements {
			if strings.Contains(message, element) {
				score += 1.0
			}
		}

		// Bonus for topic relevance
		if strings.Contains(message, strings.ToLower(topic)) {
			score += 0.5
		}
	}

	if totalMessages > 0 {
		return score / totalMessages
	}
	return 0.0
}

// measureEmotionalImpact analyzes emotional elements in dialogue
func (c ConvictionMeter) measureEmotionalImpact(history []string) float64 {
	emotionalIndicators := []string{
		"feel", "believe", "passionate", "important",
		"crucial", "essential", "critical", "vital",
	}

	score := 0.0
	totalMessages := float64(len(history))

	for _, message := range history {
		message = strings.ToLower(message)
		for _, indicator := range emotionalIndicators {
			if strings.Contains(message, indicator) {
				score += 1.0
			}
		}
	}

	if totalMessages > 0 {
		return score / totalMessages
	}
	return 0.0
}

// hasContradiction checks for basic contradictions between messages
func (c ConvictionMeter) hasContradiction(prev, current string) bool {
	// Simplified contradiction detection
	contradictionPairs := []struct {
		first  string
		second string
	}{
		{"agree", "disagree"},
		{"support", "oppose"},
		{"yes", "no"},
		{"true", "false"},
	}

	prev = strings.ToLower(prev)
	current = strings.ToLower(current)

	for _, pair := range contradictionPairs {
		if strings.Contains(prev, pair.first) && strings.Contains(current, pair.second) {
			return true
		}
		if strings.Contains(prev, pair.second) && strings.Contains(current, pair.first) {
			return true
		}
	}

	return false
}
