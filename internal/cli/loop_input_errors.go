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

// loopInputKindEnum is how the daemon reports an enum-backed string input.
const loopInputKindEnum = "enum"

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
	if loopInputKindIsTextual(failure.Kind) {
		message += fmt.Sprintf(
			`; quote it as a JSON string, for example --input '%s="false"'`,
			shellSafeLoopInputField(failure.Field),
		)
	}
	return message
}

// loopInputKindIsTextual reports whether the daemon's reported kind accepts
// string text. It mirrors effectiveInputKind, which reports ref kinds and
// "enum" instead of the base declared type.
func loopInputKindIsTextual(kind string) bool {
	trimmed := strings.TrimSpace(kind)
	switch dsl.InputType(trimmed) {
	case dsl.InputTypeString, dsl.InputTypeFile, dsl.InputTypeAgent, dsl.InputTypeRef:
		return true
	}
	switch dsl.InputRefKind(trimmed) {
	case dsl.InputRefKindSkill, dsl.InputRefKindLoop, dsl.InputRefKindWorktree,
		dsl.InputRefKindSession, dsl.InputRefKindWorkspace, dsl.InputRefKindSecret:
		return true
	}
	return trimmed == loopInputKindEnum
}

// shellSafeLoopInputField keeps the example pasteable as one shell word: input
// keys are author-controlled, so strip terminal controls and close/reopen the
// single-quoted string around any quote.
func shellSafeLoopInputField(field string) string {
	field = strings.TrimSpace(safeLoopPromptText(field))
	if field == "" {
		return "field"
	}
	return strings.ReplaceAll(field, "'", `'\''`)
}
