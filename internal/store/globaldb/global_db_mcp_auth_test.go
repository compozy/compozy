package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/vault"
)

const testMCPDefinitionFingerprint = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestMCPAuthTokenStorePersistsAcrossReopenWithPrivatePermissions(t *testing.T) {
	t.Parallel()

	t.Run("Should persist encrypted scoped tokens across reopen with private permissions", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
		db, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("os.Stat(%q) error = %v", path, err)
		}
		if got := info.Mode().Perm() & 0o077; got != 0 {
			t.Fatalf("global db permissions = %#o, want no group/other bits", info.Mode().Perm())
		}

		expiresAt := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
		if err := db.SaveMCPAuthToken(ctx, mcpauth.TokenRecord{
			Target:                globalMCPAuthTarget("linear"),
			DefinitionFingerprint: testMCPDefinitionFingerprint,
			Issuer:                "https://issuer.example",
			ClientID:              "client",
			Scopes:                []string{"read", "write"},
			AccessToken:           "access-token",
			RefreshToken:          "refresh-token",
			TokenType:             "Bearer",
			ExpiresAt:             expiresAt,
			ObtainedAt:            expiresAt.Add(-time.Hour),
			UpdatedAt:             expiresAt.Add(-time.Minute),
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken() error = %v", err)
		}
		var rawAccessToken, rawRefreshToken string
		if err := db.db.QueryRowContext(
			ctx,
			`SELECT access_token_ref, refresh_token_ref FROM mcp_auth_tokens
		 WHERE scope = ? AND workspace_id = ? AND server_name = ?`,
			"global",
			"",
			"linear",
		).Scan(&rawAccessToken, &rawRefreshToken); err != nil {
			t.Fatalf("query raw token row error = %v", err)
		}
		for label, raw := range map[string]string{"access": rawAccessToken, "refresh": rawRefreshToken} {
			plaintext := label + "-token"
			if raw == plaintext {
				t.Fatalf("raw %s token = %q, want vault ref without plaintext token material", label, raw)
			}
			if !strings.HasPrefix(raw, "vault:mcp/") {
				t.Fatalf("raw %s token = %q, want vault:mcp ref", label, raw)
			}
		}
		if got, want := rawAccessToken, "vault:mcp/global/linear/oauth/access-token"; got != want {
			t.Fatalf("raw access token ref = %q, want %q", got, want)
		}
		if got, want := rawRefreshToken, "vault:mcp/global/linear/oauth/refresh-token"; got != want {
			t.Fatalf("raw refresh token ref = %q, want %q", got, want)
		}
		var encryptedValue string
		if err := db.db.QueryRowContext(
			ctx,
			`SELECT encrypted_value FROM vault_secrets WHERE ref = ?`,
			rawAccessToken,
		).Scan(&encryptedValue); err != nil {
			t.Fatalf("query vault access token error = %v", err)
		}
		if strings.Contains(encryptedValue, "access-token") {
			t.Fatalf("vault encrypted access token = %q, want no plaintext token material", encryptedValue)
		}
		if _, err := os.Stat(path + ".mcp-auth.key"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(legacy MCP auth key) error = %v, want not exists", err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(ctx); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})

		token, err := reopened.GetMCPAuthToken(ctx, globalMCPAuthTarget("linear"))
		if err != nil {
			t.Fatalf("GetMCPAuthToken() error = %v", err)
		}
		if token.AccessToken != "access-token" || token.RefreshToken != "refresh-token" {
			t.Fatalf("token = %#v, want persisted token material", token)
		}
		if len(token.Scopes) != 2 || token.Scopes[0] != "read" || token.Scopes[1] != "write" {
			t.Fatalf("token scopes = %#v", token.Scopes)
		}
		if !token.ExpiresAt.Equal(expiresAt) {
			t.Fatalf("token expires_at = %s, want %s", token.ExpiresAt, expiresAt)
		}
		replacement := mcpauth.TokenRecord{
			Target: globalMCPAuthTarget("linear"), DefinitionFingerprint: testMCPDefinitionFingerprint,
			ClientID:    "client",
			AccessToken: "replacement-access-token",
			ObtainedAt:  expiresAt,
			UpdatedAt:   expiresAt,
		}
		if err := reopened.SaveMCPAuthToken(ctx, replacement); err != nil {
			t.Fatalf("SaveMCPAuthToken(without refresh) error = %v", err)
		}
		replaced, err := reopened.GetMCPAuthToken(ctx, replacement.Target)
		if err != nil {
			t.Fatalf("GetMCPAuthToken(replaced) error = %v", err)
		}
		if replaced.AccessToken != replacement.AccessToken || replaced.RefreshToken != "" {
			t.Fatalf("replaced token = %#v, want new access token without refresh token", replaced)
		}
		if err := reopened.Close(ctx); err != nil {
			t.Fatalf("Close(after rotation) error = %v", err)
		}

		reopenedAfterRotation, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(after rotation) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopenedAfterRotation.Close(ctx); err != nil {
				t.Errorf("Close(reopened after rotation) error = %v", err)
			}
		})
		replaced, err = reopenedAfterRotation.GetMCPAuthToken(ctx, replacement.Target)
		if err != nil {
			t.Fatalf("GetMCPAuthToken(after rotation reopen) error = %v", err)
		}
		if replaced.AccessToken != replacement.AccessToken || replaced.RefreshToken != "" {
			t.Fatalf("reopened rotated token = %#v, want new access token without refresh token", replaced)
		}
		assertVaultRefPresence(ctx, t, reopenedAfterRotation.db, rawRefreshToken, false)

		if err := reopenedAfterRotation.DeleteMCPAuthToken(ctx, globalMCPAuthTarget("linear")); err != nil {
			t.Fatalf("DeleteMCPAuthToken() error = %v", err)
		}
		if _, err := reopenedAfterRotation.GetMCPAuthToken(
			ctx,
			globalMCPAuthTarget("linear"),
		); !errors.Is(
			err,
			mcpauth.ErrTokenNotFound,
		) {
			t.Fatalf("GetMCPAuthToken(deleted) error = %v, want ErrTokenNotFound", err)
		}
	})
}

func TestMCPAuthTokenStoreRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		token mcpauth.TokenRecord
	}{
		{
			name: "Should reject a missing access token",
			token: mcpauth.TokenRecord{
				Target: globalMCPAuthTarget("linear"), DefinitionFingerprint: testMCPDefinitionFingerprint,
				ClientID: "client",
			},
		},
		{
			name: "Should reject a missing client ID",
			token: mcpauth.TokenRecord{
				Target: globalMCPAuthTarget("linear"), DefinitionFingerprint: testMCPDefinitionFingerprint,
				AccessToken: "access-token",
			},
		},
		{
			name: "Should reject a missing definition fingerprint",
			token: mcpauth.TokenRecord{
				Target: globalMCPAuthTarget("linear"), ClientID: "client", AccessToken: "access-token",
			},
		},
		{
			name: "Should reject an ambiguous target",
			token: mcpauth.TokenRecord{
				Target: mcpauth.Target{
					Scope: mcpauth.ScopeUser, WorkspaceID: "workspace-a", ServerName: "linear",
				},
				DefinitionFingerprint: testMCPDefinitionFingerprint,
				ClientID:              "client",
				AccessToken:           "access-token",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			db := openTestGlobalDB(t)
			if err := db.SaveMCPAuthToken(ctx, tc.token); err == nil {
				t.Fatal("SaveMCPAuthToken() error = nil, want invalid token failure")
			}
		})
	}

	t.Run("Should validate read and delete targets", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		invalid := mcpauth.Target{Scope: mcpauth.ScopeWorkspace, ServerName: "linear"}
		if _, err := db.GetMCPAuthToken(ctx, invalid); err == nil {
			t.Fatal("GetMCPAuthToken(invalid target) error = nil, want validation failure")
		}
		if err := db.DeleteMCPAuthToken(ctx, invalid); err == nil {
			t.Fatal("DeleteMCPAuthToken(invalid target) error = nil, want validation failure")
		}
		if err := db.DeleteMCPAuthToken(ctx, globalMCPAuthTarget("missing")); err != nil {
			t.Fatalf("DeleteMCPAuthToken(missing target) error = %v, want idempotent success", err)
		}
	})
}

func TestMCPAuthTokenStoreIsolatesSameNamedScopedCredentials(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate same-named credentials across global and workspace scopes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		issuedAt := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
		targets := []mcpauth.Target{
			globalMCPAuthTarget("linear"),
			{Scope: mcpauth.ScopeWorkspace, WorkspaceID: "workspace-a", ServerName: "linear"},
			{Scope: mcpauth.ScopeWorkspace, WorkspaceID: "workspace/b", ServerName: "linear"},
			{Scope: mcpauth.ScopeWorkspace, WorkspaceID: "workspace-a", ServerName: "QA OAuth MCP"},
		}
		for index, target := range targets {
			if err := db.SaveMCPAuthToken(ctx, mcpauth.TokenRecord{
				Target:                target,
				DefinitionFingerprint: testMCPDefinitionFingerprint,
				ClientID:              "client-id",
				AccessToken:           fmt.Sprintf("access-token-%d", index),
				RefreshToken:          fmt.Sprintf("refresh-token-%d", index),
				TokenType:             "Bearer",
				ObtainedAt:            issuedAt,
				UpdatedAt:             issuedAt,
			}); err != nil {
				t.Fatalf("SaveMCPAuthToken(%#v) error = %v", target, err)
			}
		}

		for index, target := range targets {
			token, err := db.GetMCPAuthToken(ctx, target)
			if err != nil {
				t.Fatalf("GetMCPAuthToken(%#v) error = %v", target, err)
			}
			if got, want := token.AccessToken, fmt.Sprintf("access-token-%d", index); got != want {
				t.Fatalf("GetMCPAuthToken(%#v).AccessToken = %q, want %q", target, got, want)
			}
			var accessRef string
			if err := db.db.QueryRowContext(
				ctx,
				`SELECT access_token_ref FROM mcp_auth_tokens
			 WHERE scope = ? AND workspace_id = ? AND server_name = ?`,
				string(target.Scope),
				target.WorkspaceID,
				target.ServerName,
			).Scan(&accessRef); err != nil {
				t.Fatalf("query access ref for %#v error = %v", target, err)
			}
			prefix, err := vault.MCPSecretOwnerPrefix(string(target.Scope), target.WorkspaceID, target.ServerName)
			if err != nil {
				t.Fatalf("MCPSecretOwnerPrefix(%#v) error = %v", target, err)
			}
			if got, want := accessRef, prefix+"oauth/access-token"; got != want {
				t.Fatalf("access ref for %#v = %q, want %q", target, got, want)
			}
		}

		listed, err := db.ListMCPAuthTokens(ctx)
		if err != nil {
			t.Fatalf("ListMCPAuthTokens() error = %v", err)
		}
		if len(listed) != len(targets) {
			t.Fatalf("ListMCPAuthTokens() count = %d, want %d", len(listed), len(targets))
		}
		listedByTarget := make(map[string]mcpauth.TokenRecord, len(listed))
		for _, token := range listed {
			key, err := token.Target.Key()
			if err != nil {
				t.Fatalf("listed target %#v is invalid: %v", token.Target, err)
			}
			listedByTarget[key] = token
		}
		for index, target := range targets {
			key, err := target.Key()
			if err != nil {
				t.Fatalf("target %#v is invalid: %v", target, err)
			}
			token, ok := listedByTarget[key]
			if !ok {
				t.Fatalf("ListMCPAuthTokens() omitted target %#v", target)
			}
			if got, want := token.AccessToken, fmt.Sprintf("access-token-%d", index); got != want {
				t.Fatalf("listed token for %#v = %q, want %q", target, got, want)
			}
		}

		if err := db.DeleteMCPAuthToken(ctx, targets[1]); err != nil {
			t.Fatalf("DeleteMCPAuthToken(workspace-a) error = %v", err)
		}
		if _, err := db.GetMCPAuthToken(ctx, targets[1]); !errors.Is(err, mcpauth.ErrTokenNotFound) {
			t.Fatalf("GetMCPAuthToken(deleted workspace-a) error = %v, want ErrTokenNotFound", err)
		}
		for _, target := range []mcpauth.Target{targets[0], targets[2]} {
			if _, err := db.GetMCPAuthToken(ctx, target); err != nil {
				t.Fatalf("GetMCPAuthToken(preserved %#v) error = %v", target, err)
			}
		}
	})
}

func TestMCPOAuthRegistrationStorePersistsTargetScopedDCRState(t *testing.T) {
	t.Parallel()

	t.Run("Should persist DCR registration metadata and delete only its target secrets", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		issuedAt := time.Date(2026, 7, 30, 15, 0, 0, 0, time.UTC)
		target := mcpauth.Target{
			Scope: mcpauth.ScopeWorkspace, WorkspaceID: "workspace-dcr", ServerName: "linear",
		}
		registration := mcpOAuthRegistrationRecord(t, target, issuedAt)
		saved, err := db.SaveMCPAuthRegistration(ctx, registration, mcpOAuthRegistrationSecrets())
		if err != nil {
			t.Fatalf("SaveMCPAuthRegistration() error = %v", err)
		}
		stored, err := db.GetMCPAuthRegistration(ctx, target)
		if err != nil {
			t.Fatalf("GetMCPAuthRegistration() error = %v", err)
		}
		if stored.ClientID != saved.ClientID || stored.ResourceURL != saved.ResourceURL {
			t.Fatalf("stored registration = %#v, want persisted client and resource identity", stored)
		}
		if stored.TokenEndpointAuthMethod != saved.TokenEndpointAuthMethod {
			t.Fatalf(
				"stored token endpoint auth method = %q, want %q",
				stored.TokenEndpointAuthMethod,
				saved.TokenEndpointAuthMethod,
			)
		}
		if stored.ClientSecretRef != saved.ClientSecretRef ||
			stored.RegistrationAccessTokenRef != saved.RegistrationAccessTokenRef {
			t.Fatalf("stored registration refs = %#v, want DCR vault refs", stored)
		}
		if !stored.ClientIDIssuedAt.Equal(saved.ClientIDIssuedAt) ||
			!stored.ClientSecretExpiresAt.Equal(saved.ClientSecretExpiresAt) {
			t.Fatalf("stored registration times = %#v, want issued and expiry times", stored)
		}

		if err := db.DeleteMCPAuthRegistration(ctx, target); err != nil {
			t.Fatalf("DeleteMCPAuthRegistration() error = %v", err)
		}
		if _, err := db.GetMCPAuthRegistration(ctx, target); !errors.Is(err, mcpauth.ErrRegistrationNotFound) {
			t.Fatalf("GetMCPAuthRegistration(deleted) error = %v, want ErrRegistrationNotFound", err)
		}
		for _, ref := range []string{saved.ClientSecretRef, saved.RegistrationAccessTokenRef} {
			assertVaultRefPresence(ctx, t, db.db, ref, false)
		}
	})

	t.Run("Should persist empty registration scopes as a JSON array", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		registration := mcpOAuthRegistrationRecord(
			t,
			globalMCPAuthTarget("empty-scopes"),
			time.Date(2026, 7, 30, 15, 15, 0, 0, time.UTC),
		)
		registration.Scopes = nil
		saved, err := db.SaveMCPAuthRegistration(ctx, registration, mcpOAuthRegistrationSecrets())
		if err != nil {
			t.Fatalf("SaveMCPAuthRegistration(empty scopes) error = %v", err)
		}
		if saved.Scopes == nil || len(saved.Scopes) != 0 {
			t.Fatalf("SaveMCPAuthRegistration(empty scopes).Scopes = %#v, want non-nil empty slice", saved.Scopes)
		}
		var scopesJSON string
		if err := db.db.QueryRowContext(
			ctx,
			`SELECT scopes_json FROM mcp_oauth_registrations
			 WHERE scope = ? AND workspace_id = ? AND server_name = ?`,
			string(saved.Target.Scope),
			saved.Target.WorkspaceID,
			saved.Target.ServerName,
		).Scan(&scopesJSON); err != nil {
			t.Fatalf("query persisted registration scopes error = %v", err)
		}
		if scopesJSON != `[]` {
			t.Fatalf("persisted scopes_json = %q, want []", scopesJSON)
		}
	})

	t.Run("Should reject a registration access token without its management URI", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		registration := mcpOAuthRegistrationRecord(
			t,
			globalMCPAuthTarget("linear"),
			time.Date(2026, 7, 30, 15, 30, 0, 0, time.UTC),
		)
		registration.RegistrationClientURI = ""

		if _, err := db.SaveMCPAuthRegistration(ctx, registration, mcpOAuthRegistrationSecrets()); err == nil {
			t.Fatal("SaveMCPAuthRegistration() error = nil, want access token and URI validation failure")
		} else if !strings.Contains(err.Error(), "access token and client URI must be set together") {
			t.Fatalf("SaveMCPAuthRegistration() error = %v, want paired management credential validation", err)
		}
	})

	t.Run("Should delete full authorization state only for its exact target", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		issuedAt := time.Date(2026, 7, 30, 16, 0, 0, 0, time.UTC)
		targets := []mcpauth.Target{
			globalMCPAuthTarget("linear"),
			{Scope: mcpauth.ScopeWorkspace, WorkspaceID: "workspace-authorization", ServerName: "linear"},
		}
		registrations := make([]mcpauth.ClientRegistration, 0, len(targets))
		for _, target := range targets {
			if err := db.SaveMCPAuthToken(ctx, mcpAuthorizationTokenRecord(target, issuedAt)); err != nil {
				t.Fatalf("SaveMCPAuthToken(%#v) error = %v", target, err)
			}
			registration, err := db.SaveMCPAuthRegistration(
				ctx,
				mcpOAuthRegistrationRecord(t, target, issuedAt),
				mcpOAuthRegistrationSecrets(),
			)
			if err != nil {
				t.Fatalf("SaveMCPAuthRegistration(%#v) error = %v", target, err)
			}
			registrations = append(registrations, registration)
		}

		if err := db.DeleteMCPAuthorizationState(ctx, targets[0]); err != nil {
			t.Fatalf("DeleteMCPAuthorizationState(global) error = %v", err)
		}
		if err := db.DeleteMCPAuthorizationState(ctx, targets[0]); err != nil {
			t.Fatalf("DeleteMCPAuthorizationState(global retry) error = %v", err)
		}
		if _, err := db.GetMCPAuthToken(ctx, targets[0]); !errors.Is(err, mcpauth.ErrTokenNotFound) {
			t.Fatalf("GetMCPAuthToken(deleted global) error = %v, want ErrTokenNotFound", err)
		}
		if _, err := db.GetMCPAuthRegistration(ctx, targets[0]); !errors.Is(err, mcpauth.ErrRegistrationNotFound) {
			t.Fatalf("GetMCPAuthRegistration(deleted global) error = %v, want ErrRegistrationNotFound", err)
		}
		for _, ref := range mcpAuthorizationSecretRefs(t, targets[0], registrations[0]) {
			assertVaultRefPresence(ctx, t, db.db, ref, false)
		}
		if _, err := db.GetMCPAuthToken(ctx, targets[1]); err != nil {
			t.Fatalf("GetMCPAuthToken(preserved workspace) error = %v", err)
		}
		if _, err := db.GetMCPAuthRegistration(ctx, targets[1]); err != nil {
			t.Fatalf("GetMCPAuthRegistration(preserved workspace) error = %v", err)
		}
		for _, ref := range mcpAuthorizationSecretRefs(t, targets[1], registrations[1]) {
			assertVaultRefPresence(ctx, t, db.db, ref, true)
		}
	})
}

func TestMCPAuthTokenScopeMigration(t *testing.T) {
	t.Run("Should hard-cut name-only tokens while preserving scoped marketplace secrets", func(t *testing.T) {
		t.Parallel()

		databasePath := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
		legacyDB, err := sql.Open(sqliteDriverName, databasePath)
		if err != nil {
			t.Fatalf("sql.Open(legacy) error = %v", err)
		}
		prefixStream := globalMigrationPrefix(t, "00001_baseline.sql", "00002_schema.sql")
		if err := applyGlobalMigrationPrefix(t, legacyDB, prefixStream); err != nil {
			closeErr := legacyDB.Close()
			t.Fatalf("Apply(global v2) error = %v; close error = %v", err, closeErr)
		}
		ctx := testutil.Context(t)
		status, err := store.Status(ctx, legacyDB, prefixStream)
		if err != nil {
			closeErr := legacyDB.Close()
			t.Fatalf("Status(global v2) error = %v; close error = %v", err, closeErr)
		}
		if status.Version != 2 || status.AppliedCount != 2 {
			closeErr := legacyDB.Close()
			t.Fatalf(
				"Status(global v2) = %#v, want version 2 with 2 applied migrations; close error = %v",
				status,
				closeErr,
			)
		}

		legacyAccessRef := "vault:mcp/linear/oauth/access-token"
		legacyRefreshRef := "vault:mcp/linear/oauth/refresh-token"
		nestedLegacyAccessRef := "vault:mcp/team/linear/oauth/access-token"
		nestedLegacyRefreshRef := "vault:mcp/team/linear/oauth/refresh-token"
		preservedRefs := []string{
			"vault:mcp/linear/env/api-token",
			"vault:mcp/global/linear/oauth/client-secret",
			"vault:mcp/ws/workspace-a/linear/oauth/client-secret",
		}
		if _, err := legacyDB.ExecContext(
			ctx,
			`INSERT INTO mcp_auth_tokens (
				server_name, issuer, client_id, scopes_json, access_token_ref, refresh_token_ref,
				token_type, expires_at, obtained_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
			"linear", "https://issuer.example", "client", `["read"]`, legacyAccessRef, legacyRefreshRef,
			"Bearer", "2026-07-13T12:00:00Z", "2026-07-13T12:00:00Z",
		); err != nil {
			closeErr := legacyDB.Close()
			t.Fatalf("seed name-only token error = %v; close error = %v", err, closeErr)
		}
		if _, err := legacyDB.ExecContext(
			ctx,
			`INSERT INTO mcp_auth_tokens (
				server_name, issuer, client_id, scopes_json, access_token_ref, refresh_token_ref,
				token_type, expires_at, obtained_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
			"team/linear", "https://issuer.example", "client", `[]`, nestedLegacyAccessRef,
			nestedLegacyRefreshRef, "Bearer", "2026-07-13T12:00:00Z", "2026-07-13T12:00:00Z",
		); err != nil {
			closeErr := legacyDB.Close()
			t.Fatalf("seed nested name-only token error = %v; close error = %v", err, closeErr)
		}
		legacyTokenRefs := []string{
			legacyAccessRef,
			legacyRefreshRef,
			nestedLegacyAccessRef,
			nestedLegacyRefreshRef,
		}
		for _, ref := range append(legacyTokenRefs, preservedRefs...) {
			if _, err := legacyDB.ExecContext(
				ctx,
				`INSERT INTO vault_secrets (ref, kind, encrypted_value, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?)`,
				ref, "mcp-oauth", "encrypted", "2026-07-13T12:00:00Z", "2026-07-13T12:00:00Z",
			); err != nil {
				closeErr := legacyDB.Close()
				t.Fatalf("seed vault ref %q error = %v; close error = %v", ref, err, closeErr)
			}
		}
		if err := legacyDB.Close(); err != nil {
			t.Fatalf("Close(legacy) error = %v", err)
		}

		migrated, err := openGlobalMigrationUpgrade(t, databasePath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(full-stream migration) error = %v", err)
		}
		ctx = testutil.Context(t)
		migratedStatus, err := store.Status(ctx, migrated.db, MigrationStream())
		if err != nil {
			closeErr := migrated.Close(ctx)
			t.Fatalf("Status(global after migration) error = %v; close error = %v", err, closeErr)
		}
		assertCompleteMigrationStream(t, migratedStatus, MigrationStream())
		assertMCPAuthRowCount(ctx, t, migrated.db, 0)
		for _, ref := range legacyTokenRefs {
			assertVaultRefPresence(ctx, t, migrated.db, ref, false)
		}
		for _, ref := range preservedRefs {
			assertVaultRefPresence(ctx, t, migrated.db, ref, true)
		}
		if err := migrated.Close(ctx); err != nil {
			t.Fatalf("Close(migrated) error = %v", err)
		}

		reopened, err := OpenGlobalDB(testutil.Context(t), databasePath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen v3) error = %v", err)
		}
		ctx = testutil.Context(t)
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		reopenedStatus, err := store.Status(ctx, reopened.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(reopened v3) error = %v", err)
		}
		if reopenedStatus != migratedStatus {
			t.Fatalf("Status(reopened v3) = %#v, want %#v", reopenedStatus, migratedStatus)
		}
		assertMCPAuthRowCount(ctx, t, reopened.db, 0)
		for _, ref := range preservedRefs {
			assertVaultRefPresence(ctx, t, reopened.db, ref, true)
		}
	})

	t.Run("Should preserve durable token rows while marking pre-fingerprint credentials unbound", func(t *testing.T) {
		t.Parallel()

		databasePath := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
		previousDB, err := sql.Open(sqliteDriverName, databasePath)
		if err != nil {
			t.Fatalf("sql.Open(previous) error = %v", err)
		}
		prefix := globalMigrationPrefix(
			t,
			"00001_baseline.sql",
			"00002_schema.sql",
			"00003_schema.sql",
			"00004_schema.sql",
			"00005_schema.sql",
		)
		if err := applyGlobalMigrationPrefix(t, previousDB, prefix); err != nil {
			closeErr := previousDB.Close()
			t.Fatalf("Apply(global v5) error = %v; close error = %v", err, closeErr)
		}
		ctx := testutil.Context(t)
		if _, err := previousDB.ExecContext(
			ctx,
			`INSERT INTO mcp_auth_tokens (
				scope, workspace_id, server_name, issuer, client_id, scopes_json,
				access_token_ref, refresh_token_ref, token_type, expires_at, obtained_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
			"global",
			"",
			"linear",
			"https://issuer.example",
			"client-id",
			`["read"]`,
			"vault:mcp/global/linear/oauth/access-token",
			"",
			"Bearer",
			"2026-07-16T12:00:00Z",
			"2026-07-16T12:00:00Z",
		); err != nil {
			closeErr := previousDB.Close()
			t.Fatalf("seed global v5 token row error = %v; close error = %v", err, closeErr)
		}
		if _, err := previousDB.ExecContext(
			ctx,
			`INSERT INTO vault_secrets (ref, kind, encrypted_value, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			"vault:mcp/global/linear/oauth/access-token",
			"mcp-oauth",
			"encrypted",
			"2026-07-16T12:00:00Z",
			"2026-07-16T12:00:00Z",
		); err != nil {
			closeErr := previousDB.Close()
			t.Fatalf("seed global v5 token secret error = %v; close error = %v", err, closeErr)
		}
		if err := previousDB.Close(); err != nil {
			t.Fatalf("Close(previous) error = %v", err)
		}

		migrated, err := openGlobalMigrationUpgrade(t, databasePath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(migrate latest) error = %v", err)
		}
		ctx = testutil.Context(t)
		assertUnboundMCPAuthMigrationRow(ctx, t, migrated.db)
		status, err := store.Status(ctx, migrated.db, MigrationStream())
		if err != nil {
			closeErr := migrated.Close(ctx)
			t.Fatalf("Status(global latest) error = %v; close error = %v", err, closeErr)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		if err := migrated.Close(ctx); err != nil {
			t.Fatalf("Close(migrated) error = %v", err)
		}

		reopened, err := OpenGlobalDB(testutil.Context(t), databasePath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen latest) error = %v", err)
		}
		ctx = testutil.Context(t)
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		assertUnboundMCPAuthMigrationRow(ctx, t, reopened.db)
	})

	t.Run("Should preserve DCR registration rows when adding token endpoint auth method", func(t *testing.T) {
		t.Parallel()

		databasePath := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
		previousDB, err := sql.Open(sqliteDriverName, databasePath)
		if err != nil {
			t.Fatalf("sql.Open(previous) error = %v", err)
		}
		prefix := globalMigrationPrefixBefore(t, "00033_schema.sql")
		if err := applyGlobalMigrationPrefix(t, previousDB, prefix); err != nil {
			closeErr := previousDB.Close()
			t.Fatalf("Apply(global v32) error = %v; close error = %v", err, closeErr)
		}
		ctx := testutil.Context(t)
		if _, err := previousDB.ExecContext(
			ctx,
			`INSERT INTO mcp_oauth_registrations (
				scope, workspace_id, server_name, definition_fingerprint, resource_url,
				issuer, client_id, redirect_uri, scopes_json, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"global",
			"",
			"linear",
			testMCPDefinitionFingerprint,
			"https://mcp.example/resource",
			"https://issuer.example",
			"dcr-client",
			"http://127.0.0.1:8787/callback",
			`["read"]`,
			"2026-07-30T15:00:00Z",
		); err != nil {
			closeErr := previousDB.Close()
			t.Fatalf("seed global v32 registration error = %v; close error = %v", err, closeErr)
		}
		if err := previousDB.Close(); err != nil {
			t.Fatalf("Close(previous) error = %v", err)
		}

		migrated, err := openGlobalMigrationUpgrade(t, databasePath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(migrate v33) error = %v", err)
		}
		ctx = testutil.Context(t)
		stored, err := migrated.GetMCPAuthRegistration(ctx, globalMCPAuthTarget("linear"))
		if err != nil {
			closeErr := migrated.Close(ctx)
			t.Fatalf("GetMCPAuthRegistration(migrated) error = %v; close error = %v", err, closeErr)
		}
		if stored.ClientID != "dcr-client" || stored.TokenEndpointAuthMethod != "" {
			closeErr := migrated.Close(ctx)
			t.Fatalf(
				"migrated registration = %#v, want preserved client and empty method; close error = %v",
				stored,
				closeErr,
			)
		}
		if err := migrated.Close(ctx); err != nil {
			t.Fatalf("Close(migrated) error = %v", err)
		}

		reopened, err := OpenGlobalDB(ctx, databasePath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen v33) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		ctx = testutil.Context(t)
		stored, err = reopened.GetMCPAuthRegistration(ctx, globalMCPAuthTarget("linear"))
		if err != nil {
			t.Fatalf("GetMCPAuthRegistration(reopened) error = %v", err)
		}
		if stored.ClientID != "dcr-client" || stored.TokenEndpointAuthMethod != "" {
			t.Fatalf("reopened registration = %#v, want preserved client and empty method", stored)
		}
	})
}

func assertUnboundMCPAuthMigrationRow(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	var fingerprint, accessRef string
	if err := db.QueryRowContext(
		ctx,
		`SELECT definition_fingerprint, access_token_ref
		 FROM mcp_auth_tokens
		 WHERE scope = 'global' AND workspace_id = '' AND server_name = 'linear'`,
	).Scan(&fingerprint, &accessRef); err != nil {
		t.Fatalf("query migrated MCP auth token error = %v", err)
	}
	if fingerprint != "" {
		t.Fatalf("migrated definition fingerprint = %q, want unbound empty value", fingerprint)
	}
	if accessRef != "vault:mcp/global/linear/oauth/access-token" {
		t.Fatalf("migrated access token ref = %q, want preserved ref", accessRef)
	}
	assertVaultRefPresence(ctx, t, db, accessRef, true)
}

func globalMigrationPrefix(t *testing.T, names ...string) store.MigrationStream {
	t.Helper()
	fullStream := MigrationStream()
	memoryDirectory := &atlasmigrate.MemDir{}
	files := fstest.MapFS{}
	for _, name := range names {
		contents, err := fs.ReadFile(fullStream.FS, fullStream.Dir+"/"+name)
		if err != nil {
			t.Fatalf("read global migration %q: %v", name, err)
		}
		if err := memoryDirectory.WriteFile(name, contents); err != nil {
			t.Fatalf("write in-memory global migration %q: %v", name, err)
		}
		files[fullStream.Dir+"/"+name] = &fstest.MapFile{Data: append([]byte(nil), contents...)}
	}
	checksum, err := memoryDirectory.Checksum()
	if err != nil {
		t.Fatalf("checksum global migration prefix: %v", err)
	}
	checksumBytes, err := checksum.MarshalText()
	if err != nil {
		t.Fatalf("marshal global migration prefix checksum: %v", err)
	}
	files[fullStream.Dir+"/"+atlasmigrate.HashFileName] = &fstest.MapFile{Data: checksumBytes}
	fullStream.FS = files
	return fullStream
}

func globalMigrationPrefixBefore(t *testing.T, excludedMigration string) store.MigrationStream {
	t.Helper()
	fullStream := MigrationStream()
	entries, err := fs.ReadDir(fullStream.FS, fullStream.Dir)
	if err != nil {
		t.Fatalf("read global migration directory: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") || entry.Name() >= excludedMigration {
			continue
		}
		names = append(names, entry.Name())
	}
	return globalMigrationPrefix(t, names...)
}

func assertMCPAuthRowCount(ctx context.Context, t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var got int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mcp_auth_tokens`).Scan(&got); err != nil {
		t.Fatalf("count mcp_auth_tokens error = %v", err)
	}
	if got != want {
		t.Fatalf("mcp_auth_tokens count = %d, want %d", got, want)
	}
}

func assertVaultRefPresence(ctx context.Context, t *testing.T, db *sql.DB, ref string, want bool) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vault_secrets WHERE ref = ?`, ref).
		Scan(&count); err != nil {
		t.Fatalf("count vault ref %q error = %v", ref, err)
	}
	if got := count == 1; got != want {
		t.Fatalf("vault ref %q presence = %t, want %t", ref, got, want)
	}
}

func globalMCPAuthTarget(serverName string) mcpauth.Target {
	return mcpauth.Target{Scope: mcpauth.ScopeUser, ServerName: serverName}
}

func mcpOAuthRegistrationRecord(
	t *testing.T,
	target mcpauth.Target,
	issuedAt time.Time,
) mcpauth.ClientRegistration {
	t.Helper()
	return mcpauth.ClientRegistration{
		Target:                  target,
		DefinitionFingerprint:   testMCPDefinitionFingerprint,
		ResourceURL:             "https://mcp.example/resource",
		Issuer:                  "https://issuer.example",
		ClientID:                "dcr-client",
		TokenEndpointAuthMethod: "client_secret_basic",
		RegistrationClientURI:   "https://issuer.example/register/dcr-client",
		ClientIDIssuedAt:        issuedAt,
		ClientSecretExpiresAt:   issuedAt.Add(time.Hour),
		RedirectURL:             "http://127.0.0.1:8787/callback",
		Scopes:                  []string{"read", "write"},
		UpdatedAt:               issuedAt,
	}
}

func mcpOAuthRegistrationSecrets() mcpauth.RegistrationSecrets {
	return mcpauth.RegistrationSecrets{
		ClientSecret:            "client-secret",
		RegistrationAccessToken: "registration-access-token",
	}
}

func mcpAuthorizationTokenRecord(target mcpauth.Target, issuedAt time.Time) mcpauth.TokenRecord {
	return mcpauth.TokenRecord{
		Target:                target,
		DefinitionFingerprint: testMCPDefinitionFingerprint,
		Issuer:                "https://issuer.example",
		ClientID:              "dcr-client",
		AccessToken:           "access-token",
		RefreshToken:          "refresh-token",
		ObtainedAt:            issuedAt,
		UpdatedAt:             issuedAt,
	}
}

func mcpAuthorizationSecretRefs(
	t *testing.T,
	target mcpauth.Target,
	registration mcpauth.ClientRegistration,
) []string {
	t.Helper()
	prefix, err := vault.MCPSecretOwnerPrefix(string(target.Scope), target.WorkspaceID, target.ServerName)
	if err != nil {
		t.Fatalf("MCPSecretOwnerPrefix(%#v) error = %v", target, err)
	}
	return []string{
		prefix + "oauth/access-token",
		prefix + "oauth/refresh-token",
		registration.ClientSecretRef,
		registration.RegistrationAccessTokenRef,
	}
}
