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

func (n *Normalizer) NormalizeWithContext(exec core.Execution, mainExecMap *core.MainExecutionMap) map[string]any {
	normalized := n.Normalize(exec)

	// Add execution context if provided
	if mainExecMap != nil {
		// Convert task executions to a map for template access
		tasksMap := make(map[string]any)
		if mainExecMap.Tasks != nil {
			for _, taskExec := range mainExecMap.Tasks {
				if taskExec.TaskID != "" {
					tasksMap[taskExec.TaskID] = map[string]any{
						"input":  taskExec.Input,
						"output": taskExec.Output,
						"status": taskExec.Status,
					}
				}
			}
		}
		normalized["tasks"] = tasksMap

		// Convert agent executions to a map for template access
		agentsMap := make(map[string]any)
		if mainExecMap.Agents != nil {
			for _, agentExec := range mainExecMap.Agents {
				if agentExec.AgentID != nil && *agentExec.AgentID != "" {
					agentsMap[*agentExec.AgentID] = map[string]any{
						"input":  agentExec.Input,
						"output": agentExec.Output,
						"status": agentExec.Status,
					}
				}
			}
		}
		normalized["agents"] = agentsMap

		// Convert tool executions to a map for template access
		toolsMap := make(map[string]any)
		if mainExecMap.Tools != nil {
			for _, toolExec := range mainExecMap.Tools {
				if toolExec.ToolID != nil && *toolExec.ToolID != "" {
					toolsMap[*toolExec.ToolID] = map[string]any{
						"input":  toolExec.Input,
						"output": toolExec.Output,
						"status": toolExec.Status,
					}
				}
			}
		}
		normalized["tools"] = toolsMap
	}

	return normalized
}

func (n *Normalizer) ParseExecution(exec core.Execution) error {
	return n.ParseExecutionWithContext(exec, nil)
}

func (n *Normalizer) ParseExecutionWithContext(exec core.Execution, mainExecMap *core.MainExecutionMap) error {
	if n.TemplateEngine == nil {
		return fmt.Errorf("template engine is not initialized")
	}
	normalized := n.NormalizeWithContext(exec, mainExecMap)
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
