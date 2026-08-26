package terminal

import (
	"context"
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
	inputRequestTTL             = 15 * time.Minute
	maxInputRequestsPerTerminal = 4
	maxInputRequestsPerScope    = 32
)

type pendingInput struct {
	projection PendingInputRequest
	session    *session
	result     chan InputOutcome
	timer      *time.Timer
	resolving  bool
}

type inputRegistry struct {
	mu            sync.Mutex
	pending       map[InputRequestID]*pendingInput
	resolved      map[InputRequestID]struct{}
	resolvedOrder []InputRequestID
}

func newInputRegistry() *inputRegistry {
	return &inputRegistry{pending: make(map[InputRequestID]*pendingInput), resolved: make(map[InputRequestID]struct{})}
}

func (s *session) RequestInput(ctx context.Context, request InputRequest) (*InputOutcome, error) {
	if s.Info().Mode != ModePTY {
		return nil, &Error{Code: "terminal_not_interactive", Message: "terminal is not interactive", Err: ErrNotInteractive}
	}
	if err := s.runningGate(); err != nil {
		return nil, err
	}
	info := s.Info()
	echoProc, err := requireEchoAwareProc(s.proc)
	if err != nil {
		return nil, err
	}
	redacted := request.Redact
	echoEnabled, err := echoProc.EchoEnabled()
	if err != nil {
		return nil, err
	}
	redacted = redacted || !echoEnabled
	var pending *pendingInput
	var requester Actor
	err = s.lease.withAgentController(func(controller Actor) error {
		requester = controller
		var createErr error
		pending, createErr = s.manager.inputs.create(s, controller, request, redacted, func() (InputRequestID, error) {
			return newInputRequestID(s.manager.entropy)
		})
		return createErr
	})
	if err != nil {
		return nil, err
	}
	s.manager.events.Emit(context.WithoutCancel(ctx), TerminalEvent{
		Kind: EventKindInputRequested, WorkspaceID: info.WS, ProfileID: info.ProfileID, ProfileName: s.profileName,
		TerminalID: info.ID, Actor: requester, Reason: request.Reason,
		Detail: EventDetail{RequestID: pending.projection.ID, Redacted: redacted}, At: s.manager.now(),
	})
	outcome := <-pending.result
	return &outcome, nil
}

func (s *session) AnswerInput(ctx context.Context, actor Actor, id InputRequestID, answer InputAnswer) (*InputOutcome, error) {
	if err := s.authorizeProfile(actor); err != nil {
		return nil, err
	}
	if actor.Kind != ActorKindHuman {
		return nil, &Error{Code: "input_answer_requires_write", Message: "only a human write participant can answer an input request", Err: ErrInputRequiresWrite}
	}
	pending, err := s.manager.inputs.claim(s, id)
	if err != nil {
		return nil, err
	}
	filtered := s.filter.FilterInput(answer.Input)
	if err := s.deliverInputMode(ctx, actor, answer.Input, pending.projection.Redacted, true); err != nil {
		s.manager.inputs.release(id)
		return nil, err
	}
	outcome := InputOutcome{Outcome: "answered", Redacted: pending.projection.Redacted, Length: len(filtered)}
	s.manager.inputs.complete(id, outcome)
	s.emitInputProvided(ctx, pending, actor, "answered", len(filtered))
	return &outcome, nil
}

func (s *session) RejectInput(ctx context.Context, actor Actor, id InputRequestID, _ string) error {
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	if actor.Kind != ActorKindHuman {
		return &Error{Code: "input_answer_requires_write", Message: "only a human write participant can reject an input request", Err: ErrInputRequiresWrite}
	}
	pending, err := s.manager.inputs.claim(s, id)
	if err != nil {
		return err
	}
	if err := s.lease.answerHandoff(actor, nil, func([]byte) (int, error) { return 0, nil }); err != nil {
		s.manager.inputs.release(id)
		return err
	}
	s.manager.inputs.complete(id, InputOutcome{Outcome: "rejected", Redacted: pending.projection.Redacted})
	s.emitInputProvided(ctx, pending, actor, "rejected", 0)
	return nil
}

// PendingInput returns the server-owned redaction facts used by the answer transport.
func (s *session) PendingInput(id InputRequestID) (*PendingInputRequest, error) {
	pending, err := s.manager.inputs.inspect(s, id)
	if err != nil {
		return nil, err
	}
	projection := pending.projection
	return &projection, nil
}

func (s *session) emitInputProvided(ctx context.Context, pending *pendingInput, actor Actor, outcome string, length int) {
	s.manager.events.Emit(context.WithoutCancel(ctx), TerminalEvent{
		Kind: EventKindInputProvided, WorkspaceID: pending.projection.WorkspaceID,
		ProfileID: pending.projection.ProfileID, ProfileName: pending.projection.ProfileName,
		TerminalID: pending.projection.TerminalID, Actor: actor,
		Detail: EventDetail{RequestID: pending.projection.ID, Redacted: pending.projection.Redacted, Length: length, Outcome: outcome},
		At:     s.manager.now(),
	})
}

func (m *Service) InputRequests(_ context.Context, workspaceID string, scope store.ReadScope, terminalID ID) ([]PendingInputRequest, error) {
	if err := scope.Validate(); err != nil {
		return nil, fmt.Errorf("terminal: validate input request scope: %w", err)
	}
	return m.inputs.list(workspaceID, scope, terminalID), nil
}

func (r *inputRegistry) create(
	session *session,
	requester Actor,
	request InputRequest,
	redacted bool,
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
		return nil, &Error{Code: "input_request_limit_reached", Message: "terminal input request limit reached", Current: terminalCount, Max: maxInputRequestsPerTerminal, Err: ErrInputLimit}
	}
	if scopeCount >= maxInputRequestsPerScope {
		return nil, &Error{Code: "input_request_limit_reached", Message: "workspace input request limit reached", Current: scopeCount, Max: maxInputRequestsPerScope, Err: ErrInputLimit}
	}
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	pending := &pendingInput{
		projection: PendingInputRequest{
			ID: id, TerminalID: info.ID, WorkspaceID: info.WS, ProfileID: info.ProfileID, ProfileName: session.profileName,
			Reason: strings.TrimSpace(request.Reason), PromptExcerpt: strings.TrimSpace(request.PromptExcerpt),
			Redacted: redacted, RequestedAt: session.manager.now(),
		},
		session: session, result: make(chan InputOutcome, 1),
	}
	pending.timer = time.AfterFunc(session.manager.inputRequestTTL, func() {
		if resolved := r.resolve(id, InputOutcome{Outcome: "expired", Redacted: redacted}); resolved != nil {
			resolved.session.emitInputProvided(context.Background(), resolved, Actor{Kind: ActorKindSystem, ID: "input-request-expiry", ProfileID: info.ProfileID}, "expired", 0)
		}
	})
	r.pending[id] = pending
	return pending, nil
}

func (r *inputRegistry) claim(session *session, id InputRequestID) (*pendingInput, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := r.pending[id]
	if pending == nil || pending.session != session {
		if _, answered := r.resolved[id]; answered {
			return nil, &Error{Code: "input_request_already_answered", Message: "terminal input request is already resolved", Err: ErrInputAnswered}
		}
		return nil, &Error{Code: "input_request_not_found", Message: "terminal input request was not found", Err: ErrInputNotFound}
	}
	if pending.resolving {
		return nil, &Error{Code: "input_request_already_answered", Message: "terminal input request is already being resolved", Err: ErrInputAnswered}
	}
	pending.resolving = true
	return pending, nil
}

func (r *inputRegistry) inspect(session *session, id InputRequestID) (*pendingInput, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := r.pending[id]
	if pending == nil || pending.session != session {
		if _, answered := r.resolved[id]; answered {
			return nil, &Error{Code: "input_request_already_answered", Message: "terminal input request is already resolved", Err: ErrInputAnswered}
		}
		return nil, &Error{Code: "input_request_not_found", Message: "terminal input request was not found", Err: ErrInputNotFound}
	}
	if pending.resolving {
		return nil, &Error{Code: "input_request_already_answered", Message: "terminal input request is already being resolved", Err: ErrInputAnswered}
	}
	return pending, nil
}

func (r *inputRegistry) release(id InputRequestID) {
	r.mu.Lock()
	if pending := r.pending[id]; pending != nil {
		pending.resolving = false
	}
	r.mu.Unlock()
}

func (r *inputRegistry) complete(id InputRequestID, outcome InputOutcome) *pendingInput {
	return r.resolve(id, outcome)
}

func (r *inputRegistry) resolve(id InputRequestID, outcome InputOutcome) *pendingInput {
	r.mu.Lock()
	pending := r.pending[id]
	if pending == nil {
		r.mu.Unlock()
		return nil
	}
	delete(r.pending, id)
	r.resolved[id] = struct{}{}
	r.resolvedOrder = append(r.resolvedOrder, id)
	if len(r.resolvedOrder) > 256 {
		oldest := r.resolvedOrder[0]
		r.resolvedOrder = r.resolvedOrder[1:]
		delete(r.resolved, oldest)
	}
	r.mu.Unlock()
	if pending.timer != nil {
		pending.timer.Stop()
	}
	pending.result <- outcome
	return pending
}

func (r *inputRegistry) resolveTerminal(session *session, outcome string) []*pendingInput {
	r.mu.Lock()
	ids := make([]InputRequestID, 0)
	for id, pending := range r.pending {
		if pending.session == session {
			ids = append(ids, id)
		}
	}
	r.mu.Unlock()
	resolved := make([]*pendingInput, 0, len(ids))
	for _, id := range ids {
		r.mu.Lock()
		candidate := r.pending[id]
		redacted := candidate != nil && candidate.projection.Redacted
		r.mu.Unlock()
		pending := r.resolve(id, InputOutcome{Outcome: outcome, Redacted: redacted})
		if pending != nil {
			resolved = append(resolved, pending)
		}
	}
	return resolved
}

func (r *inputRegistry) list(workspaceID string, scope store.ReadScope, terminalID ID) []PendingInputRequest {
	r.mu.Lock()
	items := make([]PendingInputRequest, 0)
	for _, pending := range r.pending {
		item := pending.projection
		if item.WorkspaceID == workspaceID && scope.Matches(item.ProfileID) && (terminalID == "" || item.TerminalID == terminalID) {
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

func (m *Service) resolveInputRequestsOnClose(ctx context.Context, event TerminalEvent) {
	if event.Kind != EventKindClosed {
		return
	}
	key := terminalKey{workspaceID: event.WorkspaceID, profileID: event.ProfileID, id: event.TerminalID}
	m.mu.RLock()
	session := m.terminals[key]
	m.mu.RUnlock()
	if session == nil {
		return
	}
	outcome := "superseded"
	if event.Reason == "profile_archived" {
		outcome = "rejected"
	}
	for _, pending := range m.inputs.resolveTerminal(session, outcome) {
		pending.session.emitInputProvided(ctx, pending, event.Actor, outcome, 0)
	}
}

func (s *session) supersedeInputRequests(ctx context.Context, actor Actor) {
	for _, pending := range s.manager.inputs.resolveTerminal(s, "superseded") {
		s.emitInputProvided(ctx, pending, actor, "superseded", 0)
	}
}

func newInputRequestID(entropy io.Reader) (InputRequestID, error) {
	raw := make([]byte, 8)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		return "", fmt.Errorf("terminal: generate input request id: %w", err)
	}
	return InputRequestID("input-" + hex.EncodeToString(raw)), nil
}
