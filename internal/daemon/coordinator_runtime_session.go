package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/coordinator"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
)

func (r *coordinatorRuntime) startCoordinatorSession(
	ctx context.Context,
	decision coordinator.Decision,
	cfg aghconfig.ResolvedCoordinatorRole,
	coordinatorParticipation participation.Spec,
) (*session.Info, error) {
	policy := coordinator.PermissionPolicy(coordinatorParticipation)
	now := r.now().UTC()
	promptOverlay := coordinator.PromptOverlay(coordinator.PromptInput{
		WorkspaceID:          decision.WorkspaceID,
		TaskID:               decision.TaskID,
		RunID:                decision.RunID,
		WorkflowID:           decision.WorkflowID,
		NetworkParticipation: coordinatorParticipation,
	})
	if r.contextOverlay != nil &&
		strings.TrimSpace(decision.TaskID) != "" &&
		strings.TrimSpace(decision.RunID) != "" {
		taskRecord, err := r.store.GetTask(ctx, decision.TaskID)
		if err != nil {
			return nil, fmt.Errorf("daemon: load coordinator task context task %q: %w", decision.TaskID, err)
		}
		run, err := r.store.GetTaskRun(ctx, decision.RunID)
		if err != nil {
			return nil, fmt.Errorf("daemon: load coordinator task context run %q: %w", decision.RunID, err)
		}
		taskOverlay, err := r.contextOverlay.TaskRunPromptOverlay(ctx, taskRecord, run, nil)
		if err != nil {
			return nil, fmt.Errorf("daemon: render coordinator task context overlay: %w", err)
		}
		promptOverlay = joinPromptOverlays(taskOverlay, promptOverlay)
	}
	role := coordinatorInvocationRole(cfg, r.roleEvents)
	correlation := roleInvocationCorrelationFromContext(ctx, decision.WorkspaceID)
	created, err := invokeRoleWithFallback(ctx, role, correlation, func(
		attemptCtx context.Context,
		route roleAttemptRoute,
	) (*session.Session, bool, error) {
		spawned, createErr := r.sessions.Create(attemptCtx, session.CreateOpts{
			AgentName:                    route.AgentName,
			Provider:                     route.Provider,
			Model:                        route.Model,
			ReasoningEffort:              route.ReasoningEffort,
			Name:                         coordinatorSessionName(decision.WorkspaceID),
			Workspace:                    decision.WorkspaceID,
			ResolvedNetworkParticipation: &coordinatorParticipation,
			PromptOverlay:                promptOverlay,
			Type:                         session.SessionTypeCoordinator,
			Lineage:                      coordinator.Lineage(now, cfg, policy),
		})
		return spawned, spawned != nil, createErr
	})
	if err != nil {
		if created != nil {
			cleanupErr := r.stopFailedCoordinatorSession(ctx, created, err)
			return nil, errors.Join(fmt.Errorf("daemon: create coordinator session: %w", err), cleanupErr)
		}
		return nil, fmt.Errorf("daemon: create coordinator session: %w", err)
	}
	if created == nil {
		return nil, errors.New("daemon: coordinator session create returned nil")
	}
	info := created.Info()
	if info == nil {
		return nil, errors.New("daemon: coordinator session create returned nil info")
	}
	return info, nil
}

func (r *coordinatorRuntime) stopFailedCoordinatorSession(
	ctx context.Context,
	created *session.Session,
	cause error,
) error {
	if created == nil {
		return nil
	}
	info := created.Info()
	if info == nil || strings.TrimSpace(info.ID) == "" {
		return errors.New("daemon: accepted coordinator session returned no cleanup identity")
	}
	return r.stopCoordinatorSessionWithCause(ctx, info.ID, cause.Error())
}

func coordinatorInvocationRole(
	cfg aghconfig.ResolvedCoordinatorRole,
	events roleEventSummaryWriter,
) ResolvedRole {
	return ResolvedRole{
		Role:            aghconfig.RoleCoordinator,
		AgentName:       cfg.AgentName,
		Provider:        cfg.Provider,
		Model:           cfg.Model,
		ReasoningEffort: cfg.ReasoningEffort,
		Fallbacks:       append([]aghconfig.RoleFallback(nil), cfg.Fallbacks...),
		eventWriter:     events,
	}
}

func (r *coordinatorRuntime) stopCoordinatorSessionWithCause(
	ctx context.Context,
	sessionID string,
	detail string,
) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("daemon: accepted coordinator session returned no cleanup identity")
	}
	stopCtx, cancel := detachedDaemonOperationContext(ctx, coordinatorRuntimeCleanupTimeout)
	defer cancel()
	if err := r.sessions.StopWithCause(stopCtx, sessionID, session.CauseFailed, detail); err != nil {
		return fmt.Errorf("daemon: stop coordinator session %q: %w", sessionID, err)
	}
	return nil
}

func (r *coordinatorRuntime) activeCoordinator(ctx context.Context, workspaceID string) (*session.Info, error) {
	infos, err := r.sessions.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list sessions for coordinator singleton: %w", err)
	}
	now := r.now().UTC()
	for _, info := range infos {
		if coordinator.HealthySession(info, workspaceID, now) {
			return info, nil
		}
	}
	return nil, nil
}

func defaultEnabledCoordinatorRole() aghconfig.ResolvedCoordinatorRole {
	cfg := aghconfig.DefaultResolvedCoordinatorRole()
	cfg.Enabled = true
	return cfg
}

func coordinatorSessionName(workspaceID string) string {
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" {
		return "AGH Coordinator"
	}
	return "AGH Coordinator " + trimmed
}
