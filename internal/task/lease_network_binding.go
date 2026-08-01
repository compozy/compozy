package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/network/participation"
)

const taskRunNetworkBindingCleanupTimeout = 30 * time.Second

func (m *Service) bindClaimedRunNetwork(
	ctx context.Context,
	claim *ClaimResult,
	actor ActorContext,
) error {
	if claim == nil || claim.Run.NetworkSpecSnapshot().Mode != participation.ModeLive {
		return nil
	}
	binder, ok := m.sessions.(RunNetworkSessionBinder)
	if !ok {
		return nil
	}
	sessionID := strings.TrimSpace(claim.Run.SessionID)
	err := binder.BindTaskRunNetwork(ctx, sessionID, claim.Run)
	if err == nil {
		return nil
	}
	primaryErr := fmt.Errorf(
		"task: bind claimed run %q network participation for session %q: %w",
		claim.Run.ID,
		sessionID,
		err,
	)
	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		taskRunNetworkBindingCleanupTimeout,
	)
	defer cancel()
	_, releaseErr := m.ReleaseRunLease(cleanupCtx, LeaseRelease{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Reason:     "network participation binding failed",
		Now:        m.now().UTC(),
	}, actor)
	if releaseErr != nil {
		releaseErr = fmt.Errorf(
			"task: release claimed run %q after network binding failure: %w",
			claim.Run.ID,
			releaseErr,
		)
	}
	return errors.Join(primaryErr, releaseErr)
}

func (m *Service) restoreTaskRunNetworkBestEffort(
	ctx context.Context,
	sessionID string,
	runID string,
) {
	binder, ok := m.sessions.(RunNetworkSessionBinder)
	if !ok || strings.TrimSpace(sessionID) == "" {
		return
	}
	restoreCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		taskRunNetworkBindingCleanupTimeout,
	)
	defer cancel()
	if err := binder.RestoreTaskRunNetwork(restoreCtx, sessionID); err != nil {
		slog.WarnContext(
			restoreCtx,
			"task: restore session network participation after lease settlement failed",
			"session_id",
			strings.TrimSpace(sessionID),
			"run_id",
			strings.TrimSpace(runID),
			"error",
			err,
		)
	}
}
