package types

// ResponseStyle represents the conversation style
type ResponseStyle string

const (
	ResponseStyleFormal    ResponseStyle = "formal"    // Professional and business-like tone
	ResponseStyleCasual    ResponseStyle = "casual"    // Relaxed and conversational tone
	ResponseStyleTechnical ResponseStyle = "technical" // Technical and precise language
	ResponseStyleDebate    ResponseStyle = "debate"    // Argumentative and persuasive style
	ResponseStyleHumorous  ResponseStyle = "humorous"  // Light-hearted and funny tone
)

// Voice represents available TTS voices from OpenAI
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

// IsValid checks if the ResponseStyle is valid
func (s ResponseStyle) IsValid() bool {
	switch s {
	case ResponseStyleFormal, ResponseStyleCasual, ResponseStyleTechnical,
		ResponseStyleDebate, ResponseStyleHumorous:
		return true
	}
	return false
}

// IsValid checks if the Voice is valid
func (v Voice) IsValid() bool {
	switch v {
	case VoiceAlloy, VoiceEcho, VoiceFable, VoiceOnyx, VoiceNova, VoiceShimmer:
		return true
	}
	return false
}

// String converts the enum to string
func (s ResponseStyle) String() string {
	return string(s)
}

// String converts the enum to string
func (v Voice) String() string {
	return string(v)
}
