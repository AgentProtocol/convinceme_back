package server

import "time"

type Config struct {
    OpenAIKey     string
    Port          string
    ResponseDelay time.Duration
    Agents        map[string]AgentConfig
}

type AgentConfig struct {
    Name           string
    Role           string
    Model          string
    Voice          string
    DebatePosition string
} 