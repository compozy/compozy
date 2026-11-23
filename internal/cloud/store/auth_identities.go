package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
)

var (
	ErrIdentityConflict = errors.New("cloud account email is already in use")
	ErrIdentityRace     = errors.New("cloud identity resolution must retry")
)

type AuthAccount struct {
	ID, Email, Name, State string
}

func mapAuthAccount(a sqlcgen.Account) AuthAccount {
	return AuthAccount{ID: a.ID, Email: a.Email, Name: a.Name, State: a.State}
}

func (q *Queries) LockAuthAccount(ctx context.Context, id string) (AuthAccount, error) {
	a, err := q.generated.LockAuthAccount(ctx, id)
	return mapAuthAccount(a), authLookupError(err)
}

func (q *Queries) AuthAccountDeleted(ctx context.Context, id string) (bool, error) {
	return q.generated.HasDeletedAuthAccount(ctx, id)
}

type VerifiedIdentity struct {
	Provider, Subject, Email, Name string
}

// LockIdentityAccount resolves the provider subject before considering an email match.
func (q *Queries) LockIdentityAccount(
	ctx context.Context, identity VerifiedIdentity, newID string, now time.Time,
) (AuthAccount, bool, error) {
	id, err := q.generated.FindIdentityAccount(ctx, sqlcgen.FindIdentityAccountParams{
		Provider: identity.Provider, Subject: identity.Subject,
	})
	switch err = authLookupError(err); {
	case err == nil:
		account, err := q.LockAuthAccount(ctx, id)
		return account, false, err
	case !errors.Is(err, ErrAuthNotFound):
		return AuthAccount{}, false, err
	}

	account, err := q.generated.LockAuthAccountByEmail(ctx, identity.Email)
	if !errors.Is(authLookupError(err), ErrAuthNotFound) {
		return mapAuthAccount(account), false, err
	}
	account, err = q.generated.InsertAuthAccount(ctx, sqlcgen.InsertAuthAccountParams{
		ID: newID, Email: identity.Email, Name: identity.Name,
		FirstSignInProvider: identity.Provider, CreatedAt: timestamp(now),
	})
	if errors.Is(authLookupError(err), ErrAuthNotFound) {
		return AuthAccount{}, false, ErrIdentityRace
	}
	return mapAuthAccount(account), err == nil, err
}

func (q *Queries) SaveIdentity(
	ctx context.Context, account AuthAccount, identity VerifiedIdentity, now time.Time,
) error {
	_, err := q.generated.RecordSignInIdentity(ctx, sqlcgen.RecordSignInIdentityParams{
		Provider: identity.Provider, Subject: identity.Subject, AccountID: account.ID,
		Email: identity.Email, CreatedAt: timestamp(now),
	})
	if errors.Is(authLookupError(err), ErrAuthNotFound) {
		return ErrIdentityRace
	}
	if err != nil || account.Email == identity.Email {
		return err
	}
	err = q.generated.UpdateAuthAccountEmail(
		ctx,
		sqlcgen.UpdateAuthAccountEmailParams{ID: account.ID, Email: identity.Email},
	)
	if constraint, ok := errors.AsType[*pgconn.PgError](err); ok && constraint.Code == "23505" {
		return ErrIdentityConflict
	}
	return err
}

func (q *Queries) AdvanceAccountActivity(ctx context.Context, id string, now time.Time) error {
	return q.generated.AdvanceAuthAccountActivity(ctx, sqlcgen.AdvanceAuthAccountActivityParams{
		ID: id, AsOf: timestamp(now),
	})
}
