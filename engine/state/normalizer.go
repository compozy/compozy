package state

import (
	"fmt"

	"github.com/compozy/compozy/pkg/tplengine"
)

// -----------------------------------------------------------------------------
// Normalizer
// -----------------------------------------------------------------------------

type Normalizer struct {
	TemplateEngine *tplengine.TemplateEngine
}

func NewNormalizer(format tplengine.EngineFormat) *Normalizer {
	return &Normalizer{
		TemplateEngine: tplengine.NewEngine(format),
	}
}

func (n *Normalizer) NormalizeState(state State) map[string]any {
	return map[string]any{
		"trigger": map[string]any{
			"input": state.GetTrigger(),
		},
		"input":  state.GetInput(),
		"output": state.GetOutput(),
		"env":    state.GetEnv(),
	}
}

func (n *Normalizer) ParseTemplateValue(value any, data map[string]any) (any, error) {
	if value == nil {
		return nil, nil
	}

	switch v := value.(type) {
	case string:
		if tplengine.HasTemplate(v) {
			parsed, err := n.TemplateEngine.RenderString(v, data)
			if err != nil {
				return nil, err
			}
			return parsed, nil
		}
		return v, nil

	case map[string]any:
		result := make(map[string]any, len(v))
		for k, val := range v {
			parsedVal, err := n.ParseTemplateValue(val, data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse template in map key %s: %w", k, err)
			}
			result[k] = parsedVal
		}
		return result, nil

	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			parsedVal, err := n.ParseTemplateValue(val, data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse template in array index %d: %w", i, err)
			}
			result[i] = parsedVal
		}
		return result, nil

	default:
		// For other types (int, bool, etc.), return as is
		return v, nil
	}
}

func (n *Normalizer) ParseTemplates(state State) error {
	if n.TemplateEngine == nil {
		return fmt.Errorf("template engine is not initialized")
	}
	normalized := n.NormalizeState(state)

	for k, v := range *state.GetTrigger() {
		parsedValue, err := n.ParseTemplateValue(v, normalized)
		if err != nil {
			return fmt.Errorf("failed to parse template in trigger[%s]: %w", k, err)
		}
		(*state.GetTrigger())[k] = parsedValue
	}

	for k, v := range *state.GetInput() {
		parsedValue, err := n.ParseTemplateValue(v, normalized)
		if err != nil {
			return fmt.Errorf("failed to parse template in input[%s]: %w", k, err)
		}
		(*state.GetInput())[k] = parsedValue
	}

	for k, v := range *state.GetEnv() {
		if tplengine.HasTemplate(v) {
			parsed, err := n.TemplateEngine.RenderString(v, normalized)
			if err != nil {
				return fmt.Errorf("failed to parse template in env[%s]: %w", k, err)
			}
			(*state.GetEnv())[k] = parsed
		}
	}

	return nil
}
