package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	looppkg "github.com/compozy/agh/internal/loop"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type nativeGoalRuntime interface {
	findGoalReportTarget(
		context.Context,
		looppkg.WorkspaceID,
		string,
	) (goalpkg.ToolReportTarget, bool, error)
	resolveActiveGoalOriginAlias(context.Context, looppkg.WorkspaceID, string) (string, bool, error)
	recordGoalReport(context.Context, goalpkg.RecordToolReportRequest) (goalpkg.ReportIntent, error)
}

type nativeGoalReportResult struct {
	Status      string    `json:"status"`
	EvidenceRef *string   `json:"evidence_ref"`
	PromptID    string    `json:"prompt_id"`
	RecordedAt  time.Time `json:"recorded_at"`
}

func (n *daemonNativeTools) goalGet(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopWorkspaceInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeLoopWorkspaceID(req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := nativeGoalSessionID(req.ToolID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	snapshot, err := n.visibleGoalSnapshot(ctx, workspaceID, sessionID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeGoalToolError(req.ToolID, err)
	}
	if snapshot == nil {
		return toolspkg.ToolResult{}, nativeGoalToolError(
			req.ToolID,
			nativeGoalNotActiveError("caller session has no visible Goal"),
		)
	}
	payload, err := core.GoalSnapshotPayloadFromSession(snapshot)
	if err != nil {
		return toolspkg.ToolResult{}, nativeGoalToolError(req.ToolID, err)
	}
	return structuredResult(
		contract.SessionGoalResponse{Goal: payload},
		fmt.Sprintf("Goal %s is %s", snapshot.RunID, snapshot.Status),
	)
}

func (n *daemonNativeTools) goalReport(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeGoalReportInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeLoopWorkspaceID(req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := nativeGoalSessionID(req.ToolID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	runtime := n.goalRuntime()
	if runtime == nil {
		return toolspkg.ToolResult{}, nativeGoalToolError(
			req.ToolID,
			errors.New("daemon: Goal tool runtime is unavailable"),
		)
	}
	target, found, err := runtime.findGoalReportTarget(
		ctx,
		looppkg.WorkspaceID(workspaceID),
		sessionID,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeGoalToolError(req.ToolID, err)
	}
	if !found {
		return toolspkg.ToolResult{}, nativeGoalToolError(
			req.ToolID,
			nativeGoalNotActiveError("caller session is not the current Goal prompt binding"),
		)
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeGoalToolError(req.ToolID, err)
	}
	intent, err := runtime.recordGoalReport(ctx, goalpkg.RecordToolReportRequest{
		Target: target, Status: input.Status, Evidence: input.Evidence,
		ActorKind: string(actor.Actor.Kind), ActorID: actor.Actor.Ref,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeGoalToolError(req.ToolID, err)
	}
	var evidenceRef *string
	if intent.EvidenceRef != "" {
		value := intent.EvidenceRef
		evidenceRef = &value
	}
	return structuredResult(nativeGoalReportResult{
		Status: intent.Status, EvidenceRef: evidenceRef,
		PromptID: intent.PromptID, RecordedAt: intent.RecordedAt,
	}, fmt.Sprintf("Goal report %s recorded", intent.Status))
}

func (n *daemonNativeTools) loopTurns(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopTurnsInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, err := nativeLoopWorkspaceAndRunID(
		req.ToolID,
		input.WorkspaceID,
		input.RunID,
		scope,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.NodeID != "" && strings.TrimSpace(input.NodeID) == "" {
		return toolspkg.ToolResult{}, nativeGoalToolError(
			req.ToolID,
			fmt.Errorf("%w: Goal turn node must be nonempty when present", looppkg.ErrValidation),
		)
	}
	page, err := n.loopService().ListGoalTurns(ctx, workspaceID, runID, core.GoalTurnListQuery{
		NodeID: strings.TrimSpace(input.NodeID), ItemIndex: input.ItemIndex,
		AfterSeq: input.AfterSeq, Limit: input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeGoalToolError(req.ToolID, err)
	}
	payload, err := core.GoalTurnPagePayloadFromSession(page)
	if err != nil {
		return toolspkg.ToolResult{}, nativeGoalToolError(req.ToolID, err)
	}
	return structuredResult(payload, fmt.Sprintf("%d Goal turns", len(payload.Turns)))
}

func (n *daemonNativeTools) goalGetAvailability(
	ctx context.Context,
	scope toolspkg.Scope,
) toolspkg.Availability {
	if n.loopService() == nil {
		return toolspkg.Unavailable(toolspkg.ReasonDependencyMissing)
	}
	workspaceID, sessionID, ok := nativeGoalScope(scope)
	if !ok {
		return toolspkg.Unavailable(toolspkg.ReasonSessionDenied)
	}
	snapshot, err := n.visibleGoalSnapshot(ctx, workspaceID, sessionID)
	if err != nil {
		if nativeGoalNotActive(err) {
			return toolspkg.Unavailable(toolspkg.ReasonGoalNotActive)
		}
		return toolspkg.Unavailable(toolspkg.ReasonBackendUnhealthy)
	}
	if snapshot == nil {
		return toolspkg.Unavailable(toolspkg.ReasonGoalNotActive)
	}
	return toolspkg.Available()
}

func (n *daemonNativeTools) goalReportAvailability(
	ctx context.Context,
	scope toolspkg.Scope,
) toolspkg.Availability {
	runtime := n.goalRuntime()
	if runtime == nil {
		return toolspkg.Unavailable(toolspkg.ReasonDependencyMissing)
	}
	workspaceID, sessionID, ok := nativeGoalScope(scope)
	if !ok {
		return toolspkg.Unavailable(toolspkg.ReasonSessionDenied)
	}
	_, found, err := runtime.findGoalReportTarget(ctx, looppkg.WorkspaceID(workspaceID), sessionID)
	if err != nil {
		return toolspkg.Unavailable(toolspkg.ReasonBackendUnhealthy)
	}
	if !found {
		return toolspkg.Unavailable(toolspkg.ReasonGoalNotActive)
	}
	return toolspkg.Available()
}

func (n *daemonNativeTools) visibleGoalSnapshot(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.GoalSnapshot, error) {
	service := n.loopService()
	if service == nil {
		return nil, errors.New("daemon: Loop service is unavailable")
	}
	snapshot, err := service.GetSessionGoal(ctx, workspaceID, sessionID)
	if err != nil || snapshot != nil {
		return snapshot, err
	}
	runtime := n.goalRuntime()
	if runtime == nil {
		return nil, nil
	}
	originSessionID, found, err := runtime.resolveActiveGoalOriginAlias(
		ctx,
		looppkg.WorkspaceID(workspaceID),
		sessionID,
	)
	if err != nil || !found {
		return nil, err
	}
	return service.GetSessionGoal(ctx, workspaceID, originSessionID)
}

func (n *daemonNativeTools) goalRuntime() nativeGoalRuntime {
	service := n.loopService()
	if service == nil {
		return nil
	}
	runtime, ok := service.(nativeGoalRuntime)
	if !ok {
		return nil
	}
	return runtime
}

func nativeGoalScope(scope toolspkg.Scope) (string, string, bool) {
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	sessionID := strings.TrimSpace(scope.SessionID)
	return workspaceID, sessionID, workspaceID != "" && sessionID != ""
}

func nativeGoalSessionID(id toolspkg.ToolID, scope toolspkg.Scope) (string, error) {
	sessionID := strings.TrimSpace(scope.SessionID)
	if sessionID != "" {
		return sessionID, nil
	}
	return "", toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		fmt.Sprintf("tool %q requires a caller session", id),
		toolspkg.ErrToolDenied,
		toolspkg.ReasonSessionDenied,
	)
}

func nativeGoalToolError(id toolspkg.ToolID, err error) error {
	if err == nil {
		return nil
	}
	if reasonErr, ok := errors.AsType[*looppkg.ReasonError](err); ok {
		switch reasonErr.Code {
		case looppkg.ReasonCodeGoalNotActive:
			return toolspkg.NewToolError(
				toolspkg.ErrorCodeUnavailable, id, err.Error(),
				fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, err),
				toolspkg.ReasonGoalNotActive,
			)
		case looppkg.ReasonCodeGoalReportConflict:
			return toolspkg.NewToolError(
				toolspkg.ErrorCodeConflict, id, err.Error(),
				fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
				toolspkg.ReasonGoalReportConflict,
			)
		case looppkg.ReasonCodeGoalEvidenceTooLarge:
			return toolspkg.NewToolError(
				toolspkg.ErrorCodeInvalidInput, id, err.Error(),
				fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
				toolspkg.ReasonGoalEvidenceTooLarge,
			)
		}
	}
	return nativeLoopToolError(id, err)
}

func nativeGoalNotActive(err error) bool {
	reasonErr, ok := errors.AsType[*looppkg.ReasonError](err)
	return ok && reasonErr.Code == looppkg.ReasonCodeGoalNotActive
}

func nativeGoalNotActiveError(detail string) error {
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeGoalNotActive,
		Err:  fmt.Errorf("%w: %s", looppkg.ErrTransitionConflict, strings.TrimSpace(detail)),
	}
}

type nativeGoalReportInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Status      string `json:"status"`
	Evidence    string `json:"evidence,omitempty"`
}

type nativeLoopTurnsInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	RunID       string `json:"run_id"`
	NodeID      string `json:"node,omitempty"`
	ItemIndex   *int   `json:"item,omitempty"`
	AfterSeq    int64  `json:"after_seq,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}
