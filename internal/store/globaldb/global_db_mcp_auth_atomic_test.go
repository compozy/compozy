package globaldb

import (
	"strings"
	"testing"
	"time"

	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/testutil"
)

func TestMCPAuthTokenStoreAtomicityContract(t *testing.T) {
	t.Parallel()

	t.Run("Should rollback vault writes when metadata upsert fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		initial := mcpAuthTokenAtomicityRecord("linear", "old-access", "old-refresh")
		if err := db.SaveMCPAuthToken(ctx, initial); err != nil {
			t.Fatalf("SaveMCPAuthToken(initial) error = %v", err)
		}
		if _, err := db.db.ExecContext(
			ctx,
			"CREATE TRIGGER fail_mcp_auth_token_update "+
				"BEFORE UPDATE ON mcp_auth_tokens "+
				"BEGIN "+
				"SELECT RAISE(FAIL, 'forced mcp metadata update failure'); "+
				"END;",
		); err != nil {
			t.Fatalf("create failure trigger error = %v", err)
		}

		err := db.SaveMCPAuthToken(ctx, mcpAuthTokenAtomicityRecord("linear", "new-access", "new-refresh"))
		if err == nil {
			t.Fatal("SaveMCPAuthToken(update) error = nil, want injected metadata failure")
		}
		if !strings.Contains(err.Error(), "forced mcp metadata update failure") {
			t.Fatalf("SaveMCPAuthToken(update) error = %v, want injected metadata failure", err)
		}

		preserved, err := db.GetMCPAuthToken(ctx, userMCPAuthTarget("linear"))
		if err != nil {
			t.Fatalf("GetMCPAuthToken() error = %v", err)
		}
		if preserved.AccessToken != initial.AccessToken || preserved.RefreshToken != initial.RefreshToken {
			t.Fatalf(
				"token after failed save = access %q refresh %q, want access %q refresh %q",
				preserved.AccessToken,
				preserved.RefreshToken,
				initial.AccessToken,
				initial.RefreshToken,
			)
		}
	})

	t.Run("Should rollback metadata delete when vault cleanup fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		initial := mcpAuthTokenAtomicityRecord("linear", "access-token", "refresh-token")
		if err := db.SaveMCPAuthToken(ctx, initial); err != nil {
			t.Fatalf("SaveMCPAuthToken(initial) error = %v", err)
		}
		if _, err := db.db.ExecContext(
			ctx,
			"CREATE TRIGGER fail_mcp_vault_secret_delete "+
				"BEFORE DELETE ON vault_secrets "+
				"WHEN OLD.ref LIKE 'vault:mcp/user/linear/oauth/%' "+
				"BEGIN "+
				"SELECT RAISE(FAIL, 'forced vault secret delete failure'); "+
				"END;",
		); err != nil {
			t.Fatalf("create failure trigger error = %v", err)
		}

		err := db.DeleteMCPAuthToken(ctx, userMCPAuthTarget("linear"))
		if err == nil {
			t.Fatal("DeleteMCPAuthToken() error = nil, want injected vault delete failure")
		}
		if !strings.Contains(err.Error(), "forced vault secret delete failure") {
			t.Fatalf("DeleteMCPAuthToken() error = %v, want injected vault delete failure", err)
		}

		preserved, err := db.GetMCPAuthToken(ctx, userMCPAuthTarget("linear"))
		if err != nil {
			t.Fatalf("GetMCPAuthToken() error = %v", err)
		}
		if preserved.AccessToken != initial.AccessToken || preserved.RefreshToken != initial.RefreshToken {
			t.Fatalf(
				"token after failed delete = access %q refresh %q, want access %q refresh %q",
				preserved.AccessToken,
				preserved.RefreshToken,
				initial.AccessToken,
				initial.RefreshToken,
			)
		}
	})
}

func TestMCPOAuthRegistrationStoreAtomicityContract(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back registration metadata update when the target row rejects it", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		registration := mcpOAuthRegistrationRecord(
			t,
			userMCPAuthTarget("linear"),
			time.Date(2026, 7, 30, 16, 0, 0, 0, time.UTC),
		)
		saved, err := db.SaveMCPAuthRegistration(ctx, registration, mcpOAuthRegistrationSecrets())
		if err != nil {
			t.Fatalf("SaveMCPAuthRegistration(initial) error = %v", err)
		}
		if _, err := db.db.ExecContext(
			ctx,
			"CREATE TRIGGER fail_mcp_oauth_registration_update "+
				"BEFORE UPDATE ON mcp_oauth_registrations "+
				"BEGIN "+
				"SELECT RAISE(FAIL, 'forced MCP registration update failure'); "+
				"END;",
		); err != nil {
			t.Fatalf("create failure trigger error = %v", err)
		}
		replacement := registration
		replacement.ClientID = "replacement-client"
		replacementSecrets := mcpauth.RegistrationSecrets{
			ClientSecret:            "replacement-client-secret",
			RegistrationAccessToken: "replacement-registration-access-token",
		}
		if _, err := db.SaveMCPAuthRegistration(ctx, replacement, replacementSecrets); err == nil {
			t.Fatal("SaveMCPAuthRegistration(update) error = nil, want injected metadata failure")
		} else if !strings.Contains(err.Error(), "forced MCP registration update failure") {
			t.Fatalf("SaveMCPAuthRegistration(update) error = %v, want injected metadata failure", err)
		}
		preserved, err := db.GetMCPAuthRegistration(ctx, registration.Target)
		if err != nil {
			t.Fatalf("GetMCPAuthRegistration() error = %v", err)
		}
		if preserved.ClientID != saved.ClientID {
			t.Fatalf("registration after failed save = %#v, want original client ID", preserved)
		}
		service, err := db.vaultService()
		if err != nil {
			t.Fatalf("vaultService() error = %v", err)
		}
		clientSecret, err := service.ResolveRef(ctx, saved.ClientSecretRef)
		if err != nil {
			t.Fatalf("ResolveRef(client secret) error = %v", err)
		}
		if clientSecret != "client-secret" {
			t.Fatalf("client secret after failed save = %q, want original secret", clientSecret)
		}
		registrationToken, err := service.ResolveRef(ctx, saved.RegistrationAccessTokenRef)
		if err != nil {
			t.Fatalf("ResolveRef(registration access token) error = %v", err)
		}
		if registrationToken != "registration-access-token" {
			t.Fatalf(
				"registration access token after failed save = %q, want original token",
				registrationToken,
			)
		}
	})

	t.Run("Should roll back registration deletion when DCR secret cleanup fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		registration := mcpOAuthRegistrationRecord(
			t,
			userMCPAuthTarget("linear"),
			time.Date(2026, 7, 30, 16, 30, 0, 0, time.UTC),
		)
		saved, err := db.SaveMCPAuthRegistration(ctx, registration, mcpOAuthRegistrationSecrets())
		if err != nil {
			t.Fatalf("SaveMCPAuthRegistration(initial) error = %v", err)
		}
		if _, err := db.db.ExecContext(
			ctx,
			"CREATE TRIGGER fail_mcp_oauth_registration_secret_delete "+
				"BEFORE DELETE ON vault_secrets "+
				"WHEN OLD.kind = 'mcp_oauth_dcr_client_secret' "+
				"BEGIN "+
				"SELECT RAISE(FAIL, 'forced MCP registration secret cleanup failure'); "+
				"END;",
		); err != nil {
			t.Fatalf("create failure trigger error = %v", err)
		}
		if err := db.DeleteMCPAuthRegistration(ctx, saved.Target); err == nil {
			t.Fatal("DeleteMCPAuthRegistration() error = nil, want injected vault cleanup failure")
		} else if !strings.Contains(err.Error(), "forced MCP registration secret cleanup failure") {
			t.Fatalf("DeleteMCPAuthRegistration() error = %v, want injected vault cleanup failure", err)
		}
		preserved, err := db.GetMCPAuthRegistration(ctx, saved.Target)
		if err != nil {
			t.Fatalf("GetMCPAuthRegistration() after failed delete error = %v", err)
		}
		if preserved.ClientID != saved.ClientID {
			t.Fatalf("registration after failed delete = %#v, want original client ID", preserved)
		}
	})

	t.Run("Should roll back full authorization cleanup when token secret deletion fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		issuedAt := time.Date(2026, 7, 30, 17, 0, 0, 0, time.UTC)
		target := userMCPAuthTarget("linear")
		if err := db.SaveMCPAuthToken(ctx, mcpAuthorizationTokenRecord(target, issuedAt)); err != nil {
			t.Fatalf("SaveMCPAuthToken() error = %v", err)
		}
		if _, err := db.SaveMCPAuthRegistration(
			ctx,
			mcpOAuthRegistrationRecord(t, target, issuedAt),
			mcpOAuthRegistrationSecrets(),
		); err != nil {
			t.Fatalf("SaveMCPAuthRegistration() error = %v", err)
		}
		if _, err := db.db.ExecContext(
			ctx,
			"CREATE TRIGGER fail_mcp_authorization_access_token_delete "+
				"BEFORE DELETE ON vault_secrets "+
				"WHEN OLD.kind = 'mcp_oauth_access_token' "+
				"BEGIN "+
				"SELECT RAISE(FAIL, 'forced authorization cleanup failure'); "+
				"END;",
		); err != nil {
			t.Fatalf("create failure trigger error = %v", err)
		}

		if err := db.DeleteMCPAuthorizationState(ctx, target); err == nil {
			t.Fatal("DeleteMCPAuthorizationState() error = nil, want injected vault cleanup failure")
		} else if !strings.Contains(err.Error(), "forced authorization cleanup failure") {
			t.Fatalf("DeleteMCPAuthorizationState() error = %v, want injected vault cleanup failure", err)
		}
		if _, err := db.GetMCPAuthToken(ctx, target); err != nil {
			t.Fatalf("GetMCPAuthToken() after failed authorization cleanup error = %v", err)
		}
		if _, err := db.GetMCPAuthRegistration(ctx, target); err != nil {
			t.Fatalf("GetMCPAuthRegistration() after failed authorization cleanup error = %v", err)
		}
	})
}

func mcpAuthTokenAtomicityRecord(serverName string, accessToken string, refreshToken string) mcpauth.TokenRecord {
	issuedAt := time.Date(2026, 5, 17, 16, 0, 0, 0, time.UTC)
	return mcpauth.TokenRecord{
		Target:                userMCPAuthTarget(serverName),
		DefinitionFingerprint: testMCPDefinitionFingerprint,
		Issuer:                "https://issuer.example",
		ClientID:              "client",
		Scopes:                []string{"read", "write"},
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		TokenType:             "Bearer",
		ExpiresAt:             issuedAt.Add(time.Hour),
		ObtainedAt:            issuedAt,
		UpdatedAt:             issuedAt,
	}
}
