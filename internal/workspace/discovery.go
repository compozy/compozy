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
	var nearest *Workspace
	nearestRoot := ""
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
		if !pathIsWithinRoot(canonicalPath, canonicalCandidate) {
			continue
		}
		candidateDepth := pathDepth(canonicalCandidate)
		nearestDepth := pathDepth(nearestRoot)
		if nearest == nil ||
			candidateDepth > nearestDepth ||
			(candidateDepth == nearestDepth && workspace.ID < nearest.ID) {
			candidate := cloneWorkspace(workspace)
			nearest = &candidate
			nearestRoot = canonicalCandidate
		}
	}
	if nearest == nil {
		return Workspace{}, ErrWorkspaceNotFound
	}
	return *nearest, nil
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
