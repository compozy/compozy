package sessiondb

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/store"
)

// NewEventMetadataOpener binds read-only session metadata access to one sessions directory.
func NewEventMetadataOpener(sessionsDir string) store.SessionEventMetadataOpener {
	root := strings.TrimSpace(sessionsDir)
	return func(ctx context.Context, sessionID string) (store.EventMetadataReadCloser, error) {
		path := store.SessionDBFile(filepath.Join(root, strings.TrimSpace(sessionID)))
		return OpenSessionDBReadOnly(ctx, sessionID, path)
	}
}
