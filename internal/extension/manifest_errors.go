package extensionpkg

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ManifestNotFoundError describes a missing manifest directory.
type ManifestNotFoundError struct {
	Dir   string
	Paths []string
}

// ManifestValidationError describes an invalid manifest field.
type ManifestValidationError struct {
	Field   string
	Value   string
	Message string
}

// ManifestDecodeError describes a malformed manifest and its source location.
type ManifestDecodeError struct {
	path   string
	line   int
	column int
	cause  error
}

var _ error = (*ManifestDecodeError)(nil)

func (e *ManifestDecodeError) Error() string {
	return e.message(e.path)
}

func (e *ManifestDecodeError) Is(target error) bool { return target == ErrManifestInvalid }

func (e *ManifestDecodeError) Unwrap() error { return e.cause }

// PackageMessage identifies the manifest by its package-relative filename.
func (e *ManifestDecodeError) PackageMessage() string { return e.message(filepath.Base(e.path)) }

func (e *ManifestDecodeError) message(path string) string {
	return fmt.Sprintf("extension: decode manifest %q: %v", path, e.cause)
}

// Issue combines the source location with the caller's redacted message.
func (e *ManifestDecodeError) Issue(message string) ValidationIssue {
	return ValidationIssue{
		Path: filepath.Base(e.path), Line: e.line, Column: e.column, Message: message, Severity: IssueSeverityError,
	}
}

// ManifestCompatibilityError describes a daemon-version compatibility failure.
type ManifestCompatibilityError struct {
	CurrentVersion string
	MinVersion     string
}

// Error returns the typed missing-manifest message.
func (e *ManifestNotFoundError) Error() string {
	if len(e.Paths) == 0 {
		return fmt.Sprintf("%s in %q", ErrManifestNotFound, e.Dir)
	}
	return fmt.Sprintf("%s in %q (tried %s)", ErrManifestNotFound, e.Dir, strings.Join(e.Paths, ", "))
}

// Is matches sentinel errors for errors.Is.
func (e *ManifestNotFoundError) Is(target error) bool {
	return target == ErrManifestNotFound
}

// Error returns the field-specific validation message.
func (e *ManifestValidationError) Error() string {
	base := fmt.Sprintf("%s field %q", ErrManifestInvalid, e.Field)
	if strings.TrimSpace(e.Value) != "" {
		base = fmt.Sprintf("%s (value %q)", base, e.Value)
	}
	if strings.TrimSpace(e.Message) == "" {
		return base
	}
	return fmt.Sprintf("%s: %s", base, e.Message)
}

// Is matches sentinel errors for errors.Is.
func (e *ManifestValidationError) Is(target error) bool {
	return target == ErrManifestInvalid
}

// Error returns the daemon-version compatibility message.
func (e *ManifestCompatibilityError) Error() string {
	return fmt.Sprintf(
		"%s: current daemon version %q does not satisfy min_compozy_version %q",
		ErrManifestIncompatible,
		e.CurrentVersion,
		e.MinVersion,
	)
}

// Is matches sentinel errors for errors.Is.
func (e *ManifestCompatibilityError) Is(target error) bool {
	return target == ErrManifestIncompatible
}
