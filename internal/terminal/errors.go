package terminal

import (
	"context"
	"errors"
	"fmt"
)

const (
	errorMessageExpired              = "terminal has expired"
	errorMessageExited               = "terminal has exited"
	errorMessageGenerationFenced     = "terminal action came from a stale runtime generation"
	errorMessageInputAlreadyResolved = "terminal input request is already resolved"
	errorMessageInputPending         = "agent terminal mutation is blocked while an input request is pending"
	errorMessageNotFound             = "terminal not found"
	errorMessageNotInteractive       = "terminal is not interactive"
	errorMessageShuttingDown         = "terminal manager is shutting down"
)

var (
	ErrNotFound                = errors.New("terminal not found")
	ErrRequiresWorkspace       = errors.New("terminal requires workspace")
	ErrGenerationFenced        = errors.New("terminal generation fenced")
	ErrLeaseRevoked            = errors.New("terminal lease revoked")
	ErrWriteOwnerHeld          = errors.New("terminal write owner held")
	ErrWriteAttachmentRequired = errors.New("terminal write attachment required")
	ErrWriteLeaseRequired      = errors.New("terminal write lease required")
	ErrNotInteractive          = errors.New("terminal not interactive")
	ErrApprovalRequired        = errors.New("terminal approval required")
	ErrPolicyDenied            = errors.New("terminal command denied by policy")
	ErrTypingGrant             = errors.New("terminal typing grant rejected")
	ErrInputNotFound           = errors.New("terminal input request not found")
	ErrInputAnswered           = errors.New("terminal input request already answered")
	ErrInputSuperseded         = errors.New("terminal input request superseded")
	ErrInputResolved           = errors.New("terminal input request resolved without an answer")
	ErrInputResolving          = errors.New("terminal input request resolution in progress")
	ErrInputLimit              = errors.New("terminal input request limit reached")
	ErrInputRequiresWrite      = errors.New("terminal input answer requires write")
	ErrInputPending            = errors.New("terminal input request pending")
	ErrWaitTimeout             = errors.New("terminal wait timeout")
	ErrExited                  = errors.New("terminal exited")
	ErrExpired                 = errors.New("terminal expired")
	ErrLimitReached            = errors.New("terminal limit reached")
	ErrSubscriberLimit         = errors.New("terminal subscriber limit reached")
	ErrSlowConsumer            = errors.New("terminal subscriber is a slow consumer")
	ErrInvalidCwd              = errors.New("terminal invalid cwd")
	ErrInteractive             = errors.New("terminal interactive unavailable")
	ErrApprovalRejected        = errors.New("terminal approval rejected")
	ErrRecording               = errors.New("terminal recording unavailable")
	ErrJournalUnavailable      = errors.New("terminal journal unavailable")
	ErrShuttingDown            = errors.New("terminal manager shutting down")
	ErrUnsupported             = errors.New("terminal operation unsupported")
	ErrTicketInvalid           = errors.New("terminal attach ticket invalid")
	ErrTicketExpired           = errors.New("terminal attach ticket expired")
	ErrRunIdentityIncomplete   = errors.New("terminal agent run identity incomplete")
	ErrServiceUnavailable      = errors.New("terminal service unavailable")
)

// ErrorCode is the closed public terminal failure vocabulary.
type ErrorCode string

const (
	ErrorCodeNotFound                 ErrorCode = "terminal_not_found"
	ErrorCodeProfileSelectionConflict ErrorCode = "profile_selection_conflict"
	ErrorCodeProfileSessionConflict   ErrorCode = "profile_session_conflict"
	ErrorCodeRequiresWorkspace        ErrorCode = "terminal_requires_workspace"
	ErrorCodeProfileArchived          ErrorCode = "profile_archived"
	ErrorCodeProfileUnavailable       ErrorCode = "profile_unavailable"
	ErrorCodeLimitReached             ErrorCode = "terminal_limit_reached"
	ErrorCodeSubscriberLimitReached   ErrorCode = "subscriber_limit_reached"
	ErrorCodeExited                   ErrorCode = "terminal_exited"
	ErrorCodeExpired                  ErrorCode = "terminal_expired"
	ErrorCodeInteractiveUnavailable   ErrorCode = "terminal_interactive_unavailable"
	ErrorCodeNotInteractive           ErrorCode = "terminal_not_interactive"
	ErrorCodeInvalidCwd               ErrorCode = "invalid_cwd"
	ErrorCodeTimeoutOutOfRange        ErrorCode = "timeout_out_of_range"
	ErrorCodeWriteOwnerHeld           ErrorCode = "write_owner_held"
	ErrorCodeLeaseRevoked             ErrorCode = "lease_revoked"
	ErrorCodeGenerationFenced         ErrorCode = "generation_fenced"
	ErrorCodeTypingGrantRejected      ErrorCode = "typing_grant_rejected"
	ErrorCodeApprovalRejected         ErrorCode = "approval_rejected"
	ErrorCodeTicketInvalid            ErrorCode = "ticket_invalid"
	ErrorCodeTicketExpired            ErrorCode = "ticket_expired"
	ErrorCodeInputRequestNotFound     ErrorCode = "input_request_not_found"
	ErrorCodeInputRequestAnswered     ErrorCode = "input_request_already_answered"
	ErrorCodeInputRequestSuperseded   ErrorCode = "input_request_superseded"
	ErrorCodeInputRequestLimitReached ErrorCode = "input_request_limit_reached"
	ErrorCodeInputAnswerRequiresWrite ErrorCode = "input_answer_requires_write"
	ErrorCodeRecordingAlreadyStarted  ErrorCode = "recording_already_started"
	ErrorCodeRecordingNotActive       ErrorCode = "recording_not_active"
	ErrorCodeRecordingUnavailable     ErrorCode = "recording_unavailable"
	ErrorCodeSlowConsumer             ErrorCode = "slow_consumer"
	ErrorCodeJournalUnavailable       ErrorCode = "journal_unavailable"
)

var errorCodeValues = [...]ErrorCode{
	ErrorCodeNotFound, ErrorCodeProfileSelectionConflict, ErrorCodeProfileSessionConflict,
	ErrorCodeRequiresWorkspace, ErrorCodeProfileArchived, ErrorCodeProfileUnavailable,
	ErrorCodeLimitReached, ErrorCodeSubscriberLimitReached, ErrorCodeExited, ErrorCodeExpired,
	ErrorCodeInteractiveUnavailable, ErrorCodeNotInteractive, ErrorCodeInvalidCwd,
	ErrorCodeTimeoutOutOfRange, ErrorCodeWriteOwnerHeld, ErrorCodeLeaseRevoked,
	ErrorCodeGenerationFenced, ErrorCodeTypingGrantRejected, ErrorCodeApprovalRejected,
	ErrorCodeTicketInvalid, ErrorCodeTicketExpired, ErrorCodeInputRequestNotFound,
	ErrorCodeInputRequestAnswered, ErrorCodeInputRequestSuperseded, ErrorCodeInputRequestLimitReached,
	ErrorCodeInputAnswerRequiresWrite, ErrorCodeRecordingAlreadyStarted, ErrorCodeRecordingNotActive,
	ErrorCodeRecordingUnavailable, ErrorCodeSlowConsumer, ErrorCodeJournalUnavailable,
}

func ErrorCodeValues() []string {
	values := make([]string, len(errorCodeValues))
	for index, code := range errorCodeValues {
		values[index] = string(code)
	}
	return values
}

func IsErrorCode(code ErrorCode) bool {
	for _, candidate := range errorCodeValues {
		if code == candidate {
			return true
		}
	}
	return false
}

type Error struct {
	Code       ErrorCode
	Message    string
	Controller *Actor
	Current    int
	Max        int
	Path       string
	Mode       Mode
	Platform   string
	Action     string
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

func newTerminalError(code ErrorCode, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
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
