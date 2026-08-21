package vault

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestProfileSecretRefsEnforceGrammarAndOwnership(t *testing.T) {
	t.Parallel()

	t.Run("Should parse profile provider and extension refs", func(t *testing.T) {
		t.Parallel()

		for _, ref := range []string{
			"vault:profiles/marketing/providers/openai/api_key",
			"vault:profiles/marketing/extensions/growth/token",
		} {
			parsed, err := ParseProfileSecretRef(ref)
			if err != nil {
				t.Fatalf("ParseProfileSecretRef(%q) error = %v", ref, err)
			}
			if parsed.ProfileName != "marketing" {
				t.Fatalf("ParseProfileSecretRef(%q).ProfileName = %q, want marketing", ref, parsed.ProfileName)
			}
			if err := ValidateSecretRef(ref); err != nil {
				t.Fatalf("ValidateSecretRef(%q) error = %v", ref, err)
			}
		}
	})

	t.Run("Should reject another profile owner", func(t *testing.T) {
		t.Parallel()

		err := ValidateProfileSecretRefAccess(
			"vault:profiles/sales/providers/openai/api_key",
			"marketing",
		)
		if err == nil || !strings.Contains(err.Error(), `profile "marketing" cannot access`) {
			t.Fatalf("ValidateProfileSecretRefAccess() error = %v, want owner rejection", err)
		}
	})

	t.Run("Should reject environment refs with the stable profile code", func(t *testing.T) {
		t.Parallel()

		err := ValidateProfileScopedRef("env:OPENAI_API_KEY", "marketing")
		if !errors.Is(err, ErrProfileSecretEnvForbidden) {
			t.Fatalf("ValidateProfileScopedRef() error = %v, want ErrProfileSecretEnvForbidden", err)
		}
		var typed *ProfileSecretError
		if !errors.As(err, &typed) || typed.Code != "profile_secret_env_forbidden" {
			t.Fatalf("ValidateProfileScopedRef() error = %#v, want stable code", err)
		}
	})

	t.Run("Should reject generic paths inside the profiles namespace", func(t *testing.T) {
		t.Parallel()

		for _, ref := range []string{
			"vault:profiles/marketing/random/value",
			"vault:profiles/Marketing/providers/openai/api_key",
			"vault:profiles/marketing/providers/openai/nested/api_key",
		} {
			err := ValidateSecretRef(ref)
			if err == nil {
				t.Fatalf("ValidateSecretRef(%q) error = nil, want grammar rejection", ref)
			}
			if !errors.Is(err, ErrUnsupportedSecretRef) {
				t.Fatalf("ValidateSecretRef(%q) error = %v, want ErrUnsupportedSecretRef", ref, err)
			}
		}
	})
}

func TestSecretLikeEnvName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  string
		want bool
	}{
		{name: "Should reject token values", env: "GITHUB_TOKEN", want: true},
		{name: "Should reject secret values", env: "CLIENT_SECRET", want: true},
		{name: "Should reject API key values", env: "OPENAI_API_KEY", want: true},
		{name: "Should reject JWT private key values", env: "JWT_PRIVATE_KEY", want: true},
		{name: "Should reject SSH private key values", env: "SSH_PRIVATE_KEY", want: true},
		{name: "Should reject GitHub app private key values", env: "GITHUB_APP_PRIVATE_KEY", want: true},
		{name: "Should reject compact private key values", env: "SERVICE_PRIVATEKEY", want: true},
		{name: "Should allow token endpoint URLs", env: "COMPOZY_BRIDGE_LINEAR_TOKEN_URL", want: false},
		{name: "Should allow secret named path variables", env: "COMPOZY_SECRET_GUARD_HOST_CALL_PATH", want: false},
		{name: "Should allow credential file paths", env: "AWS_SHARED_CREDENTIALS_FILE", want: false},
		{name: "Should allow private key file paths", env: "GITHUB_APP_PRIVATE_KEY_FILE", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := SecretLikeEnvName(tc.env); got != tc.want {
				t.Fatalf("SecretLikeEnvName(%q) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

func TestValidateNonSecretEnvMapRejectsPrivateKeyNames(t *testing.T) {
	t.Parallel()

	t.Run("Should reject private key literals in non-secret env maps", func(t *testing.T) {
		t.Parallel()

		err := ValidateNonSecretEnvMap("skill.mcp_servers[0]", map[string]string{
			"GITHUB_APP_PRIVATE_KEY": "-----BEGIN PRIVATE KEY-----",
		})
		if err == nil {
			t.Fatal("ValidateNonSecretEnvMap() error = nil, want private key rejection")
		}
		if !strings.Contains(err.Error(), "GITHUB_APP_PRIVATE_KEY must move secret-like values to secret_env") {
			t.Fatalf("ValidateNonSecretEnvMap() error = %q, want private key guidance", err)
		}
	})
}

func TestMCPSecretOwnerPrefixIsCollisionSafeAcrossScopes(t *testing.T) {
	t.Parallel()

	t.Run("Should keep user profile and workspace MCP secret prefixes collision-safe", func(t *testing.T) {
		t.Parallel()

		workspaceID := "workspace/alpha"
		userPrefix, err := MCPSecretOwnerPrefix(MCPUserScope, "", "linear")
		if err != nil {
			t.Fatalf("MCPSecretOwnerPrefix(user) error = %v", err)
		}
		profilePrefix, err := MCPSecretOwnerPrefix(MCPProfileScope, "marketing", "linear")
		if err != nil {
			t.Fatalf("MCPSecretOwnerPrefix(profile) error = %v", err)
		}
		workspacePrefix, err := MCPSecretOwnerPrefix(MCPWorkspaceScope, workspaceID, "linear")
		if err != nil {
			t.Fatalf("MCPSecretOwnerPrefix(workspace) error = %v", err)
		}
		if userPrefix == workspacePrefix || userPrefix == profilePrefix || profilePrefix == workspacePrefix {
			t.Fatalf(
				"MCP owner prefixes collided: user=%q profile=%q workspace=%q",
				userPrefix,
				profilePrefix,
				workspacePrefix,
			)
		}
		if got, want := userPrefix, "vault:mcp/user/linear/"; got != want {
			t.Fatalf("user prefix = %q, want %q", got, want)
		}
		if got, want := profilePrefix, "vault:mcp/profile/marketing/linear/"; got != want {
			t.Fatalf("profile prefix = %q, want %q", got, want)
		}
		userDCRRefs, err := MCPDCRSecretRefsForTarget(MCPUserScope, "", "linear")
		if err != nil {
			t.Fatalf("MCPDCRSecretRefsForTarget(user) error = %v", err)
		}
		if got, want := userDCRRefs.ClientSecretRef, userPrefix+"oauth/dcr-client-secret"; got != want {
			t.Fatalf("user DCR client secret ref = %q, want %q", got, want)
		}
		if got, want := userDCRRefs.RegistrationAccessTokenRef, userPrefix+"oauth/registration-access-token"; got != want {
			t.Fatalf("user DCR registration token ref = %q, want %q", got, want)
		}
		segment, err := MCPOwnerSegment(workspaceID)
		if err != nil {
			t.Fatalf("MCPOwnerSegment() error = %v", err)
		}
		if got, want := workspacePrefix, "vault:mcp/ws/"+segment+"/linear/"; got != want {
			t.Fatalf("workspace prefix = %q, want %q", got, want)
		}
		reservedSegment, err := MCPOwnerSegment(segment)
		if err != nil {
			t.Fatalf("MCPOwnerSegment(reserved prefix) error = %v", err)
		}
		if reservedSegment == segment {
			t.Fatalf("encoded workspace ID collided with reserved literal: %q", segment)
		}
		if _, err := MCPSecretOwnerPrefix("global", "", "linear"); err == nil {
			t.Fatal("MCPSecretOwnerPrefix(global) error = nil, want obsolete scope rejection")
		} else if !strings.Contains(err.Error(), "unsupported MCP secret scope") {
			t.Fatalf("MCPSecretOwnerPrefix(global) error = %v, want unsupported scope diagnostic", err)
		}
	})

	t.Run("Should encode unsafe MCP server names without colliding with reserved literals", func(t *testing.T) {
		t.Parallel()

		serverName := "QA OAuth MCP"
		segment, err := MCPServerSegment(serverName)
		if err != nil {
			t.Fatalf("MCPServerSegment() error = %v", err)
		}
		wantSegment := "encoded-" + hex.EncodeToString([]byte(serverName))
		if segment != wantSegment {
			t.Fatalf("MCPServerSegment() = %q, want %q", segment, wantSegment)
		}
		prefix, err := MCPSecretOwnerPrefix(MCPUserScope, "", serverName)
		if err != nil {
			t.Fatalf("MCPSecretOwnerPrefix() error = %v", err)
		}
		if got, want := prefix, "vault:mcp/user/"+wantSegment+"/"; got != want {
			t.Fatalf("MCPSecretOwnerPrefix() = %q, want %q", got, want)
		}
		reservedSegment, err := MCPServerSegment(segment)
		if err != nil {
			t.Fatalf("MCPServerSegment(reserved prefix) error = %v", err)
		}
		if reservedSegment == segment {
			t.Fatalf("encoded server name collided with reserved literal: %q", segment)
		}
	})
}

func TestValidateMCPSecretRefAccessIsolatesOwners(t *testing.T) {
	t.Parallel()

	t.Run("Should allow owned and shared refs while rejecting another workspace", func(t *testing.T) {
		t.Parallel()

		workspaceA := "workspace-a"
		workspaceBPrefix, err := MCPSecretOwnerPrefix(MCPWorkspaceScope, "workspace-b", "linear")
		if err != nil {
			t.Fatalf("MCPSecretOwnerPrefix(workspace-b) error = %v", err)
		}
		workspaceAPrefix, err := MCPSecretOwnerPrefix(MCPWorkspaceScope, workspaceA, "linear")
		if err != nil {
			t.Fatalf("MCPSecretOwnerPrefix(workspace-a) error = %v", err)
		}
		if err := ValidateMCPSecretRefAccess(
			workspaceAPrefix+"env/TOKEN",
			MCPWorkspaceScope,
			workspaceA,
			"linear",
		); err != nil {
			t.Fatalf("ValidateMCPSecretRefAccess(owned) error = %v", err)
		}
		if err := ValidateMCPSecretRefAccess(
			MCPSharedRefPrefix+"linear-token",
			MCPWorkspaceScope,
			workspaceA,
			"linear",
		); err != nil {
			t.Fatalf("ValidateMCPSecretRefAccess(shared) error = %v", err)
		}
		if err := ValidateMCPSecretRefAccess(
			workspaceBPrefix+"env/TOKEN",
			MCPWorkspaceScope,
			workspaceA,
			"linear",
		); err == nil {
			t.Fatal("ValidateMCPSecretRefAccess(other workspace) error = nil, want owner rejection")
		}
	})

	t.Run("Should reject caller refs in the daemon-managed OAuth subtree", func(t *testing.T) {
		t.Parallel()

		refs, err := MCPDCRSecretRefsForTarget(MCPUserScope, "", "linear")
		if err != nil {
			t.Fatalf("MCPDCRSecretRefsForTarget() error = %v", err)
		}
		for _, ref := range []string{
			refs.ClientSecretRef,
			refs.RegistrationAccessTokenRef,
			strings.Replace(refs.ClientSecretRef, "/oauth/", "/OAuth/", 1),
		} {
			err := ValidateMCPSecretRefAccess(ref, MCPUserScope, "", "linear")
			if err == nil || !strings.Contains(err.Error(), "daemon-managed OAuth subtree") {
				t.Fatalf("ValidateMCPSecretRefAccess(%q) error = %v, want OAuth subtree rejection", ref, err)
			}
		}
	})
}
