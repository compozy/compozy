package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/vault"
)

var _ mcpauth.RegistrationStore = (*VaultRepo)(nil)

// SaveMCPAuthRegistration atomically persists one registration and its DCR secrets.
func (g *VaultRepo) SaveMCPAuthRegistration(
	ctx context.Context,
	registration mcpauth.ClientRegistration,
	secrets mcpauth.RegistrationSecrets,
) (mcpauth.ClientRegistration, error) {
	if err := g.checkReady(ctx, "save MCP OAuth registration"); err != nil {
		return mcpauth.ClientRegistration{}, err
	}
	normalized, normalizedSecrets, err := normalizeMCPOAuthRegistration(registration, secrets, g.now())
	if err != nil {
		return mcpauth.ClientRegistration{}, err
	}
	scopesJSON, err := json.Marshal(normalized.Scopes)
	if err != nil {
		return mcpauth.ClientRegistration{}, fmt.Errorf("store: marshal MCP OAuth registration scopes: %w", err)
	}

	err = store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		previousClientSecretRef, previousRegistrationTokenRef, err := getMCPOAuthRegistrationRefsWithExecutor(
			ctx,
			tx,
			normalized.Target,
		)
		if err != nil && !errors.Is(err, mcpauth.ErrRegistrationNotFound) {
			return err
		}
		service, err := g.vaultServiceForStore(transactionVaultStore{owner: g, exec: tx})
		if err != nil {
			return err
		}
		if normalizedSecrets.ClientSecret != "" {
			if _, err := service.PutSecret(
				ctx,
				normalized.ClientSecretRef,
				"mcp_oauth_dcr_client_secret",
				normalizedSecrets.ClientSecret,
			); err != nil {
				return fmt.Errorf(
					"store: save MCP OAuth registration client secret for %q: %w",
					normalized.Target.ServerName,
					err,
				)
			}
		}
		if normalizedSecrets.RegistrationAccessToken != "" {
			if _, err := service.PutSecret(
				ctx,
				normalized.RegistrationAccessTokenRef,
				"mcp_oauth_registration_access_token",
				normalizedSecrets.RegistrationAccessToken,
			); err != nil {
				return fmt.Errorf(
					"store: save MCP OAuth registration access token for %q: %w",
					normalized.Target.ServerName,
					err,
				)
			}
		}
		params := registrationUpsertParams(normalized, string(scopesJSON))
		if err := queries.UpsertMCPOAuthRegistration(ctx, params); err != nil {
			return fmt.Errorf("store: save MCP OAuth registration for %q: %w", normalized.Target.ServerName, err)
		}
		for _, ref := range staleMCPOAuthRegistrationRefs(
			previousClientSecretRef,
			previousRegistrationTokenRef,
			normalized.ClientSecretRef,
			normalized.RegistrationAccessTokenRef,
		) {
			if err := deleteMCPRegistrationSecret(ctx, queries, normalized.Target.ServerName, ref); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return mcpauth.ClientRegistration{}, err
	}
	return normalized, nil
}

// GetMCPAuthRegistration returns one target-scoped OAuth client registration.
func (g *VaultRepo) GetMCPAuthRegistration(
	ctx context.Context,
	target mcpauth.Target,
) (mcpauth.ClientRegistration, error) {
	if err := g.checkReady(ctx, "get MCP OAuth registration"); err != nil {
		return mcpauth.ClientRegistration{}, err
	}
	target = target.Normalize()
	if err := target.Validate(); err != nil {
		return mcpauth.ClientRegistration{}, fmt.Errorf("store: invalid MCP auth target: %w", err)
	}
	row, err := g.queries.GetMCPOAuthRegistration(ctx, mcpOAuthRegistrationTargetParams(target))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return mcpauth.ClientRegistration{}, mcpauth.ErrRegistrationNotFound
		}
		return mcpauth.ClientRegistration{}, fmt.Errorf(
			"store: get MCP OAuth registration for %q: %w",
			target.ServerName,
			err,
		)
	}
	return mcpOAuthRegistrationFromGenerated(row)
}

// DeleteMCPAuthRegistration removes one target-scoped OAuth client registration and its DCR secrets.
func (g *VaultRepo) DeleteMCPAuthRegistration(ctx context.Context, target mcpauth.Target) error {
	if err := g.checkReady(ctx, "delete MCP OAuth registration"); err != nil {
		return err
	}
	target = target.Normalize()
	if err := target.Validate(); err != nil {
		return fmt.Errorf("store: invalid MCP auth target: %w", err)
	}
	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		clientSecretRef, registrationTokenRef, err := getMCPOAuthRegistrationRefsWithExecutor(ctx, tx, target)
		if err != nil {
			if errors.Is(err, mcpauth.ErrRegistrationNotFound) {
				return nil
			}
			return err
		}
		queries := sqlcgen.New(tx)
		if _, err := queries.DeleteMCPOAuthRegistration(ctx, mcpOAuthRegistrationDeleteParams(target)); err != nil {
			return fmt.Errorf("store: delete MCP OAuth registration for %q: %w", target.ServerName, err)
		}
		for _, ref := range []string{clientSecretRef, registrationTokenRef} {
			if err := deleteMCPRegistrationSecret(ctx, queries, target.ServerName, ref); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteMCPAuthorizationState atomically removes OAuth tokens, DCR registration state, and their Vault secrets.
func (g *VaultRepo) DeleteMCPAuthorizationState(ctx context.Context, target mcpauth.Target) error {
	if err := g.checkReady(ctx, "delete MCP authorization state"); err != nil {
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
		clientSecretRef, registrationTokenRef, err := getMCPOAuthRegistrationRefsWithExecutor(ctx, tx, target)
		if err != nil && !errors.Is(err, mcpauth.ErrRegistrationNotFound) {
			return err
		}
		queries := sqlcgen.New(tx)
		if _, err := queries.DeleteMCPAuthToken(ctx, mcpAuthDeleteParams(target)); err != nil {
			return fmt.Errorf("store: delete MCP auth token for %q: %w", target.ServerName, err)
		}
		if _, err := queries.DeleteMCPOAuthRegistration(ctx, mcpOAuthRegistrationDeleteParams(target)); err != nil {
			return fmt.Errorf("store: delete MCP OAuth registration for %q: %w", target.ServerName, err)
		}
		for _, ref := range uniqueMCPAuthorizationRefs(
			accessTokenRef,
			refreshTokenRef,
			clientSecretRef,
			registrationTokenRef,
		) {
			if err := deleteMCPAuthorizationSecret(ctx, queries, target.ServerName, ref); err != nil {
				return err
			}
		}
		return nil
	})
}

func normalizeMCPOAuthRegistration(
	registration mcpauth.ClientRegistration,
	secrets mcpauth.RegistrationSecrets,
	now time.Time,
) (mcpauth.ClientRegistration, mcpauth.RegistrationSecrets, error) {
	registration.Target = registration.Target.Normalize()
	registration.DefinitionFingerprint = strings.TrimSpace(registration.DefinitionFingerprint)
	registration.ResourceURL = strings.TrimSpace(registration.ResourceURL)
	registration.Issuer = strings.TrimSpace(registration.Issuer)
	registration.ClientID = strings.TrimSpace(registration.ClientID)
	registration.TokenEndpointAuthMethod = strings.TrimSpace(registration.TokenEndpointAuthMethod)
	registration.RegistrationClientURI = strings.TrimSpace(registration.RegistrationClientURI)
	secrets.ClientSecret = strings.TrimSpace(secrets.ClientSecret)
	secrets.RegistrationAccessToken = strings.TrimSpace(secrets.RegistrationAccessToken)
	registration.RedirectURL = strings.TrimSpace(registration.RedirectURL)
	registration.Scopes = trimTokenScopes(registration.Scopes)
	if registration.Scopes == nil {
		registration.Scopes = []string{}
	}
	if registration.UpdatedAt.IsZero() {
		registration.UpdatedAt = now.UTC()
	}
	if err := registration.Target.Validate(); err != nil {
		return mcpauth.ClientRegistration{}, mcpauth.RegistrationSecrets{}, fmt.Errorf(
			"store: invalid MCP auth target: %w",
			err,
		)
	}
	if err := mcpauth.ValidateDefinitionFingerprint(registration.DefinitionFingerprint); err != nil {
		return mcpauth.ClientRegistration{}, mcpauth.RegistrationSecrets{}, fmt.Errorf(
			"store: invalid MCP OAuth registration fingerprint: %w",
			err,
		)
	}
	if registration.ResourceURL == "" {
		return mcpauth.ClientRegistration{}, mcpauth.RegistrationSecrets{}, errors.New(
			"store: MCP OAuth registration resource URL is required",
		)
	}
	if registration.Issuer == "" {
		return mcpauth.ClientRegistration{}, mcpauth.RegistrationSecrets{}, errors.New(
			"store: MCP OAuth registration issuer is required",
		)
	}
	if registration.ClientID == "" {
		return mcpauth.ClientRegistration{}, mcpauth.RegistrationSecrets{}, errors.New(
			"store: MCP OAuth registration client ID is required",
		)
	}
	if registration.RedirectURL == "" {
		return mcpauth.ClientRegistration{}, mcpauth.RegistrationSecrets{}, errors.New(
			"store: MCP OAuth registration redirect URL is required",
		)
	}
	if (secrets.RegistrationAccessToken == "") != (registration.RegistrationClientURI == "") {
		return mcpauth.ClientRegistration{}, mcpauth.RegistrationSecrets{}, errors.New(
			"store: MCP OAuth registration access token and client URI must be set together",
		)
	}
	refs, err := vault.MCPDCRSecretRefsForTarget(
		string(registration.Target.Scope),
		registration.Target.WorkspaceID,
		registration.Target.ServerName,
	)
	if err != nil {
		return mcpauth.ClientRegistration{}, mcpauth.RegistrationSecrets{}, fmt.Errorf(
			"store: build MCP OAuth registration refs: %w",
			err,
		)
	}
	registration.ClientSecretRef = ""
	registration.RegistrationAccessTokenRef = ""
	if secrets.ClientSecret != "" {
		registration.ClientSecretRef = refs.ClientSecretRef
	}
	if secrets.RegistrationAccessToken != "" {
		registration.RegistrationAccessTokenRef = refs.RegistrationAccessTokenRef
	}
	return registration, secrets, nil
}

func registrationUpsertParams(
	registration mcpauth.ClientRegistration,
	scopesJSON string,
) sqlcgen.UpsertMCPOAuthRegistrationParams {
	return sqlcgen.UpsertMCPOAuthRegistrationParams{
		Scope:                      string(registration.Target.Scope),
		WorkspaceID:                registration.Target.WorkspaceID,
		ServerName:                 registration.Target.ServerName,
		DefinitionFingerprint:      registration.DefinitionFingerprint,
		ResourceUrl:                registration.ResourceURL,
		Issuer:                     registration.Issuer,
		ClientID:                   registration.ClientID,
		TokenEndpointAuthMethod:    registration.TokenEndpointAuthMethod,
		ClientSecretRef:            registration.ClientSecretRef,
		RegistrationAccessTokenRef: registration.RegistrationAccessTokenRef,
		RegistrationClientUri:      registration.RegistrationClientURI,
		ClientIDIssuedAt:           nullableMCPTime(registration.ClientIDIssuedAt),
		ClientSecretExpiresAt:      nullableMCPTime(registration.ClientSecretExpiresAt),
		RedirectUri:                registration.RedirectURL,
		ScopesJson:                 scopesJSON,
		UpdatedAt:                  store.FormatTimestamp(registration.UpdatedAt),
	}
}

func mcpOAuthRegistrationFromGenerated(row sqlcgen.McpOauthRegistration) (mcpauth.ClientRegistration, error) {
	registration := mcpauth.ClientRegistration{
		Target: mcpauth.Target{
			Scope:       mcpauth.Scope(row.Scope),
			WorkspaceID: row.WorkspaceID,
			ServerName:  row.ServerName,
		},
		DefinitionFingerprint:      row.DefinitionFingerprint,
		ResourceURL:                row.ResourceUrl,
		Issuer:                     row.Issuer,
		ClientID:                   row.ClientID,
		TokenEndpointAuthMethod:    row.TokenEndpointAuthMethod,
		ClientSecretRef:            row.ClientSecretRef,
		RegistrationAccessTokenRef: row.RegistrationAccessTokenRef,
		RegistrationClientURI:      row.RegistrationClientUri,
		RedirectURL:                row.RedirectUri,
	}
	if err := json.Unmarshal([]byte(row.ScopesJson), &registration.Scopes); err != nil {
		return mcpauth.ClientRegistration{}, fmt.Errorf("store: decode MCP OAuth registration scopes: %w", err)
	}
	if row.ClientIDIssuedAt.Valid {
		issuedAt, err := store.ParseTimestamp(row.ClientIDIssuedAt.String)
		if err != nil {
			return mcpauth.ClientRegistration{}, fmt.Errorf(
				"store: parse MCP OAuth registration client_id_issued_at: %w",
				err,
			)
		}
		registration.ClientIDIssuedAt = issuedAt
	}
	if row.ClientSecretExpiresAt.Valid {
		expiresAt, err := store.ParseTimestamp(row.ClientSecretExpiresAt.String)
		if err != nil {
			return mcpauth.ClientRegistration{}, fmt.Errorf(
				"store: parse MCP OAuth registration client_secret_expires_at: %w",
				err,
			)
		}
		registration.ClientSecretExpiresAt = expiresAt
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return mcpauth.ClientRegistration{}, fmt.Errorf("store: parse MCP OAuth registration updated_at: %w", err)
	}
	registration.UpdatedAt = updatedAt
	return registration, nil
}

func getMCPOAuthRegistrationRefsWithExecutor(
	ctx context.Context,
	exec globalSQLExecutor,
	target mcpauth.Target,
) (string, string, error) {
	row, err := sqlcgen.New(exec).GetMCPOAuthRegistrationRefs(ctx, mcpOAuthRegistrationRefsParams(target))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", mcpauth.ErrRegistrationNotFound
		}
		return "", "", fmt.Errorf("store: get MCP OAuth registration refs for %q: %w", target.ServerName, err)
	}
	return strings.TrimSpace(row.ClientSecretRef), strings.TrimSpace(row.RegistrationAccessTokenRef), nil
}

func staleMCPOAuthRegistrationRefs(
	previousClientSecretRef string,
	previousRegistrationTokenRef string,
	clientSecretRef string,
	registrationTokenRef string,
) []string {
	previous := []string{previousClientSecretRef, previousRegistrationTokenRef}
	current := map[string]struct{}{clientSecretRef: {}, registrationTokenRef: {}}
	stale := make([]string, 0, len(previous))
	for _, ref := range previous {
		if strings.TrimSpace(ref) == "" {
			continue
		}
		if _, retained := current[ref]; retained {
			continue
		}
		stale = append(stale, ref)
	}
	return stale
}

func deleteMCPRegistrationSecret(
	ctx context.Context,
	queries *sqlcgen.Queries,
	serverName string,
	ref string,
) error {
	if strings.TrimSpace(ref) == "" {
		return nil
	}
	if _, err := queries.DeleteVaultSecret(ctx, ref); err != nil {
		return fmt.Errorf("store: delete MCP OAuth registration secret for %q: %w", serverName, err)
	}
	return nil
}

func uniqueMCPAuthorizationRefs(refs ...string) []string {
	unique := make(map[string]struct{}, len(refs))
	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			continue
		}
		if _, exists := unique[trimmed]; exists {
			continue
		}
		unique[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func deleteMCPAuthorizationSecret(
	ctx context.Context,
	queries *sqlcgen.Queries,
	serverName string,
	ref string,
) error {
	if _, err := queries.DeleteVaultSecret(ctx, ref); err != nil {
		return fmt.Errorf("store: delete MCP authorization secret for %q: %w", serverName, err)
	}
	return nil
}

func mcpOAuthRegistrationTargetParams(target mcpauth.Target) sqlcgen.GetMCPOAuthRegistrationParams {
	return sqlcgen.GetMCPOAuthRegistrationParams{
		Scope: string(target.Scope), WorkspaceID: target.WorkspaceID, ServerName: target.ServerName,
	}
}

func mcpOAuthRegistrationDeleteParams(target mcpauth.Target) sqlcgen.DeleteMCPOAuthRegistrationParams {
	return sqlcgen.DeleteMCPOAuthRegistrationParams{
		Scope: string(target.Scope), WorkspaceID: target.WorkspaceID, ServerName: target.ServerName,
	}
}

func mcpOAuthRegistrationRefsParams(target mcpauth.Target) sqlcgen.GetMCPOAuthRegistrationRefsParams {
	return sqlcgen.GetMCPOAuthRegistrationRefsParams{
		Scope: string(target.Scope), WorkspaceID: target.WorkspaceID, ServerName: target.ServerName,
	}
}
