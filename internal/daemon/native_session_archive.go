package daemon

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionArchiveInput struct {
	Processes contract.SessionProcessPolicy `json:"processes"`
	Workspace string                        `json:"workspace,omitempty"`
	SessionID string                        `json:"session_id"`
	Stop      bool                          `json:"stop,omitempty"`
	// WithChildren also archives the session's hierarchy descendants.
	WithChildren bool `json:"with_children,omitempty"`
}

func (n *daemonNativeTools) sessionArchive(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.setSessionArchived(ctx, scope, req, true)
}

func (n *daemonNativeTools) sessionUnarchive(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.setSessionArchived(ctx, scope, req, false)
}

func (n *daemonNativeTools) setSessionArchived(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	archived bool,
) (toolspkg.ToolResult, error) {
	var input sessionArchiveInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	if archived && input.WithChildren {
		// The tree would include the caller, whose turn is executing this call.
		within, err := n.nativeCallerWithinHierarchy(ctx, scope.SessionID, input.SessionID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		if within {
			return toolspkg.ToolResult{}, nativeSelfTargetError(req.ToolID)
		}
	}
	if archived && input.Stop {
		// Stopping the caller, or an ancestor whose stop cascades to it, would
		// cancel the turn executing this call, as session_stop refuses for self.
		within, err := n.nativeCallerWithinSession(ctx, scope.SessionID, input.SessionID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		if within {
			return toolspkg.ToolResult{}, nativeSelfTargetError(req.ToolID)
		}
	}
	if _, err := n.nativeSessionInWorkspace(ctx, scope, req.ToolID, workspaceID, input.SessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, ok := n.deps.Sessions.(core.SessionArchiveManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: session archive manager is required")
	}
	if archived {
		if err := n.prepareSessionProcessPolicy(ctx, req.ToolID, input.Processes); err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	var members []*session.Info
	var info *session.Info
	var committedErr error
	if archived && input.WithChildren {
		info, members, err = n.archiveSessionTree(ctx, workspaceID, input)
		if info != nil {
			// The tree committed; its cleanup runs before this error is reported.
			committedErr, err = err, nil
		}
	} else {
		info, err = executeSessionArchive(ctx, manager, workspaceID, input, archived)
	}
	if archived && input.Stop && err != nil {
		n.continueSessionProcessCleanup(input.SessionID, input.Processes, err)
		for _, member := range members {
			n.continueSessionProcessCleanup(member.ID, input.Processes, err)
		}
	}
	if errors.Is(err, session.ErrSessionArchiveStopFailed) {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeSessionArchiveStopFailed,
			req.ToolID,
			err.Error(),
			err,
		)
	}
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if archived {
		// Every member is attempted: the tree is archived and no later call retries it.
		var cleanupErr error
		for _, member := range append([]*session.Info{info}, members...) {
			cleanupErr = errors.Join(cleanupErr, n.finishSessionProcessPolicy(ctx, member, input.Processes))
		}
		if cleanupErr = errors.Join(cleanupErr, committedErr); cleanupErr != nil {
			return toolspkg.ToolResult{}, cleanupErr
		}
	}
	payload := core.SessionPayloadFromInfo(info)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}

// archiveSessionTree returns the descendants even on failure, so cleanup of a
// stop already accepted still reaches them.
func (n *daemonNativeTools) archiveSessionTree(
	ctx context.Context,
	workspaceID string,
	input sessionArchiveInput,
) (*session.Info, []*session.Info, error) {
	manager, ok := n.deps.Sessions.(core.SessionTreeManager)
	if !ok {
		return nil, nil, errors.New("daemon: session tree manager is required")
	}
	if input.Processes == contract.SessionProcessesTerminate {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
	}
	descendants, err := manager.Descendants(ctx, workspaceID, input.SessionID)
	if err != nil {
		return nil, nil, err
	}
	tree, err := manager.ArchiveWithChildren(ctx, workspaceID, input.SessionID, input.Stop)
	if tree == nil {
		return nil, descendants, err
	}
	return tree.Root, tree.Descendants, err
}

func executeSessionArchive(
	ctx context.Context, manager core.SessionArchiveManager, workspaceID string,
	input sessionArchiveInput, archived bool,
) (*session.Info, error) {
	if archived && input.Processes == contract.SessionProcessesTerminate {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Keep the accepted archive stop and cleanup in one bounded lifetime.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
	}
	var info *session.Info
	var err error
	switch {
	case archived && input.Stop:
		info, err = manager.ArchiveWithStop(ctx, workspaceID, input.SessionID)
	case archived:
		info, err = manager.Archive(ctx, workspaceID, input.SessionID)
	default:
		info, err = manager.Unarchive(ctx, workspaceID, input.SessionID)
	}
	return info, err
}

// nativeCallerWithinSession reports whether stopping target would stop the
// calling session: the caller is target, or reaches it through parent links a
// parent stop cascades over (subagent children and auto-stop spawned children).
// Continue, fork, and adopted links name a parent without stopping with it.
func (n *daemonNativeTools) nativeCallerWithinSession(
	ctx context.Context,
	callerID string,
	target string,
) (bool, error) {
	target = strings.TrimSpace(target)
	current := strings.TrimSpace(callerID)
	// Spawn depth has no fixed bound, so walk the whole chain; the visited set
	// ends a corrupt lineage cycle instead of a depth cutoff.
	visited := make(map[string]struct{})
	for {
		if current == "" {
			return false, nil
		}
		if _, seen := visited[current]; seen {
			return false, nil
		}
		visited[current] = struct{}{}
		if current == target {
			return true, nil
		}
		info, err := n.deps.Sessions.Status(ctx, current)
		if errors.Is(err, session.ErrSessionNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if info == nil || !nativeLineageStopsWithParent(info.Lineage) {
			return false, nil
		}
		current = strings.TrimSpace(info.Lineage.ParentSessionID)
	}
}

func nativeLineageStopsWithParent(lineage *store.SessionLineage) bool {
	if lineage == nil || lineage.Kind.Derived() {
		return false
	}
	return lineage.SpawnRole == store.SubagentSpawnRole || lineage.AutoStopOnParent
}
