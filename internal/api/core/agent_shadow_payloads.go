package core

import (
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

// AgentListPayloads expands effective definitions with their inactive shadow rows.
func AgentListPayloads(
	entries []AgentCatalogEntry,
	cfg *compozyconfig.Config,
	homePaths compozyconfig.HomePaths,
	workspaceID string,
) []contract.AgentPayload {
	payloads := make([]contract.AgentPayload, 0, len(entries))
	for _, entry := range entries {
		payloads = append(payloads, AgentPayloadFromEntryWithConfig(entry, cfg))
		for _, shadow := range entry.Def.ShadowedDefinitions {
			payloads = append(payloads, agentShadowPayload(entry, shadow, cfg, homePaths, workspaceID))
		}
	}
	sort.SliceStable(payloads, func(i, j int) bool {
		if payloads[i].Name != payloads[j].Name {
			return payloads[i].Name < payloads[j].Name
		}
		if payloads[i].Shadowed != payloads[j].Shadowed {
			return !payloads[i].Shadowed
		}
		return payloads[i].Layer < payloads[j].Layer
	})
	return payloads
}

func agentShadowPayload(
	winner AgentCatalogEntry,
	shadow compozyconfig.AgentDefinitionRef,
	cfg *compozyconfig.Config,
	homePaths compozyconfig.HomePaths,
	workspaceID string,
) contract.AgentPayload {
	definition, err := compozyconfig.LoadAgentDefFile(shadow.Path)
	if err != nil {
		return contract.AgentPayload{
			Name: winner.Def.Name, Scope: "shadowed", Shadowed: true,
			Layer: strings.TrimSpace(shadow.Layer), WorkspaceID: strings.TrimSpace(workspaceID),
			Diagnostics: []contract.AgentDiagnosticPayload{{
				Path: shadow.Path, ErrorKind: "definition_load", Message: err.Error(),
			}},
		}
	}
	definition.SourceLayer = strings.TrimSpace(shadow.Layer)
	entry := AgentCatalogEntryFromDef(homePaths, definition, strings.TrimSpace(workspaceID))
	payload := AgentPayloadFromEntryWithConfig(entry, cfg)
	payload.Scope = "shadowed"
	payload.Shadowed = true
	payload.Shadows = nil
	return payload
}
