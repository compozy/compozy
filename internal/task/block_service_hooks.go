package task

import (
	"context"

	"log/slog"

	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
)

func (m *Service) dispatchTaskBlocked(
	ctx context.Context,
	block TaskBlock,
	taskRecord Task,
	actor ActorContext,
	release *BlockTaskAndReleaseRunResult,
) {
	payload := hookspkg.TaskBlockedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskBlocked,
			Timestamp: m.now().UTC(),
		},
		TaskContext: m.taskHookContext(taskRecord, actor, release),
		BlockID:     strings.TrimSpace(block.ID),
		Kind:        string(block.Kind.Normalize()),
		Reason:      redactTaskSecretText(strings.TrimSpace(block.Reason)),
		Details:     redactTaskSecretJSON(cloneRawJSON(block.Details)),
	}
	_, err := m.taskHooks.DispatchTaskBlocked(taskRunObservationHookContext(ctx), payload)
	m.reportTaskHookFailure(hookspkg.HookTaskBlocked, err, taskRecord)
}

func (m *Service) dispatchTaskUnblocked(
	ctx context.Context,
	block TaskBlock,
	taskRecord Task,
	actor ActorContext,
) {
	payload := hookspkg.TaskUnblockedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskUnblocked,
			Timestamp: m.now().UTC(),
		},
		TaskContext: m.taskHookContext(taskRecord, actor, nil),
		BlockID:     strings.TrimSpace(block.ID),
		Kind:        string(block.Kind.Normalize()),
		Reason:      redactTaskSecretText(strings.TrimSpace(block.Reason)),
		Details:     redactTaskSecretJSON(cloneRawJSON(block.Details)),
		ClearedAt:   block.ClearedAt,
		ClearNote:   redactTaskSecretText(strings.TrimSpace(block.ClearNote)),
	}
	_, err := m.taskHooks.DispatchTaskUnblocked(taskRunObservationHookContext(ctx), payload)
	m.reportTaskHookFailure(hookspkg.HookTaskUnblocked, err, taskRecord)
}

func (m *Service) dispatchTaskNeedsAttention(
	ctx context.Context,
	taskRecord Task,
	actor ActorContext,
	reason string,
	at time.Time,
	release *BlockTaskAndReleaseRunResult,
) {
	payload := hookspkg.TaskNeedsAttentionPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskNeedsAttention,
			Timestamp: m.now().UTC(),
		},
		TaskContext: m.taskHookContext(taskRecord, actor, release),
		Reason:      redactTaskSecretText(strings.TrimSpace(reason)),
		At:          at,
	}
	_, err := m.taskHooks.DispatchTaskNeedsAttention(taskRunObservationHookContext(ctx), payload)
	m.reportTaskHookFailure(hookspkg.HookTaskNeedsAttention, err, taskRecord)
}

func (m *Service) dispatchTaskRecovered(
	ctx context.Context,
	taskRecord Task,
	actor ActorContext,
	note string,
	at time.Time,
) {
	payload := hookspkg.TaskRecoveredPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskRecovered,
			Timestamp: m.now().UTC(),
		},
		TaskContext: m.taskHookContext(taskRecord, actor, nil),
		Note:        redactTaskSecretText(strings.TrimSpace(note)),
		At:          at,
	}
	_, err := m.taskHooks.DispatchTaskRecovered(taskRunObservationHookContext(ctx), payload)
	m.reportTaskHookFailure(hookspkg.HookTaskRecovered, err, taskRecord)
}

func (m *Service) taskHookContext(
	taskRecord Task,
	actor ActorContext,
	release *BlockTaskAndReleaseRunResult,
) hookspkg.TaskContext {
	contextPayload := hookspkg.TaskContext{
		TaskID:       strings.TrimSpace(taskRecord.ID),
		ParentTaskID: strings.TrimSpace(taskRecord.ParentTaskID),
		WorkspaceID:  strings.TrimSpace(taskRecord.WorkspaceID),
		WorkflowID:   taskRunMetadataString(taskRecord.Metadata, "workflow_id"),
		AgentName:    taskHookAgentName(taskRecord, actor),
		ActorKind:    string(actor.Actor.Kind.Normalize()),
		ActorID:      strings.TrimSpace(actor.Actor.Ref),
		OriginKind:   string(actor.Origin.Kind.Normalize()),
		OriginRef:    strings.TrimSpace(actor.Origin.Ref),
		TaskStatus:   string(taskRecord.Status.Normalize()),
	}
	if release != nil {
		contextPayload.RunID = strings.TrimSpace(release.Run.ID)
		contextPayload.ReleaseReason = strings.TrimSpace(release.ReleaseReason)
		contextPayload.ClaimTokenHash = strings.TrimSpace(release.ClaimTokenHash)
		contextPayload.ResolvedNetworkParticipation = participation.CloneSpec(
			release.Run.NetworkSpecSnapshot(),
		)
	}
	return contextPayload
}

func taskHookAgentName(taskRecord Task, actor ActorContext) string {
	if value := taskRunMetadataString(taskRecord.Metadata, "agent_name"); value != "" {
		return value
	}
	if actor.Actor.Kind.Normalize() == ActorKindAgentSession {
		return strings.TrimSpace(actor.Actor.Ref)
	}
	return ""
}

func (m *Service) reportTaskHookFailure(event hookspkg.HookEvent, err error, taskRecord Task) {
	if err == nil {
		return
	}
	slog.Error(
		"task: task lifecycle hook failed after committed mutation",
		"error", err,
		"hook_event", event,
		"task_id", taskRecord.ID,
		"task_status", taskRecord.Status,
	)
}

type taskBlockCreatedPayload struct {
	Status         Status    `json:"status"`
	BlockID        string    `json:"block_id"`
	BlockKind      BlockKind `json:"block_kind"`
	Reason         string    `json:"reason"`
	ExpiresAt      time.Time `json:"expires_at,omitzero"`
	ClaimTokenHash string    `json:"claim_token_hash,omitempty"`
}

type taskBlockClearedPayload struct {
	Status    Status    `json:"status"`
	BlockID   string    `json:"block_id"`
	BlockKind BlockKind `json:"block_kind"`
	Reason    string    `json:"reason"`
	ClearNote string    `json:"clear_note,omitempty"`
	ClearedAt time.Time `json:"cleared_at"`
}

type taskBlockExpiredPayload struct {
	Status    Status    `json:"status"`
	BlockID   string    `json:"block_id"`
	BlockKind BlockKind `json:"block_kind"`
	Reason    string    `json:"reason"`
	ExpiresAt time.Time `json:"expires_at"`
	ClearedAt time.Time `json:"cleared_at"`
}
