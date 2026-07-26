package globaldb

import (
	"strings"
	"testing"
	"time"

	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/compozy/agh/internal/testutil"
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

		preserved, err := db.GetMCPAuthToken(ctx, globalMCPAuthTarget("linear"))
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
				"WHEN OLD.ref LIKE 'vault:mcp/global/linear/oauth/%' "+
				"BEGIN "+
				"SELECT RAISE(FAIL, 'forced vault secret delete failure'); "+
				"END;",
		); err != nil {
			t.Fatalf("create failure trigger error = %v", err)
		}

		err := db.DeleteMCPAuthToken(ctx, globalMCPAuthTarget("linear"))
		if err == nil {
			t.Fatal("DeleteMCPAuthToken() error = nil, want injected vault delete failure")
		}
		if !strings.Contains(err.Error(), "forced vault secret delete failure") {
			t.Fatalf("DeleteMCPAuthToken() error = %v, want injected vault delete failure", err)
		}

		preserved, err := db.GetMCPAuthToken(ctx, globalMCPAuthTarget("linear"))
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

func mcpAuthTokenAtomicityRecord(serverName string, accessToken string, refreshToken string) mcpauth.TokenRecord {
	issuedAt := time.Date(2026, 5, 17, 16, 0, 0, 0, time.UTC)
	return mcpauth.TokenRecord{
		Target:                globalMCPAuthTarget(serverName),
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
