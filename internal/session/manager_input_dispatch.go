package session

import (
	"errors"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

func (m *Manager) startNextQueuedInputPrompt(sessionID string) {
	target, session, selected, ok := m.peekNextQueuedInputPrompt(sessionID)
	if !ok {
		return
	}
	if selected.OwnerKind == managedInputOwnerGoal {
		m.startManagedInputPrompt(session, selected)
		return
	}
	entry, ok, err := m.inputQueue.ClaimNext(m.fallbackLifecycleContext(), target)
	if err != nil {
		m.sessionLogger(session).Warn("session: claim queued input failed", "error", err)
		return
	}
	if !ok {
		return
	}
	if entry.ID != selected.ID {
		if releaseErr := m.inputQueue.Release(m.fallbackLifecycleContext(), target, entry.ID); releaseErr != nil {
			m.sessionLogger(session).Warn(
				"session: release reordered queued input failed",
				"entry_id", entry.ID,
				"error", releaseErr,
			)
		}
		m.startNextQueuedInputPrompt(target)
		return
	}
	m.dispatchHumanQueuedInput(target, session, entry)
}

func (m *Manager) peekNextQueuedInputPrompt(
	sessionID string,
) (string, *Session, store.SessionInputQueueEntry, bool) {
	if m == nil || m.inputQueue == nil {
		return "", nil, store.SessionInputQueueEntry{}, false
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return "", nil, store.SessionInputQueueEntry{}, false
	}
	session, err := m.lookupPromptSession(m.fallbackLifecycleContext(), target)
	if err != nil || session.IsPrompting() {
		return "", nil, store.SessionInputQueueEntry{}, false
	}
	entry, ok, err := m.inputQueue.PeekNext(m.fallbackLifecycleContext(), target)
	if err != nil {
		m.sessionLogger(session).Warn("session: peek queued input failed", "error", err)
		return "", nil, store.SessionInputQueueEntry{}, false
	}
	return target, session, entry, ok
}

func (m *Manager) dispatchHumanQueuedInput(
	target string,
	session *Session,
	entry store.SessionInputQueueEntry,
) {
	if entry.Mode == store.SessionInputQueueModeSteer {
		m.emitQueuedSteerFallback(session, entry)
	}
	req, ok := m.newQueuedInputPromptRequest(session, target, entry)
	if !ok {
		return
	}
	events, err := m.submitPromptRequest(m.fallbackLifecycleContext(), req)
	if err != nil {
		m.handleQueuedInputDispatchError(session, target, entry, req, err)
		return
	}
	m.acceptQueuedInputDispatch(session, target, entry, req)
	go m.drainQueuedInputEvents(target, events)
}

func (m *Manager) emitQueuedSteerFallback(session *Session, entry store.SessionInputQueueEntry) {
	evidence := queueEntryEvidence(entry, 0)
	evidence["fallback_to_queue"] = true
	m.emitTranscriptMarker(
		m.fallbackLifecycleContext(),
		session,
		session.CurrentTurnID(),
		transcript.MarkerPromptSteered,
		"Staged steering input fell back to queued dispatch.",
		evidence,
	)
}

func (m *Manager) newQueuedInputPromptRequest(
	session *Session,
	target string,
	entry store.SessionInputQueueEntry,
) (promptRequest, bool) {
	meta, err := normalizePromptMeta(
		TurnSourceUser,
		acp.PromptMeta{TurnSource: string(TurnSourceUser)},
		promptSubmissionPathUserFacing,
	)
	if err != nil {
		m.sessionLogger(session).Warn(
			"session: normalize queued input metadata failed",
			"entry_id", entry.ID,
			"error", err,
		)
		return promptRequest{}, false
	}
	return promptRequest{
		turnID:          m.newPromptTurnID(),
		target:          target,
		message:         entry.Text,
		authoredMessage: entry.Text,
		turnSource:      TurnSourceUser,
		meta:            meta,
	}, true
}

func (m *Manager) handleQueuedInputDispatchError(
	session *Session,
	target string,
	entry store.SessionInputQueueEntry,
	req promptRequest,
	cause error,
) {
	if errors.Is(cause, ErrPromptInProgress) {
		if err := m.inputQueue.Release(m.fallbackLifecycleContext(), target, entry.ID); err != nil {
			m.sessionLogger(session).Warn("session: release queued input failed", "entry_id", entry.ID, "error", err)
		}
		return
	}
	if err := m.inputQueue.MarkFailed(m.fallbackLifecycleContext(), target, entry.ID, cause.Error()); err != nil {
		m.sessionLogger(session).Warn("session: mark queued input failed", "entry_id", entry.ID, "error", err)
	}
	m.emitTranscriptMarker(
		m.fallbackLifecycleContext(),
		session,
		req.turnID,
		transcript.MarkerPromptDropped,
		"Queued input failed before dispatch.",
		queueEntryEvidence(entry, 0),
	)
	m.startNextQueuedInputPrompt(target)
}

func (m *Manager) acceptQueuedInputDispatch(
	session *Session,
	target string,
	entry store.SessionInputQueueEntry,
	req promptRequest,
) {
	if err := m.inputQueue.MarkSent(m.fallbackLifecycleContext(), target, entry.ID); err != nil {
		m.sessionLogger(session).Warn("session: mark queued input sent failed", "entry_id", entry.ID, "error", err)
	}
	m.emitTranscriptMarker(
		m.fallbackLifecycleContext(),
		session,
		req.turnID,
		transcript.MarkerPromptAccepted,
		"Queued input accepted for dispatch.",
		queueEntryEvidence(entry, 0),
	)
}

func (m *Manager) drainQueuedInputEvents(sessionID string, events <-chan acp.AgentEvent) {
	finishDrain := m.trackPromptDrain()
	defer finishDrain()
	for range events {
		continue
	}
	m.startNextQueuedInputPrompt(sessionID)
}
