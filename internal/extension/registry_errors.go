package extensionpkg

import (
	"fmt"

	"strings"
)

// Error returns the typed missing-extension message.
func (e *ExtensionNotFoundError) Error() string {
	trimmedName := strings.TrimSpace(e.Name)
	if trimmedName == "" {
		return ErrExtensionNotFound.Error()
	}
	return fmt.Sprintf("%s: %s", ErrExtensionNotFound, trimmedName)
}

// Is matches sentinel errors for errors.Is.
func (e *ExtensionNotFoundError) Is(target error) bool {
	return target == ErrExtensionNotFound
}

// Error returns the typed duplicate-extension message.
func (e *ExtensionExistsError) Error() string {
	trimmedName := strings.TrimSpace(e.Name)
	if trimmedName == "" {
		return ErrExtensionExists.Error()
	}
	return fmt.Sprintf("%s: %s", ErrExtensionExists, trimmedName)
}

// Is matches sentinel errors for errors.Is.
func (e *ExtensionExistsError) Is(target error) bool {
	return target == ErrExtensionExists
}

// Error returns the typed checksum mismatch message.
func (e *ExtensionChecksumMismatchError) Error() string {
	if e == nil {
		return ErrExtensionChecksumMismatch.Error()
	}
	return fmt.Sprintf(
		"%s: expected %s, got %s",
		ErrExtensionChecksumMismatch,
		e.ExpectedChecksum,
		e.ActualChecksum,
	)
}

// Is matches sentinel errors for errors.Is.
func (e *ExtensionChecksumMismatchError) Is(target error) bool {
	return target == ErrExtensionChecksumMismatch
}
