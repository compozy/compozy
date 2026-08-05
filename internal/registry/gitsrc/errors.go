package gitsrc

import (
	"errors"
	"fmt"
)

// ErrGitUnavailable reports that the required git executable cannot be found.
var ErrGitUnavailable = errors.New("gitsrc: git executable unavailable")

// ErrGitVersionUnsupported reports a Git binary too old to pin approved DNS answers.
var ErrGitVersionUnsupported = errors.New("gitsrc: Git 2.37 or newer is required")

var (
	// ErrInvalidRepositoryRef reports a repository URL outside the public HTTPS grammar.
	ErrInvalidRepositoryRef = errors.New("gitsrc: invalid repository URL")
	// ErrRepositoryDestinationBlocked reports a repository whose resolved addresses violate policy.
	ErrRepositoryDestinationBlocked = errors.New("gitsrc: repository destination blocked")
	// ErrRepositoryTooLarge reports that the checked-out repository exceeds the archive content budget.
	ErrRepositoryTooLarge = errors.New("gitsrc: repository exceeds max uncompressed size")
	// ErrRepositoryTooManyFiles reports that the checked-out repository exceeds the archive entry budget.
	ErrRepositoryTooManyFiles = errors.New("gitsrc: repository exceeds max file count")
)

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

func newGitUnavailableError(err error) error {
	if err == nil {
		err = fmt.Errorf("%w", ErrGitUnavailable)
	}
	return &gitUnavailableError{cause: err}
}
