package callagents

import "github.com/compozy/compozy/engine/schema"

var inputSchema = schema.Schema{
	"type": "object",
	"properties": map[string]any{
		"agents": map[string]any{
			"type":     "array",
			"minItems": 1,
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{
						"type":        "string",
						"minLength":   1,
						"description": "Identifier of the agent to execute. Call cp__list_agents to discover available IDs.",
					},
					"action_id": map[string]any{
						"type":        "string",
						"minLength":   1,
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
			},
		},
	},
	"required":             []any{"agents"},
	"additionalProperties": false,
}

var outputSchema = schema.Schema{
	"type": "object",
	"properties": map[string]any{
		"results": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":     "object",
				"required": []any{"success", "agent_id", "duration_ms"},
				"properties": map[string]any{
					"success":   map[string]any{"type": "boolean"},
					"agent_id":  map[string]any{"type": "string"},
					"action_id": map[string]any{"type": "string"},
					"exec_id":   map[string]any{"type": "string"},
					"response": map[string]any{
						"type":                 "object",
						"additionalProperties": map[string]any{},
					},
					"error": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"message": map[string]any{"type": "string"},
							"code":    map[string]any{"type": "string"},
						},
						"required":             []any{"message", "code"},
						"additionalProperties": false,
					},
					"duration_ms": map[string]any{
						"type":    "integer",
						"minimum": 0,
					},
				},
				"additionalProperties": false,
			},
		},
		"total_count": map[string]any{
			"type":    "integer",
			"minimum": 0,
		},
		"success_count": map[string]any{
			"type":    "integer",
			"minimum": 0,
		},
		"failure_count": map[string]any{
			"type":    "integer",
			"minimum": 0,
		},
		"total_duration_ms": map[string]any{
			"type":    "integer",
			"minimum": 0,
		},
	},
	"required":             []any{"results", "total_count", "success_count", "failure_count", "total_duration_ms"},
	"additionalProperties": false,
}
