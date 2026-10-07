package store

import (
	"time"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

// Native bootstrap states of a native-fork child (settled at its first bind).
const (
	SessionNativeStatePending = "pending"
	SessionNativeStateLoaded  = "loaded"
	SessionNativeStateFailed  = "failed"
)

// SessionDerivation is the derive record persisted in a continued or forked child's
// metadata document. It is written once at creation; only the native bootstrap state,
// the pending route (cleared by an explicit runtime choice), and the first-prompt state
// change afterwards.
type SessionDerivation struct {
	Kind            LineageKind             `json:"kind"`
	SourceSessionID string                  `json:"source_session_id"`
	IdempotencyKey  string                  `json:"idempotency_key"`
	Seed            string                  `json:"seed"`
	Native          *SessionNativeBootstrap `json:"native,omitzero"`
	PendingRoute    *SessionPendingRoute    `json:"pending_route,omitzero"`
	FirstPrompt     SessionFirstPrompt      `json:"first_prompt"`
	CreatedAt       time.Time               `json:"created_at"`
}

// SessionNativeBootstrap pins a native clone id to the provider identity it loads under.
type SessionNativeBootstrap struct {
	ACPSessionID       string     `json:"acp_session_id"`
	Provider           string     `json:"provider"`
	AuthMode           string     `json:"auth_mode"`
	HomePolicy         string     `json:"home_policy"`
	CommandFingerprint string     `json:"command_fingerprint"`
	State              string     `json:"state"`
	Error              string     `json:"error,omitempty"`
	SettledAt          *time.Time `json:"settled_at,omitempty"`
}

// SessionPendingRoute is the declared route chosen at derive time, applied at the
// child's first bind. Index is 1-based in the target agent's fallback_chain.
type SessionPendingRoute struct {
	Index              int                         `json:"index"`
	Provider           string                      `json:"provider"`
	Model              string                      `json:"model"`
	ReasoningEffort    string                      `json:"reasoning_effort,omitempty"`
	Speed              speedpkg.Speed              `json:"speed,omitempty"`
	ACPOptions         []SessionACPOptionSelection `json:"acp_options,omitempty"`
	CommandFingerprint string                      `json:"command_fingerprint,omitempty"`
}

// SessionFirstPrompt records whether the derive admitted a first message.
type SessionFirstPrompt struct {
	State        string `json:"state"`
	AdmissionKey string `json:"admission_key,omitempty"`
	MessageID    string `json:"message_id,omitempty"`
}

// SessionImportedContext is the immutable carried context of a derived child: the
// bounded, pruned canonical transcript taken from one source snapshot. MessagesJSON is
// never rewritten; Consumed records the prompt dispatch that first carried it.
type SessionImportedContext struct {
	SourceSessionID      string                             `json:"source_session_id"`
	Kind                 LineageKind                        `json:"kind"`
	OriginAgentName      string                             `json:"origin_agent_name"`
	OriginMessageID      string                             `json:"origin_message_id,omitempty"`
	ThroughTurnID        string                             `json:"through_turn_id"`
	MessagesJSON         string                             `json:"messages_json"`
	MessageCount         int                                `json:"message_count"`
	Bytes                int                                `json:"bytes"`
	Truncated            bool                               `json:"truncated"`
	OmittedCount         int                                `json:"omitted_count"`
	SourceTurnInProgress bool                               `json:"source_turn_in_progress"`
	SourceEpoch          int64                              `json:"source_epoch"`
	SourceGeneration     int64                              `json:"source_generation"`
	SourceMaxSequence    int64                              `json:"source_max_sequence"`
	Consumed             *SessionImportedContextConsumption `json:"consumed,omitzero"`
	CreatedAt            time.Time                          `json:"created_at"`
}

// SessionImportedContextConsumption names the prompt dispatch that first carried the
// imported context to an agent.
type SessionImportedContextConsumption struct {
	AdmissionKey string    `json:"admission_key,omitempty"`
	MessageID    string    `json:"message_id,omitempty"`
	At           time.Time `json:"at"`
}

// CloneSessionDerivation deep-copies a derivation record.
func CloneSessionDerivation(derivation *SessionDerivation) *SessionDerivation {
	if derivation == nil {
		return nil
	}
	cloned := *derivation
	if derivation.Native != nil {
		native := *derivation.Native
		if derivation.Native.SettledAt != nil {
			settled := derivation.Native.SettledAt.UTC()
			native.SettledAt = &settled
		}
		cloned.Native = &native
	}
	cloned.PendingRoute = CloneSessionPendingRoute(derivation.PendingRoute)
	return &cloned
}

// CloneSessionPendingRoute deep-copies a pending route.
func CloneSessionPendingRoute(route *SessionPendingRoute) *SessionPendingRoute {
	if route == nil {
		return nil
	}
	cloned := *route
	cloned.ACPOptions = CloneSessionACPOptionSelections(route.ACPOptions)
	return &cloned
}

// CloneSessionImportedContext deep-copies an imported context.
func CloneSessionImportedContext(context *SessionImportedContext) *SessionImportedContext {
	if context == nil {
		return nil
	}
	cloned := *context
	if context.Consumed != nil {
		consumed := *context.Consumed
		cloned.Consumed = &consumed
	}
	return &cloned
}
