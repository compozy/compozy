package auth

import (
	"strings"
	"testing"
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
