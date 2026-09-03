package session

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type humanQueuedInput struct {
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
	m.dispatchHumanQueuedInput(target, session, humanQueuedInput{
		id:                entry.ID,
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
	entry humanQueuedInput,
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
	entry humanQueuedInput,
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
	entry humanQueuedInput,
	req promptRequest,
	cause error,
) {
	if errors.Is(cause, ErrPromptInProgress) {
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
	entry humanQueuedInput,
	req promptRequest,
) {
	if err := m.inputQueue.MarkSent(m.fallbackLifecycleContext(), target, entry.id); err != nil {
		m.sessionLogger(session).Warn("session: mark queued input sent failed", "entry_id", entry.id, "error", err)
	}
	m.emitTranscriptMarker(
		m.fallbackLifecycleContext(),
		session,
		req.turnID,
		transcript.MarkerPromptAccepted,
		"Queued input accepted for dispatch.",
		queueEntryEvidence(entry.id, entry.sessionGeneration, entry.status, entry.mode, 0),
	)
}

func (m *Manager) drainQueuedInputEvents(events <-chan acp.AgentEvent) {
	for range events {
		continue
	}
}
