package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

var errInvalidTerminalErrorEnvelope = errors.New("terminal error envelope is invalid")

type terminalAPIError struct {
	statusCode int
	status     string
	payload    contract.TerminalErrorResponse
}

func (e *terminalAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	return apiErrorMessage(e.payload.Error.Message, e.status)
}

func (e *terminalAPIError) cliExitCode() int {
	if e == nil || e.statusCode == 0 {
		return 1
	}
	return apiStatusExitCode(e.statusCode)
}

func (e *terminalAPIError) TerminalErrorEnvelope() contract.TerminalErrorResponse {
	if e == nil {
		return contract.TerminalErrorResponse{}
	}
	return e.payload
}

func parseTerminalAPIError(statusCode int, status string, body []byte) (bool, error) {
	var payload contract.TerminalErrorResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		strings.TrimSpace(payload.Error.Code) == "" ||
		strings.TrimSpace(payload.Error.Message) == "" {
		return false, nil
	}
	payload.Error.Message = redactToolDiagnostic(payload.Error.Message)
	return true, &terminalAPIError{statusCode: statusCode, status: status, payload: payload}
}

func terminalStreamFrameError(payload []byte, operation string) error {
	matched, err := parseTerminalAPIError(0, "", payload)
	if matched {
		return terminalPermanentError(err)
	}
	return terminalPermanentError(fmt.Errorf(
		"cli: decode terminal %s ERROR frame: %w",
		operation,
		errInvalidTerminalErrorEnvelope,
	))
}
