package registry

import (
	"errors"
	"fmt"
	"log/slog"
)

// CleanupOperation identifies a degraded best-effort cleanup step without
// exposing filesystem paths or underlying error text.
type CleanupOperation string

const (
	CleanupOperationStaleTemp      CleanupOperation = "remove_stale_temp"
	CleanupOperationStagingTemp    CleanupOperation = "remove_staging_temp"
	CleanupOperationExtractionRoot CleanupOperation = "close_extraction_root"
	CleanupOperationBackup         CleanupOperation = "remove_replaced_backup"
	CleanupOperationStream         CleanupOperation = "close_download_stream"
	CleanupOperationPublication    CleanupOperation = "finalize_published_install"
	CleanupOperationSourceParent   CleanupOperation = "close_install_source_parent"
	CleanupOperationTargetParent   CleanupOperation = "close_install_target_parent"
)

// CleanupDiagnostic reports completed-install cleanup degradation.
type CleanupDiagnostic struct {
	Operation CleanupOperation `json:"operation"`
}

type cleanupFailureError struct {
	operation CleanupOperation
	cause     error
}

func (e *cleanupFailureError) Error() string {
	if e == nil {
		return "registry: cleanup failed"
	}
	return fmt.Sprintf("registry: cleanup failed during %s", e.operation)
}

func (e *cleanupFailureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func appendInstallCleanup(result *InstallResult, operation CleanupOperation) {
	if result == nil {
		return
	}
	var appended bool
	result.CleanupDiagnostics, appended = appendCleanupDiagnostic(result.CleanupDiagnostics, operation)
	if appended {
		reportInstallCleanup(operation)
	}
}

func appendCleanupDiagnostic(
	diagnostics []CleanupDiagnostic,
	operation CleanupOperation,
) ([]CleanupDiagnostic, bool) {
	for _, diagnostic := range diagnostics {
		if diagnostic.Operation == operation {
			return diagnostics, false
		}
	}
	return append(diagnostics, CleanupDiagnostic{Operation: operation}), true
}

func reportInstallCleanup(operation CleanupOperation) {
	slog.Warn("registry cleanup degraded", "operation", operation)
}

func recordInstallCleanupFailure(operation CleanupOperation, cause error) error {
	if cause == nil {
		return nil
	}
	slog.Warn("registry cleanup degraded", "operation", operation, "error", cause)
	return &cleanupFailureError{operation: operation, cause: cause}
}

func joinCleanupFailure(base error, operation CleanupOperation, cause error) error {
	return errors.Join(base, recordInstallCleanupFailure(operation, cause))
}
