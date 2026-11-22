package cli

import (
	"cmp"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/loop/dsl"
)

// parseLoopInputValidationAPIError renders the daemon's field-addressed Loop
// input refusal instead of echoing the raw 422 body.
func parseLoopInputValidationAPIError(statusCode int, status string, body []byte) (bool, error) {
	var payload contract.LoopValidationResponse
	if json.Unmarshal(body, &payload) != nil || payload.InputValidation == nil {
		return false, nil
	}
	return true, &daemonAPIError{
		statusCode: statusCode,
		status:     status,
		payload: contract.ErrorPayload{
			Error: loopInputValidationMessage(*payload.InputValidation),
		},
	}
}

func loopInputValidationMessage(failure contract.LoopInputValidationErrorPayload) string {
	kind := cmp.Or(strings.TrimSpace(failure.Kind), "input")
	message := fmt.Sprintf("input %q (%s) at %s: ", failure.Field, kind, failure.Origin)
	if strings.TrimSpace(failure.Reason) != "type_mismatch" {
		return message + strings.ReplaceAll(
			cmp.Or(strings.TrimSpace(failure.Reason), "invalid value"), "_", " ",
		)
	}
	message += "type mismatch"
	if value := strings.TrimSpace(failure.Value); value != "" {
		message += fmt.Sprintf(" for value %q", value)
	}
	switch failure.Kind {
	case string(dsl.InputTypeString), string(dsl.InputTypeFile), string(dsl.InputTypeAgent),
		string(dsl.InputTypeRef):
		message += fmt.Sprintf(
			`; quote it as a JSON string, for example --input '%s="false"'`, failure.Field,
		)
	}
	return message
}
