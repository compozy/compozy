package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (r *Resolver) lookupWorkspaceByEnclosingRoot(
	ctx context.Context,
	canonicalPath string,
) (Workspace, error) {
	workspaces, err := r.store.ListWorkspaces(ctx)
	if err != nil {
		return Workspace{}, fmt.Errorf("workspace: list workspaces for enclosing root match: %w", err)
	}
	candidates := make([]EnclosingRootCandidate, 0, len(workspaces))
	for _, workspace := range workspaces {
		if err := checkContext(ctx); err != nil {
			return Workspace{}, err
		}
		root := strings.TrimSpace(workspace.RootDir)
		if root == "" {
			continue
		}
		canonicalCandidate, err := canonicalRoot(root)
		switch {
		case err == nil:
		case errors.Is(err, ErrWorkspaceRootMissing), errors.Is(err, os.ErrNotExist):
			continue
		default:
			return Workspace{}, fmt.Errorf(
				"workspace: canonicalize registered workspace root %q: %w",
				root,
				err,
			)
		}
		candidates = append(candidates, EnclosingRootCandidate{
			ID:   workspace.ID,
			Root: canonicalCandidate,
		})
	}
	nearest, ok := SelectNearestEnclosingRoot(canonicalPath, candidates)
	if !ok {
		return Workspace{}, ErrWorkspaceNotFound
	}
	for _, workspace := range workspaces {
		if workspace.ID == nearest.ID {
			return cloneWorkspace(workspace), nil
		}
	}
	return Workspace{}, ErrWorkspaceNotFound
}

// EnclosingRootCandidate identifies a canonical workspace root for nearest-root selection.
type EnclosingRootCandidate struct {
	ID   string
	Root string
}

// SelectNearestEnclosingRoot returns the deepest enclosing root, breaking ties by ID.
func SelectNearestEnclosingRoot(
	canonicalPath string,
	candidates []EnclosingRootCandidate,
) (EnclosingRootCandidate, bool) {
	var nearest EnclosingRootCandidate
	found := false
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ID) == "" || !pathIsWithinRoot(canonicalPath, candidate.Root) {
			continue
		}
		candidateDepth := pathDepth(candidate.Root)
		nearestDepth := pathDepth(nearest.Root)
		if !found ||
			candidateDepth > nearestDepth ||
			(candidateDepth == nearestDepth && candidate.ID < nearest.ID) {
			nearest = candidate
			found = true
		}
	}
	return nearest, found
}

func pathIsWithinRoot(path string, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." ||
		(relative != ".." &&
			!strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
			!filepath.IsAbs(relative))
}

func pathDepth(path string) int {
	cleaned := filepath.Clean(path)
	depth := 0
	for {
		parent := filepath.Dir(cleaned)
		if parent == cleaned {
			return depth
		}
		depth++
		cleaned = parent
	}
}
