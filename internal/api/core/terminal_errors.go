package core

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

var (
	errTerminalRequestInvalid = errors.New("terminal request is invalid")
	errTerminalUnavailable    = errors.New("terminal service is unavailable")
	errTerminalReadOnly       = errors.New("terminal attachment is read-only")
)

func (h *BaseHandlers) respondTerminalUnavailable(c *gin.Context) {
	h.respondTerminalError(c, errTerminalUnavailable)
}

func terminalRequestError(err error) error {
	return fmt.Errorf("%w: %w", errTerminalRequestInvalid, err)
}

func (h *BaseHandlers) respondTerminalError(c *gin.Context, err error) {
	status, code := terminalErrorStatusCode(err)
	payload := terminalErrorResponse(status, code, err, h.MaskInternalErrors)
	var typed *terminalpkg.Error
	if errors.As(err, &typed) && typed.Max > 0 {
		payload.Error.Details = map[string]string{
			"current": strconv.Itoa(typed.Current),
			"max":     strconv.Itoa(typed.Max),
		}
	}
	c.AbortWithStatusJSON(status, payload)
}

func (h *BaseHandlers) respondTerminalStatus(
	c *gin.Context,
	status int,
	code contract.TerminalErrorCode,
	message string,
) {
	err := errors.New(message)
	c.AbortWithStatusJSON(status, terminalErrorResponse(status, code, err, h.MaskInternalErrors))
}

func (h *BaseHandlers) respondTerminalMappedError(
	c *gin.Context,
	status int,
	code contract.TerminalErrorCode,
	err error,
) {
	c.AbortWithStatusJSON(status, terminalErrorResponse(status, code, err, h.MaskInternalErrors))
}

func (h *BaseHandlers) respondTerminalProfileError(c *gin.Context, err error) {
	if !isProfileDomainError(err) {
		h.respondTerminalMappedError(c, StatusForAgentIdentityError(err), "terminal_identity_invalid", err)
		return
	}
	status, profilePayload := profileErrorResponse(err)
	payload := contract.TerminalErrorResponse{Error: contract.TerminalErrorDetail{
		Code: contract.TerminalErrorCode(profilePayload.Error.Code), Message: profilePayload.Error.Message,
	}}
	if profilePayload.Error.Action != "" {
		payload.Error.Details = map[string]string{"action": profilePayload.Error.Action}
	}
	c.AbortWithStatusJSON(status, payload)
}

func terminalErrorResponse(
	status int,
	code contract.TerminalErrorCode,
	err error,
	maskInternalErrors bool,
) contract.TerminalErrorResponse {
	legacy := ErrorPayloadForStatus(status, err, maskInternalErrors)
	return contract.TerminalErrorResponse{Error: contract.TerminalErrorDetail{
		Code: code, Message: legacy.Error, Details: legacy.Details,
	}}
}

func terminalErrorStatusCode(err error) (int, contract.TerminalErrorCode) {
	switch {
	case errors.Is(err, errTerminalRequestInvalid):
		return http.StatusBadRequest, "invalid_request"
	case errors.Is(err, errTerminalUnavailable):
		return http.StatusServiceUnavailable, "terminal_unavailable"
	case errors.Is(err, errTerminalTicketExpired):
		return http.StatusForbidden, "ticket_expired"
	case errors.Is(err, errTerminalTicketInvalid):
		return http.StatusForbidden, "ticket_invalid"
	case errors.Is(err, errTerminalReadOnly):
		return http.StatusForbidden, "input_answer_requires_write"
	case errors.Is(err, windowmanager.ErrClientUnauthorized):
		return http.StatusForbidden, "terminal_client_unauthorized"
	}
	if typed, ok := errors.AsType[*terminalpkg.Error](err); ok {
		return terminalDomainStatus(typed), contract.TerminalErrorCode(typed.Code)
	}
	return http.StatusInternalServerError, "internal_error"
}

func terminalDomainStatus(err *terminalpkg.Error) int {
	switch {
	case errors.Is(err, terminalpkg.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, terminalpkg.ErrExpired):
		return http.StatusGone
	case errors.Is(err, terminalpkg.ErrLimitReached),
		errors.Is(err, terminalpkg.ErrSubscriberLimit),
		errors.Is(err, terminalpkg.ErrWriteOwnerHeld),
		errors.Is(err, terminalpkg.ErrLeaseRevoked),
		errors.Is(err, terminalpkg.ErrGenerationFenced),
		errors.Is(err, terminalpkg.ErrExited),
		errors.Is(err, terminalpkg.ErrInputAnswered),
		errors.Is(err, terminalpkg.ErrInputLimit),
		errors.Is(err, terminalpkg.ErrRecording):
		return http.StatusConflict
	case errors.Is(err, terminalpkg.ErrInputNotFound):
		return http.StatusNotFound
	case errors.Is(err, terminalpkg.ErrInputRequiresWrite),
		errors.Is(err, terminalpkg.ErrApprovalRequired),
		errors.Is(err, terminalpkg.ErrTypingGrant):
		return http.StatusForbidden
	case errors.Is(err, terminalpkg.ErrRequiresWorkspace),
		errors.Is(err, terminalpkg.ErrInvalidCwd),
		errors.Is(err, terminalpkg.ErrInteractive),
		errors.Is(err, terminalpkg.ErrNotInteractive),
		errors.Is(err, terminalpkg.ErrUnsupported):
		return http.StatusUnprocessableEntity
	case errors.Is(err, terminalpkg.ErrShuttingDown),
		errors.Is(err, terminalpkg.ErrJournalUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
