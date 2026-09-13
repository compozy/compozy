package extensionpkg

import (
	"errors"
	"fmt"

	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
)

// ErrExtensionNameConflict means an installed name belongs to another acquisition origin.
var ErrExtensionNameConflict = errors.New("extension: instance name belongs to another origin")

// ExtensionNameConflictError identifies the installed acquisition without exposing credentials.
type ExtensionNameConflictError struct {
	Name            string
	InstalledOrigin marketplacepkg.Origin
	SourceName      string
}

var _ error = (*ExtensionNameConflictError)(nil)

func (e *ExtensionNameConflictError) Error() string {
	return fmt.Sprintf("%s: %q", ErrExtensionNameConflict, e.Name)
}

func (e *ExtensionNameConflictError) Unwrap() error { return ErrExtensionNameConflict }
