package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/fileutil"
)

// discoveredRemovalTarget is one Git-listed linked worktree with no registry row.
// Path keeps Git's spelling because Git matches stale entries by that exact path.
type discoveredRemovalTarget struct {
	entry GitWorktree
	stale bool
}

// RemoveDiscoveredOptions carries the caller's acknowledgements for one target.
type RemoveDiscoveredOptions struct {
	// Force acknowledges dirty or unpushed loss on a live checkout.
	Force bool
	// ExpectStale freezes a confirmation that only clears a stale Git entry:
	// a checkout that is live again is refused instead of removed.
	ExpectStale bool
}

// RemoveDiscovered deletes a linked worktree that Git lists but no registry row
// owns. Stale entries only lose their admin metadata. Live checkouts keep the
// dirty and unpushed refusals of registered removal, and branches stay untouched.
func (s *Service) RemoveDiscovered(
	ctx context.Context,
	workspaceID string,
	path string,
	options RemoveDiscoveredOptions,
) (*RemovalRefusal, error) {
	force := options.Force
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, refusal(ErrRefInvalid, "path is required")
	}
	if _, err := s.capability.Check(ctx); err != nil {
		return nil, err
	}
	workspace, err := s.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return nil, err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return nil, err
	}
	defer release()

	target, err := s.readDiscoveredRemovalTarget(ctx, workspaceID, workspace, commonDir, path)
	if err != nil {
		return nil, err
	}
	if options.ExpectStale && !target.stale {
		return nil, refusal(
			ErrDiscoveredChanged, "the checkout is present again; refresh and confirm its removal",
		)
	}
	if !target.stale {
		item, risk, refusalResult, err := s.evaluateDiscoveredRemoval(ctx, workspaceID, commonDir, target, force)
		if err != nil || refusalResult != nil {
			return refusalResult, err
		}
		if err := s.dispatchGate(ctx, EventPreRemove, HookPreRemovePayload{
			Worktree: eventPayloadFromWorktree(item, workspace.Root), Force: force, Risk: risk, Discovered: true,
		}); err != nil {
			return nil, err
		}
		// Hooks can run arbitrary code. Close that final race before deleting.
		target, err = s.readDiscoveredRemovalTarget(ctx, workspaceID, workspace, commonDir, path)
		if err != nil {
			return nil, err
		}
		if !target.stale {
			if _, _, refusalResult, err = s.evaluateDiscoveredRemoval(
				ctx, workspaceID, commonDir, target, force,
			); err != nil || refusalResult != nil {
				return refusalResult, err
			}
		}
	}
	if err := s.runDiscoveredRemoval(ctx, workspace.Root, commonDir, target, force); err != nil {
		return nil, err
	}
	s.invalidateDiscovery(workspaceID)
	return nil, nil
}

// readDiscoveredRemovalTarget rereads Git's listing under the repository lock so
// the target cannot be the main checkout, a bare entry, or a registered row.
func (s *Service) readDiscoveredRemovalTarget(
	ctx context.Context,
	workspaceID string,
	workspace Workspace,
	commonDir string,
	path string,
) (discoveredRemovalTarget, error) {
	entries, err := s.readWorktreeList(ctx, workspace.Root, commonDir)
	if err != nil {
		return discoveredRemovalTarget{}, err
	}
	requested := filepath.Clean(path)
	requestedKey := canonicalComparablePath(path)
	var matched *GitWorktree
	for index := range entries {
		listed := entries[index].Path
		if filepath.Clean(listed) == requested || canonicalComparablePath(listed) == requestedKey {
			matched = &entries[index]
			break
		}
	}
	if matched == nil {
		return discoveredRemovalTarget{}, ErrNotFound
	}
	if matched.Main || matched.Bare {
		return discoveredRemovalTarget{}, ErrAdoptionMainCheckout
	}
	rows, err := s.store.List(ctx, workspaceID)
	if err != nil {
		return discoveredRemovalTarget{}, fmt.Errorf("worktree: list registry rows: %w", err)
	}
	matchedKey := canonicalComparablePath(matched.Path)
	for _, row := range rows {
		if ownsCheckoutPath(row) && canonicalComparablePath(row.Path) == matchedKey {
			return discoveredRemovalTarget{}, refusal(
				ErrDiscoveredRegistered, "remove the registered worktree by its id",
			)
		}
	}
	// One repository can back several workspaces, and Git lists every linked
	// worktree to each of them. A live record anywhere keeps its own removal path.
	for _, candidate := range []string{matched.Path, matchedKey} {
		owned, err := s.store.PathOwnedByAnyWorkspace(ctx, candidate)
		if err != nil {
			return discoveredRemovalTarget{}, fmt.Errorf("worktree: find path owner: %w", err)
		}
		if owned {
			return discoveredRemovalTarget{}, refusal(
				ErrDiscoveredRegistered, "another workspace registers this worktree",
			)
		}
	}
	stale := matched.Prunable
	if !stale {
		if _, statErr := os.Lstat(matched.Path); errors.Is(statErr, os.ErrNotExist) {
			stale = true
		} else if statErr != nil {
			return discoveredRemovalTarget{}, fmt.Errorf("worktree: inspect discovered path: %w", statErr)
		}
	}
	return discoveredRemovalTarget{entry: *matched, stale: stale}, nil
}

// evaluateDiscoveredRemoval validates the live checkout's Git identity and its
// work-loss risk without persisting status, because no registry row owns it.
func (s *Service) evaluateDiscoveredRemoval(
	ctx context.Context,
	workspaceID string,
	commonDir string,
	target discoveredRemovalTarget,
	force bool,
) (Worktree, RemovalRisk, *RemovalRefusal, error) {
	candidate, err := fileutil.CanonicalExistingDirectory(target.entry.Path)
	if err != nil {
		return Worktree{}, RemovalRisk{}, nil, refusal(ErrAdoptionUnreadable, err.Error())
	}
	identity, err := s.validateAdoptionIdentity(ctx, candidate, commonDir)
	if err != nil {
		return Worktree{}, RemovalRisk{}, nil, err
	}
	if err := s.rejectCoreWorktreeRedirect(ctx, candidate); err != nil {
		return Worktree{}, RemovalRisk{}, nil, err
	}
	item := Worktree{
		WorkspaceID: workspaceID, Name: discoveredLabel(target.entry), Branch: target.entry.Branch,
		Path: candidate, GitDir: identity.adminGitDir, State: StateReady,
	}
	status := s.collectStatus(ctx, item)
	risk, err := s.removalRiskFromStatus(ctx, item, &status)
	if err != nil {
		return Worktree{}, RemovalRisk{}, nil, fmt.Errorf("%w: %v", ErrSafetyCheckFailed, err)
	}
	refusalResult, err := classifyRemovalRisk(risk, force)
	return item, risk, refusalResult, err
}

// runDiscoveredRemoval keeps deletion inside Git so the admin entry and any
// checkout directory go together. Git documents a second --force as the
// override for a locked worktree, so only forced requests may remove one.
func (s *Service) runDiscoveredRemoval(
	ctx context.Context,
	workspaceRoot string,
	commonDir string,
	target discoveredRemovalTarget,
	force bool,
) error {
	args := []string{"--git-dir", commonDir, worktreeDescriptorName, exitRemoveAction}
	if force {
		args = append(args, "--force")
		if target.entry.Locked {
			args = append(args, "--force")
		}
	}
	args = append(args, target.entry.Path)
	_, stderr, err := s.runner.Run(ctx, workspaceRoot, args...)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrRemovalFailed, diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	return nil
}
