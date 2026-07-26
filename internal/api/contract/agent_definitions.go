package contract

// AgentOrigin identifies the authored root that owns an effective definition.
type AgentOrigin string

const (
	// AgentOriginGlobal identifies a definition stored under AGH_HOME.
	AgentOriginGlobal AgentOrigin = "global"
	// AgentOriginWorkspace identifies a definition stored under a workspace root.
	AgentOriginWorkspace AgentOrigin = "workspace"
)

// UpdateAgentRequest replaces the effective route agent definition.
type UpdateAgentRequest struct {
	Workspace      string             `json:"workspace,omitempty"`
	Agent          CreateAgentPayload `json:"agent"`
	ExpectedDigest string             `json:"expected_digest"`
}

// DuplicateAgentRequest clones the effective route agent definition.
type DuplicateAgentRequest struct {
	Name      string                   `json:"name"`
	Scope     AgentCreateScope         `json:"scope,omitempty"`
	Workspace string                   `json:"workspace,omitempty"`
	Overrides *DuplicateAgentOverrides `json:"overrides,omitempty"`
}

// DuplicateAgentOverrides captures caller-editable fields applied to a clone.
type DuplicateAgentOverrides struct {
	Provider        string                   `json:"provider,omitempty"`
	Command         string                   `json:"command,omitempty"`
	Model           string                   `json:"model,omitempty"`
	ReasoningEffort ReasoningEffort          `json:"reasoning_effort,omitempty"`
	Tools           []string                 `json:"tools,omitempty"`
	Toolsets        []string                 `json:"toolsets,omitempty"`
	DenyTools       []string                 `json:"deny_tools,omitempty"`
	Permissions     SettingsPermissionMode   `json:"permissions,omitempty"`
	CategoryPath    []string                 `json:"category_path,omitempty"`
	Skills          *CreateAgentSkillsConfig `json:"skills,omitempty"`
	Prompt          string                   `json:"prompt,omitempty"`
}

// DeleteAgentResponse reports the effective definition removed from disk.
type DeleteAgentResponse struct {
	Name             string      `json:"name"`
	Origin           AgentOrigin `json:"origin"`
	UnshadowedOrigin AgentOrigin `json:"unshadowed_origin,omitempty"`
}

// AgentRuntimeValueSource identifies the authored/default layer that supplied
// one effective runtime value.
type AgentRuntimeValueSource string

const (
	AgentRuntimeValueSourceAgent           AgentRuntimeValueSource = "agent"
	AgentRuntimeValueSourceProjectDefault  AgentRuntimeValueSource = "project_default"
	AgentRuntimeValueSourceProviderDefault AgentRuntimeValueSource = "provider_default"
	AgentRuntimeValueSourceModelDefault    AgentRuntimeValueSource = "model_default"
)

// AgentEffectiveRuntimeSources reports provenance for effective agent runtime values.
type AgentEffectiveRuntimeSources struct {
	Provider        AgentRuntimeValueSource `json:"provider"`
	Model           AgentRuntimeValueSource `json:"model,omitempty"`
	ReasoningEffort AgentRuntimeValueSource `json:"reasoning_effort,omitempty"`
}

// AgentEffectiveRuntimePayload is the workspace-scoped runtime projection for
// one authored agent definition.
type AgentEffectiveRuntimePayload struct {
	Provider        string                       `json:"provider"`
	Model           string                       `json:"model,omitempty"`
	ReasoningEffort ReasoningEffort              `json:"reasoning_effort,omitempty"`
	Sources         AgentEffectiveRuntimeSources `json:"sources"`
}

// AgentPayload is the shared agent definition response payload.
type AgentPayload struct {
	Name             string                        `json:"name"`
	Provider         string                        `json:"provider"`
	Command          string                        `json:"command,omitempty"`
	Model            string                        `json:"model,omitempty"`
	ReasoningEffort  ReasoningEffort               `json:"reasoning_effort,omitempty"`
	Tools            []string                      `json:"tools,omitempty"`
	Toolsets         []string                      `json:"toolsets,omitempty"`
	DenyTools        []string                      `json:"deny_tools,omitempty"`
	Permissions      string                        `json:"permissions,omitempty"`
	CategoryPath     []string                      `json:"category_path,omitempty"`
	MCPServers       []AgentMCPServerJSON          `json:"mcp_servers,omitempty"`
	Origin           AgentOrigin                   `json:"origin"`
	WorkspaceID      string                        `json:"workspace_id,omitempty"`
	Skills           *CreateAgentSkillsConfig      `json:"skills,omitempty"`
	DefinitionDigest string                        `json:"definition_digest"`
	Prompt           string                        `json:"prompt"`
	Diagnostics      []AgentDiagnosticPayload      `json:"diagnostics,omitempty"`
	EffectiveRuntime *AgentEffectiveRuntimePayload `json:"effective_runtime,omitempty"`
}
