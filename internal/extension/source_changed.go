package extensionpkg

import (
	"encoding/hex"
	"errors"
	"strings"
)

var ErrExtensionSourceChanged = errors.New("extension: source changed since it was listed")

// SourceChangedError identifies an approved acquisition that no longer matches its source or bytes.
type SourceChangedError struct {
	ListedDigest  string
	FetchedDigest string
	Cause         error
}

func (e *SourceChangedError) Error() string        { return ErrExtensionSourceChanged.Error() }
func (e *SourceChangedError) Unwrap() error        { return e.Cause }
func (e *SourceChangedError) Is(target error) bool { return target == ErrExtensionSourceChanged }

// ValidateExpectedDigest accepts an omitted pin, otherwise requires a full SHA-256 digest.
func ValidateExpectedDigest(digest string) error {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return nil
	}
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != 32 {
		return &ManifestValidationError{
			Field:   "expected_digest",
			Message: "expected_digest must be a SHA-256 hex digest",
		}
	}
	return nil
}

// CheckExpectedDigest compares the approved digest before any managed state mutation.
func CheckExpectedDigest(expected, actual string) error {
	if err := ValidateExpectedDigest(expected); err != nil {
		return err
	}
	expected, actual = strings.ToLower(strings.TrimSpace(expected)), strings.ToLower(strings.TrimSpace(actual))
	if expected != "" && expected != actual {
		return &SourceChangedError{ListedDigest: expected, FetchedDigest: actual}
	}
	return nil
}
