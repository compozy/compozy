package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
)

type PartialWriteError struct {
	Delivered int
	Err       error
}

func (e *PartialWriteError) Error() string {
	return fmt.Sprintf("terminal input stopped after %d bytes: %v", e.Delivered, e.Err)
}

func (e *PartialWriteError) Unwrap() error { return e.Err }

func (s *session) Write(ctx context.Context, actor Actor, input []byte) error {
	_, err := s.deliverInputMode(ctx, actor, input, false, nil, false)
	return err
}

type inputDeliveryState struct {
	BytesDelivered      int
	CharactersDelivered int
	Complete            bool
	Content             []byte
}

type inputVisibilityProc interface {
	InputVisible() (bool, error)
	WriteRedacted([]byte) (terminalpty.RedactedWriteResult, error)
}

func requireInputVisibilityProc(proc Proc) (inputVisibilityProc, error) {
	visibilityProc, ok := proc.(inputVisibilityProc)
	if !ok {
		return nil, fmt.Errorf("terminal input requires an echo-aware PTY: %w", ErrInteractive)
	}
	return visibilityProc, nil
}

func (s *session) deliverInputMode(
	ctx context.Context,
	actor Actor,
	input []byte,
	clientRedact bool,
	redactedCharacters *int,
	appendNewline bool,
) (inputDeliveryState, error) {
	if err := requestContextError(ctx, "write"); err != nil {
		return inputDeliveryState{}, err
	}
	if err := s.authorizeProfile(actor); err != nil {
		return inputDeliveryState{}, err
	}
	if s.Info().Mode != ModePTY {
		return inputDeliveryState{}, &Error{
			Code: ErrorCodeNotInteractive, Message: errorMessageNotInteractive, Err: ErrNotInteractive,
		}
	}
	if s.audit.Blocked() {
		return inputDeliveryState{}, &Error{
			Code:    ErrorCodeJournalUnavailable,
			Message: "terminal input is blocked while journal delivery is unavailable",
			Err:     ErrJournalUnavailable,
		}
	}
	if err := s.runningGate(); err != nil {
		return inputDeliveryState{}, err
	}
	// A submission stays intact across filtering, audit reservation, PTY delivery,
	// and journal commit. Separate actors may submit concurrently; arrival order
	// decides which complete submission reaches the process first.
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	filtered := s.filter.FilterInput(input)
	contentBytes := len(filtered)
	if appendNewline &&
		(len(filtered) == 0 || (filtered[len(filtered)-1] != '\n' && filtered[len(filtered)-1] != '\r')) {
		filtered = append(filtered, '\n')
	}
	info := s.Info()
	writer, redacted, err := s.inputWriter(clientRedact)
	if err != nil {
		return inputDeliveryState{}, err
	}
	auditInput := JournalInput{Content: filtered}
	if redacted {
		characters := utf8.RuneCount(filtered)
		if redactedCharacters != nil {
			characters = max(*redactedCharacters, 0)
		}
		auditInput = JournalInput{Redacted: true, Characters: characters}
	}
	reservation, admitted := s.manager.reserveJournalInput(info, auditInput)
	if !admitted {
		return inputDeliveryState{}, &Error{
			Code:    ErrorCodeJournalUnavailable,
			Message: "terminal input is blocked while the journal lane is full",
			Err:     ErrJournalUnavailable,
		}
	}
	deliveryErr := writeAllInput(filtered, writer)
	state, err := s.commitInputDelivery(actor, filtered, contentBytes, auditInput, reservation, deliveryErr)
	if err != nil {
		return state, err
	}
	return state, deliveryErr
}

func (s *session) commitInputDelivery(
	actor Actor,
	filtered []byte,
	contentBytes int,
	auditInput JournalInput,
	reservation JournalInputReservation,
	deliveryErr error,
) (inputDeliveryState, error) {
	delivered := len(filtered)
	if deliveryErr != nil {
		var partial *PartialWriteError
		if !errors.As(deliveryErr, &partial) || partial.Delivered <= 0 {
			reservation.Release()
			return inputDeliveryState{}, deliveryErr
		}
		delivered = min(partial.Delivered, len(filtered))
	}
	deliveredInput := auditInput
	characters := 0
	if auditInput.Redacted {
		characters = deliveredRedactedCharacters(filtered, delivered, auditInput.Characters)
		deliveredInput.Characters = characters
	} else {
		deliveredInput.Content = append([]byte(nil), filtered[:delivered]...)
	}
	reservation.Commit(actor, deliveredInput)
	if auditInput.Redacted {
		s.acceptRedactedInput(characters)
	}
	state := inputDeliveryState{
		BytesDelivered: delivered, CharactersDelivered: characters, Complete: deliveryErr == nil,
		Content: append([]byte(nil), filtered[:min(delivered, contentBytes)]...),
	}
	return state, nil
}

func deliveredRedactedCharacters(input []byte, delivered, fullCharacters int) int {
	if delivered >= len(input) {
		return max(fullCharacters, 0)
	}
	prefix := input[:max(min(delivered, len(input)), 0)]
	for len(prefix) > 0 && !utf8.Valid(prefix) {
		prefix = prefix[:len(prefix)-1]
	}
	return min(utf8.RuneCount(prefix), max(fullCharacters, 0))
}

func writeAllInput(input []byte, write func([]byte) (int, error)) error {
	delivered := 0
	for delivered < len(input) {
		written, err := write(input[delivered:])
		delivered += written
		if err != nil {
			return &PartialWriteError{Delivered: delivered, Err: fmt.Errorf("terminal: write input: %w", err)}
		}
		if written == 0 {
			return &PartialWriteError{Delivered: delivered, Err: io.ErrNoProgress}
		}
	}
	return nil
}

func (s *session) inputWriter(
	clientRedact bool,
) (func([]byte) (int, error), bool, error) {
	redacted := clientRedact
	writer := s.proc.Write
	if clientRedact {
		visibilityProc, err := requireInputVisibilityProc(s.proc)
		if err != nil {
			return nil, false, err
		}
		writer = redactedInputWriter(visibilityProc)
	}
	if visibilityProc, ok := s.proc.(inputVisibilityProc); ok {
		inputVisible, err := visibilityProc.InputVisible()
		if err != nil {
			return nil, false, err
		}
		if clientRedact && inputVisible {
			return nil, false, inputRequiresHiddenError(nil)
		}
		redacted = redacted || !inputVisible
		if redacted && !clientRedact {
			writer = redactedInputWriter(visibilityProc)
		}
	}
	return writer, redacted, nil
}

func redactedInputWriter(proc inputVisibilityProc) func([]byte) (int, error) {
	return func(input []byte) (int, error) {
		result, err := proc.WriteRedacted(input)
		if errors.Is(err, terminalpty.ErrInputVisible) {
			return 0, inputRequiresHiddenError(err)
		}
		return result.BytesDelivered, err
	}
}
