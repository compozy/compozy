package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const lifecycleOperationRetentionLimit = 200

func (m *Manager) Recover(ctx context.Context) error {
	if ctx == nil {
		return errors.New("profile: recover context is required")
	}
	rows, err := m.store.DB().QueryContext(
		ctx,
		`SELECT id FROM profile_lifecycle_ops WHERE status IN ('applied', 'finalizing') ORDER BY created_at, id`,
	)
	if err != nil {
		return fmt.Errorf("profile: list recoverable operations: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			closeErr := rows.Close()
			return errors.Join(fmt.Errorf("profile: scan recoverable operation: %w", err), closeErr)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		closeErr := rows.Close()
		return errors.Join(fmt.Errorf("profile: iterate recoverable operations: %w", err), closeErr)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("profile: close recoverable operation rows: %w", err)
	}
	for _, id := range ids {
		if err := m.finalizeOperation(ctx, id, true); err != nil {
			return err
		}
	}
	return m.pruneDoneOperations(ctx, lifecycleOperationRetentionLimit)
}

func (m *Manager) finalizeOperation(ctx context.Context, opID string, recovered bool) error {
	if err := m.write(ctx, "begin profile lifecycle finalization", func(exec globaldb.ProfileWriteExecutor) error {
		result, err := exec.ExecContext(
			ctx,
			`UPDATE profile_lifecycle_ops SET status = 'finalizing', updated_at = ?
			 WHERE id = ? AND status IN ('applied', 'finalizing')`,
			formatTimestamp(m.now()),
			opID,
		)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("profile: lifecycle operation %s is not finalizable", opID)
		}
		return nil
	}); err != nil {
		return err
	}
	steps, err := m.pendingSteps(ctx, opID)
	if err != nil {
		return m.failOperation(ctx, opID, -1, err)
	}
	for _, step := range steps {
		if err := m.executeStep(ctx, opID, step); err != nil {
			return m.failOperation(ctx, opID, step.Seq, err)
		}
		if err := m.write(ctx, "complete profile lifecycle step", func(exec globaldb.ProfileWriteExecutor) error {
			_, err := exec.ExecContext(
				ctx,
				`UPDATE profile_lifecycle_op_steps SET status = 'done', error_message = NULL, updated_at = ?
				 WHERE op_id = ? AND seq = ?`,
				formatTimestamp(m.now()), opID, step.Seq,
			)
			return err
		}); err != nil {
			return m.failOperation(ctx, opID, step.Seq, err)
		}
	}
	if err := m.write(ctx, "complete profile lifecycle operation", func(exec globaldb.ProfileWriteExecutor) error {
		now := formatTimestamp(m.now())
		_, err := exec.ExecContext(
			ctx,
			`UPDATE profile_lifecycle_ops
			 SET status = 'done', updated_at = ?, completed_at = ?, error_code = NULL, error_message = NULL
			 WHERE id = ? AND status = 'finalizing'`,
			now, now, opID,
		)
		return err
	}); err != nil {
		return m.failOperation(ctx, opID, -1, err)
	}
	if recovered {
		m.recordOperationEvent(ctx, eventspkg.ProfileLifecycleOpRecovered, opID, nil)
	}
	return nil
}

func (m *Manager) pendingSteps(ctx context.Context, opID string) (steps []lifecycleStep, err error) {
	rows, err := m.store.DB().QueryContext(
		ctx,
		`SELECT seq, action, COALESCE(path_old, ''), COALESCE(path_new, ''), status
		 FROM profile_lifecycle_op_steps WHERE op_id = ? AND status <> 'done' ORDER BY seq`,
		opID,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	for rows.Next() {
		var step lifecycleStep
		if err := rows.Scan(&step.Seq, &step.Action, &step.PathOld, &step.PathNew, &step.Status); err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (m *Manager) executeStep(ctx context.Context, opID string, step lifecycleStep) error {
	switch step.Action {
	case stepMkdirProfile:
		path, err := m.containedProfilePath(step.PathNew)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("profile: create profile directory %q: %w", path, err)
		}
		if err := os.Chmod(path, 0o700); err != nil {
			return fmt.Errorf("profile: set profile directory permissions %q: %w", path, err)
		}
		return nil
	case stepWriteDeclaredSeed:
		return m.writeDeclaredSeed(ctx, opID, step.PathNew)
	case stepRenameProfile:
		oldPath, err := m.containedProfilePath(step.PathOld)
		if err != nil {
			return err
		}
		newPath, err := m.containedProfilePath(step.PathNew)
		if err != nil {
			return err
		}
		if _, err := os.Stat(newPath); err == nil {
			if _, oldErr := os.Stat(oldPath); errors.Is(oldErr, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("profile: rename target %q already exists", newPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("profile: inspect rename target %q: %w", newPath, err)
		}
		if _, err := os.Stat(oldPath); errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(newPath, 0o700); err != nil {
				return fmt.Errorf("profile: create recovered profile directory %q: %w", newPath, err)
			}
			return nil
		} else if err != nil {
			return fmt.Errorf("profile: inspect rename source %q: %w", oldPath, err)
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("profile: rename %q to %q: %w", oldPath, newPath, err)
		}
		return nil
	case stepRemoveProfile:
		path, err := m.containedProfilePath(step.PathOld)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("profile: remove profile directory %q: %w", path, err)
		}
		return nil
	case stepRemoveDesktopPartitions:
		return m.removeDesktopPartitions(ctx, opID)
	default:
		return fmt.Errorf("profile: unsupported lifecycle step action %q", step.Action)
	}
}

// removeDesktopPartitions purges the deleted profile's window arrangements.
//
// Desktops live in the client-state store, which cannot join the catalog's delete
// transaction — so the removal runs from the journal after apply commits, where a
// crash resumes it instead of losing it. Purging twice is a no-op by design.
func (m *Manager) removeDesktopPartitions(ctx context.Context, opID string) error {
	if m.desktops == nil {
		return nil
	}
	var profileID string
	if err := m.store.DB().QueryRowContext(
		ctx,
		`SELECT profile_id FROM profile_lifecycle_ops WHERE id = ?`,
		opID,
	).Scan(&profileID); err != nil {
		return fmt.Errorf("profile: read desktop-partition owner for operation %s: %w", opID, err)
	}
	if err := m.desktops.PurgeDesktopPartitions(ctx, profileID); err != nil {
		return fmt.Errorf("profile: remove desktop partitions for %q: %w", profileID, err)
	}
	return nil
}

func (m *Manager) writeDeclaredSeed(ctx context.Context, opID, profilePath string) error {
	path, err := m.containedProfilePath(profilePath)
	if err != nil {
		return err
	}
	var agent, provider, sandbox sql.NullString
	if err := m.store.DB().QueryRowContext(
		ctx,
		`SELECT default_agent, default_provider, default_sandbox
		 FROM profile_lifecycle_op_seed WHERE op_id = ?`,
		opID,
	).Scan(&agent, &provider, &sandbox); err != nil {
		return fmt.Errorf("profile: read declared seed for operation %s: %w", opID, err)
	}
	if !agent.Valid && !provider.Valid && !sandbox.Valid {
		return nil
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(
		m.home,
		"",
		compozyconfig.WriteScopeProfile,
		filepath.Base(path),
	)
	if err != nil {
		return fmt.Errorf("profile: resolve declared profile config target: %w", err)
	}
	_, err = compozyconfig.EditConfigOverlay(m.home, "", target, func(editor *compozyconfig.OverlayEditor) error {
		for _, candidate := range []struct {
			key   string
			value sql.NullString
		}{
			{key: "agent", value: agent},
			{key: "provider", value: provider},
			{key: "sandbox", value: sandbox},
		} {
			if !candidate.value.Valid {
				continue
			}
			if err := editor.SetValue([]string{"defaults", candidate.key}, candidate.value.String); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("profile: write declared seed for operation %s: %w", opID, err)
	}
	return nil
}

func (m *Manager) containedProfilePath(path string) (string, error) {
	root, err := filepath.Abs(m.home.ProfilesDir)
	if err != nil {
		return "", fmt.Errorf("profile: resolve profiles root: %w", err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", fmt.Errorf("profile: create profiles root %q: %w", root, err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return "", fmt.Errorf("profile: secure profiles root %q: %w", root, err)
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("profile: resolve profiles root target: %w", err)
	}
	candidate, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", fmt.Errorf("profile: resolve lifecycle path: %w", err)
	}
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", fmt.Errorf("profile: compare lifecycle path: %w", err)
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("profile: lifecycle path %q escapes profiles root", candidate)
	}
	if strings.Contains(relative, string(filepath.Separator)) {
		return "", fmt.Errorf("profile: lifecycle path %q is not a direct profile directory", candidate)
	}
	parentReal, err := filepath.EvalSymlinks(filepath.Dir(candidate))
	if err != nil {
		return "", fmt.Errorf("profile: resolve lifecycle path parent: %w", err)
	}
	if filepath.Clean(parentReal) != filepath.Clean(rootReal) {
		return "", fmt.Errorf("profile: lifecycle path %q resolves outside profiles root", candidate)
	}
	info, err := os.Lstat(candidate)
	if errors.Is(err, os.ErrNotExist) {
		return candidate, nil
	}
	if err != nil {
		return "", fmt.Errorf("profile: inspect lifecycle path %q: %w", candidate, err)
	}
	if err := validateContainedProfileTarget(rootReal, candidate, info); err != nil {
		return "", err
	}
	return candidate, nil
}

func validateContainedProfileTarget(rootReal string, candidate string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("profile: lifecycle path %q must not be a symlink", candidate)
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return fmt.Errorf("profile: resolve lifecycle path target: %w", err)
	}
	resolvedRelative, err := filepath.Rel(rootReal, resolved)
	if err != nil {
		return fmt.Errorf("profile: compare lifecycle target: %w", err)
	}
	if resolvedRelative == "." || resolvedRelative == ".." ||
		strings.HasPrefix(resolvedRelative, ".."+string(filepath.Separator)) ||
		strings.Contains(resolvedRelative, string(filepath.Separator)) {
		return fmt.Errorf("profile: lifecycle path %q resolves outside profiles root", candidate)
	}
	return nil
}

func (m *Manager) failOperation(ctx context.Context, opID string, seq int, cause error) error {
	updateErr := m.write(
		context.WithoutCancel(ctx),
		"fail profile lifecycle operation",
		func(exec globaldb.ProfileWriteExecutor) error {
			now := formatTimestamp(m.now())
			if seq >= 0 {
				if _, err := exec.ExecContext(
					context.WithoutCancel(ctx),
					`UPDATE profile_lifecycle_op_steps SET status = 'failed', error_message = ?, updated_at = ?
				 WHERE op_id = ? AND seq = ?`,
					cause.Error(), now, opID, seq,
				); err != nil {
					return err
				}
			}
			_, err := exec.ExecContext(
				context.WithoutCancel(ctx),
				`UPDATE profile_lifecycle_ops
			 SET status = 'failed', error_code = 'profile_finalize_failed', error_message = ?, updated_at = ?
			 WHERE id = ?`,
				cause.Error(), now, opID,
			)
			return err
		},
	)
	m.recordOperationEvent(context.WithoutCancel(ctx), eventspkg.ProfileLifecycleOpFailed, opID, cause)
	return errors.Join(cause, updateErr)
}

func (m *Manager) pruneDoneOperations(ctx context.Context, keep int) error {
	if keep < 1 {
		return fmt.Errorf("profile: lifecycle retention must keep at least one completed operation")
	}
	return m.write(ctx, "prune completed profile lifecycle operations", func(exec globaldb.ProfileWriteExecutor) error {
		_, err := exec.ExecContext(ctx, `
			DELETE FROM profile_lifecycle_ops
			WHERE status = 'done' AND id NOT IN (
				SELECT id FROM profile_lifecycle_ops WHERE status = 'done'
				ORDER BY completed_at DESC, id DESC LIMIT ?
			)`, keep)
		return err
	})
}
