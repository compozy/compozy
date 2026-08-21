package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store/globaldb"
)

// Journaled filesystem and support-state steps. The journal is the record a crash
// recovers from, so every step name is durable and forward-only.
const (
	stepMkdirProfile            = "mkdir_profile"
	stepWriteDeclaredSeed       = "write_declared_seed"
	stepRenameProfile           = "rename_profile"
	stepRemoveProfile           = "remove_profile"
	stepRemoveDesktopPartitions = "remove_desktop_partitions"
)

const lifecycleOperationRetentionWindow = 30 * 24 * time.Hour

const (
	opStatusApplied    = "applied"
	opStatusFinalizing = "finalizing"
	opStatusDone       = "done"
	opStatusFailed     = "failed"
)

type lifecycleStep struct {
	Seq              int
	Action           string
	PathOld, PathNew string
	Status           string
}

func (m *Manager) insertOperation(
	ctx context.Context,
	exec globaldb.ProfileWriteExecutor,
	opID, kind, profileID, oldName, newName, revision string,
	steps []lifecycleStep,
) error {
	now := formatTimestamp(m.now())
	if _, err := exec.ExecContext(
		ctx,
		`INSERT INTO profile_lifecycle_ops
		 (id, kind, profile_id, old_name, new_name, plan_revision, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'applied', ?, ?)`,
		opID, kind, profileID, nullableString(oldName), nullableString(newName), revision, now, now,
	); err != nil {
		return fmt.Errorf("profile: insert lifecycle operation %q: %w", opID, err)
	}
	for _, step := range steps {
		if _, err := exec.ExecContext(
			ctx,
			`INSERT INTO profile_lifecycle_op_steps
			 (op_id, seq, action, path_old, path_new, status, updated_at)
			 VALUES (?, ?, ?, ?, ?, 'pending', ?)`,
			opID, step.Seq, step.Action, nullableString(step.PathOld), nullableString(step.PathNew), now,
		); err != nil {
			return fmt.Errorf("profile: insert lifecycle operation %q step %d: %w", opID, step.Seq, err)
		}
	}
	return nil
}

func ensureNameAvailable(
	ctx context.Context,
	q queryer,
	name string,
	excludingProfileID string,
) error {
	var holderID, holderState string
	err := q.QueryRowContext(
		ctx,
		`SELECT id, state FROM profiles WHERE name = ? AND id <> ?`,
		name,
		excludingProfileID,
	).Scan(&holderID, &holderState)
	if err == nil {
		return domainError(
			"profile_name_taken",
			fmt.Sprintf("profile name %q is held by profile %s (%s)", name, holderID, holderState),
			"choose another profile name",
			ErrNameTaken,
		)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("profile: check name holder for %q: %w", name, err)
	}
	var opID string
	err = q.QueryRowContext(
		ctx,
		`SELECT id FROM profile_lifecycle_ops
		 WHERE status <> 'done' AND (old_name = ? OR new_name = ?)
		 ORDER BY created_at LIMIT 1`,
		name,
		name,
	).Scan(&opID)
	if err == nil {
		return domainError(
			"profile_name_taken",
			fmt.Sprintf("profile name %q is reserved by lifecycle operation %s", name, opID),
			"retry after the lifecycle operation completes",
			ErrNameTaken,
		)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("profile: check lifecycle reservation for %q: %w", name, err)
	}
	return nil
}

func (m *Manager) profileDir(name string) string { return filepath.Join(m.home.ProfilesDir, name) }

func (m *Manager) ListOps(ctx context.Context) (ops []LifecycleOp, err error) {
	if ctx == nil {
		return nil, errors.New("profile: list operations context is required")
	}
	rows, err := m.store.DB().QueryContext(ctx, `
		SELECT o.id, o.kind, COALESCE(p.name, o.new_name, o.old_name, ''), o.status,
		       COALESCE((SELECT action FROM profile_lifecycle_op_steps s
		                 WHERE s.op_id = o.id AND s.status <> 'done' ORDER BY s.seq LIMIT 1), ''),
		       COALESCE(o.error_message, '')
		FROM profile_lifecycle_ops o
		LEFT JOIN profiles p ON p.id = o.profile_id
		WHERE o.status <> 'done' OR o.completed_at >= ?
		ORDER BY o.created_at DESC, o.id DESC`,
		formatTimestamp(m.now().Add(-lifecycleOperationRetentionWindow)),
	)
	if err != nil {
		return nil, fmt.Errorf("profile: list lifecycle operations: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("profile: close lifecycle operation rows: %w", closeErr))
		}
	}()
	ops = make([]LifecycleOp, 0)
	for rows.Next() {
		var op LifecycleOp
		if err := rows.Scan(&op.ID, &op.Kind, &op.Profile, &op.Status, &op.Step, &op.Error); err != nil {
			return nil, fmt.Errorf("profile: scan lifecycle operation: %w", err)
		}
		ops = append(ops, op)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("profile: iterate lifecycle operations: %w", err)
	}
	return ops, nil
}

func (m *Manager) RetryOp(ctx context.Context, opID string) (LifecycleOp, error) {
	opID = strings.TrimSpace(opID)
	if opID == "" {
		return LifecycleOp{}, fmt.Errorf("%w: lifecycle operation id is required", ErrInvalidInput)
	}
	m.opMu.Lock()
	defer m.opMu.Unlock()
	if err := m.write(ctx, "retry profile lifecycle operation", func(exec globaldb.ProfileWriteExecutor) error {
		result, err := exec.ExecContext(
			ctx,
			`UPDATE profile_lifecycle_ops
			 SET status = 'applied', error_code = NULL, error_message = NULL, updated_at = ?
			 WHERE id = ? AND status = 'failed'`,
			formatTimestamp(m.now()),
			opID,
		)
		if err != nil {
			return fmt.Errorf("profile: retry operation %q: %w", opID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("profile: inspect retry operation %q: %w", opID, err)
		}
		if affected != 1 {
			return domainError(
				"profile_op_not_retryable",
				fmt.Sprintf("profile operation %s is not failed", opID),
				"run compozy profile ops",
				ErrInvalidInput,
			)
		}
		_, err = exec.ExecContext(
			ctx,
			`UPDATE profile_lifecycle_op_steps
			 SET status = 'pending', error_message = NULL, updated_at = ?
			 WHERE op_id = ? AND status = 'failed'`,
			formatTimestamp(m.now()),
			opID,
		)
		return err
	}); err != nil {
		return LifecycleOp{}, err
	}
	if err := m.finalizeOperation(context.WithoutCancel(ctx), opID, false); err != nil {
		return LifecycleOp{}, err
	}
	return m.getOperation(ctx, opID)
}

func (m *Manager) getOperation(ctx context.Context, opID string) (LifecycleOp, error) {
	var op LifecycleOp
	err := m.store.DB().QueryRowContext(ctx, `
		SELECT o.id, o.kind, COALESCE(p.name, o.new_name, o.old_name, ''), o.status,
		       COALESCE((SELECT action FROM profile_lifecycle_op_steps s
		                 WHERE s.op_id = o.id AND s.status <> 'done' ORDER BY s.seq LIMIT 1), ''),
		       COALESCE(o.error_message, '')
		FROM profile_lifecycle_ops o LEFT JOIN profiles p ON p.id = o.profile_id WHERE o.id = ?`, opID,
	).Scan(&op.ID, &op.Kind, &op.Profile, &op.Status, &op.Step, &op.Error)
	if errors.Is(err, sql.ErrNoRows) {
		return LifecycleOp{}, domainError(
			"profile_op_not_found",
			"profile lifecycle operation was not found",
			"run compozy profile ops",
			ErrNotFound,
		)
	}
	if err != nil {
		return LifecycleOp{}, fmt.Errorf("profile: get lifecycle operation %q: %w", opID, err)
	}
	return op, nil
}
