package terminal

import (
	"encoding/hex"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const (
	inputRequestExpiryActorID   = "input-request-expiry"
	inputRequestNotFoundMessage = "terminal input request was not found"
)

type pendingInput struct {
	projection         PendingInputRequest
	session            *session
	result             chan InputOutcome
	timer              *time.Timer
	expiresAt          time.Time
	timerGeneration    uint64
	resolving          bool
	terminalResolution *inputTerminalResolution
}

type inputTerminalResolution struct {
	outcome InputResolutionOutcome
	actor   Actor
	reason  string
}

type inputRegistry struct {
	mu            sync.Mutex
	pending       map[InputRequestID]*pendingInput
	resolved      map[InputRequestID]ResolvedInputRequest
	resolvedOrder []InputRequestID
}

func newInputRegistry() *inputRegistry {
	return &inputRegistry{
		pending: make(map[InputRequestID]*pendingInput), resolved: make(map[InputRequestID]ResolvedInputRequest),
	}
}

func (r *inputRegistry) create(
	session *session,
	request InputRequest,
	redacted bool,
	requester Actor,
	generateID func() (InputRequestID, error),
) (*pendingInput, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	info := session.Info()
	terminalCount, scopeCount := 0, 0
	for _, candidate := range r.pending {
		if candidate.projection.TerminalID == info.ID && candidate.projection.ProfileID == info.ProfileID &&
			candidate.projection.WorkspaceID == info.WS {
			terminalCount++
		}
		if candidate.projection.ProfileID == info.ProfileID && candidate.projection.WorkspaceID == info.WS {
			scopeCount++
		}
	}
	if terminalCount >= maxInputRequestsPerTerminal {
		return nil, &Error{
			Code: ErrorCodeInputRequestLimitReached, Message: "terminal input request limit reached",
			Current: terminalCount, Max: maxInputRequestsPerTerminal, Err: ErrInputLimit,
		}
	}
	if scopeCount >= maxInputRequestsPerScope {
		return nil, &Error{
			Code: ErrorCodeInputRequestLimitReached, Message: "workspace input request limit reached",
			Current: scopeCount, Max: maxInputRequestsPerScope, Err: ErrInputLimit,
		}
	}
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	now := session.manager.now()
	pending := &pendingInput{
		projection: PendingInputRequest{
			ID: id, TerminalID: info.ID, WorkspaceID: info.WS, ProfileID: info.ProfileID,
			ProfileName: session.profileName, Reason: strings.TrimSpace(request.Reason),
			PromptExcerpt: strings.TrimSpace(request.PromptExcerpt), Redacted: redacted,
			RequestedAt: now, Requester: inputActorProjection(requester),
		},
		session:   session,
		result:    make(chan InputOutcome, 1),
		expiresAt: now.Add(session.manager.inputRequestTTL),
	}
	r.pending[id] = pending
	r.armExpiryLocked(pending, session.manager.inputRequestTTL)
	return pending, nil
}

func (r *inputRegistry) armExpiryLocked(pending *pendingInput, remaining time.Duration) {
	pending.timerGeneration++
	generation := pending.timerGeneration
	pending.timer = time.AfterFunc(remaining, func() {
		r.expire(pending, generation)
	})
}

func (r *inputRegistry) expire(pending *pendingInput, generation uint64) {
	r.mu.Lock()
	current := r.pending[pending.projection.ID]
	if current != pending || pending.resolving || pending.timerGeneration != generation {
		r.mu.Unlock()
		return
	}
	actor := Actor{Kind: ActorKindSystem, ID: inputRequestExpiryActorID, ProfileID: pending.projection.ProfileID}
	resolved := r.resolveLocked(
		pending,
		InputOutcome{Outcome: InputResolutionOutcomeExpired, Redacted: pending.projection.Redacted},
		actor,
		"",
	)
	r.mu.Unlock()
	r.finishResolution(resolved, InputOutcome{
		Outcome: InputResolutionOutcomeExpired, Redacted: resolved.projection.Redacted,
	})
	r.emitExpired(resolved)
}

func (r *inputRegistry) emitExpired(pending *pendingInput) {
	ctx, cancel := boundedCleanupContext(pending.session.ctx, inputEventDeliveryTimeout)
	defer cancel()
	pending.session.emitInputProvided(
		ctx,
		pending,
		Actor{Kind: ActorKindSystem, ID: inputRequestExpiryActorID, ProfileID: pending.projection.ProfileID},
		"expired",
		0,
		"",
	)
}

func (r *inputRegistry) claim(session *session, id InputRequestID) (*pendingInput, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending, err := r.lookupPendingLocked(session, id)
	if err != nil {
		return nil, err
	}
	pending.resolving = true
	pending.timerGeneration++
	if pending.timer != nil {
		pending.timer.Stop()
		pending.timer = nil
	}
	return pending, nil
}

func (r *inputRegistry) inspect(session *session, id InputRequestID) (*pendingInput, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lookupPendingLocked(session, id)
}

func (r *inputRegistry) lookupPendingLocked(
	session *session,
	id InputRequestID,
) (*pendingInput, error) {
	pending := r.pending[id]
	if pending == nil || pending.session != session {
		if resolved, ok := r.resolved[id]; ok {
			return nil, resolvedInputRequestError(resolved)
		}
		return nil, &Error{
			Code: ErrorCodeInputRequestNotFound, Message: inputRequestNotFoundMessage, Err: ErrInputNotFound,
		}
	}
	if pending.resolving {
		return nil, &Error{
			Code:    ErrorCodeInputRequestAnswered,
			Message: "terminal input request is already being resolved",
			Err:     ErrInputResolving,
		}
	}
	return pending, nil
}

func resolvedInputRequestError(resolved ResolvedInputRequest) error {
	switch resolved.Outcome {
	case InputResolutionOutcomeAnswered:
		return &Error{
			Code: ErrorCodeInputRequestAnswered, Message: errorMessageInputAlreadyResolved,
			Err: ErrInputAnswered,
		}
	case InputResolutionOutcomeSuperseded:
		return &Error{
			Code: ErrorCodeInputRequestSuperseded, Message: "terminal input request was superseded",
			Err: ErrInputSuperseded,
		}
	case InputResolutionOutcomeExpired:
		return &Error{
			Code: ErrorCodeInputRequestNotFound, Message: inputRequestNotFoundMessage,
			Err: ErrInputNotFound,
		}
	default:
		return fmt.Errorf("%s (%s): %w", errorMessageInputAlreadyResolved, resolved.Outcome, ErrInputResolved)
	}
}

func (r *inputRegistry) release(pending *pendingInput) {
	r.mu.Lock()
	if r.pending[pending.projection.ID] != pending || !pending.resolving {
		r.mu.Unlock()
		return
	}
	if pending.terminalResolution != nil {
		resolution := *pending.terminalResolution
		resolved := r.resolveLocked(
			pending,
			InputOutcome{Outcome: resolution.outcome, Redacted: pending.projection.Redacted},
			resolution.actor,
			resolution.reason,
		)
		r.mu.Unlock()
		r.finishTerminalResolution(resolved, resolution)
		return
	}
	pending.resolving = false
	remaining := pending.expiresAt.Sub(pending.session.manager.now())
	if remaining > 0 {
		r.armExpiryLocked(pending, remaining)
		r.mu.Unlock()
		return
	}
	actor := Actor{Kind: ActorKindSystem, ID: inputRequestExpiryActorID, ProfileID: pending.projection.ProfileID}
	resolved := r.resolveLocked(
		pending,
		InputOutcome{Outcome: InputResolutionOutcomeExpired, Redacted: pending.projection.Redacted},
		actor,
		"",
	)
	r.mu.Unlock()
	r.finishResolution(resolved, InputOutcome{
		Outcome: InputResolutionOutcomeExpired, Redacted: resolved.projection.Redacted,
	})
	r.emitExpired(resolved)
}

func (r *inputRegistry) complete(pending *pendingInput, outcome InputOutcome, actor Actor, reason string) bool {
	r.mu.Lock()
	if r.pending[pending.projection.ID] != pending || !pending.resolving {
		r.mu.Unlock()
		return false
	}
	resolved := r.resolveLocked(pending, outcome, actor, reason)
	r.mu.Unlock()
	r.finishResolution(resolved, outcome)
	return true
}

func (r *inputRegistry) resolveLocked(
	pending *pendingInput,
	outcome InputOutcome,
	actor Actor,
	reason string,
) *pendingInput {
	id := pending.projection.ID
	delete(r.pending, id)
	r.resolved[id] = ResolvedInputRequest{
		ID: id, TerminalID: pending.projection.TerminalID, WorkspaceID: pending.projection.WorkspaceID,
		ProfileID: pending.projection.ProfileID, ProfileName: pending.projection.ProfileName,
		Requester: pending.projection.Requester, Outcome: outcome.Outcome, ResolvedBy: inputActorProjection(actor),
		Reason: strings.TrimSpace(reason), Redacted: pending.projection.Redacted, Length: max(outcome.Length, 0),
		RequestedAt: pending.projection.RequestedAt, ResolvedAt: pending.session.manager.now(),
	}
	r.resolvedOrder = append(r.resolvedOrder, id)
	if len(r.resolvedOrder) > 256 {
		oldest := r.resolvedOrder[0]
		r.resolvedOrder = r.resolvedOrder[1:]
		delete(r.resolved, oldest)
	}
	pending.timerGeneration++
	return pending
}

func (r *inputRegistry) finishResolution(pending *pendingInput, outcome InputOutcome) {
	if pending.timer != nil {
		pending.timer.Stop()
	}
	pending.result <- outcome
}

func (r *inputRegistry) resolveTerminal(session *session, resolution inputTerminalResolution) {
	r.mu.Lock()
	resolved := make([]*pendingInput, 0)
	for _, pending := range r.pending {
		if pending.session != session {
			continue
		}
		if pending.resolving {
			if pending.terminalResolution == nil {
				copyOfResolution := resolution
				pending.terminalResolution = &copyOfResolution
			}
			continue
		}
		resolved = append(resolved, r.resolveLocked(
			pending,
			InputOutcome{Outcome: resolution.outcome, Redacted: pending.projection.Redacted},
			resolution.actor,
			resolution.reason,
		))
	}
	r.mu.Unlock()
	for _, pending := range resolved {
		r.finishTerminalResolution(pending, resolution)
	}
}

func (r *inputRegistry) finishTerminalResolution(
	pending *pendingInput,
	resolution inputTerminalResolution,
) {
	r.finishResolution(pending, InputOutcome{
		Outcome: resolution.outcome, Redacted: pending.projection.Redacted,
	})
	ctx, cancel := boundedCleanupContext(pending.session.ctx, inputEventDeliveryTimeout)
	defer cancel()
	pending.session.emitInputProvided(
		ctx, pending, resolution.actor, resolution.outcome, 0, resolution.reason,
	)
}

func (r *inputRegistry) list(workspaceID string, scope store.ReadScope, terminalID ID) []PendingInputRequest {
	r.mu.Lock()
	items := make([]PendingInputRequest, 0)
	for _, pending := range r.pending {
		item := pending.projection
		if item.WorkspaceID == workspaceID && scope.Matches(item.ProfileID) &&
			(terminalID == "" || item.TerminalID == terminalID) {
			items = append(items, item)
		}
	}
	r.mu.Unlock()
	slices.SortFunc(items, func(left, right PendingInputRequest) int {
		if left.RequestedAt.Equal(right.RequestedAt) {
			return strings.Compare(string(left.ID), string(right.ID))
		}
		if left.RequestedAt.Before(right.RequestedAt) {
			return -1
		}
		return 1
	})
	return items
}

func (r *inputRegistry) listResolved(
	workspaceID string,
	scope store.ReadScope,
	terminalID ID,
) []ResolvedInputRequest {
	r.mu.Lock()
	items := make([]ResolvedInputRequest, 0)
	for _, id := range r.resolvedOrder {
		item, ok := r.resolved[id]
		if ok && item.WorkspaceID == workspaceID && scope.Matches(item.ProfileID) &&
			(terminalID == "" || item.TerminalID == terminalID) {
			items = append(items, item)
		}
	}
	r.mu.Unlock()
	slices.SortFunc(items, func(left, right ResolvedInputRequest) int {
		if left.ResolvedAt.Equal(right.ResolvedAt) {
			return strings.Compare(string(left.ID), string(right.ID))
		}
		if left.ResolvedAt.Before(right.ResolvedAt) {
			return -1
		}
		return 1
	})
	return items
}

func inputActorProjection(actor Actor) InputActorProjection {
	return InputActorProjection{Kind: actor.Kind, ID: strings.TrimSpace(actor.ID)}
}

func newInputRequestID(entropy io.Reader) (InputRequestID, error) {
	raw := make([]byte, 8)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		return "", fmt.Errorf("terminal: generate input request id: %w", err)
	}
	return InputRequestID("input-" + hex.EncodeToString(raw)), nil
}
