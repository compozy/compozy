package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

type sessionDeleteTombstoneState uint8

const (
	sessionDeleteTombstoneStaged sessionDeleteTombstoneState = iota + 1
	sessionDeleteTombstoneCommitted
)

var errInvalidSessionDeleteTombstone = errors.New("session: invalid deletion tombstone")

func sessionDeleteTombstoneName(prefix string, target string, deletionID string) string {
	encodedTarget := base64.RawURLEncoding.EncodeToString([]byte(target))
	return prefix + encodedTarget + "." + deletionID
}

func parseSessionDeleteTombstoneParts(name string) (sessionDeleteTombstoneState, string, string, error) {
	cleanName := strings.TrimSpace(name)
	state := sessionDeleteTombstoneState(0)
	value := ""
	switch {
	case strings.HasPrefix(cleanName, sessionDeleteStagedPrefix):
		state = sessionDeleteTombstoneStaged
		value = strings.TrimPrefix(cleanName, sessionDeleteStagedPrefix)
	case strings.HasPrefix(cleanName, sessionDeleteCommittedPrefix):
		state = sessionDeleteTombstoneCommitted
		value = strings.TrimPrefix(cleanName, sessionDeleteCommittedPrefix)
	default:
		return 0, "", "", fmt.Errorf("%w name %q", errInvalidSessionDeleteTombstone, name)
	}
	separator := strings.LastIndexByte(value, '.')
	if separator <= 0 || separator == len(value)-1 {
		return 0, "", "", fmt.Errorf("%w name %q", errInvalidSessionDeleteTombstone, name)
	}
	decodedTarget, err := base64.RawURLEncoding.DecodeString(value[:separator])
	if err != nil {
		return 0, "", "", fmt.Errorf(
			"%w target %q: %w",
			errInvalidSessionDeleteTombstone,
			name,
			err,
		)
	}
	target, err := normalizeStoredSessionID(string(decodedTarget))
	if err != nil {
		return 0, "", "", fmt.Errorf(
			"%w target %q: %w",
			errInvalidSessionDeleteTombstone,
			name,
			err,
		)
	}
	return state, target, value[separator+1:], nil
}

func (m *Manager) cleanupDeleteTombstones() {
	entries, err := os.ReadDir(m.homePaths.SessionsDir)
	if err != nil {
		m.logger.Warn("session: scan deletion tombstones", "error", err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), sessionDeleteTombstonePrefix) {
			continue
		}
		path := filepath.Join(m.homePaths.SessionsDir, entry.Name())
		if err := m.cleanupSessionDeleteTombstone(path, entry.Name()); err != nil {
			m.logger.Warn("session: retain deletion tombstone for later retry", "path", path, "error", err)
		}
	}
}

func (m *Manager) acquireDeleteTombstoneLease(
	ctx context.Context,
	canonicalDBPath string,
) (*sessiondb.FamilyLease, error) {
	acquireLease := m.acquireSessionDBFamilyLease
	if acquireLease == nil {
		acquireLease = sessiondb.AcquireFamilyLease
	}
	return acquireLease(ctx, canonicalDBPath)
}

func (m *Manager) cleanupSessionDeleteTombstone(path string, name string) (retErr error) {
	state, target, deletionID, err := parseSessionDeleteTombstoneParts(name)
	if err != nil {
		return err
	}
	ctx, cancel := m.lifecycleCleanupContext()
	defer cancel()

	canonicalDBPath := store.SessionDBFile(filepath.Join(m.homePaths.SessionsDir, target))
	lease, err := m.acquireDeleteTombstoneLease(ctx, canonicalDBPath)
	if err != nil {
		return err
	}
	defer lease.Release()

	parent, err := fileutil.OpenDirectoryForMutation(m.homePaths.SessionsDir)
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, parent.Close())
	}()
	directory, err := parent.OpenDirectoryForMove(name)
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, directory.Close())
	}()

	meta, err := readSessionMetaFromDirectory(directory)
	if err != nil {
		return fmt.Errorf("session: read deletion tombstone metadata: %w", err)
	}
	if strings.TrimSpace(meta.ID) != target {
		return fmt.Errorf("session: deletion tombstone metadata id %q does not match %q", meta.ID, target)
	}
	owner, err := (store.SessionDBOwner{SessionID: target, WorkspaceID: meta.WorkspaceID}).Normalize()
	if err != nil {
		return err
	}
	database, err := lease.BindOwnedFile(ctx, owner, store.SessionDBFile(path))
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, database.Close())
	}()
	if err := requireSessionDBInDirectory(database, directory); err != nil {
		return err
	}

	if state == sessionDeleteTombstoneStaged {
		restore, err := m.shouldRestoreStagedSessionDelete(ctx, target, owner)
		if err != nil {
			return err
		}
		if err := m.recoverSessionAttachmentDelete(meta.WorkspaceID, target, deletionID, restore); err != nil {
			return fmt.Errorf("session: recover staged attachment deletion for %q: %w", target, err)
		}
		if restore {
			return m.restoreStagedSessionDelete(
				ctx,
				owner,
				canonicalDBPath,
				parent,
				directory,
				database,
			)
		}
	} else if err := m.recoverSessionAttachmentDelete(meta.WorkspaceID, target, deletionID, false); err != nil {
		return fmt.Errorf("session: finish committed attachment deletion for %q: %w", target, err)
	}

	if err := database.VerifyOwnerAt(ctx, owner, store.SessionDBFile(path)); err != nil {
		return err
	}
	return m.removeBoundSessionDelete(directory, path)
}

func (m *Manager) restoreStagedSessionDelete(
	ctx context.Context,
	owner store.SessionDBOwner,
	canonicalDBPath string,
	parent *fileutil.Directory,
	directory *fileutil.Directory,
	database *sessiondb.FamilyFile,
) error {
	target := owner.SessionID
	result, moveErr := directory.MoveTo(parent, target, false)
	if !result.Committed() {
		return errors.Join(moveErr, result.PostCommitErr)
	}
	return errors.Join(
		moveErr,
		result.PostCommitErr,
		database.VerifyOwnerAt(ctx, owner, canonicalDBPath),
	)
}

func (m *Manager) shouldRestoreStagedSessionDelete(
	ctx context.Context,
	target string,
	owner store.SessionDBOwner,
) (bool, error) {
	catalogOwner, exists, known, err := m.sessionCatalogOwnerState(ctx, target)
	if err != nil {
		return false, err
	}
	if known && !exists {
		return false, nil
	}
	if exists && catalogOwner != owner {
		return false, fmt.Errorf(
			"%w: staged deletion owner %+v does not match catalog owner %+v",
			store.ErrSessionWorkspaceMismatch,
			owner,
			catalogOwner,
		)
	}
	return true, nil
}

func (m *Manager) sessionCatalogOwnerState(
	ctx context.Context,
	target string,
) (store.SessionDBOwner, bool, bool, error) {
	if m == nil || m.sessionCatalog == nil {
		return store.SessionDBOwner{}, false, false, nil
	}
	owner, err := store.LookupSessionDBOwner(ctx, m.sessionCatalog, target)
	if errors.Is(err, store.ErrSessionNotFound) {
		return store.SessionDBOwner{}, false, true, nil
	}
	if err != nil {
		return store.SessionDBOwner{}, false, true, fmt.Errorf(
			"session: resolve catalog owner for deletion recovery: %w",
			err,
		)
	}
	return owner, true, true, nil
}

func readSessionMetaFromDirectory(directory *fileutil.Directory) (store.SessionMeta, error) {
	payload, _, err := directory.ReadRegularFile(store.SessionMetaName)
	if err != nil {
		return store.SessionMeta{}, err
	}
	var meta store.SessionMeta
	if err := json.Unmarshal(payload, &meta); err != nil {
		return store.SessionMeta{}, fmt.Errorf("session: decode deletion metadata: %w", err)
	}
	if err := meta.Validate(); err != nil {
		return store.SessionMeta{}, err
	}
	return meta, nil
}

func requireSessionDBInDirectory(database *sessiondb.FamilyFile, directory *fileutil.Directory) (retErr error) {
	file, err := directory.OpenRegularFile(store.SessionDatabaseName)
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, file.Close())
	}()
	return database.RequireSameFile(file)
}

func (m *Manager) removeStagedSessionDelete(entry stagedSessionDelete, path string) error {
	if entry.capabilities == nil || entry.capabilities.directory == nil {
		return errors.New("session: staged deletion directory capability is required")
	}
	return m.removeBoundSessionDelete(entry.capabilities.directory, path)
}

func (m *Manager) removeBoundSessionDelete(directory *fileutil.Directory, path string) error {
	if m.removeAllPath != nil {
		return m.removeAllPath(path)
	}
	if err := directory.RemoveBoundTree(); err != nil {
		return fmt.Errorf("session: remove deletion tombstone %q: %w", path, err)
	}
	return nil
}
