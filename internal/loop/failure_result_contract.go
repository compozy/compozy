package loop

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/loop/dsl"
)

const (
	payloadInspectionMaxBytes = 1 << 20
	payloadDeclaredCode       = "payload_declared"
	payloadInspectionSkipped  = "payload_inspection_skipped"
)

// PayloadDiagnostic is a recordable result of a skipped payload inspection.
type PayloadDiagnostic struct {
	Code  string `json:"code"`
	Cause string `json:"cause"`
}

// PayloadInspection is the pure result of inspecting a successful transport body.
type PayloadInspection struct {
	Failure    *ActionFailure     `json:"failure,omitempty"`
	Diagnostic *PayloadDiagnostic `json:"diagnostic,omitempty"`
}

// InspectPayloadFailure applies a custom result contract or the built-in v0 truth table.
func InspectPayloadFailure(raw json.RawMessage, contract *dsl.ResultContract) PayloadInspection {
	if len(raw) == 0 {
		return PayloadInspection{}
	}
	if len(raw) > payloadInspectionMaxBytes || !utf8.Valid(raw) {
		return skippedPayloadInspection("payload is not inspectable within the configured bound")
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return skippedPayloadInspection("payload is not an inspectable JSON object")
	}
	if body == nil {
		return skippedPayloadInspection("payload is not an inspectable JSON object")
	}
	if contract != nil {
		return inspectDeclaredResultContract(body, *contract)
	}
	if value, exists := body["error"]; exists {
		if failure, failed := declaredPayloadValue("error", value, body, ""); failed {
			return PayloadInspection{Failure: &failure}
		}
	}
	if value, exists := body["success"]; exists && declaresUnsuccessful(value) {
		failure := NewActionFailure(payloadDeclaredCode, "payload field success declared failure", "")
		return PayloadInspection{Failure: &failure}
	}
	return PayloadInspection{}
}

func inspectDeclaredResultContract(body map[string]any, contract dsl.ResultContract) PayloadInspection {
	failureField := strings.TrimSpace(contract.FailureField)
	if failureField == "" {
		return PayloadInspection{}
	}
	value, exists := lookupPayloadPath(body, failureField)
	if !exists {
		return PayloadInspection{}
	}
	failure, failed := declaredPayloadValue(failureField, value, body, contract.MessageField)
	if !failed {
		return PayloadInspection{}
	}
	return PayloadInspection{Failure: &failure}
}

func declaredPayloadValue(
	field string,
	value any,
	body map[string]any,
	messageField string,
) (ActionFailure, bool) {
	switch typed := value.(type) {
	case nil:
		return ActionFailure{}, false
	case string:
		if strings.TrimSpace(typed) == "" {
			return ActionFailure{}, false
		}
	case bool:
		if !typed {
			return ActionFailure{}, false
		}
	case map[string]any:
		if len(typed) == 0 {
			return NewActionFailure(payloadDeclaredCode, "declared error was empty", ""), true
		}
	case []any:
		if len(typed) == 0 {
			return NewActionFailure(payloadDeclaredCode, "declared error was empty", ""), true
		}
	}
	cause := declaredPayloadCause(field, value, body, messageField)
	return NewActionFailure(payloadDeclaredCode, cause, ""), true
}

func declaredPayloadCause(field string, value any, body map[string]any, messageField string) string {
	if path := strings.TrimSpace(messageField); path != "" {
		if message, ok := lookupPayloadPath(body, path); ok {
			if text, ok := message.(string); ok && strings.TrimSpace(text) != "" {
				return text
			}
		}
	}
	if text, ok := value.(string); ok {
		return text
	}
	if object, ok := value.(map[string]any); ok {
		if message, ok := object["message"].(string); ok && strings.TrimSpace(message) != "" {
			return message
		}
	}
	encoded, err := json.Marshal(value)
	if err == nil {
		return fmt.Sprintf("payload field %s declared failure: %s", field, encoded)
	}
	return fmt.Sprintf("payload field %s declared failure", field)
}

func lookupPayloadPath(body map[string]any, rawPath string) (any, bool) {
	segments := strings.Split(strings.TrimSpace(rawPath), ".")
	if len(segments) == 0 || segments[0] == "" {
		return nil, false
	}
	var current any = body
	for _, segment := range segments {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func declaresUnsuccessful(value any) bool {
	switch typed := value.(type) {
	case bool:
		return !typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "false")
	default:
		return false
	}
}

func skippedPayloadInspection(cause string) PayloadInspection {
	return PayloadInspection{Diagnostic: &PayloadDiagnostic{Code: payloadInspectionSkipped, Cause: cause}}
}
