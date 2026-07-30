package gitsrc

import (
	"errors"
	"fmt"
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

func newGitUnavailableError(err error) error {
	if err == nil {
		err = fmt.Errorf("%w", ErrGitUnavailable)
	}
	return &gitUnavailableError{cause: err}
}
