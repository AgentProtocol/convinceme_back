package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TTSService handles text-to-speech conversion using ElevenLabs
type TTSService struct {
	apiKey string
	voice  string
}

// ElevenLabsRequest represents the request body for ElevenLabs API
type ElevenLabsRequest struct {
	Text     string                 `json:"text"`
	ModelID  string                 `json:"model_id"`
	VoiceID  string                 `json:"voice_id"`
	Settings *ElevenLabsVoiceConfig `json:"voice_settings,omitempty"`
}

// ElevenLabsVoiceConfig represents voice settings for ElevenLabs
type ElevenLabsVoiceConfig struct {
	Stability       float32 `json:"stability"`
	SimilarityBoost float32 `json:"similarity_boost"`
}

// Voice IDs for ElevenLabs
const (
	VoiceMarkID = "UgBBYS2sOqTuMpoF3BR0"  // Mark's voice ID
	VoiceFinnID = "vBKc2FfBKJfcZNyEt1n6"  // Finn's voice ID
)

// NewTTSService creates a new TTS service instance
func NewTTSService(apiKey string, voice string) (*TTSService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("ElevenLabs API key is required")
	}

	return &TTSService{
		apiKey: apiKey,
		voice:  voice,
	}, nil
}

// getVoiceID maps voice names to ElevenLabs voice IDs
func (s *TTSService) getVoiceID(voice string) string {
	
	// Convert voice name to lowercase for case-insensitive matching
	voice = strings.ToLower(strings.TrimSpace(voice))
	
	voiceMap := map[string]string{
		"mark": VoiceMarkID,
		"finn": VoiceFinnID,
	}

	if id, ok := voiceMap[voice]; ok {
		return id
	}
	
	return VoiceMarkID // Default to Mark if voice not found
}

// GenerateAudio generates audio from text using ElevenLabs
func (s *TTSService) GenerateAudio(ctx context.Context, text string) ([]byte, error) {
	// Preprocess text to fix pronunciation
	text = s.preprocessTextForPronunciation(text)

	url := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s", s.getVoiceID(s.voice))

	requestBody := ElevenLabsRequest{
		Text:    text,
		ModelID: "eleven_multilingual_v2",
		Settings: &ElevenLabsVoiceConfig{
			Stability:       0.5,
			SimilarityBoost: 0.75,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("xi-api-key", s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

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

	return audioData, nil
}

// preprocessTextForPronunciation modifies text to ensure correct pronunciation
func (s *TTSService) preprocessTextForPronunciation(text string) string {
	// Fix "memecoins" pronunciation
	// Replace with phonetic spelling or spacing that guides pronunciation
	text = strings.ReplaceAll(text, "memecoins", "meemcoins")
	text = strings.ReplaceAll(text, "Memecoins", "meemcoins")
	text = strings.ReplaceAll(text, "MEMECOINS", "meemcoins")
	
	// Fix "$DOGE" pronunciation
	text = strings.ReplaceAll(text, "$DOGE", "doje coin")
	text = strings.ReplaceAll(text, "$doge", "doje coin")
	text = strings.ReplaceAll(text, "DOGE", "doje")
	text = strings.ReplaceAll(text, "Doge", "doje")
	text = strings.ReplaceAll(text, "doge", "doje")
	
	return text
}
