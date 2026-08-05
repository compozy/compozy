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
	descriptor.OutputSchema = json.RawMessage(sessionStatusOutputSchema)
	return descriptor
}

const sessionStatusOutputSchema = `{
	"type":"object",
	"required":["session"],
	"properties":{
		"session":{
			"type":"object",
			"required":["id","agent_name","runtime","state","archived_at","created_at","updated_at"],
			"properties":{
				"id":{"type":"string"},
				"name":{"type":"string"},
				"agent_name":{"type":"string"},
				"runtime":{
					"type":"object",
					"required":["status"],
					"properties":{
						"status":{"type":"string","enum":["unbound","binding","ready","reconfiguring","failed"]},
						"transition":{"type":"string","enum":["","initial_bind","live_configuration","process_replacement"]},
						"failure":{"type":"string"},
						"selected":{"type":"object"},
						"selection_revision":{"type":"integer","minimum":0},
						"effective":{"type":"object"},
						"acp_session_id":{"type":"string"},
						"acp_caps":{"type":"object"}
					},
					"additionalProperties":false
				},
				"workspace_id":{"type":"string"},
				"workspace_path":{"type":"string"},
				"state":{"type":"string","enum":["starting","active","stopping","stopped"]},
				"archived_at":{"type":["string","null"],"format":"date-time"},
				"stop_reason":{"type":"string"},
				"stop_detail":{"type":"string"},
				"created_at":{"type":"string","format":"date-time"},
				"updated_at":{"type":"string","format":"date-time"}
			},
			"additionalProperties":true
		}
	},
	"additionalProperties":false
}`
