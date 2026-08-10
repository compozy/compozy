package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

const (
	appControlSchemaVersion = 1
	appControlTimeout       = 2 * time.Second
)

type appControlCaller func(context.Context, string, string, any) (any, error)

type appControlRequest struct {
	SchemaVersion int    `json:"schema_version"`
	ID            int    `json:"id"`
	Method        string `json:"method"`
	Params        any    `json:"params,omitempty"`
}

type appControlResponse struct {
	SchemaVersion int                     `json:"schema_version"`
	ID            int                     `json:"id"`
	Result        json.RawMessage         `json:"result,omitempty"`
	Error         *appCommandErrorPayload `json:"error,omitempty"`
}

func callAppControl(ctx context.Context, socketPath string, method string, params any) (any, error) {
	if ctx == nil {
		return nil, errors.New("app control: context is required")
	}
	if _, err := os.Stat(socketPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, newAppCommandError(
				appNotRunningCode,
				"the CompozyOS desktop app is not running",
				err,
			)
		}
		return nil, fmt.Errorf("app control: inspect socket: %w", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, appControlTimeout)
	defer cancel()
	connection, err := (&net.Dialer{}).DialContext(callCtx, "unix", socketPath)
	if err != nil {
		return nil, newAppCommandError(
			appControlUnavailableCode,
			"the CompozyOS desktop app control channel is unavailable",
			err,
		)
	}
	if deadline, ok := callCtx.Deadline(); ok {
		if err := connection.SetDeadline(deadline); err != nil {
			return nil, closeAppControlAfterError(
				connection,
				fmt.Errorf("app control: set deadline: %w", err),
			)
		}
	}

	request := appControlRequest{
		SchemaVersion: appControlSchemaVersion,
		ID:            1,
		Method:        method,
		Params:        params,
	}
	return exchangeAppControl(connection, request)
}

func exchangeAppControl(connection net.Conn, request appControlRequest) (any, error) {
	if err := json.NewEncoder(connection).Encode(request); err != nil {
		return nil, newAppCommandError(
			appControlUnavailableCode,
			"the CompozyOS desktop app control channel did not accept the request",
			closeAppControlAfterError(connection, err),
		)
	}
	var response appControlResponse
	if err := json.NewDecoder(bufio.NewReader(connection)).Decode(&response); err != nil {
		return nil, newAppCommandError(
			appControlUnavailableCode,
			"the CompozyOS desktop app control channel did not respond",
			closeAppControlAfterError(connection, err),
		)
	}
	if err := connection.Close(); err != nil {
		return nil, newAppCommandError(
			appControlUnavailableCode,
			"the CompozyOS desktop app control channel did not close cleanly",
			err,
		)
	}
	if response.SchemaVersion != appControlSchemaVersion || response.ID != request.ID {
		return nil, newAppCommandError(
			appControlUnavailableCode,
			"the CompozyOS desktop app control response is incompatible",
			nil,
		)
	}
	if response.Error != nil {
		return nil, newAppCommandError(response.Error.Code, response.Error.Message, nil)
	}
	if len(response.Result) == 0 {
		return map[string]any{"ok": true}, nil
	}
	var result any
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, newAppCommandError(
			appControlUnavailableCode,
			"the CompozyOS desktop app returned an invalid control response",
			err,
		)
	}
	return result, nil
}

func closeAppControlAfterError(connection net.Conn, cause error) error {
	if closeErr := connection.Close(); closeErr != nil {
		return errors.Join(cause, closeErr)
	}
	return cause
}
