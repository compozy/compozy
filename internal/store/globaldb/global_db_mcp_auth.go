package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	"github.com/compozy/agh/internal/vault"
)

const (
	globalDBMCPAuthBearerValue = "Bearer"
)

var _ mcpauth.TokenStore = (*VaultRepo)(nil)

// SaveMCPAuthToken persists one remote MCP OAuth token record.
func (g *VaultRepo) SaveMCPAuthToken(ctx context.Context, token mcpauth.TokenRecord) error {
	if err := g.checkReady(ctx, "save MCP auth token"); err != nil {
		return err
	}
	normalized, err := normalizeMCPAuthToken(token, g.now())
	if err != nil {
		return err
	}
	scopesJSON, err := json.Marshal(normalized.Scopes)
	if err != nil {
		return fmt.Errorf("store: marshal MCP auth token scopes: %w", err)
	}
	prefix, err := vault.MCPSecretOwnerPrefix(
		string(normalized.Target.Scope),
		normalized.Target.WorkspaceID,
		normalized.Target.ServerName,
	)
	if err != nil {
		return fmt.Errorf("store: build MCP auth token ref: %w", err)
	}
	accessTokenRef := prefix + "oauth/access-token"
	refreshTokenRef := ""
	if strings.TrimSpace(normalized.RefreshToken) != "" {
		refreshTokenRef = prefix + "oauth/refresh-token"
	}

	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		service, err := g.vaultServiceForStore(transactionVaultStore{owner: g, exec: tx})
		if err != nil {
			return err
		}
		if _, err := service.PutSecret(
			ctx,
			accessTokenRef,
			"mcp_oauth_access_token",
			normalized.AccessToken,
		); err != nil {
			return fmt.Errorf("store: save MCP auth access token for %q: %w", normalized.Target.ServerName, err)
		}
		if refreshTokenRef != "" {
			if _, err := service.PutSecret(
				ctx,
				refreshTokenRef,
				"mcp_oauth_refresh_token",
				normalized.RefreshToken,
			); err != nil {
				return fmt.Errorf("store: save MCP auth refresh token for %q: %w", normalized.Target.ServerName, err)
			}
		} else if err := service.DeleteSecret(ctx, prefix+"oauth/refresh-token"); err != nil &&
			!errors.Is(err, vault.ErrSecretNotFound) {
			return fmt.Errorf("store: clear MCP auth refresh token for %q: %w", normalized.Target.ServerName, err)
		}

		err = sqlcgen.New(tx).UpsertMCPAuthToken(ctx, sqlcgen.UpsertMCPAuthTokenParams{
			Scope: string(normalized.Target.Scope), WorkspaceID: normalized.Target.WorkspaceID,
			ServerName: normalized.Target.ServerName, DefinitionFingerprint: normalized.DefinitionFingerprint,
			Issuer: normalized.Issuer, ClientID: normalized.ClientID,
			ScopesJson: string(scopesJSON), AccessTokenRef: accessTokenRef, RefreshTokenRef: refreshTokenRef,
			TokenType: normalized.TokenType, ExpiresAt: nullableMCPTime(normalized.ExpiresAt),
			ObtainedAt: store.FormatTimestamp(normalized.ObtainedAt),
			UpdatedAt:  store.FormatTimestamp(normalized.UpdatedAt),
		})
		if err != nil {
			return fmt.Errorf("store: save MCP auth token for %q: %w", normalized.Target.ServerName, err)
		}
		return nil
	})
}

// GetMCPAuthToken returns one persisted token record.
func (g *VaultRepo) GetMCPAuthToken(ctx context.Context, target mcpauth.Target) (mcpauth.TokenRecord, error) {
	if err := g.checkReady(ctx, "get MCP auth token"); err != nil {
		return mcpauth.TokenRecord{}, err
	}
	target = target.Normalize()
	if err := target.Validate(); err != nil {
		return mcpauth.TokenRecord{}, fmt.Errorf("store: invalid MCP auth target: %w", err)
	}

	row, err := g.queries.GetMCPAuthToken(ctx, mcpAuthTargetParams(target))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return mcpauth.TokenRecord{}, mcpauth.ErrTokenNotFound
		}
		return mcpauth.TokenRecord{}, fmt.Errorf("store: get MCP auth token for %q: %w", target.ServerName, err)
	}
	token, err := mcpAuthTokenFromGenerated(row)
	if err != nil {
		return mcpauth.TokenRecord{}, err
	}
	if err := g.resolveMCPAuthTokenSecrets(ctx, &token); err != nil {
		return mcpauth.TokenRecord{}, err
	}
	return token, nil
}

// ListMCPAuthTokens returns all persisted token records.
func (g *VaultRepo) ListMCPAuthTokens(ctx context.Context) ([]mcpauth.TokenRecord, error) {
	if err := g.checkReady(ctx, "list MCP auth tokens"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListMCPAuthTokens(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list MCP auth tokens: %w", err)
	}

	tokens := make([]mcpauth.TokenRecord, 0, len(rows))
	for _, row := range rows {
		token, mapErr := mcpAuthTokenFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		if err := g.resolveMCPAuthTokenSecrets(ctx, &token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

// DeleteMCPAuthToken removes persisted token state for one server.
func (g *VaultRepo) DeleteMCPAuthToken(ctx context.Context, target mcpauth.Target) error {
	if err := g.checkReady(ctx, "delete MCP auth token"); err != nil {
		return err
	}
	target = target.Normalize()
	if err := target.Validate(); err != nil {
		return fmt.Errorf("store: invalid MCP auth target: %w", err)
	}
	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		accessTokenRef, refreshTokenRef, err := getMCPAuthTokenRefsWithExecutor(ctx, tx, target)
		if err != nil && !errors.Is(err, mcpauth.ErrTokenNotFound) {
			return err
		}
		if _, err := sqlcgen.New(tx).DeleteMCPAuthToken(ctx, mcpAuthDeleteParams(target)); err != nil {
			return fmt.Errorf("store: delete MCP auth token for %q: %w", target.ServerName, err)
		}
		service, err := g.vaultServiceForStore(transactionVaultStore{owner: g, exec: tx})
		if err != nil {
			return err
		}
		for _, ref := range []string{accessTokenRef, refreshTokenRef} {
			if strings.TrimSpace(ref) == "" {
				continue
			}
			if err := service.DeleteSecret(ctx, ref); err != nil && !errors.Is(err, vault.ErrSecretNotFound) {
				return fmt.Errorf("store: delete MCP auth token secret for %q: %w", target.ServerName, err)
			}
		}
		return nil
	})
}

func normalizeMCPAuthToken(token mcpauth.TokenRecord, now time.Time) (mcpauth.TokenRecord, error) {
	token.Target = token.Target.Normalize()
	token.DefinitionFingerprint = strings.TrimSpace(token.DefinitionFingerprint)
	token.Issuer = strings.TrimSpace(token.Issuer)
	token.ClientID = strings.TrimSpace(token.ClientID)
	token.AccessToken = strings.TrimSpace(token.AccessToken)
	token.RefreshToken = strings.TrimSpace(token.RefreshToken)
	token.TokenType = strings.TrimSpace(token.TokenType)
	if token.TokenType == "" {
		token.TokenType = globalDBMCPAuthBearerValue
	}
	token.Scopes = trimTokenScopes(token.Scopes)
	if token.ObtainedAt.IsZero() {
		token.ObtainedAt = now.UTC()
	}
	if token.UpdatedAt.IsZero() {
		token.UpdatedAt = now.UTC()
	}
	if err := token.Target.Validate(); err != nil {
		return mcpauth.TokenRecord{}, fmt.Errorf("store: invalid MCP auth target: %w", err)
	}
	if err := mcpauth.ValidateDefinitionFingerprint(token.DefinitionFingerprint); err != nil {
		return mcpauth.TokenRecord{}, fmt.Errorf("store: invalid MCP auth definition fingerprint: %w", err)
	}
	switch {
	case token.ClientID == "":
		return mcpauth.TokenRecord{}, errors.New("store: MCP auth token client id is required")
	case token.AccessToken == "":
		return mcpauth.TokenRecord{}, errors.New("store: MCP auth token access token is required")
	default:
		return token, nil
	}
}

func trimTokenScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	trimmed := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if value := strings.TrimSpace(scope); value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return trimmed
}

func (g *VaultRepo) resolveMCPAuthTokenSecrets(ctx context.Context, token *mcpauth.TokenRecord) error {
	if token == nil {
		return nil
	}
	service, err := g.vaultService()
	if err != nil {
		return err
	}
	accessToken, err := resolveMCPAuthTokenRef(ctx, service, token.Target.ServerName, "access_token", token.AccessToken)
	if err != nil {
		return err
	}
	refreshToken, err := resolveMCPAuthTokenRef(
		ctx,
		service,
		token.Target.ServerName,
		"refresh_token",
		token.RefreshToken,
	)
	if err != nil {
		return err
	}
	token.AccessToken = accessToken
	token.RefreshToken = refreshToken
	return nil
}

func resolveMCPAuthTokenRef(
	ctx context.Context,
	service *vault.Service,
	serverName string,
	fieldName string,
	ref string,
) (string, error) {
	trimmedRef := strings.TrimSpace(ref)
	if trimmedRef == "" {
		return "", nil
	}
	if err := vault.ValidateSecretRefNamespace(trimmedRef, "mcp"); err != nil {
		return "", fmt.Errorf(
			"store: MCP auth token %s for %q is not a vault ref: %w",
			fieldName,
			strings.TrimSpace(serverName),
			err,
		)
	}
	value, err := service.ResolveRef(ctx, trimmedRef)
	if err != nil {
		return "", fmt.Errorf(
			"store: resolve MCP auth token %s for %q: %w",
			fieldName,
			strings.TrimSpace(serverName),
			err,
		)
	}
	return value, nil
}

func getMCPAuthTokenRefsWithExecutor(
	ctx context.Context,
	exec globalSQLExecutor,
	target mcpauth.Target,
) (string, string, error) {
	target = target.Normalize()
	row, err := sqlcgen.New(exec).GetMCPAuthTokenRefs(ctx, mcpAuthRefsParams(target))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", mcpauth.ErrTokenNotFound
		}
		return "", "", fmt.Errorf("store: get MCP auth token refs for %q: %w", target.ServerName, err)
	}
	return strings.TrimSpace(row.AccessTokenRef), strings.TrimSpace(row.RefreshTokenRef), nil
}

func nullableMCPTime(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(value), Valid: true}
}

func mcpAuthTokenFromGenerated(row sqlcgen.McpAuthToken) (mcpauth.TokenRecord, error) {
	token := mcpauth.TokenRecord{
		Target: mcpauth.Target{
			Scope: mcpauth.Scope(row.Scope), WorkspaceID: row.WorkspaceID, ServerName: row.ServerName,
		},
		DefinitionFingerprint: row.DefinitionFingerprint, Issuer: row.Issuer, ClientID: row.ClientID,
		AccessToken: row.AccessTokenRef, RefreshToken: row.RefreshTokenRef, TokenType: row.TokenType,
	}
	if strings.TrimSpace(row.ScopesJson) != "" {
		if err := json.Unmarshal([]byte(row.ScopesJson), &token.Scopes); err != nil {
			return mcpauth.TokenRecord{}, fmt.Errorf("store: decode MCP auth token scopes: %w", err)
		}
	}
	if row.ExpiresAt.Valid && strings.TrimSpace(row.ExpiresAt.String) != "" {
		expiresAt, err := store.ParseTimestamp(row.ExpiresAt.String)
		if err != nil {
			return mcpauth.TokenRecord{}, fmt.Errorf("store: parse MCP auth token expires_at: %w", err)
		}
		token.ExpiresAt = expiresAt
	}
	obtainedAt, err := store.ParseTimestamp(row.ObtainedAt)
	if err != nil {
		return mcpauth.TokenRecord{}, fmt.Errorf("store: parse MCP auth token obtained_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return mcpauth.TokenRecord{}, fmt.Errorf("store: parse MCP auth token updated_at: %w", err)
	}
	token.ObtainedAt, token.UpdatedAt = obtainedAt, updatedAt
	return token, nil
}

func mcpAuthTargetParams(target mcpauth.Target) sqlcgen.GetMCPAuthTokenParams {
	return sqlcgen.GetMCPAuthTokenParams{
		Scope: string(target.Scope), WorkspaceID: target.WorkspaceID, ServerName: target.ServerName,
	}
}

func mcpAuthDeleteParams(target mcpauth.Target) sqlcgen.DeleteMCPAuthTokenParams {
	return sqlcgen.DeleteMCPAuthTokenParams{
		Scope: string(target.Scope), WorkspaceID: target.WorkspaceID, ServerName: target.ServerName,
	}
}

func mcpAuthRefsParams(target mcpauth.Target) sqlcgen.GetMCPAuthTokenRefsParams {
	return sqlcgen.GetMCPAuthTokenRefsParams{
		Scope: string(target.Scope), WorkspaceID: target.WorkspaceID, ServerName: target.ServerName,
	}
}
