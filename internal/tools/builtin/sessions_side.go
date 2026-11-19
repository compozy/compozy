package builtin

import (
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionSideOpenDescriptor() toolspkg.Descriptor {
	d := nativeDescriptor(
		toolspkg.ToolIDSessionSideOpen,
		"session_side_open",
		"Session Side Open",
		"Open a side chat with the source conversation as frozen background, including work in progress. "+
			"The source is unchanged. Retry the same idempotency_key to recover the same side chat. "+
			"Read its answer with session_history or session_wait.",
		`{"type":"object","required":["session_id","message","idempotency_key"],"properties":{"workspace":{"type":"string"},"session_id":{"type":"string","minLength":1},"message":{"type":"string","minLength":1},"name":{"type":"string"},"idempotency_key":{"type":"string","minLength":1}},"additionalProperties":false}`,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "side"},
		[]string{"ask side question", "open side chat"},
	)
	d.OutputSchema = []byte(
		`{"type":"object","required":["side_session_id","source_session_id","seed","source_turn_in_progress","replayed"],"properties":{"side_session_id":{"type":"string"},"source_session_id":{"type":"string"},"seed":{"type":"string","enum":["replay","native_fork"]},"source_turn_in_progress":{"type":"boolean"},"replayed":{"type":"boolean"}},"additionalProperties":false}`,
	)
	return d
}

func sessionSideListDescriptor() toolspkg.Descriptor {
	d := nativeDescriptor(
		toolspkg.ToolIDSessionSideList,
		"session_side_list",
		"Session Side List",
		"List all side chats attached to a session, newest updated first. "+
			"Include closed side chats with include_archived. An archived source can still be read.",
		`{"type":"object","required":["session_id"],"properties":{"workspace":{"type":"string"},"session_id":{"type":"string","minLength":1},"include_archived":{"type":"boolean"}},"additionalProperties":false}`,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "side"},
		[]string{"list side chats", "side questions"},
	)
	d.OutputSchema = []byte(
		`{"type":"object","required":["sides"],"properties":{"sides":{"type":"array","items":{"type":"object","required":["id","name","state","prompting","pending_permission_count","pending_clarify_count","archived_at","created_at","updated_at"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"state":{"type":"string"},"prompting":{"type":"boolean"},"pending_permission_count":{"type":"integer"},"pending_clarify_count":{"type":"integer"},"archived_at":{"type":["string","null"],"format":"date-time"},"created_at":{"type":"string","format":"date-time"},"updated_at":{"type":"string","format":"date-time"}},"additionalProperties":false}}},"additionalProperties":false}`,
	)
	return d
}
