package gitsrc

import (
	"errors"
	"fmt"

	contract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
)

// ErrGitUnavailable reports that the required git executable cannot be found.
var ErrGitUnavailable = errors.New("gitsrc: git executable unavailable")

type gitUnavailableError struct {
	cause error
}

func (e *gitUnavailableError) Error() string {
	return "gitsrc: git executable unavailable; install Git and ensure it is available on PATH"
}

func (e *gitUnavailableError) Unwrap() []error {
	if e == nil || e.cause == nil {
		return []error{ErrGitUnavailable}
	}
	return []error{ErrGitUnavailable, e.cause}
}

func (e *gitUnavailableError) DiagnosticItem() contract.DiagnosticItem {
	return diagnostics.NewItem(
		"extension.git_unavailable",
		"extension_git_unavailable",
		contract.CategoryExtension,
		"Git is unavailable",
		"Install Git and ensure the git executable is available on PATH.",
		contract.SeverityError,
		contract.FreshnessLive,
		diagnostics.WithSuggestedCommand("git --version"),
	)
}

func newGitUnavailableError(err error) error {
	if err == nil {
		err = fmt.Errorf("%w", ErrGitUnavailable)
	}
	return &gitUnavailableError{cause: err}
}
