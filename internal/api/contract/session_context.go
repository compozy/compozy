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

// SessionContextClearedByPayload identifies an experimental compaction freshness boundary.
type SessionContextClearedByPayload struct {
	CompactionID string `json:"compaction_id"`
	Sequence     int64  `json:"sequence"`
}

type SessionContextPayload struct {
	ClearedBy      *SessionContextClearedByPayload `json:"cleared_by,omitzero"`
	State          SessionContextState             `json:"state"`
	Used           *int64                          `json:"used,omitzero"`
	Size           *int64                          `json:"size,omitzero"`
	Ratio          *float64                        `json:"ratio,omitzero"`
	SizeSource     string                          `json:"size_source,omitempty"`
	Stale          *bool                           `json:"stale,omitzero"`
	Sequence       *int64                          `json:"sequence,omitzero"`
	ReportedTurnID string                          `json:"reported_turn_id,omitempty"`
	ReportedAt     *time.Time                      `json:"reported_at,omitempty"`
	Injected       *SessionContextInjectedPayload  `json:"injected,omitzero"`
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
	Tokens           *int64    `json:"tokens,omitzero"` // absent for binary and startup_opaque
	DeliveredTurnID  string    `json:"delivered_turn_id"`
	DeliverySequence int64     `json:"delivery_sequence"`
	SentAt           time.Time `json:"sent_at"`
	LastSeenTurnID   string    `json:"last_seen_turn_id,omitempty"`
	Unchanged        bool      `json:"unchanged"`
	Stale            bool      `json:"stale"`
	Delivery         string    `json:"delivery,omitempty"`
	HookModified     bool      `json:"hook_modified,omitzero"`
	Name             string    `json:"name,omitempty"`
}

type SessionUsageTurnsResponse struct {
	Turns       []SessionUsageTurnPayload  `json:"turns"`
	Compactions []SessionCompactionPayload `json:"compactions"`
}

type SessionUsageTurnPayload struct {
	TurnID   string                      `json:"turn_id"`
	Sequence int64                       `json:"sequence"`
	Usage    *TokenUsagePayload          `json:"usage,omitzero"`
	Injected *SessionTurnInjectedPayload `json:"injected,omitzero"`
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
	Tokens       *int64 `json:"tokens,omitzero"`
	Unchanged    bool   `json:"unchanged"`
	StartupDedup bool   `json:"startup_dedup,omitzero"`
	Delivery     string `json:"delivery,omitempty"`
	HookModified bool   `json:"hook_modified,omitzero"`
	Name         string `json:"name,omitempty"`
}

// SessionCompactionContextAfterPayload is the first experimental occupancy report after a compaction.
type SessionCompactionContextAfterPayload struct {
	Used     int64  `json:"used"`
	Size     *int64 `json:"size,omitzero"`
	Sequence int64  `json:"sequence"`
}

type SessionCompactionPayload struct {
	ContextAfter *SessionCompactionContextAfterPayload `json:"context_after,omitzero"`
	TurnID       string                                `json:"turn_id"`
	Sequence     int64                                 `json:"sequence"`
	At           time.Time                             `json:"at"`
	CompactionID string                                `json:"compaction_id"`
	Trigger      string                                `json:"trigger"`
	Status       string                                `json:"status"`
	ContextUsed  *int64                                `json:"context_used,omitzero"`
	ContextSize  *int64                                `json:"context_size,omitzero"`
}
