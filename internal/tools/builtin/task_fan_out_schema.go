package builtin

const taskFanOutRunsInputSchema = `{
	"type":"object",
	"required":["task_id","designations"],
	"properties":{
		"task_id":{"type":"string"},
		"designations":{
			"type":"array",
			"minItems":1,
			"items":{
				"type":"object",
				"required":["brief"],
				"properties":{
					"brief":{"type":"string","minLength":1},
					"metadata":{},
					"idempotency_key":{"type":"string"}
				},
				"additionalProperties":false
			}
		},
		"idempotency_key":{"type":"string"},
		"worktree_per_run":{"type":"boolean"}
	},
	"additionalProperties":false
}`
