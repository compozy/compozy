package extensionpkg

import (
	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
)

const managedRemoveStatusRemoved = "removed"

func finalizeManagedExtensionRemoval(
	extensionName string,
	installDir string,
	change *stagedExtensionDirChange,
) ManagedRemoveResult {
	result := ManagedRemoveResult{Name: extensionName, Path: installDir, Status: managedRemoveStatusRemoved}
	if cleanupErr := change.Commit(); cleanupErr != nil {
		result.Warnings = []diagnosticcontract.DiagnosticItem{
			marketplaceRemoveCleanupWarning(extensionName, change.backupDir, cleanupErr),
		}
	}
	return result
}

func marketplaceRemoveCleanupWarning(
	extensionName string,
	path string,
	err error,
) diagnosticcontract.DiagnosticItem {
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            "extension.remove.cleanup_failed",
		Code:          diagnosticcontract.CodeExtensionRemoveCleanupFailed,
		Category:      diagnosticcontract.CategoryExtension,
		Title:         "Extension removed; cleanup incomplete",
		Message:       diagnostics.RedactAndBound(err.Error(), 1024),
		Severity:      diagnosticcontract.SeverityWarn,
		DataFreshness: diagnosticcontract.FreshnessLive,
	},
		diagnostics.WithEvidence(map[string]any{
			managerExtensionKey:         extensionName,
			marketplaceCleanupTargetKey: marketplaceUpdateCleanupBackup,
			manifestPathKey:             path,
		}),
	)
}

func extensionDataCleanupWarning(
	extensionName string,
	path string,
	err error,
) diagnosticcontract.DiagnosticItem {
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            "extension.remove.data_cleanup_failed",
		Code:          diagnosticcontract.CodeExtensionRemoveCleanupFailed,
		Category:      diagnosticcontract.CategoryExtension,
		Title:         "Extension removed; data quarantined",
		Message:       diagnostics.RedactAndBound(err.Error(), 1024),
		Severity:      diagnosticcontract.SeverityWarn,
		DataFreshness: diagnosticcontract.FreshnessLive,
	}, diagnostics.WithEvidence(map[string]any{
		managerExtensionKey:         extensionName,
		marketplaceCleanupTargetKey: "extension_data",
		manifestPathKey:             path,
	}))
}
