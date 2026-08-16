package cli

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/api/contract"
)

func marshalExtensionValidationExecutionError(
	args []string,
	payload contract.ExtensionValidationErrorPayload,
) ([]byte, bool) {
	switch requestedOutputFormat(args) {
	case OutputJSON:
		encoded, err := json.Marshal(payload)
		return encoded, err == nil
	case OutputJSONL:
		encoded, err := json.Marshal(struct {
			Type  string                                   `json:"type"`
			Error contract.ExtensionValidationErrorPayload `json:"error"`
		}{Type: clientErrorKey, Error: payload})
		return append(encoded, '\n'), err == nil
	case OutputToon:
		issues, err := json.Marshal(payload.Issues)
		if err != nil {
			return nil, false
		}
		code := ""
		if payload.Diagnostic != nil {
			code = payload.Diagnostic.Code
		}
		return []byte(renderToonObject(
			"error",
			[]string{cliCodeKey, clientMessageKey, cliIssuesKey},
			[]string{code, payload.Error, string(issues)},
		)), true
	default:
		return nil, false
	}
}
