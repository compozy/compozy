package contract

import "time"

type SessionContextState string

const (
	SessionContextStateReported      SessionContextState = "reported"
	SessionContextStateEstimatedSize SessionContextState = "estimated_size"
	SessionContextStateUnknown       SessionContextState = "unknown"
	SessionContextStateUnavailable   SessionContextState = "unavailable"
	SessionStreamEventUsageChanged                       = "session_usage_changed"
)

type SessionUsageChangedPayload struct {
	Sequence int64  `json:"sequence"`
	TurnID   string `json:"turn_id,omitempty"`
	Kind     string `json:"kind"`
}

type SessionContextPayload struct {
	State             SessionContextState            `json:"state"`
	Used              *int64                         `json:"used,omitempty"`
	Size              *int64                         `json:"size,omitempty"`
	Ratio             *float64                       `json:"ratio,omitempty"`
	SizeSource        string                         `json:"size_source,omitempty"`
	Stale             *bool                          `json:"stale,omitempty"`
	Sequence          *int64                         `json:"sequence,omitempty"`
	ReportedTurnID    string                         `json:"reported_turn_id,omitempty"`
	ReportedAt        *time.Time                     `json:"reported_at,omitempty"`
	PressureThreshold *float64                       `json:"pressure_threshold,omitempty"`
	Injected          *SessionContextInjectedPayload `json:"injected,omitempty"`
}

type SessionContextInjectedPayload struct {
	Estimate string                     `json:"estimate"`
	Tokens   int64                      `json:"tokens"`
	Stale    bool                       `json:"stale"`
	Rows     []SessionContextRowPayload `json:"rows"`
}

type SessionContextRowPayload struct {
	Key              string    `json:"key"`
	Label            string    `json:"label"`
	Kind             string    `json:"kind"`       // text | binary
	OwnerKind        string    `json:"owner_kind"` // full | startup_opaque
	Bytes            int64     `json:"bytes"`
	Tokens           *int64    `json:"tokens,omitempty"` // absent for binary and startup_opaque
	DeliveredTurnID  string    `json:"delivered_turn_id"`
	DeliverySequence int64     `json:"delivery_sequence"`
	SentAt           time.Time `json:"sent_at"`
	LastSeenTurnID   string    `json:"last_seen_turn_id,omitempty"`
	Unchanged        bool      `json:"unchanged"`
	Stale            bool      `json:"stale"`
	Delivery         string    `json:"delivery,omitempty"`
	HookModified     bool      `json:"hook_modified,omitempty"`
	Name             string    `json:"name,omitempty"`
}

type SessionUsageTurnsResponse struct {
	Turns       []SessionUsageTurnPayload  `json:"turns"`
	Compactions []SessionCompactionPayload `json:"compactions"`
}

type SessionUsageTurnPayload struct {
	TurnID   string                      `json:"turn_id"`
	Sequence int64                       `json:"sequence"`
	Usage    *TokenUsagePayload          `json:"usage,omitempty"`
	Injected *SessionTurnInjectedPayload `json:"injected,omitempty"`
}

type SessionTurnInjectedPayload struct {
	Estimate string                      `json:"estimate"`
	Tokens   int64                       `json:"tokens"`
	Sequence int64                       `json:"sequence"`
	SentAt   time.Time                   `json:"sent_at"`
	Spans    []SessionContextSpanPayload `json:"spans"`
}

type SessionContextSpanPayload struct {
	Key          string `json:"key"`
	Kind         string `json:"kind"`
	Bytes        int64  `json:"bytes"`
	Tokens       *int64 `json:"tokens,omitempty"`
	Unchanged    bool   `json:"unchanged"`
	StartupDedup bool   `json:"startup_dedup,omitempty"`
	Delivery     string `json:"delivery,omitempty"`
	HookModified bool   `json:"hook_modified,omitempty"`
	Name         string `json:"name,omitempty"`
}

type SessionCompactionPayload struct {
	TurnID       string    `json:"turn_id"`
	Sequence     int64     `json:"sequence"`
	At           time.Time `json:"at"`
	SpanArchived bool      `json:"span_archived"`
	FromSequence int64     `json:"from_sequence"`
	ToSequence   int64     `json:"to_sequence"`
	ContextUsed  int64     `json:"context_used"`
	ContextSize  int64     `json:"context_size"`
	Pressure     float64   `json:"pressure"`
	Strategy     string    `json:"strategy"`
}
