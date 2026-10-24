package worktree

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/fileutil"
)

// checkoutRemovalTimeout bounds a `git worktree remove` that deletes a
// checkout. Deletion time scales with the tree (dependency folders included),
// so the default per-command budget is far too short for it.
const checkoutRemovalTimeout = 15 * time.Minute

const gitDirFlag = "--git-dir"

const orphanedCheckoutDetail = "the checkout's Git metadata (.git) is missing; " +
	"the directory is left over from an interrupted removal"

// runCheckoutRemoval runs a `git worktree remove` that deletes checkout files.
// Git can delete the .git link before the rest of the tree, so killing it
// midway leaves an orphaned checkout. The command therefore outlives the
// request that started it and runs under a budget sized for large trees.
func (s *Service) runCheckoutRemoval(ctx context.Context, dir string, args ...string) ([]byte, []byte, error) {
	ctx = context.WithoutCancel(ctx)
	if runner, ok := s.runner.(GitTimeoutRunner); ok {
		return runner.RunWithTimeout(ctx, dir, checkoutRemovalTimeout, args...)
	}
	return s.runner.Run(ctx, dir, args...)
}

// orphanedCheckout is a linked worktree whose directory survives while its
// .git link is gone. Git lists the entry as prunable and refuses to remove it,
// even when forced, because it can no longer validate the checkout.
type orphanedCheckout struct {
	path        string
	adminGitDir string
}

// findOrphanedCheckout reports whether candidate is an orphaned checkout of the
// repository at commonDir. It matches Git's own record: an admin entry whose
// gitdir backlink names candidate/.git.
func findOrphanedCheckout(candidate, commonDir string) (orphanedCheckout, bool, error) {
	info, err := os.Lstat(candidate)
	if err != nil || !info.IsDir() {
		return orphanedCheckout{}, false, nil
	}
	if _, err := os.Lstat(filepath.Join(candidate, ".git")); !errors.Is(err, os.ErrNotExist) {
		return orphanedCheckout{}, false, nil
	}
	canonical, err := fileutil.CanonicalExistingDirectoryExact(candidate)
	if err != nil {
		return orphanedCheckout{}, false, fmt.Errorf("worktree: resolve orphaned checkout: %w", err)
	}
	adminRoot := filepath.Join(commonDir, "worktrees")
	entries, err := os.ReadDir(adminRoot)
	if errors.Is(err, os.ErrNotExist) {
		return orphanedCheckout{}, false, nil
	}
	if err != nil {
		return orphanedCheckout{}, false, fmt.Errorf("worktree: list Git admin entries: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		adminPath := filepath.Join(adminRoot, entry.Name())
		if backlinkCheckout(adminPath) != canonical {
			continue
		}
		adminGitDir, err := validateAdoptionAdminPath(adminPath, commonDir)
		if err != nil {
			return orphanedCheckout{}, false, err
		}
		return orphanedCheckout{path: canonical, adminGitDir: adminGitDir}, true, nil
	}
	return orphanedCheckout{}, false, nil
}

// backlinkCheckout resolves the checkout directory an admin entry points at,
// or "" when its gitdir file is unreadable or does not name a .git link.
func backlinkCheckout(adminPath string) string {
	raw, err := os.ReadFile(filepath.Join(adminPath, "gitdir"))
	if err != nil {
		return ""
	}
	backlink := strings.TrimSpace(string(raw))
	if backlink == "" {
		return ""
	}
	if !filepath.IsAbs(backlink) {
		backlink = filepath.Join(adminPath, backlink)
	}
	backlink = filepath.Clean(backlink)
	if filepath.Base(backlink) != ".git" {
		return ""
	}
	checkout, err := filepath.EvalSymlinks(filepath.Dir(backlink))
	if err != nil {
		return ""
	}
	return checkout
}

// orphanedRemovalRisk names what deleting an orphaned checkout loses: every
// leftover file, since Git cannot tell changed from unchanged ones, and the
// commits only a detached HEAD still reaches. A branch keeps its own commits.
func (s *Service) orphanedRemovalRisk(
	ctx context.Context,
	root string,
	orphan orphanedCheckout,
) (RemovalRisk, error) {
	files, err := countLeftoverFiles(orphan.path)
	if err != nil {
		return RemovalRisk{}, err
	}
	risk := RemovalRisk{LeftoverFiles: files}
	head, err := os.ReadFile(filepath.Join(orphan.adminGitDir, "HEAD"))
	if err != nil {
		return RemovalRisk{}, fmt.Errorf("read orphaned checkout HEAD: %w", err)
	}
	if strings.HasPrefix(strings.TrimSpace(string(head)), "ref:") {
		return risk, nil
	}
	stdout, stderr, err := s.runner.Run(
		ctx, root, gitDirFlag, orphan.adminGitDir, removalRevListCommand, "--count",
		removalHeadRef, "--not", "--branches", "--tags", "--remotes",
	)
	if err != nil {
		return RemovalRisk{}, fmt.Errorf(
			"count orphaned detached commits: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err,
		)
	}
	risk.UnpushedCommits, err = strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil {
		return RemovalRisk{}, fmt.Errorf("parse orphaned detached commit count: %w", err)
	}
	return risk, nil
}

func countLeftoverFiles(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("count leftover checkout files: %w", err)
	}
	return count, nil
}

// classifyOrphanedRemoval refuses to delete leftover work without force. An
// empty leftover directory with nothing unique to lose needs no confirmation.
func classifyOrphanedRemoval(risk RemovalRisk, force bool) (*RemovalRefusal, error) {
	if force || (risk.LeftoverFiles == 0 && risk.UnpushedCommits == 0) {
		return nil, nil
	}
	return &RemovalRefusal{Code: ErrCheckoutOrphaned.Error(), Risk: risk},
		refusal(ErrCheckoutOrphaned, orphanedCheckoutDetail)
}

// evaluateOrphanedRemoval measures an orphaned checkout's risk and refuses an
// unforced removal that would lose files or commits.
func (s *Service) evaluateOrphanedRemoval(
	ctx context.Context,
	root string,
	orphan orphanedCheckout,
	force bool,
) (RemovalRisk, *RemovalRefusal, error) {
	risk, err := s.orphanedRemovalRisk(ctx, root, orphan)
	if err != nil {
		return RemovalRisk{}, nil, fmt.Errorf("%w: %v", ErrSafetyCheckFailed, err)
	}
	refusalResult, err := classifyOrphanedRemoval(risk, force)
	return risk, refusalResult, err
}

// interruptedRemovalRefusal answers a removal that Git abandoned after
// deleting the .git link with the same refusal and inventory a new request
// would get, so callers can offer force without another round trip.
func (s *Service) interruptedRemovalRefusal(
	ctx context.Context,
	root string,
	orphan orphanedCheckout,
	cause error,
) (*RemovalRefusal, error) {
	s.logger.WarnContext(
		ctx, "worktree removal interrupted; checkout left orphaned",
		"error", diagnostics.RedactAndBound(cause.Error(), 2048),
	)
	risk, err := s.orphanedRemovalRisk(ctx, root, orphan)
	if err != nil {
		return nil, fmt.Errorf("%w: orphaned checkout inventory: %v", ErrSafetyCheckFailed, err)
	}
	return &RemovalRefusal{Code: ErrCheckoutOrphaned.Error(), Risk: risk},
		refusal(ErrCheckoutOrphaned, orphanedCheckoutDetail)
}

// removeOrphanedCheckout deletes the leftover directory first and only then
// clears Git's entry, which Git accepts once the checkout is gone. Stopping in
// between leaves another orphaned checkout, never an untracked directory.
func (s *Service) removeOrphanedCheckout(
	ctx context.Context,
	root string,
	commonDir string,
	gitPath string,
	orphan orphanedCheckout,
) error {
	for _, protected := range []string{commonDir, root} {
		within, err := fileutil.PathWithinRoot(orphan.path, protected)
		if err != nil || within {
			return refusal(ErrSafetyCheckFailed, "the leftover directory contains the repository")
		}
	}
	if err := os.RemoveAll(orphan.path); err != nil {
		return fmt.Errorf("%w: delete leftover checkout: %v", ErrRemovalFailed, err)
	}
	args := []string{gitDirFlag, commonDir, worktreeDescriptorName, exitRemoveAction, "--force"}
	if _, err := os.Lstat(filepath.Join(orphan.adminGitDir, "locked")); err == nil {
		// Git documents a second --force as the override for a locked entry.
		args = append(args, "--force")
	}
	args = append(args, gitPath)
	if _, stderr, err := s.runCheckoutRemoval(ctx, root, args...); err != nil {
		return fmt.Errorf("%w: clear orphaned Git entry: %s: %v",
			ErrRemovalFailed, diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	return nil
}
