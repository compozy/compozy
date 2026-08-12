package builtin

const loopEnvironmentInputSchema = `{
	"type":"object",
	"required":["mode"],
	"additionalProperties":false,
	"properties":{
		"mode":{"type":"string","enum":["root","worktree","per_run","directory"]},
		"worktree_ref":{"type":"string"},
		"directory":{"type":"string"}
	}
}`

const loopConfigInputSchema = `{
	"type":"object",
	"additionalProperties":false,
	"properties":{
		"human_gate_enabled":{"type":"boolean"},
		"reattempt_strategy":{"type":"string","enum":["failed_only","full_body"]},
		"enabled_checks_json":{},
		"iteration_cap":{"type":"integer","minimum":0},
		"budget_tokens":{"type":"integer","minimum":0},
		"budget_wall_sec":{"type":"integer","minimum":0},
		"budget_on_exceeded":{"type":"string","enum":["halt","escalate"]},
		"no_progress_window":{"type":"integer","minimum":0},
		"fan_out_width":{"type":"integer","minimum":0},
		"gate_max_revisions":{"type":"integer","minimum":0},
		"runtime_defaults":{"type":"object","additionalProperties":true},
		"runtime_rules":{"type":"array","items":{"type":"object","additionalProperties":true}},
		"environment":` + loopEnvironmentInputSchema + `,
		"lifecycle":{"type":"object","additionalProperties":true}
	}
}`

// loopCreateDefinitionInputSchema documents the environment-bearing portion of
// the heterogeneous Loop DSL while leaving unrelated authored fields open.
const loopCreateDefinitionInputSchema = `{
	"type":"object",
	"additionalProperties":true,
	"properties":{
		"apiVersion":{"type":"string"},
		"kind":{"type":"string"},
		"graph":{
			"type":"object",
			"additionalProperties":true,
			"properties":{
				"nodes":{
					"type":"array",
					"items":{
						"type":"object",
						"additionalProperties":true,
						"properties":{
							"params":{
								"type":"object",
								"additionalProperties":true,
								"propertyNames":{"not":{"const":"cwd"}},
								"properties":{"environment":` + loopEnvironmentInputSchema + `}
							}
						}
					}
				}
			}
		}
	}
}`
