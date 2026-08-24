package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

type profileCommandError struct {
	payload contract.ProfileErrorPayload
}

func (e *profileCommandError) Error() string {
	if e == nil {
		return "profile operation failed"
	}
	return strings.TrimSpace(e.payload.Error.Message)
}

func (e *profileCommandError) cliExitCode() int { return 1 }

func (e *profileCommandError) profileErrorPayload() contract.ProfileErrorPayload {
	if e == nil {
		return contract.ProfileErrorPayload{}
	}
	return e.payload
}

func parseProfileAPIError(_ int, _ string, body []byte) (bool, error) {
	var payload contract.ProfileErrorPayload
	if json.Unmarshal(body, &payload) != nil || !strings.HasPrefix(payload.Error.Code, "profile_") {
		return false, nil
	}
	return true, &profileCommandError{payload: payload}
}

func marshalProfileExecutionError(args []string, payload contract.ProfileErrorPayload) ([]byte, bool) {
	switch requestedOutputFormat(args) {
	case OutputJSON:
		encoded, err := json.Marshal(payload)
		return encoded, err == nil
	case OutputJSONL:
		encoded, err := json.Marshal(payload)
		return append(encoded, '\n'), err == nil
	case OutputToon:
		return []byte(renderToonObject(automationErrorKey, []string{
			cliCodeKey, bridgeMessageKey, authoredContextActionKey,
		}, []string{
			payload.Error.Code, payload.Error.Message, payload.Error.Action,
		})), true
	default:
		return nil, false
	}
}

func renderProfileExecutionError(err error) (string, bool) {
	typed, ok := errors.AsType[interface {
		error
		profileErrorPayload() contract.ProfileErrorPayload
	}](err)
	if !ok {
		return "", false
	}
	payload := typed.profileErrorPayload().Error
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = "profile operation failed"
	}
	if action := strings.TrimSpace(payload.Action); action != "" {
		return fmt.Sprintf("Error: %s — %s", message, action), true
	}
	return "Error: " + message, true
}

func newProfileSelectionError(code, message, action string) error {
	return &profileCommandError{payload: contract.ProfileErrorPayload{Error: contract.ProfileError{
		Code: code, Message: message, Action: action,
	}}}
}
