package observe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

// verifyRecoveredSessionOwner proves every available durable owner before
// reconciliation is allowed to repair metadata. The boolean reports whether
// either the catalog or an existing SessionDB supplied authoritative ownership.
func (o *Observer) verifyRecoveredSessionOwner(
	ctx context.Context,
	entryName string,
	meta store.SessionMeta,
) (bool, error) {
	entryID := strings.TrimSpace(entryName)
	owner, err := (store.SessionDBOwner{
		SessionID:   meta.ID,
		WorkspaceID: meta.WorkspaceID,
	}).Normalize()
	if err != nil {
		return false, err
	}
	if entryID == "" || owner.SessionID != entryID {
		return false, fmt.Errorf(
			"observe: metadata identity %q does not match session directory %q",
			owner.SessionID,
			entryID,
		)
	}

	dbPath := store.SessionDBFile(filepath.Join(o.homePaths.SessionsDir, entryID))
	lease, err := sessiondb.AcquireFamilyLease(ctx, dbPath)
	if err != nil {
		return false, err
	}
	defer lease.Release()

	catalogAuthoritative := false
	catalogOwner, err := store.LookupSessionDBOwner(ctx, o.registry, owner.SessionID)
	switch {
	case err == nil:
		catalogAuthoritative = true
		if catalogOwner != owner {
			return false, fmt.Errorf(
				"%w: metadata owner %+v does not match catalog owner %+v",
				store.ErrSessionWorkspaceMismatch,
				owner,
				catalogOwner,
			)
		}
	case errors.Is(err, store.ErrSessionNotFound):
	default:
		return false, err
	}

	if _, err := os.Lstat(dbPath); errors.Is(err, os.ErrNotExist) {
		return catalogAuthoritative, nil
	} else if err != nil {
		return false, fmt.Errorf("observe: inspect recovered session database %q: %w", dbPath, err)
	}
	if err := lease.VerifyOwner(ctx, owner, dbPath); err != nil {
		return false, fmt.Errorf("observe: verify recovered session database owner %q: %w", owner.SessionID, err)
	}
	return true, nil
}
