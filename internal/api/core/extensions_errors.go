package core

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
	registrygit "github.com/compozy/compozy/internal/registry/gitsrc"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

type extensionErrorKind uint8

const (
	extensionErrorUnknown extensionErrorKind = iota
	extensionErrorNotFound
	extensionErrorConflict
	extensionErrorUnprocessable
	extensionErrorBadRequest
	extensionErrorForbidden
	extensionErrorUnavailable
	extensionErrorNetworkConfirmationRequired
	extensionErrorAgentConflict
	extensionErrorEnvBindingUndeclared
	extensionErrorEnvBindingDangling
	extensionErrorEnvBindingInvalid
)

// ExtensionStatusCode maps extension-domain errors onto transport status codes.
func ExtensionStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	switch classifyExtensionError(err) {
	case extensionErrorNotFound:
		return http.StatusNotFound
	case extensionErrorConflict, extensionErrorNetworkConfirmationRequired, extensionErrorAgentConflict:
		return http.StatusConflict
	case extensionErrorUnprocessable:
		return http.StatusUnprocessableEntity
	case extensionErrorBadRequest, extensionErrorEnvBindingUndeclared,
		extensionErrorEnvBindingDangling, extensionErrorEnvBindingInvalid:
		return http.StatusBadRequest
	case extensionErrorForbidden:
		return http.StatusForbidden
	case extensionErrorUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func classifyExtensionError(err error) extensionErrorKind {
	switch {
	case errors.Is(err, extensionpkg.ErrExtensionNetworkConfirmationRequired):
		return extensionErrorNetworkConfirmationRequired
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
	case errors.Is(err, extensionpkg.ErrExtensionExists),
		errors.Is(err, extensionpkg.ErrExtensionNotDevLinked),
		errors.Is(err, extensionpkg.ErrExtensionDevOriginMissing):
		return extensionErrorConflict
	case errors.Is(err, extensionpkg.ErrExtensionChecksumUnverified),
		errors.Is(err, extensionpkg.ErrExtensionUnverifiedPolicyBlocked):
		return extensionErrorUnprocessable
	case errors.Is(err, extensionpkg.ErrExtensionChecksumMismatch),
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

func (h *BaseHandlers) respondExtensionError(c *gin.Context, status int, err error) {
	mask := false
	if h != nil {
		mask = h.MaskInternalErrors
	}
	if payload, ok := extensionOperationErrorPayload(c, status, err, mask); ok {
		c.JSON(status, payload)
		return
	}
	if errors.Is(err, extensionpkg.ErrManifestInvalid) {
		payload := ErrorPayloadForStatus(status, err, mask)
		issue := contract.ValidationIssue{
			Message:  diagnosticspkg.Redact(taskpkg.RedactClaimTokens(payload.Error)),
			Severity: contract.IssueSeverityError,
		}
		if validationErr, ok := errors.AsType[*extensionpkg.ManifestValidationError](err); ok && validationErr != nil {
			issue.Field = strings.TrimSpace(validationErr.Field)
		}
		c.JSON(status, contract.ExtensionValidationErrorPayload{
			Error: payload.Error, Diagnostic: payload.Diagnostic, Issues: []contract.ValidationIssue{issue},
		})
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
