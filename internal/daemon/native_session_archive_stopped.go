package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type sessionArchiveStoppedInput struct {
	Workspace     string `json:"workspace,omitempty"`
	AllWorkspaces bool   `json:"all_workspaces,omitempty"`
	DryRun        bool   `json:"dry_run,omitempty"`
}

// sessionArchiveStopped archives, or previews with dry_run, every eligible
// stopped thread of the caller profile in one workspace (the caller's by
// default) or in all of them. The caller's own thread is always skipped. All
// workspaces needs the reach compozy__session_cleanup requires.
func (n *daemonNativeTools) sessionArchiveStopped(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionArchiveStoppedInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	archiver, ok := n.deps.Sessions.(core.SessionArchiveStoppedManager)
	if !ok {
		return toolspkg.ToolResult{}, sessionCleanupError(req.ToolID, session.ErrSessionArchiveUnavailable)
	}
	profileID, _, _, err := n.nativeCurrentProfileIdentity(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	request := session.ArchiveStoppedRequest{
		ProfileID:       profileID,
		DryRun:          input.DryRun,
		CallerSessionID: scope.SessionID,
	}
	if input.AllWorkspaces && strings.TrimSpace(input.Workspace) != "" {
		return toolspkg.ToolResult{}, nativeInputError(
			req.ToolID, errors.New("choose either workspace or all_workspaces, not both"),
		)
	}
	if input.AllWorkspaces {
		allowed, err := n.sessionCleanupReachesAllWorkspaces(ctx, scope)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		if !allowed {
			return toolspkg.ToolResult{}, nativeWorkspaceAccessDecisionError(
				req.ToolID, workspaceaccess.Decision{Denial: workspaceaccess.DenialNeedsGrant},
			)
		}
		request.AllWorkspaces = true
	} else {
		workspaceID, err := n.archiveStoppedWorkspaceID(ctx, nativeCallerWorkspaceInput(input.Workspace, scope))
		if err != nil {
			return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
		}
		if workspaceID == "" {
			return toolspkg.ToolResult{}, nativeRequiredInputError(req.ToolID, nativeWorkspaceInputKey)
		}
		request.WorkspaceID = workspaceID
	}
	report, err := archiver.ArchiveStopped(ctx, request)
	if err != nil {
		if errors.Is(err, session.ErrArchiveStoppedInvalidRequest) {
			return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
		}
		return toolspkg.ToolResult{}, sessionCleanupError(req.ToolID, err)
	}
	payload := core.ArchiveStoppedPayload(&report)
	verb := "archived"
	if payload.DryRun {
		verb = "would archive"
	}
	skipped := 0
	for _, count := range payload.SkipCounts {
		skipped += count
	}
	return structuredResult(payload, fmt.Sprintf("%s %d threads (%d sessions); skipped %d; failed %d",
		verb, payload.ArchivedThreadCount, payload.ArchivedSessionCount, skipped, payload.FailureCount))
}

// archiveStoppedWorkspaceID resolves an unarchived workspace: an archived one
// is not found, as on HTTP and UDS and as All projects leaves it out.
func (n *daemonNativeTools) archiveStoppedWorkspaceID(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || n.deps.Workspaces == nil {
		return n.workspaceID(ctx, ref)
	}
	workspace, err := n.deps.Workspaces.Get(ctx, ref)
	if err != nil {
		return "", err
	}
	if workspace.ArchivedAt != nil {
		return "", fmt.Errorf("%w: %s is archived", workspacepkg.ErrWorkspaceNotFound, ref)
	}
	return strings.TrimSpace(workspace.ID), nil
}
