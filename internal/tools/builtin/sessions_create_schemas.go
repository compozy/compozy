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
		}
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
				"runtime":` + sessionRuntimeOutputSchema + `,
				"state":{"type":"string","enum":["starting","active","stopping","stopped"]}
			},
			"additionalProperties":true
		}
	},
	"additionalProperties":false
}`

// sessionRuntimeSelectionOutputSchema mirrors contract.RuntimeSelectionPayload
// and contract.PromptRuntimeSelectionPayload; omitted pointers may also be null.
const sessionRuntimeSelectionOutputSchema = `{
	"type":["object","null"],
	"required":["provider"],
	"properties":{
		"provider":{"type":"string"},
		"model":{"type":"string"},
		"reasoning_effort":{"type":"string"},
		"speed":{"type":"string","enum":["normal","fast"]},
		"acp_options":{"type":"array","items":{"type":"object"}},
		"speed_resolution":{"type":["object","null"]}
	},
	"additionalProperties":false
}`

// sessionRuntimeOutputSchema mirrors contract.SessionRuntimePayload. Every native
// tool that returns a session payload embeds this single definition so the
// advertised runtime shape cannot drift between tools.
const sessionRuntimeOutputSchema = `{
	"type":"object",
	"required":["status","selection_revision"],
	"properties":{
		"status":{"type":"string","enum":["unbound","binding","ready","reconfiguring","recovering","failed"]},
		"transition":{"type":"string","enum":[
			"","initial_bind","live_configuration","process_replacement","automatic_recovery"
		]},
		"failure":{"type":"string"},
		"generation":{"type":"integer","minimum":0},
		"recovery":{
			"type":["object","null"],
			"required":["attempt","max_attempts","generation","started_at","last_attempt_at"],
			"properties":{
				"attempt":{"type":"integer","minimum":0},
				"max_attempts":{"type":"integer","minimum":0},
				"generation":{"type":"integer","minimum":0},
				"started_at":{"type":"string","format":"date-time"},
				"last_attempt_at":{"type":"string","format":"date-time"},
				"next_attempt_at":{"type":["string","null"],"format":"date-time"},
				"last_error":{"type":"string"}
			},
			"additionalProperties":false
		},
		"selected":` + sessionRuntimeSelectionOutputSchema + `,
		"selection_revision":{"type":"integer","minimum":0},
		"effective":` + sessionRuntimeSelectionOutputSchema + `,
		"last_known":` + sessionRuntimeSelectionOutputSchema + `,
		"acp_session_id":{"type":"string"},
		"acp_caps":{"type":["object","null"]}
	},
	"additionalProperties":false
}`
