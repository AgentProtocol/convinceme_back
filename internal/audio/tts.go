package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"

	"github.com/sashabaranov/go-openai"
)

type TTSService struct {
	client *openai.Client
	voice  openai.SpeechVoice
}

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

func (t *TTSService) GenerateAudio(ctx context.Context, text string) ([]byte, error) {
	req := openai.CreateSpeechRequest{
		Model:          openai.TTSModel1,
		Input:          text,
		Voice:          t.voice,
		ResponseFormat: openai.SpeechResponseFormatAac,
	}

	resp, err := t.client.CreateSpeech(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create speech: %v", err)
	}
	defer resp.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	log.Printf("Generated audio for text: %s", text)

	return buf.Bytes(), nil
}
