package daemon

import "github.com/compozy/compozy/internal/session"

type compactSubagentProvider struct {
	Provider        string   `json:"provider"`
	DisplayName     string   `json:"display_name"`
	CanDelegate     bool     `json:"can_delegate"`
	Constraints     []string `json:"constraints"`
	ModelsTotal     int      `json:"models_total"`
	ModelsTruncated bool     `json:"models_truncated"`
}

var subagentCapabilitiesViews = resultViews[session.SubagentCapabilities]{
	Compact: func(c session.SubagentCapabilities) any {
		providers := make([]compactSubagentProvider, 0, len(c.Providers))
		for _, p := range c.Providers {
			providers = append(providers, compactSubagentProvider{
				Provider: p.Provider, DisplayName: p.DisplayName, CanDelegate: p.CanDelegate,
				Constraints: nonNilSubagentStrings(p.Constraints),
				ModelsTotal: p.ModelsTotal, ModelsTruncated: p.ModelsTruncated,
			})
		}
		result := subagentCapabilitiesPayload(c)
		result["providers"] = providers
		return result
	},
	Full:    func(c session.SubagentCapabilities) any { return subagentCapabilitiesPayload(c) },
	Preview: func(session.SubagentCapabilities) string { return "Subagent capabilities" },
}
