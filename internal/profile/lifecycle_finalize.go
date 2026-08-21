package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb"
)

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
	return m.pruneDoneOperations(ctx, 200)
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
		if err := m.executeStep(step); err != nil {
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
		_, err := exec.ExecContext(
			ctx,
			`UPDATE profile_lifecycle_ops
			 SET status = 'done', updated_at = ?, completed_at = ?, error_code = NULL, error_message = NULL
			 WHERE id = ? AND status = 'finalizing'`,
			formatTimestamp(m.now()), formatTimestamp(m.now()), opID,
		)
		return err
	}); err != nil {
		return m.failOperation(ctx, opID, -1, err)
	}
	if recovered {
		m.recordOperationEvent(ctx, "profile.lifecycle_op_recovered", opID, nil)
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

func (m *Manager) executeStep(step lifecycleStep) error {
	switch step.Action {
	case "mkdir_profile":
		path, err := m.containedProfilePath(step.PathNew)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("profile: create profile directory %q: %w", path, err)
		}
		return os.Chmod(path, 0o700)
	case "rename_profile":
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
			return os.MkdirAll(newPath, 0o700)
		} else if err != nil {
			return fmt.Errorf("profile: inspect rename source %q: %w", oldPath, err)
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("profile: rename %q to %q: %w", oldPath, newPath, err)
		}
		return nil
	case "remove_profile":
		path, err := m.containedProfilePath(step.PathOld)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("profile: remove profile directory %q: %w", path, err)
		}
		return nil
	default:
		return fmt.Errorf("profile: unsupported lifecycle step action %q", step.Action)
	}
}

func (m *Manager) containedProfilePath(path string) (string, error) {
	root, err := filepath.Abs(m.home.ProfilesDir)
	if err != nil {
		return "", fmt.Errorf("profile: resolve profiles root: %w", err)
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
	return candidate, nil
}

func (m *Manager) failOperation(ctx context.Context, opID string, seq int, cause error) error {
	updateErr := m.write(context.WithoutCancel(ctx), "fail profile lifecycle operation", func(exec globaldb.ProfileWriteExecutor) error {
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
	})
	m.recordOperationEvent(context.WithoutCancel(ctx), "profile.lifecycle_op_failed", opID, cause)
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
