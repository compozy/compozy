package session

import (
	"context"
	"encoding/json/v2"
	"errors"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type managerSubagentRuntime struct{ m *Manager }

var _ subagentRuntime = managerSubagentRuntime{}

func (r managerSubagentRuntime) Snapshot(ctx context.Context, id string) (subagentSnapshot, error) {
	info, err := r.m.Status(ctx, id)
	if err != nil {
		return subagentSnapshot{}, err
	}
	snap := subagentSnapshot{Info: info}
	r.m.lifecycleMu.Lock()
	snap.Delivering = r.m.worktreeDeliveryFences[info.WorkspaceID+"\x00"+info.WorktreeID]
	r.m.lifecycleMu.Unlock()
	if child, ok := r.m.Get(id); ok {
		snap.Active = info.Liveness != nil && info.Liveness.Activity != nil && info.Liveness.Activity.TurnID != ""
		snap.TurnID = child.CurrentTurnID()
		if proc := child.processHandle(); proc != nil {
			caps := proc.CapsSnapshot()
			_, supports := r.m.driver.(AgentSteerer)
			snap.CanSteer = supports &&
				(caps.SteerCapability == compozyconfig.SteerCapabilityExtension ||
					caps.SteerCapability == compozyconfig.SteerCapabilityConcurrentPrompt)
		}
	}
	if r.m.inputQueue != nil {
		pending, err := r.m.inputQueue.List(ctx, id)
		if err != nil {
			return subagentSnapshot{}, err
		}
		snap.Queued = len(pending)
		for i := range pending {
			if pending[i].Mode == store.SessionInputQueueModeSteer {
				snap.UserSteer = true
			}
		}
	}
	return snap, nil
}
func (r managerSubagentRuntime) Spawn(ctx context.Context, opts SpawnOpts) (string, error) {
	child, err := r.m.Spawn(ctx, opts)
	if child == nil {
		return "", err
	}
	return child.ID, err
}
func (r managerSubagentRuntime) Admit(
	ctx context.Context,
	row store.SessionSubagent,
	text string,
	resume bool,
) error {
	if row.ChildSessionID == nil {
		return errors.New("session: subagent child is missing")
	}
	// Explicit queue mode does not resume retained sessions. Recovery owns the
	// missing first admission, including children accepted before runtime binding.
	if _, active := r.m.Get(*row.ChildSessionID); !active {
		if !resume {
			return ErrSessionNotActive
		}
		if _, err := r.m.Resume(ctx, *row.ChildSessionID); err != nil {
			return err
		}
	}
	result, err := r.m.SendPrompt(
		ctx,
		*row.ChildSessionID,
		SendPromptOpts{Message: text, MessageID: row.ID, IdempotencyKey: row.ID, Mode: BusyInputModeQueue},
	)
	if err != nil {
		return err
	}
	if result.Events != nil {
		r.m.startTrackedPromptTask(func() {
			for range result.Events {
				continue
			}
		})
	}
	return nil
}
func (r managerSubagentRuntime) HasAdmission(ctx context.Context, row store.SessionSubagent) (bool, error) {
	if row.ChildSessionID == nil {
		return false, nil
	}
	if r.m.inputQueue != nil {
		inputs, err := r.m.inputQueue.List(ctx, *row.ChildSessionID)
		if err != nil {
			return false, err
		}
		for i := range inputs {
			if inputs[i].IdempotencyKey == row.ID {
				return true, nil
			}
		}
	}
	found := false
	err := r.walkTranscript(ctx, *row.ChildSessionID, func(entry transcript.Entry) bool {
		var meta struct {
			MessageID string `json:"message_id"`
		}
		if json.Unmarshal(entry.Message.Metadata, &meta) == nil && entry.Message.Role == transcript.UIRoleUser &&
			meta.MessageID == row.ID {
			found = true
			return true
		}
		return false
	})
	return found, err
}
func (r managerSubagentRuntime) Stop(ctx context.Context, id string) error { return r.m.Stop(ctx, id) }
func (r managerSubagentRuntime) Result(ctx context.Context, id string) (subagentTurnResult, error) {
	query := store.EventQuery{Limit: 200}
	var turn string
	var events []store.SessionEvent
	for {
		page, err := r.m.Events(ctx, id, query)
		if err != nil {
			return subagentTurnResult{}, err
		}
		finished := len(page) < query.Limit
		for _, event := range slices.Backward(page) {
			if event.TurnID == "" {
				continue
			}
			decoded, err := transcript.UnmarshalAgentEvent(event.Content)
			if err != nil {
				return subagentTurnResult{}, err
			}
			if decoded.ParentToolCallID() != "" {
				continue
			}
			events = append(events, event)
			// Stop receipts can own newer turn IDs than the last prompt. Select
			// the admitted prompt, never a lifecycle-only turn or an older answer.
			if event.Type == acp.EventTypeUserMessage || event.Type == acp.EventTypeSyntheticReentry {
				turn = event.TurnID
				finished = true
				break
			}
		}
		if finished {
			break
		}
		query.BeforeSequence = page[0].Sequence
	}
	events = slices.DeleteFunc(events, func(event store.SessionEvent) bool { return event.TurnID != turn })
	// Canonical assembly preserves chunk boundaries and complete assistant messages.
	// The UI projection merges a turn's assistant segments and has no turn metadata.
	var result subagentTurnResult
	completed, err := subagentTurnCompleted(events)
	result.Completed = completed
	if err != nil {
		return subagentTurnResult{}, err
	}
	for _, event := range events {
		if event.Type != acp.EventTypeError {
			continue
		}
		// The settling turn ended in an error (a provider failure that left the
		// session itself alive): the subagent failed with that error (UT-021).
		if decoded, err := transcript.UnmarshalAgentEvent(event.Content); err == nil {
			result.Error = firstTrimmedNonEmpty(decoded.Error, decoded.Text, "turn failed")
		}
		break
	}
	messages, err := transcript.Assemble(events)
	if err != nil {
		return subagentTurnResult{}, err
	}
	for _, message := range slices.Backward(messages) {
		if message.Role == transcript.RoleAssistant && strings.TrimSpace(message.Content) != "" {
			result.Text = message.Content
			break
		}
	}
	return result, nil
}

func subagentTurnCompleted(events []store.SessionEvent) (bool, error) {
	for _, event := range events {
		if event.Type != acp.EventTypeDone {
			continue
		}
		decoded, err := transcript.UnmarshalAgentEvent(event.Content)
		if err != nil {
			return false, err
		}
		return decoded.PromptStopReason != acp.PromptStopReasonCancelled &&
			decoded.StopReason != string(acp.PromptStopReasonCancelled), nil
	}
	return false, nil
}

func (r managerSubagentRuntime) walkTranscript(
	ctx context.Context,
	id string,
	visit func(transcript.Entry) bool,
) error {
	query := transcript.PageQuery{Limit: 200}
	for {
		page, err := r.m.TranscriptPage(ctx, id, query)
		if err != nil {
			return err
		}
		for _, entry := range slices.Backward(page.Entries) {
			if visit(entry) {
				return nil
			}
		}
		if !page.HasOlder {
			return nil
		}
		query.BeforeSequence = page.NextBeforeSequence
	}
}

func (r managerSubagentRuntime) QueueWake(
	ctx context.Context,
	wake store.SessionSubagentWake,
	rows []store.SessionSubagent,
) (string, error) {
	parent, err := r.m.Status(ctx, wake.ParentSessionID)
	if err != nil {
		return "", err
	}
	if !subagentParentAcceptsWake(parent) {
		return "", ErrSessionNotActive
	}
	if r.m.inputQueue == nil {
		return "", errors.New("session: subagent wakes require the durable input queue")
	}
	generation, err := r.m.currentInputGeneration(ctx, parent.ID)
	if err != nil {
		return "", err
	}
	id, err := store.NewID("inq")
	if err != nil {
		return "", err
	}
	turn, err := r.m.newPromptTurnID()
	if err != nil {
		return "", err
	}
	metadata, err := json.Marshal(subagentWakeMeta(rows))
	if err != nil {
		return "", err
	}
	entry, _, err := r.m.inputQueueStore.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
		ID:                id,
		SessionID:         parent.ID,
		OwnerKind:         store.SessionInputOwnerSynthetic,
		MessageID:         wake.WakeMessageID,
		Priority:          1,
		TurnID:            turn,
		Mode:              store.SessionInputQueueModeQueue,
		Delivery:          store.SessionInputDeliveryAfterTurn,
		Text:              subagentWakeText(rows),
		SessionGeneration: generation,
		QueueCap:          r.m.busyInput.QueueCap,
		SyntheticPrompt: &store.SessionInputSyntheticPrompt{
			RunID:    wake.WakeMessageID,
			Delivery: store.SessionInputDeliveryAfterTurn,
			Metadata: metadata,
		},
		Now: r.m.now(),
	})
	if err != nil {
		return "", err
	}
	// The caller records the wake's queue identity before a tracked dispatch can call back.
	r.m.startTrackedPromptTask(func() { r.m.startNextQueuedInputPrompt(parent.ID) })
	return entry.ID, nil
}
func (r managerSubagentRuntime) CancelWake(ctx context.Context, wake store.SessionSubagentWake) error {
	if wake.InputEntryID == "" {
		return nil
	}
	_, err := r.m.inputQueue.Cancel(ctx, wake.ParentSessionID, wake.InputEntryID)
	if errors.Is(err, store.ErrSessionInputQueueEntryNotQueued) {
		return nil
	}
	return err
}
func (r managerSubagentRuntime) Steer(ctx context.Context, parent, turn, id, text string) (acp.SteerResult, error) {
	child, ok := r.m.Get(parent)
	if !ok {
		return acp.SteerResult{}, ErrSessionNotFound
	}
	driver, ok := r.m.driver.(AgentSteerer)
	if !ok {
		return acp.SteerResult{Attempt: acp.SteerAttemptUnsupported}, nil
	}
	if child.CurrentTurnID() != turn {
		return acp.SteerResult{}, acp.ErrSteerTurnMismatch
	}
	steerCtx, cancel := context.WithTimeout(ctx, defaultLifecycleTimeout)
	defer cancel()
	result, err := driver.Steer(steerCtx, child.processHandle(), turn, text)
	if err == nil &&
		(result.Attempt == acp.SteerAttemptInjected || result.Attempt == acp.SteerAttemptPendingInjection) {
		r.m.emitTranscriptMarker(
			ctx,
			child,
			turn,
			transcript.MarkerPromptSteered,
			text,
			map[string]any{"message_id": id, "kind": subagentWakeKind},
		)
	}
	return result, err
}
func (r managerSubagentRuntime) PublishParent(ctx context.Context, parent string) {
	info, err := r.m.Status(ctx, parent)
	if err != nil {
		r.m.logger.WarnContext(ctx, "subagent.catalog_publish", "parent_session_id", parent, "error", err)
		return
	}
	r.m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
}
func (r managerSubagentRuntime) SettleParent(ctx context.Context, parent string) error {
	if err := r.m.settleSessionAttention(ctx, parent, r.m.now().UTC()); err != nil {
		return err
	}
	r.PublishParent(ctx, parent)
	return nil
}

func (r managerSubagentRuntime) WakeInputStatus(ctx context.Context, wake store.SessionSubagentWake) (string, error) {
	if wake.InputEntryID == "" {
		return "", nil
	}
	entry, err := r.m.inputQueue.Get(ctx, wake.ParentSessionID, wake.InputEntryID)
	if errors.Is(err, store.ErrSessionInputQueueEntryNotFound) {
		return "", nil
	}
	return entry.Status, err
}

// WakeTurnCompleted reports whether the turn a wake input started finished
// without being canceled. Admission (input status sent) alone proves nothing:
// a crash can end the turn before the agent consumed the wake.
func (r managerSubagentRuntime) WakeTurnCompleted(ctx context.Context, wake store.SessionSubagentWake) (bool, error) {
	if wake.InputEntryID == "" {
		return false, nil
	}
	entry, err := r.m.inputQueue.Get(ctx, wake.ParentSessionID, wake.InputEntryID)
	if errors.Is(err, store.ErrSessionInputQueueEntryNotFound) || (err == nil && entry.TurnID == "") {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	query := store.EventQuery{Limit: 200}
	var events []store.SessionEvent
	for {
		page, err := r.m.Events(ctx, wake.ParentSessionID, query)
		if err != nil {
			return false, err
		}
		started := false
		for _, event := range slices.Backward(page) {
			if event.TurnID != entry.TurnID {
				continue
			}
			events = append(events, event)
			if event.Type == acp.EventTypeSyntheticReentry || event.Type == acp.EventTypeUserMessage {
				started = true
				break
			}
		}
		if started || len(page) < query.Limit {
			break
		}
		query.BeforeSequence = page[0].Sequence
	}
	return subagentTurnCompleted(events)
}

func (r managerSubagentRuntime) ResumeChild(ctx context.Context, row store.SessionSubagent) error {
	info, err := r.m.Status(ctx, *row.ChildSessionID)
	if err != nil {
		return err
	}
	identity := row.ID + ":resume:" + info.UpdatedAt.UTC().Format("20060102T150405.000000000")
	if _, err := r.m.Resume(ctx, *row.ChildSessionID); err != nil {
		return err
	}
	result, err := r.m.SendPrompt(ctx, *row.ChildSessionID, SendPromptOpts{
		Message:        "Continue the interrupted delegated task.",
		MessageID:      identity,
		IdempotencyKey: identity,
		Mode:           BusyInputModeQueue,
	})
	if err != nil {
		return err
	}
	if result.Events != nil {
		r.m.startTrackedPromptTask(func() {
			for range result.Events {
				continue
			}
		})
	}
	return nil
}
