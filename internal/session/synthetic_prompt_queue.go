package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session/inputqueue"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) enqueueDurableSyntheticPrompt(
	ctx context.Context,
	session *Session,
	req promptRequest,
	opts SyntheticPromptOpts,
) (store.SessionInputQueueEntry, error) {
	generation, err := m.currentInputGeneration(ctx, session.ID)
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	metadata, err := json.Marshal(req.meta.Synthetic)
	if err != nil {
		return store.SessionInputQueueEntry{}, fmt.Errorf("session: encode synthetic input: %w", err)
	}
	return m.inputQueue.EnqueueSynthetic(ctx, inputqueue.InputRequest{
		MessageID: opts.MessageID, Priority: opts.Priority,
		SessionID: session.ID, Text: req.message, TargetTurnID: session.CurrentTurnID(), Generation: generation,
		Coalesce: opts.Coalesce,
	}, req.turnID, &store.SessionInputSyntheticPrompt{
		RunID: req.runID, Metadata: metadata, Delivery: string(req.delivery), CoalesceKey: opts.CoalesceKey,
	}, req.meta.Synthetic.TaskRunID)
}

// This observer owns only delivery completion. Durable rows and the common pump
// own admission and execution, including when this caller or daemon disappears.
// A promoted wake completes its admission stream after confirmed injection into the live turn.
func (m *Manager) waitSyntheticPromptPersistence(session *Session, admitted *store.SessionInputQueueEntry) bool {
	ctx := m.fallbackLifecycleContext()
	ticker := time.NewTicker(promptDeliveryCatchUpInterval)
	defer ticker.Stop()
	for {
		entry, err := m.inputQueue.Get(ctx, session.ID, admitted.ID)
		if err != nil {
			m.sessionLogger(session).
				WarnContext(ctx, "session: observe synthetic input failed", "entry_id", admitted.ID, "error", err)
			return false
		}
		switch entry.Status {
		case store.SessionInputQueueStatusFailed, store.SessionInputQueueStatusCanceled:
			return false
		case store.SessionInputQueueStatusSent:
			if entry.SteerDelivery == store.SteerDeliveryPendingInjection {
				break
			}
			if entry.SteerDelivery == store.SteerDeliveryInjected && entry.TurnID != admitted.TurnID {
				return true
			}
			if session.CurrentTurnID() != admitted.TurnID {
				return false
			}
		}
		if session.Info().State != StateActive {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

// syntheticPromptMeta decodes a row's wake metadata. A row carrying several wakes names their count and a
// neutral reason, so the transcript does not attribute every outcome to the first monitor.
func syntheticPromptMeta(prompt *store.SessionInputSyntheticPrompt) (*acp.PromptSyntheticMeta, error) {
	var meta *acp.PromptSyntheticMeta
	if err := json.Unmarshal(prompt.Metadata, &meta); err != nil {
		return nil, err
	}
	if wakes := prompt.CoalescedWakes(); wakes > 1 && meta != nil {
		meta.Summary, meta.Reason = fmt.Sprintf("%d monitors", wakes), "monitor_coalesced"
	}
	return meta, nil
}
