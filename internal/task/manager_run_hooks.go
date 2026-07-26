package task

import (
	"context"
	"encoding/json"

	"log/slog"

	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (m *Service) dispatchTaskRunEnqueued(
	ctx context.Context,
	run Run,
	taskRecord Task,
	actor ActorContext,
	idempotencyKey string,
) {
	payload := hookspkg.TaskRunEnqueuedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskRunEnqueued,
			Timestamp: m.now().UTC(),
		},
		TaskRunContext: m.taskRunHookContext(run, taskRecord, actor),
		IdempotencyKey: strings.TrimSpace(idempotencyKey),
	}
	_, err := m.taskHooks.DispatchTaskRunEnqueued(taskRunObservationHookContext(ctx), payload)
	m.reportTaskRunHookFailure(hookspkg.HookTaskRunEnqueued, err, run, taskRecord)
}

func (m *Service) dispatchTaskRunPostClaim(
	ctx context.Context,
	run Run,
	taskRecord Task,
	actor ActorContext,
) {
	payload := hookspkg.TaskRunPostClaimPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskRunPostClaim,
			Timestamp: m.now().UTC(),
		},
		TaskRunContext: m.taskRunHookContext(run, taskRecord, actor),
		ClaimedAt:      run.ClaimedAt,
	}
	_, err := m.taskHooks.DispatchTaskRunPostClaim(taskRunObservationHookContext(ctx), payload)
	m.reportTaskRunHookFailure(hookspkg.HookTaskRunPostClaim, err, run, taskRecord)
}

func (m *Service) dispatchTaskRunLeaseRecovered(
	ctx context.Context,
	run Run,
	taskRecord Task,
	actor ActorContext,
	previousStatus RunStatus,
	previousSessionID string,
	recovery RunBootRecovery,
) {
	payload := hookspkg.TaskRunLeaseRecoveredPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskRunLeaseRecovered,
			Timestamp: m.now().UTC(),
		},
		TaskRunContext:    m.taskRunHookContext(run, taskRecord, actor),
		PreviousRunStatus: previousStatus.Normalize().String(),
		PreviousSessionID: strings.TrimSpace(previousSessionID),
		RecoveryAction:    string(recovery.Action.Normalize()),
		RecoveryReason:    strings.TrimSpace(recovery.Reason),
	}
	_, err := m.taskHooks.DispatchTaskRunLeaseRecovered(taskRunObservationHookContext(ctx), payload)
	m.reportTaskRunHookFailure(hookspkg.HookTaskRunLeaseRecovered, err, run, taskRecord)
}

func (m *Service) reportTaskRunHookFailure(
	event hookspkg.HookEvent,
	err error,
	run Run,
	taskRecord Task,
) {
	if err == nil {
		return
	}
	slog.Error(
		"task: task-run lifecycle hook failed after committed mutation",
		"error", err,
		"hook_event", event,
		"task_id", taskRecord.ID,
		"run_id", run.ID,
		"run_status", run.Status,
	)
}

func (m *Service) taskRunHookContext(
	run Run,
	taskRecord Task,
	actor ActorContext,
) hookspkg.TaskRunContext {
	soulSnapshotID, soulDigest := taskRunSoulMetadata(run.Metadata)
	runKind := run.RunKind.Normalize().String()
	networkSpec := run.NetworkSpecSnapshot()
	workspaceID := strings.TrimSpace(run.WorkspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(taskRecord.WorkspaceID)
	}
	wakeID, targetSessionID, ownerKey := run.NetworkWakeCorrelation()
	return hookspkg.TaskRunContext{
		TaskID:                       strings.TrimSpace(run.TaskID),
		RunID:                        strings.TrimSpace(run.ID),
		RunKind:                      &runKind,
		WakeID:                       strings.TrimSpace(wakeID),
		OwnerKey:                     strings.TrimSpace(ownerKey),
		TargetSessionID:              strings.TrimSpace(targetSessionID),
		LoopRunID:                    strings.TrimSpace(run.LoopRunID),
		WorkspaceID:                  workspaceID,
		WorkflowID:                   taskRunMetadataString(run.Metadata, "workflow_id"),
		ResolvedNetworkParticipation: new(networkSpec),
		AgentName:                    taskRunHookAgentName(run, actor),
		SessionID:                    strings.TrimSpace(run.SessionID),
		ActorKind:                    string(actor.Actor.Kind.Normalize()),
		ActorID:                      strings.TrimSpace(actor.Actor.Ref),
		OriginKind:                   string(actor.Origin.Kind.Normalize()),
		OriginRef:                    strings.TrimSpace(actor.Origin.Ref),
		TaskStatus:                   string(taskRecord.Status.Normalize()),
		RunStatus:                    run.Status.Normalize().String(),
		SoulSnapshotID:               soulSnapshotID,
		SoulDigest:                   soulDigest,
		Attempt:                      int(run.Attempt),
		LeaseUntil:                   run.LeaseUntil,
		Error:                        strings.TrimSpace(run.Error),
	}
}

func taskRunSoulMetadata(metadata json.RawMessage) (string, string) {
	if len(metadata) == 0 {
		return "", ""
	}
	var envelope struct {
		Soul *struct {
			SnapshotID string `json:"snapshot_id"`
			Digest     string `json:"digest"`
		} `json:"soul"`
	}
	if err := json.Unmarshal(metadata, &envelope); err != nil || envelope.Soul == nil {
		return "", ""
	}
	return strings.TrimSpace(envelope.Soul.SnapshotID), strings.TrimSpace(envelope.Soul.Digest)
}

func taskRunHookAgentName(run Run, actor ActorContext) string {
	if value := taskRunMetadataString(run.Metadata, "agent_name"); value != "" {
		return value
	}
	if actor.Actor.Kind.Normalize() == ActorKindAgentSession {
		return strings.TrimSpace(actor.Actor.Ref)
	}
	return ""
}

func taskRunMetadataString(raw json.RawMessage, key string) string {
	var data map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &data) != nil {
		return ""
	}
	value, ok := data[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func taskRunMetadataStringList(raw json.RawMessage, key string) []string {
	var data map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &data) != nil {
		return nil
	}
	value, ok := data[key]
	if !ok {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			continue
		}
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
