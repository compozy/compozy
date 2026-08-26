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
	var permanent *terminalClientPermanentError
	if errors.As(err, &permanent) {
		return false
	}
	var profileErr *profileCommandError
	if errors.As(err, &profileErr) {
		return false
	}
	var apiErr *daemonAPIError
	if errors.As(err, &apiErr) {
		return apiErr.statusCode >= http.StatusInternalServerError
	}
	var gatewayErr *gatewayClientError
	if errors.As(err, &gatewayErr) {
		return true
	}
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
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
