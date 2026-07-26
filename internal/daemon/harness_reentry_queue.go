package daemon

import (
	"sort"
	"strings"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/heartbeat"

	"github.com/compozy/agh/internal/session"
)

func insertSyntheticWake(items []harnessSyntheticWake, item harnessSyntheticWake) []harnessSyntheticWake {
	index := sort.Search(len(items), func(i int) bool {
		return compareSyntheticWake(item, items[i]) < 0
	})
	items = append(items, harnessSyntheticWake{})
	copy(items[index+1:], items[index:])
	items[index] = item
	return items
}

func compareSyntheticWake(left harnessSyntheticWake, right harnessSyntheticWake) int {
	if !left.completedAt.Equal(right.completedAt) {
		if left.completedAt.Before(right.completedAt) {
			return -1
		}
		return 1
	}
	if left.completionSeq != right.completionSeq {
		if left.completionSeq < right.completionSeq {
			return -1
		}
		return 1
	}
	return strings.Compare(left.runID, right.runID)
}

func (b *harnessReentryBridge) drainWakeQueue(sessionID string) {
	for {
		select {
		case <-b.ctx.Done():
			b.discardWakeQueue(sessionID)
			return
		default:
		}
		item, ok := b.nextWake(sessionID)
		if !ok {
			return
		}
		b.dispatchWake(item)
	}
}

func (b *harnessReentryBridge) discardWakeQueue(sessionID string) {
	b.queueMu.Lock()
	defer b.queueMu.Unlock()

	queue := b.queues[sessionID]
	if queue == nil {
		return
	}
	queue.dispatching = false
	queue.items = nil
	delete(b.queues, sessionID)
}

func (b *harnessReentryBridge) nextWake(sessionID string) (harnessSyntheticWake, bool) {
	b.queueMu.Lock()
	defer b.queueMu.Unlock()

	queue := b.queues[sessionID]
	if queue == nil || len(queue.items) == 0 {
		if queue != nil {
			queue.dispatching = false
			delete(b.queues, sessionID)
		}
		return harnessSyntheticWake{}, false
	}

	item := queue.items[0]
	if len(queue.items) == 1 {
		queue.items = nil
	} else {
		queue.items = append([]harnessSyntheticWake(nil), queue.items[1:]...)
	}
	return item, true
}

func (b *harnessReentryBridge) dispatchWake(item harnessSyntheticWake) {
	existing, err := b.syntheticEventExists(item.targetSessionID, item.runID)
	if err == nil && existing {
		b.finalizeRunOutcome(
			item.runID,
			item.targetSessionID,
			item.targetAgentName,
			harnessReentryOutcomeEmitted,
			harnessReentryReasonAlreadyRecorded,
		)
		return
	}
	if b.dispatchHeartbeatWake(item) {
		return
	}

	eventsCh, promptErr := b.sessions.PromptSynthetic(b.ctx, item.targetSessionID, session.SyntheticPromptOpts{
		Message:                 item.syntheticMessage,
		InterruptIfAgentWaiting: true,
		Metadata: acp.PromptSyntheticMeta{
			TaskID:         item.syntheticMeta.TaskID,
			TaskRunID:      item.syntheticMeta.TaskRunID,
			ClaimTokenHash: item.syntheticMeta.ClaimTokenHash,
			Reason:         item.syntheticMeta.Reason,
			Summary:        item.syntheticMeta.Summary,
		},
	})
	if promptErr != nil {
		b.finalizeRunOutcome(
			item.runID,
			item.targetSessionID,
			item.targetAgentName,
			harnessReentryOutcomeDropped,
			classifySyntheticPromptError(promptErr),
		)
		return
	}
	if eventsCh == nil {
		b.finalizeRunOutcome(
			item.runID,
			item.targetSessionID,
			item.targetAgentName,
			harnessReentryOutcomeDropped,
			harnessReentryReasonEventMissing,
		)
		return
	}

	b.startWorker(func() {
		b.awaitSyntheticWake(item, eventsCh)
	})
}

func (b *harnessReentryBridge) dispatchHeartbeatWake(item harnessSyntheticWake) bool {
	if b == nil || b.heartbeatWake == nil {
		return false
	}
	if strings.TrimSpace(item.targetWorkspaceID) == "" || strings.TrimSpace(item.targetAgentName) == "" {
		return false
	}
	decision, err := b.heartbeatWake.Wake(b.ctx, heartbeat.WakeRequest{
		WorkspaceID: strings.TrimSpace(item.targetWorkspaceID),
		AgentName:   strings.TrimSpace(item.targetAgentName),
		SessionID:   strings.TrimSpace(item.targetSessionID),
		Source:      heartbeat.WakeSourceHarnessReentry,
		SyntheticCorrelation: heartbeat.WakeSyntheticCorrelation{
			TaskID:               item.syntheticMeta.TaskID,
			TaskRunID:            item.syntheticMeta.TaskRunID,
			WorkflowID:           item.syntheticMeta.WorkflowID,
			ClaimTokenHash:       item.syntheticMeta.ClaimTokenHash,
			CoordinatorSessionID: item.syntheticMeta.CoordinatorSessionID,
		},
	})
	if err != nil {
		b.finalizeRunOutcome(
			item.runID,
			item.targetSessionID,
			item.targetAgentName,
			harnessReentryOutcomeDropped,
			harnessReentryReasonDispatchFailed,
		)
		return true
	}
	if decision.Result == heartbeat.WakeResultSkipped &&
		decision.Reason == heartbeat.WakeReasonHeartbeatNoPolicy {
		return false
	}
	switch decision.Result {
	case heartbeat.WakeResultSent:
		b.finalizeRunOutcome(
			item.runID,
			item.targetSessionID,
			item.targetAgentName,
			harnessReentryOutcomeEmitted,
			string(decision.Reason),
		)
	case heartbeat.WakeResultSkipped:
		if decision.Reason == heartbeat.WakeReasonSessionPromptActive ||
			decision.Reason == heartbeat.WakeReasonSessionPromptRace {
			return false
		}
		b.finalizeRunOutcome(
			item.runID,
			item.targetSessionID,
			item.targetAgentName,
			harnessReentryOutcomeDropped,
			string(decision.Reason),
		)
	case heartbeat.WakeResultFailed:
		b.finalizeRunOutcome(
			item.runID,
			item.targetSessionID,
			item.targetAgentName,
			harnessReentryOutcomeDropped,
			harnessReentryReasonDispatchFailed,
		)
	default:
		b.finalizeRunOutcome(
			item.runID,
			item.targetSessionID,
			item.targetAgentName,
			harnessReentryOutcomeDropped,
			string(decision.Reason),
		)
	}
	return true
}
