package testutil

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
)

var reportFailure = func(t *testing.T, format string, args ...any) {
	t.Fatalf(format, args...)
}

// RequireNoError fails the test if err is non-nil. Optional msgAndArgs provide context in the failure message.
func RequireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		return
	}
	if len(msgAndArgs) > 0 {
		reportFailure(t, "%s: %v", fmt.Sprint(msgAndArgs...), err)
		return
	}
	reportFailure(t, "unexpected error: %v", err)
}

// RequireValidationError fails when err is nil or does not wrap a BuildError. It optionally enforces that the
// rendered error message contains the provided substring.
func RequireValidationError(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil {
		reportFailure(t, "expected validation error")
		return
	}
	var buildErr *sdkerrors.BuildError
	if !errors.As(err, &buildErr) {
		reportFailure(t, "expected build error, got %T", err)
		return
	}
	if contains != "" && !strings.Contains(err.Error(), contains) {
		reportFailure(t, "expected error containing %q, got %v", contains, err)
	}
}

// AssertBuildError verifies that err is a BuildError containing messages for each expected entry.
func AssertBuildError(t *testing.T, err error, expectedErrors []string) {
	t.Helper()
	if err == nil {
		reportFailure(t, "expected build error")
		return
	}
	var buildErr *sdkerrors.BuildError
	if !errors.As(err, &buildErr) {
		reportFailure(t, "expected build error, got %T", err)
		return
	}
	for _, want := range expectedErrors {
		matched := false
		for _, inner := range buildErr.Errors {
			if inner == nil {
				continue
			}
			if strings.Contains(inner.Error(), want) {
				matched = true
				break
			}
		}
		if !matched {
			reportFailure(t, "expected build error containing %q, got %v", want, err)
		}
	}
}
