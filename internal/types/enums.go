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

// Voice represents available TTS voices
type Voice string

const (
	VoiceMark  Voice = "mark"
	VoiceFinn  Voice = "finn"
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

// String converts the enum to string
func (s ResponseStyle) String() string {
	return string(s)
}

// IsValid checks if the Voice is valid
func (v Voice) IsValid() bool {
	switch v {
	case VoiceMark, VoiceFinn:
		return true
	}
	return false
}

// String converts the enum to string
func (v Voice) String() string {
	return string(v)
}
