package extensionpkg

import (
	"fmt"
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
		"%s: current daemon version %q does not satisfy min_agh_version %q",
		ErrManifestIncompatible,
		e.CurrentVersion,
		e.MinVersion,
	)
}

// Is matches sentinel errors for errors.Is.
func (e *ManifestCompatibilityError) Is(target error) bool {
	return target == ErrManifestIncompatible
}
