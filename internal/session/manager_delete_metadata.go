package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func writeMissingSessionDeleteReceipt(
	ctx context.Context, directory *fileutil.Directory, path string, owner store.SessionDBOwner, info *Info,
) (retErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, _, err := directory.ReadRegularFile(store.SessionMetaName); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return fmt.Errorf("session: probe deletion metadata: %w", err)
		}
		return nil
	}
	payload, err := json.Marshal(
		sessionDeleteReceipt{Owner: owner, ProfileID: info.ProfileID, WorkspaceID: info.WorkspaceID},
	)
	if err != nil {
		return fmt.Errorf("session: encode deletion receipt: %w", err)
	}
	// Move capabilities deliberately lack write rights on Windows. Reopen the
	// same held directory with mutation rights, preserving its identity proof.
	writable, err := directory.ReopenForMutation(path)
	if err != nil {
		return fmt.Errorf("session: reopen deletion receipt directory: %w", err)
	}
	defer func() { retErr = errors.Join(retErr, writable.Close()) }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := writable.AtomicWriteFile(sessionDeleteReceiptName, payload, 0o600, true); err != nil {
		return fmt.Errorf("session: write deletion receipt: %w", err)
	}
	return nil
}

func readSessionDeleteMetadata(
	ctx context.Context, directory *fileutil.Directory,
) (store.SessionMeta, store.SessionDBOwner, error) {
	if err := ctx.Err(); err != nil {
		return store.SessionMeta{}, store.SessionDBOwner{}, err
	}
	meta, err := readSessionMetaFromDirectory(directory)
	if !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return meta, store.SessionDBOwner{}, fmt.Errorf("session: read deletion metadata: %w", err)
		}
		owner, err := meta.DatabaseOwner()
		return meta, owner, err
	}
	if err := ctx.Err(); err != nil {
		return store.SessionMeta{}, store.SessionDBOwner{}, err
	}
	payload, _, err := directory.ReadRegularFile(sessionDeleteReceiptName)
	if err != nil {
		return store.SessionMeta{}, store.SessionDBOwner{}, fmt.Errorf("session: read deletion receipt: %w", err)
	}
	var receipt sessionDeleteReceipt
	if err := json.Unmarshal(payload, &receipt); err != nil {
		return store.SessionMeta{}, store.SessionDBOwner{}, fmt.Errorf("session: decode deletion receipt: %w", err)
	}
	owner, err := receipt.Owner.Normalize()
	if err != nil {
		return store.SessionMeta{}, store.SessionDBOwner{}, fmt.Errorf("session: normalize deletion owner: %w", err)
	}
	return store.SessionMeta{
		ID:              owner.SessionID,
		ProfileID:       receipt.ProfileID,
		WorkspaceID:     receipt.WorkspaceID,
		CreationProfile: &store.SessionCreationProfile{WorkspaceID: owner.WorkspaceID},
	}, owner, nil
}
