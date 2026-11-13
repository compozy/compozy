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

const trashDirectory = ".trash"

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
	if err := os.MkdirAll(trash, 0o700); err != nil {
		s.logTrashError(err)
		return "", nil
	}
	info, err := os.Lstat(trash)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", refusal(ErrSafetyCheckFailed, "the worktree trash is not a directory")
	}
	staged := filepath.Join(trash, item.ID)
	s.trash.mu.Lock()
	s.trash.roots[trash] = struct{}{}
	s.trash.active[staged] = true
	s.trash.mu.Unlock()
	if _, err := os.Lstat(staged); !errors.Is(err, os.ErrNotExist) {
		s.releaseTrash(staged)
		return "", refusal(ErrRemovalFailed, "a staged checkout already exists")
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
		if err := s.removeAbsentCheckout(ctx, workspace.Root, commonDir, item, mode.force); err != nil {
			if restoreErr := s.renameCheckout(staged, item.Path); restoreErr != nil {
				return staged, true, nil, errors.Join(err, fmt.Errorf("worktree: restore staged checkout: %w", restoreErr))
			}
			return staged, false, nil, err
		}
	} else {
		irreversible, refusalResult, err := s.runFencedRemoval(ctx, workspace.Root, item, commonDir, mode.force)
		if err != nil || refusalResult != nil {
			return "", irreversible, refusalResult, err
		}
	}
	return staged, true, nil, s.finishRemoval(ctx, workspace, item, mode)
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
	rows, err := s.store.ListRecoverable(ctx)
	if err != nil {
		return err
	}
	protected := make(map[string]bool)
	for _, row := range rows {
		if row.State == StateRemoving {
			protected[row.ID] = true
		}
	}
	roots, err := s.trashRoots(ctx)
	if err != nil {
		return err
	}
	var sweepErr error
	for _, path := range roots {
		sweepErr = errors.Join(sweepErr, s.collectTrashRoot(ctx, path, protected, pressured))
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
	if s.resolver != nil {
		workspaces, err := s.resolver.ListWorktreeWorkspaces(ctx)
		if err != nil {
			return nil, err
		}
		for _, workspace := range workspaces {
			if workspace.WorktreesRoot != "" {
				roots[filepath.Join(workspace.WorktreesRoot, trashDirectory)] = struct{}{}
			}
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
	return result, nil
}

func (s *Service) collectTrashRoot(
	ctx context.Context, path string, protected map[string]bool, pressured func() bool,
) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("worktree: refusing a linked trash directory")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	var sweepErr error
	for _, entry := range entries {
		s.trash.mu.Lock()
		active := s.trash.active[filepath.Join(path, entry.Name())]
		s.trash.mu.Unlock()
		if active || protected[entry.Name()] {
			continue
		}
		sweepErr = errors.Join(sweepErr, deleteTrashEntry(ctx, root, entry.Name(), pressured))
	}
	return sweepErr
}

func deleteTrashEntry(ctx context.Context, root *os.Root, path string, pressured func() bool) error {
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
		if err := deleteTrashDirectory(ctx, root, path, pressured); err != nil {
			return err
		}
	}
	return root.Remove(path)
}

func deleteTrashDirectory(ctx context.Context, root *os.Root, path string, pressured func() bool) error {
	dir, err := root.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	for {
		names, readErr := dir.Readdirnames(64)
		for _, name := range names {
			if err := deleteTrashEntry(ctx, root, filepath.Join(path, name), pressured); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
		if err := waitTrash(ctx, 10*time.Millisecond); err != nil {
			return err
		}
	}
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
