package loop

import (
	"time"

	"github.com/compozy/compozy/internal/tools"
)

// FailureClass is the closed node-failure vocabulary.
type FailureClass string

const (
	FailureTransport         FailureClass = "transport"
	FailurePayloadDeclared   FailureClass = "payload_declared"
	FailureQualityRejection  FailureClass = "quality_rejection"
	FailureAuthoring         FailureClass = "authoring"
	FailureCancellation      FailureClass = "cancellation"
	FailureAttemptTimeout    FailureClass = "attempt_timeout"
	FailureBudgetExhausted   FailureClass = "budget_exhausted"
	FailureTargetUnavailable FailureClass = "target_unavailable"
)

// IsKnownFailureClass reports whether value belongs to the closed failure vocabulary.
func IsKnownFailureClass(value FailureClass) bool {
	switch value {
	case FailureTransport,
		FailurePayloadDeclared,
		FailureQualityRejection,
		FailureAuthoring,
		FailureCancellation,
		FailureAttemptTimeout,
		FailureBudgetExhausted,
		FailureTargetUnavailable:
		return true
	default:
		return false
	}
}

// RetryEligible reports whether the class may use automatic node retry.
func (c FailureClass) RetryEligible() bool {
	return c == FailureTransport || c == FailureAttemptTimeout
}

// ClassifiedFailure is the canonical operator-safe failure projection.
type ClassifiedFailure struct {
	Class         FailureClass   `json:"class"`
	Code          string         `json:"code"`
	Cause         string         `json:"cause"`
	Hint          string         `json:"hint,omitempty"`
	Target        string         `json:"target,omitempty"`
	RetryAfter    *time.Duration `json:"retry_after,omitempty"`
	RetryEligible bool           `json:"retry_eligible"`
}

// FailureEvidence contains immutable facts observed at the coordinator boundary.
// Text fields must already have crossed NewActionFailure's sanitization boundary.
type FailureEvidence struct {
	CancelRequested   bool
	PayloadFailure    *ActionFailure
	QualityRejected   bool
	Authoring         bool
	AttemptTimedOut   bool
	BudgetExhausted   bool
	TargetUnavailable bool
	Transport         bool
	LeaseExpired      bool
	ToolError         *tools.ToolError
	Code              string
	Cause             string
	Hint              string
	Target            string
	RetryAfter        *time.Duration
	BackoffMax        time.Duration
}
