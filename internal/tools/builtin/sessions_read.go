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
		"Read one bounded page of persisted events for one runtime session, including automatic runtime recovery: "+
			"the newest 50 events by default. Page forward with after_sequence and page.next_after_sequence, "+
			"or backward with before_sequence and page.next_before_sequence.",
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
				"branch":{"type":"string"},
 "pull_request":{"type":["object","null"]},
 "pull_request_lookup":{"type":["object","null"]},
 "board_column":{"type":"string","enum":["needs_you","working","waiting","in_review","done"]},
 "parent_title":{"type":"string"},
 "waiting_on":{"type":"string","enum":["sessions","monitor"]},
 "children":{"type":"object"},
 "id":{"type":"string"},
				"name":{"type":"string"},
				"agent_name":{"type":"string"},
				"runtime":` + sessionRuntimeOutputSchema + `,
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
	"required":["events","page"],
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
		},
		"page":` + sessionReadPageSchema + `
	},
	"additionalProperties":false
}`

// sessionReadPageSchema describes the bounded page metadata of session event and history reads.
const sessionReadPageSchema = `{
	"type":"object",
	"required":["limit","has_more"],
	"properties":{
		"limit":{"type":"integer","minimum":1},
		"has_more":{"type":"boolean"},
		"next_after_sequence":{"type":"integer","minimum":1,
			"description":"Pass as after_sequence to read the next forward page."},
		"next_before_sequence":{"type":"integer","minimum":1,
			"description":"Pass as before_sequence to read the next older page."}
	},
	"additionalProperties":false
}`

const sessionEventQueryInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string"},
		"type":{"type":"string","description":"Filter event type before reading content. ` +
	`Use agent_message to read assistant text without tool payloads."},
		"agent_name":{"type":"string"},
		"turn_id":{"type":"string"},
		"after_sequence":{"type":"integer",
			"description":"Read the oldest events, or turns that start after this sequence, ascending ` +
	`(forward paging)."},
		"before_sequence":{"type":"integer","minimum":1,
			"description":"Read the newest events or turns before this sequence (backward paging). ` +
	`Cannot be combined with after_sequence."},
		"limit":{"type":"integer",
			"description":"Page size: events for session_events (default 50), turns for session_history ` +
	`(default 10). Omitted or 0 uses the default."},
		"since":{"type":"string"},
		"archive":{"type":"string","enum":["active","archived","all"]}
	},
	"additionalProperties":false
}`
