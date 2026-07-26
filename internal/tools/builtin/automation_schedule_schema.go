package builtin

const automationScheduleInputSchema = `{
	"type":"object",
	"required":["mode"],
	"properties":{
		"mode":{"type":"string","enum":["cron","every","at"]},
		"expr":{"type":"string"},
		"interval":{"type":"string"},
		"time":{"type":"string"},
		"catch_up_policy":{
			"type":"string",
			"enum":["skip_missed","coalesce","replay","run_once_on_catchup"]
		},
		"misfire_grace_seconds":{"type":"integer","minimum":0}
	},
	"additionalProperties":false
}`
