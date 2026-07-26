package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	catalogApprovalsKey   = "approvals"
	catalogPermissionsKey = "permissions"
)

func toolApprovalDescriptors() []toolspkg.Descriptor {
	set := nativeDescriptor(
		toolspkg.ToolIDToolApprovalsSet,
		"tool_approvals_set",
		"Tool Approvals Set",
		"Set one explicit agent-wide or tool-wide native-tool approval decision in the caller's workspace.",
		toolApprovalsSetInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDToolApprovals},
		[]string{catalogToolsKey, catalogApprovalsKey, catalogPermissionsKey, "set"},
		[]string{"set wider tool approval", "remember tool permission"},
	)
	set.OutputSchema = json.RawMessage(toolApprovalsSetOutputSchema)
	list := nativeDescriptor(
		toolspkg.ToolIDToolApprovalsList,
		"tool_approvals_list",
		"Tool Approvals List",
		"List remembered native-tool approval decisions for the caller's workspace.",
		toolApprovalsListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDToolApprovals},
		[]string{catalogToolsKey, catalogApprovalsKey, catalogPermissionsKey, "list"},
		[]string{"remembered tool approvals", "tool permissions"},
	)
	list.OutputSchema = json.RawMessage(toolApprovalsListOutputSchema)
	revoke := nativeDescriptor(
		toolspkg.ToolIDToolApprovalsRevoke,
		"tool_approvals_revoke",
		"Tool Approvals Revoke",
		"Revoke one remembered native-tool approval decision in the caller's workspace.",
		toolApprovalsRevokeInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDToolApprovals},
		[]string{catalogToolsKey, catalogApprovalsKey, catalogPermissionsKey, "revoke"},
		[]string{"revoke tool approval", "remove tool permission"},
	)
	revoke.OutputSchema = json.RawMessage(toolApprovalsRevokeOutputSchema)
	return []toolspkg.Descriptor{set, list, revoke}
}

const toolApprovalsSetInputSchema = `{
	"type":"object",
	"required":["tool_id","decision","scope"],
	"properties":{
		"tool_id":{"type":"string","minLength":1},
		"decision":{"type":"string","enum":["allow","reject"]},
		"scope":{"type":"string","enum":["agent","tool"]},
		"agent_name":{"type":"string","minLength":1},
		"workspace_id":{"type":"string","minLength":1}
	},
	"additionalProperties":false
}`

const toolApprovalsListInputSchema = `{
	"type":"object",
	"properties":{
		"workspace_id":{"type":"string","minLength":1}
	},
	"additionalProperties":false
}`

const toolApprovalsRevokeInputSchema = `{
	"type":"object",
	"required":["id"],
	"properties":{
		"id":{"type":"string","minLength":1},
		"workspace_id":{"type":"string","minLength":1}
	},
	"additionalProperties":false
}`

const toolApprovalGrantOutputObjectSchema = `{
	"type":"object",
	"required":["id","workspace_id","tool_id","decision","created_at","last_used_at"],
	"properties":{
		"id":{"type":"string"},
		"workspace_id":{"type":"string"},
		"agent_name":{"type":"string"},
		"tool_id":{"type":"string"},
		"input_digest":{"type":"string"},
		"decision":{"type":"string","enum":["allow","reject"]},
		"created_at":{"type":"string","format":"date-time"},
		"last_used_at":{"type":"string","format":"date-time"}
	},
	"additionalProperties":false
}`

const toolApprovalsSetOutputSchema = `{
	"type":"object",
	"required":["grant"],
	"properties":{"grant":` + toolApprovalGrantOutputObjectSchema + `},
	"additionalProperties":false
}`

const toolApprovalsListOutputSchema = `{
	"type":"object",
	"required":["grants","total"],
	"properties":{
		"grants":{"type":"array","items":{"$ref":"#/$defs/grant"}},
		"total":{"type":"integer","minimum":0}
	},
	"$defs":{
		"grant":` + toolApprovalGrantOutputObjectSchema + `
	},
	"additionalProperties":false
}`

const toolApprovalsRevokeOutputSchema = `{
	"type":"object",
	"required":["revoked_id","workspace_id"],
	"properties":{
		"revoked_id":{"type":"string"},
		"workspace_id":{"type":"string"}
	},
	"additionalProperties":false
}`
