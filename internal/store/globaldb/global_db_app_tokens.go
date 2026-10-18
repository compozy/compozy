package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ gateway.AppTokenStore = (*GatewayRepo)(nil)

func (g *GatewayRepo) CreateAppToken(ctx context.Context, record gateway.StoredAppToken) ([]string, error) {
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
		if count >= gateway.AppTokenLimit {
			return gateway.ErrAppTokenLimit
		}
		if err := queries.CreateAppToken(ctx, sqlcgen.CreateAppTokenParams{
			ID: record.Token.Session.ID, Name: record.Token.Session.Name, TokenHash: record.TokenHash,
			CreatedAt: store.FormatTimestamp(record.Token.Session.CreatedAt),
		}); err != nil {
			return fmt.Errorf("store: insert app token: %w", err)
		}
		if err := replaceAppTokenOrigins(ctx, queries, record.Token.Session.ID,
			record.Token.Origins, record.Token.Session.CreatedAt); err != nil {
			return err
		}
		origins, err = queries.ListActiveAppTokenOrigins(ctx)
		return err
	})
	return origins, err
}

func (g *GatewayRepo) ListAppTokens(
	ctx context.Context,
	state gateway.AppTokenState,
) (_ []gateway.AppToken, err error) {
	if err := g.checkReady(ctx, "list app tokens"); err != nil {
		return nil, err
	}
	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("store: begin app token list: %w", err)
	}
	defer func() { err = errors.Join(err, rollbackTx(tx, "list app tokens")) }()
	queries := sqlcgen.New(tx)
	rows, err := queries.ListAppTokens(ctx, string(state))
	if err != nil {
		return nil, fmt.Errorf("store: list app tokens: %w", err)
	}
	tokens := make([]gateway.AppToken, 0, len(rows))
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

func (g *GatewayRepo) UpdateAppToken(
	ctx context.Context,
	id string,
	patch gateway.AppTokenPatch,
	now time.Time,
) (gateway.AppToken, []string, error) {
	if err := g.checkReady(ctx, "update app token"); err != nil {
		return gateway.AppToken{}, nil, err
	}
	var token gateway.AppToken
	var origins []string
	err := g.withImmediateTransaction(ctx, "update app token", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		current, err := getAppToken(ctx, queries, id)
		if err != nil {
			return err
		}
		if !current.Session.RevokedAt.IsZero() {
			return gateway.ErrAppTokenRevoked
		}
		if patch.Name != nil {
			if err := queries.RenameAppToken(ctx, sqlcgen.RenameAppTokenParams{Name: *patch.Name, ID: id}); err != nil {
				return fmt.Errorf("store: rename app token: %w", err)
			}
			current.Session.Name = *patch.Name
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

func (g *GatewayRepo) RevokeAppToken(
	ctx context.Context,
	id string,
	now time.Time,
) (gateway.AppToken, bool, []string, error) {
	if err := g.checkReady(ctx, "revoke app token"); err != nil {
		return gateway.AppToken{}, false, nil, err
	}
	var token gateway.AppToken
	var origins []string
	var changed bool
	err := g.withImmediateTransaction(ctx, "revoke app token", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		current, err := getAppToken(ctx, queries, id)
		if err != nil {
			return err
		}
		changed = current.Session.RevokedAt.IsZero()
		if changed {
			if err := queries.RevokeAppToken(ctx, sqlcgen.RevokeAppTokenParams{
				ID: id, RevokedAt: nullableGatewayTime(now),
			}); err != nil {
				return fmt.Errorf("store: revoke app token: %w", err)
			}
			current.Session.RevokedAt = now
			current.Session.RevokeEpoch++
		}
		token = current
		origins, err = queries.ListActiveAppTokenOrigins(ctx)
		return err
	})
	return token, changed, origins, err
}

func (g *GatewayRepo) FindAppTokenByTokenHash(
	ctx context.Context,
	hash string,
) (_ gateway.StoredAppToken, err error) {
	if err := g.checkReady(ctx, "find app token"); err != nil {
		return gateway.StoredAppToken{}, err
	}
	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return gateway.StoredAppToken{}, fmt.Errorf("store: begin app token lookup: %w", err)
	}
	defer func() { err = errors.Join(err, rollbackTx(tx, "find app token")) }()
	queries := sqlcgen.New(tx)
	row, err := queries.FindAppTokenByTokenHash(ctx, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return gateway.StoredAppToken{}, gateway.ErrAppTokenNotFound
	}
	if err != nil {
		return gateway.StoredAppToken{}, fmt.Errorf("store: find app token credential: %w", err)
	}
	token, err := appTokenFromGenerated(ctx, queries, row)
	if err != nil {
		return gateway.StoredAppToken{}, err
	}
	if err := tx.Commit(); err != nil {
		return gateway.StoredAppToken{}, fmt.Errorf("store: commit app token lookup: %w", err)
	}
	return gateway.StoredAppToken{Token: token, TokenHash: row.TokenHash}, nil
}

func (g *GatewayRepo) TouchAppTokenLastSeen(
	ctx context.Context,
	id string,
	epoch uint64,
	now time.Time,
) error {
	if err := g.checkReady(ctx, "touch app token last use"); err != nil {
		return err
	}
	value, err := gatewayDeviceEpoch(epoch)
	if err != nil {
		return err
	}
	count, err := g.queries.TouchAppTokenLastSeen(ctx, sqlcgen.TouchAppTokenLastSeenParams{
		ID: id, RevokeEpoch: value, LastSeenAt: nullableGatewayTime(now),
	})
	if err != nil {
		return fmt.Errorf("store: touch app token last use: %w", err)
	}
	if count == 0 {
		return gateway.ErrAppTokenRevoked
	}
	return nil
}

func (g *GatewayRepo) ListActiveAppTokenOrigins(ctx context.Context) ([]string, error) {
	if err := g.checkReady(ctx, "list active app token origins"); err != nil {
		return nil, err
	}
	origins, err := g.queries.ListActiveAppTokenOrigins(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list active app token origins: %w", err)
	}
	return origins, nil
}

func getAppToken(ctx context.Context, queries *sqlcgen.Queries, id string) (gateway.AppToken, error) {
	row, err := queries.GetAppToken(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return gateway.AppToken{}, gateway.ErrAppTokenNotFound
	}
	if err != nil {
		return gateway.AppToken{}, fmt.Errorf("store: get app token: %w", err)
	}
	return appTokenFromGenerated(ctx, queries, row)
}

func appTokenFromGenerated(
	ctx context.Context,
	queries *sqlcgen.Queries,
	row sqlcgen.GatewayDeviceSession,
) (gateway.AppToken, error) {
	session, err := gatewayDeviceSessionFromGenerated(row)
	if err != nil {
		return gateway.AppToken{}, err
	}
	origins, err := queries.ListAppTokenOrigins(ctx, session.ID)
	if err != nil {
		return gateway.AppToken{}, fmt.Errorf("store: list app token origins: %w", err)
	}
	if origins == nil {
		origins = []string{}
	}
	return gateway.AppToken{Session: session, Origins: origins}, nil
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
			DeviceID: id, Origin: origin, CreatedAt: store.FormatTimestamp(now),
		}); err != nil {
			return fmt.Errorf("store: insert app token origin: %w", err)
		}
	}
	return nil
}
