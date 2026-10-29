package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func workspaceLayoutDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		workspaceLayoutDescriptor(
			toolspkg.ToolIDWorkspaceFolderList, "workspace_folder_list", "Workspace Folder List",
			"Read the caller profile's root workspace folders and manual workspace order shown in All projects. "+
				"Placements list workspaces (or @user) by folder, then the root; unplaced workspaces follow in name order.",
			emptyObjectInputSchema, toolspkg.RiskRead, true,
			[]string{"list project folders", "workspace folders", "project order"},
		),
		workspaceLayoutDescriptor(
			toolspkg.ToolIDWorkspaceFolderCreate, "workspace_folder_create", "Workspace Folder Create",
			"Create a named root folder at the top of the All-projects folder order. Folders hold workspaces only.",
			workspaceFolderCreateInputSchema, toolspkg.RiskMutating, false,
			[]string{"create project folder", "group projects"},
		),
		workspaceLayoutDescriptor(
			toolspkg.ToolIDWorkspaceFolderRename, "workspace_folder_rename", "Workspace Folder Rename",
			"Rename one root workspace folder.",
			workspaceFolderRenameInputSchema, toolspkg.RiskMutating, false,
			[]string{"rename project folder"},
		),
		workspaceLayoutDescriptor(
			toolspkg.ToolIDWorkspaceFolderDelete, "workspace_folder_delete", "Workspace Folder Delete",
			"Delete one root workspace folder. Its workspaces return to the root; no workspace or session is touched.",
			workspaceFolderIDInputSchema, toolspkg.RiskMutating, false,
			[]string{"delete project folder", "ungroup projects"},
		),
		workspaceLayoutDescriptor(
			toolspkg.ToolIDWorkspaceFolderReorder, "workspace_folder_reorder", "Workspace Folder Reorder",
			"Set the complete workspace folder order; folder_ids must list every folder exactly once.",
			workspaceFolderReorderInputSchema, toolspkg.RiskMutating, false,
			[]string{"reorder project folders"},
		),
		workspaceLayoutDescriptor(
			toolspkg.ToolIDWorkspaceMove, "workspace_move", "Workspace Move",
			"Move one workspace (ID, name, path, or @user; defaults to the caller's workspace) to the end of "+
				"a root folder, or to the end of the root when folder_id is omitted.",
			workspaceMoveInputSchema, toolspkg.RiskMutating, false,
			[]string{"move project to folder", "remove project from folder"},
		),
		workspaceLayoutDescriptor(
			toolspkg.ToolIDWorkspaceReorder, "workspace_reorder", "Workspace Reorder",
			"Pin a manual workspace order in one root folder, or in the root when folder_id is omitted. "+
				"Listed workspaces move into that container in order; members not listed keep their order after them.",
			workspaceReorderInputSchema, toolspkg.RiskMutating, false,
			[]string{"reorder projects", "manual project order"},
		),
	}
}

func workspaceLayoutDescriptor(
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
		id, nativeName, title, description+" Requires an operator or an unrestricted agent.",
		inputSchema, risk, readOnly, false, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDWorkspace},
		[]string{workspaceWorkspaceKey, sessionsFolderKey},
		hints,
	)
	descriptor.OutputSchema = json.RawMessage(workspaceLayoutOutputSchema)
	if id == toolspkg.ToolIDWorkspaceFolderCreate || id == toolspkg.ToolIDWorkspaceFolderRename {
		descriptor.OutputSchema = json.RawMessage(workspaceFolderOutputSchema)
	}
	return descriptor
}

const emptyObjectInputSchema = `{"type":"object","properties":{},"additionalProperties":false}`

const workspaceFolderCreateInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{"name":{"type":"string","minLength":1,"maxLength":120}},
	"additionalProperties":false
}`

const workspaceFolderRenameInputSchema = `{
	"type":"object",
	"required":["folder_id","name"],
	"properties":{
		"folder_id":{"type":"string","minLength":1},
		"name":{"type":"string","minLength":1,"maxLength":120}
	},
	"additionalProperties":false
}`

const workspaceFolderIDInputSchema = `{
	"type":"object",
	"required":["folder_id"],
	"properties":{"folder_id":{"type":"string","minLength":1}},
	"additionalProperties":false
}`

const workspaceFolderReorderInputSchema = `{
	"type":"object",
	"required":["folder_ids"],
	"properties":{
		"folder_ids":{"type":"array","maxItems":200,"items":{"type":"string","minLength":1}}
	},
	"additionalProperties":false
}`

const workspaceMoveInputSchema = `{
	"type":"object",
	"properties":{
		"workspace":{"type":"string"},
		"folder_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const workspaceReorderInputSchema = `{
	"type":"object",
	"required":["workspace_ids"],
	"properties":{
		"folder_id":{"type":"string"},
		"workspace_ids":{"type":"array","minItems":1,"maxItems":500,"items":{"type":"string","minLength":1}}
	},
	"additionalProperties":false
}`

const workspaceFolderPayloadSchema = `{
	"type":"object",
	"required":["id","profile_id","name","position","workspace_count","created_at","updated_at"],
	"properties":{
		"id":{"type":"string"},
		"profile_id":{"type":"string"},
		"name":{"type":"string"},
		"position":{"type":"integer"},
		"workspace_count":{"type":"integer"},
		"created_at":{"type":"string"},
		"updated_at":{"type":"string"}
	}
}`

const workspaceLayoutOutputSchema = `{
	"type":"object",
	"required":["folders","placements"],
	"properties":{
		"folders":{"type":"array","items":` + workspaceFolderPayloadSchema + `},
		"placements":{"type":"array","items":{
			"type":"object",
			"required":["workspace_id","position"],
			"properties":{
				"workspace_id":{"type":"string"},
				"folder_id":{"type":"string"},
				"position":{"type":"integer"}
			}
		}}
	}
}`

const workspaceFolderOutputSchema = `{
	"type":"object",
	"required":["folder"],
	"properties":{"folder":` + workspaceFolderPayloadSchema + `}
}`
