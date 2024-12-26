package audio

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
)

type STTService struct {
	client *openai.Client
}

func NewSTTService(apiKey string) *STTService {
	return &STTService{
		client: openai.NewClient(apiKey),
	}
}

func (s *STTService) RecognizeSpeech(ctx context.Context, audioFilePath string) (string, error) {
	req := openai.AudioRequest{
		Model:    "whisper-1",
		FilePath: audioFilePath,
	}

	resp, err := s.client.CreateTranscription(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to recognize speech: %v", err)
	}

	return resp.Text, nil
}

// HandleSTT processes audio files and returns transcribed text
func HandleSTT(c *gin.Context) {
	file, _, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid audio file"})
		return
	}
	defer file.Close()

	tempFile, err := os.CreateTemp("", "audio-*.wav")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
		return
	}
	defer os.Remove(tempFile.Name())

	if _, err := io.Copy(tempFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save audio file"})
		return
	}

	if err := godotenv.Load(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load environment variables"})
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OPENAI_API_KEY is not set"})
		return
	}

	sttService := NewSTTService(apiKey)
	text, err := sttService.RecognizeSpeech(c.Request.Context(), tempFile.Name())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"text":    text,
	})
}
