package builtin

import (
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionMoveToProfileDescriptor() toolspkg.Descriptor {
	return nativeDescriptor(
		toolspkg.ToolIDSessionMoveToProfile,
		"session_move_to_profile",
		"Session Move To Profile",
		"Copy a stopped user session into another profile and archive the original. The copy is a fork "+
			"owned by target_profile (a profile name) that carries the conversation; the original keeps its "+
			"profile and history. A worktree is released and its branch checked out again in a new worktree "+
			"owned by the target profile. Refused for the calling session, a running, archived, or child "+
			"session, a session with live or unarchived descendants, or a worktree with uncommitted changes. "+
			"Archived, stopped descendants stay in the source profile. An agent session also "+
			"needs permission to write to the target profile. Not session_move, which only places a session "+
			"in a folder.",
		sessionMoveToProfileInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "profile", "move"},
		[]string{"move session to another profile", "change session profile", "transfer session to profile"},
	)
}

const sessionMoveToProfileInputSchema = `{
	"type":"object",
	"required":["session_id","target_profile"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"target_profile":{"type":"string","minLength":1}
	},
	"additionalProperties":false
}`
