package core

import (
	"sort"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"

	authproviders "github.com/compozy/agh/internal/providers"
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

func providerDiagnosticItems(providers []contract.ProviderStatusPayload) []contract.DiagnosticItem {
	items := make([]contract.DiagnosticItem, 0, len(providers))
	for _, provider := range providers {
		items = append(items, providerDiagnosticItem(provider))
	}
	return items
}

func providerSuggestedCommand(providerName string, status contract.ProviderAuthStatusPayload) string {
	classification := authproviders.Classification{
		State:   authproviders.ProviderAuthState(strings.TrimSpace(status.State)),
		Code:    strings.TrimSpace(status.Code),
		Message: strings.TrimSpace(status.Message),
	}
	return authproviders.SuggestedCommand(providerName, classification)
}

func configDiagnosticItem(status contract.ConfigRuntimeStatusPayload) contract.DiagnosticItem {
	if status.Validated {
		return diagnostics.NewItem(
			"doctor.config.validate",
			contract.CodeConfigValidated,
			contract.CategoryConfig,
			"Config validates",
			"Runtime config validates against the current schema.",
			contract.SeverityOK,
			contract.FreshnessLive,
			diagnostics.WithEvidence(map[string]any{"apply_state": status.ApplyState}),
		)
	}
	message := strings.TrimSpace(status.ValidationError)
	if message == "" {
		message = "Runtime config failed validation."
	}
	return diagnostics.NewItem(
		"doctor.config.validate",
		contract.CodeConfigValidateFailed,
		contract.CategoryConfig,
		"Config validation failed",
		message,
		contract.SeverityCritical,
		contract.FreshnessLive,
	)
}
