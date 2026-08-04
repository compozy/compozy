package loop

import (
	"fmt"
	"strings"
	"time"
)

// AttemptDisposition is the closed durable outcome vocabulary for one node attempt.
type AttemptDisposition string

const (
	AttemptSucceeded   AttemptDisposition = "succeeded"
	AttemptRetried     AttemptDisposition = "retried"
	AttemptRouted      AttemptDisposition = "routed"
	AttemptAbsorbed    AttemptDisposition = "absorbed"
	AttemptEscalated   AttemptDisposition = "escalated"
	AttemptQuarantined AttemptDisposition = "quarantined"
	AttemptCanceled    AttemptDisposition = "canceled"
	AttemptResumed     AttemptDisposition = "resumed"
)

// Valid reports whether value belongs to the closed attempt-disposition vocabulary.
func (d AttemptDisposition) Valid() bool {
	switch d {
	case AttemptSucceeded,
		AttemptRetried,
		AttemptRouted,
		AttemptAbsorbed,
		AttemptEscalated,
		AttemptQuarantined,
		AttemptCanceled,
		AttemptResumed:
		return true
	default:
		return false
	}
}

// NodeAttempt is one immutable attempt-ledger row.
type NodeAttempt struct {
	LoopRunID     RunID              `json:"loop_run_id"`
	Generation    int                `json:"generation"`
	NodeID        NodeID             `json:"node_id"`
	ItemIndex     int                `json:"item_index"`
	Attempt       int                `json:"attempt"`
	FailureClass  *FailureClass      `json:"failure_class,omitempty"`
	FailureCode   string             `json:"failure_code,omitempty"`
	Cause         string             `json:"cause,omitempty"`
	Hint          string             `json:"hint,omitempty"`
	Target        string             `json:"target,omitempty"`
	Disposition   AttemptDisposition `json:"disposition"`
	StartedAt     time.Time          `json:"started_at"`
	EndedAt       *time.Time         `json:"ended_at,omitempty"`
	NextAttemptAt *time.Time         `json:"next_attempt_at,omitempty"`
}

func (a NodeAttempt) normalized(loopRunID RunID) NodeAttempt {
	if a.LoopRunID == "" {
		a.LoopRunID = loopRunID
	}
	a.NodeID = NodeID(strings.TrimSpace(string(a.NodeID)))
	a.FailureCode = strings.TrimSpace(a.FailureCode)
	a.Cause = strings.TrimSpace(a.Cause)
	a.Hint = strings.TrimSpace(a.Hint)
	a.Target = strings.TrimSpace(a.Target)
	if !a.StartedAt.IsZero() {
		a.StartedAt = a.StartedAt.UTC()
	}
	if a.EndedAt != nil {
		value := a.EndedAt.UTC()
		a.EndedAt = &value
	}
	if a.NextAttemptAt != nil {
		value := a.NextAttemptAt.UTC()
		a.NextAttemptAt = &value
	}
	return a
}

func (a NodeAttempt) validate(loopRunID RunID, generation int) error {
	if a.Generation < 1 || generation < 1 {
		return fmt.Errorf("%w: node attempt generation must be positive", ErrValidation)
	}
	if a.LoopRunID != loopRunID || a.Generation != generation {
		return fmt.Errorf("%w: node attempt snapshot identity does not match generation", ErrValidation)
	}
	if a.NodeID == "" || a.ItemIndex < 0 || a.Attempt < 1 || a.StartedAt.IsZero() {
		return fmt.Errorf("%w: node attempt identity and started_at are required", ErrValidation)
	}
	if !a.Disposition.Valid() {
		return fmt.Errorf("%w: node attempt disposition is invalid: %q", ErrValidation, a.Disposition)
	}
	if a.FailureClass != nil && !IsKnownFailureClass(*a.FailureClass) {
		return fmt.Errorf("%w: node attempt failure_class is invalid: %q", ErrValidation, *a.FailureClass)
	}
	if a.Disposition == AttemptRetried && a.NextAttemptAt == nil {
		return fmt.Errorf("%w: retried node attempt requires next_attempt_at", ErrValidation)
	}
	return nil
}
