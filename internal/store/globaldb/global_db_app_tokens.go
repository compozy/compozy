package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/compozy/compozy/internal/apptoken"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ apptoken.Store = (*AppTokenRepo)(nil)

func (g *AppTokenRepo) CreateAppToken(ctx context.Context, record apptoken.StoredToken) ([]string, error) {
	if err := g.checkReady(ctx, "create app token"); err != nil {
		return nil, err
	}
	var origins []string
	err := g.withImmediateTransaction(ctx, "create app token", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		count, err := queries.CountActiveAppTokens(ctx)
		if err != nil {
			return fmt.Errorf("store: count active app tokens: %w", err)
		}
		if count >= apptoken.Limit {
			return apptoken.ErrLimit
		}
		if err := queries.CreateAppToken(ctx, sqlcgen.CreateAppTokenParams{
			ID: record.Token.ID, Name: record.Token.Name, TokenHash: record.TokenHash,
			CreatedAt: store.FormatTimestamp(record.Token.CreatedAt),
		}); err != nil {
			return fmt.Errorf("store: insert app token: %w", err)
		}
		if err := replaceAppTokenOrigins(ctx, queries, record.Token.ID,
			record.Token.Origins, record.Token.CreatedAt); err != nil {
			return err
		}
		origins, err = queries.ListActiveAppTokenOrigins(ctx)
		return err
	})
	return origins, err
}

func (g *AppTokenRepo) ListAppTokens(
	ctx context.Context,
	state apptoken.State,
) (_ []apptoken.Token, err error) {
	if err := g.checkReady(ctx, "list app tokens"); err != nil {
		return nil, err
	}
	tx, err := g.readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("store: begin app token list: %w", err)
	}
	defer func() { err = errors.Join(err, rollbackTx(tx, "list app tokens")) }()
	queries := sqlcgen.New(tx)
	rows, err := queries.ListAppTokens(ctx, string(state))
	if err != nil {
		return nil, fmt.Errorf("store: list app tokens: %w", err)
	}
	tokens := make([]apptoken.Token, 0, len(rows))
	for _, row := range rows {
		token, mapErr := appTokenFromGenerated(ctx, queries, row)
		if mapErr != nil {
			return nil, mapErr
		}
		tokens = append(tokens, token)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: commit app token list: %w", err)
	}
	return tokens, nil
}

func (g *AppTokenRepo) UpdateAppToken(
	ctx context.Context,
	id string,
	patch apptoken.Patch,
	now time.Time,
) (apptoken.Token, []string, error) {
	if err := g.checkReady(ctx, "update app token"); err != nil {
		return apptoken.Token{}, nil, err
	}
	var token apptoken.Token
	var origins []string
	err := g.withImmediateTransaction(ctx, "update app token", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		current, err := getAppToken(ctx, queries, id)
		if err != nil {
			return err
		}
		if !current.RevokedAt.IsZero() {
			return apptoken.ErrRevoked
		}
		if patch.Name != nil {
			if err := queries.RenameAppToken(ctx, sqlcgen.RenameAppTokenParams{Name: *patch.Name, ID: id}); err != nil {
				return fmt.Errorf("store: rename app token: %w", err)
			}
			current.Name = *patch.Name
		}
		if patch.Origins != nil {
			if err := replaceAppTokenOrigins(ctx, queries, id, *patch.Origins, now); err != nil {
				return err
			}
			current.Origins = *patch.Origins
		}
		token = current
		origins, err = queries.ListActiveAppTokenOrigins(ctx)
		return err
	})
	return token, origins, err
}

func (g *AppTokenRepo) RevokeAppToken(
	ctx context.Context,
	id string,
	now time.Time,
) (apptoken.Token, bool, []string, error) {
	if err := g.checkReady(ctx, "revoke app token"); err != nil {
		return apptoken.Token{}, false, nil, err
	}
	var token apptoken.Token
	var origins []string
	var changed bool
	err := g.withImmediateTransaction(ctx, "revoke app token", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		current, err := getAppToken(ctx, queries, id)
		if err != nil {
			return err
		}
		changed = current.RevokedAt.IsZero()
		if changed {
			if err := queries.RevokeAppToken(ctx, sqlcgen.RevokeAppTokenParams{
				ID: id, RevokedAt: nullableTime(new(now)),
			}); err != nil {
				return fmt.Errorf("store: revoke app token: %w", err)
			}
			current.RevokedAt = now
			current.RevokeEpoch++
		}
		token = current
		origins, err = queries.ListActiveAppTokenOrigins(ctx)
		return err
	})
	return token, changed, origins, err
}

func (g *AppTokenRepo) FindAppTokenByTokenHash(
	ctx context.Context,
	hash string,
) (_ apptoken.StoredToken, err error) {
	if err := g.checkReady(ctx, "find app token"); err != nil {
		return apptoken.StoredToken{}, err
	}
	tx, err := g.readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return apptoken.StoredToken{}, fmt.Errorf("store: begin app token lookup: %w", err)
	}
	defer func() { err = errors.Join(err, rollbackTx(tx, "find app token")) }()
	queries := sqlcgen.New(tx)
	row, err := queries.FindAppTokenByTokenHash(ctx, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return apptoken.StoredToken{}, apptoken.ErrNotFound
	}
	if err != nil {
		return apptoken.StoredToken{}, fmt.Errorf("store: find app token credential: %w", err)
	}
	token, err := appTokenFromGenerated(ctx, queries, row)
	if err != nil {
		return apptoken.StoredToken{}, err
	}
	if err := tx.Commit(); err != nil {
		return apptoken.StoredToken{}, fmt.Errorf("store: commit app token lookup: %w", err)
	}
	return apptoken.StoredToken{Token: token, TokenHash: row.TokenHash}, nil
}

func (g *AppTokenRepo) TouchAppTokenLastSeen(
	ctx context.Context,
	id string,
	epoch uint64,
	now time.Time,
) error {
	if err := g.checkReady(ctx, "touch app token last use"); err != nil {
		return err
	}
	value, err := appTokenEpoch(epoch)
	if err != nil {
		return err
	}
	count, err := executeWriteQueryResult(
		ctx,
		g.repoBase,
		"globaldb.TouchAppTokenLastSeen",
		func(ctx context.Context, queries *sqlcgen.Queries) (int64, error) {
			return queries.TouchAppTokenLastSeen(ctx, sqlcgen.TouchAppTokenLastSeenParams{
				ID: id, RevokeEpoch: value, LastSeenAt: nullableTime(new(now)),
			})
		},
	)
	if err != nil {
		return fmt.Errorf("store: touch app token last use: %w", err)
	}
	if count == 0 {
		return apptoken.ErrRevoked
	}
	return nil
}

func (g *AppTokenRepo) ListActiveAppTokenOrigins(ctx context.Context) ([]string, error) {
	if err := g.checkReady(ctx, "list active app token origins"); err != nil {
		return nil, err
	}
	origins, err := g.readQueries.ListActiveAppTokenOrigins(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list active app token origins: %w", err)
	}
	return origins, nil
}

func getAppToken(ctx context.Context, queries *sqlcgen.Queries, id string) (apptoken.Token, error) {
	row, err := queries.GetAppToken(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return apptoken.Token{}, apptoken.ErrNotFound
	}
	if err != nil {
		return apptoken.Token{}, fmt.Errorf("store: get app token: %w", err)
	}
	return appTokenFromGenerated(ctx, queries, row)
}

func appTokenFromGenerated(
	ctx context.Context,
	queries *sqlcgen.Queries,
	row sqlcgen.AppToken,
) (apptoken.Token, error) {
	if row.RevokeEpoch < 0 {
		return apptoken.Token{}, errors.New("store: negative app token revocation epoch")
	}
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return apptoken.Token{}, fmt.Errorf("store: parse app token creation timestamp: %w", err)
	}
	lastSeenAt, err := appTokenTimestamp(row.LastSeenAt)
	if err != nil {
		return apptoken.Token{}, fmt.Errorf("store: parse app token last-seen timestamp: %w", err)
	}
	revokedAt, err := appTokenTimestamp(row.RevokedAt)
	if err != nil {
		return apptoken.Token{}, fmt.Errorf("store: parse app token revocation timestamp: %w", err)
	}
	origins, err := queries.ListAppTokenOrigins(ctx, row.ID)
	if err != nil {
		return apptoken.Token{}, fmt.Errorf("store: list app token origins: %w", err)
	}
	if origins == nil {
		origins = []string{}
	}
	return apptoken.Token{
		ID: row.ID, Name: row.Name, ActorKind: apptoken.ActorKind,
		RevokeEpoch: uint64(row.RevokeEpoch), CreatedAt: createdAt,
		LastSeenAt: lastSeenAt, RevokedAt: revokedAt, Origins: origins,
	}, nil
}

func replaceAppTokenOrigins(
	ctx context.Context,
	queries *sqlcgen.Queries,
	id string,
	origins []string,
	now time.Time,
) error {
	if err := queries.DeleteAppTokenOrigins(ctx, id); err != nil {
		return fmt.Errorf("store: clear app token origins: %w", err)
	}
	for _, origin := range origins {
		if err := queries.InsertAppTokenOrigin(ctx, sqlcgen.InsertAppTokenOriginParams{
			AppTokenID: id, Origin: origin, CreatedAt: store.FormatTimestamp(now),
		}); err != nil {
			return fmt.Errorf("store: insert app token origin: %w", err)
		}
	}
	return nil
}

func appTokenEpoch(epoch uint64) (int64, error) {
	if epoch > math.MaxInt64 {
		return 0, errors.New("store: app token revocation epoch exceeds SQLite integer range")
	}
	return int64(epoch), nil
}

func appTokenTimestamp(value sql.NullString) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, nil
	}
	return store.ParseTimestamp(value.String)
}

func (g *AppTokenRepo) RevalidateAppTokenEpoch(ctx context.Context, id string, epoch uint64) error {
	if err := g.checkReady(ctx, "revalidate app token epoch"); err != nil {
		return err
	}
	value, err := appTokenEpoch(epoch)
	if err != nil {
		return err
	}
	matches, err := g.readQueries.AppTokenEpochMatches(ctx, sqlcgen.AppTokenEpochMatchesParams{
		ID: id, RevokeEpoch: value,
	})
	if err != nil {
		return fmt.Errorf("store: revalidate app token epoch: %w", err)
	}
	if !matches {
		return apptoken.ErrRevoked
	}
	return nil
}
