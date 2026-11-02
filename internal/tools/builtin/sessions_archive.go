package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionArchiveDescriptor(archived bool) toolspkg.Descriptor {
	toolID := toolspkg.ToolIDSessionArchive
	name := "session_archive"
	title := "Session Archive"
	description := "Archive a stopped session without deleting its history. Set stop=true to stop a live " +
		"session through the normal stop path (canceling any active turn) and archive it in one call. " +
		"Set with_children=true to also archive its child sessions, recursively, in one transaction."
	keywords := []string{"archive session", "hide session", "stop and archive session"}
	inputSchema := sessionArchiveInputSchema
	if !archived {
		inputSchema = sessionIDInputSchema
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
		inputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "archive"},
		keywords,
	)
	descriptor.OutputSchema = json.RawMessage(sessionStatusOutputSchema)
	return descriptor
}

const sessionArchiveInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string"},
		"processes":` + sessionProcessPolicySchema + `,
 "stop":{
			"type":"boolean",
			"description":"Stop a live session first; a failed stop leaves it unarchived."
		},
		"with_children":{
			"type":"boolean",
			"description":"Also archive child sessions, recursively; a tree containing the caller is refused."
		}
	},
	"additionalProperties":false
}`
