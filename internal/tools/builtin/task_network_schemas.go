package builtin

const taskPromoteFromThreadInputSchema = `{
	"type":"object",
	"required":["channel","thread_id","origin_message_id"],
	"properties":{
		"workspace":{"type":"string"},
		"channel":{"type":"string"},
		"thread_id":{"type":"string"},
		"origin_message_id":{"type":"string"},
		"title":{"type":"string"},
		"description":{"type":"string"},
		"priority":{"type":"string"},
		"metadata":{}
	},
	"additionalProperties":false
}`

const taskFanOutRunsInputSchema = `{
	"type":"object",
	"required":["task_id","designations"],
	"properties":{
		"task_id":{"type":"string"},
		"network_participation":` + networkParticipationRequestSchema + `,
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
