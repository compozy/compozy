package terminal

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const (
	inputRequestTTL             = 15 * time.Minute
	inputEventDeliveryTimeout   = 5 * time.Second
	maxInputRequestsPerTerminal = 4
	maxInputRequestsPerScope    = 32
)

func (s *session) RequestInput(ctx context.Context, request InputRequest) (*InputOutcome, error) {
	if ctx == nil {
		return nil, errors.New("terminal: input request context is required")
	}
	if s.Info().Mode != ModePTY {
		return nil, &Error{
			Code:    errorCodeNotInteractive,
			Message: errorMessageNotInteractive,
			Err:     ErrNotInteractive,
		}
	}
	if err := s.runningGate(); err != nil {
		return nil, err
	}
	info := s.Info()
	visibilityProc, err := requireInputVisibilityProc(s.proc)
	if err != nil {
		return nil, err
	}
	redacted := request.Redact
	inputVisible, err := visibilityProc.InputVisible()
	if err != nil {
		return nil, err
	}
	redacted = redacted || !inputVisible
	var pending *pendingInput
	var requester Actor
	err = s.lease.withAgentController(func(controller Actor) error {
		requester = controller
		var createErr error
		pending, createErr = s.manager.inputs.create(s, request, redacted, func() (InputRequestID, error) {
			return newInputRequestID(s.manager.entropy)
		})
		return createErr
	})
	if err != nil {
		return nil, err
	}
	s.manager.events.Notify(ctx, Event{
		Kind: EventKindInputRequested, WorkspaceID: info.WS, ProfileID: info.ProfileID, ProfileName: s.profileName,
		TerminalID: info.ID, Actor: requester, Reason: request.Reason,
		Detail: &EventDetail{RequestID: pending.projection.ID, Redacted: redacted}, At: s.manager.now(),
	})
	select {
	case outcome := <-pending.result:
		return &outcome, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("terminal: input request canceled: %w", context.Cause(ctx))
	}
}

func (s *session) AnswerInput(
	ctx context.Context,
	actor Actor,
	id InputRequestID,
	answer InputAnswer,
) (*InputOutcome, error) {
	if err := requestContextError(ctx, "answer input"); err != nil {
		return nil, err
	}
	if err := s.authorizeProfile(actor); err != nil {
		return nil, err
	}
	if actor.Kind != ActorKindHuman {
		return nil, &Error{
			Code:    errorCodeInputAnswerRequiresWrite,
			Message: "only a human write participant can answer an input request",
			Err:     ErrInputRequiresWrite,
		}
	}
	if err := s.lease.authorize(actor); err != nil {
		return nil, err
	}
	pending, err := s.manager.inputs.claim(s, id)
	if err != nil {
		return nil, err
	}
	filtered := s.filter.FilterInput(answer.Input)
	delivery := slices.Clone(answer.Input)
	if len(delivery) == 0 || (delivery[len(delivery)-1] != '\n' && delivery[len(delivery)-1] != '\r') {
		delivery = append(delivery, '\n')
	}
	if err := s.deliverInputMode(ctx, actor, delivery, pending.projection.Redacted); err != nil {
		s.manager.inputs.release(pending)
		return nil, err
	}
	outcome := InputOutcome{Outcome: "answered", Redacted: pending.projection.Redacted, Length: len(filtered)}
	if !s.manager.inputs.complete(pending, outcome) {
		return nil, inputRequestAlreadyResolvedError()
	}
	s.emitInputProvided(ctx, pending, actor, "answered", len(filtered), "")
	return &outcome, nil
}

func (s *session) RejectInput(ctx context.Context, actor Actor, id InputRequestID, reason string) error {
	if err := requestContextError(ctx, "reject input"); err != nil {
		return err
	}
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	if actor.Kind != ActorKindHuman {
		return &Error{
			Code:    "input_answer_requires_write",
			Message: "only a human write participant can reject an input request",
			Err:     ErrInputRequiresWrite,
		}
	}
	if err := s.lease.authorize(actor); err != nil {
		return err
	}
	pending, err := s.manager.inputs.claim(s, id)
	if err != nil {
		return err
	}
	if !s.manager.inputs.complete(
		pending,
		InputOutcome{Outcome: "rejected", Redacted: pending.projection.Redacted},
	) {
		return inputRequestAlreadyResolvedError()
	}
	s.emitInputProvided(ctx, pending, actor, "rejected", 0, strings.TrimSpace(reason))
	return nil
}

func inputRequestAlreadyResolvedError() error {
	return &Error{
		Code:    errorCodeInputAlreadyAnswered,
		Message: errorMessageInputAlreadyResolved,
		Err:     ErrInputAnswered,
	}
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

func (s *session) emitInputProvided(
	ctx context.Context,
	pending *pendingInput,
	actor Actor,
	outcome string,
	length int,
	reason string,
) {
	s.manager.events.Notify(ctx, Event{
		Kind:        EventKindInputProvided,
		WorkspaceID: pending.projection.WorkspaceID,
		ProfileID:   pending.projection.ProfileID,
		ProfileName: pending.projection.ProfileName,
		TerminalID:  pending.projection.TerminalID,
		Actor:       actor,
		Reason:      reason,
		Detail: &EventDetail{
			RequestID: pending.projection.ID,
			Redacted:  pending.projection.Redacted,
			Length:    length,
			Outcome:   outcome,
		},
		At: s.manager.now(),
	})
}

func (m *Service) InputRequests(
	ctx context.Context,
	workspaceID string,
	scope store.ReadScope,
	terminalID ID,
) ([]PendingInputRequest, error) {
	if err := requestContextError(ctx, "list input requests"); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, fmt.Errorf("terminal: validate input request scope: %w", err)
	}
	return m.inputs.list(workspaceID, scope, terminalID), nil
}

func (m *Service) resolveInputRequestsOnClose(_ context.Context, event Event) {
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
	m.inputs.resolveTerminal(session, inputTerminalResolution{
		outcome: outcome, actor: event.Actor, reason: event.Reason,
	})
}

func (s *session) supersedeInputRequests(_ context.Context, actor Actor) {
	s.manager.inputs.resolveTerminal(s, inputTerminalResolution{outcome: "superseded", actor: actor})
}
