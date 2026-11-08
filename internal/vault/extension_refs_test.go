package vault

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestExtensionSecretRefUsesInstanceQualifiedCollisionSafeNames(t *testing.T) {
	t.Parallel()

	t.Run("Should keep global and workspace refs separate", func(t *testing.T) {
		t.Parallel()

		if got, want := ExtensionSecretRef("my-ext", "", "API_KEY"),
			"vault:extensions/global/my-ext/env/API_KEY"; got != want {
			t.Fatalf("ExtensionSecretRef(global) = %q, want %q", got, want)
		}
		if got, want := ExtensionSecretRef("my-ext", "ws-1", "API_KEY"),
			"vault:extensions/ws/ws-1/my-ext/env/API_KEY"; got != want {
			t.Fatalf("ExtensionSecretRef(workspace) = %q, want %q", got, want)
		}
	})

	t.Run("Should encode unsafe segments without colliding with reserved literals", func(t *testing.T) {
		t.Parallel()

		unsafe := "My Ext!"
		wantSegment := "encoded-" + hex.EncodeToString([]byte(unsafe))
		want := "vault:extensions/ws/" +
			"encoded-" + hex.EncodeToString([]byte("Workspace One")) + "/" +
			wantSegment + "/env/" +
			"encoded-" + hex.EncodeToString([]byte("API KEY"))
		if got := ExtensionSecretRef(unsafe, "Workspace One", "API KEY"); got != want {
			t.Fatalf("ExtensionSecretRef() = %q, want %q", got, want)
		}
		if ExtensionSecretRef(wantSegment, "", "API_KEY") == ExtensionSecretRef(unsafe, "", "API_KEY") {
			t.Fatalf("unsafe extension collided with reserved literal %q", wantSegment)
		}
	})
	// Invariant: canonical refs resolve to exactly the owner used by package lifecycle locks.
	// Owner: Vault ref grammar; canonical suite: TestExtensionSecretRefUsesInstanceQualifiedCollisionSafeNames.
	t.Run("Should recover canonical owners without accepting ambiguous encodings", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"kit", "encoded-literal", "My Extension!"} {
			for _, workspace := range []string{"", "Workspace One"} {
				ref := ExtensionProfileSecretRef(name, "profile-one", workspace, "TOKEN")
				for _, persisted := range []string{ref, strings.Replace(ref, "/global/", "/user/", 1)} {
					if got, err := ExtensionSecretRefOwner(persisted); err != nil || got != name {
						t.Fatalf("owner = %q, %v, want %q", got, err, name)
					}
					if err := ValidateExtensionSecretRefOwner(persisted, name); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		for _, ref := range []string{"vault:providers/kit/TOKEN", "vault:extensions/kit/TOKEN", "vault:extensions/global/encoded-zz/env/TOKEN", "vault:extensions/global/encoded-6b6974/env/TOKEN"} {
			if owner, err := ExtensionSecretRefOwner(ref); err == nil {
				t.Fatalf("ambiguous owner accepted: %q", owner)
			}
		}
	})
}

func TestExtensionOAuthClientSecretOwnership(t *testing.T) {
	t.Parallel()
	t.Run("Should accept only the owning extension client secret reference", func(t *testing.T) {
		t.Parallel()
		ref := ExtensionSecretRef("client", "", "CLIENT_SECRET")
		for _, owner := range []string{"extension:client", "extension:other", ""} {
			target := MCPSecretTarget{Owner: owner, Scope: MCPUserScope, ServerName: "remote"}
			got, err := NormalizeMCPClientSecretRef(ref, target)
			if owner == "extension:client" {
				if err != nil || got != ref {
					t.Fatalf("owned ref = %q, %v", got, err)
				}
			} else if err == nil {
				t.Fatalf("accepted ref for owner %q", owner)
			}
		}
	})
}
