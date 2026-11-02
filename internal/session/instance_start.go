package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

type instanceSessionRegistrar interface {
	RegisterInstanceSession(
		context.Context,
		*store.SessionInfo,
		string,
		*store.SessionCreationIdentity,
	) (store.SessionCreationRegistration, error)
}

func (m *Manager) persistInstanceAcceptedStart(ctx context.Context, instanceID string, session *Session) error {
	registrar, ok := m.sessionCatalog.(instanceSessionRegistrar)
	if !ok {
		return errors.New("session: instance session registration unavailable")
	}
	if session.pendingDeriveReceipt != nil || session.Type != SessionTypeUser {
		return errors.New("session: instance main must be a new user session")
	}
	session.persistMu.Lock()
	defer session.persistMu.Unlock()
	if err := m.writeMeta(session); err != nil {
		return err
	}
	meta := session.Meta()
	identity := creationIdentityFromMeta(&meta)
	if identity != nil {
		if m.creationStore == nil || meta.CreationProfile == nil {
			return errors.New("session: instance creation profile unavailable")
		}
		ref, err := m.creationStore.PutSessionCreationProfile(ctx, *meta.CreationProfile)
		if err != nil {
			return err
		}
		if ref != identity.CreationProfileRef {
			return errors.New("session: instance creation profile ref changed")
		}
	}
	if _, err := registrar.RegisterInstanceSession(
		ctx,
		new(sessionCatalogInfoFromRuntime(session.Info())),
		instanceID,
		identity,
	); err != nil {
		return fmt.Errorf("session: register instance main %q: %w", session.ID, err)
	}
	session.mu.Lock()
	session.InstanceID = instanceID
	session.mu.Unlock()
	return m.finishSessionCatalogPersistence(ctx, session)
}
