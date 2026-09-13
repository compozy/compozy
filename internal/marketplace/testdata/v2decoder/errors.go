package marketplace

import (
	"errors"
	"fmt"
)

var (
	ErrResponseTooLarge  = errors.New("marketplace catalog: response exceeds size limit")
	ErrSourceUnavailable = errors.New("marketplace catalog: source unavailable")
	ErrEntryNotFound     = errors.New("marketplace catalog: entry not found")
	ErrKindStateMissing  = errors.New("marketplace catalog: kind state missing")
	ErrServiceClosed     = errors.New("marketplace catalog: service is closed")
)

// UnsupportedManifestVersionError reports a feed and client manifest-version mismatch.
type UnsupportedManifestVersionError struct {
	Kind    Kind
	Version int
}

func (e *UnsupportedManifestVersionError) Error() string {
	if e == nil {
		return "marketplace catalog: unsupported feed manifest version"
	}
	if e.Version < ManifestVersion {
		return fmt.Sprintf(
			"marketplace catalog %q manifest_version %d is unsupported; feed too old (client supports version %d)",
			e.Kind,
			e.Version,
			ManifestVersion,
		)
	}
	if e.Version > ManifestVersion {
		return fmt.Sprintf(
			"marketplace catalog %q manifest_version %d is unsupported; client too old (supports version %d)",
			e.Kind,
			e.Version,
			ManifestVersion,
		)
	}
	return fmt.Sprintf(
		"marketplace catalog %q manifest_version %d is unsupported",
		e.Kind,
		e.Version,
	)
}

type httpStatusError struct {
	status int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("marketplace catalog: feed returned HTTP %d", e.status)
}
