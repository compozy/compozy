package daemon

import (
	"context"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/coordinator"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
)

func bindCoordinatorParticipation(decision coordinator.Decision) (participation.Spec, error) {
	spec := decision.NetworkParticipation
	if err := participation.ValidateSpec(spec); err != nil {
		return participation.Spec{}, fmt.Errorf("daemon: validate coordinator network participation: %w", err)
	}
	if spec.Mode == participation.ModeLive &&
		strings.TrimSpace(spec.WorkspaceID) != strings.TrimSpace(decision.WorkspaceID) {
		return participation.Spec{}, fmt.Errorf(
			"daemon: coordinator participation workspace %q does not match coordinator workspace %q",
			spec.WorkspaceID,
			decision.WorkspaceID,
		)
	}
	return spec, nil
}

func (r *coordinatorRuntime) dispatchPreSpawn(
	ctx context.Context,
	payload hookspkg.CoordinatorPreSpawnPayload,
) (hookspkg.CoordinatorPreSpawnPayload, error) {
	if r.hooks == nil {
		return payload, nil
	}
	result, err := r.hooks.DispatchCoordinatorPreSpawn(ctx, payload)
	if err != nil {
		return result, fmt.Errorf("daemon: dispatch coordinator pre-spawn hook: %w", err)
	}
	return result, nil
}

func (r *coordinatorRuntime) dispatchSpawned(
	ctx context.Context,
	decision coordinator.Decision,
	info *session.Info,
	cfg aghconfig.ResolvedCoordinatorRole,
	reason string,
) {
	if r.hooks == nil || info == nil {
		return
	}
	_, err := r.hooks.DispatchCoordinatorSpawned(ctx, hookspkg.CoordinatorSpawnedPayload{
		PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookCoordinatorSpawned, Timestamp: r.now().UTC()},
		CoordinatorContext: hookspkg.CoordinatorContext{
			WorkspaceID:                  decision.WorkspaceID,
			Workspace:                    info.Workspace,
			AgentName:                    info.AgentName,
			CoordinatorSessionID:         info.ID,
			TaskID:                       decision.TaskID,
			RunID:                        decision.RunID,
			WorkflowID:                   decision.WorkflowID,
			ResolvedNetworkParticipation: participation.CloneSpec(info.NetworkParticipation),
			Provider:                     cfg.Provider,
			Model:                        cfg.Model,
		},
		DecisionKind: "lifecycle",
		Decision:     reason,
	})
	if err != nil {
		r.logger.Warn("daemon: dispatch coordinator spawned hook failed", "error", err)
	}
}

func (r *coordinatorRuntime) dispatchStopped(ctx context.Context, info *session.Info) {
	if r.hooks == nil || info == nil {
		return
	}
	_, err := r.hooks.DispatchCoordinatorStopped(ctx, hookspkg.CoordinatorStoppedPayload{
		PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookCoordinatorStopped, Timestamp: r.now().UTC()},
		CoordinatorContext: hookspkg.CoordinatorContext{
			WorkspaceID:                  info.WorkspaceID,
			Workspace:                    info.Workspace,
			AgentName:                    info.AgentName,
			CoordinatorSessionID:         info.ID,
			ResolvedNetworkParticipation: participation.CloneSpec(info.NetworkParticipation),
			Provider:                     info.Provider,
		},
		DecisionKind: "lifecycle",
		Decision:     coordinator.ReasonCoordinatorStopped,
		StopReason:   string(info.StopReason),
	})
	if err != nil {
		r.logger.Warn("daemon: dispatch coordinator stopped hook failed", "error", err)
	}
}

func (r *coordinatorRuntime) dispatchFailed(
	ctx context.Context,
	decision coordinator.Decision,
	coordinatorParticipation *participation.Spec,
	reason string,
	failed error,
) {
	if r.hooks == nil || failed == nil {
		return
	}
	_, err := r.hooks.DispatchCoordinatorFailed(ctx, hookspkg.CoordinatorFailedPayload{
		PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookCoordinatorFailed, Timestamp: r.now().UTC()},
		CoordinatorContext: hookspkg.CoordinatorContext{
			WorkspaceID:                  decision.WorkspaceID,
			TaskID:                       decision.TaskID,
			RunID:                        decision.RunID,
			WorkflowID:                   decision.WorkflowID,
			ResolvedNetworkParticipation: cloneCoordinatorParticipation(coordinatorParticipation),
		},
		DecisionKind: "bootstrap",
		Decision:     reason,
		Error:        failed.Error(),
	})
	if err != nil {
		r.logger.Warn("daemon: dispatch coordinator failed hook failed", "error", err)
	}
}

func (r *coordinatorRuntime) dispatchDecision(
	ctx context.Context,
	decision coordinator.Decision,
	coordinatorParticipation *participation.Spec,
	reason string,
	override string,
) {
	if r.hooks == nil {
		return
	}
	value := decision.Reason
	if strings.TrimSpace(override) != "" {
		value = strings.TrimSpace(override)
	}
	_, err := r.hooks.DispatchCoordinatorDecision(ctx, hookspkg.CoordinatorDecisionPayload{
		PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookCoordinatorDecision, Timestamp: r.now().UTC()},
		CoordinatorContext: hookspkg.CoordinatorContext{
			WorkspaceID:                  decision.WorkspaceID,
			TaskID:                       decision.TaskID,
			RunID:                        decision.RunID,
			WorkflowID:                   decision.WorkflowID,
			ResolvedNetworkParticipation: cloneCoordinatorParticipation(coordinatorParticipation),
		},
		DecisionKind: "bootstrap",
		Decision:     firstNonEmpty(value, reason),
	})
	if err != nil {
		r.logger.Warn("daemon: dispatch coordinator decision hook failed", "error", err)
	}
}

func (r *coordinatorRuntime) preSpawnPayload(
	decision coordinator.Decision,
	cfg aghconfig.ResolvedCoordinatorRole,
	coordinatorParticipation participation.Spec,
	reason string,
) hookspkg.CoordinatorPreSpawnPayload {
	return hookspkg.CoordinatorPreSpawnPayload{
		PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookCoordinatorPreSpawn, Timestamp: r.now().UTC()},
		CoordinatorContext: hookspkg.CoordinatorContext{
			WorkspaceID:                  decision.WorkspaceID,
			AgentName:                    cfg.AgentName,
			TaskID:                       decision.TaskID,
			RunID:                        decision.RunID,
			WorkflowID:                   decision.WorkflowID,
			ResolvedNetworkParticipation: participation.CloneSpec(coordinatorParticipation),
			Provider:                     cfg.Provider,
			Model:                        cfg.Model,
		},
		Reason: reason,
	}
}

func cloneCoordinatorParticipation(spec *participation.Spec) *participation.Spec {
	if spec == nil {
		return nil
	}
	return participation.CloneSpec(*spec)
}
