package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	storepkg "github.com/compozy/compozy/internal/store"
)

const moveGlobalMemoryDirOperation = "move_global_dir"

func (s *Store) reconcileProfileMemoryMaintenance(ctx context.Context) error {
	if s == nil || s.catalog == nil {
		return nil
	}
	// The migration belongs to the default profile's global store. All profile
	// views share the catalog, but a profile/workspace view must not claim the
	// one-shot move or mark it complete before that owner can perform it.
	if _, ok := legacyMemorySourceForTarget(
		cleanDirPath(s.globalDir),
	); !ok ||
		strings.TrimSpace(s.workspaceRoot) != "" {
		return nil
	}
	db, err := s.catalog.ensureDB(ctx)
	if err != nil {
		return err
	}
	var status string
	err = db.QueryRowContext(
		ctx,
		`SELECT status FROM memory_maintenance_ops WHERE op = ?`,
		moveGlobalMemoryDirOperation,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) || strings.TrimSpace(status) == "done" {
		return nil
	}
	if err != nil {
		return fmt.Errorf("memory: read profile maintenance operation: %w", err)
	}
	if strings.TrimSpace(status) != "pending" {
		return fmt.Errorf("memory: invalid profile maintenance status %q", status)
	}
	if s.profileMaintenancePending != nil {
		s.profileMaintenancePending.Store(true)
	}
	if err := moveDefaultProfileMemoryDir(s.globalDir); err != nil {
		return err
	}
	result, err := db.ExecContext(
		ctx,
		`UPDATE memory_maintenance_ops
		 SET status = 'done', completed_at = ?
		 WHERE op = ? AND status = 'pending'`,
		storepkg.FormatTimestamp(s.catalog.now().UTC()),
		moveGlobalMemoryDirOperation,
	)
	if err != nil {
		return fmt.Errorf("memory: complete profile maintenance operation: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("memory: inspect profile maintenance completion: %w", err)
	}
	if affected != 1 {
		return errors.New("memory: profile maintenance operation changed concurrently")
	}
	if s.profileMaintenancePending != nil {
		s.profileMaintenancePending.Store(false)
	}
	return nil
}

func moveDefaultProfileMemoryDir(target string) error {
	target = cleanDirPath(target)
	source, ok := legacyMemorySourceForTarget(target)
	if !ok {
		return nil
	}
	sourceExists, err := directoryExists(source)
	if err != nil {
		return err
	}
	targetExists, err := directoryExists(target)
	if err != nil {
		return err
	}
	if sourceExists && targetExists {
		empty, emptyErr := directoryEmpty(target)
		if emptyErr != nil {
			return emptyErr
		}
		if !empty {
			return fmt.Errorf("memory: both legacy and profile memory directories contain state")
		}
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("memory: remove empty profile memory directory %q: %w", target, err)
		}
		targetExists = false
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("memory: create profile directory %q: %w", filepath.Dir(target), err)
	}
	if sourceExists && !targetExists {
		if err := os.Rename(source, target); err != nil {
			return fmt.Errorf("memory: move legacy directory %q to %q: %w", source, target, err)
		}
	} else if !targetExists {
		if err := os.Mkdir(target, 0o700); err != nil {
			return fmt.Errorf("memory: create profile memory directory %q: %w", target, err)
		}
	}
	if err := os.Chmod(target, 0o700); err != nil {
		return fmt.Errorf("memory: secure profile memory directory %q: %w", target, err)
	}
	return nil
}

func legacyMemorySourceForTarget(target string) (string, bool) {
	if filepath.Base(target) != compozyconfig.MemoryDirName {
		return "", false
	}
	profileDir := filepath.Dir(target)
	if filepath.Base(profileDir) != compozyconfig.DefaultProfileDirName {
		return "", false
	}
	profilesDir := filepath.Dir(profileDir)
	if filepath.Base(profilesDir) != compozyconfig.ProfilesDirName {
		return "", false
	}
	return filepath.Join(filepath.Dir(profilesDir), compozyconfig.MemoryDirName), true
}

func directoryExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("memory: stat directory %q: %w", path, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("memory: path %q is not a directory", path)
	}
	return true, nil
}

func directoryEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("memory: read directory %q: %w", path, err)
	}
	return len(entries) == 0, nil
}
