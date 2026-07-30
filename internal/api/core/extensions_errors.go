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
	case errors.Is(err, extensionpkg.ErrExtensionNotFound):
		return http.StatusNotFound
	case errors.Is(err, marketplacepkg.ErrEntryNotFound), errors.Is(err, registrypkg.ErrPackageNotFound):
		return http.StatusNotFound
	case errors.Is(err, extensionpkg.ErrExtensionExists):
		return http.StatusConflict
	case errors.Is(err, extensionpkg.ErrExtensionChecksumMismatch):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrExtensionArchiveDigestMismatch):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrExtensionChecksumUnverified),
		errors.Is(err, extensionpkg.ErrExtensionUnverifiedPolicyBlocked):
		return http.StatusUnprocessableEntity
	case errors.Is(err, extensionpkg.ErrManifestInvalid):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrManifestIncompatible):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrManifestNotFound):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrExtensionHasActiveBundles):
		return http.StatusConflict
	case errors.Is(err, extensionpkg.ErrExtensionNotDevLinked):
		return http.StatusConflict
	case errors.Is(err, extensionpkg.ErrExtensionDevOriginMissing):
		return http.StatusConflict
	case errors.Is(err, extensionpkg.ErrExtensionGenerationInvalid):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrExtensionWorkspaceDenied):
		return http.StatusForbidden
	case errors.Is(err, extensionpkg.ErrMarketplaceSourceUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, registrygit.ErrGitUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, extensionpkg.ErrExtensionSearchInvalid):
		return http.StatusBadRequest
	case errors.Is(err, os.ErrNotExist):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func (h *BaseHandlers) respondExtensionError(c *gin.Context, status int, err error) {
	mask := false
	if h != nil {
		mask = h.MaskInternalErrors
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
