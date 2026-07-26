package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/coordinator"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
)

// reconcileCreatedCoordinator resolves the coordinator singleton after a fresh
// session was created: it supersedes a concurrently created coordinator or
// promotes the new session, dispatching the matching lifecycle decision.
func (r *coordinatorRuntime) reconcileCreatedCoordinator(
	ctx context.Context,
	info *session.Info,
	decision coordinator.Decision,
	createdCfg aghconfig.ResolvedCoordinatorRole,
	reason string,
) (*session.Info, bool, error) {
	r.mu.Lock()
	existing, err := r.activeCoordinator(ctx, decision.WorkspaceID)
	if err != nil {
		r.mu.Unlock()
		cleanupErr := r.cleanupCreatedCoordinatorSession(
			ctx,
			info,
			"coordinator singleton reconciliation failed",
		)
		if cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
		r.dispatchFailed(ctx, decision, participation.CloneSpec(info.NetworkParticipation), reason, err)
		return nil, false, err
	}
	if existing != nil && strings.TrimSpace(existing.ID) != strings.TrimSpace(info.ID) {
		shouldPrompt := r.beginCoordinatorWakeLocked(existing, decision)
		r.mu.Unlock()
		existingParticipation := participation.CloneSpec(existing.NetworkParticipation)
		r.dispatchDecision(ctx, decision, existingParticipation, reason, coordinator.DecisionExisting)
		if err := r.cleanupCreatedCoordinatorSession(
			ctx,
			info,
			"duplicate coordinator session superseded",
		); err != nil {
			if shouldPrompt {
				r.finishCoordinatorWake(existing, decision)
			}
			r.dispatchFailed(ctx, decision, participation.CloneSpec(info.NetworkParticipation), reason, err)
			return existing, false, err
		}
		if err := r.wakeCoordinatorIfNeeded(ctx, existing, decision, reason, shouldPrompt); err != nil {
			r.dispatchFailed(ctx, decision, existingParticipation, reason, err)
			return existing, false, err
		}
		return existing, false, nil
	}
	shouldPrompt := r.beginCoordinatorWakeLocked(info, decision)
	r.mu.Unlock()
	if err := r.wakeCoordinatorIfNeeded(ctx, info, decision, reason, shouldPrompt); err != nil {
		r.dispatchFailed(ctx, decision, participation.CloneSpec(info.NetworkParticipation), reason, err)
		return nil, false, err
	}
	r.dispatchSpawned(ctx, decision, info, createdCfg, reason)
	return info, true, nil
}

// wakeCoordinatorIfNeeded prompts the coordinator when a wake was begun and
// always clears the wake state afterwards. The caller owns failure dispatch.
func (r *coordinatorRuntime) wakeCoordinatorIfNeeded(
	ctx context.Context,
	target *session.Info,
	decision coordinator.Decision,
	reason string,
	shouldPrompt bool,
) error {
	if !shouldPrompt {
		return nil
	}
	if err := r.promptCoordinator(ctx, target, decision, reason); err != nil {
		r.finishCoordinatorWake(target, decision)
		return err
	}
	r.finishCoordinatorWake(target, decision)
	return nil
}

func (r *coordinatorRuntime) cleanupCreatedCoordinatorSession(
	ctx context.Context,
	info *session.Info,
	detail string,
) error {
	if r == nil || info == nil {
		return nil
	}
	sessionID := strings.TrimSpace(info.ID)
	if sessionID == "" {
		return nil
	}
	return r.stopCoordinatorSessionWithCause(ctx, sessionID, detail)
}

func (r *coordinatorRuntime) createCoordinatorSession(
	ctx context.Context,
	decision coordinator.Decision,
	cfg aghconfig.ResolvedCoordinatorRole,
	reason string,
) (*session.Info, aghconfig.ResolvedCoordinatorRole, bool, error) {
	coordinatorParticipation, err := bindCoordinatorParticipation(decision)
	if err != nil {
		r.dispatchFailed(ctx, decision, nil, reason, err)
		return nil, cfg, false, err
	}
	preSpawn := r.preSpawnPayload(decision, cfg, coordinatorParticipation, reason)
	preSpawn, err = r.dispatchPreSpawn(ctx, preSpawn)
	if err != nil {
		if preSpawn.Denied {
			r.dispatchDecision(ctx, decision, &coordinatorParticipation, reason, coordinator.DecisionDenied)
			return nil, cfg, false, nil
		}
		r.dispatchFailed(ctx, decision, &coordinatorParticipation, reason, err)
		return nil, cfg, false, err
	}
	if preSpawn.Denied {
		r.dispatchDecision(ctx, decision, &coordinatorParticipation, reason, coordinator.DecisionDenied)
		return nil, cfg, false, nil
	}

	cfg.AgentName = firstNonEmpty(preSpawn.AgentName, cfg.AgentName)
	cfg.Provider = firstNonEmpty(preSpawn.Provider, cfg.Provider)
	cfg.Model = firstNonEmpty(preSpawn.Model, cfg.Model)
	info, err := r.startCoordinatorSession(ctx, decision, cfg, coordinatorParticipation)
	if err != nil {
		r.dispatchFailed(ctx, decision, &coordinatorParticipation, reason, err)
		return nil, cfg, false, err
	}
	return info, cfg, true, nil
}

func (r *coordinatorRuntime) promptCoordinator(
	ctx context.Context,
	info *session.Info,
	decision coordinator.Decision,
	reason string,
) error {
	if ctx == nil {
		return errors.New("daemon: coordinator prompt context is required")
	}
	if info == nil {
		return errors.New("daemon: coordinator prompt requires session info")
	}
	sessionID := strings.TrimSpace(info.ID)
	if sessionID == "" {
		return errors.New("daemon: coordinator prompt requires session id")
	}
	message := coordinatorWakeMessage(decision)
	events, err := r.sessions.PromptSynthetic(ctx, sessionID, session.SyntheticPromptOpts{
		Message: message,
		Metadata: acp.PromptSyntheticMeta{
			TaskID:               strings.TrimSpace(decision.TaskID),
			TaskRunID:            strings.TrimSpace(decision.RunID),
			WorkflowID:           strings.TrimSpace(decision.WorkflowID),
			CoordinatorSessionID: sessionID,
			Reason:               strings.TrimSpace(reason),
			Summary:              coordinatorWakeSummary(decision),
		},
		InterruptIfAgentWaiting: true,
	})
	if err != nil {
		return fmt.Errorf("daemon: prompt coordinator session %q: %w", sessionID, err)
	}
	r.drainPromptEvents(sessionID, strings.TrimSpace(decision.RunID), events)
	return nil
}

func (r *coordinatorRuntime) drainPromptEvents(sessionID string, runID string, events <-chan acp.AgentEvent) {
	if r == nil || events == nil {
		return
	}
	r.wg.Go(func() {
		drainCoordinatorPromptEvents(r.ctx, r.logger, sessionID, runID, events)
	})
}

func drainCoordinatorPromptEvents(
	ctx context.Context,
	logger *slog.Logger,
	sessionID string,
	runID string,
	events <-chan acp.AgentEvent,
) {
	if events == nil {
		return
	}
	if ctx == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.Type == acp.EventTypeError && logger != nil {
				logger.Warn(
					"daemon: coordinator prompt returned agent error",
					"session_id", sessionID,
					daemonLogRunIDKey, runID,
				)
			}
		}
	}
}
