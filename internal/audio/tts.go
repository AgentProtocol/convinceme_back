package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// TTSService handles text-to-speech conversion
type TTSService struct {
	apiKey string
	voice  string
}

// NewTTSService creates a new TTS service instance
func NewTTSService(apiKey string, voice string) (*TTSService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	return &TTSService{
		apiKey: apiKey,
		voice:  voice,
	}, nil
}

// GenerateAudio converts text to speech using OpenAI's TTS API
func (s *TTSService) GenerateAudio(ctx context.Context, text string) ([]byte, error) {
	url := "https://api.openai.com/v1/audio/speech"

	requestBody := map[string]interface{}{
		"model": "tts-1",
		"input": text,
		"voice": s.voice,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	log.Printf("Generated audio for text: %s", text)

	return audioData, nil
}
