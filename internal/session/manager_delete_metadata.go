package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
)

const sessionDeleteReceiptName = "deletion-owner.json"

// A deletion receipt preserves only the verified owner and reconciliation scope.
// It never replaces missing runtime metadata or permits resuming the session.
// It may survive a failed staging or restoration in the live directory; keeping
// it there preserves recovery evidence and only tombstone recovery consumes it.
type sessionDeleteReceipt struct {
	Owner       store.SessionDBOwner `json:"owner"`
	ProfileID   string               `json:"profile_id"`
	WorkspaceID string               `json:"workspace_id"`
}

func (m *Manager) sessionDeleteOwner(ctx context.Context, target string) (store.SessionDBOwner, error) {
	meta, err := m.readSessionMetaReadOnly(ctx, target)
	if err == nil {
		return meta.DatabaseOwner()
	}
	metaPath := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target))
	if _, statErr := os.Lstat(metaPath); !errors.Is(statErr, os.ErrNotExist) {
		return store.SessionDBOwner{}, err
	}
	owner, lookupErr := store.LookupSessionOwner(ctx, m.sessionCatalog, m.creationStore, target)
	if lookupErr != nil {
		return store.SessionDBOwner{}, lookupErr
	}
	return owner.DatabaseOwner, nil
}

func writeMissingSessionDeleteReceipt(directory *fileutil.Directory, owner store.SessionDBOwner, info *Info) error {
	if _, _, err := directory.ReadRegularFile(store.SessionMetaName); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	payload, err := json.Marshal(
		sessionDeleteReceipt{Owner: owner, ProfileID: info.ProfileID, WorkspaceID: info.WorkspaceID},
	)
	if err != nil {
		return err
	}
	return directory.AtomicWriteFile(sessionDeleteReceiptName, payload, 0o600, true)
}

func readSessionDeleteMetadata(directory *fileutil.Directory) (store.SessionMeta, error) {
	meta, err := readSessionMetaFromDirectory(directory)
	if !errors.Is(err, os.ErrNotExist) {
		return meta, err
	}
	payload, _, err := directory.ReadRegularFile(sessionDeleteReceiptName)
	if err != nil {
		return store.SessionMeta{}, err
	}
	var receipt sessionDeleteReceipt
	if err := json.Unmarshal(payload, &receipt); err != nil {
		return store.SessionMeta{}, err
	}
	owner, err := receipt.Owner.Normalize()
	if err != nil {
		return store.SessionMeta{}, err
	}
	return store.SessionMeta{
		ID:              owner.SessionID,
		ProfileID:       receipt.ProfileID,
		WorkspaceID:     receipt.WorkspaceID,
		CreationProfile: &store.SessionCreationProfile{WorkspaceID: owner.WorkspaceID},
	}, nil
}
