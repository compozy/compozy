package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

// sessionDeriveAPIError is a continue/fork failure that happened after the daemon
// committed the new session: the payload names the child so it stays reachable.
type sessionDeriveAPIError struct {
	statusCode int
	status     string
	payload    contract.SessionDeriveErrorPayload
}

func (e *sessionDeriveAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	return apiErrorMessage(e.payload.Error, e.status)
}

func (e *sessionDeriveAPIError) cliExitCode() int {
	if e == nil {
		return 1
	}
	return apiStatusExitCode(e.statusCode)
}

func (e *sessionDeriveAPIError) errorPayload() contract.ErrorPayload {
	if e == nil {
		return contract.ErrorPayload{}
	}
	return e.payload.ErrorPayload
}

func (e *sessionDeriveAPIError) sessionDeriveErrorPayload() contract.SessionDeriveErrorPayload {
	if e == nil {
		return contract.SessionDeriveErrorPayload{}
	}
	return e.payload
}

// parseSessionDeriveAPIError matches only error bodies that carry child_session_id,
// which the continue/fork routes set after the child was committed.
func parseSessionDeriveAPIError(statusCode int, status string, body []byte) (bool, error) {
	var payload contract.SessionDeriveErrorPayload
	if json.Unmarshal(body, &payload) != nil || strings.TrimSpace(payload.ChildSessionID) == "" ||
		strings.TrimSpace(payload.Error) == "" {
		return false, nil
	}
	payload.Error = redactToolDiagnostic(payload.Error)
	return true, &sessionDeriveAPIError{statusCode: statusCode, status: status, payload: payload}
}

// sessionDeriveCommandError is how `session continue|fork` report a daemon failure:
// `<code>: <message>` with exit status 1 (_dx.md), plus how to reach a child that was
// already created. It unwraps to the daemon error so structured output keeps its payload.
type sessionDeriveCommandError struct {
	message string
	err     error
}

func (e *sessionDeriveCommandError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *sessionDeriveCommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *sessionDeriveCommandError) cliExitCode() int { return 1 }

func sessionDeriveCommandFailure(err error, idempotencyKey string) error {
	var payload contract.SessionDeriveErrorPayload
	if deriveErr, ok := errors.AsType[*sessionDeriveAPIError](err); ok && deriveErr != nil {
		payload = deriveErr.payload
	} else if apiErr, ok := errors.AsType[*daemonAPIError](err); ok && apiErr != nil {
		payload.ErrorPayload = apiErr.payload
	} else {
		return err
	}
	message := strings.TrimSpace(err.Error())
	if code := strings.TrimSpace(payload.Code); code != "" && !strings.HasPrefix(message, code+":") {
		message = code + ": " + message
	}
	if child := strings.TrimSpace(payload.ChildSessionID); child != "" {
		message += fmt.Sprintf(
			"\nsession %s was already created; open it with `compozy session status %s`, "+
				"or rerun with --idempotency-key %s to get it back",
			child, child, idempotencyKey,
		)
	}
	return &sessionDeriveCommandError{message: message, err: err}
}

func marshalSessionDeriveExecutionError(args []string, payload contract.SessionDeriveErrorPayload) ([]byte, bool) {
	switch requestedOutputFormat(args) {
	case OutputJSON:
		encoded, err := json.Marshal(payload)
		return encoded, err == nil
	case OutputJSONL:
		encoded, err := json.Marshal(struct {
			Type  string                             `json:"type"`
			Error contract.SessionDeriveErrorPayload `json:"error"`
		}{Type: automationErrorKey, Error: payload})
		return append(encoded, '\n'), err == nil
	default:
		return marshalDaemonAPIExecutionError(args, payload.ErrorPayload)
	}
}
