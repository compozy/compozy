package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionArchiveDescriptor(archived bool) toolspkg.Descriptor {
	toolID := toolspkg.ToolIDSessionArchive
	name := "session_archive"
	title := "Session Archive"
	description := "Archive a stopped session without deleting its history."
	keywords := []string{"archive session", "hide session"}
	if !archived {
		toolID = toolspkg.ToolIDSessionUnarchive
		name = "session_unarchive"
		title = "Session Unarchive"
		description = "Restore an archived session to normal listings."
		keywords = []string{"unarchive session", "restore session"}
	}
	descriptor := nativeDescriptor(
		toolID,
		name,
		title,
		description,
		sessionIDInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "archive"},
		keywords,
	)
	descriptor.OutputSchema = json.RawMessage(sessionCreateOutputSchema)
	return descriptor
}
