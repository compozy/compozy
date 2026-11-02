package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func sessionWorkLinkDescriptor(unlink bool) toolspkg.Descriptor {
	id, name, title, description := toolspkg.ToolIDSessionLink, "session_link", "Link Session Work", "Link a branch to the calling session without moving its runtime."
	if unlink {
		id, name, title, description = toolspkg.ToolIDSessionUnlink, "session_unlink", "Unlink Session Work", "Unlink work from the calling session without deleting files or pull requests."
	}
	return nativeDescriptor(id, name, title, description, `{"type":"object","required":["branch"],"properties":{"repository":{"type":"string"},"branch":{"type":"string","minLength":1},"pr_number":{"type":"integer","minimum":1}},"additionalProperties":false}`, toolspkg.RiskMutating, false, false, false, []toolspkg.ToolsetID{toolspkg.ToolsetIDSessions}, []string{"sessions", "linked work"}, []string{"link branch", "link pull request"})
}
