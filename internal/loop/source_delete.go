package loop

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// StagedWritableDefinitionDeletion owns a reversible definition-directory removal.
type StagedWritableDefinitionDeletion struct {
	originalDir string
	stagingDir  string
	stagingRoot string
}

// StageWritableDefinitionDeletion atomically moves a writable definition outside its scanned root.
func StageWritableDefinitionDeletion(root string, name string) (*StagedWritableDefinitionDeletion, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("loop: writable root is required")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("loop: resolve root %q: %w", root, err)
	}
	loopDir, err := safeLoopDir(rootAbs, name)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(loopDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrDefinitionNotFound, name)
		}
		return nil, fmt.Errorf("loop: inspect definition %q: %w", loopDir, err)
	}
	stagingRoot, err := os.MkdirTemp(filepath.Dir(rootAbs), ".loop-delete-")
	if err != nil {
		return nil, fmt.Errorf("loop: create definition deletion stage: %w", err)
	}
	stagingDir := filepath.Join(stagingRoot, filepath.Base(loopDir))
	if err := os.Rename(loopDir, stagingDir); err != nil {
		cleanupErr := os.RemoveAll(stagingRoot)
		return nil, errors.Join(
			fmt.Errorf("loop: stage definition %q for deletion: %w", loopDir, err),
			cleanupErr,
		)
	}
	return &StagedWritableDefinitionDeletion{
		originalDir: loopDir,
		stagingDir:  stagingDir,
		stagingRoot: stagingRoot,
	}, nil
}

// Restore moves a staged definition back into the scanned writable root.
func (d *StagedWritableDefinitionDeletion) Restore() error {
	if d == nil {
		return errors.New("loop: staged definition deletion is required")
	}
	if err := os.Rename(d.stagingDir, d.originalDir); err != nil {
		return fmt.Errorf("loop: restore staged definition %q: %w", d.originalDir, err)
	}
	if err := os.Remove(d.stagingRoot); err != nil {
		slog.Warn(
			"loop: remove empty definition deletion stage after restore",
			"staging_root", d.stagingRoot,
			"error", err,
		)
	}
	return nil
}

// Commit permanently removes a staged definition directory.
func (d *StagedWritableDefinitionDeletion) Commit() error {
	if d == nil {
		return errors.New("loop: staged definition deletion is required")
	}
	if err := os.RemoveAll(d.stagingRoot); err != nil {
		return fmt.Errorf("loop: commit staged definition deletion %q: %w", d.stagingRoot, err)
	}
	return nil
}
