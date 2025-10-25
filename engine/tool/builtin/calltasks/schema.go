package calltasks

import "github.com/compozy/compozy/engine/schema"

var inputSchema = schema.Schema{
	"type": "object",
	"properties": map[string]any{
		"tasks": map[string]any{
			"type":     "array",
			"minItems": 1,
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string"},
					"with": map[string]any{
						"type":                 "object",
						"additionalProperties": map[string]any{},
					},
					"timeout_ms": map[string]any{
						"type":    "integer",
						"minimum": 0,
					},
				},
				"required":             []any{"task_id"},
				"additionalProperties": false,
			},
			"description": "Ordered list of task execution requests.",
		},
	},
	"required":             []any{"tasks"},
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
					"success": map[string]any{"type": "boolean"},
					"task_id": map[string]any{"type": "string"},
					"exec_id": map[string]any{"type": "string"},
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
