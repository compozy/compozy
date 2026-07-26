package task

import (
	"context"

	"fmt"
	"strings"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
)

func requireForceFailStatus(run Run) error {
	switch run.Status.Normalize() {
	case TaskRunStatusQueued, TaskRunStatusClaimed:
		return nil
	case TaskRunStatusCompleted, TaskRunStatusFailed, TaskRunStatusCanceled:
		return forceRunDiagnosticError(
			diagnosticcontract.CodeTaskRunAlreadyTerminal,
			"Task run is already terminal",
			fmt.Sprintf("Run %s is already %s and cannot be force failed.", run.ID, run.Status.Normalize()),
			diagnosticcontract.SeverityInfo,
			fmt.Sprintf("agh task inspect %s", run.ID),
			map[string]any{runEvidenceIDKey: run.ID, leaseStatusKey: run.Status.Normalize().String()},
			ErrInvalidStatusTransition,
		)
	default:
		return forceRunDiagnosticError(
			diagnosticcontract.CodeTaskRunStillActive,
			"Task run is still active",
			fmt.Sprintf(
				"Run %s is %s; active runs must be stopped before forced failure.",
				run.ID,
				run.Status.Normalize(),
			),
			diagnosticcontract.SeverityError,
			fmt.Sprintf("agh task cancel %s --reason %q", run.ID, "stop before force fail"),
			map[string]any{runEvidenceIDKey: run.ID, leaseStatusKey: run.Status.Normalize().String()},
			ErrInvalidStatusTransition,
		)
	}
}

func (m *Service) requireRetryChainDepth(ctx context.Context, source Run) error {
	runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: source.TaskID})
	if err != nil {
		return err
	}
	byID := make(map[string]Run, len(runs))
	for _, run := range runs {
		byID[run.ID] = run
	}
	depth := 0
	for current := source; strings.TrimSpace(current.PreviousRunID) != ""; {
		depth++
		if depth >= MaxRetryRunChainDepth {
			return forceRunDiagnosticError(
				diagnosticcontract.CodeRetryChainTooDeep,
				"Retry chain is too deep",
				fmt.Sprintf("Run %s already has retry depth %d.", source.ID, depth),
				diagnosticcontract.SeverityError,
				fmt.Sprintf("agh task inspect %s", source.TaskID),
				map[string]any{runEvidenceIDKey: source.ID, taskEvidenceIDKey: source.TaskID, "depth": depth},
				ErrRetryChainTooDeep,
			)
		}
		previous, ok := byID[strings.TrimSpace(current.PreviousRunID)]
		if !ok {
			break
		}
		current = previous
	}
	return nil
}

func normalizeForceReleaseRun(req ForceReleaseRun) (ForceReleaseRun, error) {
	req.Reason = strings.TrimSpace(req.Reason)
	req.Metadata = normalizeRawJSON(req.Metadata)
	if err := ValidateMetadataSize(req.Metadata, "force_release.metadata"); err != nil {
		return ForceReleaseRun{}, err
	}
	return req, nil
}

func normalizeForceFailRun(req ForceFailRun) (ForceFailRun, error) {
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		return ForceFailRun{}, forceRunDiagnosticError(
			diagnosticcontract.CodeForceOpRequiresReason,
			"Force fail requires a reason",
			"Provide --reason so the recovery audit event explains why the run was failed.",
			diagnosticcontract.SeverityError,
			"agh task fail <run-id> --reason \"operator recovery\"",
			nil,
			ErrForceOpRequiresReason,
		)
	}
	req.Metadata = normalizeRawJSON(req.Metadata)
	if err := ValidateMetadataSize(req.Metadata, "force_fail.metadata"); err != nil {
		return ForceFailRun{}, err
	}
	return req, nil
}

func normalizeRetryRunRequest(req RetryRunRequest) (RetryRunRequest, error) {
	req.Metadata = normalizeRawJSON(req.Metadata)
	if err := ValidateMetadataSize(req.Metadata, "retry_run.metadata"); err != nil {
		return RetryRunRequest{}, err
	}
	return req, nil
}

func normalizeRecoverRunRequest(req RecoverRunRequest) (RecoverRunRequest, error) {
	req.Reason = strings.TrimSpace(req.Reason)
	req.Metadata = normalizeRawJSON(req.Metadata)
	if err := ValidateMetadataSize(req.Metadata, "recover_run.metadata"); err != nil {
		return RecoverRunRequest{}, err
	}
	return req, nil
}

func normalizeBulkForceRunRequest(req BulkForceRunRequest, requireReason bool) (BulkForceRunRequest, error) {
	if len(req.RunIDs) == 0 {
		return BulkForceRunRequest{}, fmt.Errorf("%w: bulk force run_ids is required", ErrValidation)
	}
	if len(req.RunIDs) > MaxForceRunBulkIDs {
		return BulkForceRunRequest{}, forceRunDiagnosticError(
			diagnosticcontract.CodeBulkTooLarge,
			"Bulk force operation is too large",
			fmt.Sprintf("Bulk force operations accept at most %d run ids.", MaxForceRunBulkIDs),
			diagnosticcontract.SeverityError,
			"",
			map[string]any{"limit": MaxForceRunBulkIDs, "count": len(req.RunIDs)},
			ErrBulkTooLarge,
		)
	}
	seen := make(map[string]struct{}, len(req.RunIDs))
	ids := make([]string, 0, len(req.RunIDs))
	for _, id := range req.RunIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			return BulkForceRunRequest{}, fmt.Errorf(
				"%w: bulk force run_ids must not contain empty values",
				ErrValidation,
			)
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		ids = append(ids, trimmed)
	}
	req.RunIDs = ids
	req.Reason = strings.TrimSpace(req.Reason)
	if requireReason && req.Reason == "" {
		return BulkForceRunRequest{}, forceRunDiagnosticError(
			diagnosticcontract.CodeForceOpRequiresReason,
			"Force fail requires a reason",
			"Provide a reason so every failed row has clear audit evidence.",
			diagnosticcontract.SeverityError,
			"agh task fail <run-id...> --reason \"operator recovery\"",
			nil,
			ErrForceOpRequiresReason,
		)
	}
	req.Metadata = normalizeRawJSON(req.Metadata)
	if err := ValidateMetadataSize(req.Metadata, "bulk_force.metadata"); err != nil {
		return BulkForceRunRequest{}, err
	}
	return req, nil
}
