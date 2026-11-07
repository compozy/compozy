package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/compozy/compozy/internal/projectfiles/changes"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionChangesInput struct {
	Workspace  string        `json:"workspace"`
	SessionID  string        `json:"session_id"`
	Scope      changes.Scope `json:"scope"`
	Base       string        `json:"base"`
	PatchPaths []string      `json:"patch_paths"`
	Reveal     bool          `json:"reveal"`
}

func (n *daemonNativeTools) sessionChanges(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionChangesInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	info, err := n.nativeSessionInWorkspace(ctx, scope, req.ToolID, workspaceID, input.SessionID)
	if err != nil {
		return sessionChangesFailure("session_not_found")
	}
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if !readScope.Matches(info.ProfileID) {
		return sessionChangesFailure("session_not_found")
	}
	if info.WorkspaceID == "" {
		return sessionChangesFailure("root_unavailable")
	}
	root := projectfiles.Root{
		Kind:        projectfiles.RootKindWorkspace,
		WorkspaceID: resolved.WorkspaceID,
		Name:        resolved.Name,
		AbsPath:     resolved.RootDir,
	}
	if info.WorktreeID != "" {
		if n.deps.Worktrees == nil {
			return sessionChangesFailure("root_unavailable")
		}
		item, resolveErr := n.deps.Worktrees.Resolve(ctx, workspaceID, info.WorktreeID)
		if resolveErr != nil {
			return sessionChangesFailure("root_unavailable")
		}
		if !readScope.Matches(item.ProfileID) {
			return sessionChangesFailure("session_not_found")
		}
		root.Kind = projectfiles.RootKindWorktree
		root.WorktreeID, root.AbsPath = item.ID, item.Path
		root.Branch, root.BaseRef = item.Branch, item.BaseRef
	}
	root, err = projectfiles.CanonicalRoot(root)
	if err != nil {
		return sessionChangesFailure("root_unavailable")
	}
	engine := changes.NewEngine()
	request := changes.Request{Scope: input.Scope, BaseRef: input.Base, Reveal: input.Reveal}
	summary, err := engine.Summary(ctx, root, request)
	if err != nil {
		return nativeChangesError(err)
	}
	payload := core.SessionChangesPayload(root, summary)
	payload.PullRequest = n.sessionChangesPullRequest(ctx, info)
	response := struct {
		contract.SessionChangesResponse
		Patches []contract.ChangePatchPayload `json:"patches,omitempty"`
	}{SessionChangesResponse: payload}
	if len(input.PatchPaths) > 0 {
		request.Revision = summary.Revision
		targets := sessionChangeTargets(summary, input.PatchPaths)
		patches, patchErr := engine.Patches(ctx, root, request, targets)
		if patchErr != nil {
			return nativeChangesError(patchErr)
		}
		response.Patches = core.ChangePatchPayloads(patches)
	}
	return structuredResult(response, fmt.Sprintf("%d files changed", summary.Totals.Files))
}

func sessionChangesFailure(code string) (toolspkg.ToolResult, error) {
	message := map[string]string{
		"session_not_found": "session not found", "root_unavailable": "this session has no available project root",
		"path_denied":    "path cannot be shown",
		"file_not_found": "file not found",
		"not_a_file":     "path is not a file",
		"base_not_found": "base ref not found", "invalid_scope": "scope must be branch or uncommitted",
		"too_many_paths":       "request at most 20 paths (8 KiB)",
		"changes_stale":        "changes moved since revision; refetch the summary",
		"not_a_git_repository": "this folder is not a git repository", "invalid_path": "invalid root-relative path",
		"path_outside_root": "path is outside this project", "changes_unavailable": "could not read changes; try again",
	}[code]
	return structuredResult(
		map[string]any{watchEventsPayloadErrorKey: map[string]string{"code": code, "message": message}},
		message,
	)
}

func nativeChangesError(err error) (toolspkg.ToolResult, error) {
	code := "changes_unavailable"
	for _, item := range []struct {
		err  error
		code string
	}{
		{changes.ErrBaseNotFound, "base_not_found"},
		{changes.ErrInvalidScope, "invalid_scope"},
		{changes.ErrTooManyPaths, "too_many_paths"},
		{changes.ErrStale, "changes_stale"},
		{changes.ErrNotARepository, "not_a_git_repository"},
		{projectfiles.ErrInvalidPath, "invalid_path"},
		{projectfiles.ErrPathOutsideRoot, "path_outside_root"},
		{projectfiles.ErrPathDenied, "path_denied"},
		{projectfiles.ErrNotFound, "file_not_found"},
		{projectfiles.ErrNotAFile, "not_a_file"},
		{projectfiles.ErrRootUnavailable, "root_unavailable"},
	} {
		if errors.Is(err, item.err) {
			code = item.code
			break
		}
	}
	return sessionChangesFailure(code)
}

func sessionChangeTargets(summary changes.Summary, paths []string) []changes.PatchTarget {
	targets := make([]changes.PatchTarget, 0, len(paths))
	for _, path := range paths {
		target := changes.PatchTarget{Path: path}
		for _, file := range summary.Files {
			if file.Path == path {
				target.OldPath = file.OldPath
				break
			}
		}
		targets = append(targets, target)
	}
	return targets
}

func (n *daemonNativeTools) sessionChangesPullRequest(
	ctx context.Context,
	info *session.Info,
) *contract.SessionPullRequestPayload {
	if projection, ok := n.deps.Sessions.(interface {
		SessionPullRequests(context.Context, []*session.Info) (map[string]session.PullRequestProjection, error)
	}); ok {
		views, viewErr := projection.SessionPullRequests(ctx, []*session.Info{info})
		if viewErr == nil {
			payload, _ := core.SessionChangesPullRequestPayload(views[info.ID].PullRequestView)
			return payload
		}
	}
	return nil
}
