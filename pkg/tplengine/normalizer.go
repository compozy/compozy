package tplengine

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

type Normalizer struct {
	TemplateEngine *TemplateEngine
}

func NewNormalizer() *Normalizer {
	return &Normalizer{
		TemplateEngine: NewEngine(FormatJSON),
	}
}

func (n *Normalizer) Normalize(exec core.Execution) map[string]any {
	return map[string]any{
		"trigger": map[string]any{
			"input": exec.GetParentInput(),
		},
		"input":  exec.GetInput(),
		"output": exec.GetOutput(),
		"env":    exec.GetEnv(),
	}
}

func (n *Normalizer) ParseExecution(exec core.Execution) error {
	if n.TemplateEngine == nil {
		return fmt.Errorf("template engine is not initialized")
	}
	normalized := n.Normalize(exec)
	for k, v := range *exec.GetParentInput() {
		parsedValue, err := n.TemplateEngine.ParseMap(v, normalized)
		if err != nil {
			return fmt.Errorf("failed to parse template in input[%s]: %w", k, err)
		}
		(*exec.GetParentInput())[k] = parsedValue
	}
	for k, v := range *exec.GetInput() {
		parsedValue, err := n.TemplateEngine.ParseMap(v, normalized)
		if err != nil {
			return fmt.Errorf("failed to parse template in input[%s]: %w", k, err)
		}
		(*exec.GetInput())[k] = parsedValue
	}
	for k, v := range *exec.GetEnv() {
		if HasTemplate(v) {
			parsed, err := n.TemplateEngine.RenderString(v, normalized)
			if err != nil {
				return fmt.Errorf("failed to parse template in env[%s]: %w", k, err)
			}
			(*exec.GetEnv())[k] = parsed
		}
	}
	return nil
}
