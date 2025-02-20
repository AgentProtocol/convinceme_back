package tools

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// VectorService handles text embeddings
type VectorService struct {
	client *openai.Client
}

// NewVectorService creates a new vector service
func NewVectorService(apiKey string) *VectorService {
	return &VectorService{
		client: openai.NewClient(apiKey),
	}
}

// GetEmbedding generates a vector embedding for the given text
func (v *VectorService) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Clean and prepare text
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty text")
	}

	// Get embedding from OpenAI
	resp, err := v.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: openai.AdaEmbeddingV2,
		Input: []string{text},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding: %v", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data received")
	}

	return resp.Data[0].Embedding, nil
}

// GetBatchEmbeddings generates embeddings for multiple texts
func (v *VectorService) GetBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty texts array")
	}

	// Clean texts
	for i := range texts {
		texts[i] = strings.TrimSpace(texts[i])
	}

	// Get embeddings from OpenAI
	resp, err := v.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: openai.AdaEmbeddingV2,
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings: %v", err)
	}

	// Extract embeddings
	embeddings := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}

// CosineSimilarity calculates the cosine similarity between two vectors
func (v *VectorService) CosineSimilarity(vec1, vec2 []float32) float32 {
	if len(vec1) != len(vec2) {
		return 0
	}

	var dotProduct float32
	var norm1 float32
	var norm2 float32

	for i := 0; i < len(vec1); i++ {
		dotProduct += vec1[i] * vec2[i]
		norm1 += vec1[i] * vec1[i]
		norm2 += vec2[i] * vec2[i]
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(norm1))) * float32(math.Sqrt(float64(norm2))))
}
