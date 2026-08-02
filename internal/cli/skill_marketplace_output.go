package cli

import (
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

func skillInstallBundle(item skillInstallItem) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			cleanupDiagnostics := stringOrDash(skillCleanupDiagnosticOperations(item.CleanupDiagnostics))
			return renderHumanSection("Skill Install", []keyValue{
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
				{Label: skillOutputSlugValue, Value: stringOrDash(item.Slug)},
				{Label: versionValue, Value: stringOrDash(item.Version)},
				{Label: "Registry", Value: stringOrDash(item.Registry)},
				{Label: skillOutputPathValue, Value: stringOrDash(item.Path)},
				{Label: cliHashValue, Value: stringOrDash(item.Hash)},
				{Label: skillOutputStatusValue, Value: stringOrDash(item.Status)},
				{Label: "Cleanup diagnostics", Value: cleanupDiagnostics},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"skill_install",
				[]string{
					automationNameKey,
					skillOutputSlugKey,
					versionKey,
					skillOutputRegistryKey,
					skillOutputPathKey,
					"hash",
					skillOutputStatusKey,
					"cleanup_diagnostics",
				},
				[]string{
					item.Name,
					item.Slug,
					item.Version,
					item.Registry,
					item.Path,
					item.Hash,
					item.Status,
					skillCleanupDiagnosticOperations(item.CleanupDiagnostics),
				},
			), nil
		},
	}
}

func skillRemoveBundle(item skillRemoveItem) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Skill Remove", []keyValue{
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
				{Label: skillOutputSlugValue, Value: stringOrDash(item.Slug)},
				{Label: skillOutputPathValue, Value: stringOrDash(item.Path)},
				{Label: skillOutputStatusValue, Value: stringOrDash(item.Status)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"skill_remove",
				[]string{automationNameKey, skillOutputSlugKey, skillOutputPathKey, skillOutputStatusKey},
				[]string{item.Name, item.Slug, item.Path, item.Status},
			), nil
		},
	}
}

func skillUpdateBundle(items []skillUpdateItem) outputBundle {
	return listBundle(
		items,
		items,
		"Skill Updates",
		[]string{
			automationNameValue,
			skillOutputSlugValue,
			"Current",
			"Latest",
			skillOutputPathValue,
			skillOutputStatusValue,
			"Cleanup diagnostics",
		},
		"skill_updates",
		[]string{
			automationNameKey,
			skillOutputSlugKey,
			skillOutputCurrentVersionKey,
			"latest_version",
			skillOutputPathKey,
			skillOutputStatusKey,
			"cleanup_diagnostics",
		},
		func(item skillUpdateItem) []string {
			return []string{
				stringOrDash(item.Name),
				stringOrDash(item.Slug),
				stringOrDash(item.CurrentVersion),
				stringOrDash(item.LatestVersion),
				stringOrDash(item.Path),
				stringOrDash(item.Status),
				stringOrDash(skillCleanupDiagnosticOperations(item.CleanupDiagnostics)),
			}
		},
		func(item skillUpdateItem) []string {
			return []string{
				item.Name,
				item.Slug,
				item.CurrentVersion,
				item.LatestVersion,
				item.Path,
				item.Status,
				skillCleanupDiagnosticOperations(item.CleanupDiagnostics),
			}
		},
	)
}

func skillCleanupDiagnosticOperations(
	diagnostics []contract.SkillMarketplaceCleanupDiagnosticPayload,
) string {
	operations := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		operation := strings.TrimSpace(diagnostic.Operation)
		if operation == "" || slices.Contains(operations, operation) {
			continue
		}
		operations = append(operations, operation)
	}
	return strings.Join(operations, ", ")
}
