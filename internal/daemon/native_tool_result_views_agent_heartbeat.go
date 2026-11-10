package daemon

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/heartbeat/source"
)

type compactAgentHeartbeatStatus struct {
	AgentName        string                                      `json:"agent_name"`
	Enabled          bool                                        `json:"enabled"`
	Present          bool                                        `json:"present"`
	Active           bool                                        `json:"active"`
	Valid            bool                                        `json:"valid"`
	ValidationStatus contract.AuthoredValidationStatus           `json:"validation_status"`
	Summary          string                                      `json:"summary,omitempty"`
	Diagnostics      []contract.AuthoredContextDiagnosticPayload `json:"diagnostics,omitempty"`
	WakeState        *contract.HeartbeatWakeStatePayload         `json:"wake_state,omitempty"`
	WakeEvents       []contract.HeartbeatWakeEventPayload        `json:"wake_events,omitempty"`
	SessionHealth    *contract.SessionHealthPayload              `json:"session_health,omitempty"`
	RevisionCursor   string                                      `json:"revision_cursor,omitempty"`
}

func compactHeartbeatStatus(p contract.HeartbeatStatusResponse) compactAgentHeartbeatStatus {
	return compactAgentHeartbeatStatus{
		AgentName:        p.AgentName,
		Enabled:          p.Enabled,
		Present:          p.Present,
		Active:           p.Active,
		Valid:            p.Valid,
		ValidationStatus: p.ValidationStatus,
		Summary:          p.Summary,
		Diagnostics:      p.Diagnostics,
		WakeState:        p.WakeState,
		WakeEvents:       p.WakeEvents,
		SessionHealth:    p.SessionHealth,
		RevisionCursor:   p.RevisionCursor,
	}
}

type compactHeartbeatSources struct {
	Agent          string                                `json:"agent"`
	ScriptsEnabled bool                                  `json:"scripts_enabled"`
	Limits         contract.HeartbeatSourceLimitsPayload `json:"limits"`
	Kinds          []compactHeartbeatSourceKind          `json:"kinds"`
	Scripts        []compactHeartbeatSourceScript        `json:"scripts"`
}

func compactSourceCatalog(p contract.HeartbeatSourceCatalogPayload) compactHeartbeatSources {
	result := compactHeartbeatSources{
		Agent:          p.Agent,
		ScriptsEnabled: p.ScriptsEnabled,
		Limits:         p.Limits,
	}
	if p.Kinds != nil {
		result.Kinds = make([]compactHeartbeatSourceKind, len(p.Kinds))
		for i, value := range p.Kinds {
			result.Kinds[i] = compactSourceKind(value)
		}
	}
	if p.Scripts != nil {
		result.Scripts = make([]compactHeartbeatSourceScript, len(p.Scripts))
		for i, value := range p.Scripts {
			result.Scripts[i] = compactSourceScript(value)
		}
	}
	return result
}

type compactHeartbeatSourceKind struct {
	Kind        string `json:"kind"`
	Extension   string `json:"extension"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

func compactSourceKind(p source.KindDescriptor) compactHeartbeatSourceKind {
	return compactHeartbeatSourceKind{
		Kind:        p.Kind,
		Extension:   p.Extension,
		Title:       p.Title,
		Description: p.Description,
	}
}

type compactHeartbeatSourceScript struct {
	Name       string `json:"name"`
	Approved   bool   `json:"approved"`
	Executable bool   `json:"executable"`
}

func compactSourceScript(p contract.HeartbeatSourceScriptPayload) compactHeartbeatSourceScript {
	return compactHeartbeatSourceScript{
		Name:       p.Name,
		Approved:   p.Approved,
		Executable: p.Executable,
	}
}

var agentHeartbeatStatusViews = resultViews[contract.HeartbeatStatusResponse]{
	Compact: func(p contract.HeartbeatStatusResponse) any {
		return map[string]any{"heartbeat": compactHeartbeatStatus(p)}
	},
	Full:    func(p contract.HeartbeatStatusResponse) any { return map[string]any{"heartbeat": p} },
	Preview: func(p contract.HeartbeatStatusResponse) string { return string(p.ValidationStatus) },
}

var agentHeartbeatSourcesViews = resultViews[contract.HeartbeatSourceCatalogPayload]{
	Compact: func(p contract.HeartbeatSourceCatalogPayload) any { return compactSourceCatalog(p) },
	Preview: func(contract.HeartbeatSourceCatalogPayload) string { return "Check-in sources" },
}
