package daemon

import (
	"errors"

	"strings"

	"time"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"

	taskpkg "github.com/compozy/agh/internal/task"
)

func (b *harnessReentryBridge) applyWakeDecision(
	task taskpkg.Task,
	run taskpkg.Run,
	target harnessWakeTargetSnapshot,
	agentName string,
	completionSequence int64,
	completedAt time.Time,
	decision harnessReentryDecision,
) error {
	switch decision.outcome {
	case harnessReentryOutcomeSilent:
		b.finalizeRunOutcome(
			run.ID,
			target.SessionID,
			agentName,
			harnessReentryOutcomeSilent,
			decision.reason,
		)
		return nil
	case harnessReentryOutcomeDropped:
		b.finalizeRunOutcome(
			run.ID,
			target.SessionID,
			agentName,
			harnessReentryOutcomeDropped,
			decision.reason,
		)
		return nil
	}

	existing, err := b.syntheticEventExists(target.SessionID, run.ID)
	if err != nil {
		b.releaseProcessing(run.ID)
		return err
	}
	if existing {
		b.finalizeRunOutcome(
			run.ID,
			target.SessionID,
			agentName,
			harnessReentryOutcomeEmitted,
			harnessReentryReasonAlreadyRecorded,
		)
		return nil
	}
	if target.Missing {
		b.finalizeRunOutcome(
			run.ID,
			target.SessionID,
			agentName,
			harnessReentryOutcomeDropped,
			harnessReentryReasonTargetMissing,
		)
		return nil
	}
	if target.State != session.StateActive {
		b.finalizeRunOutcome(
			run.ID,
			target.SessionID,
			agentName,
			harnessReentryOutcomeDropped,
			inactiveTargetReason(target.State),
		)
		return nil
	}

	b.enqueueWake(harnessSyntheticWake{
		taskID:            task.ID,
		runID:             run.ID,
		targetSessionID:   target.SessionID,
		targetAgentName:   agentName,
		targetWorkspaceID: target.WorkspaceID,
		reason:            decision.reason,
		summary:           decision.syntheticMeta.Summary,
		completedAt:       completedAt,
		completionSeq:     completionSequence,
		syntheticMessage:  decision.syntheticMessage,
		syntheticMeta:     decision.syntheticMeta,
	})
	return nil
}

func (b *harnessReentryBridge) resolveWakeTargetSnapshot(
	metadata detachedHarnessRunMetadata,
) harnessWakeTargetSnapshot {
	target := harnessWakeTargetSnapshot{
		SessionID:            strings.TrimSpace(metadata.WakeTarget.SessionID),
		Type:                 session.Type(strings.TrimSpace(metadata.WakeTarget.SessionType)),
		WorkspaceID:          strings.TrimSpace(metadata.WakeTarget.WorkspaceID),
		NetworkParticipation: participation.LocalSpec(),
		Missing:              true,
	}

	info, err := b.sessions.Status(b.operationContext(), target.SessionID)
	switch {
	case err == nil && info != nil:
		target.AgentName = strings.TrimSpace(info.AgentName)
		target.Type = info.Type
		target.State = info.State
		if workspaceID := strings.TrimSpace(info.WorkspaceID); workspaceID != "" {
			target.WorkspaceID = workspaceID
		}
		target.NetworkParticipation = info.NetworkParticipation
		target.Missing = false
	case errors.Is(err, session.ErrSessionNotFound):
		target.Missing = true
	default:
		target.Missing = true
	}

	return target
}

func (b *harnessReentryBridge) resolveSummaryAgentName(
	target harnessWakeTargetSnapshot,
	metadata detachedHarnessRunMetadata,
) string {
	if agentName := strings.TrimSpace(target.AgentName); agentName != "" {
		return agentName
	}
	if ownerID := strings.TrimSpace(metadata.OwnerSessionID); ownerID != "" {
		info, err := b.sessions.Status(b.operationContext(), ownerID)
		if err == nil && info != nil && strings.TrimSpace(info.AgentName) != "" {
			return strings.TrimSpace(info.AgentName)
		}
	}
	return harnessSummaryDefaultAgentName
}

func (b *harnessReentryBridge) evaluateDecision(
	task taskpkg.Task,
	run taskpkg.Run,
	metadata detachedHarnessRunMetadata,
	target harnessWakeTargetSnapshot,
) harnessReentryDecision {
	reason, trigger := syntheticReasonForTerminalRun(run.Status.Normalize())
	summary := detachedHarnessSummary(metadata.Summary)
	input := HarnessResolutionInput{
		Surface: ResolutionSurfaceTurn,
		Session: HarnessSessionInput{
			Type:                 target.Type,
			NetworkParticipation: target.NetworkParticipation,
			WorkspaceID:          target.WorkspaceID,
		},
		Turn: HarnessTurnRequest{
			Source: session.TurnSourceSynthetic,
			Synthetic: &SyntheticTurnMetadata{
				Reason:      reason,
				Trigger:     trigger,
				SourceTask:  task.ID,
				SourceRunID: run.ID,
			},
			Detached: &DetachedRunMetadata{
				TaskID:    task.ID,
				TaskRunID: run.ID,
			},
		},
	}

	resolved, err := b.resolver.Resolve(input)
	if err != nil || resolved.Policy.ReentryMode != ReentryModeSynthetic {
		if err == nil && b.recorder != nil {
			b.recorder.RecordSyntheticContextResolved(
				b.ctx,
				target.SessionID,
				b.resolveSummaryAgentName(target, metadata),
				resolved,
				run.EndedAt,
			)
		}
		return harnessReentryDecision{
			outcome: harnessReentryOutcomeSilent,
			reason:  harnessReentryReasonPolicySilent,
			target:  target,
		}
	}
	if b.recorder != nil {
		b.recorder.RecordSyntheticContextResolved(
			b.ctx,
			target.SessionID,
			b.resolveSummaryAgentName(target, metadata),
			resolved,
			run.EndedAt,
		)
	}

	return harnessReentryDecision{
		outcome:          harnessReentryOutcomeEmitted,
		reason:           reason,
		target:           target,
		syntheticMessage: buildDetachedHarnessSyntheticMessage(task, run, summary),
		syntheticMeta: acp.PromptSyntheticMeta{
			TaskID:         task.ID,
			TaskRunID:      run.ID,
			ClaimTokenHash: strings.TrimSpace(run.ClaimTokenHash),
			Reason:         reason,
			Summary:        summary,
		},
	}
}

func (b *harnessReentryBridge) enqueueWake(item harnessSyntheticWake) {
	b.queueMu.Lock()
	queue := b.queues[item.targetSessionID]
	if queue == nil {
		queue = &harnessWakeQueue{}
		b.queues[item.targetSessionID] = queue
	}
	queue.items = insertSyntheticWake(queue.items, item)
	shouldStart := !queue.dispatching
	if shouldStart {
		queue.dispatching = true
	}
	b.queueMu.Unlock()

	if shouldStart {
		targetSessionID := item.targetSessionID
		b.startWorker(func() {
			b.drainWakeQueue(targetSessionID)
		})
	}
}
