package core

import (
	"errors"
	"fmt"
	"net/http"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

var (
	errTerminalRequestInvalid = errors.New("terminal request is invalid")
	errTerminalUnavailable    = errors.New("terminal service is unavailable")
)

func (h *BaseHandlers) respondTerminalUnavailable(c *gin.Context) {
	h.respondTerminalError(c, errTerminalUnavailable)
}

func terminalRequestError(err error) error {
	return fmt.Errorf("%w: %v", errTerminalRequestInvalid, err)
}

func (h *BaseHandlers) respondTerminalError(c *gin.Context, err error) {
	status, code := terminalErrorStatusCode(err)
	payload := ErrorPayloadForStatus(status, err, h.MaskInternalErrors)
	payload.Code = code
	c.AbortWithStatusJSON(status, payload)
}

func (h *BaseHandlers) respondTerminalStatus(c *gin.Context, status int, code, message string) {
	err := errors.New(message)
	payload := ErrorPayloadForStatus(status, err, h.MaskInternalErrors)
	payload.Code = code
	c.AbortWithStatusJSON(status, payload)
}

func terminalErrorStatusCode(err error) (int, string) {
	switch {
	case errors.Is(err, errTerminalRequestInvalid):
		return http.StatusBadRequest, "invalid_request"
	case errors.Is(err, errTerminalUnavailable):
		return http.StatusServiceUnavailable, "terminal_unavailable"
	case errors.Is(err, errTerminalTicketExpired):
		return http.StatusForbidden, "ticket_expired"
	case errors.Is(err, errTerminalTicketInvalid):
		return http.StatusForbidden, "ticket_invalid"
	}
	var typed *terminalpkg.Error
	if errors.As(err, &typed) {
		return terminalDomainStatus(typed), typed.Code
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
		errors.Is(err, terminalpkg.ErrExited):
		return http.StatusConflict
	case errors.Is(err, terminalpkg.ErrRequiresWorkspace),
		errors.Is(err, terminalpkg.ErrInvalidCwd),
		errors.Is(err, terminalpkg.ErrInteractive),
		errors.Is(err, terminalpkg.ErrNotInteractive),
		errors.Is(err, terminalpkg.ErrUnsupported):
		return http.StatusUnprocessableEntity
	case errors.Is(err, terminalpkg.ErrShuttingDown):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
