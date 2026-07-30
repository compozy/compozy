package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

const loopDeleteRollbackTimeout = 5 * time.Second

func (s *daemonLoopAPIService) DeleteLoop(ctx context.Context, workspaceID string, name string) error {
	s.publishMu.Lock()
	defer s.publishMu.Unlock()

	ws, record, err := s.findLoopRecord(workspaceID, name)
	if err != nil {
		return err
	}
	if err := ensureWritableLoopSource(record.Spec); err != nil {
		return err
	}
	root := filepath.Dir(strings.TrimSpace(record.Spec.Dir))
	if strings.TrimSpace(root) == "." || strings.TrimSpace(root) == "" {
		return fmt.Errorf("%w: loop definition root is unavailable", looppkg.ErrValidation)
	}
	staged, err := looppkg.StageWritableDefinitionDeletion(root, name)
	if err != nil {
		return err
	}
	state, err := s.persistence.DeleteLoopDefinitionState(ctx, ws, name)
	if err != nil {
		return errors.Join(
			fmt.Errorf("daemon: delete Loop definition state %q: %w", name, err),
			wrapLoopDeleteRestoreError(name, staged.Restore()),
		)
	}
	if err := s.syncLoopResources(ctx); err != nil {
		return s.rollbackLoopDelete(ctx, staged, ws, name, state, err)
	}
	if err := staged.Commit(); err != nil {
		logger := s.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn(
			"daemon: Loop definition deletion cleanup failed after commit",
			"workspace_id", workspaceID,
			"loop_name", name,
			"error", err,
		)
	}
	return nil
}

func (s *daemonLoopAPIService) rollbackLoopDelete(
	ctx context.Context,
	staged *looppkg.StagedWritableDefinitionDeletion,
	workspaceID looppkg.WorkspaceID,
	name string,
	state looppkg.DefinitionState,
	primary error,
) error {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), loopDeleteRollbackTimeout)
	defer cancel()

	restoreStateErr := s.persistence.RestoreLoopDefinitionState(rollbackCtx, workspaceID, name, state)
	restoreDefinitionErr := staged.Restore()
	reconcileErr := s.syncLoopResources(rollbackCtx)
	return errors.Join(
		primary,
		wrapLoopDeleteStateRestoreError(name, restoreStateErr),
		wrapLoopDeleteRestoreError(name, restoreDefinitionErr),
		wrapLoopDeleteReconcileError(name, reconcileErr),
	)
}

func wrapLoopDeleteStateRestoreError(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("daemon: restore Loop definition state %q: %w", name, err)
}

func wrapLoopDeleteRestoreError(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("daemon: restore staged Loop definition %q: %w", name, err)
}

func wrapLoopDeleteReconcileError(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("daemon: reconcile restored Loop definition %q: %w", name, err)
}
