package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/compozy/compozy/internal/projectfiles/changes"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	nativeChangesUnavailable     = "changes_unavailable"
	nativeChangesRootUnavailable = "root_unavailable"
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
		return sessionChangesFailure(nativeChangesRootUnavailable)
	}
	root := projectfiles.Root{
		Kind:        projectfiles.RootKindWorkspace,
		WorkspaceID: workspaceID,
		Name:        resolved.Name,
		AbsPath:     resolved.RootDir,
	}
	if info.WorktreeID != "" {
		if n.deps.Worktrees == nil {
			return sessionChangesFailure(nativeChangesRootUnavailable)
		}
		item, resolveErr := n.deps.Worktrees.Resolve(ctx, workspaceID, info.WorktreeID)
		if resolveErr != nil {
			return sessionChangesFailure(nativeChangesRootUnavailable)
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
		return sessionChangesFailure(nativeChangesRootUnavailable)
	}
	engine := changes.NewEngine()
	request := changes.Request{Scope: input.Scope, BaseRef: input.Base, Reveal: input.Reveal}
	summary, patches, err := readSessionChanges(ctx, engine, root, request, input.PatchPaths)
	if err != nil {
		return nativeChangesError(ctx, n.deps.Logger, root.AbsPath, err)
	}
	payload := core.SessionChangesPayload(root, summary)
	payload.PullRequest = n.sessionChangesPullRequest(ctx, info)
	response := struct {
		contract.SessionChangesResponse
		Patches []contract.ChangePatchPayload `json:"patches,omitempty"`
	}{SessionChangesResponse: payload, Patches: core.ChangePatchPayloads(patches)}
	return structuredResult(response, fmt.Sprintf("%d files changed", summary.Totals.Files))
}

func sessionChangesFailure(code string) (toolspkg.ToolResult, error) {
	message := map[string]string{
		"session_not_found": "session not found", nativeChangesRootUnavailable: "this session has no available project root",
		"path_denied":    "path cannot be shown",
		"file_not_found": "file not found",
		"not_a_file":     "path is not a file",
		"base_not_found": "base ref not found", "invalid_scope": "scope must be branch or uncommitted",
		"too_many_paths":       "request at most 20 paths (8 KiB)",
		"changes_stale":        "changes moved since revision; refetch the summary",
		"not_a_git_repository": "this folder is not a git repository", "invalid_path": "invalid root-relative path",
		"path_outside_root": "path is outside this project", nativeChangesUnavailable: "could not read changes; try again",
	}[code]
	return structuredResult(
		map[string]any{watchEventsPayloadErrorKey: map[string]string{"code": code, "message": message}},
		message,
	)
}

func nativeChangesError(ctx context.Context, logger *slog.Logger, root string, err error) (toolspkg.ToolResult, error) {
	code := nativeChangesUnavailable
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
		{projectfiles.ErrRootUnavailable, nativeChangesRootUnavailable},
	} {
		if errors.Is(err, item.err) {
			code = item.code
			break
		}
	}
	if code == nativeChangesUnavailable || code == nativeChangesRootUnavailable {
		changes.LogFailure(ctx, logger, root, err)
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

func readSessionChanges(
	ctx context.Context,
	engine changes.Engine,
	root projectfiles.Root,
	request changes.Request,
	paths []string,
) (changes.Summary, []changes.Patch, error) {
	for range 2 {
		summary, err := engine.Summary(ctx, root, request)
		if err != nil {
			return changes.Summary{}, nil, err
		}
		if len(paths) == 0 {
			return summary, nil, nil
		}
		request.Revision = summary.Revision
		patches, err := engine.Patches(ctx, root, request, sessionChangeTargets(summary, paths))
		request.Revision = ""
		if errors.Is(err, changes.ErrStale) {
			continue
		}
		if err != nil {
			return changes.Summary{}, nil, err
		}
		valid := len(patches) == len(paths)
		for _, patch := range patches {
			// Off-page patches retain the engine revision and before/after fingerprint checks.
			found := summary.Truncated && patch.Fingerprint != ""
			for _, file := range summary.Files {
				if file.Path == patch.Path {
					found = file.Fingerprint == patch.Fingerprint
					break
				}
			}
			valid = valid && found
		}
		if valid {
			return summary, patches, nil
		}
	}
	return changes.Summary{}, nil, changes.ErrStale
}
