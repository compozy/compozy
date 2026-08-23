package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionStatusDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionStatus,
		"session_status",
		"Session Status",
		"Read one runtime session snapshot through the existing session status surface.",
		sessionIDInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsStatusKey},
		[]string{"session status", "session snapshot"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionStatusOutputSchema)
	return descriptor
}

func sessionEventsDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionEvents,
		"session_events",
		"Session Events",
		"Read persisted events for one runtime session, including automatic runtime recovery.",
		sessionEventQueryInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsEventsKey},
		[]string{"session events", "event log", "runtime recovery"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionEventsOutputSchema)
	return descriptor
}

const sessionStatusOutputSchema = `{
	"type":"object",
	"required":["session"],
	"properties":{
		"session":{
			"type":"object",
			"required":["id","agent_name","runtime","state","archived_at","created_at","updated_at"],
			"properties":{
				"id":{"type":"string"},
				"name":{"type":"string"},
				"agent_name":{"type":"string"},
				"runtime":{
					"type":"object",
					"required":["status","generation"],
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
								"attempt":{"type":"integer","minimum":1},
								"max_attempts":{"type":"integer","minimum":1},
								"generation":{"type":"integer","minimum":1},
								"started_at":{"type":"string","format":"date-time"},
								"last_attempt_at":{"type":"string","format":"date-time"},
								"next_attempt_at":{"type":["string","null"],"format":"date-time"},
								"last_error":{"type":"string"}
							},
							"additionalProperties":false
						},
						"selected":{"type":"object"},
						"selection_revision":{"type":"integer","minimum":0},
						"effective":{"type":"object"},
						"acp_session_id":{"type":"string"},
						"acp_caps":{"type":"object"}
					},
					"additionalProperties":false
				},
				"workspace_id":{"type":"string"},
				"workspace_path":{"type":"string"},
				"state":{"type":"string","enum":["starting","active","stopping","stopped"]},
				"archived_at":{"type":["string","null"],"format":"date-time"},
				"stop_reason":{"type":"string"},
				"stop_detail":{"type":"string"},
				"created_at":{"type":"string","format":"date-time"},
				"updated_at":{"type":"string","format":"date-time"}
			},
			"additionalProperties":true
		}
	},
	"additionalProperties":false
}`

const sessionEventsOutputSchema = `{
	"type":"object",
	"required":["events"],
	"properties":{
		"events":{
			"type":"array",
			"items":{
				"type":"object",
				"required":["id","session_id","sequence","turn_id","type","agent_name","spawn_depth","content","timestamp"],
				"properties":{
					"id":{"type":"string"},
					"session_id":{"type":"string"},
					"sequence":{"type":"integer","minimum":0},
					"turn_id":{"type":"string"},
					"type":{
						"type":"string",
						"examples":["runtime_recovery_started","runtime_recovery_succeeded","runtime_recovery_exhausted"]
					},
					"agent_name":{"type":"string"},
					"workspace_id":{"type":"string"},
					"workspace_path":{"type":"string"},
					"spawn_depth":{"type":"integer","minimum":0},
					"content":{},
					"timestamp":{"type":"string","format":"date-time"}
				},
				"additionalProperties":true
			}
		}
	},
	"additionalProperties":false
}`
