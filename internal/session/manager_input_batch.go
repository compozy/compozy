package session

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// maxQueuedInputBatch bounds how many queued messages one dispatched turn may carry.
const maxQueuedInputBatch = 8

// queuedInputBatchSeparator divides the attributed messages of one batched provider prompt.
const queuedInputBatchSeparator = "\n\n---\n\n"

// claimQueuedInputBatch leases the consecutive messages from other sessions and reply wakes that
// follow a claimed head of the same kind, so one turn answers them together. Operator input,
// interrupts, Goal prompts, subagent wakes, and entries whose attachments, slash commands, or
// runtime differ always end the batch and keep a turn of their own.
func (m *Manager) claimQueuedInputBatch(
	session *Session,
	target string,
	head *store.SessionInputQueueEntry,
) []queuedInput {
	if !batchableQueuedInput(head) {
		return nil
	}
	effective := effectiveQueuedInputRuntime(session)
	headRuntime := effective(head.Runtime)
	accept := func(candidate *store.SessionInputQueueEntry) bool {
		return batchableQueuedInput(candidate) &&
			len(candidate.Attachments) == 0 &&
			candidate.SessionGeneration == head.SessionGeneration &&
			sameSessionInputRuntime(effective(candidate.Runtime), headRuntime)
	}
	claimed, err := m.inputQueue.ClaimBatch(m.fallbackLifecycleContext(), target, maxQueuedInputBatch-1, accept)
	if err != nil {
		m.sessionLogger(session).Warn("session: claim queued input batch failed", "error", err)
		return nil
	}
	batch := make([]queuedInput, 0, len(claimed))
	for index := range claimed {
		batch = append(batch, queuedInputFromEntry(&claimed[index]))
	}
	return batch
}

// batchableQueuedInput reports an after-turn message from another session or a reply wake without
// slash commands. Only those may share a turn; human input is never merged.
func batchableQueuedInput(entry *store.SessionInputQueueEntry) bool {
	if entry.Delivery != store.SessionInputDeliveryAfterTurn || len(entry.SkillInvocations) > 0 {
		return false
	}
	if entry.Mode != store.SessionInputQueueModeQueue && entry.Mode != store.SessionInputQueueModeSteer {
		return false
	}
	switch entry.OwnerKind {
	case "":
		return entry.OriginClass == store.SessionInputOriginClassAgent
	case store.SessionInputOwnerSynthetic:
		if entry.SyntheticPrompt == nil {
			return false
		}
		var meta acp.PromptSyntheticMeta
		if json.Unmarshal(entry.SyntheticPrompt.Metadata, &meta) != nil {
			return false
		}
		return meta.Kind == acp.PromptSyntheticKindSessionReply
	default:
		return false
	}
}

// effectiveQueuedInputRuntime resolves the runtime an entry dispatches on: an entry without a runtime
// selection (a reply wake) runs on the session's current runtime, so it batches with messages stamped
// with that same runtime.
func effectiveQueuedInputRuntime(session *Session) func(store.SessionInputRuntime) store.SessionInputRuntime {
	inherited, err := normalizePromptRuntimeSelection(session, nil)
	return func(runtime store.SessionInputRuntime) store.SessionInputRuntime {
		if err != nil || runtimeSelectionFromStore(runtime) != nil {
			return runtime
		}
		return storeRuntimeSelection(inherited)
	}
}

func sameSessionInputRuntime(left, right store.SessionInputRuntime) bool {
	left, right = left.Normalize(), right.Normalize()
	return left.Provider == right.Provider && left.Model == right.Model &&
		left.ReasoningEffort == right.ReasoningEffort && left.Speed == right.Speed &&
		slices.EqualFunc(left.ACPOptions, right.ACPOptions, func(a, b store.SessionACPOptionSelection) bool {
			return a.ID == b.ID && a.ValueID == b.ValueID &&
				(a.BoolValue == nil) == (b.BoolValue == nil) && (a.BoolValue == nil || *a.BoolValue == *b.BoolValue)
		})
}

// releaseQueuedInputBatch returns batched entries that never started to the queue.
func (m *Manager) releaseQueuedInputBatch(session *Session, target string, batch []queuedInput) {
	for index := range batch {
		if err := m.inputQueue.Release(m.fallbackLifecycleContext(), target, batch[index].id); err != nil {
			m.sessionLogger(session).Warn(
				"session: release batched queued input failed", "entry_id", batch[index].id, "error", err,
			)
		}
	}
}

// batchedPromptInput is one queued message carried by a turn another entry started.
type batchedPromptInput struct {
	request promptRequest
	// message is the hook-reviewed text the provider receives under its sender header.
	message string
}

// prepareBatchedPromptInputs builds the request of every batched message in the head's turn and
// passes each through input.pre_submit, exactly as it would have been reviewed on its own. A follower
// whose origin, request, or hook fails leaves the batch: it returns to the queue to dispatch (and settle
// any failure) as its own head later, and the head's turn proceeds without it. A follower never fails the
// head.
func (m *Manager) prepareBatchedPromptInputs(
	ctx context.Context,
	session *Session,
	req *promptRequest,
) []batchedPromptInput {
	if len(req.batch) == 0 {
		return nil
	}
	inputs := make([]batchedPromptInput, 0, len(req.batch))
	for _, entry := range req.batch {
		input, err := m.prepareBatchedPromptInput(ctx, session, req, entry)
		if err != nil {
			m.dropBatchedPromptInput(session, req, entry, err)
			continue
		}
		inputs = append(inputs, input)
	}
	return inputs
}

func (m *Manager) prepareBatchedPromptInput(
	ctx context.Context,
	session *Session,
	req *promptRequest,
	entry queuedInput,
) (batchedPromptInput, error) {
	follower, err := m.newQueuedInputPromptRequest(req.target, entry)
	if err != nil {
		return batchedPromptInput{}, err
	}
	follower.turnID = req.turnID
	message, err := m.dispatchInputPreSubmit(ctx, session, req.turnID, follower.turnSource, follower.message, nil)
	if err != nil {
		return batchedPromptInput{}, err
	}
	follower.message = message
	return batchedPromptInput{request: follower, message: message}, nil
}

// dropBatchedPromptInput returns a follower that cannot join the head's turn to the queue and tells the
// dispatcher, so the head neither marks it sent nor releases it a second time.
func (m *Manager) dropBatchedPromptInput(session *Session, req *promptRequest, entry queuedInput, cause error) {
	logger := m.sessionLogger(session)
	logger.Warn("session: batched queued input left the batch", "entry_id", entry.id, "error", cause)
	if err := m.inputQueue.Release(m.fallbackLifecycleContext(), req.target, entry.id); err != nil {
		logger.Warn("session: release dropped batched input failed", "entry_id", entry.id, "error", err)
	}
	if req.batchDropped != nil {
		req.batchDropped(entry.id)
	}
}

// batchedPromptMessage appends every batched message, each under its own sender header, to the
// head's provider prompt. Reply wakes already name their sender in their text.
func batchedPromptMessage(message string, inputs []batchedPromptInput) string {
	if len(inputs) == 0 {
		return message
	}
	var digest strings.Builder
	digest.WriteString(message)
	for index := range inputs {
		digest.WriteString(queuedInputBatchSeparator)
		digest.WriteString(promptOriginMessage(inputs[index].request.meta.Origin, inputs[index].message))
	}
	return digest.String()
}

// raiseBatchedPromptHops folds every batched message's hop into the shared turn before the provider runs.
func raiseBatchedPromptHops(session *Session, inputs []batchedPromptInput) {
	for index := range inputs {
		follower := inputs[index].request
		session.raiseTurnHop(follower.turnID, promptMetaHop(follower.meta))
	}
}

// recordBatchedPromptInputs persists one input event per batched message in the shared turn, with
// that message's own origin, message id, and event id, so the transcript keeps each sender and every
// reply watch binds to the turn that consumed its message. It runs only once the turn durably started:
// a head that fails earlier releases its followers, and an event recorded under the failed turn would
// collide with the follower's own dispatch later (event ids are deterministic per admission). The turn
// already carries these messages, so a failed record is logged rather than failing the started turn.
func (m *Manager) recordBatchedPromptInputs(ctx context.Context, session *Session, inputs []batchedPromptInput) {
	for index := range inputs {
		follower := inputs[index].request
		if err := m.recordPromptInputEvent(ctx, session, &follower); err != nil {
			m.sessionLogger(session).Error(
				"session: record batched input event failed", "message_id", follower.messageID, "error", err,
			)
		}
	}
}

// requestDispatchMessage builds the provider prompt of req's turn: its own attributed input followed by
// every batched message it carries.
func (m *Manager) requestDispatchMessage(
	ctx context.Context,
	session *Session,
	req *promptRequest,
	message string,
) (preparedPromptInput, error) {
	batched := m.prepareBatchedPromptInputs(ctx, session, req)
	prepared, err := m.promptDispatchMessage(ctx, session, message, req.delivery)
	if err != nil {
		return preparedPromptInput{}, err
	}
	prepared.message = batchedPromptMessage(prepared.message, batched)
	prepared.batched = batched
	return prepared, nil
}
