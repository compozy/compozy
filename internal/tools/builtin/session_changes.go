package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func sessionChangesDescriptor() toolspkg.Descriptor {
	return nativeDescriptor(
		toolspkg.ToolIDSessionChanges,
		"session_changes",
		"Session Changes",
		"Read a session's branch or uncommitted changes and optional bounded patches. Sensitive content requires reveal.",
		`{"type":"object","properties":{"workspace":{"type":"string"},"session_id":{"type":"string","minLength":1},"scope":{"type":"string","enum":["branch","uncommitted"]},"base":{"type":"string"},"patch_paths":{"type":"array","maxItems":20,"items":{"type":"string"}},"reveal":{"type":"boolean"}},"required":["session_id"],"additionalProperties":false}`,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{"sessions", "changes"},
		[]string{"session changes", "git diff"},
	)
}
