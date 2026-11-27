package core

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extension/agentplugin"
	"github.com/compozy/compozy/internal/extensionmcp"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/marketplace/pluginsource"
	registrypkg "github.com/compozy/compozy/internal/registry"
	registrygit "github.com/compozy/compozy/internal/registry/gitsrc"
	"github.com/compozy/compozy/internal/resources"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

type extensionErrorKind uint8

const (
	extensionErrorUnknown extensionErrorKind = iota
	extensionErrorMCPNameTaken
	extensionErrorSourceChanged
	extensionErrorSourceUnreachable
	extensionErrorMarketplaceSourceNotFound
	extensionErrorNameConflict
	extensionErrorInputsRequired
	extensionErrorInputInvalid
	extensionErrorInputOverwriteDenied
	extensionErrorDependenciesRequired
	extensionErrorDependencyGraphInvalid
	extensionErrorDependencyInUse
	extensionErrorNotFound
	extensionErrorNotBuiltin
	extensionErrorBuiltinNotRemoved
	extensionErrorExists
	extensionErrorRestoreFailed
	extensionErrorRemovalCompensationFailed
	extensionErrorBuiltinRemovalCompensationFailed
	extensionErrorConflict
	extensionErrorUnprocessable
	extensionErrorBadRequest
	extensionErrorForbidden
	extensionErrorUnavailable
	extensionErrorKindRemoved
	extensionErrorAgentConflict
	extensionErrorEnvBindingUndeclared
	extensionErrorEnvBindingDangling
	extensionErrorEnvBindingInvalid
	extensionErrorAgentPluginNotManifest
	extensionErrorAgentPluginSchemaUnsupported
	extensionErrorAgentPluginManifestInvalid
)

// ExtensionStatusCode maps extension-domain errors onto transport status codes.
func ExtensionStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	switch classifyExtensionError(err) {
	case extensionErrorNotFound, extensionErrorMarketplaceSourceNotFound:
		return http.StatusNotFound
	case extensionErrorConflict,
		extensionErrorAgentConflict,
		extensionErrorBuiltinNotRemoved,
		extensionErrorExists,
		extensionErrorSourceChanged, extensionErrorNameConflict, extensionErrorDependencyInUse:
		return http.StatusConflict
	case extensionErrorKindRemoved, extensionErrorUnprocessable, extensionErrorMCPNameTaken, extensionErrorNotBuiltin,
		extensionErrorInputsRequired, extensionErrorInputInvalid,
		extensionErrorDependenciesRequired, extensionErrorDependencyGraphInvalid,
		extensionErrorAgentPluginNotManifest,
		extensionErrorAgentPluginSchemaUnsupported,
		extensionErrorAgentPluginManifestInvalid:
		return http.StatusUnprocessableEntity
	case extensionErrorBadRequest, extensionErrorEnvBindingUndeclared,
		extensionErrorEnvBindingDangling, extensionErrorEnvBindingInvalid:
		return http.StatusBadRequest
	case extensionErrorForbidden, extensionErrorInputOverwriteDenied:
		return http.StatusForbidden
	case extensionErrorUnavailable, extensionErrorSourceUnreachable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func classifyExtensionError(err error) extensionErrorKind {
	if kind := classifyExtensionPrecedenceError(err); kind != extensionErrorUnknown {
		return kind
	}
	if _, ok := errors.AsType[*extensionpkg.ManifestDecodeError](err); ok {
		return extensionErrorUnprocessable
	}
	switch {
	case errors.Is(err, extensionpkg.ErrExtensionBuiltinNotRemoved):
		return extensionErrorBuiltinNotRemoved
	case errors.Is(err, extensionpkg.ErrAgentPluginNotManifest):
		return extensionErrorAgentPluginNotManifest
	case errors.Is(err, agentplugin.ErrSchemaUnsupported):
		return extensionErrorAgentPluginSchemaUnsupported
	case errors.Is(err, extensionpkg.ErrAgentPluginManifestInvalid):
		return extensionErrorAgentPluginManifestInvalid
	case errors.Is(err, extensionpkg.ErrExtensionKindRemoved):
		return extensionErrorKindRemoved
	case errors.Is(err, extensionpkg.ErrExtensionAgentConflict):
		return extensionErrorAgentConflict
	case errors.Is(err, extensionpkg.ErrExtensionEnvBindingUndeclared):
		return extensionErrorEnvBindingUndeclared
	case errors.Is(err, extensionpkg.ErrExtensionEnvBindingDangling):
		return extensionErrorEnvBindingDangling
	case errors.Is(err, extensionpkg.ErrExtensionEnvBindingInvalid):
		return extensionErrorEnvBindingInvalid
	case errors.Is(err, extensionpkg.ErrExtensionNotFound),
		errors.Is(err, marketplacepkg.ErrEntryNotFound),
		errors.Is(err, registrypkg.ErrPackageNotFound):
		return extensionErrorNotFound
	case errors.Is(err, extensionpkg.ErrExtensionNotBuiltin):
		return extensionErrorNotBuiltin
	case errors.Is(err, extensionpkg.ErrExtensionExists):
		return extensionErrorExists
	case errors.Is(err, extensionpkg.ErrExtensionBuiltinAttached),
		errors.Is(err, extensionpkg.ErrExtensionNotDevLinked),
		errors.Is(err, extensionpkg.ErrExtensionDevOriginMissing):
		return extensionErrorConflict
	case errors.Is(err, extensionpkg.ErrExtensionChecksumUnverified),
		errors.Is(err, extensionpkg.ErrExtensionUnverifiedPolicyBlocked):
		return extensionErrorUnprocessable
	case errors.Is(err, resources.ErrValidation),
		errors.Is(err, marketplacepkg.ErrInstallSlugInvalid),
		errors.Is(err, extensionpkg.ErrExtensionChecksumMismatch),
		errors.Is(err, extensionpkg.ErrExtensionArchiveDigestMismatch),
		errors.Is(err, extensionpkg.ErrManifestInvalid),
		errors.Is(err, extensionpkg.ErrManifestIncompatible),
		errors.Is(err, extensionpkg.ErrManifestNotFound),
		errors.Is(err, extensionpkg.ErrExtensionGenerationInvalid),
		errors.Is(err, extensionpkg.ErrExtensionSearchInvalid),
		errors.Is(err, registrygit.ErrInvalidRepositoryRef),
		errors.Is(err, os.ErrNotExist):
		return extensionErrorBadRequest
	case errors.Is(err, extensionpkg.ErrExtensionWorkspaceDenied),
		errors.Is(err, registrygit.ErrRepositoryDestinationBlocked),
		errors.Is(err, taskpkg.ErrPermissionDenied):
		return extensionErrorForbidden
	case errors.Is(err, extensionpkg.ErrMarketplaceSourceUnavailable),
		errors.Is(err, registrygit.ErrGitUnavailable),
		errors.Is(err, registrygit.ErrGitVersionUnsupported):
		return extensionErrorUnavailable
	default:
		return extensionErrorUnknown
	}
}

// classifyExtensionPrecedenceError matches the kinds whose errors wrap a cause of any other kind, so they must win
// over the cause-typed kinds, in order: removal compensation failures, restore failures, then acquisition failures.
// A restore failure can wrap an acquisition cause such as missing inputs, and it must still answer 500.
func classifyExtensionPrecedenceError(err error) extensionErrorKind {
	switch {
	case errors.Is(err, extensionpkg.ErrExtensionBuiltinRemovalCompensationFailed):
		return extensionErrorBuiltinRemovalCompensationFailed
	case errors.Is(err, extensionpkg.ErrExtensionRemovalCompensationFailed):
		return extensionErrorRemovalCompensationFailed
	case errors.Is(err, extensionpkg.ErrExtensionRestoreFailed):
		return extensionErrorRestoreFailed
	default:
		return classifyExtensionAcquisitionError(err)
	}
}

func classifyExtensionAcquisitionError(err error) extensionErrorKind {
	switch {
	case errors.Is(err, marketplacepkg.ErrSourceNotFound):
		return extensionErrorMarketplaceSourceNotFound
	case errors.Is(err, pluginsource.ErrSourceUnreachable):
		return extensionErrorSourceUnreachable
	case errors.Is(err, extensionmcp.ErrNameTaken):
		return extensionErrorMCPNameTaken
	case errors.Is(err, extensionpkg.ErrExtensionNameConflict):
		return extensionErrorNameConflict
	case errors.Is(err, extensionpkg.ErrExtensionSourceChanged):
		return extensionErrorSourceChanged
	case errors.Is(err, extensionpkg.ErrExtensionInputsRequired):
		return extensionErrorInputsRequired
	case errors.Is(err, extensionpkg.ErrExtensionInputInvalid):
		return extensionErrorInputInvalid
	case errors.Is(err, extensionpkg.ErrExtensionInputOverwriteDenied):
		return extensionErrorInputOverwriteDenied
	case errors.Is(err, extensionpkg.ErrExtensionDependenciesRequired):
		return extensionErrorDependenciesRequired
	case errors.Is(err, extensionpkg.ErrExtensionDependencyGraphInvalid):
		return extensionErrorDependencyGraphInvalid
	case errors.Is(err, extensionpkg.ErrExtensionDependencyInUse):
		return extensionErrorDependencyInUse
	default:
		return extensionErrorUnknown
	}
}

func (h *BaseHandlers) respondExtensionError(c *gin.Context, status int, err error) {
	mask := false
	if h != nil {
		mask = h.MaskInternalErrors
	}
	if payload, ok := extensionOperationErrorPayload(c, status, err, mask); ok {
		c.JSON(status, payload)
		return
	}
	if errors.Is(err, extensionpkg.ErrAgentPluginManifestInvalid) {
		c.JSON(status, agentPluginManifestErrorPayload(status, err, mask))
		return
	}
	if errors.Is(err, extensionpkg.ErrManifestInvalid) {
		c.JSON(status, ExtensionValidationErrorPayload(status, err, mask))
		return
	}
	if item, ok := ExtensionGitDependencyDiagnostic(err); ok {
		payload := ErrorPayloadForStatus(status, err, mask)
		payload.Diagnostic = &item
		c.JSON(status, payload)
		return
	}
	RespondError(c, status, err, mask)
}

// ExtensionValidationErrorPayload builds the positioned validation payload of a manifest validation failure, the
// same on every public transport, naming the dependency it belongs to.
func ExtensionValidationErrorPayload(
	status int, err error, maskInternalErrors bool,
) contract.ExtensionValidationErrorPayload {
	payload := ErrorPayloadForStatus(status, err, maskInternalErrors)
	message := diagnosticspkg.Redact(taskpkg.RedactClaimTokens(payload.Error))
	issue := contract.ValidationIssue{Message: message, Severity: contract.IssueSeverityError}
	if decodeErr, ok := errors.AsType[*extensionpkg.ManifestDecodeError](
		err,
	); ok &&
		status < http.StatusInternalServerError {
		payload.Error = diagnosticspkg.Redact(taskpkg.RedactClaimTokens(decodeErr.PackageMessage(err.Error())))
		issue = decodeErr.Issue(payload.Error)
	} else if validationErr, ok := errors.AsType[*extensionpkg.ManifestValidationError](
		err,
	); ok &&
		validationErr != nil {
		issue.Field = strings.TrimSpace(validationErr.Field)
	}
	validation := contract.ExtensionValidationErrorPayload{
		Error: payload.Error, Diagnostic: payload.Diagnostic, Issues: []contract.ValidationIssue{issue},
	}
	validation.Dependency, validation.DependenciesInstalled = dependencyErrorFields(err)
	return validation
}
