package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	sessionsSessionsKey = "sessions"
	sessionsCommandsKey = "commands"
)

const (
	sessionsEventsKey    = "events"
	sessionsHistoryKey   = "history"
	sessionsInputKey     = "input"
	sessionsInterruptKey = "interrupt"
	sessionsListKey      = "list"
	sessionsPromptKey    = "prompt"
	sessionsQueueKey     = "queue"
	sessionsRuntimeKey   = "runtime"
	sessionsStatusKey    = "status"
	sessionsSteerKey     = "steer"
)

var sessionTools = []toolspkg.Descriptor{
	sessionListDescriptor(),
	commandListDescriptor(),
	sessionArchiveDescriptor(true),
	sessionArchiveDescriptor(false),
	sessionRenameDescriptor(),
	sessionCreateDescriptor(),
	sessionPromptDescriptor(),
	sessionRewindDescriptor(),
	sessionRuntimeSetDescriptor(),
	sessionRuntimeClearDescriptor(),
	sessionInputsListDescriptor(),
	sessionInputReplaceDescriptor(),
	sessionInputCancelDescriptor(),
	sessionInputPromoteDescriptor(),
	sessionStatusDescriptor(),
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
	sessionEventsDescriptor(),
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
	descriptors := make([]toolspkg.Descriptor, len(sessionTools))
	copy(descriptors, sessionTools)
	return descriptors
}

func sessionCreateDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
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
	)
	descriptor.OutputSchema = json.RawMessage(sessionCreateOutputSchema)
	return descriptor
}

func sessionPromptDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionPrompt,
		"session_prompt",
		"Session Prompt",
		"Send a prompt to a session. While it is busy, choose queue, interrupt, or steer; "+
			"steering uses the expected active turn.",
		sessionPromptInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{
			sessionsSessionsKey,
			sessionsPromptKey,
			sessionsRuntimeKey,
			sessionsQueueKey,
			sessionsSteerKey,
			sessionsInterruptKey,
		},
		[]string{"session prompt", "queue input", "steer active turn", "interrupt session turn"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionPromptOutputSchema)
	return descriptor
}

func sessionInputsListDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionInputsList,
		"session_inputs_list",
		"Session Inputs List",
		"List durable pending input for one session in dispatch order.",
		sessionIDInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "inputs", sessionsListKey},
		[]string{"pending session input", "queued session input"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionInputsListOutputSchema)
	return descriptor
}

func sessionInputReplaceDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionInputReplace,
		"session_input_replace",
		"Session Input Replace",
		"Atomically replace one queued session input with new text and durable message identity.",
		sessionInputReplaceInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsInputKey, descriptorKeywordReplace, sessionsQueueKey},
		[]string{"replace queued input", "update session input"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionInputOutputSchema)
	return descriptor
}

func sessionInputCancelDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionInputCancel,
		"session_input_cancel",
		"Session Input Cancel",
		"Cancel one queued session input so it will not be delivered.",
		sessionInputCancelInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsInputKey, descriptorKeywordCancel, sessionsQueueKey},
		[]string{"cancel queued input", "drop session input"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionPromptOutputSchema)
	return descriptor
}

func sessionInputPromoteDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionInputPromote,
		"session_input_promote",
		"Session Input Promote",
		"Atomically replace queued input with steering for the expected active turn.",
		sessionInputPromoteInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsInputKey, "promote", sessionsSteerKey, sessionsQueueKey},
		[]string{"promote queued input", "steer queued input"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionPromptOutputSchema)
	return descriptor
}

const sessionListInputSchema = `{
	"type":"object",
	"properties":{
		"workspace":{"type":"string"},
		"worktree":{"type":"string"},
		"state":{"type":"string","enum":["starting","active","stopping","stopped"]},
		"type":{"type":"string","enum":["user","system","coordinator","spawned"]},
		"agent":{"type":"string"},
		"parent":{"type":"string"},
		"root":{"type":"string"},
		"q":{"type":"string"},
		"resumable":{"type":"boolean"},
		"archive":{"type":"string","enum":["exclude","only","include"]},
		"include_health":{"type":"boolean"},
		"sort":{"type":"string","enum":["recent","last_activity"]},
		"cursor":{"type":"string"},
		"limit":{"type":"integer","minimum":1,"maximum":100}
	},
	"additionalProperties":false
}`

const sessionPromptInputSchema = `{
	"type":"object",
	"required":["session_id","message_id","idempotency_key"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"message":{"type":"string"},
		"attachments":{"type":"array","items":{"type":"string","minLength":1}},
		"message_id":{"type":"string","minLength":1},
		"idempotency_key":{"type":"string","minLength":1},
		"mode":{"type":"string","enum":["queue","interrupt","steer"]},
		"expected_turn_id":{"type":"string","minLength":1},
		"wait":{"type":"boolean","default":false,"description":"Wait for completion instead of immediate acceptance."},
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
	"not":{
		"allOf":[
			{"not":{"required":["message"],"properties":{"message":{"type":"string","minLength":1}}}},
			{"not":{"required":["attachments"],"properties":{"attachments":{"type":"array","minItems":1}}}}
		]
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

const sessionPromptOutputSchema = `{
	"type":"object",
	"required":["prompt"],
	"properties":{
		"prompt":{
			"type":"object",
			"required":["status","delivery","message_id","idempotency_key","replayed"],
			"properties":{
				"status":{"type":"string"},
				"disposition":{"type":"string","enum":["direct","steering","queued","interrupting"]},
				"steer_delivery":{"type":"string","enum":["injected","pending_injection","interrupt_fallback"]},
				"turn_id":{"type":"string"},
				"entry_id":{"type":"string"},
				"mode":{"type":"string","enum":["queue","interrupt","steer"]},
				"delivery":{"type":"string","enum":["none","direct","after_turn","interrupt_then_prompt"]},
				"message_id":{"type":"string"},
				"idempotency_key":{"type":"string"},
				"replayed":{"type":"boolean"},
				"queue_entry_id":{"type":"string"},
				"queue_position":{"type":"integer"},
				"queue_generation":{"type":"integer"},
				"estimated_send_at":{"type":"string","format":"date-time"},
				"previous_turn_id":{"type":"string"},
				"new_turn_id":{"type":"string"},
				"canceled_queued_entries":{"type":"integer"},
				"goal":{"type":"object"}
			},
			"additionalProperties":false
		}
	},
	"additionalProperties":false
}`

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
		"inputs":{"type":"array","items":` + sessionInputPayloadSchema + `}
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
		"message_id":{"type":"string"},
		"idempotency_key":{"type":"string"},
		"target_turn_id":{"type":"string"},
		"status":{"type":"string"},
		"mode":{"type":"string","enum":["queue","interrupt","steer"]},
		"delivery":{"type":"string","enum":["none","direct","after_turn","interrupt_then_prompt"]},
		"steer_delivery":{"type":"string","enum":["injected","pending_injection","interrupt_fallback"]},
		"text":{"type":"string","minLength":1},
		"queue_generation":{"type":"integer"},
		"enqueued_at":{"type":"string","format":"date-time"},
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
		"since":{"type":"string"},
		"archive":{"type":"string","enum":["active","archived","all"]}
	},
	"additionalProperties":false
}`
