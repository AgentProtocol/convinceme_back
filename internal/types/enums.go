package types

import (
	"fmt"
)

// ResponseStyle represents the conversation style
type ResponseStyle string

const (
	ResponseStyleFormal    ResponseStyle = "formal"    // Professional and business-like tone
	ResponseStyleCasual    ResponseStyle = "casual"    // Relaxed and conversational tone
	ResponseStyleTechnical ResponseStyle = "technical" // Technical and precise language
	ResponseStyleDebate    ResponseStyle = "debate"    // Argumentative and persuasive style
	ResponseStyleHumorous  ResponseStyle = "humorous"  // Light-hearted and funny tone
)

// Voice represents available TTS voices
type Voice string

const (
	// VoiceAlloy - A versatile, neutral voice that maintains clarity and engagement
	VoiceAlloy Voice = "alloy"

	// VoiceEcho - A baritone voice with depth and warmth, good for narration
	VoiceEcho Voice = "echo"

	// VoiceFable - A youthful voice with a bright and optimistic tone
	VoiceFable Voice = "fable"

	// VoiceOnyx - A deep and authoritative male voice with gravitas
	VoiceOnyx Voice = "onyx"

	// VoiceNova - A feminine voice with a professional and welcoming tone
	VoiceNova Voice = "nova"

	// VoiceShimmer - A clear, energetic voice with a friendly character
	VoiceShimmer Voice = "shimmer"
)

var (
	// AllResponseStyles contains all valid response styles
	AllResponseStyles = []ResponseStyle{
		ResponseStyleFormal,
		ResponseStyleCasual,
		ResponseStyleTechnical,
		ResponseStyleDebate,
		ResponseStyleHumorous,
	}

	// AllVoices contains all valid voices
	AllVoices = []Voice{
		VoiceAlloy,
		VoiceEcho,
		VoiceFable,
		VoiceOnyx,
		VoiceNova,
		VoiceShimmer,
	}

	// responseStyleMap maps string values to ResponseStyle
	responseStyleMap = map[string]ResponseStyle{
		string(ResponseStyleFormal):    ResponseStyleFormal,
		string(ResponseStyleCasual):    ResponseStyleCasual,
		string(ResponseStyleTechnical): ResponseStyleTechnical,
		string(ResponseStyleDebate):    ResponseStyleDebate,
		string(ResponseStyleHumorous):  ResponseStyleHumorous,
	}

	// voiceMap maps string values to Voice
	voiceMap = map[string]Voice{
		string(VoiceAlloy):   VoiceAlloy,
		string(VoiceEcho):    VoiceEcho,
		string(VoiceFable):   VoiceFable,
		string(VoiceOnyx):    VoiceOnyx,
		string(VoiceNova):    VoiceNova,
		string(VoiceShimmer): VoiceShimmer,
	}
)

// Error types for invalid values
var (
	ErrInvalidResponseStyle = fmt.Errorf("invalid response style")
	ErrInvalidVoice         = fmt.Errorf("invalid voice")
)

// IsValid checks if the ResponseStyle is valid
func (s ResponseStyle) IsValid() bool {
	_, ok := responseStyleMap[string(s)]
	return ok
}

// String converts the enum to string
func (s ResponseStyle) String() string {
	return string(s)
}

// ParseResponseStyle parses a string into a ResponseStyle
func ParseResponseStyle(s string) (ResponseStyle, error) {
	if style, ok := responseStyleMap[s]; ok {
		return style, nil
	}
	return "", fmt.Errorf("%w: %s", ErrInvalidResponseStyle, s)
}

// GetAllResponseStyles returns all valid response styles
func GetAllResponseStyles() []ResponseStyle {
	return AllResponseStyles
}

// Description returns a human-readable description of the response style
func (s ResponseStyle) Description() string {
	switch s {
	case ResponseStyleFormal:
		return "Professional and business-like tone"
	case ResponseStyleCasual:
		return "Relaxed and conversational tone"
	case ResponseStyleTechnical:
		return "Technical and precise language"
	case ResponseStyleDebate:
		return "Argumentative and persuasive style"
	case ResponseStyleHumorous:
		return "Light-hearted and funny tone"
	default:
		return "Unknown response style"
	}
}

// IsValid checks if the Voice is valid
func (v Voice) IsValid() bool {
	_, ok := voiceMap[string(v)]
	return ok
}

// String converts the enum to string
func (v Voice) String() string {
	return string(v)
}

// ParseVoice parses a string into a Voice
func ParseVoice(s string) (Voice, error) {
	if voice, ok := voiceMap[s]; ok {
		return voice, nil
	}
	return "", fmt.Errorf("%w: %s", ErrInvalidVoice, s)
}

// GetAllVoices returns all valid voices
func GetAllVoices() []Voice {
	return AllVoices
}

// Description returns a human-readable description of the voice
func (v Voice) Description() string {
	switch v {
	case VoiceAlloy:
		return "A versatile, neutral voice that maintains clarity and engagement"
	case VoiceEcho:
		return "A baritone voice with depth and warmth, good for narration"
	case VoiceFable:
		return "A youthful voice with a bright and optimistic tone"
	case VoiceOnyx:
		return "A deep and authoritative male voice with gravitas"
	case VoiceNova:
		return "A feminine voice with a professional and welcoming tone"
	case VoiceShimmer:
		return "A clear, energetic voice with a friendly character"
	default:
		return "Unknown voice"
	}
}
