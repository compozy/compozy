package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

// InputOrigin identifies the layer that supplied one effective Loop input.
type InputOrigin string

const (
	InputOriginRun        InputOrigin = "run"
	InputOriginWorkspace  InputOrigin = "workspace"
	InputOriginGlobal     InputOrigin = "global"
	InputOriginDefinition InputOrigin = "definition"
)

const (
	InputDefaultReasonUnknownInput InputDefaultReason = "unknown_input"
	InputDefaultReasonTypeMismatch InputDefaultReason = "type_mismatch"
	InputDefaultReasonRequired     InputDefaultReason = "required"
)

// InputDefaultReason is the closed machine-readable input resolution failure vocabulary.
type InputDefaultReason string

// InputDefaultError is the typed validation error shared by static defaults and effective starts.
type InputDefaultError struct {
	Loop   string             `json:"loop"`
	Key    string             `json:"key"`
	Reason InputDefaultReason `json:"reason"`
	Err    error              `json:"-"`
}

func (e *InputDefaultError) Error() string {
	if e == nil {
		return ""
	}
	message := fmt.Sprintf("input_default: loop=%s key=%s reason=%s", e.Loop, e.Key, e.Reason)
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

// Unwrap keeps input-default failures in the Loop validation family.
func (e *InputDefaultError) Unwrap() error {
	if e == nil || e.Err == nil {
		return ErrValidation
	}
	return errors.Join(ErrValidation, e.Err)
}

// AsInputDefaultError extracts the structured input-default diagnostic.
func AsInputDefaultError(err error) (*InputDefaultError, bool) {
	var target *InputDefaultError
	if !errors.As(err, &target) {
		return nil, false
	}
	return target, true
}

// InputDefaultLayers carries source-preserving configured values for one named Loop.
type InputDefaultLayers struct {
	Global    map[string]any
	Workspace map[string]any
}

// ResolvedInputs contains effective values and the exact winning source for every present key.
type ResolvedInputs struct {
	Values  map[string]any
	Origins map[string]InputOrigin
}

func (s *service) resolveEffectiveInputs(
	ctx context.Context,
	ws WorkspaceID,
	loopName string,
	def dsl.Definition,
	run map[string]any,
) (ResolvedInputs, error) {
	layers, err := s.resolveInputDefaults(ctx, ws, loopName)
	if err != nil {
		return ResolvedInputs{}, fmt.Errorf("resolve Loop input defaults: %w", err)
	}
	return ResolveInputDefaults(def, loopName, run, layers)
}

// ValidateDefinitionInputDefaults validates only author-contained defaults.
func ValidateDefinitionInputDefaults(def dsl.Definition) error {
	loopName := strings.TrimSpace(def.Meta.Name)
	for _, key := range sortedInputKeys(def.Inputs) {
		input := def.Inputs[key]
		if input.Default == nil {
			continue
		}
		if err := validateInputType(key, input.Type, input.Default); err != nil {
			return inputDefaultError(loopName, key, InputDefaultReasonTypeMismatch, err)
		}
	}
	return nil
}

// ResolveInputs applies run and definition values for callers without configured layers.
func ResolveInputs(def dsl.Definition, inputs Inputs) (map[string]any, error) {
	resolved, err := ResolveInputDefaults(def, def.Meta.Name, inputs.Values, InputDefaultLayers{})
	if err != nil {
		return nil, err
	}
	return resolved.Values, nil
}

// ResolveInputDefaults applies run > workspace > global > definition precedence per key.
func ResolveInputDefaults(
	def dsl.Definition,
	loopName string,
	run map[string]any,
	layers InputDefaultLayers,
) (ResolvedInputs, error) {
	name := strings.TrimSpace(loopName)
	for _, source := range []map[string]any{run, layers.Workspace, layers.Global} {
		if key := firstUnknownInput(def.Inputs, source); key != "" {
			return ResolvedInputs{}, inputDefaultError(
				name,
				key,
				InputDefaultReasonUnknownInput,
				fmt.Errorf("input %q is not declared", key),
			)
		}
	}

	resolved := ResolvedInputs{
		Values:  make(map[string]any, len(def.Inputs)),
		Origins: make(map[string]InputOrigin, len(def.Inputs)),
	}
	for _, key := range sortedInputKeys(def.Inputs) {
		input := def.Inputs[key]
		value, origin, present := selectInputValue(key, input, run, layers)
		if !present {
			if input.Required {
				return ResolvedInputs{}, inputDefaultError(
					name,
					key,
					InputDefaultReasonRequired,
					fmt.Errorf("input %q is required", key),
				)
			}
			continue
		}
		if err := validateInputType(key, input.Type, value); err != nil {
			return ResolvedInputs{}, inputDefaultError(name, key, InputDefaultReasonTypeMismatch, err)
		}
		resolved.Values[key] = cloneAnyValue(value)
		resolved.Origins[key] = origin
	}
	return resolved, nil
}

func selectInputValue(
	key string,
	input dsl.Input,
	run map[string]any,
	layers InputDefaultLayers,
) (any, InputOrigin, bool) {
	if value, ok := run[key]; ok {
		return value, InputOriginRun, true
	}
	if value, ok := layers.Workspace[key]; ok {
		return value, InputOriginWorkspace, true
	}
	if value, ok := layers.Global[key]; ok {
		return value, InputOriginGlobal, true
	}
	if input.Default != nil {
		return input.Default, InputOriginDefinition, true
	}
	return nil, "", false
}

func firstUnknownInput(declared map[string]dsl.Input, values map[string]any) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := declared[key]; !ok {
			return key
		}
	}
	return ""
}

func sortedInputKeys(inputs map[string]dsl.Input) []string {
	keys := make([]string, 0, len(inputs))
	for key := range inputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func inputDefaultError(
	loopName string,
	key string,
	reason InputDefaultReason,
	err error,
) error {
	return &InputDefaultError{
		Loop: strings.TrimSpace(loopName), Key: strings.TrimSpace(key), Reason: reason, Err: err,
	}
}

func validateInputType(name string, inputType dsl.InputType, value any) error {
	switch inputType {
	case dsl.InputTypeString, dsl.InputTypeFile, dsl.InputTypeAgent, dsl.InputTypeRef:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("input %q must be a string", name)
		}
	case dsl.InputTypeNumber:
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
			float64, float32, json.Number:
		default:
			return fmt.Errorf("input %q must be a number", name)
		}
	case dsl.InputTypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("input %q must be a boolean", name)
		}
	case "":
		return fmt.Errorf("input %q type is required", name)
	default:
		return fmt.Errorf("input %q type is invalid: %q", name, inputType)
	}
	return nil
}
