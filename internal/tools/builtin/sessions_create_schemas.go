package builtin

var sessionCreateInputSchema = `{
	"type":"object",
	"required":["agent"],
	"properties":{
		"workspace":{"type":"string"},
		"agent":` + agentNameInputSchema + `,
		"name":{"type":"string"},
		"worktree":{"type":"string","minLength":1},
		"new_worktree":{
			"type":"object",
			"properties":{"name":{"type":"string"}},
			"additionalProperties":false
		},
		"network_participation":` + networkParticipationRequestSchema + `
	},
	"additionalProperties":false
}`

const sessionCreateOutputSchema = `{
	"type":"object",
	"required":["session"],
	"properties":{
		"session":{
			"type":"object",
			"required":["id","agent_name","runtime","state"],
			"properties":{
				"id":{"type":"string"},
				"agent_name":{"type":"string"},
				"runtime":{
					"type":"object",
					"required":["status"],
					"properties":{
						"status":{"type":"string","enum":["unbound","binding","ready","reconfiguring","failed"]},
						"transition":{"type":"string","enum":["","initial_bind","live_configuration","process_replacement"]},
						"failure":{"type":"string"},
						"selected":{"type":"object"},
						"selection_revision":{"type":"integer","minimum":0},
						"effective":{"type":"object"},
						"acp_session_id":{"type":"string"},
						"acp_caps":{"type":"object"}
					},
					"additionalProperties":false
				},
				"state":{"type":"string","enum":["starting","active","stopping","stopped"]}
			},
			"additionalProperties":true
		}
	},
	"additionalProperties":false
}`
