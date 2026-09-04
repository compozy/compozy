package terminal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/store"
)

const (
	inputRequestTTL             = 15 * time.Minute
	inputEventDeliveryTimeout   = 5 * time.Second
	maxInputRequestsPerTerminal = 4
	maxInputRequestsPerScope    = 32
)

func (s *session) RequestInput(ctx context.Context, actor Actor, request InputRequest) (*InputOutcome, error) {
	if ctx == nil {
		return nil, errors.New("terminal: input request context is required")
	}
	if err := s.authorizeProfile(actor); err != nil {
		return nil, err
	}
	if s.Info().Mode != ModePTY {
		return nil, &Error{
			Code:    ErrorCodeNotInteractive,
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
	if request.Redact && inputVisible {
		return nil, inputRequiresHiddenError(nil)
	}
	redacted = redacted || !inputVisible
	pending, err := s.manager.inputs.create(s, request, redacted, actor, func() (InputRequestID, error) {
		return newInputRequestID(s.manager.entropy)
	})
	if err != nil {
		return nil, err
	}
	s.manager.events.Notify(ctx, Event{
		Kind: EventKindInputRequested, WorkspaceID: info.WS, ProfileID: info.ProfileID, ProfileName: s.profileName,
		TerminalID: info.ID, Actor: actor, Reason: request.Reason,
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
	pending, err := s.manager.inputs.claim(s, id)
	if err != nil {
		return nil, err
	}
	characters := utf8.RuneCount(answer.Input)
	deliveryState, deliveryErr := s.deliverInputMode(
		ctx, actor, answer.Input, pending.projection.Redacted, &characters, true,
	)
	if deliveryErr != nil {
		if deliveryState.BytesDelivered == 0 && errors.Is(deliveryErr, ErrInputRequiresHidden) {
			outcome := InputOutcome{
				Outcome: InputResolutionOutcomeSuperseded, Redacted: pending.projection.Redacted,
			}
			if !s.manager.inputs.complete(pending, outcome, actor, "input_visibility_changed") {
				return nil, inputRequestResolutionLostError()
			}
			s.emitInputProvided(ctx, pending, actor, "superseded", 0, "input_visibility_changed")
			return nil, deliveryErr
		}
		if deliveryState.BytesDelivered == 0 {
			s.manager.inputs.release(pending)
			return nil, deliveryErr
		}
		outcome := inputAnswerOutcome(pending, deliveryState)
		if !s.manager.inputs.complete(pending, outcome, actor, "") {
			return nil, inputRequestResolutionLostError()
		}
		s.emitInputProvided(ctx, pending, actor, "answered", outcome.Length, "")
		return &outcome, deliveryErr
	}
	outcome := inputAnswerOutcome(pending, deliveryState)
	if !s.manager.inputs.complete(pending, outcome, actor, "") {
		return nil, inputRequestResolutionLostError()
	}
	s.emitInputProvided(ctx, pending, actor, "answered", outcome.Length, "")
	return &outcome, nil
}

func inputAnswerOutcome(
	pending *pendingInput,
	delivery inputDeliveryState,
) InputOutcome {
	bytesDelivered := len(delivery.Content)
	characters := utf8.RuneCount(delivery.Content)
	if pending.projection.Redacted {
		characters = delivery.CharactersDelivered
	}
	return InputOutcome{
		Outcome: InputResolutionOutcomeAnswered, Redacted: pending.projection.Redacted,
		Length: characters, DeliveredBytes: bytesDelivered,
	}
}

func (s *session) RejectInput(ctx context.Context, actor Actor, id InputRequestID, reason string) error {
	if err := requestContextError(ctx, "reject input"); err != nil {
		return err
	}
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	pending, err := s.manager.inputs.claim(s, id)
	if err != nil {
		return err
	}
	trimmedReason := strings.TrimSpace(reason)
	if !s.manager.inputs.complete(
		pending,
		InputOutcome{Outcome: InputResolutionOutcomeRejected, Redacted: pending.projection.Redacted},
		actor,
		trimmedReason,
	) {
		return inputRequestResolutionLostError()
	}
	s.emitInputProvided(ctx, pending, actor, "rejected", 0, trimmedReason)
	return nil
}

func inputRequestResolutionLostError() error {
	return fmt.Errorf("terminal input request resolution lost: %w", ErrInputResolved)
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
	outcome InputResolutionOutcome,
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
			Outcome:   string(outcome),
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

func (m *Service) ResolvedInputRequests(
	ctx context.Context,
	workspaceID string,
	scope store.ReadScope,
	terminalID ID,
) ([]ResolvedInputRequest, error) {
	if err := requestContextError(ctx, "list resolved input requests"); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, fmt.Errorf("terminal: validate resolved input request scope: %w", err)
	}
	return m.inputs.listResolved(workspaceID, scope, terminalID), nil
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
	outcome := InputResolutionOutcomeSuperseded
	if event.Reason == "profile_archived" {
		outcome = InputResolutionOutcomeRejected
	}
	m.inputs.resolveTerminal(session, inputTerminalResolution{
		outcome: outcome, actor: event.Actor, reason: event.Reason,
	})
}

func (s *session) supersedeInputRequests(_ context.Context, actor Actor) {
	s.manager.inputs.resolveTerminal(s, inputTerminalResolution{
		outcome: InputResolutionOutcomeSuperseded, actor: actor,
	})
}
