package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/worktree"
)

func (b *taskSessionBridge) AttachTaskSession(
	ctx context.Context,
	_ string,
	sessionID string,
) (*taskpkg.SessionRef, error) {
	return b.attachTaskSession(ctx, sessionID)
}

func (b *taskSessionBridge) AttachTaskRunSession(
	ctx context.Context,
	run taskpkg.Run,
	sessionID string,
) (*taskpkg.SessionRef, error) {
	ref, err := b.attachTaskSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if runWorkspaceID := strings.TrimSpace(run.WorkspaceID); runWorkspaceID != "" &&
		strings.TrimSpace(ref.WorkspaceID) != runWorkspaceID {
		return nil, fmt.Errorf(
			"%w: attached session belongs to a different workspace",
			taskpkg.ErrSessionAttachNotAllowed,
		)
	}
	mode := run.ResolvedWorktreeModeValue().Normalize()
	if mode == "" {
		mode = taskpkg.WorktreeModeNone
	}
	switch mode {
	case taskpkg.WorktreeModeNone:
		if strings.TrimSpace(ref.WorktreeID) != "" {
			return nil, fmt.Errorf(
				"%w: root run cannot attach a worktree-bound session",
				taskpkg.ErrSessionAttachNotAllowed,
			)
		}
	case taskpkg.WorktreeModePerRun:
		return nil, fmt.Errorf("%w: per-run worktree requires a dedicated session", taskpkg.ErrSessionAttachNotAllowed)
	case taskpkg.WorktreeModeRef:
		if b.worktrees == nil {
			return nil, fmt.Errorf("%w: worktree service is unavailable", taskpkg.ErrSessionAttachNotAllowed)
		}
		item, getErr := b.worktrees.Get(
			ctx,
			strings.TrimSpace(run.WorkspaceID),
			strings.TrimSpace(run.ResolvedWorktreeRefValue()),
		)
		if getErr != nil || item == nil || item.State != worktree.StateReady ||
			strings.TrimSpace(item.ID) != strings.TrimSpace(ref.WorktreeID) {
			return nil, fmt.Errorf(
				"%w: attached session does not match resolved worktree",
				taskpkg.ErrSessionAttachNotAllowed,
			)
		}
	default:
		return nil, fmt.Errorf("%w: unsupported resolved worktree mode %q", taskpkg.ErrValidation, mode)
	}
	return ref, nil
}

func (b *taskSessionBridge) attachTaskSession(
	ctx context.Context,
	sessionID string,
) (*taskpkg.SessionRef, error) {
	if ctx == nil {
		return nil, errors.New("daemon: attach task session context is required")
	}

	info, err := b.sessions.Status(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf(
			"%w: session %q is unavailable",
			taskpkg.ErrSessionAttachNotAllowed,
			strings.TrimSpace(sessionID),
		)
	}
	if !isTaskSessionStateLive(info.State) {
		return nil, fmt.Errorf(
			"%w: session %q is %q",
			taskpkg.ErrSessionAttachNotAllowed,
			strings.TrimSpace(sessionID),
			info.State,
		)
	}

	return &taskpkg.SessionRef{
		SessionID:   strings.TrimSpace(info.ID),
		WorkspaceID: strings.TrimSpace(info.WorkspaceID),
		WorktreeID:  strings.TrimSpace(info.WorktreeID),
		StartedAt:   info.CreatedAt,
	}, nil
}

func (b *taskSessionBridge) RequestTaskStop(
	ctx context.Context,
	sessionID string,
	reason taskpkg.StopReason,
) error {
	if ctx == nil {
		return errors.New("daemon: request task stop context is required")
	}

	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return fmt.Errorf("%w: task session stop id is required", taskpkg.ErrValidation)
	}

	if requester, ok := b.sessions.(taskBridgeSessionRequestStopper); ok {
		if err := requester.RequestStopWithCause(
			ctx,
			trimmedID,
			taskStopCause(reason),
			taskStopDetail(reason),
		); err != nil {
			if errors.Is(err, session.ErrSessionNotFound) {
				return nil
			}
			return err
		}
		return nil
	}

	return b.ForceTaskStop(ctx, trimmedID, reason)
}

func (b *taskSessionBridge) ForceTaskStop(
	ctx context.Context,
	sessionID string,
	reason taskpkg.StopReason,
) error {
	if ctx == nil {
		return errors.New("daemon: force task stop context is required")
	}

	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return fmt.Errorf("%w: task session stop id is required", taskpkg.ErrValidation)
	}

	if err := b.sessions.StopWithCause(ctx, trimmedID, taskStopCause(reason), taskStopDetail(reason)); err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return nil
		}
		return err
	}
	return nil
}
