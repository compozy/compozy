package sessiondb

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// NewEventMetadataOpener binds read-only session metadata access to one sessions directory.
func NewEventMetadataOpener(sessionsDir string) store.SessionEventMetadataOpener {
	root := strings.TrimSpace(sessionsDir)
	return func(ctx context.Context, owner store.SessionDBOwner) (store.EventMetadataReadCloser, error) {
		path := store.SessionDBFile(filepath.Join(root, strings.TrimSpace(owner.SessionID)))
		return OpenSessionDBReadOnly(ctx, owner, path)
	}
}
