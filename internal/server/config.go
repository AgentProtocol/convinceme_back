package server

// Config holds server configuration
type Config struct {
	Port           string
	OpenAIKey      string
	ElevenLabsKey  string
	ResponseDelay  int
}

type AgentConfig struct {
	Name           string
	Role           string
	Model          string
	Voice          string
	DebatePosition string
}
