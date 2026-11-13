package worktree

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/fileutil"
)

const (
	trashDirectory    = ".compozy-trash"
	trashOwnerFile    = ".compozy-owner"
	trashOwnerContent = "CompozyOS worktree trash v1\n"
)

var errTrashRegisteredCheckout = errors.New("worktree: trash entry is a registered checkout")

type worktreeTrashRootResolver interface {
	ListWorktreeTrashRoots(context.Context) ([]string, error)
}

// Staged checkouts remain protected until repository bookkeeping releases its lock.
type trashCollector struct {
	mu     sync.Mutex
	roots  map[string]struct{}
	active map[string]bool
	cancel context.CancelFunc
	done   chan struct{}
	wake   chan struct{}
}

func newTrashCollector() *trashCollector {
	return &trashCollector{
		roots: make(map[string]struct{}), active: make(map[string]bool), wake: make(chan struct{}, 1),
	}
}

func (s *Service) stageRemoval(workspace Workspace, item Worktree, commonDir string, force bool) (string, error) {
	if _, err := os.Lstat(item.Path); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	if _, err := checkoutRemovalArgs(commonDir, item.Path, item.GitDir, force); err != nil {
		return "", err
	}
	if !hasAbsoluteCheckoutPointer(item.Path, item.GitDir) {
		// Orphaned checkouts and relative pointers retain the synchronous Git path.
		return "", nil
	}
	// Keep Git's populated-submodule safeguards on the synchronous path.
	if hasSubmoduleRemovalMetadata(item, commonDir) {
		return "", nil
	}
	_, managedRoot := s.worktreeSettings(workspace)
	if managedRoot == "" || filepath.Base(item.ID) != item.ID || item.ID == "." || item.ID == "" {
		return "", nil
	}
	trash := filepath.Join(managedRoot, trashDirectory)
	for _, protected := range []string{commonDir, workspace.Root, trash} {
		within, err := fileutil.PathWithinRoot(item.Path, protected)
		if err != nil || within {
			return "", refusal(ErrSafetyCheckFailed, "the checkout contains a protected directory")
		}
	}
	if err := prepareOwnedTrash(trash); err != nil {
		s.logTrashError(err)
		return "", nil
	}
	staged := filepath.Join(trash, item.ID)
	s.trash.mu.Lock()
	s.trash.roots[trash] = struct{}{}
	s.trash.active[staged] = true
	s.trash.mu.Unlock()
	if _, err := os.Lstat(staged); !errors.Is(err, os.ErrNotExist) {
		s.releaseTrash(staged)
		return "", nil
	}
	if err := s.renameCheckout(item.Path, staged); err != nil {
		s.releaseTrash(staged)
		s.logger.Debug("worktree trash staging unavailable; using synchronous removal", "worktree_id", item.ID,
			"error", diagnostics.RedactAndBound(err.Error(), 2048))
		return "", nil
	}
	return staged, nil
}

func (s *Service) completeFencedRemoval(
	ctx context.Context, workspace Workspace, item Worktree, commonDir string, mode removalMode,
) (string, bool, *RemovalRefusal, error) {
	staged, err := s.stageRemoval(workspace, item, commonDir, mode.force)
	if err != nil {
		return "", false, nil, err
	}
	if staged != "" {
		refusalResult, err := s.checkStagedRemoval(ctx, item, staged, mode)
		if err == nil && refusalResult == nil {
			err = s.removeAbsentCheckout(ctx, workspace.Root, commonDir, item, mode.force)
		}
		if err != nil || refusalResult != nil {
			if restoreErr := s.renameCheckout(staged, item.Path); restoreErr != nil {
				return staged, true, refusalResult, errors.Join(
					err,
					fmt.Errorf("worktree: restore staged checkout: %w", restoreErr),
				)
			}
			return staged, false, refusalResult, err
		}
	} else {
		irreversible, refusalResult, err := s.runFencedRemoval(ctx, workspace.Root, item, commonDir, mode.force)
		if err != nil || refusalResult != nil {
			return "", irreversible, refusalResult, err
		}
	}
	return staged, true, nil, s.finishRemoval(ctx, workspace, item, mode)
}

func (s *Service) checkStagedRemoval(
	ctx context.Context, item Worktree, staged string, mode removalMode,
) (*RemovalRefusal, error) {
	if mode.force {
		return nil, nil
	}
	item.Path = staged
	status := s.collectStatus(ctx, item)
	risk, err := s.removalRiskFromStatus(ctx, item, &status)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSafetyCheckFailed, err)
	}
	if mode.keepBranch {
		risk.UnpushedCommits = 0
	}
	return classifyRemovalRisk(risk, false)
}

func (s *Service) releaseTrash(path string) {
	if path == "" {
		return
	}
	s.trash.mu.Lock()
	delete(s.trash.active, path)
	s.trash.mu.Unlock()
	select {
	case s.trash.wake <- struct{}{}:
	default:
	}
}

// StartTrashCollector owns one cancellable, paced worker and sweeps leftovers on boot.
func (s *Service) StartTrashCollector(ctx context.Context, pressured func() bool) {
	s.trash.mu.Lock()
	defer s.trash.mu.Unlock()
	if s.trash.done != nil {
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	s.trash.cancel, s.trash.done = cancel, done
	go func() {
		defer close(done)
		defer cancel()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if err := s.collectTrash(workerCtx, pressured); err != nil && workerCtx.Err() == nil {
				s.logTrashError(err)
			}
			select {
			case <-workerCtx.Done():
				return
			case <-s.trash.wake:
			case <-ticker.C:
			}
		}
	}()
}

func (s *Service) stopTrashCollector() {
	s.trash.mu.Lock()
	cancel, done := s.trash.cancel, s.trash.done
	s.trash.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (s *Service) collectTrash(ctx context.Context, pressured func() bool) error {
	roots, sweepErr := s.trashRoots(ctx)
	for _, path := range roots {
		sweepErr = errors.Join(sweepErr, s.collectTrashRoot(ctx, path, pressured))
	}
	return sweepErr
}

func (s *Service) trashRoots(ctx context.Context) ([]string, error) {
	s.configMu.RLock()
	root := s.root
	s.configMu.RUnlock()
	roots := make(map[string]struct{})
	if root != "" {
		roots[filepath.Join(root, trashDirectory)] = struct{}{}
	}
	var resolveErr error
	if resolver, ok := s.resolver.(worktreeTrashRootResolver); ok {
		configured, err := resolver.ListWorktreeTrashRoots(ctx)
		resolveErr = err
		for _, configuredRoot := range configured {
			roots[filepath.Join(configuredRoot, trashDirectory)] = struct{}{}
		}
	}
	s.trash.mu.Lock()
	defer s.trash.mu.Unlock()
	for path := range s.trash.roots {
		roots[path] = struct{}{}
	}
	result := make([]string, 0, len(roots))
	for path := range roots {
		result = append(result, path)
	}
	return result, resolveErr
}

func (s *Service) collectTrashRoot(
	ctx context.Context, path string, pressured func() bool,
) error {
	root, err := openOwnedTrash(path)
	if err != nil || root == nil {
		return err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, readErr := dir.ReadDir(-1)
	if err := errors.Join(readErr, dir.Close()); err != nil {
		return err
	}
	var sweepErr error
	for _, entry := range entries {
		if entry.Name() == trashOwnerFile {
			continue
		}
		s.trash.mu.Lock()
		active := s.trash.active[filepath.Join(path, entry.Name())]
		s.trash.mu.Unlock()
		if active {
			continue
		}
		// Read after the active check: a failed restore can release staging during this sweep.
		protected, err := s.removingTrashEntry(ctx, entry.Name())
		if err != nil {
			sweepErr = errors.Join(sweepErr, err)
			continue
		}
		if protected {
			continue
		}
		if err := s.deleteTrashEntry(ctx, root, entry.Name(), pressured); !errors.Is(err, errTrashRegisteredCheckout) {
			sweepErr = errors.Join(sweepErr, err)
		}
	}
	return sweepErr
}

func (s *Service) deleteTrashEntry(ctx context.Context, root *os.Root, path string, pressured func() bool) error {
	if err := yieldTrash(ctx, pressured); err != nil {
		return err
	}
	info, err := root.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		owned, err := s.store.PathOwnedByAnyWorkspace(ctx, canonicalComparablePath(filepath.Join(root.Name(), path)))
		if err != nil {
			return err
		}
		if owned {
			return errTrashRegisteredCheckout
		}
		if err := s.deleteTrashDirectory(ctx, root, path, pressured); err != nil {
			return err
		}
	}
	return root.Remove(path)
}

func (s *Service) deleteTrashDirectory(ctx context.Context, root *os.Root, path string, pressured func() bool) error {
	for {
		// Reopen after each batch: deleting through an advancing handle can skip entries on APFS.
		dir, err := root.Open(path)
		if err != nil {
			return err
		}
		names, readErr := dir.Readdirnames(64)
		if err := errors.Join(dir.Close(), nonEOFError(readErr)); err != nil {
			return err
		}
		if len(names) == 0 {
			return nil
		}
		for _, name := range names {
			if err := s.deleteTrashEntry(ctx, root, filepath.Join(path, name), pressured); err != nil {
				return err
			}
		}
		if err := waitTrash(ctx, 10*time.Millisecond); err != nil {
			return err
		}
	}
}

func nonEOFError(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func yieldTrash(ctx context.Context, pressured func() bool) error {
	for pressured != nil && pressured() {
		if err := waitTrash(ctx, 250*time.Millisecond); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func waitTrash(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *Service) logTrashError(err error) {
	s.logger.Warn("worktree trash collection failed",
		"error", diagnostics.RedactAndBound(strings.TrimSpace(err.Error()), 2048))
}

func prepareOwnedTrash(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0o700); err == nil {
		if err := createTrashOwner(path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	root, err := openOwnedTrash(path)
	if err != nil {
		return err
	}
	if root == nil {
		return errors.New("worktree: trash directory has no matching ownership marker")
	}
	return root.Close()
}

func createTrashOwner(path string) error {
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	marker, err := root.OpenFile(trashOwnerFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := marker.WriteString(trashOwnerContent)
	return errors.Join(writeErr, marker.Close())
}

func openOwnedTrash(path string) (*os.Root, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("worktree: refusing a linked trash directory")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	marker, markerErr := root.Lstat(trashOwnerFile)
	if markerErr == nil && marker.Mode().IsRegular() && marker.Size() == int64(len(trashOwnerContent)) {
		content, readErr := root.ReadFile(trashOwnerFile)
		if readErr == nil && string(content) == trashOwnerContent {
			return root, nil
		}
		markerErr = readErr
	}
	return nil, errors.Join(nonMissingTrashMarkerError(markerErr), root.Close())
}

func nonMissingTrashMarkerError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func hasSubmoduleRemovalMetadata(item Worktree, commonDir string) bool {
	for _, path := range []string{
		filepath.Join(item.Path, ".gitmodules"), filepath.Join(item.GitDir, "modules"),
		filepath.Join(commonDir, "modules"),
	} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return true
		}
	}
	return false
}
