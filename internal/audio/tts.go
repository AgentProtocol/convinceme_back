package audio

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sashabaranov/go-openai"
)

type TTSService struct {
	client *openai.Client
	voice  openai.SpeechVoice
}

// NewTTSService creates a new TTS service with the specified voice
func NewTTSService(apiKey string, voiceStr string) *TTSService {
	voice := openai.VoiceAlloy // default voice
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
	}

	return &TTSService{
		client: openai.NewClient(apiKey),
		voice:  voice,
	}
}

func (s *TTSService) TextToSpeech(ctx context.Context, text string, outputPath string) error {
	req := openai.CreateSpeechRequest{
		Model: openai.TTSModel1,
		Input: text,
		Voice: s.voice,
	}

	resp, err := s.client.CreateSpeech(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create speech: %v", err)
	}
	defer resp.Close()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Create the output file
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer out.Close()

	// Copy the response body to the output file
	_, err = io.Copy(out, resp)
	if err != nil {
		return fmt.Errorf("failed to write audio file: %v", err)
	}

	return nil
}

// GenerateAndStream generates audio from text and streams it to the writer
func (t *TTSService) GenerateAndStream(ctx context.Context, text string, writer io.Writer) error {
	req := openai.CreateSpeechRequest{
		Model:          openai.TTSModel1,
		Input:          text,
		Voice:          t.voice,
		ResponseFormat: openai.SpeechResponseFormatOpus,
	}

	resp, err := t.client.CreateSpeech(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create speech: %v", err)
	}

	// Stream the Opus data directly from OpenAI
	_, err = io.Copy(writer, resp)
	return err
}

func (s *TTSService) GenerateToFile(ctx context.Context, text, outputDir string) (string, error) {
	outputFile := filepath.Join(outputDir, "output.opus")

	f, err := os.Create(outputFile)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %v", err)
	}
	defer f.Close()

	if err := s.GenerateAndStream(ctx, text, f); err != nil {
		return "", err
	}

	return outputFile, nil
}
