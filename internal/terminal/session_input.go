package terminal

import "context"

func (s *session) Write(ctx context.Context, actor Actor, input []byte) error {
	return s.deliverInputMode(ctx, actor, input, false)
}

type inputVisibilityProc interface {
	InputVisible() (bool, error)
	WriteRedacted([]byte) (int, error)
}

func requireInputVisibilityProc(proc Proc) (inputVisibilityProc, error) {
	visibilityProc, ok := proc.(inputVisibilityProc)
	if !ok {
		return nil, &Error{
			Code:    errorCodeNotInteractive,
			Message: "terminal input requests require an echo-aware interactive terminal",
			Err:     ErrNotInteractive,
		}
	}
	return visibilityProc, nil
}

func (s *session) deliverInputMode(
	ctx context.Context,
	actor Actor,
	input []byte,
	clientRedact bool,
) error {
	if err := requestContextError(ctx, "write"); err != nil {
		return err
	}
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	if s.Info().Mode != ModePTY {
		return &Error{Code: errorCodeNotInteractive, Message: errorMessageNotInteractive, Err: ErrNotInteractive}
	}
	if s.audit.Blocked() {
		return &Error{
			Code:    "journal_unavailable",
			Message: "terminal input is blocked while journal delivery is unavailable",
			Err:     ErrJournalUnavailable,
		}
	}
	if err := s.runningGate(); err != nil {
		return err
	}
	if err := s.authorizePendingInputMutation(actor); err != nil {
		return err
	}
	if err := s.authorizeAgentInput(ctx, actor); err != nil {
		return err
	}
	filtered := s.filter.FilterInput(input)
	info := s.Info()
	writer, redacted, err := s.inputWriter(clientRedact)
	if err != nil {
		return err
	}
	auditInput := filtered
	if redacted {
		auditInput = nil
	}
	reservation, admitted := s.manager.reserveJournalInput(info, auditInput)
	if !admitted {
		return &Error{
			Code:    "journal_unavailable",
			Message: "terminal input is blocked while the journal lane is full",
			Err:     ErrJournalUnavailable,
		}
	}
	if err := s.lease.deliverWith(actor, filtered, writer); err != nil {
		s.manager.releaseJournalInput(info, reservation)
		return err
	}
	s.manager.commitJournalInput(info, actor, auditInput, reservation)
	return nil
}

func (s *session) authorizePendingInputMutation(actor Actor) error {
	if actor.Kind != ActorKindAgent || !s.manager.inputs.hasPending(s) {
		return nil
	}
	return &Error{
		Code: errorCodeInputPending, Message: errorMessageInputPending, Err: ErrInputPending,
	}
}

func (s *session) authorizeAgentInput(ctx context.Context, actor Actor) error {
	if actor.Kind != ActorKindAgent {
		return nil
	}
	if s.manager.typingGrants == nil {
		return &Error{
			Code:    "typing_grant_rejected",
			Message: "agent typing requires a one-time terminal grant",
			Err:     ErrTypingGrant,
		}
	}
	return s.manager.typingGrants.AuthorizeTerminalInput(ctx, actor, s.Info())
}

func (s *session) inputWriter(clientRedact bool) (func([]byte) (int, error), bool, error) {
	redacted := clientRedact
	writer := s.proc.Write
	if clientRedact {
		visibilityProc, err := requireInputVisibilityProc(s.proc)
		if err != nil {
			return nil, false, err
		}
		writer = visibilityProc.WriteRedacted
	}
	if visibilityProc, ok := s.proc.(inputVisibilityProc); ok {
		inputVisible, err := visibilityProc.InputVisible()
		if err != nil {
			return nil, false, err
		}
		redacted = redacted || !inputVisible
		if redacted && !clientRedact {
			writer = visibilityProc.WriteRedacted
		}
	}
	return writer, redacted, nil
}
