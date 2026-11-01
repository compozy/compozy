package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

const (
	workspaceWorkspaceKey = "workspace"
)

const (
	workspaceListKey = "list"
)

var workspaceTools = []toolspkg.Descriptor{
	nativeDescriptor(toolspkg.ToolIDAgentDraftInstructions, "agent_draft_instructions", "Draft Agent Instructions",
		"Generate instruction text under read-only triage without saving an agent definition.",
		`{"type":"object","required":["goal","runtime"],"properties":{"goal":{"type":"string","minLength":1},"runtime":{"type":"object","required":["provider"],"properties":{"provider":{"type":"string","minLength":1},"model":{"type":"string"},"reasoning_effort":{"type":"string"}},"additionalProperties":false}},"additionalProperties":false}`,
		toolspkg.RiskMutating, false, false, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDWorkspace}, []string{"agent", "draft"}, []string{"fill with AI", "draft instructions"}),
	workspaceArchiveDescriptor(
		toolspkg.ToolIDWorkspaceArchive,
		"workspace_archive",
		"Archive Workspace",
		"Archive a project without removing its data. Refuses active sessions, live Loops and enabled automations.",
		"archive project",
	),
	workspaceArchiveDescriptor(
		toolspkg.ToolIDWorkspaceUnarchive,
		"workspace_unarchive",
		"Unarchive Workspace",
		"Restore an archived project to the active catalog with its data and workspace ID unchanged.",
		"unarchive project",
	),
	nativeDescriptor(
		toolspkg.ToolIDAgentList,
		"agent_list",
		"Agent List",
		"List agent definitions visible in a workspace through the existing agent catalog.",
		agentListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog, toolspkg.ToolsetIDWorkspace},
		[]string{workspaceWorkspaceKey, descriptorKeywordAgent, descriptorKeywordCatalog},
		[]string{"available agents", "agent catalog", "list agents"},
	),
	nativeDescriptor(
		toolspkg.ToolIDWorkspaceList,
		"workspace_list",
		"Workspace List",
		"List registered workspaces. A session-bound caller sees its own workspace plus every other "+
			"workspace its permission mode already lets it address (approve-all, or a recorded allow under "+
			"approve-reads); pass any listed ID, name, or path to tools that accept workspace.",
		workspaceListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDWorkspace},
		[]string{workspaceWorkspaceKey, workspaceListKey},
		[]string{"workspace list", "registered workspaces"},
	),
	nativeDescriptor(
		toolspkg.ToolIDWorkspaceInfo,
		"workspace_info",
		"Workspace Info",
		"Read one registered workspace record.",
		workspaceRefInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDWorkspace},
		[]string{workspaceWorkspaceKey, "info"},
		[]string{"workspace info", "workspace record"},
	),
	nativeDescriptor(
		toolspkg.ToolIDWorkspaceDescribe,
		"workspace_describe",
		"Workspace Describe",
		"Read one resolved workspace detail projection with sessions, agents, skills, and providers.",
		workspaceRefInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDWorkspace},
		[]string{workspaceWorkspaceKey, "describe"},
		[]string{"workspace describe", "workspace detail"},
	),
	nativeDescriptor(
		toolspkg.ToolIDAgentCreate,
		"agent_create",
		"Agent Create",
		"Author one AGENT.md definition at global or workspace scope through the existing agent authoring surface.",
		agentCreateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDWorkspace},
		[]string{workspaceWorkspaceKey, descriptorKeywordAgent, descriptorKeywordCreate},
		[]string{"create agent", "author agent definition"},
	),
}

func workspaceDescriptors() []toolspkg.Descriptor {
	return append(append([]toolspkg.Descriptor(nil), workspaceTools...), workspaceLayoutDescriptors()...)
}

const workspaceRefInputSchema = `{
	"type":"object",
	"properties":{
		"workspace":{"type":"string"}
	},
	"additionalProperties":false
}`

const agentListInputSchema = `{
	"type":"object",
	"properties":{"workspace":{"type":"string"}},
	"additionalProperties":false
}`

const acpOptionSelectionInputSchema = `{
	"oneOf":[
		{"type":"object","required":["id","value_id"],"properties":{
			"id":{"type":"string","minLength":1},"value_id":{"type":"string","minLength":1}
		},"additionalProperties":false},
		{"type":"object","required":["id","bool_value"],"properties":{
			"id":{"type":"string","minLength":1},"bool_value":{"type":"boolean"}
		},"additionalProperties":false}
	]
}`

const sessionEnvInputSchema = `{
	"type":"object",
	"additionalProperties":{
		"type":"object","required":["ref","required"],
		"properties":{"ref":{"type":"string","pattern":"^(env:|vault:)"},"required":{"type":"boolean"}},
		"additionalProperties":false
	}
}`

var agentCreateInputSchema = `{
	"type":"object",
	"required":["scope","name","prompt"],
	"properties":{
		"scope":{"type":"string","enum":["global","workspace"]},
		"workspace":{"type":"string"},
		"name":` + agentNameInputSchema + `,
		"provider":{"type":"string"},
		"model":{"type":"string"},
		"reasoning_effort":{"type":"string","enum":["none","minimal","low","medium","high","xhigh","max","ultra"]},
		"speed":{"type":"string","enum":["normal","fast"]},
		"acp_options":{"type":"array","items":` + acpOptionSelectionInputSchema + `},
		"command":{"type":"string"},
		"session_env":` + sessionEnvInputSchema + `,
		"prompt":{"type":"string"},
		"workspace_access":{"type":"string","enum":["","unrestricted"]},
 "permissions":{"type":"string","enum":["deny-all","approve-reads","approve-all"]},
		"tools":{"type":"array","items":{"type":"string"}},
		"toolsets":{"type":"array","items":{"type":"string"}},
		"deny_tools":{"type":"array","items":{"type":"string"}},
		"display_name":{"type":"string"},
 "description":{"type":"string"},
 "kind":{"type":"string","enum":["on_demand","stays_on"]},
 "appearance":{"type":"object","additionalProperties":true},
		"category_path":{"type":"array","items":{"type":"string"}},
		"disabled_skills":{"type":"array","items":{"type":"string"}},
		"fallback_chain":{"type":"array","items":` + agentFallbackRouteInputSchema + `}
	},
	"additionalProperties":false
}`

var agentFallbackRouteInputSchema = `{
	"type":"object",
	"required":["provider","model"],
	"properties":{
		"provider":{"type":"string","minLength":1},
		"model":{"type":"string","minLength":1},
		"reasoning_effort":{"type":"string","enum":["none","minimal","low","medium","high","xhigh","max","ultra"]},
		"speed":{"type":"string","enum":["normal","fast"]},
		"acp_options":{"type":"array","items":` + acpOptionSelectionInputSchema + `},
		"command":{"type":"string"},
		"session_env":` + sessionEnvInputSchema + `
	},
	"additionalProperties":false
}`

const workspaceListInputSchema = `{"type":"object","properties":{"archived":{"type":"boolean"}},"additionalProperties":false}`

func workspaceArchiveDescriptor(id toolspkg.ToolID, name, title, description, phrase string) toolspkg.Descriptor {
	return nativeDescriptor(
		id,
		name,
		title,
		description,
		workspaceRefInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{
			toolspkg.ToolsetIDWorkspace,
		},
		[]string{"workspace", name},
		[]string{phrase},
	)
}
