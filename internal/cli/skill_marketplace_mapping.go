package cli

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/skills"
	skillmarketplace "github.com/compozy/compozy/internal/skills/marketplace"
)

func skillInstallItemFromRecord(item SkillMarketplaceInstallRecord) skillInstallItem {
	return skillInstallItem{
		Name:               item.Name,
		Slug:               item.Slug,
		Version:            item.Version,
		Registry:           item.Registry,
		Path:               item.Path,
		Hash:               item.Hash,
		Status:             item.Status,
		CleanupDiagnostics: skillCleanupDiagnosticsFromContract(item.CleanupDiagnostics),
	}
}

func skillUpdateItemsFromRecords(items []SkillMarketplaceUpdateRecord) []skillUpdateItem {
	updates := make([]skillUpdateItem, 0, len(items))
	for _, item := range items {
		updates = append(updates, skillUpdateItem{
			Name:               item.Name,
			Slug:               item.Slug,
			CurrentVersion:     item.CurrentVersion,
			LatestVersion:      item.LatestVersion,
			Path:               item.Path,
			Status:             item.Status,
			CleanupDiagnostics: skillCleanupDiagnosticsFromContract(item.CleanupDiagnostics),
		})
	}
	return updates
}

func skillCleanupDiagnosticsFromDomain(
	diagnostics []skillmarketplace.CleanupDiagnostic,
) []contract.SkillMarketplaceCleanupDiagnosticPayload {
	items := make([]contract.SkillMarketplaceCleanupDiagnosticPayload, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		items = appendSkillCleanupDiagnostic(items, diagnostic.Operation)
	}
	return items
}

func skillCleanupDiagnosticsFromContract(
	diagnostics []contract.SkillMarketplaceCleanupDiagnosticPayload,
) []contract.SkillMarketplaceCleanupDiagnosticPayload {
	items := make([]contract.SkillMarketplaceCleanupDiagnosticPayload, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		items = appendSkillCleanupDiagnostic(items, diagnostic.Operation)
	}
	return items
}

func appendSkillCleanupDiagnostic(
	diagnostics []contract.SkillMarketplaceCleanupDiagnosticPayload,
	operation string,
) []contract.SkillMarketplaceCleanupDiagnosticPayload {
	normalized := strings.TrimSpace(operation)
	if normalized == "" {
		return diagnostics
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Operation == normalized {
			return diagnostics
		}
	}
	return append(diagnostics, contract.SkillMarketplaceCleanupDiagnosticPayload{Operation: normalized})
}

func skillRemoveItemFromRecord(item SkillMarketplaceRemoveRecord) skillRemoveItem {
	return skillRemoveItem{
		Name:   item.Name,
		Slug:   item.Slug,
		Path:   item.Path,
		Status: item.Status,
	}
}

func criticalWarnings(warnings []skills.Warning) []string {
	items := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if warning.Severity != skills.SeverityCritical {
			continue
		}
		items = append(items, firstNonEmpty(warning.Message, warning.Pattern))
	}
	return items
}
