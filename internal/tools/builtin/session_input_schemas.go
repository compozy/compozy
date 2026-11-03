package builtin

const sessionInputReplaceInputSchema = `{
	"type":"object",
	"required":["session_id","queue_entry_id","text","message_id","idempotency_key"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"queue_entry_id":{"type":"string","minLength":1},
		"text":{"type":"string","minLength":1},
		"message_id":{"type":"string","minLength":1},
		"idempotency_key":{"type":"string","minLength":1}
	},
	"additionalProperties":false
}`

const sessionInputCancelInputSchema = `{
	"type":"object",
	"required":["session_id","queue_entry_id"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"queue_entry_id":{"type":"string","minLength":1}
	},
	"additionalProperties":false
}`

const sessionInputPromoteInputSchema = `{
	"type":"object",
	"required":["session_id","queue_entry_id","text","message_id","idempotency_key"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"queue_entry_id":{"type":"string","minLength":1},
		"text":{"type":"string","minLength":1},
		"message_id":{"type":"string","minLength":1},
		"idempotency_key":{"type":"string","minLength":1},
		"expected_turn_id":{"type":"string","minLength":1}
	},
	"additionalProperties":false
}`

const sessionInputsListOutputSchema = `{
	"type":"object",
	"required":["inputs"],
	"properties":{
		"inputs":{"type":"array","items":` + sessionInputPayloadSchema + `},
		"queue":{
			"type":"object",
			"required":["entries","cap"],
			"properties":{
				"entries":{"type":"integer","minimum":0},
				"cap":{"type":"integer","minimum":0}
			},
			"additionalProperties":false
		}
	},
	"additionalProperties":false
}`

const sessionInputOutputSchema = `{
	"type":"object",
	"required":["input"],
	"properties":{
		"input":` + sessionInputPayloadSchema + `
	},
	"additionalProperties":false
}`

const sessionInputPayloadSchema = `{
	"type":"object",
	"required":["id","session_id","status","mode","delivery","text","queue_generation","enqueued_at"],
	"properties":{
		"id":{"type":"string","minLength":1},
		"session_id":{"type":"string","minLength":1},
		"origin":` + sessionPromptOriginSchema + `,
		"owner_kind":{"type":"string"},
		"owner_id":{"type":"string"},
		"message_id":{"type":"string"},
		"idempotency_key":{"type":"string"},
		"target_turn_id":{"type":"string"},
		"status":{"type":"string","enum":["queued","dispatching","sent","failed","canceled"]},
		"mode":{"type":"string","enum":["queue","interrupt","steer"]},
		"delivery":{"type":"string","enum":["none","direct","after_turn","interrupt_then_prompt"]},
		"steer_delivery":{"type":"string","enum":["injected","pending_injection","interrupt_fallback"]},
		"text":{"type":"string"},
		"queue_generation":{"type":"integer"},
		"enqueued_at":{"type":"string","format":"date-time"},
		"coalesced_wakes":{"type":"integer","minimum":0},
		"held":{"type":"boolean"},
		"skill_invocations":{"type":"array","items":{
			"type":"object",
			"required":["command_id","token","name","source","start","end"],
			"properties":{
				"command_id":{"type":"string"},
				"token":{"type":"string"},
				"name":{"type":"string"},
				"source":{
					"type":"object",
					"required":["kind","scope"],
					"properties":{
						"kind":{"type":"string"},
						"id":{"type":"string"},
						"key":{"type":"string"},
						"scope":{"type":"string"},
						"origin":{"type":"string"}
					},
					"additionalProperties":false
				},
				"start":{"type":"integer"},
				"end":{"type":"integer"}
			},
			"additionalProperties":false
		}},
		"mentions":{"type":"array","items":{
			"type":"object",
			"required":["kind","token","target","label","start","end"],
			"properties":{
				"kind":{"type":"string","enum":["file","folder","session"]},
				"token":{"type":"string"},
				"target":{"type":"string"},
				"label":{"type":"string"},
				"start":{"type":"integer"},
				"end":{"type":"integer"}
			},
			"additionalProperties":false
		}},
		"attachments":{"type":"array","items":{
			"type":"object",
			"required":["id","name","mime_type","bytes","sha256","kind","width","height"],
			"properties":{
				"id":{"type":"string"},
				"name":{"type":"string"},
				"mime_type":{"type":"string"},
				"bytes":{"type":"integer"},
				"sha256":{"type":"string"},
				"kind":{"type":"string"},
				"width":{"type":"integer"},
				"height":{"type":"integer"}
			},
			"additionalProperties":false
		}},
		"runtime":{
			"type":"object",
			"required":["provider"],
			"properties":{
				"provider":{"type":"string","minLength":1},
				"model":{"type":"string"},
				"reasoning_effort":{"type":"string","enum":["none","minimal","low","medium","high","xhigh","max","ultra"]},
				"speed":{"type":"string","enum":["normal","fast"]},
				"acp_options":{"type":"array","items":{
					"type":"object",
					"required":["id"],
					"properties":{
						"id":{"type":"string"},
						"value_id":{"type":"string"},
						"bool_value":{"type":"boolean"}
					},
					"additionalProperties":false
				}}
			},
			"additionalProperties":false
		}
	},
	"additionalProperties":false
}`
