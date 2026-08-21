package loop

import (
	"context"
	"fmt"
	"time"
)

const (
	// AttentionSilence reports ambiguous loss of liveness evidence without declaring death.
	AttentionSilence = "silence"
	// AttentionWaitIntervention reports a node parked for explicit operator recovery.
	AttentionWaitIntervention = "wait_intervention"
)

// NodeLivenessObservation records one evidence probe for a live node attempt.
type NodeLivenessObservation struct {
	WorkspaceID  WorkspaceID
	LoopRunID    RunID
	TaskRunID    string
	ObservedAt   time.Time
	Evidence     bool
	SilenceAfter time.Duration
}

// Validate rejects observations that cannot be scoped to one live cell.
func (o NodeLivenessObservation) Validate() error {
	if o.WorkspaceID == "" || o.LoopRunID == "" || o.TaskRunID == "" || o.ObservedAt.IsZero() ||
		o.SilenceAfter < 0 {
		return fmt.Errorf("%w: node liveness observation is incomplete", ErrValidation)
	}
	return nil
}

// NodeLivenessRecorder persists evidence and silence attention through one CAS authority.
type NodeLivenessRecorder interface {
	RecordNodeLiveness(context.Context, NodeLivenessObservation) error
}
