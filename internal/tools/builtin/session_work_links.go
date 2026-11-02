package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

const sessionWorkLinkInputSchema = `{
  "type": "object",
  "required": ["branch"],
  "properties": {
    "repository": {"type": "string"},
    "branch": {"type": "string", "minLength": 1},
    "pr_number": {"type": "integer", "minimum": 1}
  },
  "additionalProperties": false
}`

func sessionWorkLinkDescriptor(unlink bool) toolspkg.Descriptor {
	id, name, title := toolspkg.ToolIDSessionLink, "session_link", "Link Session Work"
	description := "Link a branch to the calling session without moving its runtime."
	if unlink {
		id, name, title = toolspkg.ToolIDSessionUnlink, "session_unlink", "Unlink Session Work"
		description = "Unlink work from the calling session without deleting files or pull requests."
	}
	return nativeDescriptor(
		id,
		name,
		title,
		description,
		sessionWorkLinkInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{"sessions", "linked work"},
		[]string{"link branch", "link pull request"},
	)
}
