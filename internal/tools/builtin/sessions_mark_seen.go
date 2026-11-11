package builtin

import (
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionMarkSeenDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionMarkSeen,
		"session_mark_seen",
		"Session Mark as Seen",
		"Mark a session's Needs you as seen for the operator: a turn that ended early, a return from Later, "+
			"or a stopped session's failure leaves Needs you until it happens again. A session that still "+
			"waits on a permission answer, a question, a sign-in, a stop, or a work signal it cannot check is "+
			"refused with attention_requires_action; answer or resolve it instead. A newer occurrence that "+
			"arrives while marking is kept and refused with tool_conflict. A session that does not need the "+
			"operator is returned unchanged.",
		sessionMarkSeenInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "attention"},
		[]string{"mark session as seen", "dismiss needs you", "clear needs attention", "acknowledge session"},
	)
	return withResultViews(descriptor, sessionRecordCompactOutputSchema, sessionStatusOutputSchema)
}

const sessionMarkSeenInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string"}
	},
	"additionalProperties":false
}`
