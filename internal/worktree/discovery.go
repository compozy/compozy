package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

type discoveryCacheEntry struct {
	entries    []GitWorktree
	diagnostic string
	expiresAt  time.Time
}

type RepoState struct {
	GitBacked    bool
	GitAvailable bool
	Diagnostic   string
}

type DiscoveredWorktree struct {
	Name        string
	Branch      string
	Path        string
	Detached    bool
	Stale       bool
	Selectable  bool
	Unavailable bool
	Reason      string
}

type Listing struct {
	Worktrees  []Worktree
	Discovered []DiscoveredWorktree
	Repo       RepoState
}

func (s *Service) invalidateDiscovery(workspaceID string) {
	s.cacheMu.Lock()
	delete(s.discovery, workspaceID)
	s.cacheEpoch++
	s.cacheMu.Unlock()
}

func (s *Service) Get(ctx context.Context, workspaceID, ref string) (*Worktree, error) {
	item, err := s.store.Get(ctx, workspaceID, strings.TrimSpace(ref))
	if errors.Is(err, ErrNotFound) || item == nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("worktree: read registry row: %w", err)
	}
	return item, nil
}

// List combines registered and discovered checkouts, reconciling vanished ready
// rows without overwriting concurrent lifecycle changes or guessing on stat errors.
func (s *Service) List(ctx context.Context, workspaceID string, refresh bool) (*Listing, error) {
	workspace, err := s.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	repo, err := s.repoState(ctx, workspace)
	if err != nil {
		return nil, err
	}
	listing := &Listing{Repo: repo}
	rows, err := s.store.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("worktree: list registry rows: %w", err)
	}
	listing.Worktrees = rows
	if !repo.GitAvailable {
		return listing, nil
	}
	entries, scanDiagnostic, cached := s.cachedDiscoveryEntries(workspaceID, refresh)
	if !cached {
		commonDir, commonDirErr := s.commonDir(ctx, workspace.Root)
		if commonDirErr != nil {
			listing.Repo.Diagnostic = diagnostics.RedactAndBound(commonDirErr.Error(), 2048)
			return listing, nil
		}
		settings, _ := s.worktreeSettings(workspace)
		entries, scanDiagnostic = s.scanDiscoveryEntries(
			ctx, workspaceID, workspace.Root, commonDir, settings.DiscoveryCacheTTL,
		)
	}
	if scanDiagnostic != "" {
		listing.Repo.Diagnostic = scanDiagnostic
		return listing, nil
	}
	registeredPaths := make(map[string]struct{}, len(rows))
	gitPaths := make(map[string]GitWorktree, len(entries))
	for _, row := range rows {
		registeredPaths[canonicalComparablePath(row.Path)] = struct{}{}
	}
	for _, entry := range entries {
		pathKey := canonicalComparablePath(entry.Path)
		gitPaths[pathKey] = entry
		if entry.Main {
			continue
		}
		if _, exists := registeredPaths[pathKey]; exists {
			continue
		}
		_, statErr := os.Stat(entry.Path)
		listing.Discovered = append(listing.Discovered, DiscoveredWorktree{
			Name: discoveredLabel(entry), Branch: entry.Branch, Path: entry.Path,
			Detached: entry.Detached, Stale: entry.Prunable, Selectable: !entry.Prunable && statErr == nil,
			Unavailable: statErr != nil, Reason: entry.PrunableReason,
		})
	}
	for index := range listing.Worktrees {
		row := &listing.Worktrees[index]
		if row.State != StateReady {
			continue
		}
		_, inGit := gitPaths[canonicalComparablePath(row.Path)]
		_, statErr := os.Stat(row.Path)
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return nil, fmt.Errorf("worktree: inspect registered path: %w", statErr)
		}
		if !inGit || errors.Is(statErr, os.ErrNotExist) {
			if err := s.markMissing(ctx, row); err != nil {
				return nil, err
			}
		}
	}
	listing.Worktrees = slices.DeleteFunc(listing.Worktrees, func(row Worktree) bool {
		return row.State == StateDismissed
	})
	return listing, nil
}

// markMissing updates a ready snapshot conditionally; a concurrent winner is
// reread into the snapshot so stale discovery cannot resurrect its previous state.
func (s *Service) markMissing(ctx context.Context, row *Worktree) error {
	swapped, err := s.store.CompareAndSwapState(ctx, row.WorkspaceID, row.ID, StateReady, StateMissing, s.now().UTC())
	if err != nil {
		return fmt.Errorf("worktree: mark missing: %w", err)
	}
	if !swapped {
		current, err := s.store.Get(ctx, row.WorkspaceID, row.ID)
		if err != nil {
			return fmt.Errorf("worktree: reread reconciled row: %w", err)
		}
		if current != nil {
			*row = *current
		}
		return nil
	}
	row.State = StateMissing
	s.emit(ctx, EventMissing, *row)
	return nil
}

func (s *Service) repoState(ctx context.Context, workspace Workspace) (RepoState, error) {
	state := RepoState{}
	if _, err := os.Stat(filepath.Join(workspace.Root, ".git")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return RepoState{}, fmt.Errorf("worktree: inspect workspace Git metadata: %w", err)
	}
	state.GitBacked = true
	diagnostic, err := s.capability.Check(ctx)
	if err != nil {
		state.Diagnostic = diagnostic.Code
		return state, nil
	}
	state.GitAvailable = true
	return state, nil
}

func (s *Service) cachedDiscoveryEntries(workspaceID string, refresh bool) ([]GitWorktree, string, bool) {
	if refresh {
		return nil, "", false
	}
	now := s.now().UTC()
	s.cacheMu.Lock()
	cached, ok := s.discovery[workspaceID]
	s.cacheMu.Unlock()
	if ok && now.Before(cached.expiresAt) {
		return append([]GitWorktree(nil), cached.entries...), cached.diagnostic, true
	}
	return nil, "", false
}

func (s *Service) scanDiscoveryEntries(
	ctx context.Context,
	workspaceID string,
	root string,
	commonDir string,
	ttl time.Duration,
) ([]GitWorktree, string) {
	s.cacheMu.Lock()
	epoch := s.cacheEpoch
	s.cacheMu.Unlock()
	entries, err := s.readWorktreeList(ctx, root, commonDir)
	diagnostic := ""
	if err != nil {
		diagnostic = diagnostics.RedactAndBound(err.Error(), 2048)
	}
	s.cacheMu.Lock()
	if s.cacheEpoch == epoch {
		s.discovery[workspaceID] = discoveryCacheEntry{
			entries: append([]GitWorktree(nil), entries...), diagnostic: diagnostic,
			expiresAt: s.now().UTC().Add(ttl),
		}
	}
	s.cacheMu.Unlock()
	return entries, diagnostic
}

func canonicalComparablePath(value string) string {
	abs, err := filepath.Abs(filepath.Clean(value))
	if err != nil {
		return filepath.Clean(value)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(abs)
}
