package builtin

const loopNodesInputSchema = `{
	"type":"object",
	"required":["state"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"state":{"type":"string","enum":["waiting","quarantined","attention","retrying"]},
		"loop_name":{"type":"string"},
		"run_id":{"type":"string"},
		"cursor":{"type":"string"},
		"limit":{"type":"integer","minimum":1,"maximum":200}
	}
}`

const loopNodeMutationInputSchema = `{
	"type":"object",
	"required":["run_id","node_id"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"run_id":{"type":"string","minLength":1},
		"node_id":{"type":"string","minLength":1},
		"reason":{"type":"string"}
	}
}`

const loopNodePauseInputSchema = `{
	"type":"object",
	"required":["run_id","node_id","mode"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"run_id":{"type":"string","minLength":1},
		"node_id":{"type":"string","minLength":1},
		"mode":{"type":"string","enum":["drain","cancel"]},
		"reason":{"type":"string"}
	}
}`

const loopNodeResumeInputSchema = `{
	"type":"object",
	"required":["run_id","node_id","mode"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"run_id":{"type":"string","minLength":1},
		"node_id":{"type":"string","minLength":1},
		"mode":{"type":"string","enum":["plain","reset_attempts","immediate"]},
		"item_index":{"type":"integer","minimum":0},
		"payload":{}
	}
}`

const loopGenerationOutputSchema = `{
	"type":"object",
	"required":["node_id","status"],
	"properties":{
		"generation":{"type":"integer","minimum":1},
		"node_id":{"type":"string","minLength":1},
		"item_index":{"type":"integer","minimum":0},
		"status":{"type":"string"},
		"output_ref":{"type":"string"},
		"task_run_id":{"type":"string"},
		"child_loop_run_id":{"type":"string"},
		"resolved_runtime":{"type":"object"},
		"attempt":{"type":"integer","minimum":1},
		"next_attempt_at":{"type":"string","format":"date-time"},
		"failure_class":{"type":"string"},
		"disposition":{"type":"string"}
	},
	"additionalProperties":false
}`

const loopMutationOutputSchema = `{
	"type":"object",
	"required":["ok","run_id"],
	"properties":{
		"ok":{"type":"boolean"},
		"run_id":{"type":"string","minLength":1},
		"node_id":{"type":"string"},
		"status":{"type":"string"},
		"provenance":{
			"type":"object",
			"required":["actor_kind","actor_id","requested_at"],
			"properties":{
				"actor_kind":{"type":"string"},
				"actor_id":{"type":"string"},
				"reason":{"type":"string"},
				"rule_id":{"type":"string"},
				"requested_at":{"type":"string","format":"date-time"}
			},
			"additionalProperties":false
		},
		"control":{"type":"object","additionalProperties":true}
	},
	"additionalProperties":false
}`

const loopNodesOutputSchema = `{
	"type":"object",
	"required":["items"],
	"properties":{
		"items":{
			"type":"array",
			"items":{
				"type":"object",
				"required":["state","loop_run_id","loop_name","generation","node_id","item_index","state_at"],
				"properties":{
					"state":{"type":"string","enum":["waiting","quarantined","attention","retrying"]},
					"loop_run_id":{"type":"string","minLength":1},
					"loop_name":{"type":"string","minLength":1},
					"generation":{"type":"integer","minimum":0},
					"node_id":{"type":"string","minLength":1},
					"item_index":{"type":"integer","minimum":0},
					"state_at":{"type":"string","format":"date-time"},
					"wait":{"type":"object","additionalProperties":true},
					"control":{"type":"object","additionalProperties":true},
					"output":` + loopGenerationOutputSchema + `
				},
				"additionalProperties":false
			}
		},
		"next_cursor":{"type":"string"}
	},
	"additionalProperties":false
}`
