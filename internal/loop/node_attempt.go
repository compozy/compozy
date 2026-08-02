package loop

import "time"

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
	LoopRunID     RunID
	Generation    int
	NodeID        NodeID
	ItemIndex     int
	Attempt       int
	FailureClass  *FailureClass
	FailureCode   string
	Cause         string
	Hint          string
	Target        string
	Disposition   AttemptDisposition
	StartedAt     time.Time
	EndedAt       *time.Time
	NextAttemptAt *time.Time
}
