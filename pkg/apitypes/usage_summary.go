package apitypes

import (
	"encoding/json"
	"time"
)

// UsageEntry models a single provider/model usage aggregate.
type UsageEntry struct {
	Provider           string     `json:"provider"`
	Model              string     `json:"model"`
	PromptTokens       int        `json:"prompt_tokens"`
	CompletionTokens   int        `json:"completion_tokens"`
	TotalTokens        int        `json:"total_tokens"`
	ReasoningTokens    *int       `json:"reasoning_tokens,omitempty"`
	CachedPromptTokens *int       `json:"cached_prompt_tokens,omitempty"`
	InputAudioTokens   *int       `json:"input_audio_tokens,omitempty"`
	OutputAudioTokens  *int       `json:"output_audio_tokens,omitempty"`
	AgentIDs           []string   `json:"agent_ids,omitempty"`
	CapturedAt         *time.Time `json:"captured_at,omitempty"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
	Source             string     `json:"source,omitempty"`
}

// UsageSummary represents aggregated LLM usage surfaced via APIs and CLI.
type UsageSummary struct {
	Entries []UsageEntry `json:"entries"`
}

// MarshalJSON emits the summary as an array for a compact API.
func (s *UsageSummary) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("null"), nil
	}
	return json.Marshal(s.Entries)
}

// UnmarshalJSON accepts either an array or object wrapper.
func (s *UsageSummary) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		s.Entries = nil
		return nil
	}
	var entries []UsageEntry
	if err := json.Unmarshal(data, &entries); err == nil {
		s.Entries = entries
		return nil
	}
	var wrapper struct {
		Entries []UsageEntry `json:"entries"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	s.Entries = wrapper.Entries
	return nil
}
