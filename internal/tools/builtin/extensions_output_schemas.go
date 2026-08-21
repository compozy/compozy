package builtin

const extensionDiagnosticOutputSchema = `{
	"type":"object",
	"required":["id","code","category","title","message","severity","data_freshness"],
	"additionalProperties":true
}`

const extensionPayloadOutputSchema = `{
	"type":"object",
	"required":[
		"name","version","type","format","source","enabled","state",
		"network_confirmation_required","consecutive_failures","restart_backoff_ms",
		"update_available","digest_matched","daemon_running"
	],
	"properties":{
		"name":{"type":"string"},
		"version":{"type":"string"},
		"type":{"type":"string"},
		"format":{"type":"string","enum":["compozy","agent-plugin"]},
		"source":{"type":"string"},
		"enabled":{"type":"boolean"},
		"state":{"type":"string"},
		"network_confirmation_required":{"type":"boolean"},
		"consecutive_failures":{"type":"integer"},
		"restart_backoff_ms":{"type":"integer"},
		"update_available":{"type":"boolean"},
		"digest_matched":{"type":"boolean"},
		"daemon_running":{"type":"boolean"},
		"diagnostics":{"type":"array","items":` + extensionDiagnosticOutputSchema + `}
	},
	"additionalProperties":true
}`

const extensionSingleOutputSchema = `{
	"type":"object",
	"required":["extension"],
	"properties":{"extension":` + extensionPayloadOutputSchema + `},
	"additionalProperties":false
}`

const extensionListOutputSchema = `{
	"type":"object",
	"required":["extensions"],
	"properties":{"extensions":{"type":"array","items":` + extensionPayloadOutputSchema + `}},
	"additionalProperties":false
}`

const extensionValidateOutputSchema = `{
	"type":"object",
	"required":["status","format","issues"],
	"properties":{
		"status":{"type":"string","enum":["valid","invalid"]},
		"format":{"type":"string","enum":["compozy","agent-plugin"]},
		"name":{"type":"string"},
		"version":{"type":"string"},
		"would_ingest":{"type":"array","items":{
			"type":"object","required":["kind","name"],
			"properties":{
				"kind":{"type":"string"},"name":{"type":"string"},"transport":{"type":"string"}
			},
			"additionalProperties":false
		}},
		"issues":{"type":"array","items":{
			"type":"object","required":["message","severity"],"additionalProperties":true
		}}
	},
	"additionalProperties":true
}`

const extensionUpdateOutputSchema = `{
	"type":"object",
	"required":["updates"],
	"properties":{"updates":{"type":"array","items":{
		"type":"object",
		"required":["name","slug","registry","path","status"],
		"additionalProperties":true
	}}},
	"additionalProperties":false
}`

const extensionLogsOutputSchema = `{
	"type":"object",
	"required":["logs","stream_epoch"],
	"properties":{
		"stream_epoch":{"type":"string","minLength":1},
		"logs":{"type":"array","items":{
			"type":"object","required":["sequence","timestamp","message","stream_epoch"],
			"properties":{
				"sequence":{"type":"integer","minimum":1},
				"timestamp":{"type":"string","format":"date-time"},
				"message":{"type":"string"},
				"generation_hash":{"type":"string"},
				"stream_epoch":{"type":"string","minLength":1}
			},
			"additionalProperties":false
		}}
	},
	"additionalProperties":false
}`

const extensionInventoryOutputSchema = `{
	"type":"object",
	"required":["extension","format","enabled","items"],
	"properties":{
		"extension":{"type":"string"},
		"format":{"type":"string","enum":["compozy","agent-plugin"]},
		"enabled":{"type":"boolean"},
		"items":{"type":"array","items":{
			"type":"object","required":["kind","id","name","live"],
			"properties":{
				"kind":{"type":"string"},"id":{"type":"string"},
				"name":{"type":"string"},"live":{"type":"boolean"}
			},
			"additionalProperties":false
		}},
		"diagnostics":{"type":"array","items":` + extensionDiagnosticOutputSchema + `}
	},
	"additionalProperties":false
}`
