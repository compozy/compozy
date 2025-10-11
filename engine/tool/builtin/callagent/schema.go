package callagent

import "github.com/compozy/compozy/engine/schema"

var inputSchema = schema.Schema{
	"type": "object",
	"properties": map[string]any{
		"agent_id": map[string]any{
			"type":        "string",
			"description": "Identifier of the agent to execute. Call cp__list_agents first to discover available IDs.",
		},
		"action_id": map[string]any{
			"type":        "string",
			"description": "Optional action identifier for the agent. Required when the agent defines multiple actions.",
		},
		"prompt": map[string]any{
			"type":        "string",
			"description": "Optional natural language instructions passed to the agent when executing prompt-driven flows.",
		},
		"with": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{},
			"description":          "Structured action input payload matching the agent action schema.",
		},
		"timeout_ms": map[string]any{
			"type":        "integer",
			"minimum":     0,
			"description": "Optional timeout override in milliseconds for the agent execution.",
		},
	},
	"required":             []any{"agent_id"},
	"additionalProperties": false,
}

var outputSchema = schema.Schema{
	"type":     "object",
	"required": []any{"success", "agent_id"},
	"properties": map[string]any{
		"success":   map[string]any{"type": "boolean"},
		"agent_id":  map[string]any{"type": "string"},
		"action_id": map[string]any{"type": "string"},
		"exec_id":   map[string]any{"type": "string"},
		"response": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{},
		},
	},
	"additionalProperties": true,
}
