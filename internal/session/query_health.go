package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	loggerpkg "github.com/compozy/compozy/internal/logger"
	"github.com/compozy/compozy/internal/store"
)

// SessionMetadataHealth reads persisted witnesses without repairing metadata or changing warning cadence.
func (m *Manager) SessionMetadataHealth(ctx context.Context) (int, loggerpkg.FailureSummary, error) {
	var failures loggerpkg.FailureSummary
	if ctx == nil {
		return 0, failures, errors.New("session: metadata health context is required")
	}
	entries, err := os.ReadDir(m.homePaths.SessionsDir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, failures, nil
	}
	if err != nil {
		return 0, failures, fmt.Errorf("session: read metadata health directory: %w", err)
	}
	checked := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return checked, failures, err
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), sessionDeleteTombstonePrefix) {
			continue
		}
		meta, err := store.ReadSessionMeta(store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, entry.Name())))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		checked++
		if err == nil {
			err = m.verifyMetadataHealthWitness(ctx, entry.Name(), meta)
		}
		if err != nil {
			failures.Add(entry.Name(), err)
		}
	}
	return checked, failures, nil
}

func (m *Manager) verifyMetadataHealthWitness(ctx context.Context, id string, meta store.SessionMeta) error {
	if meta.ID != id {
		return errors.New("session: metadata identity does not match directory")
	}
	if meta.CreationProfileRef == "" || m.creationStore == nil {
		return nil
	}
	if _, err := m.creationStore.GetSessionCreationProfile(ctx, meta.CreationProfileRef); err != nil {
		return fmt.Errorf("session: read catalog creation profile: %w", err)
	}
	identity, err := m.creationStore.GetSessionCreationIdentity(ctx, id)
	if err != nil {
		return fmt.Errorf("session: read catalog creation identity: %w", err)
	}
	if identity.CreationProfileRef != meta.CreationProfileRef || identity.PolicySpecDigest != meta.PolicySpecDigest ||
		identity.CreationDigest != meta.CreationDigest {
		return store.ErrSessionCreationIdentityMismatch
	}
	return nil
}
