package task

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
)

type coordinatorResultContext struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Name        string `json:"loop_name,omitempty"`
	ParentRunID string `json:"parent_loop_run_id,omitempty"`
	Generation  int    `json:"generation,omitempty"`
	Status      string `json:"status,omitempty"`
}

func (m *Service) dispatchCoordinatorTerminal(
	ctx context.Context,
	result *CoordinatorCompletionResult,
	actor ActorContext,
) {
	loopContext, loopStatus, ok := m.coordinatorTerminalContext(result)
	if !ok || m.coordinatorHookOK == nil || !m.coordinatorHookOK(loopStatus) {
		return
	}
	dispatcher, ok := m.taskHooks.(terminalHookDispatcher)
	if !ok || dispatcher == nil {
		return
	}
	runKind := result.Run.RunKind.Normalize().String()
	payload := hookspkg.LoopTerminalPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookLoopTerminal,
			Timestamp: m.now().UTC(),
		},
		LoopContext: hookspkg.LoopContext{
			LoopRunID:                    strings.TrimSpace(result.LoopRunID),
			ParentLoopRunID:              strings.TrimSpace(loopContext.ParentRunID),
			WorkspaceID:                  strings.TrimSpace(loopContext.WorkspaceID),
			LoopName:                     strings.TrimSpace(loopContext.Name),
			Generation:                   loopContext.Generation,
			TaskID:                       strings.TrimSpace(result.Run.TaskID),
			RunID:                        strings.TrimSpace(result.Run.ID),
			RunKind:                      runKind,
			WorkflowID:                   taskRunMetadataString(result.Run.Metadata, "workflow_id"),
			ResolvedNetworkParticipation: participation.CloneSpec(result.Run.NetworkSpecSnapshot()),
			AgentName:                    taskRunHookAgentName(result.Run, actor),
			SessionID:                    strings.TrimSpace(result.Run.SessionID),
			ActorKind:                    string(actor.Actor.Kind.Normalize()),
			ActorID:                      strings.TrimSpace(actor.Actor.Ref),
			OriginKind:                   string(actor.Origin.Kind.Normalize()),
			OriginRef:                    strings.TrimSpace(actor.Origin.Ref),
		},
		Status: strings.TrimSpace(loopStatus),
	}
	_, err := dispatcher.DispatchLoopTerminal(taskRunObservationHookContext(ctx), payload)
	m.reportTerminalHookFailure(hookspkg.HookLoopTerminal, err, payload)
}

func (m *Service) coordinatorTerminalContext(
	result *CoordinatorCompletionResult,
) (coordinatorResultContext, string, bool) {
	if result == nil || len(result.Context) == 0 {
		return coordinatorResultContext{}, "", false
	}
	var payload coordinatorResultContext
	if err := json.Unmarshal(result.Context, &payload); err != nil {
		slog.Error(
			"task: coordinator terminal context decode failed",
			"error", err,
			"loop_run_id", result.LoopRunID,
		)
		return coordinatorResultContext{}, "", false
	}
	return payload, strings.TrimSpace(payload.Status), true
}

func (m *Service) reportTerminalHookFailure(
	event hookspkg.HookEvent,
	err error,
	payload hookspkg.LoopTerminalPayload,
) {
	if err == nil {
		return
	}
	slog.Error(
		"task: coordinator terminal hook failed after committed mutation",
		"error", err,
		"hook_event", event,
		"loop_run_id", payload.LoopRunID,
		"loop_name", payload.LoopName,
		"loop_status", payload.Status,
	)
}
