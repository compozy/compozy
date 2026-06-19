package recovery

import (
	"context"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/workspace"
)

// VerdictDecision is the constrained remediation outcome emitted by the
// recovery agent.
type VerdictDecision string

const (
	// VerdictReject means the agent did not apply a project-side fix.
	VerdictReject VerdictDecision = "reject"
	// VerdictFixed means the agent applied a project-side fix and the caller may
	// restart failed jobs.
	VerdictFixed VerdictDecision = "fixed"
)

// TriageVerdict is parsed from the recovery agent's constrained JSON output.
type TriageVerdict struct {
	Decision     VerdictDecision `json:"decision"`
	Reason       string          `json:"reason"`
	ChangedFiles []string        `json:"changed_files"`
}

// RemediationInput is the recovery context supplied by the orchestrator.
type RemediationInput struct {
	Outcome      RunOutcome
	FailedConfig *model.RuntimeConfig
	Recovery     workspace.AgentRecoveryConfig
}

// RemediationStrategy attempts to remediate one failed run outcome.
type RemediationStrategy interface {
	Name() string
	Remediate(ctx context.Context, in RemediationInput) (TriageVerdict, error)
}

func rejectVerdict(reason string) TriageVerdict {
	if reason == "" {
		reason = "recovery agent did not return a valid fixed verdict"
	}
	return TriageVerdict{
		Decision:     VerdictReject,
		Reason:       reason,
		ChangedFiles: []string{},
	}
}
