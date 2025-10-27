package errors

import (
	"errors"
	"fmt"
	"strings"
)

// BuildError aggregates configuration errors gathered during fluent builder
// invocations and reports them in a single return value from Build calls.
type BuildError struct {
	Errors []error
}

// Error renders the aggregated error list using a concise, developer-friendly
// message so callers understand why a build failed.
func (e *BuildError) Error() string {
	if e == nil {
		return "build failed"
	}

	errs := e.nonNilErrors()
	if len(errs) == 0 {
		return "build failed"
	}

	if len(errs) == 1 {
		return fmt.Sprintf("build failed: %v", errs[0])
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("build failed with %d errors:\n", len(errs)))
	for idx, err := range errs {
		builder.WriteString(fmt.Sprintf("  %d. %v", idx+1, err))
		if idx < len(errs)-1 {
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}

// Unwrap exposes the first aggregated error so standard helpers can unwrap the
// chain while preserving compatibility with errors.Is and errors.As.
func (e *BuildError) Unwrap() error {
	if e == nil {
		return nil
	}

	errs := e.nonNilErrors()
	if len(errs) == 0 {
		return nil
	}

	return errs[0]
}

// Is allows errors.Is to match against any aggregated error.
func (e *BuildError) Is(target error) bool {
	if e == nil {
		return false
	}

	for _, err := range e.nonNilErrors() {
		if errors.Is(err, target) {
			return true
		}
	}

	return false
}

// As allows errors.As to project any aggregated error into the provided
// target.
func (e *BuildError) As(target any) bool {
	if e == nil {
		return false
	}

	for _, err := range e.nonNilErrors() {
		if errors.As(err, target) {
			return true
		}
	}

	return false
}

func (e *BuildError) nonNilErrors() []error {
	if e == nil {
		return nil
	}

	filtered := make([]error, 0, len(e.Errors))
	for _, err := range e.Errors {
		if err != nil {
			filtered = append(filtered, err)
		}
	}

	return filtered
}
