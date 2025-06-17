package server

// Config holds server configuration
type Config struct {
	Port                     string
	OpenAIKey                string
	ElevenLabsKey            string
	ResponseDelay            int
	JWTSecret                string // Secret key for JWT authentication
	RequireEmailVerification bool   // Whether to require email verification
	RequireInvitation        bool   // Whether to require invitation codes for registration
}

type AgentConfig struct {
	Name           string
	Role           string
	Model          string
	Voice          string
	DebatePosition string
}
