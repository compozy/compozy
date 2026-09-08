package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type queuedInput struct {
	syntheticPrompt   *store.SessionInputSyntheticPrompt
	id                string
	promptAdmissionID string
	messageID         string
	idempotencyKey    string
	turnID            string
	eventID           string
	mode              string
	status            string
	text              string
	runtime           store.SessionInputRuntime
	skillInvocations  []commandpkg.Invocation
	attachments       []AttachmentMeta
	sessionGeneration int64
}

func (m *Manager) startNextQueuedInputPrompt(sessionID string) {
	target, session, selected, ok := m.peekNextQueuedInputPrompt(sessionID)
	if !ok {
		return
	}
	if selected.OwnerKind == managedInputOwnerGoal {
		m.startManagedInputPrompt(session, managedInputFromQueueEntry(&selected))
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
	m.dispatchQueuedInput(target, session, queuedInput{
		id:                entry.ID,
		syntheticPrompt:   entry.SyntheticPrompt,
		promptAdmissionID: entry.PromptAdmissionID,
		messageID:         entry.MessageID,
		idempotencyKey:    entry.IdempotencyKey,
		turnID:            entry.TurnID,
		eventID:           entry.EventID,
		mode:              entry.Mode,
		status:            entry.Status,
		text:              entry.Text,
		runtime:           entry.Runtime,
		skillInvocations:  append([]commandpkg.Invocation(nil), entry.SkillInvocations...),
		attachments:       attachmentMetaFromStore(entry.Attachments),
		sessionGeneration: entry.SessionGeneration,
	})
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
	if err != nil || session.IsPrompting() || session.Info().State != StateActive {
		return "", nil, store.SessionInputQueueEntry{}, false
	}
	entry, ok, err := m.inputQueue.PeekNext(m.fallbackLifecycleContext(), target)
	if err != nil {
		m.sessionLogger(session).Warn("session: peek queued input failed", "error", err)
		return "", nil, store.SessionInputQueueEntry{}, false
	}
	return target, session, entry, ok
}

func (m *Manager) dispatchQueuedInput(
	target string,
	session *Session,
	entry queuedInput,
) {
	req, err := m.newQueuedInputPromptRequest(target, entry)
	if err != nil {
		m.handleQueuedInputDispatchError(session, target, entry, req, err)
		return
	}
	events, err := m.submitPromptRequest(m.fallbackLifecycleContext(), req)
	if err != nil {
		m.handleQueuedInputDispatchError(session, target, entry, req, err)
		return
	}
	m.acceptQueuedInputDispatch(session, target, entry, req)
	m.startTrackedPromptTask(func() {
		m.drainQueuedInputEvents(events)
	})
}

func (m *Manager) newQueuedInputPromptRequest(
	target string,
	entry queuedInput,
) (promptRequest, error) {
	req := promptRequest{
		target:           target,
		message:          entry.text,
		authoredMessage:  entry.text,
		messageID:        entry.messageID,
		idempotencyKey:   entry.idempotencyKey,
		eventID:          entry.eventID,
		turnSource:       TurnSourceUser,
		meta:             acp.PromptMeta{TurnSource: string(TurnSourceUser)},
		runtime:          runtimeSelectionFromStore(entry.runtime),
		skillInvocations: append([]commandpkg.Invocation(nil), entry.skillInvocations...),
		attachments:      cloneAttachmentMeta(entry.attachments),
	}
	turnID := strings.TrimSpace(entry.turnID)
	if turnID == "" {
		var err error
		turnID, err = m.newPromptTurnID()
		if err != nil {
			return req, err
		}
	}
	req.turnID = turnID
	if entry.syntheticPrompt != nil {
		req.turnSource = TurnSourceSynthetic
		req.meta = acp.PromptMeta{TurnSource: acp.PromptTurnSourceSynthetic}
		if err := json.Unmarshal(entry.syntheticPrompt.Metadata, &req.meta.Synthetic); err != nil {
			return req, fmt.Errorf("session: decode synthetic input metadata: %w", err)
		}
		var err error
		req.meta, err = normalizePromptMeta(TurnSourceSynthetic, req.meta, promptSubmissionPathSynthetic)
		if err != nil {
			return req, err
		}
		req.runID = entry.syntheticPrompt.RunID
		return req, nil
	}
	var err error
	req.runID, err = m.newPromptRunID()
	if err != nil {
		return req, err
	}
	return req, nil
}

func (m *Manager) handleQueuedInputDispatchError(
	session *Session,
	target string,
	entry queuedInput,
	req promptRequest,
	cause error,
) {
	if errors.Is(cause, ErrPromptInProgress) || errors.Is(cause, ErrSessionNotActive) {
		if err := m.inputQueue.Release(m.fallbackLifecycleContext(), target, entry.id); err != nil {
			m.sessionLogger(session).Warn("session: release queued input failed", "entry_id", entry.id, "error", err)
		}
		return
	}
	if err := m.inputQueue.MarkFailed(m.fallbackLifecycleContext(), target, entry.id, cause.Error()); err != nil {
		m.sessionLogger(session).Warn("session: mark queued input failed", "entry_id", entry.id, "error", err)
	}
	m.emitTranscriptMarker(
		m.fallbackLifecycleContext(),
		session,
		req.turnID,
		transcript.MarkerPromptDropped,
		"Queued input failed before dispatch.",
		queueEntryEvidence(entry.id, entry.sessionGeneration, entry.status, entry.mode, 0),
	)
	m.startNextQueuedInputPrompt(target)
}

func (m *Manager) acceptQueuedInputDispatch(
	session *Session,
	target string,
	entry queuedInput,
	req promptRequest,
) {
	if err := m.inputQueue.MarkSent(m.fallbackLifecycleContext(), target, entry.id); err != nil {
		m.sessionLogger(session).Warn("session: mark queued input sent failed", "entry_id", entry.id, "error", err)
		if failErr := m.inputQueue.MarkFailed(
			m.fallbackLifecycleContext(),
			target,
			entry.id,
			err.Error(),
		); failErr != nil {
			m.sessionLogger(session).Error("session: mark queued input failed", "entry_id", entry.id, "error", failErr)
		}
		m.emitTranscriptMarker(
			m.fallbackLifecycleContext(),
			session,
			req.turnID,
			transcript.MarkerPromptDropped,
			"Queued input dispatched but its sent receipt could not be persisted.",
			queueEntryEvidence(entry.id, entry.sessionGeneration, store.SessionInputQueueStatusFailed, entry.mode, 0),
		)
		return
	}
	evidence := queueEntryEvidence(entry.id, entry.sessionGeneration, entry.status, entry.mode, 0)
	evidence["message_id"] = entry.messageID
	evidence["authored_text"] = entry.text
	evidence["input_event_id"] = entry.eventID
	evidence["target_turn_id"] = req.turnID
	m.emitTranscriptMarker(
		m.fallbackLifecycleContext(),
		session,
		req.turnID,
		transcript.MarkerPromptAccepted,
		"Queued input accepted for dispatch.",
		evidence,
	)
}

func (m *Manager) drainQueuedInputEvents(events <-chan acp.AgentEvent) {
	for range events {
		continue
	}
}
