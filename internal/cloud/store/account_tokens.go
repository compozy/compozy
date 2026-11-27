// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

type AccountTokenIdentity struct {
	ID, AccountID, Name string
	Revoked             bool
}

func (q *Queries) ReadAccountToken(ctx context.Context, hash []byte) (AccountTokenIdentity, error) {
	row, err := q.generated.ReadAccountToken(ctx, hash)
	return AccountTokenIdentity{ID: row.ID, AccountID: row.AccountID, Name: row.Name, Revoked: row.RevokedAt.Valid},
		authLookupError(err)
}

func (q *Queries) InsertAccountToken(ctx context.Context, id, account, name string, hash []byte, now time.Time) error {
	return q.generated.InsertAccountToken(ctx, sqlcgen.InsertAccountTokenParams{
		ID: id, AccountID: account, Name: name, TokenSha256: hash, CreatedAt: timestamp(now),
	})
}

func (q *Queries) ListAccountTokens(ctx context.Context, account string) ([]remoteproto.AccountToken, error) {
	rows, err := q.generated.ListAccountTokens(ctx, account)
	if err != nil {
		return nil, err
	}
	tokens := make([]remoteproto.AccountToken, 0, len(rows))
	for _, row := range rows {
		token := remoteproto.AccountToken{ID: row.ID, Name: row.Name}
		if row.LastUsedAt.Valid {
			token.LastUsedAt = new(row.LastUsedAt.Time)
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func (q *Queries) RevokeAccountToken(ctx context.Context, id, account string, now time.Time) (bool, error) {
	count, err := q.generated.RevokeAccountToken(ctx, sqlcgen.RevokeAccountTokenParams{
		ID: id, AccountID: account, RevokedAt: timestamp(now),
	})
	return count == 1, err
}

func (q *Queries) TouchAccountToken(ctx context.Context, id, installation string, now time.Time) error {
	if err := q.generated.TouchAccountToken(
		ctx,
		sqlcgen.TouchAccountTokenParams{ID: id, AsOf: timestamp(now)},
	); err != nil {
		return err
	}
	return q.generated.TouchAccountTokenInstance(ctx, sqlcgen.TouchAccountTokenInstanceParams{
		TokenID: id, InstallationID: installation, LastSeenAt: timestamp(now),
	})
}
