package session

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/acp"
)

// SyntheticPromptOpts carries daemon-owned synthetic prompt input plus
// wake-up metadata required for persistence and later reentry handling.
type SyntheticPromptOpts struct {
	Message                 string
	Metadata                acp.PromptSyntheticMeta
	TurnID                  string
	SkipIfBusy              bool
	InterruptIfAgentWaiting bool
}

// PromptSynthetic admits daemon-owned work to the shared durable queue.
func (m *Manager) PromptSynthetic(
	ctx context.Context,
	id string,
	opts SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	req, err := m.parseSyntheticPromptRequest(ctx, id, opts)
	if err != nil {
		return nil, err
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}
	session, err := m.lookupPromptSession(ctx, req.target)
	if err != nil {
		return nil, err
	}
	if m.inputQueue == nil {
		return nil, errors.New("session: durable input queue is required for synthetic prompts")
	}
	pending, err := m.inputQueue.List(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	if opts.SkipIfBusy && (session.IsPrompting() || len(pending) > 0) {
		return nil, ErrPromptInProgress
	}
	interrupt := opts.InterruptIfAgentWaiting && !opts.SkipIfBusy && len(pending) == 0 &&
		session.isCurrentPromptAgentWaiting()
	delivery, err := m.openDurablePromptDelivery(m.fallbackLifecycleContext(), session, req.turnID)
	if err != nil {
		return nil, err
	}
	entry, err := m.enqueueDurableSyntheticPrompt(ctx, session, req)
	if err != nil {
		delivery.cancel()
		return nil, err
	}
	m.startTrackedPromptTask(func() {
		defer close(delivery.persistenceDone)
		m.waitSyntheticPromptPersistence(session, &entry)
	})
	if interrupt {
		cancelCtx, cancel := context.WithTimeout(m.fallbackLifecycleContext(), m.supervision.TimeoutCancelGrace)
		defer cancel()
		if _, err := m.CancelPrompt(cancelCtx, session.ID); err != nil {
			m.sessionLogger(session).WarnContext(ctx,
				"session: synthetic wake cancellation failed; input remains queued",
				"entry_id", entry.ID, "error", err,
			)
		}
	}
	m.startNextQueuedInputPrompt(session.ID)
	return delivery.events, nil
}

func (m *Manager) parseSyntheticPromptRequest(
	ctx context.Context,
	id string,
	opts SyntheticPromptOpts,
) (promptRequest, error) {
	if ctx == nil {
		return promptRequest{}, errors.New("session: prompt context is required")
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return promptRequest{}, errors.New("session: session id is required")
	}

	message := strings.TrimSpace(opts.Message)
	if message == "" {
		return promptRequest{}, errors.New("session: prompt message is required")
	}

	meta, err := normalizePromptMeta(
		TurnSourceSynthetic,
		acp.PromptMeta{
			TurnSource: acp.PromptTurnSourceSynthetic,
			Synthetic:  &opts.Metadata,
		},
		promptSubmissionPathSynthetic,
	)
	if err != nil {
		return promptRequest{}, err
	}

	turnID := strings.TrimSpace(opts.TurnID)
	if turnID == "" {
		turnID, err = m.newPromptTurnID()
		if err != nil {
			return promptRequest{}, err
		}
	}
	runID := promptMetaRunID(meta)
	if runID == "" {
		runID, err = m.newPromptRunID()
		if err != nil {
			return promptRequest{}, err
		}
	}

	return promptRequest{
		turnID:          turnID,
		runID:           runID,
		target:          target,
		message:         message,
		authoredMessage: message,
		turnSource:      TurnSourceSynthetic,
		meta:            meta,
	}, nil
}

func promptMetaRunID(meta acp.PromptMeta) string {
	normalized := meta.Normalize()
	if normalized.Synthetic == nil {
		return ""
	}
	if runID := strings.TrimSpace(normalized.Synthetic.TaskRunID); runID != "" {
		return runID
	}
	if normalized.Synthetic.Goal != nil {
		return strings.TrimSpace(normalized.Synthetic.Goal.RunID)
	}
	return ""
}
