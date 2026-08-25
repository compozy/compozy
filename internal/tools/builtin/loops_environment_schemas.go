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

// JSON Schema closes the public matcher/runtime shapes and rejects empty JSON
// strings. Runtime validation remains authoritative for Unicode whitespace
// because it normalizes values with strings.TrimSpace before applying the same
// selector and non-empty-runtime contract.
const loopRuntimeRuleInputSchema = `{
	"type":"object",
	"required":["match","runtime"],
	"additionalProperties":false,
	"description":"One field-merged runtime override. Match specificity is id > type + complexity > type > ` +
	`complexity; later equal-specificity rules win per non-empty runtime field.",
	"properties":{
		"match":{
			"type":"object",
			"additionalProperties":false,
			"description":"Exactly id, type, complexity, or type + complexity. The conjunction is AND and id is exclusive.",
			"properties":{
				"id":{"type":"string","minLength":1},
				"type":{"type":"string","minLength":1},
				"complexity":{"type":"string","minLength":1}
			},
			"oneOf":[
				{"required":["id"],"not":{"anyOf":[{"required":["type"]},{"required":["complexity"]}]}},
				{"required":["type"],"not":{"anyOf":[{"required":["id"]},{"required":["complexity"]}]}},
				{"required":["complexity"],"not":{"anyOf":[{"required":["id"]},{"required":["type"]}]}},
				{"required":["type","complexity"],"not":{"required":["id"]}}
			]
		},
		"runtime":{
			"type":"object",
			"additionalProperties":false,
			"description":"At least one non-empty provider, model, reasoning, or speed field.",
			"properties":{
				"provider":{"type":"string","minLength":1},
				"model":{"type":"string","minLength":1},
				"reasoning":{"type":"string","enum":["none","minimal","low","medium","high","xhigh","max"]},
				"speed":{"type":"string","enum":["normal","fast"]}
			},
			"anyOf":[
				{"required":["provider"]},
				{"required":["model"]},
				{"required":["reasoning"]},
				{"required":["speed"]}
			]
		}
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
		"runtime_rules":{"type":"array","items":` + loopRuntimeRuleInputSchema + `},
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
