package loop

import (
	"encoding/json"
	"time"
)

// CancelState is the durable graceful-cancellation state machine.
type CancelState string

const (
	CancelStateNone       CancelState = ""
	CancelStateRequested  CancelState = "requested"
	CancelStateDelivering CancelState = "delivering"
	CancelStateDraining   CancelState = "draining"
	CancelStateCanceled   CancelState = "canceled"
)

// Valid reports whether value belongs to the closed cancel-state vocabulary.
func (s CancelState) Valid() bool {
	switch s {
	case CancelStateNone,
		CancelStateRequested,
		CancelStateDelivering,
		CancelStateDraining,
		CancelStateCanceled:
		return true
	default:
		return false
	}
}

// ControlProvenance attributes one durable node-control mutation.
type ControlProvenance struct {
	ActorKind   string
	ActorID     string
	Reason      string
	RuleID      string
	RequestedAt time.Time
}

// NodeControl is the cross-generation control state for one authored node.
type NodeControl struct {
	LoopRunID               RunID
	NodeID                  NodeID
	Paused                  bool
	PauseProvenance         *ControlProvenance
	Quarantined             bool
	QuarantineEntry         json.RawMessage
	QuarantinedAt           *time.Time
	AttentionFlag           string
	AttentionReason         string
	AttentionProducerNodeID string
	CancelState             CancelState
	CancelProvenance        *ControlProvenance
	LastEvidenceAt          *time.Time
	DeathResumeStreak       int
	GateRevisions           map[int]int
	Revision                int64
	UpdatedAt               time.Time
}
