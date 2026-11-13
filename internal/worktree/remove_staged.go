package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		// Git cleanup already committed; finish metadata without restoring a broken checkout.
		return staged, nil
	} else if err != nil {
		return staged, fmt.Errorf("%w: inspect staged Git identity: %v", ErrRemovalFailed, err)
	}
	s.trash.mu.Lock()
	s.trash.active[staged] = true
	s.trash.mu.Unlock()
	if err := s.renameCheckout(staged, item.Path); err != nil {
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
