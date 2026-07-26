package task

import (
	"strings"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
)

func normalizePauseTaskRequest(req PauseTaskRequest) (PauseTaskRequest, error) {
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		return PauseTaskRequest{}, forceRunDiagnosticError(
			diagnosticcontract.CodeForceOpRequiresReason,
			"Task pause requires a reason",
			"Provide --reason so the pause audit event explains why new claims were stopped.",
			diagnosticcontract.SeverityError,
			"agh task pause <task-id> --reason \"incident response\"",
			nil,
			ErrForceOpRequiresReason,
		)
	}
	req.Metadata = normalizeRawJSON(req.Metadata)
	if err := ValidateMetadataSize(req.Metadata, "task_pause.metadata"); err != nil {
		return PauseTaskRequest{}, err
	}
	return req, nil
}

func normalizeResumeTaskRequest(req ResumeTaskRequest) (ResumeTaskRequest, error) {
	req.Metadata = normalizeRawJSON(req.Metadata)
	if err := ValidateMetadataSize(req.Metadata, "task_resume.metadata"); err != nil {
		return ResumeTaskRequest{}, err
	}
	return req, nil
}

func actorLabel(actor ActorContext) string {
	kind := strings.TrimSpace(string(actor.Actor.Kind.Normalize()))
	ref := strings.TrimSpace(actor.Actor.Ref)
	if kind == "" {
		return ref
	}
	if ref == "" {
		return kind
	}
	return kind + ":" + ref
}
