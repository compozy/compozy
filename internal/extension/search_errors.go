package extensionpkg

import "errors"

// ErrExtensionSearchInvalid reports a malformed source or cursor selection.
var ErrExtensionSearchInvalid = errors.New("extension: invalid search request")
