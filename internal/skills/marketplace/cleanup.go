package marketplace

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	registrypkg "github.com/compozy/compozy/internal/registry"
)

const (
	CleanupOperationMarketplaceStaging = "remove_marketplace_staging_root"
	CleanupOperationRegistrySource     = "close_marketplace_registry"
	marketplaceInstallStatusInstalled  = "installed"
)

// CleanupDiagnostic identifies a degraded cleanup after a successful skill mutation.
type CleanupDiagnostic struct {
	Operation string `json:"operation"`
}

func cleanupDiagnosticsFromRegistry(items []registrypkg.CleanupDiagnostic) []CleanupDiagnostic {
	return appendCleanupDiagnosticsFromRegistry(nil, items)
}

func appendCleanupDiagnosticsFromRegistry(
	diagnostics []CleanupDiagnostic,
	items []registrypkg.CleanupDiagnostic,
) []CleanupDiagnostic {
	for _, item := range items {
		diagnostics = appendCleanupDiagnostic(diagnostics, string(item.Operation))
	}
	return diagnostics
}

func appendCleanupDiagnostic(items []CleanupDiagnostic, operation string) []CleanupDiagnostic {
	normalized := strings.TrimSpace(operation)
	if normalized == "" {
		return items
	}
	for _, item := range items {
		if item.Operation == normalized {
			return items
		}
	}
	return append(items, CleanupDiagnostic{Operation: normalized})
}

func reportCleanupDiagnostic(operation string) {
	slog.Warn("skills marketplace cleanup degraded", "operation", strings.TrimSpace(operation))
}

func finalizeMarketplaceStagingCleanup(item *InstallResult, errTarget *error, path string) {
	removeErr := os.RemoveAll(path)
	if removeErr == nil {
		return
	}
	if item != nil && errTarget != nil && *errTarget == nil && item.Status == marketplaceInstallStatusInstalled {
		item.CleanupDiagnostics = appendCleanupDiagnostic(
			item.CleanupDiagnostics,
			CleanupOperationMarketplaceStaging,
		)
		reportCleanupDiagnostic(CleanupOperationMarketplaceStaging)
		return
	}
	if errTarget != nil {
		*errTarget = errors.Join(
			*errTarget,
			fmt.Errorf("remove temporary install directory %q: %w", path, removeErr),
		)
	}
}

func appendRegistrySourceCleanupToInstall(
	item *InstallResult,
	errTarget *error,
	closeErr error,
) {
	if closeErr == nil {
		return
	}
	if item != nil && errTarget != nil && *errTarget == nil && item.Status == marketplaceInstallStatusInstalled {
		item.CleanupDiagnostics = appendCleanupDiagnostic(
			item.CleanupDiagnostics,
			CleanupOperationRegistrySource,
		)
		reportCleanupDiagnostic(CleanupOperationRegistrySource)
		return
	}
	if errTarget != nil {
		*errTarget = errors.Join(*errTarget, closeErr)
	}
}

func appendRegistrySourceCleanupToUpdates(
	items []UpdateResult,
	errTarget *error,
	closeErr error,
) {
	if closeErr == nil {
		return
	}
	if errTarget != nil && *errTarget == nil && len(items) > 0 {
		for index := range items {
			items[index].CleanupDiagnostics = appendCleanupDiagnostic(
				items[index].CleanupDiagnostics,
				CleanupOperationRegistrySource,
			)
		}
		reportCleanupDiagnostic(CleanupOperationRegistrySource)
		return
	}
	if errTarget != nil {
		*errTarget = errors.Join(*errTarget, closeErr)
	}
}
