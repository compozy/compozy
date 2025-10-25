package callworkflow

import "github.com/compozy/compozy/engine/schema"

var inputSchema = schema.Schema{
	"type": "object",
	"properties": map[string]any{
		"workflow_id": map[string]any{
			"type":        "string",
			"description": "Identifier of the workflow to execute.",
		},
		"input": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{},
			"description":          "Input data passed to the workflow.",
		},
		"initial_task_id": map[string]any{
			"type":        "string",
			"description": "Optional task ID to resume execution from.",
		},
		"timeout_ms": map[string]any{
			"type":        "integer",
			"minimum":     0,
			"description": "Optional timeout override in milliseconds for workflow execution.",
		},
	},
	"required":             []any{"workflow_id"},
	"additionalProperties": false,
}

var outputSchema = schema.Schema{
	"type":     "object",
	"required": []any{"success", "workflow_id", "workflow_exec_id"},
	"properties": map[string]any{
		"success":          map[string]any{"type": "boolean"},
		"workflow_id":      map[string]any{"type": "string"},
		"workflow_exec_id": map[string]any{"type": "string"},
		"status": map[string]any{
			"type": "string",
			"enum": []any{"SUCCESS", "FAILED", "TIMED_OUT", "CANCELED"},
		},
		"output": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{},
		},
		"duration_ms": map[string]any{"type": "integer"},
	},
	"additionalProperties": true,
}
