package contract

import (
	"encoding/json"
	"strconv"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

// TerminalErrorCode is the closed public vocabulary for terminal domain outcomes.
type TerminalErrorCode string

const (
	TerminalErrorNotFound                 TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeNotFound)
	TerminalErrorProfileSelectionConflict TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeProfileSelectionConflict,
	)
	TerminalErrorProfileSessionConflict TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeProfileSessionConflict,
	)
	TerminalErrorRequiresWorkspace      TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeRequiresWorkspace)
	TerminalErrorProfileArchived        TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeProfileArchived)
	TerminalErrorProfileUnavailable     TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeProfileUnavailable)
	TerminalErrorLimitReached           TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeLimitReached)
	TerminalErrorSubscriberLimitReached TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeSubscriberLimitReached,
	)
	TerminalErrorExited                 TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeExited)
	TerminalErrorExpired                TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeExpired)
	TerminalErrorInteractiveUnavailable TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeInteractiveUnavailable,
	)
	TerminalErrorNotInteractive      TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeNotInteractive)
	TerminalErrorInvalidCwd          TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeInvalidCwd)
	TerminalErrorTimeoutOutOfRange   TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeTimeoutOutOfRange)
	TerminalErrorWriteOwnerHeld      TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeWriteOwnerHeld)
	TerminalErrorLeaseRevoked        TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeLeaseRevoked)
	TerminalErrorGenerationFenced    TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeGenerationFenced)
	TerminalErrorTypingGrantRejected TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeTypingGrantRejected,
	)
	TerminalErrorApprovalRejected     TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeApprovalRejected)
	TerminalErrorTicketInvalid        TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeTicketInvalid)
	TerminalErrorTicketExpired        TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeTicketExpired)
	TerminalErrorInputRequestNotFound TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeInputRequestNotFound,
	)
	TerminalErrorInputRequestAnswered TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeInputRequestAnswered,
	)
	TerminalErrorInputRequestSuperseded TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeInputRequestSuperseded,
	)
	TerminalErrorInputRequestLimitReached TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeInputRequestLimitReached,
	)
	TerminalErrorInputAnswerRequiresWrite TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeInputAnswerRequiresWrite,
	)
	TerminalErrorRecordingAlreadyStarted TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeRecordingAlreadyStarted,
	)
	TerminalErrorRecordingNotActive   TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeRecordingNotActive)
	TerminalErrorRecordingUnavailable TerminalErrorCode = TerminalErrorCode(
		terminalpkg.ErrorCodeRecordingUnavailable,
	)
	TerminalErrorSlowConsumer       TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeSlowConsumer)
	TerminalErrorJournalUnavailable TerminalErrorCode = TerminalErrorCode(terminalpkg.ErrorCodeJournalUnavailable)
)

// TerminalErrorCodeValues returns the frozen terminal error vocabulary in contract order.
func TerminalErrorCodeValues() []string {
	return terminalpkg.ErrorCodeValues()
}

// IsTerminalErrorCode reports whether code belongs to the frozen public vocabulary.
func IsTerminalErrorCode(code TerminalErrorCode) bool {
	return terminalpkg.IsErrorCode(terminalpkg.ErrorCode(code))
}

// TerminalErrorDetail is the shared terminal failure envelope. Code is a
// TerminalErrorCode for domain outcomes and a truthful generic string for
// transport failures.
type TerminalErrorDetail struct {
	Code    string                `json:"code"`
	Message string                `json:"message"`
	Details *TerminalErrorDetails `json:"details,omitempty"`
}

// TerminalErrorController identifies the actor holding terminal control.
type TerminalErrorController struct {
	Kind TerminalActorKind `json:"kind"`
	ID   string            `json:"id"`
}

// TerminalErrorDetails preserves actionable domain metadata without parsing messages.
type TerminalErrorDetails struct {
	Current    *int                     `json:"current,omitempty"`
	Max        *int                     `json:"max,omitempty"`
	Controller *TerminalErrorController `json:"controller,omitempty"`
	Path       string                   `json:"path,omitempty"`
	Mode       TerminalMode             `json:"mode,omitempty"`
	Platform   string                   `json:"platform,omitempty"`
	Action     string                   `json:"action,omitempty"`
}

// TerminalErrorResponse wraps every public terminal failure in one envelope.
type TerminalErrorResponse struct {
	Error TerminalErrorDetail `json:"error"`
}

// TerminalErrorDetailsFromDomain projects only public, non-secret error metadata.
func TerminalErrorDetailsFromDomain(err *terminalpkg.Error) *TerminalErrorDetails {
	if err == nil {
		return nil
	}
	details := &TerminalErrorDetails{
		Path: err.Path, Mode: TerminalMode(err.Mode), Platform: err.Platform, Action: err.Action,
	}
	if err.Max > 0 {
		current, maximum := err.Current, err.Max
		details.Current, details.Max = &current, &maximum
	}
	if err.Controller != nil {
		details.Controller = &TerminalErrorController{
			Kind: TerminalActorKind(err.Controller.Kind),
			ID:   err.Controller.ID,
		}
	}
	if details.Current == nil && details.Controller == nil && details.Path == "" &&
		details.Mode == "" && details.Platform == "" && details.Action == "" {
		return nil
	}
	return details
}

// TerminalErrorToolDetailsFromDomain projects terminal metadata into the
// generic tool-error details object without deriving fields from message text.
func TerminalErrorToolDetailsFromDomain(err *terminalpkg.Error) map[string]json.RawMessage {
	details := TerminalErrorDetailsFromDomain(err)
	if details == nil {
		return nil
	}
	result := make(map[string]json.RawMessage, 6)
	if details.Current != nil {
		result["current"] = json.RawMessage(strconv.Itoa(*details.Current))
	}
	if details.Max != nil {
		result["max"] = json.RawMessage(strconv.Itoa(*details.Max))
	}
	if details.Controller != nil {
		result["controller"] = json.RawMessage(
			`{"kind":` + strconv.Quote(string(details.Controller.Kind)) +
				`,"id":` + strconv.Quote(details.Controller.ID) + `}`,
		)
	}
	for key, value := range map[string]string{
		"path": details.Path, "mode": string(details.Mode), "platform": details.Platform,
		"action": details.Action,
	} {
		if value != "" {
			result[key] = json.RawMessage(strconv.Quote(value))
		}
	}
	return result
}
