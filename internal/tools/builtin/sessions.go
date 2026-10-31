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
	sessionLaterDescriptor(),
	sessionResumeLaterDescriptor(),
	sessionRenameDescriptor(),
	sessionReparentDescriptor(),
	sessionFolderListDescriptor(),
	sessionFolderCreateDescriptor(),
	sessionFolderRenameDescriptor(),
	sessionFolderDeleteDescriptor(),
	sessionFolderReorderDescriptor(),
	sessionMoveDescriptor(),
	sessionReorderDescriptor(),
	sessionCreateDescriptor(),
	sessionPromptDescriptor(),
	sessionRewindDescriptor(),
	sessionContinueDescriptor(),
	sessionForkDescriptor(),
	sessionRuntimeSetDescriptor(),
	sessionWorktreeBindingDescriptor(true),
	sessionWorktreeBindingDescriptor(false),
	sessionRuntimeClearDescriptor(),
	sessionInputsListDescriptor(),
	sessionInputsClearDescriptor(),
	sessionInputReplaceDescriptor(),
	sessionInputCancelDescriptor(),
	sessionInputPromoteDescriptor(),
	sessionStatusDescriptor(),
	nativeDescriptor(
		toolspkg.ToolIDSessionHistory,
		"session_history",
		"Session History",
		"Read one bounded page of grouped turn history for one runtime session: the newest 10 turns by default; "+
			"continue to older turns with page.next_before_sequence, or page forward from after_sequence. "+
			"To read an agent's answer without tool payloads, pass type=agent_message and limit=1. "+
			"Keep the same type filter while paging; limit counts matching turns, not bytes.",
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
	sessionSearchDescriptor(),
	sessionOutlineDescriptor(),
	nativeDescriptor(
		toolspkg.ToolIDSessionDescribe,
		"session_describe",
		"Session Describe",
		"Read a composite read-only session description with status, the newest event and turn history pages, "+
			"and their page cursors (events_page, history_page).",
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
		"Create an unbound logical user session in the effective profile: the caller session's "+
			"profile for agent calls, or the operator's resolved profile for operator calls. "+
			"Select its runtime on the first prompt.",
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
		"Send a prompt to a session. Each message costs the receiver a full turn, so send at milestones "+
			"and batch updates instead of streaming status. While the target is busy, mode chooses queue "+
			"(after its current turn), steer (into the active turn), or interrupt; omitting mode uses "+
			"session.busy_input.default_mode (steer by default). An agent never cancels another session's "+
			"turn: its interrupt is delivered as steer, including attachments, and the result "+
			"reports the mode used. Queued messages from other sessions are answered together in one turn, "+
			"and are injected into a turn that runs past session.messaging.queue_promote_after when the "+
			"target supports steering. Set supersede to replace your own older queued message to the same "+
			"target. Agent calls carry an immutable sender origin; "+
			"self-targeting is rejected. Forwarding depth is limited to 8 hops, but answering the session "+
			"that started your current turn, or replying with in_reply_to, keeps the hop. Each sender→target "+
			"pair has a rate budget (message_rate_limited when exceeded). Without notify_on_complete nobody "+
			"is woken when the receiver finishes; set it to get the receiver's final answer back as a reply "+
			"wake. To answer a message you received, pass its message id as in_reply_to. notify_on_complete "+
			"and in_reply_to require an agent session; operator-scoped calls are refused with invalid_request. "+
			"A message to a session parked in Later is queued with disposition later and does not resume it.",
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
 "board_column":{"type":"array","items":{"type":"string","enum":["needs_you","working","waiting","in_review","done"]}},
 "children":{"type":"string","enum":["include","exclude"]},
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
		"later":{
			"type":"string",
			"enum":["exclude","only","include"],
			"description":"Visibility of sessions parked in Later; excluded by default."
		},
		"include_health":{"type":"boolean"},
		"sort":{"type":"string","enum":["recent","created","name","last_activity","attention","navigator","manual"]},
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
		"mode":{"type":"string","enum":["queue","interrupt","steer"],
 "description":"How to deliver while the target is busy: queue waits for its current turn to end, ` +
	`steer joins the active turn, interrupt cancels it. An agent session's interrupt never cancels the ` +
	`target's turn: it is delivered as steer, including attachments. Without live injection it waits for the turn to end. ` +
	`Omitted means session.busy_input.default_mode (steer by default)."},
		"expected_turn_id":{"type":"string","minLength":1},
		"notify_on_complete":{"type":"boolean","default":false,
 "description":"Request a reply wake with the receiver's final answer when its turn completes. ` +
	`When omitted or false, nobody is woken when the receiver finishes. Requires an agent session."},
		"in_reply_to":{"type":"string","minLength":1,"maxLength":200,"pattern":"^\\S+$",
 "description":"Message id this message answers (shown on the received message). ` +
	`A reply keeps the hop instead of adding one. It never releases the sender's reply watch: ` +
	`a notify_on_complete wake still fires when your turn completes. Requires an agent session."},
		"supersede":{"type":"boolean","default":false,
 "description":"Cancel your own older messages still queued for the same target, in the same step ` +
	`that queues this one, so the target receives only the latest. Never touches operator input or other ` +
	`senders' messages. The result's canceled_queued_entries counts the replaced messages. Requires an agent session."},
 "wait":{"type":"boolean","default":false,"description":"Wait for completion instead of immediate acceptance."},
		"runtime":{
			"type":"object",
			"required":["provider"],
			"properties":{
				"provider":{"type":"string","minLength":1},
				"model":{"type":"string"},
				"reasoning_effort":{"type":"string","enum":["none","minimal","low","medium","high","xhigh","max","ultra"]},
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
		"sessions":{"type":"array","items":{"type":"object","properties":{"branch":{"type":"string"},
 "pull_request":{"type":["object","null"]},
 "pull_request_lookup":{"type":["object","null"]},
 "board_column":{"type":"string","enum":["needs_you","working","waiting","in_review","done"]},
 "parent_title":{"type":"string"},
 "waiting_on":{"type":"string","enum":["sessions","monitor"]},
 "children":{"type":"object"}}}},
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
				"disposition":{"type":"string","enum":["direct","steering","queued","interrupting","later"]},
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
 "target_workspace_id":{"type":"string"},
 "reply_watch":{"type":"object","required":["id","state"],
 "properties":{"id":{"type":"string"},
 "state":{"type":"string","enum":["armed","fired","delivered","abandoned"]}},"additionalProperties":false},
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
		"text":{"type":"string","minLength":1},
		"queue_generation":{"type":"integer"},
		"enqueued_at":{"type":"string","format":"date-time"},
		"runtime":{
			"type":"object",
			"required":["provider"],
			"properties":{
				"provider":{"type":"string","minLength":1},
				"model":{"type":"string"},
				"reasoning_effort":{"type":"string","enum":["none","minimal","low","medium","high","xhigh","max","ultra"]},
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
