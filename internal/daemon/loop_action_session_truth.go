package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type loopActionSessionTokenUsage interface {
	TokenUsage(context.Context, string) ([]store.TokenUsage, error)
}

// sessionProjectedTokens reads the session usage projection when the live
// prompt stream reported nothing; zero means the projection is empty too.
func (r *loopActionRuntime) sessionProjectedTokens(ctx context.Context, sessionID string) int64 {
	trimmed := strings.TrimSpace(sessionID)
	if r == nil || trimmed == "" {
		return 0
	}
	reader, ok := r.sessions.(loopActionSessionTokenUsage)
	if !ok {
		return 0
	}
	usage, err := reader.TokenUsage(ctx, trimmed)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			r.logger.Warn(
				"daemon: read loop action session token usage",
				"session_id", trimmed,
				"error", err,
			)
		}
		return 0
	}
	return sessionTokenUsageTotal(usage)
}

// persistBoundActionSession records the executing ACP session on the leased run
// so run detail and session links resolve while the node is still live.
func (r *loopActionRuntime) persistBoundActionSession(
	ctx context.Context,
	claim *taskpkg.ClaimResult,
	actor taskpkg.ActorContext,
	sessionID string,
) bool {
	if r == nil || claim == nil {
		return true
	}
	bindCtx, cancel := taskRunActivationContext(ctx)
	defer cancel()
	current, err := r.store.GetTaskRun(bindCtx, claim.Run.ID)
	if err != nil {
		r.logger.Error(
			"daemon: read loop action run session binding",
			daemonLogRunIDKey, claim.Run.ID,
			coordinatorRuntimeTaskIDKey, claim.Run.TaskID,
			"session_id", sessionID,
			"error", err,
		)
		return loopActionSessionBindingTerminalError(err)
	}
	if strings.TrimSpace(current.SessionID) == sessionID {
		return true
	}
	if _, err := r.manager.BindLeasedRunSession(bindCtx, taskpkg.LeaseSessionBinding{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		SessionID:  sessionID,
		Now:        r.now().UTC(),
	}, actor); err != nil {
		// The node execution stays healthy without the durable link; losing it
		// only degrades run detail navigation, so log instead of failing the run.
		r.logger.Error(
			"daemon: bind loop action run session",
			daemonLogRunIDKey, claim.Run.ID,
			coordinatorRuntimeTaskIDKey, claim.Run.TaskID,
			"session_id", sessionID,
			"error", err,
		)
		return loopActionSessionBindingTerminalError(err)
	}
	return true
}

func loopActionSessionBindingTerminalError(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, store.ErrClosed) ||
		errors.Is(err, taskpkg.ErrInvalidClaimToken) ||
		errors.Is(err, taskpkg.ErrLeaseExpired) ||
		errors.Is(err, taskpkg.ErrInvalidStatusTransition) ||
		errors.Is(err, taskpkg.ErrValidation) ||
		errors.Is(err, taskpkg.ErrPermissionDenied) ||
		errors.Is(err, taskpkg.ErrTaskRunNotFound) ||
		errors.Is(err, taskpkg.ErrTaskNotFound) ||
		errors.Is(err, taskpkg.ErrPayloadTooLarge) ||
		errors.Is(err, taskpkg.ErrConflict)
}
