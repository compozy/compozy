package daemon

import (
	"context"
	"encoding/json"
	"errors"

	"strings"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/heartbeat"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func (b *harnessReentryBridge) PromptHeartbeatWake(
	ctx context.Context,
	req heartbeat.SyntheticWakePromptRequest,
) (heartbeat.SyntheticWakePromptResult, error) {
	if b == nil || b.sessions == nil {
		return heartbeat.SyntheticWakePromptResult{}, errors.New("daemon: harness heartbeat prompter requires sessions")
	}
	events, err := b.sessions.PromptSynthetic(ctx, req.SessionID, session.SyntheticPromptOpts{
		Message: req.Message,
		TurnID:  req.TurnID,
		Metadata: acp.PromptSyntheticMeta{
			TaskID:               req.SyntheticCorrelation.TaskID,
			TaskRunID:            req.SyntheticCorrelation.TaskRunID,
			WorkflowID:           req.SyntheticCorrelation.WorkflowID,
			ClaimTokenHash:       req.SyntheticCorrelation.ClaimTokenHash,
			CoordinatorSessionID: req.SyntheticCorrelation.CoordinatorSessionID,
			Reason:               heartbeat.SyntheticReasonHeartbeatWake,
			Summary:              req.Summary,
			WakeEventID:          req.WakeEventID,
			PolicySnapshotID:     req.PolicySnapshotID,
			PolicyDigest:         req.PolicyDigest,
			ConfigDigest:         req.ConfigDigest,
		},
		SkipIfBusy: true,
	})
	if err != nil {
		if errors.Is(err, session.ErrPromptInProgress) {
			return heartbeat.SyntheticWakePromptResult{}, heartbeat.ErrSyntheticPromptBusy
		}
		return heartbeat.SyntheticWakePromptResult{}, err
	}
	b.drainHeartbeatWakeEvents(req.SessionID, req.WakeEventID, events)
	return heartbeat.SyntheticWakePromptResult{SyntheticPromptID: req.TurnID}, nil
}

func (b *harnessReentryBridge) drainHeartbeatWakeEvents(
	sessionID string,
	wakeEventID string,
	events <-chan acp.AgentEvent,
) {
	if b == nil || events == nil {
		return
	}
	b.startWorker(func() {
		for {
			select {
			case <-b.ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if event.Type == acp.EventTypeError {
					b.logger.Warn(
						"daemon: heartbeat harness wake agent error",
						"session_id", sessionID,
						"wake_event_id", wakeEventID,
					)
				}
			}
		}
	})
}

func (b *harnessReentryBridge) awaitSyntheticWake(
	item harnessSyntheticWake,
	events <-chan acp.AgentEvent,
) {
	sawError := false
Loop:
	for {
		select {
		case <-b.ctx.Done():
			sawError = true
			break Loop
		case event, ok := <-events:
			if !ok {
				break Loop
			}
			if event.Type == acp.EventTypeError {
				sawError = true
			}
		}
	}

	existing, err := b.syntheticEventExists(item.targetSessionID, item.runID)
	if err == nil && existing {
		b.finalizeRunOutcome(
			item.runID,
			item.targetSessionID,
			item.targetAgentName,
			harnessReentryOutcomeEmitted,
			item.reason,
		)
		return
	}
	if sawError || err != nil {
		b.finalizeRunOutcome(
			item.runID,
			item.targetSessionID,
			item.targetAgentName,
			harnessReentryOutcomeDropped,
			harnessReentryReasonDispatchFailed,
		)
		return
	}

	b.finalizeRunOutcome(
		item.runID,
		item.targetSessionID,
		item.targetAgentName,
		harnessReentryOutcomeDropped,
		harnessReentryReasonEventMissing,
	)
}

func (b *harnessReentryBridge) syntheticEventExists(sessionID string, runID string) (bool, error) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(runID) == "" {
		return false, nil
	}

	events, err := b.sessions.Events(
		b.operationContext(),
		sessionID,
		store.EventQuery{Type: acp.EventTypeSyntheticReentry},
	)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return false, nil
		}
		return false, err
	}
	for _, event := range events {
		var payload struct {
			Synthetic *acp.PromptSyntheticMeta `json:"synthetic,omitempty"`
		}
		if err := json.Unmarshal([]byte(event.Content), &payload); err != nil {
			continue
		}
		if payload.Synthetic == nil {
			continue
		}
		if strings.TrimSpace(payload.Synthetic.TaskRunID) == strings.TrimSpace(runID) {
			return true, nil
		}
	}
	return false, nil
}
