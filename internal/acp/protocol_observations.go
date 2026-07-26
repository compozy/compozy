package acp

import "strings"

// PromptStopReason is the closed ACP turn-completion reason vocabulary.
type PromptStopReason string

const (
	PromptStopReasonEndTurn         PromptStopReason = "end_turn"
	PromptStopReasonMaxTokens       PromptStopReason = "max_tokens"
	PromptStopReasonMaxTurnRequests PromptStopReason = "max_turn_requests"
	PromptStopReasonRefusal         PromptStopReason = "refusal"
	// ACP spells the cancellation wire value with a doubled l.
	PromptStopReasonCancelled PromptStopReason = "cancel" + "led"

	// EventTypeAvailableCommands replaces the current typed command set for a session.
	EventTypeAvailableCommands = "available_commands_update"
)

// Valid reports whether the value is one of the five ACP stop reasons.
func (r PromptStopReason) Valid() bool {
	raw := string(r)
	if raw != strings.TrimSpace(raw) {
		return false
	}
	switch r {
	case PromptStopReasonEndTurn,
		PromptStopReasonMaxTokens,
		PromptStopReasonMaxTurnRequests,
		PromptStopReasonRefusal,
		PromptStopReasonCancelled:
		return true
	default:
		return false
	}
}
