package globaldb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *SessionRepo) RegisterInstanceSession(
	ctx context.Context,
	info store.SessionInfo,
	instanceID string,
	identity *store.SessionCreationIdentity,
) (store.SessionCreationRegistration, error) {
	if err := g.checkReady(ctx, "register instance session"); err != nil {
		return store.SessionCreationRegistration{}, err
	}
	if err := info.Validate(); err != nil {
		return store.SessionCreationRegistration{}, err
	}
	if instanceID == "" {
		return store.SessionCreationRegistration{}, errors.New("store: instance id is required")
	}
	if identity != nil {
		if err := identity.Validate(); err != nil {
			return store.SessionCreationRegistration{}, err
		}
	}
	if info.CreatedAt.IsZero() {
		info.CreatedAt = g.now()
	}
	if info.UpdatedAt.IsZero() {
		info.UpdatedAt = info.CreatedAt
	}
	result := store.SessionCreationRegistration{SessionID: info.ID}
	err := g.withImmediateTransaction(ctx, "register instance session", func(exec globalSQLExecutor) error {
		existing, lookupErr := sqlcgen.New(exec).
			GetInstanceMainAttempt(ctx, sql.NullString{String: instanceID, Valid: true})
		if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
			return lookupErr
		}
		if lookupErr == nil && existing != info.ID {
			allowed, err := sqlcgen.New(exec).CanRegisterRotationCandidate(ctx, instanceID)
			if err != nil {
				return err
			}
			if !allowed {
				return agentinstance.ErrStarting
			}
		}
		if identity == nil {
			_, lookupErr := sqlcgen.New(exec).GetSessionWorkspaceID(ctx, info.ID)
			if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
				return lookupErr
			}
			result.Created = errors.Is(lookupErr, sql.ErrNoRows)
			if err := g.admitSessionRegistration(ctx, exec, info); err != nil {
				return err
			}
		} else {
			created, err := g.registerSessionCreationTx(ctx, exec, info, *identity)
			if err != nil {
				return err
			}
			result.Created, result.Identity = created, *identity
		}
		stamped, err := sqlcgen.New(exec).StampInstanceMainSession(ctx, sqlcgen.StampInstanceMainSessionParams{
			InstanceID: sql.NullString{String: instanceID, Valid: true}, SessionID: info.ID,
		})
		if err != nil {
			return err
		}
		if stamped != 1 {
			return agentinstance.ErrScopeMismatch
		}
		return nil
	})
	if err != nil {
		return store.SessionCreationRegistration{}, err
	}
	return result, nil
}
