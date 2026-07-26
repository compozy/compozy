package extensionpkg

import (
	"github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
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
	return diagnostics.NewItem(
		"extension.remove.cleanup_failed",
		diagnosticcontract.CodeExtensionRemoveCleanupFailed,
		diagnosticcontract.CategoryExtension,
		"Extension removed; cleanup incomplete",
		diagnostics.RedactAndBound(err.Error(), 1024),
		diagnosticcontract.SeverityWarn,
		diagnosticcontract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			managerExtensionKey: extensionName,
			"cleanup_target":    marketplaceUpdateCleanupBackup,
			"path":              path,
		}),
	)
}
