package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const sessionsFolderKey = "folder"

func sessionFolderListDescriptor() toolspkg.Descriptor {
	return sessionLayoutDescriptor(
		toolspkg.ToolIDSessionFolderList, "session_folder_list", "Session Folder List",
		"List the caller profile's session folders in one workspace, in manual order, with unarchived session counts.",
		sessionLayoutWorkspaceInputSchema, toolspkg.RiskRead, true,
		[]string{"list session folders", "session groups"},
	)
}

func sessionFolderCreateDescriptor() toolspkg.Descriptor {
	return sessionLayoutDescriptor(
		toolspkg.ToolIDSessionFolderCreate, "session_folder_create", "Session Folder Create",
		"Create a named session folder at the top of the workspace folder order.",
		sessionFolderCreateInputSchema, toolspkg.RiskMutating, false,
		[]string{"create session folder", "new folder", "group sessions"},
	)
}

func sessionFolderRenameDescriptor() toolspkg.Descriptor {
	return sessionLayoutDescriptor(
		toolspkg.ToolIDSessionFolderRename, "session_folder_rename", "Session Folder Rename",
		"Rename one session folder.",
		sessionFolderRenameInputSchema, toolspkg.RiskMutating, false,
		[]string{"rename session folder"},
	)
}

func sessionFolderDeleteDescriptor() toolspkg.Descriptor {
	return sessionLayoutDescriptor(
		toolspkg.ToolIDSessionFolderDelete, "session_folder_delete", "Session Folder Delete",
		"Delete one session folder. Its sessions return to the workspace root; no session is deleted or archived.",
		sessionFolderIDInputSchema, toolspkg.RiskMutating, false,
		[]string{"delete session folder", "remove folder", "ungroup sessions"},
	)
}

func sessionFolderReorderDescriptor() toolspkg.Descriptor {
	return sessionLayoutDescriptor(
		toolspkg.ToolIDSessionFolderReorder, "session_folder_reorder", "Session Folder Reorder",
		"Set the complete session folder order; folder_ids must list every folder exactly once.",
		sessionFolderReorderInputSchema, toolspkg.RiskMutating, false,
		[]string{"reorder session folders", "sort folders"},
	)
}

func sessionMoveDescriptor() toolspkg.Descriptor {
	return sessionLayoutDescriptor(
		toolspkg.ToolIDSessionMove, "session_move", "Session Move",
		"Move one root session to the end of a session folder, "+
			"or back to the workspace root when folder_id is omitted.",
		sessionMoveInputSchema, toolspkg.RiskMutating, false,
		[]string{"move session to folder", "remove session from folder"},
	)
}

func sessionReorderDescriptor() toolspkg.Descriptor {
	return sessionLayoutDescriptor(
		toolspkg.ToolIDSessionReorder, "session_reorder", "Session Reorder",
		"Pin a manual session order in one folder, or in the workspace root when folder_id is omitted. "+
			"Listed sessions move into that container in order; members not listed keep their order after them. "+
			"The catalog shows this order with sort=manual.",
		sessionReorderInputSchema, toolspkg.RiskMutating, false,
		[]string{"reorder sessions", "manual session order", "sort sessions manually"},
	)
}

func sessionLayoutDescriptor(
	id toolspkg.ToolID,
	nativeName string,
	title string,
	description string,
	inputSchema string,
	risk toolspkg.RiskClass,
	readOnly bool,
	hints []string,
) toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		id, nativeName, title, description, inputSchema, risk, readOnly, false, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsFolderKey},
		hints,
	)
	descriptor.OutputSchema = json.RawMessage(sessionFoldersOutputSchema)
	if id == toolspkg.ToolIDSessionFolderCreate || id == toolspkg.ToolIDSessionFolderRename {
		descriptor.OutputSchema = json.RawMessage(sessionFolderOutputSchema)
	}
	return descriptor
}

const sessionLayoutWorkspaceInputSchema = `{
	"type":"object",
	"properties":{"workspace":{"type":"string"}},
	"additionalProperties":false
}`

const sessionFolderCreateInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{
		"workspace":{"type":"string"},
		"name":{"type":"string","minLength":1,"maxLength":120}
	},
	"additionalProperties":false
}`

const sessionFolderRenameInputSchema = `{
	"type":"object",
	"required":["folder_id","name"],
	"properties":{
		"workspace":{"type":"string"},
		"folder_id":{"type":"string","minLength":1},
		"name":{"type":"string","minLength":1,"maxLength":120}
	},
	"additionalProperties":false
}`

const sessionFolderIDInputSchema = `{
	"type":"object",
	"required":["folder_id"],
	"properties":{
		"workspace":{"type":"string"},
		"folder_id":{"type":"string","minLength":1}
	},
	"additionalProperties":false
}`

const sessionFolderReorderInputSchema = `{
	"type":"object",
	"required":["folder_ids"],
	"properties":{
		"workspace":{"type":"string"},
		"folder_ids":{"type":"array","maxItems":200,"items":{"type":"string","minLength":1}}
	},
	"additionalProperties":false
}`

const sessionMoveInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"folder_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const sessionReorderInputSchema = `{
	"type":"object",
	"required":["session_ids"],
	"properties":{
		"workspace":{"type":"string"},
		"folder_id":{"type":"string"},
		"session_ids":{"type":"array","minItems":1,"maxItems":500,"items":{"type":"string","minLength":1}}
	},
	"additionalProperties":false
}`

const sessionFolderPayloadSchema = `{
	"type":"object",
	"required":["id","workspace_id","profile_id","name","position","session_count","created_at","updated_at"],
	"properties":{
		"id":{"type":"string"},
		"workspace_id":{"type":"string"},
		"profile_id":{"type":"string"},
		"name":{"type":"string"},
		"position":{"type":"integer"},
		"session_count":{"type":"integer"},
		"created_at":{"type":"string"},
		"updated_at":{"type":"string"}
	}
}`

const sessionFoldersOutputSchema = `{
	"type":"object",
	"required":["folders"],
	"properties":{"folders":{"type":"array","items":` + sessionFolderPayloadSchema + `}}
}`

const sessionFolderOutputSchema = `{
	"type":"object",
	"required":["folder"],
	"properties":{"folder":` + sessionFolderPayloadSchema + `}
}`
