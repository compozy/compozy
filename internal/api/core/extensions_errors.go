package core

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
	registrygit "github.com/compozy/compozy/internal/registry/gitsrc"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

// ExtensionStatusCode maps extension-domain errors onto transport status codes.
func ExtensionStatusCode(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case isExtensionNotFoundError(err):
		return http.StatusNotFound
	case isExtensionConflictError(err):
		return http.StatusConflict
	case isExtensionUnprocessableError(err):
		return http.StatusUnprocessableEntity
	case isExtensionBadRequestError(err):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrExtensionWorkspaceDenied):
		return http.StatusForbidden
	case isExtensionUnavailableError(err):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func isExtensionNotFoundError(err error) bool {
	return errors.Is(err, extensionpkg.ErrExtensionNotFound) ||
		errors.Is(err, marketplacepkg.ErrEntryNotFound) ||
		errors.Is(err, registrypkg.ErrPackageNotFound)
}

func isExtensionConflictError(err error) bool {
	return errors.Is(err, extensionpkg.ErrExtensionExists) ||
		errors.Is(err, extensionpkg.ErrExtensionHasActiveBundles) ||
		errors.Is(err, extensionpkg.ErrExtensionNotDevLinked) ||
		errors.Is(err, extensionpkg.ErrExtensionDevOriginMissing) ||
		errors.Is(err, extensionpkg.ErrExtensionNetworkConfirmationRequired) ||
		errors.Is(err, extensionpkg.ErrExtensionAgentConflict)
}

func isExtensionUnprocessableError(err error) bool {
	return errors.Is(err, extensionpkg.ErrExtensionChecksumUnverified) ||
		errors.Is(err, extensionpkg.ErrExtensionUnverifiedPolicyBlocked)
}

func isExtensionBadRequestError(err error) bool {
	return errors.Is(err, extensionpkg.ErrExtensionChecksumMismatch) ||
		errors.Is(err, extensionpkg.ErrExtensionArchiveDigestMismatch) ||
		errors.Is(err, extensionpkg.ErrManifestInvalid) ||
		errors.Is(err, extensionpkg.ErrManifestIncompatible) ||
		errors.Is(err, extensionpkg.ErrManifestNotFound) ||
		errors.Is(err, extensionpkg.ErrExtensionGenerationInvalid) ||
		errors.Is(err, extensionpkg.ErrExtensionSearchInvalid) ||
		errors.Is(err, extensionpkg.ErrExtensionEnvBindingInvalid) ||
		errors.Is(err, extensionpkg.ErrExtensionEnvBindingUndeclared) ||
		errors.Is(err, extensionpkg.ErrExtensionEnvBindingDangling) ||
		errors.Is(err, os.ErrNotExist)
}

func isExtensionUnavailableError(err error) bool {
	return errors.Is(err, extensionpkg.ErrMarketplaceSourceUnavailable) ||
		errors.Is(err, registrygit.ErrGitUnavailable)
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
		var validationErr *extensionpkg.ManifestValidationError
		if errors.As(err, &validationErr) && validationErr != nil {
			issue.Field = strings.TrimSpace(validationErr.Field)
		}
		c.JSON(status, contract.ExtensionValidationErrorPayload{
			Error: payload.Error, Diagnostic: payload.Diagnostic, Issues: []contract.ValidationIssue{issue},
		})
		return
	}
	if errors.Is(err, registrygit.ErrGitUnavailable) {
		payload := ErrorPayloadForStatus(status, err, mask)
		item := diagnosticspkg.NewItem(
			"extension.git_unavailable",
			diagnosticcontract.CodeExtensionGitUnavailable,
			diagnosticcontract.CategoryExtension,
			"Git is unavailable",
			"Install Git and ensure the git executable is available on PATH.",
			diagnosticcontract.SeverityError,
			diagnosticcontract.FreshnessLive,
			diagnosticspkg.WithSuggestedCommand("git --version"),
		)
		payload.Diagnostic = &item
		c.JSON(status, payload)
		return
	}
	RespondError(c, status, err, mask)
}
