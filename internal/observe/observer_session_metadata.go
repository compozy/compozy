package observe

import (
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (o *Observer) readObservedSessionMeta(sessionID string, workspaceID string) (store.SessionMeta, bool) {
	id := strings.TrimSpace(sessionID)
	if o == nil || !validObservedSessionID(id) {
		return store.SessionMeta{}, false
	}
	meta, err := store.ReadSessionMeta(store.SessionMetaFile(filepath.Join(o.homePaths.SessionsDir, id)))
	if err != nil || strings.TrimSpace(meta.ID) != id ||
		strings.TrimSpace(meta.WorkspaceID) != strings.TrimSpace(workspaceID) {
		return store.SessionMeta{}, false
	}
	return meta, true
}

func validObservedSessionID(id string) bool {
	return id != "" && id != "." && id != ".." && !filepath.IsAbs(id) && filepath.Clean(id) == id &&
		!strings.ContainsAny(id, `/\`)
}
