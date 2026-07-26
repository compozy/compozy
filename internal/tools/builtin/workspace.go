package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	workspaceWorkspaceKey = "workspace"
)

const (
	workspaceListKey = "list"
)

var workspaceTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDWorkspaceList,
		"workspace_list",
		"Workspace List",
		"List registered workspaces through the existing workspace discovery surface.",
		emptyInputSchema,
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
		[]string{workspaceWorkspaceKey, "agent", descriptorKeywordCreate},
		[]string{"create agent", "author agent definition"},
	),
}

func workspaceDescriptors() []toolspkg.Descriptor {
	return workspaceTools
}

const workspaceRefInputSchema = `{
	"type":"object",
	"required":["workspace"],
	"properties":{
		"workspace":{"type":"string"}
	},
	"additionalProperties":false
}`

const agentCreateInputSchema = `{
	"type":"object",
	"required":["scope","name","prompt"],
	"properties":{
		"scope":{"type":"string","enum":["global","workspace"]},
		"workspace":{"type":"string"},
		"name":{"type":"string"},
		"provider":{"type":"string"},
		"model":{"type":"string"},
		"reasoning_effort":{"type":"string","enum":["none","minimal","low","medium","high","xhigh","max"]},
		"command":{"type":"string"},
		"prompt":{"type":"string"},
		"permissions":{"type":"string","enum":["deny-all","approve-reads","approve-all"]},
		"tools":{"type":"array","items":{"type":"string"}},
		"toolsets":{"type":"array","items":{"type":"string"}},
		"deny_tools":{"type":"array","items":{"type":"string"}},
		"category_path":{"type":"array","items":{"type":"string"}},
		"disabled_skills":{"type":"array","items":{"type":"string"}}
	},
	"additionalProperties":false
}`
