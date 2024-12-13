package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/sashabaranov/go-openai"
)

type TTSService struct {
	client *openai.Client
	voice  openai.SpeechVoice
	hls    *HLSManager
}

// NewTTSService creates a new TTS service with the specified voice
func NewTTSService(apiKey string, voiceStr string, hlsDir string) (*TTSService, error) {
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

	hlsManager, err := NewHLSManager(hlsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create HLS manager: %v", err)
	}

	return &TTSService{
		client: openai.NewClient(apiKey),
		voice:  voice,
		hls:    hlsManager,
	}, nil
}

// GenerateAndStream generates audio from text and streams it to the writer
func (t *TTSService) GenerateAndStream(ctx context.Context, text string, agentName string) (string, error) {
	req := openai.CreateSpeechRequest{
		Model:          openai.TTSModel1,
		Input:          text,
		Voice:          t.voice,
		ResponseFormat: openai.SpeechResponseFormatAac,
	}

	resp, err := t.client.CreateSpeech(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create speech: %v", err)
	}
	defer resp.Close()

	// Read the entire response into a buffer
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp); err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Add the aa data as a new HLS segment
	segmentName, err := t.hls.AddSegment(buf.Bytes(), agentName)
	if err != nil {
		return "", fmt.Errorf("failed to add HLS segment: %v", err)
	}

	// Return the full URL path
	return fmt.Sprintf("http://localhost:8080/hls/%s", segmentName), nil
}

// GetPlaylistURL returns the URL of the HLS playlist
func (t *TTSService) GetPlaylistURL() string {
	return "http://localhost:8080/hls/playlist.m3u8"
}

// GenerateToFile generates audio and saves it to a file
func (t *TTSService) GenerateToFile(ctx context.Context, text, outputDir string) (string, error) {
	outputFile := filepath.Join(outputDir, "output.aac")

	// Generate the audio and get the HLS path
	if _, err := t.GenerateAndStream(ctx, text, "default"); err != nil {
		return "", err
	}

	return outputFile, nil
}
