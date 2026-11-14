package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/sessioncleanup"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	// sessionCleanupDefaultDays matches the range the archived view preselects.
	sessionCleanupDefaultDays = 7
	sessionCleanupMaxDays     = 36500
)

type sessionCleanupInput struct {
	Action         string                                  `json:"action,omitempty"`
	Workspace      string                                  `json:"workspace,omitempty"`
	AllWorkspaces  bool                                    `json:"all_workspaces,omitempty"`
	Delete         contract.SessionHistoryCleanupSelection `json:"delete"`
	OlderThanDays  *int                                    `json:"older_than_days,omitempty"`
	ArchivedBefore *time.Time                              `json:"archived_before,omitempty"`
	AnyTime        bool                                    `json:"any_time,omitempty"`
	SessionIDs     []string                                `json:"session_ids,omitempty"`
	CountedAt      *time.Time                              `json:"counted_at,omitempty"`
	DryRun         *bool                                   `json:"dry_run,omitempty"`
}

type sessionCleanupPlanResult struct {
	DryRun bool                               `json:"dry_run"`
	Plan   contract.SessionHistoryCleanupPlan `json:"plan"`
	Run    *contract.SessionHistoryCleanupRun `json:"run,omitempty"`
}

// sessionCleanup previews, starts, reads, or stops the caller profile's
// archived-history cleanup. The registry asks for approval only when the call
// deletes (action plan with dry_run=false).
func (n *daemonNativeTools) sessionCleanup(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionCleanupInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	cleaner, ok := n.deps.Sessions.(core.SessionHistoryCleanupManager)
	if !ok {
		return toolspkg.ToolResult{}, sessionCleanupError(req.ToolID, session.ErrSessionArchiveUnavailable)
	}
	profileID, _, _, err := n.nativeCurrentProfileIdentity(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	switch strings.TrimSpace(input.Action) {
	case "", toolspkg.SessionCleanupActionPlan:
		return n.sessionCleanupPlan(ctx, scope, req.ToolID, cleaner, profileID, input)
	case toolspkg.SessionCleanupActionStatus:
		run, found, err := cleaner.HistoryCleanupRun(profileID)
		if err != nil {
			return toolspkg.ToolResult{}, sessionCleanupError(req.ToolID, err)
		}
		response := contract.SessionHistoryCleanupRunResponse{}
		if !found {
			return structuredResult(response, "no cleanup has run since the daemon started")
		}
		if err := n.requireSessionCleanupRunScope(ctx, scope, req.ToolID, input, run); err != nil {
			return toolspkg.ToolResult{}, err
		}
		payload := core.HistoryCleanupRunPayload(run)
		response.Run = &payload
		return structuredResult(response, sessionCleanupRunPreview(payload))
	case toolspkg.SessionCleanupActionStop:
		current, found, err := cleaner.HistoryCleanupRun(profileID)
		if err != nil {
			return toolspkg.ToolResult{}, sessionCleanupError(req.ToolID, err)
		}
		if !found {
			return toolspkg.ToolResult{}, sessionCleanupError(req.ToolID, sessioncleanup.ErrNoActiveRun)
		}
		if err := n.requireSessionCleanupRunScope(ctx, scope, req.ToolID, input, current); err != nil {
			return toolspkg.ToolResult{}, err
		}
		// Stop exactly the run that passed the scope check, never one that replaced it.
		run, err := cleaner.StopHistoryCleanup(ctx, profileID, current.ID)
		if err != nil {
			return toolspkg.ToolResult{}, sessionCleanupError(req.ToolID, err)
		}
		payload := core.HistoryCleanupRunPayload(run)
		return structuredResult(
			contract.SessionHistoryCleanupRunResponse{Run: &payload},
			sessionCleanupRunPreview(payload),
		)
	default:
		return toolspkg.ToolResult{}, nativeInputError(
			req.ToolID,
			fmt.Errorf("action must be %s, %s, or %s", toolspkg.SessionCleanupActionPlan,
				toolspkg.SessionCleanupActionStatus, toolspkg.SessionCleanupActionStop),
		)
	}
}

func (n *daemonNativeTools) sessionCleanupPlan(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	cleaner core.SessionHistoryCleanupManager,
	profileID string,
	input sessionCleanupInput,
) (toolspkg.ToolResult, error) {
	request, err := n.sessionCleanupRequest(ctx, scope, id, profileID, input, time.Now().UTC())
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.DryRun == nil || *input.DryRun {
		plan, err := cleaner.PreviewHistoryCleanup(ctx, request)
		if err != nil {
			return toolspkg.ToolResult{}, sessionCleanupError(id, err)
		}
		payload := core.HistoryCleanupPlanPayload(plan, request.ArchivedBefore)
		return structuredResult(
			sessionCleanupPlanResult{DryRun: true, Plan: payload},
			fmt.Sprintf("would delete %d sessions in %d threads; %d kept in the list or Later",
				payload.Counts.Total, payload.Threads, payload.KeptLive),
		)
	}
	run, plan, err := cleaner.StartHistoryCleanup(ctx, request)
	if err != nil {
		return toolspkg.ToolResult{}, sessionCleanupError(id, err)
	}
	runPayload := core.HistoryCleanupRunPayload(run)
	return structuredResult(
		sessionCleanupPlanResult{Plan: core.HistoryCleanupPlanPayload(plan, request.ArchivedBefore), Run: &runPayload},
		fmt.Sprintf("deleting %d sessions in the background; follow with action=status", runPayload.Total),
	)
}

// sessionCleanupRequest resolves the caller's scope, selection, and range. A
// cleanup across every workspace needs operator authority or a session with
// unrestricted workspace access; otherwise it defaults to the caller's workspace.
func (n *daemonNativeTools) sessionCleanupRequest(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	profileID string,
	input sessionCleanupInput,
	now time.Time,
) (sessioncleanup.Request, error) {
	request := sessioncleanup.Request{
		Scope: sessioncleanup.Scope{ProfileID: profileID},
		Selection: sessioncleanup.Selection{
			Everything: input.Delete.Everything,
			Children:   input.Delete.Children,
			Subagents:  input.Delete.Subagents,
		},
		RootIDs: input.SessionIDs,
	}
	if request.Selection.Empty() {
		request.Selection.Everything = true
	}
	if request.Selection.Everything {
		request.Selection.Children, request.Selection.Subagents = true, true
	}
	if input.AllWorkspaces {
		allowed, err := n.sessionCleanupReachesAllWorkspaces(ctx, scope)
		if err != nil {
			return request, err
		}
		if !allowed {
			return request, nativeWorkspaceAccessDeniedError(id)
		}
		request.Scope.AllWorkspaces = true
	} else {
		workspaceID, err := n.sessionCleanupWorkspaceID(ctx, scope, id, input.Workspace)
		if err != nil {
			return request, err
		}
		request.Scope.WorkspaceID = workspaceID
	}
	// A confirmed count fixes the range where the preview resolved it.
	countedAt := now.UTC()
	if input.CountedAt != nil {
		countedAt = input.CountedAt.UTC()
		now = countedAt
	}
	// The count is fenced to the instant the range resolves from, so the two never drift apart.
	request.CountedAt = &countedAt
	cutoff, err := sessionCleanupCutoff(input, now)
	if err != nil {
		return request, nativeInputError(id, err)
	}
	request.ArchivedBefore = cutoff
	return request, nil
}

// requireSessionCleanupRunScope admits a status or stop call only for a caller
// that could have started the run: the profile's run may belong to another
// workspace, and its failures name that workspace's sessions. A run across every
// workspace needs that reach; a run in one workspace needs it to be the call's
// workspace, which the registry already authorized from the workspace input.
func (n *daemonNativeTools) requireSessionCleanupRunScope(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	input sessionCleanupInput,
	run sessioncleanup.Run,
) error {
	reachesAll, err := n.sessionCleanupReachesAllWorkspaces(ctx, scope)
	if err != nil {
		return err
	}
	if reachesAll {
		return nil
	}
	if run.Scope.AllWorkspaces || input.AllWorkspaces {
		return nativeWorkspaceAccessDeniedError(id)
	}
	workspaceID, err := n.sessionCleanupWorkspaceID(ctx, scope, id, input.Workspace)
	if err != nil {
		return err
	}
	if workspaceID != run.Scope.WorkspaceID {
		return nativeWorkspaceAccessDeniedError(id)
	}
	return nil
}

func (n *daemonNativeTools) sessionCleanupWorkspaceID(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	workspace string,
) (string, error) {
	ref := nativeCallerWorkspaceInput(workspace, scope)
	if ref == workspaceaccess.UserSelector {
		return workspaceaccess.UserSelector, nil
	}
	workspaceID, err := n.workspaceID(ctx, ref)
	if err != nil {
		return "", nativeInputError(id, err)
	}
	if workspaceID == "" {
		return "", nativeRequiredInputError(id, nativeWorkspaceInputKey)
	}
	return workspaceID, nil
}

func (n *daemonNativeTools) sessionCleanupReachesAllWorkspaces(
	ctx context.Context,
	scope toolspkg.Scope,
) (bool, error) {
	if nativeWorkspaceScopeHasOperatorAuthority(scope) {
		return true, nil
	}
	sessionID := strings.TrimSpace(scope.SessionID)
	if sessionID == "" || n.deps.Sessions == nil {
		return false, nil
	}
	info, err := n.deps.Sessions.Status(ctx, sessionID)
	if err != nil {
		return false, err
	}
	return info != nil && info.EffectiveWorkspaceAccess == workspaceaccess.Unrestricted, nil
}

// sessionCleanupCutoff picks the archive instant a thread must precede; with
// no range field it uses the archived view's default of 7 days.
func sessionCleanupCutoff(input sessionCleanupInput, now time.Time) (*time.Time, error) {
	set := 0
	for _, present := range []bool{input.OlderThanDays != nil, input.ArchivedBefore != nil, input.AnyTime} {
		if present {
			set++
		}
	}
	if set > 1 {
		return nil, errors.New("choose at most one of older_than_days, archived_before, and any_time")
	}
	switch {
	case input.AnyTime:
		return nil, nil
	case input.ArchivedBefore != nil:
		cutoff := input.ArchivedBefore.UTC()
		return &cutoff, nil
	}
	days := sessionCleanupDefaultDays
	if input.OlderThanDays != nil {
		days = *input.OlderThanDays
	}
	if days < 1 || days > sessionCleanupMaxDays {
		return nil, fmt.Errorf("older_than_days must be between 1 and %d", sessionCleanupMaxDays)
	}
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	return &cutoff, nil
}

func sessionCleanupRunPreview(run contract.SessionHistoryCleanupRun) string {
	return fmt.Sprintf("cleanup %s: %d of %d done, %d deleted, %d failed threads",
		run.State, run.Done, run.Total, run.Deleted, run.FailedThreads)
}

func sessionCleanupError(id toolspkg.ToolID, err error) error {
	switch {
	case errors.Is(err, sessioncleanup.ErrInvalidRequest):
		return nativeInputError(id, err)
	case errors.Is(err, sessioncleanup.ErrRunActive), errors.Is(err, sessioncleanup.ErrNoActiveRun):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict, id, err.Error(), fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
		)
	case errors.Is(err, sessioncleanup.ErrShuttingDown), errors.Is(err, session.ErrSessionArchiveUnavailable):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable, id, err.Error(), fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, err),
		)
	default:
		return err
	}
}
