package loop

import (
	"encoding/json"
	"time"
)

// WaitClaimState is the closed durable wait-claim vocabulary.
type WaitClaimState string

const (
	WaitClaimWaiting              WaitClaimState = "waiting"
	WaitClaimClaimed              WaitClaimState = "claimed"
	WaitClaimResumed              WaitClaimState = "resumed"
	WaitClaimInterventionRequired WaitClaimState = "intervention_required"
)

// Valid reports whether value belongs to the closed wait-claim vocabulary.
func (s WaitClaimState) Valid() bool {
	switch s {
	case WaitClaimWaiting, WaitClaimClaimed, WaitClaimResumed, WaitClaimInterventionRequired:
		return true
	default:
		return false
	}
}

// NodeWait is one durable pause point for a node cell.
type NodeWait struct {
	LoopRunID         RunID
	Generation        int
	NodeID            NodeID
	ItemIndex         int
	Kind              string
	ResumeAt          *time.Time
	NextEscalationAt  *time.Time
	EscalationCursor  int
	ClaimState        WaitClaimState
	ClaimedByKind     string
	ClaimedByID       string
	ClaimedAt         *time.Time
	AdmissionFailures int
	Expect            json.RawMessage
	AheadPayload      json.RawMessage
	IssuedEpoch       int64
	CreatedAt         time.Time
}
