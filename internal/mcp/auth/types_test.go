package auth

import (
	"context"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/vault"
)

func TestTargetKeyRejectsIdentitySeparatorBytes(t *testing.T) {
	t.Parallel()
	t.Run("Should isolate equal server names by owner and normalize omitted owners to manual", func(t *testing.T) {
		t.Parallel()
		manual := Target{Scope: ScopeUser, ServerName: "github"}
		manualKey, err := manual.Key()
		if err != nil {
			t.Fatal(err)
		}
		explicit := manual
		explicit.Owner = "manual"
		explicitKey, err := explicit.Key()
		if err != nil || explicitKey != manualKey {
			t.Fatalf("legacy owner normalization changed identity: %v", err)
		}
		extension := manual
		extension.Owner = "extension:github"
		extensionKey, err := extension.Key()
		if err != nil || extensionKey == manualKey {
			t.Fatalf("extension and manual credentials share a key: %v", err)
		}
		if extension.VaultTarget().Owner != extension.Owner {
			t.Fatal("Vault boundary lost the extension owner")
		}
		for _, owner := range []string{"extension:", "extension:bad/name", "extension:bad\x00name", "other"} {
			extension.Owner = owner
			if _, err := extension.Key(); err == nil {
				t.Fatalf("invalid owner %q was accepted", owner)
			}
		}
	})

	t.Run("Should reject NUL in workspace and server identity fields", func(t *testing.T) {
		t.Parallel()

		for _, target := range []Target{
			{Scope: ScopeUser, ServerName: "linear\x00workspace"},
			{Scope: ScopeWorkspace, WorkspaceID: "workspace\x00linear", ServerName: "linear"},
		} {
			if _, err := target.Key(); err == nil || !strings.Contains(err.Error(), "NUL") {
				t.Fatalf("Target.Key(%#v) error = %v, want NUL rejection", target, err)
			}
		}
	})
}

func TestTargetValidateWorkspaceProfileIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should accept the canonical workspace profile composite", func(t *testing.T) {
		t.Parallel()
		if err := (Target{
			Scope:       ScopeWorkspaceProfile,
			WorkspaceID: "workspace-1@pf:marketing",
			ServerName:  "linear",
		}).Validate(); err != nil {
			t.Fatalf("Target.Validate() error = %v", err)
		}
	})

	t.Run("Should reject an unbound workspace profile identity", func(t *testing.T) {
		t.Parallel()
		for _, workspaceID := range []string{"workspace-1", "workspace-1@pf:", "@pf:marketing", "workspace-1@pf:bad name"} {
			t.Run(workspaceID, func(t *testing.T) {
				t.Parallel()
				if err := (Target{Scope: ScopeWorkspaceProfile, WorkspaceID: workspaceID, ServerName: "linear"}).Validate(); err == nil {
					t.Fatalf("Target.Validate(%q) error = nil, want invalid composite", workspaceID)
				}
			})
		}
	})
}

func TestServerConfigValidate(t *testing.T) {
	t.Parallel()
	t.Run("Should authorize configured client secrets before resolving them", func(t *testing.T) {
		t.Parallel()
		for _, target := range []Target{
			{Scope: ScopeUser, ServerName: "linear"},
			{Scope: ScopeProfile, WorkspaceID: "marketing", ServerName: "linear"},
			{Scope: ScopeWorkspace, WorkspaceID: "workspace-a", ServerName: "linear"},
			{Scope: ScopeWorkspaceProfile, WorkspaceID: "workspace-a@pf:marketing", ServerName: "linear"},
		} {
			for _, owner := range []string{"manual", "extension:linear"} {
				target.Owner = owner
				prefix, err := vault.MCPSecretOwnerPrefix(target.VaultTarget())
				if err != nil {
					t.Fatal(err)
				}
				for _, ref := range []string{
					prefix + "oauth/client-secret", prefix + "configured-secret",
					"vault:mcp/shared/client-secret", "env:MCP_CLIENT_SECRET",
				} {
					cfg, err := ServerConfigFromMCP(
						t.Context(),
						target,
						clientSecretConfig(t, ref),
						func(_ context.Context, got string) (string, error) {
							if got != ref {
								t.Fatalf("resolved ref = %q, want %q", got, ref)
							}
							return "resolved-secret", nil
						},
					)
					if err != nil || cfg.ClientSecret != "resolved-secret" || cfg.ClientSecretRef != ref {
						t.Fatalf("allowed target %#v ref %q = %#v, %v", target, ref, cfg, err)
					}
				}
				otherTarget := target
				otherTarget.Owner = "manual"
				if owner == "manual" {
					otherTarget.Owner = "extension:linear"
				}
				otherPrefix, err := vault.MCPSecretOwnerPrefix(otherTarget.VaultTarget())
				if err != nil {
					t.Fatal(err)
				}
				for _, ref := range []string{
					otherPrefix + "oauth/client-secret",
					prefix + "oauth/access-token", prefix + "oauth/refresh-token",
					prefix + "oauth/dcr-client-secret", prefix + "oauth/registration-access-token",
					prefix + "OAuth/client-secret", "vault:mcp/user/foreign/oauth/client-secret",
					"vault:mcp/ext/foreign/user/linear/oauth/client-secret",
					"vault:mcp/profile/sales/linear/oauth/client-secret",
					"vault:mcp/ws/workspace-b/linear/oauth/client-secret",
				} {
					for _, resolver := range []SecretRefResolver{nil, func(context.Context, string) (string, error) {
						t.Fatal("unauthorized secret reached resolver")
						return "", nil
					}} {
						if _, err := ServerConfigFromMCP(
							t.Context(),
							target,
							clientSecretConfig(t, ref),
							resolver,
						); err == nil {
							t.Fatalf("target %#v accepted foreign or managed secret %q", target, ref)
						}
					}
				}
			}
		}
	})
	t.Run("Should retain released manual client secret references within their owner", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct{ ref, resolved string }{
			{"vault:mcp/global/linear/oauth/client-secret", "vault:mcp/user/linear/oauth/client-secret"},
			{"vault:mcp/linear/oauth/client-secret", "vault:mcp/linear/oauth/client-secret"},
		} {
			for _, target := range []Target{
				{Scope: ScopeUser, ServerName: "linear"},
				{Owner: "extension:linear", Scope: ScopeUser, ServerName: "linear"},
				{Scope: ScopeWorkspace, WorkspaceID: "workspace-a", ServerName: "linear"},
			} {
				cfg, err := ServerConfigFromMCP(
					t.Context(),
					target,
					clientSecretConfig(t, tc.ref),
					func(_ context.Context, ref string) (string, error) {
						if target.Owner != "" || target.Scope != ScopeUser {
							t.Fatal("legacy secret escaped its manual user owner")
						}
						if ref != tc.resolved {
							t.Fatalf("resolved = %q, want %q", ref, tc.resolved)
						}
						return "preserved-secret", nil
					},
				)
				if target.Owner == "" && target.Scope == ScopeUser {
					if err != nil || cfg.ClientSecret != "preserved-secret" {
						t.Fatalf("manual config = %#v, %v", cfg, err)
					}
				} else if err == nil {
					t.Fatal("foreign target accepted legacy secret")
				}
			}
		}
	})
	t.Run("Should reject unsupported auth types", func(t *testing.T) {
		t.Parallel()
		cfg := ServerConfig{
			Target: Target{Scope: ScopeUser, ServerName: "fixture"},
			Type:   "unsupported",
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("ServerConfig.Validate() error = nil, want unsupported auth type rejection")
		}
	})
}

func clientSecretConfig(t *testing.T, ref string) compozyconfig.MCPServer {
	t.Helper()
	return compozyconfig.MCPServer{
		Name:      "linear",
		Transport: compozyconfig.MCPServerTransportHTTP,
		URL:       "https://mcp.example.test/mcp",
		Auth: compozyconfig.MCPAuthConfig{
			IssuerURL:       "https://issuer.example.test",
			Registration:    "pre_registered",
			ClientID:        "client",
			ClientSecretRef: ref,
		},
	}
}
