package daemon

import (
	"errors"
	"fmt"
	"os"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	registrygit "github.com/compozy/compozy/internal/registry/gitsrc"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeExtensionUpdatePartialPayload struct {
	Updates        []contract.ManagedExtensionUpdatePayload `json:"updates"`
	FailedTarget   string                                   `json:"failed_target"`
	CompletedCount int                                      `json:"completed_count"`
}

func nativeExtensionUpdateToolError(
	id toolspkg.ToolID,
	items []contract.ManagedExtensionUpdatePayload,
	err error,
) error {
	batchErr, ok := errors.AsType[*extensionpkg.MarketplaceUpdateBatchError](err)
	if !ok {
		return nativeExtensionToolError(id, err)
	}

	mappedErr := nativeExtensionToolError(id, err)
	toolErr, ok := errors.AsType[*toolspkg.ToolError](mappedErr)
	if !ok || toolErr == nil {
		mappedErr = nativeHTTPStatusToolError(id, err, core.ExtensionStatusCode(err))
		toolErr, ok = errors.AsType[*toolspkg.ToolError](mappedErr)
		if !ok || toolErr == nil {
			return mappedErr
		}
	}
	partial, marshalErr := structuredResult(nativeExtensionUpdatePartialPayload{
		Updates:        append([]contract.ManagedExtensionUpdatePayload(nil), items...),
		FailedTarget:   batchErr.FailedName,
		CompletedCount: len(items),
	}, fmt.Sprintf("%d extension updates completed before %s failed", len(items), batchErr.FailedName))
	if marshalErr != nil {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			id,
			"extension update partial result could not be encoded",
			fmt.Errorf("%w: %w", toolspkg.ErrToolBackendFailed, errors.Join(err, marshalErr)),
			toolspkg.ReasonBackendUnhealthy,
		)
	}
	return toolErr.WithPartialResult(partial)
}

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
	case core.ExtensionAgentPluginErrorCode(err) != "":
		return toolspkg.NewToolError(
			toolspkg.ErrorCode(core.ExtensionAgentPluginErrorCode(err)),
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonExtensionValidationFailed,
		)
	case isExtensionValidationError(err):
		return nativeExtensionValidationError(id, err)
	case errors.Is(err, extensionpkg.ErrExtensionNetworkConfirmationRequired),
		errors.Is(err, extensionpkg.ErrExtensionAgentConflict):
		return nativeHTTPStatusToolError(id, err, core.ExtensionStatusCode(err))
	case errors.Is(err, extensionpkg.ErrExtensionEnvBindingInvalid),
		errors.Is(err, extensionpkg.ErrExtensionEnvBindingUndeclared),
		errors.Is(err, extensionpkg.ErrExtensionEnvBindingDangling):
		return nativeHTTPStatusToolError(id, err, core.ExtensionStatusCode(err))
	case errors.Is(err, extensionpkg.ErrExtensionDevOriginMissing),
		errors.Is(err, extensionpkg.ErrExtensionNotDevLinked):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
			toolspkg.ReasonExtensionValidationFailed,
		)
	case errors.Is(err, extensionpkg.ErrExtensionWorkspaceDenied):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			toolspkg.ReasonExtensionSourceForbidden,
		)
	case errors.Is(err, registrygit.ErrGitUnavailable),
		errors.Is(err, registrygit.ErrGitVersionUnsupported):
		return nativeExtensionGitDependencyError(id, err)
	case isExtensionSourceError(err):
		return nativeExtensionSourceError(id, err)
	default:
		return err
	}
}

func nativeExtensionGitDependencyError(id toolspkg.ToolID, err error) error {
	diagnostic, ok := core.ExtensionGitDependencyDiagnostic(err)
	if !ok {
		return nativeHTTPStatusToolError(id, err, core.ExtensionStatusCode(err))
	}
	reason := toolspkg.ReasonExtensionGitUnavailable
	if errors.Is(err, registrygit.ErrGitVersionUnsupported) {
		reason = toolspkg.ReasonExtensionGitVersionUnsupported
	}
	recovery := diagnostic.Message
	if diagnostic.SuggestedCommand != "" {
		recovery += " Run `" + diagnostic.SuggestedCommand + "`."
	}
	return toolspkg.NewOperatorToolError(
		toolspkg.ErrorCodeUnavailable,
		id,
		diagnostic.Title,
		fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, err),
		diagnostic.Title,
		recovery,
		reason,
	)
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

func isExtensionValidationError(err error) bool {
	return errors.Is(err, extensionpkg.ErrExtensionExists) ||
		errors.Is(err, extensionpkg.ErrExtensionChecksumMismatch) ||
		errors.Is(err, extensionpkg.ErrExtensionArchiveDigestMismatch) ||
		errors.Is(err, extensionpkg.ErrExtensionGenerationInvalid) ||
		errors.Is(err, extensionpkg.ErrManifestInvalid) ||
		errors.Is(err, extensionpkg.ErrManifestIncompatible) ||
		errors.Is(err, extensionpkg.ErrManifestNotFound) ||
		errors.Is(err, registrygit.ErrInvalidRepositoryRef) ||
		errors.Is(err, os.ErrNotExist)
}

func isExtensionSourceError(err error) bool {
	return errors.Is(err, extensionpkg.ErrMarketplaceSourceUnavailable) ||
		errors.Is(err, extensionpkg.ErrExtensionChecksumUnverified) ||
		errors.Is(err, extensionpkg.ErrExtensionUnverifiedPolicyBlocked) ||
		errors.Is(err, registrygit.ErrRepositoryDestinationBlocked) ||
		errors.Is(err, errExtensionMarketplaceNotConfigured) ||
		errors.Is(err, errExtensionRegistryUnsupported)
}
