package contract

// ContinueSessionRequest continues a user session with another agent, runtime, or
// declared route. Runtime and Route are mutually exclusive; the three fences are
// optional and must be sent together.
type ContinueSessionRequest struct {
	AgentName           string                         `json:"agent_name"`
	Runtime             *PromptRuntimeSelectionPayload `json:"runtime,omitempty"`
	Route               int                            `json:"route,omitempty"`
	Name                string                         `json:"name,omitempty"`
	Message             string                         `json:"message,omitempty"`
	IdempotencyKey      string                         `json:"idempotency_key"`
	ExpectedEpoch       *int64                         `json:"expected_epoch,omitempty"`
	ExpectedGeneration  *int64                         `json:"expected_generation,omitempty"`
	ExpectedMaxSequence *int64                         `json:"expected_max_sequence,omitempty"`
}

// ForkSessionRequest forks a user session with the same agent, runtime, and account.
// MessageID cuts through that durable user message and its turn; absent forks the whole
// session. The three fences are optional and must be sent together.
type ForkSessionRequest struct {
	MessageID           string `json:"message_id,omitempty"`
	Name                string `json:"name,omitempty"`
	IdempotencyKey      string `json:"idempotency_key"`
	ExpectedEpoch       *int64 `json:"expected_epoch,omitempty"`
	ExpectedGeneration  *int64 `json:"expected_generation,omitempty"`
	ExpectedMaxSequence *int64 `json:"expected_max_sequence,omitempty"`
}

// SessionDerivedPayload is the recorded outcome of one continue or fork.
type SessionDerivedPayload struct {
	Kind                 string `json:"kind"`
	SourceSessionID      string `json:"source_session_id"`
	OriginAgentName      string `json:"origin_agent_name"`
	OriginMessageID      string `json:"origin_message_id,omitempty"`
	ThroughTurnID        string `json:"through_turn_id"`
	Seed                 string `json:"seed"`
	NativeState          string `json:"native_state,omitempty"`
	ACPSessionID         string `json:"acp_session_id,omitempty"`
	NativeForkError      string `json:"native_fork_error,omitempty"`
	ReplayMessageCount   *int   `json:"replay_message_count,omitempty"`
	ReplayBytes          *int   `json:"replay_bytes,omitempty"`
	Truncated            bool   `json:"truncated"`
	OmittedCount         int    `json:"omitted_count"`
	SourceTurnInProgress bool   `json:"source_turn_in_progress"`
	FirstPrompt          string `json:"first_prompt"`
	Replayed             bool   `json:"replayed"`
	ChildDeleted         bool   `json:"child_deleted,omitempty"`
	// ChildSessionID names the derived session; it stays set after the child was deleted.
	ChildSessionID string `json:"child_session_id"`
}

// SessionDeriveResponse returns the derived session and the derive outcome. Session
// is omitted when the child was deleted since the recorded derivation.
type SessionDeriveResponse struct {
	Session *SessionPayload       `json:"session,omitempty"`
	Derived SessionDerivedPayload `json:"derived"`
}

// SessionTranscriptFences are the transcript coordinates a derive snapshot was taken at.
type SessionTranscriptFences struct {
	Epoch       int64 `json:"epoch"`
	Generation  int64 `json:"generation"`
	MaxSequence int64 `json:"max_sequence"`
}

// SessionDeriveCutPayload is the turn-resolved cut of a fork anchor.
type SessionDeriveCutPayload struct {
	MessageID   string `json:"message_id"`
	TurnID      string `json:"turn_id"`
	TurnSettled bool   `json:"turn_settled"`
}

// SessionDerivePreviewResponse reports what a continue or fork would carry right now.
type SessionDerivePreviewResponse struct {
	MessageCount int `json:"message_count"`
	// SourceMessageCount is the whole source transcript's message count, so a fork cut
	// or a truncated carry reads "k of n".
	SourceMessageCount   int                      `json:"source_message_count"`
	ReplayBytes          int                      `json:"replay_bytes"`
	Truncated            bool                     `json:"truncated"`
	OmittedCount         int                      `json:"omitted_count"`
	SourceTurnInProgress bool                     `json:"source_turn_in_progress"`
	NativeForkPossible   bool                     `json:"native_fork_possible"`
	Cut                  *SessionDeriveCutPayload `json:"cut,omitempty"`
	Transcript           SessionTranscriptFences  `json:"transcript"`
}
