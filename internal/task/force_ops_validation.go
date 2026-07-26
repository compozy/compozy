package task

import (
	"fmt"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
)

func validateRecoverRunStatus(source *Run) error {
	status := source.Status.Normalize()
	if status == TaskRunStatusNeedsAttention {
		return nil
	}
	suggested := fmt.Sprintf("agh task inspect %s", source.ID)
	if status == TaskRunStatusFailed {
		suggested = fmt.Sprintf("agh task retry %s", source.ID)
	}
	return forceRunDiagnosticError(
		diagnosticcontract.CodeTaskRunNotRecoverable,
		"Task run cannot be recovered",
		fmt.Sprintf("Run %s is %s; only needs_attention runs can be recovered.", source.ID, status),
		diagnosticcontract.SeverityError,
		suggested,
		map[string]any{runEvidenceIDKey: source.ID, leaseStatusKey: status.String()},
		ErrInvalidStatusTransition,
	)
}
