package scoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type ArgumentScore struct {
	Strength    int     `json:"strength"`    // Support for position (0-100)
	Relevance   int     `json:"relevance"`   // Relevance to discussion (0-100)
	Logic       int     `json:"logic"`       // Logical structure (0-100)
	Truth       int     `json:"truth"`       // Factual accuracy (0-100)
	Humor       int     `json:"humor"`       // Entertainment value (0-100)
	Average     float64 `json:"average"`     // Average of all scores
	Agent1_support int	`json:"Agent1_support"` //Level of support for agent 1(0-100)
	Agent2_support int	`json:"Agent2_support"` //Level of support for agent 2(0-100)
	Explanation string  `json:"explanation"` // Brief explanation
}

type Scorer struct {
	llm llms.LLM
}

func NewScorer(apiKey string) (*Scorer, error) {
	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithModel("gpt-4o-mini"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create scorer LLM: %v", err)
	}

	return &Scorer{llm: llm}, nil
}

func (s *Scorer) ScoreArgument(ctx context.Context, argument, topic string, agent1Name, agent2Name string) (*ArgumentScore, error) {
	prompt := fmt.Sprintf(`Evaluate this argument about "%s":

"%s"

Score each aspect from 0-100 and explain why:
- Strength: How well it supports their position
- Relevance: How relevant to the discussion
- Logic: Quality of reasoning and structure
- Truth: Factual accuracy and credibility
- Humor: Entertainment and engagement value
- Supporting "%s": How much does this support "%s" position (0-100)
- Supporting "%s": How much does this support "%s" position (0-100)
- Explanation: Brief explanation of scores,


Your response MUST ONLY be a valid JSON object with the following structure. Dont write the word json, just output a correct json-formatted object, starting with a { symbol
    "strength": <0-100>,
    "relevance": <0-100>,
    "logic": <0-100>,
    "truth": <0-100>,
    "humor": <0-100>,
	"Agent1_support": <0-100>,
    "Agent2_support": <0-100>,
    "Explanation": "<brief explanation of scores>"
}`, topic, argument, agent1Name, agent2Name, agent1Name, agent2Name)

	completion, err := s.llm.Call(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("scoring failed: %v", err)
	}

	completion = strings.TrimSpace(completion)
	completion = strings.Trim(completion, "`")
	log.Printf("Raw Jason in scorer.go is :\n")

	log.Printf(completion)


	var score ArgumentScore
	if err := json.Unmarshal([]byte(completion), &score); err != nil {
		return nil, fmt.Errorf("failed to parse score: %v\nraw response: %s", err, completion)
	}

	// Calculate average
	score.Average = float64(score.Strength+score.Relevance+score.Logic+score.Truth+score.Humor) / 5.0

	// Print scores
	// log.Printf("\n"+
	// 	"┏━━━━━━━━━━━━━━━━━━━━ ARGUMENT ANALYSIS ━━━━━━━━━━━━━━━━━━━━┓\n"+
	// 	"┃ Topic: %-52s ┃\n"+
	// 	"┃ Argument: %-50s ┃\n"+
	// 	"┣━━━━━━━━━━━━━━━━━━━━━━━ SCORES ━━━━━━━━━━━━━━━━━━━━━━━━┫\n"+
	// 	"┃ Strength:  %3d/100  Support for position                    ┃\n"+
	// 	"┃ Relevance: %3d/100  Conversation fit                        ┃\n"+
	// 	"┃ Logic:     %3d/100  Reasoning quality                       ┃\n"+
	// 	"┃ Truth:     %3d/100  Factual accuracy                       ┃\n"+
	// 	"┃ Humor:     %3d/100  Entertainment value                    ┃\n"+
	// 	"┣━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫\n"+
	// 	"┃ AVERAGE:   %.1f/100                                         ┃\n"+
	// 	"┃ Analysis: %-51s ┃\n"+
	// 	"┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛\n",
	// 	topic,
	// 	truncateString(argument, 50),
	// 	score.Strength,
	// 	score.Relevance,
	// 	score.Logic,
	// 	score.Truth,
	// 	score.Humor,
	// 	score.Average,
	// 	truncateString(score.Explanation, 51))

	return &score, nil
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}