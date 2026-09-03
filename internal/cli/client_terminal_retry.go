package cli

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type terminalClientPermanentError struct {
	err error
}

func (e *terminalClientPermanentError) Error() string { return e.err.Error() }
func (e *terminalClientPermanentError) Unwrap() error { return e.err }

func terminalReconnectDelay(attempt int) time.Duration {
	base := 500 * time.Millisecond * time.Duration(1<<min(attempt, 4))
	base = min(base, 8*time.Second)
	var random [1]byte
	if _, err := cryptorand.Read(random[:]); err != nil {
		return base
	}
	return base + time.Duration(random[0])*time.Millisecond
}

func terminalPermanentError(err error) error {
	if err == nil {
		return nil
	}
	return &terminalClientPermanentError{err: err}
}

func terminalReconnectableError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.As(err, new(*terminalClientPermanentError)) {
		return false
	}
	if errors.As(err, new(*profileCommandError)) {
		return false
	}
	if apiErr, ok := errors.AsType[*daemonAPIError](err); ok {
		return apiErr.statusCode >= http.StatusInternalServerError
	}
	if errors.As(err, new(*gatewayClientError)) {
		return true
	}
	if closeErr, ok := errors.AsType[*websocket.CloseError](err); ok {
		switch closeErr.Code {
		case websocket.CloseGoingAway,
			websocket.CloseAbnormalClosure,
			websocket.CloseInternalServerErr,
			websocket.CloseServiceRestart,
			websocket.CloseTryAgainLater:
			return true
		default:
			return false
		}
	}
	return true
}
