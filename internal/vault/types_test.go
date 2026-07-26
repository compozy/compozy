package vault

import (
	"encoding/hex"
	"strings"
	"testing"
)

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
		{name: "Should allow token endpoint URLs", env: "AGH_BRIDGE_LINEAR_TOKEN_URL", want: false},
		{name: "Should allow secret named path variables", env: "AGH_SECRET_GUARD_HOST_CALL_PATH", want: false},
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

	t.Run("Should keep global and workspace MCP secret prefixes collision-safe", func(t *testing.T) {
		t.Parallel()

		workspaceID := "workspace/alpha"
		globalPrefix, err := MCPSecretOwnerPrefix(MCPGlobalScope, "", "linear")
		if err != nil {
			t.Fatalf("MCPSecretOwnerPrefix(global) error = %v", err)
		}
		workspacePrefix, err := MCPSecretOwnerPrefix(MCPWorkspaceScope, workspaceID, "linear")
		if err != nil {
			t.Fatalf("MCPSecretOwnerPrefix(workspace) error = %v", err)
		}
		if globalPrefix == workspacePrefix {
			t.Fatalf("global and workspace prefixes collided: %q", globalPrefix)
		}
		if got, want := globalPrefix, "vault:mcp/global/linear/"; got != want {
			t.Fatalf("global prefix = %q, want %q", got, want)
		}
		segment, err := MCPWorkspaceSegment(workspaceID)
		if err != nil {
			t.Fatalf("MCPWorkspaceSegment() error = %v", err)
		}
		if got, want := workspacePrefix, "vault:mcp/ws/"+segment+"/linear/"; got != want {
			t.Fatalf("workspace prefix = %q, want %q", got, want)
		}
		reservedSegment, err := MCPWorkspaceSegment(segment)
		if err != nil {
			t.Fatalf("MCPWorkspaceSegment(reserved prefix) error = %v", err)
		}
		if reservedSegment == segment {
			t.Fatalf("encoded workspace ID collided with reserved literal: %q", segment)
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
		prefix, err := MCPSecretOwnerPrefix(MCPGlobalScope, "", serverName)
		if err != nil {
			t.Fatalf("MCPSecretOwnerPrefix() error = %v", err)
		}
		if got, want := prefix, "vault:mcp/global/"+wantSegment+"/"; got != want {
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
