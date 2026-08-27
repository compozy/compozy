package builtin

const nativeCallRecordOutputSchema = `{
	"type":"object",
	"required":[
		"call_id","profile_id","scope","caller","actor","root_session_id","depth","state",
		"result_bytes","result_budget_bytes","result_overflow","strict","repair_attempts","replayed",
		"created_at","updated_at"
	],
	"properties":{
		"call_id":{"type":"string"},"profile_id":{"type":"string"},
		"scope":{"type":"string","enum":["global","workspace"]},"workspace_id":{"type":"string"},
		"caller":{"type":"object"},"actor":{"type":"object"},"agent":{"type":"string"},
		"child_session_id":{"type":"string"},"parent_session_id":{"type":"string"},"root_session_id":{"type":"string"},
		"depth":{"type":"integer"},
		"state":{"type":"string","enum":[
			"queued","running","completed","invalid-result","completed-without-result",
			"failed","canceled","timeout","expired"
		]},
		"verdict":{"type":"string","enum":["returned","extracted","repaired"]},
		"expect_digest":{"type":"string"},"result_bytes":{"type":"integer"},"result_budget_bytes":{"type":"integer"},
		"result_overflow":{"type":"string","enum":["store","reject"]},"strict":{"type":"boolean"},
		"failure_code":{"type":"string"},"repair_attempts":{"type":"integer"},"replayed":{"type":"boolean"},
		"deadline_at":{"type":"string","format":"date-time"},"created_at":{"type":"string","format":"date-time"},
		"started_at":{"type":"string","format":"date-time"},"settled_at":{"type":"string","format":"date-time"},
		"updated_at":{"type":"string","format":"date-time"},"result_preview":{"type":"object"}
	},
	"additionalProperties":false
}`

const callCreateOutputSchema = `{
	"oneOf":[
		` + nativeCallRecordOutputSchema + `,
		{
			"type":"object","required":["items"],
			"properties":{"items":{"type":"array","items":{
				"type":"object",
				"properties":{
					"call":` + nativeCallRecordOutputSchema + `,
					"error":{"type":"object","required":["code","message"],"properties":{
						"code":{"type":"string"},"message":{"type":"string"},
						"available":{"type":"array","items":{"type":"object"}},
						"widening":{"type":"array","items":{"type":"string"}},
						"original_id":{"type":"string"},"reset_at":{"type":"string"},
						"expired_at":{"type":"string"},"suggestion":{"type":"string"}
					},"additionalProperties":false}
				},
				"oneOf":[
					{"required":["call"],"not":{"required":["error"]}},
					{"required":["error"],"not":{"required":["call"]}}
				],
				"additionalProperties":false
			}}},
			"additionalProperties":false
		}
	]
}`

const callReturnOutputSchema = `{
	"type":"object","required":["call_id","state"],
	"properties":{
		"call_id":{"type":"string"},
		"state":{"type":"string","enum":[
			"running","completed","invalid-result","completed-without-result",
			"failed","canceled","timeout","expired"
		]},
		"repair_prompt":{"type":"string"},"issues":{"type":"array","items":{"type":"object"}}
	},
	"additionalProperties":false
}`

const callAwaitOutputSchema = `{
	"type":"object","required":["settled","pending","outcome","resume","clamped_timeout_ms"],
	"properties":{
		"settled":{"type":"array","items":` + nativeCallRecordOutputSchema + `},
		"pending":{"type":"array","items":{"type":"string"}},
		"outcome":{"type":"string","enum":["complete","partial","timeout"]},
		"resume":{"type":"string"},"clamped_timeout_ms":{"type":"integer","minimum":0}
	},
	"additionalProperties":false
}`

const callResultOutputSchema = `{
	"type":"object","required":["call_id","result"],
	"properties":{"call_id":{"type":"string"},"result":{"type":"object"}},
	"additionalProperties":false
}`

const agentMessageOutputSchema = `{
	"type":"object","required":["message_id","delivery"],
	"properties":{
		"message_id":{"type":"string"},
		"delivery":{"type":"string","enum":["queued","delivered-into-turn","woke","failed"]}
	},
	"additionalProperties":false
}`

const callPublishOutputSchema = `{
	"type":"object","required":["network_message_id","published"],
	"properties":{"network_message_id":{"type":"string"},"published":{"type":"boolean"}},
	"additionalProperties":false
}`
