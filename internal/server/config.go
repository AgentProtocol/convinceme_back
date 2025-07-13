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
	PrivyAppID               string // Privy App ID for validating external tokens
	PrivyVerificationKey     string // Privy verification key for ES256 tokens (optional)
}

type AgentConfig struct {
	Name           string
	Role           string
	Model          string
	Voice          string
	DebatePosition string
}
