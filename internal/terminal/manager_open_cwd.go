package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (m *Service) resolveActorWorkspace(ctx context.Context, ws, cwd string, actor Actor) (string, string, error) {
	cwd, workspaceID, _, err := m.resolveOpenWorkspace(ctx, ws, "", cwd, actor.ProfileID, &actor)
	return cwd, workspaceID, err
}

// resolveOpenWorkspace returns the launch cwd, the canonical workspace ID, and the
// usage release of a requested worktree (nil without one).
func (m *Service) resolveOpenWorkspace(
	ctx context.Context,
	workspaceID string,
	worktreeRef string,
	cwd string,
	profileID string,
	actor *Actor,
	additionalRoots ...string,
) (string, string, func(), error) {
	if strings.TrimSpace(workspaceID) == "" {
		return "", "", nil, &Error{
			Code: ErrorCodeRequiresWorkspace, Message: "terminal operations require a workspace",
			Err: ErrRequiresWorkspace,
		}
	}
	if m.workspaces == nil {
		if worktreeRef != "" {
			return "", "", nil, errors.Join(
				ErrUnsupported,
				errors.New("terminal: worktree terminals require a workspace resolver"),
			)
		}
		if strings.TrimSpace(cwd) == "" {
			cwd = "."
		}
		resolved, err := filepath.Abs(cwd)
		return resolved, workspaceID, nil, err
	}
	resolved, err := m.resolveWorkspace(ctx, workspaceID, profileID)
	if errors.Is(err, workspacepkg.ErrWorkspaceRootMissing) {
		return "", "", nil, &Error{
			Code: ErrorCodeInvalidCwd,
			Message: "this project's folder no longer exists; restore the folder or update the project location " +
				"before opening a terminal",
			Err: errors.Join(ErrInvalidCwd, err),
		}
	}
	if err != nil {
		return "", "", nil, fmt.Errorf("terminal: resolve workspace %q: %w", workspaceID, err)
	}
	canonicalID := strings.TrimSpace(resolved.ID)
	if canonicalID == "" && resolved.Scope != workspacepkg.ScopeUser {
		return "", "", nil, errors.New("terminal: resolved workspace registration is required")
	}
	resolved.AdditionalDirs = append(resolved.AdditionalDirs, additionalRoots...)
	var releaseWorktree func()
	if worktreeRef != "" {
		releaseWorktree, err = m.applyWorktreeRoot(ctx, &resolved, canonicalID, profileID, worktreeRef)
		if err != nil {
			return "", "", nil, err
		}
	}
	validCwd, err := m.resolveLaunchCwd(ctx, &resolved, canonicalID, worktreeRef, cwd, actor)
	if err != nil {
		if releaseWorktree != nil {
			releaseWorktree()
		}
		return "", "", nil, err
	}
	return validCwd, canonicalID, releaseWorktree, nil
}

func (m *Service) resolveLaunchCwd(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	workspaceID string,
	worktreeRef string,
	cwd string,
	actor *Actor,
) (string, error) {
	if actor != nil && actor.Kind == ActorKindAgent && m.executionRoot != nil {
		root, err := m.executionRoot(ctx, workspaceID, *actor)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(root) != "" {
			if worktreeRef != "" && boundRootConflict(root, resolved.RootDir) {
				return "", &Error{
					Code:    ErrorCodeInvalidCwd,
					Message: "worktree " + worktreeRef + " is outside this session's bound worktree",
					Err:     ErrInvalidCwd,
				}
			}
			resolved.RootDir = root
			resolved.AdditionalDirs = nil
		}
	}
	return resolveWorkspaceCwd(resolved, cwd)
}

func (m *Service) resolveWorkspace(
	ctx context.Context,
	workspaceID string,
	profileID string,
) (workspacepkg.ResolvedWorkspace, error) {
	if workspacepkg.ParseRef(workspaceID).Kind == workspacepkg.RefUser || workspaceID == "" {
		resolver, ok := m.workspaces.(workspacepkg.UserRuntimeResolver)
		if !ok {
			return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrUserScopeUnsupported
		}
		return resolver.ResolveUser(ctx, profileID)
	}
	profileResolver, supportsProfiles := m.workspaces.(ProfileWorkspaceResolver)
	if !supportsProfiles || m.profileNames == nil {
		return m.workspaces.Resolve(ctx, workspaceID)
	}
	profileName, err := m.profileNames.ProfileName(ctx, profileID)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, fmt.Errorf("terminal: resolve profile name: %w", err)
	}
	if profileName == "" || profileName == "default" {
		return m.workspaces.Resolve(ctx, workspaceID)
	}
	return profileResolver.ResolveForProfile(ctx, workspaceID, profileName)
}

func resolveWorkspaceCwd(workspace *workspacepkg.ResolvedWorkspace, requested string) (string, error) {
	if workspace == nil {
		return "", errors.New("terminal: resolved workspace is unavailable")
	}
	root := filepath.Clean(workspace.RootDir)
	displayPath := requested
	if requested == "" {
		requested = root
		displayPath = root
	} else if !filepath.IsAbs(requested) {
		requested = filepath.Join(root, requested)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(requested))
	if err != nil {
		return "", &Error{
			Code:    ErrorCodeInvalidCwd,
			Message: fmt.Sprintf("invalid terminal cwd %q: %v", displayPath, err),
			Path:    displayPath,
			Err:     ErrInvalidCwd,
		}
	}
	allowed := append([]string{root}, workspace.AdditionalDirs...)
	for _, candidate := range allowed {
		candidateResolved, resolveErr := filepath.EvalSymlinks(filepath.Clean(candidate))
		if resolveErr != nil {
			continue
		}
		if pathWithin(candidateResolved, resolved) {
			info, statErr := os.Stat(resolved)
			if statErr == nil && info.IsDir() {
				return resolved, nil
			}
		}
	}
	return "", &Error{
		Code:    ErrorCodeInvalidCwd,
		Message: fmt.Sprintf("invalid terminal cwd %q: outside workspace", displayPath),
		Path:    displayPath,
		Err:     ErrInvalidCwd,
	}
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
