package apitypes

// UsageSummary represents aggregated LLM usage data surfaced via APIs and CLI.
type UsageSummary struct {
	Provider           string `json:"provider"`
	Model              string `json:"model"`
	PromptTokens       int    `json:"prompt_tokens"`
	CompletionTokens   int    `json:"completion_tokens"`
	TotalTokens        int    `json:"total_tokens"`
	ReasoningTokens    *int   `json:"reasoning_tokens,omitempty"`
	CachedPromptTokens *int   `json:"cached_prompt_tokens,omitempty"`
	InputAudioTokens   *int   `json:"input_audio_tokens,omitempty"`
	OutputAudioTokens  *int   `json:"output_audio_tokens,omitempty"`
}
