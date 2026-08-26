package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/contracts"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// AgentPayloadFromDef converts an agent definition into the shared payload.
func AgentPayloadFromDef(agent compozyconfig.AgentDef) contract.AgentPayload {
	return AgentPayloadFromEntry(AgentCatalogEntry{
		Def:    agent,
		Origin: contract.AgentOriginGlobal,
	})
}

// AgentCatalogEntryFromDef attaches origin truth to one disk-loaded definition.
func AgentCatalogEntryFromDef(
	home compozyconfig.HomePaths,
	agent compozyconfig.AgentDef,
	workspaceID string,
) AgentCatalogEntry {
	origin, _ := compozyconfig.AgentOriginFor(home, agent.SourcePath)
	entry := AgentCatalogEntry{Def: compozyconfig.CloneAgentDef(agent)}
	if origin == compozyconfig.AgentOriginWorkspace || (origin == "" && workspaceID != "") {
		entry.Origin = contract.AgentOriginWorkspace
		entry.WorkspaceID = workspaceID
		return entry
	}
	entry.Origin = contract.AgentOriginGlobal
	return entry
}

// AgentPayloadFromEntry converts an origin-aware catalog entry into the shared payload.
func AgentPayloadFromEntry(entry AgentCatalogEntry) contract.AgentPayload {
	agent := entry.Def
	shadows := make([]contract.AgentDefinitionShadowPayload, 0, len(agent.ShadowedDefinitions))
	for _, shadow := range agent.ShadowedDefinitions {
		shadows = append(shadows, contract.AgentDefinitionShadowPayload{
			Layer: shadow.Layer,
			Path:  shadow.Path,
		})
	}
	mcpServers := make([]contract.AgentMCPServerJSON, 0, len(agent.MCPServers))
	for _, server := range agent.MCPServers {
		redacted := compozyconfig.RedactedMCPServer(server)
		mcpServers = append(mcpServers, contract.AgentMCPServerJSON{
			Name:      redacted.Name,
			Transport: string(redacted.Transport),
			Command:   redacted.Command,
			Args:      append([]string(nil), redacted.Args...),
			Env:       redacted.Env,
			SecretEnv: redacted.SecretEnv,
			URL:       redacted.URL,
			Auth:      agentMCPAuthPayload(redacted.Auth),
		})
	}
	digest, digestErr := compozyconfig.AgentDefinitionDigest(agent)
	diagnostics := make([]contract.AgentDiagnosticPayload, 0, 1)
	if digestErr != nil {
		diagnostics = append(diagnostics, contract.AgentDiagnosticPayload{
			Path:      agent.SourcePath,
			ErrorKind: "definition_digest",
			Message:   digestErr.Error(),
		})
	}
	disabledSkills := append([]string(nil), agent.Skills.Disabled...)
	description, _, rejectDescription := contracts.SanitizeText(agent.Description)
	if rejectDescription {
		description = "[REDACTED]"
	}
	return contract.AgentPayload{
		Name:             agent.Name,
		Description:      description,
		Scope:            string(entry.Origin),
		Shadowed:         false,
		Provider:         agent.Provider,
		Command:          agent.Command,
		Model:            agent.Model,
		ReasoningEffort:  contract.ReasoningEffort(agent.ReasoningEffort),
		Speed:            agent.SpeedValue(),
		ACPOptions:       agentACPOptionsToContract(agent.ACPOptionsValue()),
		Tools:            append([]string(nil), agent.Tools...),
		Toolsets:         append([]string(nil), agent.Toolsets...),
		DenyTools:        append([]string(nil), agent.DenyTools...),
		Permissions:      agent.Permissions,
		CategoryPath:     append([]string(nil), agent.CategoryPath...),
		MCPServers:       mcpServers,
		Origin:           entry.Origin,
		WorkspaceID:      entry.WorkspaceID,
		Layer:            agent.SourceLayer,
		Shadows:          shadows,
		Skills:           &contract.CreateAgentSkillsConfig{Disabled: disabledSkills},
		DefinitionDigest: digest,
		Prompt:           agent.Prompt,
		Diagnostics:      diagnostics,
	}
}

// AgentPayloadFromEntryWithConfig attaches a runtime projection resolved from
// the requested workspace/global config without mutating authored fields.
func AgentPayloadFromEntryWithConfig(
	entry AgentCatalogEntry,
	cfg *compozyconfig.Config,
) contract.AgentPayload {
	payload := AgentPayloadFromEntry(entry)
	if cfg == nil || len(payload.Diagnostics) > 0 {
		return payload
	}
	resolved, err := cfg.ResolveAgent(entry.Def)
	if err != nil {
		return payload
	}
	payload.EffectiveRuntime = &contract.AgentEffectiveRuntimePayload{
		Provider:        resolved.Provider,
		Model:           resolved.Model,
		ReasoningEffort: contract.ReasoningEffort(resolved.ReasoningEffort),
		Speed:           resolved.SpeedValue(),
		ACPOptions:      agentACPOptionsToContract(resolved.ACPOptionsValue()),
		Sources: contract.AgentEffectiveRuntimeSources{
			Provider:        contract.AgentRuntimeValueSource(resolved.RuntimeSources.Provider.String()),
			Model:           contract.AgentRuntimeValueSource(resolved.RuntimeSources.Model.String()),
			ReasoningEffort: contract.AgentRuntimeValueSource(resolved.RuntimeSources.ReasoningEffort.String()),
			Speed:           contract.AgentRuntimeValueSource(resolved.RuntimeSources.Speed.String()),
		},
	}
	return payload
}

func agentACPOptionsToContract(
	options []compozyconfig.ACPOptionSelection,
) []contract.AgentACPOptionSelection {
	if len(options) == 0 {
		return nil
	}
	projected := make([]contract.AgentACPOptionSelection, len(options))
	for index, option := range options {
		projected[index] = contract.AgentACPOptionSelection{
			ID:      option.ID,
			ValueID: option.ValueID,
		}
		if option.BoolValue != nil {
			projected[index].BoolValue = new(*option.BoolValue)
		}
	}
	return projected
}

func agentACPOptionsFromContract(
	options []contract.AgentACPOptionSelection,
) []compozyconfig.ACPOptionSelection {
	if len(options) == 0 {
		return nil
	}
	projected := make([]compozyconfig.ACPOptionSelection, len(options))
	for index, option := range options {
		projected[index] = compozyconfig.ACPOptionSelection{
			ID:      option.ID,
			ValueID: option.ValueID,
		}
		if option.BoolValue != nil {
			projected[index].BoolValue = new(*option.BoolValue)
		}
	}
	return projected
}

// AgentPayloadsFromEntries converts origin-aware catalog entries into response payloads.
func AgentPayloadsFromEntries(entries []AgentCatalogEntry) []contract.AgentPayload {
	payloads := make([]contract.AgentPayload, 0, len(entries))
	for _, entry := range entries {
		payloads = append(payloads, AgentPayloadFromEntry(entry))
	}
	return payloads
}

// AgentPayloadFromDiagnostic converts a malformed workspace agent diagnostic into a payload row.
func AgentPayloadFromDiagnostic(
	diagnostic workspacepkg.AgentDiagnostic,
	workspaceID ...string,
) contract.AgentPayload {
	resolvedWorkspaceID := ""
	if len(workspaceID) > 0 {
		resolvedWorkspaceID = workspaceID[0]
	}
	return contract.AgentPayload{
		Name:        diagnostic.Name,
		Provider:    "",
		Origin:      contract.AgentOriginWorkspace,
		WorkspaceID: resolvedWorkspaceID,
		Prompt:      "",
		Diagnostics: []contract.AgentDiagnosticPayload{{
			Path:      diagnostic.Path,
			ErrorKind: diagnostic.ErrorKind,
			Message:   diagnostic.Message,
		}},
	}
}
