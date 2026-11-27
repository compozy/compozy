package cli

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

type localExtensionManifestDecodeError struct {
	*extensionpkg.ManifestDecodeError
}

func (e localExtensionManifestDecodeError) Error() string {
	return redactToolDiagnostic(e.ManifestDecodeError.Error())
}

func (e localExtensionManifestDecodeError) Unwrap() error { return e.ManifestDecodeError }

func (e localExtensionManifestDecodeError) extensionValidationErrorPayload() contract.ExtensionValidationErrorPayload {
	message := e.Error()
	return contract.ExtensionValidationErrorPayload{
		Error:  message,
		Issues: []contract.ValidationIssue{e.Issue(message)},
	}
}

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
