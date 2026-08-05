package extensionpkg

import (
	"strings"

	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

const (
	marketplaceDiagnosticSlugKey       = "slug"
	marketplaceInstallCleanupOperation = "cleanup_operation"
)

func appendRegistryCleanupDiagnostics(
	diagnostics []registrypkg.CleanupDiagnostic,
	additional []registrypkg.CleanupDiagnostic,
) []registrypkg.CleanupDiagnostic {
	for _, candidate := range additional {
		duplicate := false
		for _, existing := range diagnostics {
			if existing.Operation == candidate.Operation {
				duplicate = true
				break
			}
		}
		if !duplicate {
			diagnostics = append(diagnostics, candidate)
		}
	}
	return diagnostics
}

func appendExtensionInstallCleanupWarnings(
	warnings []diagnosticcontract.DiagnosticItem,
	slug string,
	diagnosticsList []registrypkg.CleanupDiagnostic,
) []diagnosticcontract.DiagnosticItem {
	seen := make(map[string]struct{}, len(diagnosticsList))
	for _, diagnostic := range diagnosticsList {
		operation := strings.TrimSpace(string(diagnostic.Operation))
		if operation == "" {
			continue
		}
		if _, duplicate := seen[operation]; duplicate {
			continue
		}
		seen[operation] = struct{}{}
		warnings = append(warnings, diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            "extension.install.cleanup_failed",
			Code:          diagnosticcontract.CodeExtensionInstallCleanupFailed,
			Category:      diagnosticcontract.CategoryExtension,
			Title:         "Extension installed; cleanup incomplete",
			Message:       "The extension is installed, but one cleanup step did not complete.",
			Severity:      diagnosticcontract.SeverityWarn,
			DataFreshness: diagnosticcontract.FreshnessLive,
		},
			diagnostics.WithEvidence(map[string]any{
				marketplaceDiagnosticSlugKey:       strings.TrimSpace(slug),
				marketplaceInstallCleanupOperation: operation,
			}),
		))
	}
	return warnings
}
