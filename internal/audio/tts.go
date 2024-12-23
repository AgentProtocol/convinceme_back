package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"

	"github.com/sashabaranov/go-openai"
)

// TTSService handles text-to-speech conversion
type TTSService struct {
	client *openai.Client
	voice  openai.SpeechVoice
}

// NewTTSService creates a new TTS service with the specified voice
func NewTTSService(apiKey string, voiceStr string) (*TTSService, error) {
	var voice openai.SpeechVoice
	switch voiceStr {
	case "alloy":
		voice = openai.VoiceAlloy
	case "echo":
		voice = openai.VoiceEcho
	case "fable":
		voice = openai.VoiceFable
	case "onyx":
		voice = openai.VoiceOnyx
	case "nova":
		voice = openai.VoiceNova
	case "shimmer":
		voice = openai.VoiceShimmer
	default:
		voice = openai.VoiceAlloy
	}

	return &TTSService{
		client: openai.NewClient(apiKey),
		voice:  voice,
	}, nil
}

// GenerateAudio generates audio from text and returns the audio data
func (t *TTSService) GenerateAudio(ctx context.Context, text string) ([]byte, error) {
	req := openai.CreateSpeechRequest{
		Model:          openai.TTSModel1,
		Input:          text,
		Voice:          t.voice,
		ResponseFormat: openai.SpeechResponseFormatMp3,
	}

	resp, err := t.client.CreateSpeech(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create speech: %v", err)
	}
	defer resp.Close()

	// Read the entire response into a buffer
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Log the generated audio
	log.Printf("Generated audio for text: %s", text)

	return buf.Bytes(), nil
}
