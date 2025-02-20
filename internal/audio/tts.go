package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
	"golang.org/x/time/rate"
)

type TTSService struct {
	client    *openai.Client
	voice     openai.SpeechVoice
	limiter   *rate.Limiter
	cache     map[string]cachedAudio
	cacheMu   sync.RWMutex
	cacheTime time.Duration
}

type cachedAudio struct {
	data      []byte
	timestamp time.Time
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
		client:    openai.NewClient(apiKey),
		voice:     voice,
		limiter:   rate.NewLimiter(rate.Every(100*time.Millisecond), 1), // 10 requests per second
		cache:     make(map[string]cachedAudio),
		cacheTime: 1 * time.Hour,
	}, nil
}

func (t *TTSService) GenerateAudio(ctx context.Context, text string) ([]byte, error) {
	// Check cache first
	if audio, ok := t.getCachedAudio(text); ok {
		log.Printf("Using cached audio for text: %s", text)
		return audio, nil
	}

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Wait for rate limiter
	if err := t.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %v", err)
	}

	req := openai.CreateSpeechRequest{
		Model:          openai.TTSModel1HD, // Using HD model for better quality
		Input:          text,
		Voice:          t.voice,
		ResponseFormat: openai.SpeechResponseFormatAac,
		Speed:          1.0,
	}

	// Retry logic for transient errors
	var resp io.ReadCloser
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		resp, err = t.client.CreateSpeech(ctx, req)
		if err == nil {
			break
		}
		log.Printf("TTS attempt %d failed: %v", attempts+1, err)
		time.Sleep(time.Duration(attempts+1) * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create speech after retries: %v", err)
	}
	defer resp.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	audioData := buf.Bytes()

	// Cache the result
	t.cacheAudio(text, audioData)

	log.Printf("Generated audio for text: %s", text)
	return audioData, nil
}

func (t *TTSService) getCachedAudio(text string) ([]byte, bool) {
	t.cacheMu.RLock()
	defer t.cacheMu.RUnlock()

	if cached, ok := t.cache[text]; ok {
		if time.Since(cached.timestamp) < t.cacheTime {
			return cached.data, true
		}
		// Cache expired, will be cleaned up later
	}
	return nil, false
}

func (t *TTSService) cacheAudio(text string, data []byte) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()

	// Clean old entries if cache is too large (>1000 entries)
	if len(t.cache) > 1000 {
		t.cleanCache()
	}

	t.cache[text] = cachedAudio{
		data:      data,
		timestamp: time.Now(),
	}
}

func (t *TTSService) cleanCache() {
	now := time.Now()
	for key, cached := range t.cache {
		if now.Sub(cached.timestamp) > t.cacheTime {
			delete(t.cache, key)
		}
	}
}
