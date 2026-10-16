package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type queuedInput struct {
	origin            json.RawMessage
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
	// batch holds the consecutive agent and reply entries claimed to run in this entry's turn.
	batch []queuedInput
}

func queuedInputFromEntry(entry *store.SessionInputQueueEntry) queuedInput {
	return queuedInput{
		id:                entry.ID,
		syntheticPrompt:   entry.SyntheticPrompt,
		origin:            entry.Origin,
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
	}
}

// maxQueuedInputSelectAttempts bounds peek/claim disagreement retries. Each
// disagreement means the queue changed between the reads; a persistent one
// would otherwise recurse through Release without end.
const maxQueuedInputSelectAttempts = 3

// queuedInputReselectDelay spaces the one deferred retry after the selection bound, so a queue that
// kept changing is picked up again even when no turn end or new enqueue follows.
const queuedInputReselectDelay = time.Second

func (m *Manager) startNextQueuedInputPrompt(sessionID string) {
	if m.selectNextQueuedInputPrompt(sessionID) {
		// The bound only paces selection; the queue stays live. One deferred pass re-kicks it, and
		// any later turn end or enqueue does the same.
		m.startTrackedPromptTask(func() {
			timer := time.NewTimer(queuedInputReselectDelay)
			defer timer.Stop()
			select {
			case <-m.fallbackLifecycleContext().Done():
				return
			case <-timer.C:
			}
			m.selectNextQueuedInputPrompt(sessionID)
		})
	}
}

// selectNextQueuedInputPrompt reports whether selection stopped at its bound with the queue still
// changing under it.
func (m *Manager) selectNextQueuedInputPrompt(sessionID string) bool {
	for attempt := 1; attempt <= maxQueuedInputSelectAttempts; attempt++ {
		if !m.tryStartNextQueuedInputPrompt(sessionID, attempt) {
			return false
		}
	}
	return true
}

// tryStartNextQueuedInputPrompt reports whether selection must retry because
// the claimed entry differed from the peeked one.
func (m *Manager) tryStartNextQueuedInputPrompt(sessionID string, attempt int) bool {
	target, session, selected, ok := m.peekNextQueuedInputPrompt(sessionID)
	if !ok {
		return false
	}
	if selected.OwnerKind == managedInputOwnerGoal {
		m.startManagedInputPrompt(session, managedInputFromQueueEntry(&selected))
		return false
	}
	dispatchCtx, unlock := m.lockSubagentInputDispatch(m.fallbackLifecycleContext(), &selected)
	defer unlock()
	entry, ok, err := m.inputQueue.ClaimNext(m.fallbackLifecycleContext(), target)
	if err != nil {
		m.sessionLogger(session).Warn("session: claim queued input failed", "error", err)
		return false
	}
	if !ok {
		return false
	}
	if entry.ID != selected.ID {
		if releaseErr := m.inputQueue.Release(m.fallbackLifecycleContext(), target, entry.ID); releaseErr != nil {
			m.sessionLogger(session).Warn(
				"session: release reordered queued input failed",
				"entry_id", entry.ID,
				"error", releaseErr,
			)
			return false
		}
		if attempt == maxQueuedInputSelectAttempts {
			m.sessionLogger(session).Warn(
				"session: queued input selection kept changing; retrying shortly",
				"peeked_entry_id", selected.ID, "claimed_entry_id", entry.ID, "attempts", attempt,
			)
		}
		return true
	}
	head := queuedInputFromEntry(&entry)
	head.batch = m.claimQueuedInputBatch(session, target, &entry)
	m.dispatchQueuedInput(dispatchCtx, unlock, target, session, head)
	return false
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
	dispatchCtx context.Context,
	unlock func(),
	target string,
	session *Session,
	entry queuedInput,
) {
	req, err := m.newQueuedInputPromptRequest(target, entry)
	if err != nil {
		unlock()
		m.handleQueuedInputDispatchError(session, target, entry, req, err)
		return
	}
	if service := m.ReplyWatches(); service != nil {
		service.OnSteerResolved(m.fallbackLifecycleContext(), target, entry.messageID)
		for index := range entry.batch {
			service.OnSteerResolved(m.fallbackLifecycleContext(), target, entry.batch[index].messageID)
		}
	}
	if req.meta.Synthetic != nil && req.meta.Synthetic.Kind == subagentWakeKind {
		if service := m.subagentService(); service != nil {
			if err := service.OnWakeDispatched(dispatchCtx, target, entry.messageID); err != nil {
				unlock()
				m.handleQueuedInputDispatchError(session, target, entry, req, err)
				return
			}
		}
	}
	// Freeze the claimed batch, then release the parent lock before submission: failed
	// provider setup can synchronously publish a turn-settled lifecycle edge.
	unlock()
	events, err := m.submitPromptRequest(m.fallbackLifecycleContext(), req)
	if err != nil {
		m.handleQueuedInputDispatchError(session, target, entry, req, err)
		return
	}
	m.acceptQueuedInputDispatch(session, target, entry, req)
	m.startTrackedPromptTask(func() {
		canceled, failed := m.drainSubagentInputEvents(events)
		if req.meta.Synthetic != nil && req.meta.Synthetic.Kind == subagentWakeKind {
			if service := m.subagentService(); service != nil {
				if failed {
					m.logSubagentError(service.OnWakeFailed(m.fallbackLifecycleContext(), target, entry.messageID))
				} else {
					m.logSubagentError(
						service.OnWakeTurnSettled(m.fallbackLifecycleContext(), target, entry.messageID, canceled),
					)
				}
			}
		}
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
		batch:            entry.batch,
	}
	origin, err := decodePromptOrigin(entry.origin)
	if err != nil {
		return req, err
	}
	req.meta.Origin = origin
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
		req.delivery = PromptDelivery(entry.syntheticPrompt.Delivery)
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
	// Batched entries never started: they return to the queue whatever failed, and a follower that
	// fails on its own is retried as the next head and settled there.
	m.releaseQueuedInputBatch(session, target, entry.batch)
	if errors.Is(cause, ErrPromptInProgress) || errors.Is(cause, ErrSessionNotActive) {
		if err := m.inputQueue.Release(m.fallbackLifecycleContext(), target, entry.id); err != nil {
			m.sessionLogger(session).Warn("session: release queued input failed", "entry_id", entry.id, "error", err)
		}
		return
	}
	if err := m.inputQueue.MarkFailed(m.fallbackLifecycleContext(), target, entry.id, cause.Error()); err != nil {
		m.sessionLogger(session).Warn("session: mark queued input failed", "entry_id", entry.id, "error", err)
	}
	if service := m.ReplyWatches(); service != nil {
		service.OnSteerResolved(m.fallbackLifecycleContext(), target, entry.messageID)
		for index := range entry.batch {
			service.OnSteerResolved(m.fallbackLifecycleContext(), target, entry.batch[index].messageID)
		}
	}
	if req.meta.Synthetic != nil && req.meta.Synthetic.Kind == subagentWakeKind {
		if service := m.subagentService(); service != nil {
			m.logSubagentError(service.OnWakeFailed(m.fallbackLifecycleContext(), target, entry.messageID))
		}
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
	m.acceptQueuedInputEntry(session, target, entry, req.turnID)
	for index := range entry.batch {
		m.acceptQueuedInputEntry(session, target, entry.batch[index], req.turnID)
	}
}

func (m *Manager) acceptQueuedInputEntry(session *Session, target string, entry queuedInput, turnID string) {
	if err := m.markQueuedInputSent(target, entry.id, turnID); err != nil {
		// The turn already started with this input, so the entry must not
		// become failed: a failed row invites a retry that duplicates the turn.
		// The lease stays dispatching until boot recovery settles it.
		m.sessionLogger(session).Error(
			"session: mark queued input sent failed after dispatch", "entry_id", entry.id, "error", err,
		)
		return
	}
	evidence := queueEntryEvidence(entry.id, entry.sessionGeneration, entry.status, entry.mode, 0)
	evidence["message_id"] = entry.messageID
	evidence["authored_text"] = entry.text
	evidence["input_event_id"] = entry.eventID
	evidence["target_turn_id"] = turnID
	m.emitTranscriptMarker(
		m.fallbackLifecycleContext(),
		session,
		turnID,
		transcript.MarkerPromptAccepted,
		"Queued input accepted for dispatch.",
		evidence,
	)
}

// queuedInputSentAttempts bounds retries of the idempotent sent receipt.
const queuedInputSentAttempts = 3

func (m *Manager) markQueuedInputSent(target, entryID, turnID string) error {
	var err error
	for range queuedInputSentAttempts {
		if err = m.inputQueue.MarkSentInTurn(m.fallbackLifecycleContext(), target, entryID, turnID); err == nil {
			return nil
		}
	}
	return err
}
