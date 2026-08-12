package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionRenameDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionRename,
		"session_rename",
		"Session Rename",
		"Change one user session's durable display name.",
		sessionRenameInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "rename"},
		[]string{"rename session", "change session name"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionStatusOutputSchema)
	return descriptor
}

const sessionRenameInputSchema = `{
	"type":"object",
	"required":["session_id","name"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"name":{"type":"string","minLength":1,"maxLength":64}
	},
	"additionalProperties":false
}`
