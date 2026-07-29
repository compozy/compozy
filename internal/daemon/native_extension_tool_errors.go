package daemon

import (
	"errors"
	"fmt"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeExtensionToolError(id toolspkg.ToolID, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, extensionpkg.ErrExtensionNotFound):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolNotFound, err),
			toolspkg.ReasonExtensionNotInstalled,
		)
	case errors.Is(err, extensionpkg.ErrExtensionExists),
		errors.Is(err, extensionpkg.ErrExtensionChecksumMismatch),
		errors.Is(err, extensionpkg.ErrExtensionArchiveDigestMismatch):
		return nativeExtensionValidationError(id, err)
	case errors.Is(err, extensionpkg.ErrExtensionHasActiveBundles),
		errors.Is(err, extensionpkg.ErrExtensionDevOriginMissing),
		errors.Is(err, extensionpkg.ErrExtensionNotDevLinked):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
			toolspkg.ReasonExtensionValidationFailed,
		)
	case errors.Is(err, extensionpkg.ErrExtensionGenerationInvalid):
		return nativeExtensionValidationError(id, err)
	case errors.Is(err, extensionpkg.ErrExtensionWorkspaceDenied):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			toolspkg.ReasonExtensionSourceForbidden,
		)
	case isExtensionSourceError(err):
		return nativeExtensionSourceError(id, err)
	default:
		return err
	}
}

func nativeExtensionValidationError(id toolspkg.ToolID, err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		"extension validation failed",
		fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
		toolspkg.ReasonExtensionValidationFailed,
	)
}

func nativeExtensionSourceError(id toolspkg.ToolID, err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		"extension source is not allowed",
		fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
		toolspkg.ReasonExtensionSourceForbidden,
	)
}

func isExtensionSourceError(err error) bool {
	return errors.Is(err, extensionpkg.ErrMarketplaceSourceUnavailable) ||
		errors.Is(err, extensionpkg.ErrExtensionChecksumUnverified) ||
		errors.Is(err, extensionpkg.ErrExtensionUnverifiedPolicyBlocked) ||
		errors.Is(err, errExtensionMarketplaceNotConfigured) ||
		errors.Is(err, errExtensionRegistryUnsupported)
}
