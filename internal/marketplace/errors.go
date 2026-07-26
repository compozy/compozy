package marketplace

import (
	"errors"
	"fmt"
)

var (
	ErrResponseTooLarge = errors.New("marketplace catalog: response exceeds size limit")
	ErrEntryNotFound    = errors.New("marketplace catalog: entry not found")
	ErrKindStateMissing = errors.New("marketplace catalog: kind state missing")
	ErrServiceClosed    = errors.New("marketplace catalog: service is closed")
)

// UnsupportedManifestVersionError requires the operator to upgrade the client.
type UnsupportedManifestVersionError struct {
	Kind    Kind
	Version int
}

func (e *UnsupportedManifestVersionError) Error() string {
	if e == nil {
		return "marketplace catalog: client too old for feed manifest"
	}
	return fmt.Sprintf(
		"marketplace catalog %q manifest_version %d is unsupported; client too old (supports version %d)",
		e.Kind,
		e.Version,
		ManifestVersion,
	)
}

type httpStatusError struct {
	status int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("marketplace catalog: feed returned HTTP %d", e.status)
}
