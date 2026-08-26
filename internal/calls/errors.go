package calls

import (
	"errors"
	"fmt"
	"strings"
)

// ErrPublicationNotFound reports an absent per-conversation idempotency row.
var ErrPublicationNotFound = errors.New("calls: publication not found")

// ErrorCode is the stable machine-readable calls failure vocabulary.
type ErrorCode string

const (
	// CodeValidation and the following constants are stable public call error codes.
	CodeValidation             ErrorCode = "call_validation"
	CodeAgentUnknown           ErrorCode = "call_agent_unknown"
	CodeExpectInvalid          ErrorCode = "call_expect_invalid"
	CodePromptRequired         ErrorCode = "call_prompt_empty"
	CodeChildrenCap            ErrorCode = "call_children_cap"
	CodeWideningRejected       ErrorCode = "call_widening_rejected"
	CodeNotFound               ErrorCode = "call_target_not_found"
	CodeTargetExpired          ErrorCode = "call_target_expired"
	CodeTargetDenied           ErrorCode = "call_target_denied"
	CodeWorkspaceDenied        ErrorCode = "call_workspace_denied"
	CodeParentTerminal         ErrorCode = "call_parent_terminal"
	CodeDepthExceeded          ErrorCode = "call_depth_exceeded"
	CodeBatchEmpty             ErrorCode = "call_batch_empty"
	CodeBatchOverCap           ErrorCode = "call_batch_over_cap"
	CodeIdempotencyConflict    ErrorCode = "call_idempotency_conflict"
	CodeNotSettled             ErrorCode = "call_not_settled"
	CodeAlreadySettled         ErrorCode = "call_already_settled"
	CodeReturnUnbound          ErrorCode = "call_return_unbound"
	CodeResultInvalid          ErrorCode = "call_result_invalid"
	CodeResultOverBudget       ErrorCode = "call_result_over_budget"
	CodeDeadlineInvalid        ErrorCode = "call_deadline_invalid"
	CodeSettlementDenied       ErrorCode = "call_settlement_denied"
	CodeMessageTooLarge        ErrorCode = "message_too_large"
	CodeMessageTargetBlocked   ErrorCode = "message_target_blocked"
	CodeMessageTargetDenied    ErrorCode = "message_target_denied"
	CodeMessageRateLimited     ErrorCode = "message_rate_limited"
	CodeMessageDuplicate       ErrorCode = "message_duplicate"
	CodeMessagePendingCap      ErrorCode = "message_pending_cap"
	CodeMessageNotFound        ErrorCode = "message_not_found"
	CodePublishNoParticipation ErrorCode = "call_publish_no_participation"
	CodePublishNotSettled      ErrorCode = "call_publish_not_settled"
)

// Error carries a stable code plus safe diagnostic detail.
type Error struct {
	Code       ErrorCode          `json:"code"`
	Message    string             `json:"message"`
	Available  []AgentRosterEntry `json:"available,omitempty"`
	Widening   []string           `json:"widening,omitempty"`
	OriginalID string             `json:"original_id,omitempty"`
	ResetAt    string             `json:"reset_at,omitempty"`
	ExpiredAt  string             `json:"expired_at,omitempty"`
	Suggestion string             `json:"suggestion,omitempty"`
	Cause      error              `json:"-"`
}

// Error renders the stable code and safe message.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, message)
}

// Unwrap exposes the preserved internal cause.
func (e *Error) Unwrap() error { return e.Cause }

// DiagnosticCode exposes the stable calls code to shared transport error rendering.
func (e *Error) DiagnosticCode() string {
	if e == nil {
		return ""
	}
	return string(e.Code)
}

// IsCode reports whether an error carries the requested public code.
func IsCode(err error, code ErrorCode) bool {
	var callErr *Error
	return errors.As(err, &callErr) && callErr.Code == code
}

func newError(code ErrorCode, message string, cause error) error {
	return &Error{Code: code, Message: strings.TrimSpace(message), Cause: cause}
}
