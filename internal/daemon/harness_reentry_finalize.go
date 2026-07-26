package daemon

import (
	"context"

	"strings"

	"time"

	"github.com/compozy/agh/internal/store"
)

func (b *harnessReentryBridge) finalizeRunOutcome(
	runID string,
	sessionID string,
	agentName string,
	outcome string,
	reason string,
) {
	defer b.releaseProcessing(runID)

	opCtx, cancel := b.finalizationContext()
	defer cancel()

	run, err := b.store.GetTaskRun(opCtx, runID)
	if err != nil {
		b.logger.Error("daemon: load detached harness run for finalization", "run_id", runID, "error", err)
		return
	}
	metadata, ok, err := maybeDecodeDetachedHarnessRunMetadata(run.Metadata)
	if err != nil {
		b.logger.Error("daemon: decode detached harness run metadata for finalization", "run_id", runID, "error", err)
		return
	}
	if !ok {
		return
	}

	now := time.Now().UTC()
	metadata.Reentry = &detachedHarnessReentry{
		Outcome:     strings.TrimSpace(outcome),
		Reason:      strings.TrimSpace(reason),
		ProcessedAt: now,
	}
	raw, err := marshalDetachedHarnessMetadata(metadata)
	if err != nil {
		b.logger.Error("daemon: marshal detached harness finalization metadata", "run_id", runID, "error", err)
		return
	}

	run.Metadata = raw
	if err := b.store.UpdateTaskRun(opCtx, run); err != nil {
		b.logger.Error("daemon: persist detached harness finalization", "run_id", runID, "error", err)
		return
	}

	switch outcome {
	case harnessReentryOutcomeEmitted:
		b.writeEventSummaryWithContext(
			opCtx,
			sessionID,
			agentName,
			harnessSummarySyntheticReentryEmitted,
			syntheticOutcomeSummary(run.TaskID, run.ID, outcome, reason),
			now,
		)
	case harnessReentryOutcomeSilent, harnessReentryOutcomeDropped:
		b.writeEventSummaryWithContext(
			opCtx,
			sessionID,
			agentName,
			harnessSummarySyntheticReentryDropped,
			syntheticOutcomeSummary(run.TaskID, run.ID, outcome, reason),
			now,
		)
	}
}

func (b *harnessReentryBridge) writeEventSummary(
	sessionID string,
	agentName string,
	eventType string,
	summary string,
	timestamp time.Time,
) {
	b.writeEventSummaryWithContext(b.operationContext(), sessionID, agentName, eventType, summary, timestamp)
}

func (b *harnessReentryBridge) writeEventSummaryWithContext(
	ctx context.Context,
	sessionID string,
	agentName string,
	eventType string,
	summary string,
	timestamp time.Time,
) {
	targetSessionID := strings.TrimSpace(sessionID)
	if targetSessionID == "" {
		return
	}
	targetAgentName := strings.TrimSpace(agentName)
	if targetAgentName == "" {
		targetAgentName = harnessSummaryDefaultAgentName
	}
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	summaryPayload := store.EventSummary{
		SessionID: targetSessionID,
		Type:      strings.TrimSpace(eventType),
		AgentName: targetAgentName,
		Summary:   strings.TrimSpace(summary),
		Timestamp: timestamp,
	}
	if b.sessions != nil {
		info, err := b.sessions.Status(ctx, targetSessionID)
		if err == nil && info != nil {
			summaryPayload = harnessEventSummaryWithLineage(summaryPayload, info.Lineage)
		}
	}
	if err := b.store.WriteEventSummary(ctx, summaryPayload); err != nil {
		b.logger.Warn(
			"daemon: write detached harness event summary failed",
			"session_id", targetSessionID,
			"event_type", eventType,
			"error", err,
		)
	}
}

func (b *harnessReentryBridge) operationContext() context.Context {
	if b == nil || b.ctx == nil {
		return context.Background()
	}
	return b.ctx
}

func (b *harnessReentryBridge) finalizationContext() (context.Context, context.CancelFunc) {
	if b == nil || b.ctx == nil {
		return context.WithTimeout(context.Background(), harnessReentryShutdownFinalizationTimeout)
	}
	if b.ctx.Err() == nil {
		return b.ctx, func() {}
	}
	return context.WithTimeout(context.Background(), harnessReentryShutdownFinalizationTimeout)
}

func (b *harnessReentryBridge) requestRescan() {
	if b == nil {
		return
	}

	select {
	case <-b.ctx.Done():
		return
	case b.rescan <- struct{}{}:
	default:
	}
}

func (b *harnessReentryBridge) claimProcessing(runID string) bool {
	b.processingMu.Lock()
	defer b.processingMu.Unlock()
	if _, ok := b.processing[strings.TrimSpace(runID)]; ok {
		return false
	}
	b.processing[strings.TrimSpace(runID)] = struct{}{}
	return true
}

func (b *harnessReentryBridge) releaseProcessing(runID string) {
	b.processingMu.Lock()
	defer b.processingMu.Unlock()
	delete(b.processing, strings.TrimSpace(runID))
}
