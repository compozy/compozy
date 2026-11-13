package worktree

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

// Recovery restores staged bytes before classifying the original checkout as absent.
// A failed restore keeps the removing fence, so collection cannot discard refused work.
func (s *Service) restoreStagedRemoval(workspace Workspace, item Worktree) (string, error) {
	_, managedRoot := s.worktreeSettings(workspace)
	if managedRoot == "" || item.ID == "" || item.ID == "." || filepath.Base(item.ID) != item.ID {
		return "", nil
	}
	trash := filepath.Join(managedRoot, trashDirectory)
	staged := filepath.Join(trash, item.ID)
	info, err := os.Lstat(staged)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return staged, err
	}
	root, err := openOwnedTrash(trash)
	if err != nil {
		return staged, err
	}
	if root == nil {
		return staged, refusal(ErrSafetyCheckFailed, "staged checkout trash ownership cannot be verified")
	}
	defer root.Close()
	if !info.IsDir() || !hasAbsoluteCheckoutPointer(staged, item.GitDir) {
		return staged, refusal(ErrSafetyCheckFailed, "staged checkout identity cannot be verified")
	}
	if _, err := os.Lstat(item.Path); !errors.Is(err, os.ErrNotExist) {
		return staged, refusal(ErrSafetyCheckFailed, "staged checkout cannot replace an existing or unreadable path")
	}
	if _, err := os.Lstat(item.GitDir); errors.Is(err, os.ErrNotExist) {
		approved, approvalErr := stagedRemovalApproved(staged)
		if approvalErr != nil {
			return staged, approvalErr
		}
		if !approved {
			return staged, refusal(ErrSafetyCheckFailed, "staged removal approval cannot be verified")
		}
		return staged, nil
	} else if err != nil {
		return staged, fmt.Errorf("%w: inspect staged Git identity: %v", ErrRemovalFailed, err)
	}
	s.trash.mu.Lock()
	s.trash.active[staged] = true
	s.trash.mu.Unlock()
	if err := s.restoreRemovalCheckout(staged, item.Path); err != nil {
		return staged, fmt.Errorf("%w: restore staged checkout: %v", ErrRemovalFailed, err)
	}
	return staged, nil
}

func hasAbsoluteCheckoutPointer(path, gitDir string) bool {
	pointerPath := filepath.Join(path, ".git")
	info, err := os.Lstat(pointerPath)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	raw, err := os.ReadFile(pointerPath)
	if err != nil {
		return false
	}
	pointer, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir:")
	pointer = strings.TrimSpace(pointer)
	return ok && filepath.IsAbs(pointer) && filepath.Clean(pointer) == filepath.Clean(gitDir)
}

func (s *Service) removingTrashEntry(ctx context.Context, id string) (bool, error) {
	rows, err := s.store.ListRecoverable(ctx)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.ID == id && row.State == StateRemoving {
			return true, nil
		}
	}
	return false, nil
}

const (
	removalApprovalFile    = ".compozy-removal-approved"
	removalApprovalContent = "CompozyOS staged removal approved v1\n"
)

func approveStagedRemoval(staged string) (err error) {
	directory, name, err := fileutil.OpenParentDirectory(filepath.Join(staged, removalApprovalFile))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	return directory.AtomicWriteFile(name, []byte(removalApprovalContent), 0o600, false)
}

func stagedRemovalApproved(staged string) (approved bool, err error) {
	marker, err := fileutil.OpenRegularFile(filepath.Join(staged, removalApprovalFile))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, marker.Close()) }()
	raw, err := io.ReadAll(io.LimitReader(marker, int64(len(removalApprovalContent)+1)))
	return string(raw) == removalApprovalContent, err
}

func (s *Service) restoreRemovalCheckout(staged, path string) error {
	approved, err := stagedRemovalApproved(staged)
	if err != nil {
		return err
	}
	if approved {
		if err := fileutil.AtomicRemoveFile(filepath.Join(staged, removalApprovalFile)); err != nil {
			return err
		}
	}
	return s.renameCheckout(staged, path)
}
