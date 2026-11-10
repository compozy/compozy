package daemon

import (
	"maps"

	"github.com/compozy/compozy/internal/api/contract"
)

type compactHealth struct {
	AgentProbes    []compactObserveProbe                `json:"agent_probes,omitempty"`
	Status         string                               `json:"status"`
	UptimeSeconds  int64                                `json:"uptime_seconds"`
	ActiveSessions int                                  `json:"active_sessions"`
	ActiveAgents   int                                  `json:"active_agents"`
	Persistence    compactPersistence                   `json:"persistence"`
	Retention      compactRetention                     `json:"retention"`
	Failures       contract.ObserveFailureHealthPayload `json:"failures"`
	Version        string                               `json:"version"`
}

func projectCompactHealth(p contract.ObserveHealthPayload) compactHealth {
	return compactHealth{
		AgentProbes:    compactObserveProbes(p.AgentProbes),
		Status:         p.Status,
		UptimeSeconds:  p.UptimeSeconds,
		ActiveSessions: p.ActiveSessions,
		ActiveAgents:   p.ActiveAgents,
		Persistence:    projectCompactPersistence(p.Persistence),
		Retention:      projectCompactRetention(p.Retention),
		Failures:       p.Failures,
		Version:        p.Version,
	}
}

type compactPersistence struct {
	Status string `json:"status"`
}

func projectCompactPersistence(p contract.ObservePersistenceHealthPayload) compactPersistence {
	return compactPersistence{
		Status: p.Status,
	}
}

type compactRetention struct {
	Enabled         bool   `json:"enabled"`
	LastSweepStatus string `json:"last_sweep_status"`
	LastSweepError  string `json:"last_sweep_error,omitempty"`
}

func projectCompactRetention(p contract.ObserveRetentionHealthPayload) compactRetention {
	return compactRetention{
		Enabled:         p.Enabled,
		LastSweepStatus: p.LastSweepStatus,
		LastSweepError:  p.LastSweepError,
	}
}

func compactObserveMetrics(p map[string]any) any {
	result := maps.Clone(p)
	if health, ok := p["health"].(contract.ObserveHealthPayload); ok {
		result["health"] = projectCompactHealth(health)
	}
	return result
}

type compactObserveProbe struct {
	AgentName string `json:"agent_name,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

func compactObserveProbes(p []contract.AgentProbeHealthPayload) []compactObserveProbe {
	if p == nil {
		return nil
	}
	result := make([]compactObserveProbe, len(p))
	for i := range p {
		result[i] = compactObserveProbe{p[i].AgentName, p[i].Provider, p[i].Status, p[i].Error}
	}
	return result
}
