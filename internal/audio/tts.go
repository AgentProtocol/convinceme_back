package audio

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
	"io"
    "net/http"
)

type TTSService struct {
    apiKey string // API key for Eleven Labs
    voice  string // Voice model identifier
}

// NewTTSService creates a new TTS service with a specified voice and API key
func NewTTSService(apiKey, voice string) *TTSService {
    return &TTSService{
        apiKey: apiKey,
        voice:  voice,
    }
}

// GenerateAudio converts text to speech using Eleven Labs' TTS API
func (s *TTSService) GenerateAudio(ctx context.Context, text string) ([]byte, error) {
    url := "https://api.elevenlabs.io/v1/text-to-speech/" + s.voice

    requestBody := map[string]interface{}{
        "text": text,
        "voice": s.voice,
        "model": "elevenlabs", // Specify the model if needed
    }

    jsonBody, err := json.Marshal(requestBody)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request body: %v", err)
    }

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
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
        return nil, fmt.Errorf("failed to generate audio: %s", resp.Status)
    }

    audioData, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }

    return audioData, nil
}

/* // Example of initializing the TTS service
apiKey := "sk_14f431c6969dade4e1d77dc630b5cad392f9f69b6bd0c8ea"
voice := "k2hfpYcftdtw2RdRiaYP" // e.g., "voice_1"
ttsService := NewTTSService(apiKey, voice) */