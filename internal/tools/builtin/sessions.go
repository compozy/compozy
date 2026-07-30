package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	sessionsSessionsKey = "sessions"
)

const (
	sessionsEventsKey  = "events"
	sessionsHistoryKey = "history"
	sessionsListKey    = "list"
	sessionsStatusKey  = "status"
)

var sessionTools = []toolspkg.Descriptor{
	sessionListDescriptor(),
	nativeDescriptor(
		toolspkg.ToolIDSessionCreate,
		"session_create",
		"Session Create",
		"Create an unbound logical user session. Select its runtime on the first prompt.",
		sessionCreateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, descriptorKeywordCreate},
		[]string{"create session", "new session"},
	),
	nativeDescriptor(
		toolspkg.ToolIDSessionPrompt,
		"session_prompt",
		"Session Prompt",
		"Send a prompt to a session, optionally selecting the runtime for that prompt.",
		sessionPromptInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "prompt", "runtime"},
		[]string{"session prompt", "select runtime", "switch model"},
	),
	nativeDescriptor(
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
	),
	nativeDescriptor(
		toolspkg.ToolIDSessionHistory,
		"session_history",
		"Session History",
		"Read grouped turn history for one runtime session.",
		sessionEventQueryInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsHistoryKey},
		[]string{"session history", "turn history"},
	),
	nativeDescriptor(
		toolspkg.ToolIDSessionEvents,
		"session_events",
		"Session Events",
		"Read persisted events for one runtime session.",
		sessionEventQueryInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsEventsKey},
		[]string{"session events", "event log"},
	),
	nativeDescriptor(
		toolspkg.ToolIDSessionDescribe,
		"session_describe",
		"Session Describe",
		"Read a composite read-only session description with status, events, and history.",
		sessionEventQueryInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "describe"},
		[]string{"session describe", "session detail"},
	),
}

func sessionListDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionList,
		"session_list",
		"Session List",
		"List one bounded, workspace-safe page of runtime sessions.",
		sessionListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsListKey},
		[]string{"session list", "runtime sessions"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionListOutputSchema)
	return descriptor
}

func sessionDescriptors() []toolspkg.Descriptor {
	return sessionTools
}

const sessionListInputSchema = `{
	"type":"object",
	"properties":{
		"workspace":{"type":"string"},
		"state":{"type":"string","enum":["starting","active","stopping","stopped"]},
		"type":{"type":"string","enum":["user","system","coordinator","spawned"]},
		"agent":{"type":"string"},
		"q":{"type":"string"},
		"resumable":{"type":"boolean"},
		"include_health":{"type":"boolean"},
		"sort":{"type":"string","enum":["recent","last_activity"]},
		"cursor":{"type":"string"},
		"limit":{"type":"integer","minimum":1,"maximum":100}
	},
	"additionalProperties":false
}`

const sessionCreateInputSchema = `{
	"type":"object",
	"required":["agent"],
	"properties":{
		"workspace":{"type":"string"},
		"agent":{"type":"string","minLength":1},
		"name":{"type":"string"}
	},
	"additionalProperties":false
}`

const sessionPromptInputSchema = `{
	"type":"object",
	"required":["session_id","message"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"message":{"type":"string","minLength":1},
		"runtime":{
			"type":"object",
			"required":["provider"],
			"properties":{
				"provider":{"type":"string","minLength":1},
				"model":{"type":"string"},
				"reasoning_effort":{"type":"string","enum":["none","minimal","low","medium","high","xhigh","max"]},
				"speed":{"type":"string","enum":["normal","fast"]}
			},
			"additionalProperties":false
		}
	},
	"additionalProperties":false
}`

const sessionListOutputSchema = `{
	"type":"object",
	"required":["sessions","page"],
	"properties":{
		"sessions":{"type":"array","items":{"type":"object"}},
		"page":{
			"type":"object",
			"required":["has_more","total","limit"],
			"properties":{
				"next_cursor":{"type":"string"},
				"has_more":{"type":"boolean"},
				"total":{"type":"integer","minimum":0},
				"limit":{"type":"integer","minimum":1,"maximum":100}
			},
			"additionalProperties":false
		}
	},
	"additionalProperties":false
}`

const sessionIDInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const sessionEventQueryInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string"},
		"type":{"type":"string"},
		"agent_name":{"type":"string"},
		"turn_id":{"type":"string"},
		"after_sequence":{"type":"integer"},
		"limit":{"type":"integer"},
		"since":{"type":"string"}
	},
	"additionalProperties":false
}`
