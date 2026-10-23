package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionReparentDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionReparent,
		"session_reparent",
		"Session Reparent",
		"Nest a top-level user session under another user session in the same workspace and profile, "+
			"or make an adopted child top-level again. Set exactly one of parent_session_id or top_level.",
		sessionReparentInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "reparent", "lineage"},
		[]string{"nest session under parent", "make session a child", "make session top-level"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionStatusOutputSchema)
	return descriptor
}

const sessionReparentInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"parent_session_id":{"type":"string","minLength":1},
		"top_level":{"type":"boolean"}
	},
	"additionalProperties":false
}`
