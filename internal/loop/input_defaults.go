package loop

import (
	"context"
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
	InputOriginAutomation InputOrigin = "automation"
	InputOriginResponse   InputOrigin = "response"
)

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

// ValidateDefinitionInputs validates author-contained defaults without resolving configured layers.
func ValidateDefinitionInputs(def dsl.Definition) error {
	loopName := strings.TrimSpace(def.Meta.Name)
	for _, key := range sortedInputKeys(def.Inputs) {
		input := def.Inputs[key]
		if input.Default == nil {
			continue
		}
		if _, reason, err := validateInputValue(key, input, input.Default); err != nil {
			return newInputValidationError(
				loopName, key, input, input.Default, InputOriginDefinition, reason, err,
			)
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

// ValidateInputLayer validates the values present in one precedence layer without requiring omissions.
func ValidateInputLayer(
	def dsl.Definition,
	loopName string,
	values map[string]any,
	origin InputOrigin,
) error {
	name := strings.TrimSpace(loopName)
	if key := firstUnknownInput(def.Inputs, values); key != "" {
		return &InputValidationError{
			Loop: name, Field: key, Origin: origin,
			Reason: InputValidationReasonUnknownInput,
			Err:    fmt.Errorf("input %q is not declared", key),
		}
	}
	for _, key := range sortedInputKeys(def.Inputs) {
		value, present := values[key]
		if !present {
			continue
		}
		input := def.Inputs[key]
		if _, reason, err := validateInputValue(key, input, value); err != nil {
			return newInputValidationError(name, key, input, value, origin, reason, err)
		}
	}
	return nil
}

// ResolveInputDefaults applies run > workspace > global > definition precedence per key.
func ResolveInputDefaults(
	def dsl.Definition,
	loopName string,
	run map[string]any,
	layers InputDefaultLayers,
) (ResolvedInputs, error) {
	name := strings.TrimSpace(loopName)
	for _, source := range []struct {
		values map[string]any
		origin InputOrigin
	}{
		{values: run, origin: InputOriginRun},
		{values: layers.Workspace, origin: InputOriginWorkspace},
		{values: layers.Global, origin: InputOriginGlobal},
	} {
		if key := firstUnknownInput(def.Inputs, source.values); key != "" {
			return ResolvedInputs{}, &InputValidationError{
				Loop: name, Field: key, Origin: source.origin,
				Reason: InputValidationReasonUnknownInput,
				Err:    fmt.Errorf("input %q is not declared", key),
			}
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
				return ResolvedInputs{}, newInputValidationError(
					name, key, input, nil, InputOriginRun, InputValidationReasonRequired,
					fmt.Errorf("input %q is required", key),
				)
			}
			continue
		}
		normalized, reason, err := validateInputValue(key, input, value)
		if err != nil {
			return ResolvedInputs{}, newInputValidationError(
				name, key, input, value, origin, reason, err,
			)
		}
		resolved.Values[key] = cloneAnyValue(normalized)
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
