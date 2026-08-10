package cli

import "encoding/json"

const (
	appNotInstalledCode       = "app_not_installed"
	appNotRunningCode         = "app_not_running"
	appLaunchFailedCode       = "app_launch_failed"
	invalidTargetPathCode     = "invalid_target_path"
	appStateSchemaUnknownCode = "app_state_schema_unknown"
	appControlUnavailableCode = "app_control_unavailable"
)

type appCommandError struct {
	code    string
	message string
	cause   error
}

type appCommandErrorEnvelope struct {
	Error appCommandErrorPayload `json:"error"`
}

type appCommandErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newAppCommandError(code string, message string, cause error) error {
	return &appCommandError{code: code, message: message, cause: cause}
}

func (e *appCommandError) Error() string {
	if e == nil {
		return "desktop app command failed"
	}
	return e.message
}

func (e *appCommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *appCommandError) payload() appCommandErrorEnvelope {
	if e == nil {
		return appCommandErrorEnvelope{}
	}
	return appCommandErrorEnvelope{Error: appCommandErrorPayload{Code: e.code, Message: e.message}}
}

func marshalAppCommandExecutionError(args []string, appErr *appCommandError) ([]byte, bool) {
	switch requestedOutputFormat(args) {
	case OutputJSON:
		encoded, err := json.Marshal(appErr.payload())
		return encoded, err == nil
	case OutputJSONL:
		encoded, err := json.Marshal(struct {
			Type  string                 `json:"type"`
			Error appCommandErrorPayload `json:"error"`
		}{Type: clientErrorKey, Error: appErr.payload().Error})
		return append(encoded, '\n'), err == nil
	case OutputToon:
		payload := appErr.payload().Error
		return []byte(renderToonObject(
			"error",
			[]string{cliCodeKey, clientMessageKey},
			[]string{payload.Code, payload.Message},
		)), true
	default:
		return nil, false
	}
}
