package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionWorktreeBindingDescriptor(bind bool) toolspkg.Descriptor {
	id, name, title := toolspkg.ToolIDSessionWorktreeUnbind, "session_worktree_unbind", "Session Worktree Unbind"
	input := sessionWorktreeUnbindInputSchema
	if bind {
		id, name, title = toolspkg.ToolIDSessionWorktreeBind, "session_worktree_bind", "Session Worktree Bind"
		input = sessionWorktreeBindInputSchema
	}
	d := nativeDescriptor(
		id,
		name,
		title,
		"Change an existing idle user session's execution location in place, preserving its ID and conversation. "+
			"Bind selects a ready worktree; unbind returns to the workspace root. "+
			"Busy sessions, including self-calls during a turn, return session_busy. "+
			"Governed sessions are refused. No files are moved.",
		input,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "worktree", "binding"},
		[]string{"bind session worktree", "change working directory"},
	)
	d.OutputSchema = json.RawMessage(sessionCreateOutputSchema)
	return d
}

const sessionWorktreeBindInputSchema = `{"type":"object","required":["session_id","worktree"],"properties":{"workspace":{"type":"string"},"session_id":{"type":"string","minLength":1},"worktree":{"type":"string","minLength":1},"expected_worktree_id":{"type":"string"}},"additionalProperties":false}`

const sessionWorktreeUnbindInputSchema = `{"type":"object","required":["session_id"],"properties":{"workspace":{"type":"string"},"session_id":{"type":"string","minLength":1},"expected_worktree_id":{"type":"string"}},"additionalProperties":false}`
