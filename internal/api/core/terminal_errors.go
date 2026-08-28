package core

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

var (
	errTerminalRequestInvalid = errors.New("terminal request is invalid")
	errTerminalUnavailable    = errors.New("terminal service is unavailable")
)

const (
	terminalTransportInvalidRequest     = "invalid_request"
	terminalTransportUnauthorized       = "unauthorized"
	terminalTransportForbidden          = "forbidden"
	terminalTransportServiceUnavailable = "service_unavailable"
	terminalTransportInternalError      = "internal_error"
	terminalTransportAPIError           = "api_error"
)

func (h *BaseHandlers) respondTerminalUnavailable(c *gin.Context) {
	h.respondTerminalError(c, errTerminalUnavailable)
}

func terminalRequestError(err error) error {
	return fmt.Errorf("%w: %w", errTerminalRequestInvalid, err)
}

func (h *BaseHandlers) respondTerminalError(c *gin.Context, err error) {
	if isProfileDomainError(err) {
		h.respondTerminalProfileError(c, err)
		return
	}
	status, code, domain := terminalErrorStatusCode(err)
	payload := terminalErrorResponse(code, err)
	if !domain && h.MaskInternalErrors && status >= http.StatusInternalServerError {
		payload.Error.Message = http.StatusText(status)
	}
	c.AbortWithStatusJSON(status, payload)
}

// TerminalErrorStatus returns the status and safe code classification for a terminal failure.
func TerminalErrorStatus(err error) (int, string, bool) {
	return terminalErrorStatusCode(err)
}

// TerminalErrorResponseForError projects a terminal failure without writing to a transport.
func TerminalErrorResponseForError(err error, status int, maskInternal bool) contract.TerminalErrorResponse {
	derivedStatus, code, domain := terminalErrorStatusCode(err)
	if domain || status <= 0 {
		status = derivedStatus
	}
	if !domain {
		code = terminalTransportCode(status)
	}
	payload := terminalErrorResponse(code, err)
	if !domain && maskInternal && status >= http.StatusInternalServerError {
		payload.Error.Message = http.StatusText(status)
	}
	return payload
}

func (h *BaseHandlers) respondTerminalStatus(
	c *gin.Context,
	status int,
	message string,
) {
	c.AbortWithStatusJSON(status, terminalErrorResponse(terminalTransportCode(status), errors.New(message)))
}

func (h *BaseHandlers) respondTerminalMappedError(
	c *gin.Context,
	status int,
	err error,
) {
	payload := terminalErrorResponse(terminalTransportCode(status), err)
	if h.MaskInternalErrors && status >= http.StatusInternalServerError {
		payload.Error.Message = http.StatusText(status)
	}
	c.AbortWithStatusJSON(status, payload)
}

func (h *BaseHandlers) respondTerminalProfileError(c *gin.Context, err error) {
	if !isProfileDomainError(err) {
		h.respondTerminalMappedError(c, StatusForAgentIdentityError(err), err)
		return
	}
	status, profilePayload := profileErrorResponse(err)
	code := contract.TerminalErrorCode(profilePayload.Error.Code)
	if !contract.IsTerminalErrorCode(code) {
		h.respondTerminalMappedError(c, status, err)
		return
	}
	payload := contract.TerminalErrorResponse{Error: contract.TerminalErrorDetail{
		Code: string(code), Message: profilePayload.Error.Message,
	}}
	if profilePayload.Error.Action != "" {
		payload.Error.Details = &contract.TerminalErrorDetails{Action: profilePayload.Error.Action}
	}
	c.AbortWithStatusJSON(status, payload)
}

func terminalErrorResponse(code string, err error) contract.TerminalErrorResponse {
	detail := contract.TerminalErrorDetail{Code: code, Message: err.Error()}
	if typed, ok := errors.AsType[*terminalpkg.Error](err); ok {
		detail.Details = contract.TerminalErrorDetailsFromDomain(typed)
	}
	return contract.TerminalErrorResponse{Error: detail}
}

func terminalErrorStatusCode(err error) (int, string, bool) {
	switch {
	case errors.Is(err, errTerminalRequestInvalid):
		return http.StatusBadRequest, terminalTransportInvalidRequest, false
	case errors.Is(err, errTerminalUnavailable):
		return http.StatusServiceUnavailable, terminalTransportServiceUnavailable, false
	case errors.Is(err, terminalpkg.ErrTicketExpired):
		return http.StatusForbidden, string(contract.TerminalErrorTicketExpired), true
	case errors.Is(err, terminalpkg.ErrTicketInvalid):
		return http.StatusForbidden, string(contract.TerminalErrorTicketInvalid), true
	case errors.Is(err, windowmanager.ErrClientUnauthorized):
		return http.StatusForbidden, terminalTransportForbidden, false
	}
	if typed, ok := errors.AsType[*terminalpkg.Error](err); ok {
		code := contract.TerminalErrorCode(typed.Code)
		if contract.IsTerminalErrorCode(code) {
			status, _ := TerminalDomainStatus(typed.Code)
			return status, string(code), true
		}
		return http.StatusInternalServerError, terminalTransportInternalError, false
	}
	status := terminalNonDomainStatus(err)
	return status, terminalTransportCode(status), false
}

func terminalTransportCode(status int) string {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusUpgradeRequired:
		return terminalTransportInvalidRequest
	case http.StatusUnauthorized:
		return terminalTransportUnauthorized
	case http.StatusForbidden:
		return terminalTransportForbidden
	case http.StatusServiceUnavailable:
		return terminalTransportServiceUnavailable
	case http.StatusInternalServerError:
		return terminalTransportInternalError
	default:
		return terminalTransportAPIError
	}
}

func terminalNonDomainStatus(err error) int {
	switch {
	case errors.Is(err, terminalpkg.ErrUnsupported):
		return http.StatusUnprocessableEntity
	case errors.Is(err, terminalpkg.ErrApprovalRequired):
		return http.StatusForbidden
	case errors.Is(err, terminalpkg.ErrPolicyDenied):
		return http.StatusForbidden
	case errors.Is(err, terminalpkg.ErrInputPending):
		return http.StatusConflict
	case errors.Is(err, terminalpkg.ErrInputResolved),
		errors.Is(err, terminalpkg.ErrInputResolving),
		errors.Is(err, terminalpkg.ErrWriteLeaseRequired):
		return http.StatusConflict
	case errors.Is(err, terminalpkg.ErrWriteAttachmentRequired):
		return http.StatusForbidden
	case errors.Is(err, terminalpkg.ErrShuttingDown):
		return http.StatusServiceUnavailable
	case errors.Is(err, terminalpkg.ErrServiceUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, terminalpkg.ErrRunIdentityIncomplete):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// TerminalDomainStatus returns the public status for every frozen terminal domain code.
func TerminalDomainStatus(code terminalpkg.ErrorCode) (int, bool) {
	switch code {
	case terminalpkg.ErrorCodeNotFound, terminalpkg.ErrorCodeInputRequestNotFound:
		return http.StatusNotFound, true
	case terminalpkg.ErrorCodeProfileSelectionConflict:
		return http.StatusBadRequest, true
	case terminalpkg.ErrorCodeExpired:
		return http.StatusGone, true
	case terminalpkg.ErrorCodeTypingGrantRejected, terminalpkg.ErrorCodeApprovalRejected,
		terminalpkg.ErrorCodeTicketInvalid, terminalpkg.ErrorCodeTicketExpired,
		terminalpkg.ErrorCodeInputAnswerRequiresWrite:
		return http.StatusForbidden, true
	case terminalpkg.ErrorCodeRequiresWorkspace, terminalpkg.ErrorCodeInteractiveUnavailable,
		terminalpkg.ErrorCodeNotInteractive, terminalpkg.ErrorCodeInvalidCwd,
		terminalpkg.ErrorCodeTimeoutOutOfRange, terminalpkg.ErrorCodeRecordingUnavailable:
		return http.StatusUnprocessableEntity, true
	case terminalpkg.ErrorCodeJournalUnavailable:
		return http.StatusServiceUnavailable, true
	case terminalpkg.ErrorCodeProfileSessionConflict, terminalpkg.ErrorCodeProfileArchived,
		terminalpkg.ErrorCodeProfileUnavailable, terminalpkg.ErrorCodeLimitReached,
		terminalpkg.ErrorCodeSubscriberLimitReached, terminalpkg.ErrorCodeExited,
		terminalpkg.ErrorCodeWriteOwnerHeld, terminalpkg.ErrorCodeLeaseRevoked,
		terminalpkg.ErrorCodeGenerationFenced, terminalpkg.ErrorCodeInputRequestAnswered,
		terminalpkg.ErrorCodeInputRequestSuperseded, terminalpkg.ErrorCodeInputRequestLimitReached,
		terminalpkg.ErrorCodeRecordingAlreadyStarted, terminalpkg.ErrorCodeRecordingNotActive,
		terminalpkg.ErrorCodeSlowConsumer:
		return http.StatusConflict, true
	default:
		return http.StatusInternalServerError, false
	}
}
