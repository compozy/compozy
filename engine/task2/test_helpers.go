package task2

import (
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// TemplateEngineAdapter wraps tplengine.TemplateEngine to implement shared.TemplateEngine
type TemplateEngineAdapter struct {
	engine *tplengine.TemplateEngine
}

// NewTemplateEngineAdapter creates a new adapter
func NewTemplateEngineAdapter(engine *tplengine.TemplateEngine) shared.TemplateEngine {
	return &TemplateEngineAdapter{engine: engine}
}

// Process implements shared.TemplateEngine
func (a *TemplateEngineAdapter) Process(template string, vars map[string]any) (string, error) {
	return a.engine.RenderString(template, vars)
}

// ProcessMap implements shared.TemplateEngine
func (a *TemplateEngineAdapter) ProcessMap(data map[string]any, vars map[string]any) (map[string]any, error) {
	result, err := a.engine.ParseMap(data, vars)
	if err != nil {
		return nil, err
	}
	// Convert result to map[string]any
	if m, ok := result.(map[string]any); ok {
		return m, nil
	}
	return nil, nil
}

// ProcessSlice implements shared.TemplateEngine
func (a *TemplateEngineAdapter) ProcessSlice(slice []any, _ map[string]any) ([]any, error) {
	// Not implemented in tplengine, return as-is
	return slice, nil
}

// ProcessString implements shared.TemplateEngine
func (a *TemplateEngineAdapter) ProcessString(
	templateStr string,
	context map[string]any,
) (*shared.ProcessResult, error) {
	result, err := a.engine.ProcessString(templateStr, context)
	if err != nil {
		return nil, err
	}
	return &shared.ProcessResult{
		Text: result.Text,
		YAML: result.YAML,
		JSON: result.JSON,
	}, nil
}

// ParseMapWithFilter implements shared.TemplateEngine
func (a *TemplateEngineAdapter) ParseMapWithFilter(
	data map[string]any,
	vars map[string]any,
	filter func(string) bool,
) (map[string]any, error) {
	result, err := a.engine.ParseMapWithFilter(data, vars, filter)
	if err != nil {
		return nil, err
	}
	// Convert result to map[string]any
	if m, ok := result.(map[string]any); ok {
		return m, nil
	}
	return nil, nil
}

// ParseMap implements shared.TemplateEngine
func (a *TemplateEngineAdapter) ParseMap(data map[string]any, vars map[string]any) (map[string]any, error) {
	result, err := a.engine.ParseMap(data, vars)
	if err != nil {
		return nil, err
	}
	// Convert result to map[string]any
	if m, ok := result.(map[string]any); ok {
		return m, nil
	}
	return nil, nil
}

// ParseValue implements shared.TemplateEngine
func (a *TemplateEngineAdapter) ParseValue(value any, vars map[string]any) (any, error) {
	return a.engine.ParseMap(value, vars)
}
