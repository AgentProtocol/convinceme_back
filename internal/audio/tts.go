package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/neo/convinceme_backend/internal/logging"
)

// TTSProvider represents the text-to-speech provider
type TTSProvider string

const (
	// ProviderOpenAI uses OpenAI for TTS
	ProviderOpenAI TTSProvider = "openai"
	// ProviderElevenLabs uses ElevenLabs for TTS
	ProviderElevenLabs TTSProvider = "elevenlabs"
)

// TTSService handles text-to-speech conversion
type TTSService struct {
	apiKey   string
	voice    string
	provider TTSProvider
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
	VoiceMarkID = "UgBBYS2sOqTuMpoF3BR0" // Mark's voice ID
	VoiceFinnID = "vBKc2FfBKJfcZNyEt1n6" // Finn's voice ID
)

// NewTTSService creates a new TTS service instance
func NewTTSService(voice string) (*TTSService, error) {
	// Determine provider from environment variable
	provider := ProviderElevenLabs

	// Get API keys from environment
	elevenLabsKey := os.Getenv("ELEVENLABS_API_KEY")
	openAIKey := os.Getenv("OPENAI_API_KEY")

	// Select provider and API key
	apiKey := elevenLabsKey
	if os.Getenv("TTS_PROVIDER") == "openai" {
		provider = ProviderOpenAI
		apiKey = openAIKey
		log.Printf("Using OpenAI for TTS")
	} else {
		log.Printf("Using ElevenLabs for TTS")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required for the selected TTS provider")
	}

	return &TTSService{
		apiKey:   apiKey,
		voice:    voice,
		provider: provider,
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

// GenerateAudio generates audio from text using the configured provider
func (s *TTSService) GenerateAudio(ctx context.Context, text string) ([]byte, error) {
	textPreview := text
	if len(text) > 50 {
		textPreview = text[:50]
	}

	logging.LogTTSEvent("audio_generation_start", s.voice, map[string]interface{}{
		"provider":     string(s.provider),
		"text_length":  len(text),
		"text_preview": textPreview,
	})

	var audioData []byte
	var err error

	switch s.provider {
	case ProviderOpenAI:
		audioData, err = s.generateAudioOpenAI(ctx, text)
	case ProviderElevenLabs:
		audioData, err = s.generateAudioElevenLabs(ctx, text)
	default:
		err = fmt.Errorf("unsupported TTS provider: %s", s.provider)
	}

	if err != nil {
		logging.LogTTSEvent("audio_generation_failed", s.voice, map[string]interface{}{
			"provider": string(s.provider),
			"error":    err,
		})
		return nil, err
	}

	logging.LogTTSEvent("audio_generation_success", s.voice, map[string]interface{}{
		"provider":   string(s.provider),
		"audio_size": len(audioData),
	})

	return audioData, nil
}

// generateAudioOpenAI converts text to speech using OpenAI's TTS API
func (s *TTSService) generateAudioOpenAI(ctx context.Context, text string) ([]byte, error) {
	url := "https://api.openai.com/v1/audio/speech"

	voice := "fable"
	if s.voice == "mark" {
		voice = "onyx"
	}

	requestBody := map[string]interface{}{
		"model":           "tts-1",
		"input":           text,
		"voice":           voice,
		"response_format": "mp3",
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

	log.Printf("Generated audio for text using OpenAI: %s", text)

	return audioData, nil
}

// generateAudioElevenLabs generates audio from text using ElevenLabs
func (s *TTSService) generateAudioElevenLabs(ctx context.Context, text string) ([]byte, error) {
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

	log.Printf("Generated audio for text using ElevenLabs: %s", text)

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
