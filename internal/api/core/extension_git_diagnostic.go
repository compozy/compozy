package core

import (
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	registrygit "github.com/compozy/compozy/internal/registry/gitsrc"
)

// ExtensionGitDependencyDiagnostic returns the canonical public Git remediation.
func ExtensionGitDependencyDiagnostic(err error) (contract.DiagnosticItem, bool) {
	spec := diagnosticspkg.ItemSpec{
		ID:            "extension.git_unavailable",
		Code:          diagnosticcontract.CodeExtensionGitUnavailable,
		Category:      diagnosticcontract.CategoryExtension,
		Title:         "Git is unavailable",
		Message:       "Install Git and ensure the git executable is available on PATH.",
		Severity:      diagnosticcontract.SeverityError,
		DataFreshness: diagnosticcontract.FreshnessLive,
	}
	switch {
	case errors.Is(err, registrygit.ErrGitVersionUnsupported):
		spec.ID = "extension.git_version_unsupported"
		spec.Code = diagnosticcontract.CodeExtensionGitVersionUnsupported
		spec.Title = "Git is too old"
		spec.Message = "Install Git 2.37 or newer and ensure the git executable is available on PATH."
	case errors.Is(err, registrygit.ErrGitUnavailable):
	default:
		return diagnosticspkg.EmptyItem(), false
	}
	return diagnosticspkg.NewItem(spec, diagnosticspkg.WithSuggestedCommand("git --version")), true
}
