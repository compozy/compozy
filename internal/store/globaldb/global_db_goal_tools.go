package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ goal.ToolStore = (*GoalRepo)(nil)

// FindGoalReportTarget returns the only currently prompting Goal bound to the session.
func (g *GoalRepo) FindGoalReportTarget(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
) (goal.ToolReportTarget, bool, error) {
	if err := g.checkReady(ctx, "find Goal report target"); err != nil {
		return goal.ToolReportTarget{}, false, err
	}
	return findGoalReportTargetWithExecutor(ctx, g.db, workspaceID, sessionID)
}

func findGoalReportTargetWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
) (target goal.ToolReportTarget, found bool, err error) {
	workspaceID = looppkg.WorkspaceID(strings.TrimSpace(string(workspaceID)))
	sessionID = strings.TrimSpace(sessionID)
	if workspaceID == "" || sessionID == "" {
		return goal.ToolReportTarget{}, false, fmt.Errorf(
			"%w: Goal report workspace_id and session_id are required",
			looppkg.ErrValidation,
		)
	}
	rows, err := sqlcgen.New(exec).FindGoalReportTargets(ctx, sqlcgen.FindGoalReportTargetsParams{
		WorkspaceID: string(workspaceID), SessionID: goalNullableString(sessionID),
	})
	if err != nil {
		return goal.ToolReportTarget{}, false, fmt.Errorf("store: query Goal report target: %w", err)
	}
	for _, row := range rows {
		candidate := goal.ToolReportTarget{
			Key: goal.TurnKey{WorkspaceID: workspaceID, LoopRunID: looppkg.RunID(row.LoopRunID),
				Generation: int(row.Generation), NodeID: looppkg.NodeID(row.NodeID), ItemIndex: int(row.ItemIndex)},
			ExpectedControlEpoch: row.ControlEpoch, ExpectedBindingEpoch: row.BindingEpoch.Int64,
			PromptID: row.PromptID.String, OriginSessionID: row.OriginSessionID.String,
			BoundSessionID: row.SessionID.String,
		}
		if validateErr := candidate.Validate(); validateErr != nil {
			return goal.ToolReportTarget{}, false, validateErr
		}
		if found {
			return goal.ToolReportTarget{}, false, fmt.Errorf(
				"store: multiple active Goal report targets for session %q",
				sessionID,
			)
		}
		target = candidate
		found = true
	}
	return target, found, nil
}

// ResolveActiveGoalOriginAlias resolves an active moved binding back to its origin session.
func (g *GoalRepo) ResolveActiveGoalOriginAlias(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
) (originSessionID string, found bool, err error) {
	if err := g.checkReady(ctx, "resolve active Goal origin alias"); err != nil {
		return "", false, err
	}
	workspaceID = looppkg.WorkspaceID(strings.TrimSpace(string(workspaceID)))
	sessionID = strings.TrimSpace(sessionID)
	if workspaceID == "" || sessionID == "" {
		return "", false, fmt.Errorf(
			"%w: Goal alias workspace_id and session_id are required",
			looppkg.ErrValidation,
		)
	}
	rows, err := sqlcgen.New(g.db).ResolveActiveGoalOriginAliases(ctx, sqlcgen.ResolveActiveGoalOriginAliasesParams{
		WorkspaceID: string(workspaceID), SessionID: sessionID,
	})
	if err != nil {
		return "", false, fmt.Errorf("store: query active Goal origin alias: %w", err)
	}
	for _, row := range rows {
		candidate := strings.TrimSpace(row.String)
		if candidate == "" {
			return "", false, fmt.Errorf("%w: Goal origin session is empty", looppkg.ErrValidation)
		}
		if found {
			return "", false, fmt.Errorf(
				"store: multiple active Goal origin aliases for session %q",
				sessionID,
			)
		}
		originSessionID = candidate
		found = true
	}
	return originSessionID, found, nil
}

// RecordGoalReport content-addresses evidence and records its prompt-bound intent atomically.
func (g *GoalRepo) RecordGoalReport(
	ctx context.Context,
	req goal.RecordToolReportRequest,
) (goal.ReportIntent, error) {
	if err := g.checkReady(ctx, "record Goal tool report"); err != nil {
		return goal.ReportIntent{}, err
	}
	payload, evidenceRef, err := normalizeGoalToolReportRequest(&req)
	if err != nil {
		return goal.ReportIntent{}, err
	}
	now := g.now()
	var intent goal.ReportIntent
	err = g.withTaskImmediateTransaction(ctx, "record Goal tool report", func(exec taskSQLExecutor) error {
		current, found, err := findGoalReportTargetWithExecutor(
			ctx,
			exec,
			req.Target.Key.WorkspaceID,
			req.Target.BoundSessionID,
		)
		if err != nil {
			return err
		}
		if !found || !goalToolReportTargetEqual(current, req.Target) {
			return goalNotActiveError("report target is no longer current")
		}
		if len(payload) > 0 {
			if err := upsertLoopOutputBlobWithExecutor(ctx, exec, evidenceRef, payload, now); err != nil {
				return err
			}
		}
		intent, err = recordReportIntentWithExecutor(ctx, exec, goal.RecordReportIntentRequest{
			Key: req.Target.Key, ExpectedControlEpoch: req.Target.ExpectedControlEpoch,
			ExpectedBindingEpoch: req.Target.ExpectedBindingEpoch, PromptID: req.Target.PromptID,
			Status: req.Status, EvidenceRef: evidenceRef, ActorKind: req.ActorKind, ActorID: req.ActorID,
		}, now)
		return err
	})
	if err != nil {
		return goal.ReportIntent{}, normalizeGoalToolReportError(err)
	}
	return intent, nil
}

func normalizeGoalToolReportRequest(req *goal.RecordToolReportRequest) (json.RawMessage, string, error) {
	if req == nil {
		return nil, "", fmt.Errorf("%w: Goal tool report request is required", looppkg.ErrValidation)
	}
	req.Target.Key.WorkspaceID = looppkg.WorkspaceID(strings.TrimSpace(string(req.Target.Key.WorkspaceID)))
	req.Target.Key.LoopRunID = looppkg.RunID(strings.TrimSpace(string(req.Target.Key.LoopRunID)))
	req.Target.Key.NodeID = looppkg.NodeID(strings.TrimSpace(string(req.Target.Key.NodeID)))
	req.Target.PromptID = strings.TrimSpace(req.Target.PromptID)
	req.Target.OriginSessionID = strings.TrimSpace(req.Target.OriginSessionID)
	req.Target.BoundSessionID = strings.TrimSpace(req.Target.BoundSessionID)
	req.Status = strings.TrimSpace(req.Status)
	req.ActorKind = strings.TrimSpace(req.ActorKind)
	req.ActorID = strings.TrimSpace(req.ActorID)
	if err := req.Target.Validate(); err != nil {
		return nil, "", err
	}
	if len(req.Evidence) > goal.MaxReportEvidenceBytes {
		return nil, "", &looppkg.ReasonError{
			Code: looppkg.ReasonCodeGoalEvidenceTooLarge,
			Err: fmt.Errorf(
				"%w: Goal report evidence exceeds %d bytes",
				looppkg.ErrValidation,
				goal.MaxReportEvidenceBytes,
			),
		}
	}
	req.Evidence = strings.TrimSpace(req.Evidence)
	if (req.Status != goalReportStatusComplete && req.Status != goalReportStatusBlocked) ||
		req.ActorKind == "" || req.ActorID == "" ||
		(req.Status == goalReportStatusBlocked && req.Evidence == "") {
		return nil, "", fmt.Errorf("%w: Goal tool report is invalid", looppkg.ErrValidation)
	}
	if req.Evidence == "" {
		return nil, "", nil
	}
	payload, err := json.Marshal(req.Evidence)
	if err != nil {
		return nil, "", fmt.Errorf("store: encode Goal report evidence: %w", err)
	}
	raw := json.RawMessage(payload)
	return raw, looppkg.OutputRefForPayload(raw), nil
}

func goalToolReportTargetEqual(left goal.ToolReportTarget, right goal.ToolReportTarget) bool {
	return left.Key == right.Key &&
		left.ExpectedControlEpoch == right.ExpectedControlEpoch &&
		left.ExpectedBindingEpoch == right.ExpectedBindingEpoch &&
		left.PromptID == right.PromptID &&
		left.OriginSessionID == right.OriginSessionID &&
		left.BoundSessionID == right.BoundSessionID
}

func normalizeGoalToolReportError(err error) error {
	var reasonErr *looppkg.ReasonError
	if !errors.As(err, &reasonErr) {
		return err
	}
	switch reasonErr.Code {
	case looppkg.ReasonCodeContinuousBindingMismatch, looppkg.ReasonCodeGoalControlStale:
		return goalNotActiveError("report target was revoked before persistence")
	default:
		return err
	}
}
