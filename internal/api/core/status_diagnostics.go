package core

import (
	"sort"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
)

func splitStatusFilter(values []string, raw string) []string {
	values = append(values, raw)
	out := make([]string, 0, len(values))
	for _, value := range values {
		for part := range strings.SplitSeq(value, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}

func diagnosticItemsFromStatus(status *contract.StatusPayload, includeProviders bool) []contract.DiagnosticItem {
	if status == nil {
		return nil
	}
	items := []contract.DiagnosticItem{
		daemonDiagnosticItem(status),
		configDiagnosticItem(status.Config),
		automationDiagnosticItem(status.Automation),
		bridgeDiagnosticItem(status.Bridges),
		networkDiagnosticItem(status.Daemon.Network),
		skillDiagnosticItem(status.Skills),
		logTailDiagnosticItem(status.LogTail),
		taskDiagnosticItem(status.Tasks),
	}
	items = append(items, configLayerDiagnosticItems(status.Config.Diagnostics)...)
	if includeProviders {
		items = append(items, providerDiagnosticItems(status.Providers)...)
	}
	for _, server := range status.MCPServers {
		items = append(items, mcpServerDiagnosticItem(server))
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items
}

func configLayerDiagnosticItems(values []contract.ConfigLayerDiagnosticPayload) []contract.DiagnosticItem {
	items := make([]contract.DiagnosticItem, 0, len(values))
	for index, value := range values {
		items = append(items, diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            "doctor.config.profile_layer_orphaned." + strconv.Itoa(index),
			Code:          contract.CodeConfigProfileLayerOrphaned,
			Category:      contract.CategoryConfig,
			Title:         "Profile config layer is dormant",
			Message:       strings.TrimSpace(value.Message),
			Severity:      contract.SeverityWarn,
			DataFreshness: contract.FreshnessLive,
		}, diagnostics.WithEvidence(map[string]any{
			"layer": value.Layer, "profile": value.Profile, "path": value.Path,
		}), diagnostics.WithSuggestedCommand("compozy profile create "+value.Profile)))
	}
	return items
}

func providerDiagnosticItems(providers []contract.ProviderStatusPayload) []contract.DiagnosticItem {
	items := make([]contract.DiagnosticItem, 0, len(providers))
	for _, provider := range providers {
		items = append(items, providerDiagnosticItem(provider))
	}
	return items
}

func configDiagnosticItem(status contract.ConfigRuntimeStatusPayload) contract.DiagnosticItem {
	if status.Validated {
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            "doctor.config.validate",
			Code:          contract.CodeConfigValidated,
			Category:      contract.CategoryConfig,
			Title:         "Config validates",
			Message:       "Runtime config validates against the current schema.",
			Severity:      contract.SeverityOK,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(map[string]any{"apply_state": status.ApplyState}),
		)
	}
	message := strings.TrimSpace(status.ValidationError)
	if message == "" {
		message = "Runtime config failed validation."
	}
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            "doctor.config.validate",
		Code:          contract.CodeConfigValidateFailed,
		Category:      contract.CategoryConfig,
		Title:         "Config validation failed",
		Message:       message,
		Severity:      contract.SeverityCritical,
		DataFreshness: contract.FreshnessLive,
	})
}
