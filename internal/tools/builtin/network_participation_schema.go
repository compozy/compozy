package builtin

const networkParticipationChannelPattern = `^[a-z0-9][a-z0-9_-]{0,63}$`

const networkParticipationRequestSchema = `{
	"type":"object",
	"properties":{
		"mode":{"type":"string","enum":["local","live"]},
		"channel_strategy":{"type":"string","enum":["named","run","loop_run"]},
		"channel_id":{"type":"string","pattern":"` + networkParticipationChannelPattern + `"},
		"bounds":{
			"type":"object",
			"properties":{
				"max_wakes":{"type":"integer","minimum":1},
				"max_wake_wall_time":{"type":"string","minLength":1},
				"max_total_wall_time":{"type":"string","minLength":1},
				"max_input_tokens":{"type":"integer","minimum":1},
				"max_output_tokens":{"type":"integer","minimum":1},
				"max_wake_depth":{"type":"integer","minimum":1},
				"coalesce_window":{"type":"string","minLength":1}
			},
			"additionalProperties":false
		}
	},
	"oneOf":[
		{
			"properties":{"mode":{"enum":["local"]}},
			"not":{"anyOf":[
				{"required":["channel_strategy"]},
				{"required":["channel_id"]},
				{"required":["bounds"]}
			]}
		},
		{
			"properties":{
				"mode":{"enum":["live"]},
				"channel_strategy":{"enum":["named"]}
			},
			"required":["mode","channel_strategy","channel_id"]
		},
		{
			"properties":{
				"mode":{"enum":["live"]},
				"channel_strategy":{"enum":["run"]}
			},
			"required":["mode","channel_strategy"],
			"not":{"required":["channel_id"]}
		},
		{
			"properties":{
				"mode":{"enum":["live"]},
				"channel_strategy":{"enum":["loop_run"]}
			},
			"required":["mode","channel_strategy"],
			"not":{"required":["channel_id"]}
		}
	],
	"additionalProperties":false
}`
