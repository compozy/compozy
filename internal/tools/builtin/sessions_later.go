package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionLaterDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionLater,
		"session_later",
		"Session Later",
		"Move a session to Later: stop its active turn and runtime, cancel its monitors and pending wakes, "+
			"terminate its processes and terminals, and park it with its child sessions. History and the "+
			"worktree are kept. An optional return_at brings it back into Needs you with a notification. "+
			"Messages sent to a parked session are queued and do not resume it.",
		sessionLaterInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "later"},
		[]string{"move session to later", "park session", "snooze session", "remind me later"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionStatusOutputSchema)
	return descriptor
}

func sessionResumeLaterDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionResumeLater,
		"session_resume_later",
		"Session Back from Later",
		"Return a parked session to the normal flow without starting it. Its children stay parked "+
			"unless with_children is true.",
		sessionResumeLaterInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "later"},
		[]string{"back from later", "unpark session", "resume later session"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionStatusOutputSchema)
	return descriptor
}

const sessionLaterInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string"},
		"reason":{
			"type":"string",
			"maxLength":1000,
			"description":"Optional Markdown note shown on the Later card (bold, italic, inline code, lists, links)."
		},
		"return_at":{
			"type":"string",
			"format":"date-time",
			"description":"Optional RFC 3339 time that brings the session back into Needs you; a past time returns it at once."
		}
	},
	"additionalProperties":false
}`

const sessionResumeLaterInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string"},
		"with_children":{"type":"boolean","description":"Also take parked child sessions out of Later."}
	},
	"additionalProperties":false
}`
