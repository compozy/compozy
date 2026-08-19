package loop

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

// InputValidationReason is the closed machine-readable input failure vocabulary.
type InputValidationReason string

const (
	InputValidationReasonUnknownInput       InputValidationReason = "unknown_input"
	InputValidationReasonRequired           InputValidationReason = "required"
	InputValidationReasonTypeMismatch       InputValidationReason = "type_mismatch"
	InputValidationReasonEnumMismatch       InputValidationReason = "enum_mismatch"
	InputValidationReasonInvalidKindPayload InputValidationReason = "invalid_kind_payload"
	InputValidationReasonUnknownReference   InputValidationReason = "unknown_reference"
	InputValidationReasonInvalidRuntime     InputValidationReason = "invalid_runtime"
)

// InputValidationError carries one field-addressed Loop input failure.
type InputValidationError struct {
	Loop   string                `json:"loop"`
	Field  string                `json:"field"`
	Kind   string                `json:"kind,omitempty"`
	Value  string                `json:"value,omitempty"`
	Origin InputOrigin           `json:"origin"`
	Reason InputValidationReason `json:"reason"`
	Err    error                 `json:"-"`
}

func (e *InputValidationError) Error() string {
	if e == nil {
		return ""
	}
	message := fmt.Sprintf(
		"input_validation: loop=%s field=%s origin=%s reason=%s",
		e.Loop,
		e.Field,
		e.Origin,
		e.Reason,
	)
	if e.Kind != "" {
		message += " kind=" + e.Kind
	}
	if e.Value != "" {
		message += " value=" + e.Value
	}
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

// Unwrap keeps input failures in the Loop validation family.
func (e *InputValidationError) Unwrap() error {
	if e == nil || e.Err == nil {
		return ErrValidation
	}
	return errors.Join(ErrValidation, e.Err)
}

// AsInputValidationError extracts the structured input diagnostic.
func AsInputValidationError(err error) (*InputValidationError, bool) {
	return errors.AsType[*InputValidationError](err)
}

func newInputValidationError(
	loopName string,
	field string,
	input dsl.Input,
	value any,
	origin InputOrigin,
	reason InputValidationReason,
	err error,
) error {
	return &InputValidationError{
		Loop: strings.TrimSpace(loopName), Field: strings.TrimSpace(field),
		Kind: effectiveInputKind(input), Value: safeInputValue(input, value),
		Origin: origin, Reason: reason, Err: err,
	}
}

func effectiveInputKind(input dsl.Input) string {
	if len(input.Enum) > 0 {
		return jsonSchemaEnumKey
	}
	if input.Type == dsl.InputTypeRef && input.Ref != nil {
		return string(input.Ref.Kind)
	}
	return string(input.Type)
}

func safeInputValue(input dsl.Input, value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	if input.Type != dsl.InputTypeRuntime {
		return ""
	}
	runtime, err := runtimeInputSpec(value)
	if err != nil {
		return ""
	}
	model := runtime.Model
	if model == "" {
		model = "-"
	}
	provider := runtime.Provider
	if provider == "" {
		provider = "-"
	}
	display := provider + "/" + model
	if runtime.Reasoning != "" {
		display += "@" + runtime.Reasoning
	}
	return display
}

func validateInputValue(name string, input dsl.Input, value any) (any, InputValidationReason, error) {
	switch input.Type {
	case dsl.InputTypeString, dsl.InputTypeFile, dsl.InputTypeAgent, dsl.InputTypeRef:
		text, ok := value.(string)
		if !ok {
			return nil, InputValidationReasonTypeMismatch, fmt.Errorf("input %q must be a string", name)
		}
		if len(input.Enum) > 0 && !slices.Contains(input.Enum, text) {
			return nil, InputValidationReasonEnumMismatch, fmt.Errorf("input %q is not an allowed value", name)
		}
		return text, "", nil
	case dsl.InputTypeRuntime:
		runtime, err := runtimeInputSpec(value)
		if err != nil {
			return nil, InputValidationReasonInvalidKindPayload, fmt.Errorf("input %q runtime: %w", name, err)
		}
		return runtimeInputValue(runtime), "", nil
	case dsl.InputTypeNumber:
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
			float64, float32, json.Number:
			return value, "", nil
		default:
			return nil, InputValidationReasonTypeMismatch, fmt.Errorf("input %q must be a number", name)
		}
	case dsl.InputTypeBoolean:
		if _, ok := value.(bool); !ok {
			return nil, InputValidationReasonTypeMismatch, fmt.Errorf("input %q must be a boolean", name)
		}
		return value, "", nil
	case "":
		return nil, InputValidationReasonInvalidKindPayload, fmt.Errorf("input %q type is required", name)
	default:
		return nil, InputValidationReasonInvalidKindPayload,
			fmt.Errorf("input %q type is invalid: %q", name, input.Type)
	}
}

func runtimeInputSpec(value any) (dsl.RuntimeSpec, error) {
	switch typed := value.(type) {
	case dsl.RuntimeSpec:
		if len(typed.Extra) > 0 {
			return dsl.RuntimeSpec{}, fmt.Errorf("contains unknown fields")
		}
		return typed, nil
	case *dsl.RuntimeSpec:
		if typed == nil {
			return dsl.RuntimeSpec{}, fmt.Errorf("must be an object")
		}
		return runtimeInputSpec(*typed)
	case map[string]any:
		return runtimeInputSpecFromMap(typed)
	case map[string]string:
		values := make(map[string]any, len(typed))
		for key, field := range typed {
			values[key] = field
		}
		return runtimeInputSpecFromMap(values)
	default:
		return dsl.RuntimeSpec{}, fmt.Errorf("must be an object")
	}
}

func runtimeInputSpecFromMap(value map[string]any) (dsl.RuntimeSpec, error) {
	runtime := dsl.RuntimeSpec{}
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		raw := value[key]
		text, ok := raw.(string)
		if !ok {
			return dsl.RuntimeSpec{}, fmt.Errorf("%s must be a string", key)
		}
		switch key {
		case runtimeFieldProvider:
			runtime.Provider = strings.TrimSpace(text)
		case runtimeFieldModel:
			runtime.Model = strings.TrimSpace(text)
		case runtimeFieldReasoning:
			runtime.Reasoning = strings.TrimSpace(text)
		default:
			return dsl.RuntimeSpec{}, fmt.Errorf("%s is unknown", key)
		}
	}
	return runtime, nil
}

func runtimeInputValue(runtime dsl.RuntimeSpec) map[string]any {
	value := make(map[string]any, 3)
	if runtime.Provider != "" {
		value[runtimeFieldProvider] = runtime.Provider
	}
	if runtime.Model != "" {
		value[runtimeFieldModel] = runtime.Model
	}
	if runtime.Reasoning != "" {
		value[runtimeFieldReasoning] = runtime.Reasoning
	}
	return value
}
