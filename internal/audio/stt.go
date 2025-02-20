package audio

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: audioFilePath,
	}

	// Retry logic for transient errors
	var resp openai.AudioResponse
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		resp, err = s.client.CreateTranscription(ctx, req)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(attempts+1) * time.Second)
	}
	if err != nil {
		return "", fmt.Errorf("failed to recognize speech after retries: %v", err)
	}

	return resp.Text, nil
}

// isValidAudioFormat checks if the file extension is supported
func isValidAudioFormat(filename string) bool {
	validExtensions := map[string]bool{
		".wav":  true,
		".mp3":  true,
		".m4a":  true,
		".mp4":  true,
		".mpeg": true,
		".mpga": true,
		".ogg":  true,
		".webm": true,
		".aac":  true,
		"":      true, // Allow files without extension (browser recordings)
	}
	ext := strings.ToLower(filepath.Ext(filename))
	return validExtensions[ext]
}

// HandleSTT processes audio files and returns transcribed text
func HandleSTT(c *gin.Context) {
	// Get API key from context
	apiKey, exists := c.Get("openai_api_key")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "OpenAI API key not found in context",
			"success": false,
		})
		return
	}

	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   fmt.Sprintf("Invalid audio file: %v", err),
			"success": false,
		})
		return
	}
	defer file.Close()

	// Get content type and validate
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "audio/") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   fmt.Sprintf("Invalid content type: %s. Expected audio/*", contentType),
			"success": false,
		})
		return
	}

	// Create temp file with correct extension
	ext := ".webm"
	if strings.Contains(contentType, "mp3") {
		ext = ".mp3"
	} else if strings.Contains(contentType, "wav") {
		ext = ".wav"
	} else if strings.Contains(contentType, "ogg") {
		ext = ".ogg"
	}

	tempFile, err := os.CreateTemp("", "audio-*"+ext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   fmt.Sprintf("Failed to create temp file: %v", err),
			"success": false,
		})
		return
	}
	tempPath := tempFile.Name()
	defer func() {
		tempFile.Close()
		os.Remove(tempPath)
	}()

	// Copy audio data to temp file
	if _, err := io.Copy(tempFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   fmt.Sprintf("Failed to save audio file: %v", err),
			"success": false,
		})
		return
	}

	// Create OpenAI client
	client := openai.NewClient(apiKey.(string))
	ctx := context.Background()

	// Open the temp file for reading
	audioFile, err := os.Open(tempPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   fmt.Sprintf("Failed to open temp file: %v", err),
			"success": false,
		})
		return
	}
	defer audioFile.Close()

	// Create transcription request
	req := openai.AudioRequest{
		Model:    openai.Whisper1,
		Reader:   audioFile,
		FilePath: filepath.Base(tempPath),
		Format:   openai.AudioResponseFormatText,
	}

	// Transcribe audio
	resp, err := client.CreateTranscription(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   fmt.Sprintf("Failed to transcribe audio: %v", err),
			"success": false,
		})
		return
	}

	// Return transcription
	c.JSON(http.StatusOK, gin.H{
		"text":    resp.Text,
		"success": true,
	})
}
