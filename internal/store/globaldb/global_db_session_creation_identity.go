package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// RegisterSessionWithCreationIdentity atomically creates or refreshes one identity-matched session.
func (g *SessionRepo) RegisterSessionWithCreationIdentity(
	ctx context.Context,
	session store.SessionInfo,
	identity store.SessionCreationIdentity,
) (store.SessionCreationRegistration, error) {
	if err := g.checkReady(ctx, "register session with creation identity"); err != nil {
		return store.SessionCreationRegistration{}, err
	}
	if err := session.Validate(); err != nil {
		return store.SessionCreationRegistration{}, err
	}
	if err := identity.Validate(); err != nil {
		return store.SessionCreationRegistration{}, err
	}
	normalized := session
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}

	registration := store.SessionCreationRegistration{
		SessionID: normalized.ID,
		Identity:  identity,
	}
	err := g.withImmediateTransaction(ctx, "register session creation identity", func(exec globalSQLExecutor) error {
		storedWorkspace, storedState, storedIdentity, found, err := readSessionCreationIdentity(
			ctx,
			exec,
			normalized.ID,
		)
		if err != nil {
			return err
		}
		if found {
			if strings.TrimSpace(storedWorkspace) != strings.TrimSpace(normalized.WorkspaceID) {
				return sessionCreationIdentityMismatch(normalized.ID)
			}
			if storedIdentityComplete(storedIdentity) {
				if storedIdentity != identity {
					return sessionCreationIdentityMismatch(normalized.ID)
				}
				if err := g.registerSession(ctx, exec, normalized); err != nil {
					return fmt.Errorf("store: refresh identity-matched session %q: %w", normalized.ID, err)
				}
				return nil
			}
			if strings.TrimSpace(storedState) != globalDBSessionStateStarting ||
				strings.TrimSpace(normalized.State) != globalDBSessionStateStarting {
				return sessionCreationIdentityMismatch(normalized.ID)
			}
		}

		if err := g.registerSession(ctx, exec, normalized); err != nil {
			return fmt.Errorf("store: register or bind identity-bound session %q: %w", normalized.ID, err)
		}
		affected, err := sqlcgen.New(exec).SetSessionCreationIdentity(
			ctx,
			sqlcgen.SetSessionCreationIdentityParams{
				CreationProfileRef: nullableSessionString(identity.CreationProfileRef),
				PolicySpecDigest:   nullableSessionString(identity.PolicySpecDigest),
				CreationDigest:     nullableSessionString(identity.CreationDigest), ID: normalized.ID,
			},
		)
		if err != nil {
			return fmt.Errorf("store: persist session creation identity %q: %w", normalized.ID, err)
		}
		if affected != 1 {
			return sessionCreationIdentityMismatch(normalized.ID)
		}
		registration.Created = !found
		return nil
	})
	if err != nil {
		return store.SessionCreationRegistration{}, err
	}
	return registration, nil
}

// GetSessionCreationIdentity loads one complete immutable identity witness.
func (g *SessionRepo) GetSessionCreationIdentity(
	ctx context.Context,
	sessionID string,
) (store.SessionCreationIdentity, error) {
	if err := g.checkReady(ctx, "get session creation identity"); err != nil {
		return store.SessionCreationIdentity{}, err
	}
	_, _, identity, found, err := readSessionCreationIdentity(ctx, g.db, strings.TrimSpace(sessionID))
	if err != nil {
		return store.SessionCreationIdentity{}, err
	}
	if !found {
		return store.SessionCreationIdentity{}, fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	if !storedIdentityComplete(identity) {
		return store.SessionCreationIdentity{}, sessionCreationIdentityMismatch(sessionID)
	}
	return identity, nil
}

func readSessionCreationIdentity(
	ctx context.Context,
	exec taskSQLExecutor,
	sessionID string,
) (string, string, store.SessionCreationIdentity, bool, error) {
	row, err := sqlcgen.New(exec).GetSessionCreationIdentity(ctx, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", store.SessionCreationIdentity{}, false, nil
	}
	if err != nil {
		return "", "", store.SessionCreationIdentity{}, false, fmt.Errorf(
			"store: read session creation identity %q: %w",
			sessionID,
			err,
		)
	}
	return row.WorkspaceID, row.State, store.SessionCreationIdentity{
		CreationProfileRef: strings.TrimSpace(row.CreationProfileRef.String),
		PolicySpecDigest:   strings.TrimSpace(row.PolicySpecDigest.String),
		CreationDigest:     strings.TrimSpace(row.CreationDigest.String),
	}, true, nil
}

func storedIdentityComplete(identity store.SessionCreationIdentity) bool {
	return identity.Validate() == nil
}

func sessionCreationIdentityMismatch(sessionID string) error {
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeSessionCreationIdentityMismatch,
		Err:  fmt.Errorf("%w: %s", store.ErrSessionCreationIdentityMismatch, sessionID),
	}
}
