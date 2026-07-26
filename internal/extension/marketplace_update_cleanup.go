package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"strings"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
)

const (
	marketplaceUpdateCleanupBackup  = "backup"
	marketplaceUpdateCleanupStaging = "staging"
)

type marketplaceUpdateApplyResult struct {
	remoteVersion string
	warnings      []diagnosticcontract.DiagnosticItem
	committed     bool
}

type marketplaceUpdateCleanup struct {
	commitChange  func(*stagedExtensionDirChange) error
	removeStaging func(string) error
}

func marketplaceUpdateCleanupForRequest(req MarketplaceUpdateRequest) marketplaceUpdateCleanup {
	commitChange := req.commitChange
	if commitChange == nil {
		commitChange = func(change *stagedExtensionDirChange) error {
			return change.Commit()
		}
	}
	removeStaging := req.removeStaging
	if removeStaging == nil {
		removeStaging = os.RemoveAll
	}
	return marketplaceUpdateCleanup{commitChange: commitChange, removeStaging: removeStaging}
}

func (c marketplaceUpdateCleanup) removeStagingDir(path string) error {
	err := c.removeStaging(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("extension: remove staged extension directory %q: %w", path, err)
}

func finalizeMarketplaceUpdateStagingCleanup(
	out *marketplaceUpdateApplyResult,
	errTarget *error,
	cleanup marketplaceUpdateCleanup,
	extensionName string,
	stagingDir string,
) {
	cleanupErr := cleanup.removeStagingDir(stagingDir)
	if cleanupErr == nil {
		return
	}
	if out != nil && out.committed {
		out.warnings = append(out.warnings, marketplaceUpdateCleanupWarning(
			extensionName,
			marketplaceUpdateCleanupStaging,
			stagingDir,
			cleanupErr,
		))
		return
	}
	if errTarget != nil {
		*errTarget = errors.Join(*errTarget, cleanupErr)
	}
}

func marketplaceUpdateCleanupWarning(
	extensionName string,
	target string,
	path string,
	err error,
) diagnosticcontract.DiagnosticItem {
	return diagnostics.NewItem(
		"extension.update.cleanup_failed",
		diagnosticcontract.CodeExtensionUpdateCleanupFailed,
		diagnosticcontract.CategoryExtension,
		"Extension updated; cleanup incomplete",
		diagnostics.RedactAndBound(err.Error(), 1024),
		diagnosticcontract.SeverityWarn,
		diagnosticcontract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			managerExtensionKey: strings.TrimSpace(extensionName),
			"cleanup_target":    strings.TrimSpace(target),
			"path":              strings.TrimSpace(path),
		}),
	)
}
