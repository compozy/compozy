package callworkflows

import "github.com/compozy/compozy/engine/schema"

var inputSchema = schema.Schema{
	"type": "object",
	"properties": map[string]any{
		"workflows": map[string]any{
			"type":     "array",
			"minItems": 1,
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"workflow_id": map[string]any{"type": "string"},
					"input": map[string]any{
						"type":                 "object",
						"additionalProperties": map[string]any{},
					},
					"initial_task_id": map[string]any{"type": "string"},
					"timeout_ms": map[string]any{
						"type":    "integer",
						"minimum": 0,
					},
				},
				"required":             []any{"workflow_id"},
				"additionalProperties": false,
			},
			"description": "Ordered list of workflow execution requests.",
		},
	},
	"required":             []any{"workflows"},
	"additionalProperties": false,
}

var outputSchema = schema.Schema{
	"type":     "object",
	"required": []any{"results", "total_count", "success_count", "failure_count", "total_duration_ms"},
	"properties": map[string]any{
		"results": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"success":          map[string]any{"type": "boolean"},
					"workflow_id":      map[string]any{"type": "string"},
					"workflow_exec_id": map[string]any{"type": "string"},
					"status":           map[string]any{"type": "string"},
					"output": map[string]any{
						"type":                 "object",
						"additionalProperties": map[string]any{},
					},
					"error": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"message": map[string]any{"type": "string"},
							"code":    map[string]any{"type": "string"},
						},
					},
					"duration_ms": map[string]any{"type": "integer"},
				},
			},
		},
		"total_count":       map[string]any{"type": "integer"},
		"success_count":     map[string]any{"type": "integer"},
		"failure_count":     map[string]any{"type": "integer"},
		"total_duration_ms": map[string]any{"type": "integer"},
	},
	"additionalProperties": false,
}
