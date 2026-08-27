package terminal

import (
	"context"
	"errors"
	"fmt"
)

const (
	errorCodeApprovalRequired         = "approval_required"
	errorCodeExpired                  = "terminal_expired"
	errorCodeExited                   = "terminal_exited"
	errorCodeGenerationFenced         = "generation_fenced"
	errorCodeInputAlreadyAnswered     = "input_request_already_answered"
	errorCodeInputAnswerRequiresWrite = "input_answer_requires_write"
	errorCodeInputPending             = "input_request_pending"
	errorCodeInvalidCwd               = "invalid_cwd"
	errorCodeNotInteractive           = "terminal_not_interactive"
	errorCodeNotFound                 = "terminal_not_found"
	errorCodeRecordingUnavailable     = "recording_unavailable"
	errorCodeShuttingDown             = "terminal_shutting_down"
	errorCodeWriteOwnerHeld           = "write_owner_held"
	errorMessageExpired               = "terminal has expired"
	errorMessageExited                = "terminal has exited"
	errorMessageGenerationFenced      = "terminal action came from a stale runtime generation"
	errorMessageInputAlreadyResolved  = "terminal input request is already resolved"
	errorMessageInputPending          = "agent terminal mutation is blocked while an input request is pending"
	errorMessageNotFound              = "terminal not found"
	errorMessageNotInteractive        = "terminal is not interactive"
	errorMessageShuttingDown          = "terminal manager is shutting down"
)

var (
	ErrNotFound           = errors.New("terminal not found")
	ErrRequiresWorkspace  = errors.New("terminal requires workspace")
	ErrGenerationFenced   = errors.New("terminal generation fenced")
	ErrLeaseRevoked       = errors.New("terminal lease revoked")
	ErrWriteOwnerHeld     = errors.New("terminal write owner held")
	ErrNotInteractive     = errors.New("terminal not interactive")
	ErrApprovalRequired   = errors.New("terminal approval required")
	ErrTypingGrant        = errors.New("terminal typing grant rejected")
	ErrInputNotFound      = errors.New("terminal input request not found")
	ErrInputAnswered      = errors.New("terminal input request already answered")
	ErrInputLimit         = errors.New("terminal input request limit reached")
	ErrInputRequiresWrite = errors.New("terminal input answer requires write")
	ErrInputPending       = errors.New("terminal input request pending")
	ErrWaitTimeout        = errors.New("terminal wait timeout")
	ErrExited             = errors.New("terminal exited")
	ErrExpired            = errors.New("terminal expired")
	ErrLimitReached       = errors.New("terminal limit reached")
	ErrSubscriberLimit    = errors.New("terminal subscriber limit reached")
	ErrInvalidCwd         = errors.New("terminal invalid cwd")
	ErrInteractive        = errors.New("terminal interactive unavailable")
	ErrApprovalRejected   = errors.New("terminal approval rejected")
	ErrRecording          = errors.New("terminal recording unavailable")
	ErrJournalUnavailable = errors.New("terminal journal unavailable")
	ErrShuttingDown       = errors.New("terminal manager shutting down")
	ErrUnsupported        = errors.New("terminal operation unsupported")
)

type Error struct {
	Code       string
	Message    string
	Controller *Actor
	Current    int
	Max        int
	Err        error
}

func (e *Error) Error() string {
	if e == nil {
		return "terminal error"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("terminal: %s", e.Code)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func requestContextError(ctx context.Context, operation string) error {
	if ctx == nil {
		return fmt.Errorf("terminal: %s context is required", operation)
	}
	if err := ctx.Err(); err != nil {
		return context.Cause(ctx)
	}
	return nil
}
