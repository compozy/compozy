package calltask

import "github.com/compozy/compozy/engine/schema"

var inputSchema = schema.Schema{
	"type": "object",
	"properties": map[string]any{
		"task_id": map[string]any{
			"type":        "string",
			"description": "Identifier of the task to execute. Must be a valid embedded task ID.",
		},
		"with": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{},
			"description":          "Structured input payload matching the task's input schema.",
		},
		"timeout_ms": map[string]any{
			"type":        "integer",
			"minimum":     0,
			"description": "Optional timeout override in milliseconds for the task execution.",
		},
	},
	"required":             []any{"task_id"},
	"additionalProperties": false,
}

var outputSchema = schema.Schema{
	"type":     "object",
	"required": []any{"success", "task_id", "exec_id"},
	"properties": map[string]any{
		"success": map[string]any{
			"type": "boolean",
		},
		"task_id": map[string]any{
			"type": "string",
		},
		"exec_id": map[string]any{
			"type": "string",
		},
		"output": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{},
		},
		"duration_ms": map[string]any{
			"type":    "integer",
			"minimum": 0,
		},
	},
	"additionalProperties": true,
}
