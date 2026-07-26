package settings

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
)

func TestMCPServerTargetSelectorValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid put selectors without mutating MCP sources", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "alpha"
command = "config-before"
`)
		sidecarPath := filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName)
		writeFile(t, sidecarPath, `{
  "mcpServers": {
    "alpha": { "command": "sidecar-before" }
  }
}`)
		configBefore := readFile(t, homePaths.ConfigFile)
		sidecarBefore := readFile(t, sidecarPath)
		service := testService(t, homePaths, Dependencies{})

		_, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "alpha",
			Target:            TargetSelector("cfg"),
			MCPServer: &aghconfig.MCPServer{
				Command: "after",
			},
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("PutCollectionItem(invalid target) error = %v, want ErrValidation", err)
		}
		if !strings.Contains(err.Error(), "unsupported MCP target selector") {
			t.Fatalf("PutCollectionItem(invalid target) error = %v, want selector context", err)
		}
		if got := readFile(t, homePaths.ConfigFile); got != configBefore {
			t.Fatalf("config payload changed after invalid target:\n%s", got)
		}
		if got := readFile(t, sidecarPath); got != sidecarBefore {
			t.Fatalf("sidecar payload changed after invalid target:\n%s", got)
		}
	})

	t.Run("Should reject invalid delete selectors without mutating MCP sources", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "alpha"
command = "config-before"
`)
		sidecarPath := filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName)
		writeFile(t, sidecarPath, `{
  "mcpServers": {
    "alpha": { "command": "sidecar-before" }
  }
}`)
		configBefore := readFile(t, homePaths.ConfigFile)
		sidecarBefore := readFile(t, sidecarPath)
		service := testService(t, homePaths, Dependencies{})

		_, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "alpha",
			Target:            TargetSelector("CONFIG"),
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("DeleteCollectionItem(invalid target) error = %v, want ErrValidation", err)
		}
		if !strings.Contains(err.Error(), "unsupported MCP target selector") {
			t.Fatalf("DeleteCollectionItem(invalid target) error = %v, want selector context", err)
		}
		if got := readFile(t, homePaths.ConfigFile); got != configBefore {
			t.Fatalf("config payload changed after invalid target:\n%s", got)
		}
		if got := readFile(t, sidecarPath); got != sidecarBefore {
			t.Fatalf("sidecar payload changed after invalid target:\n%s", got)
		}
	})
}

func TestMCPServerDefinitionMutationsInvalidatePendingOAuthSessions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "linear"
transport = "http"
url = "https://original.example/mcp"

[mcp_servers.auth]
type = "oauth2_pkce"
authorization_url = "https://issuer.example/authorize"
token_url = "https://issuer.example/token"
client_id = "client-id"
`)
	runtime := &recordingMCPAuthRuntime{}
	service := testService(t, homePaths, Dependencies{MCPAuth: runtime})
	target := mcpauth.Target{Scope: mcpauth.ScopeGlobal, ServerName: "linear"}

	if _, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
		Name:              "linear",
		Target:            TargetConfig,
		MCPServer: &aghconfig.MCPServer{
			Transport: aghconfig.MCPServerTransportHTTP,
			URL:       "https://replacement.example/mcp",
			Auth: aghconfig.MCPAuthConfig{
				Type: aghconfig.MCPAuthTypeOAuth2PKCE, AuthorizationURL: "https://issuer.example/authorize",
				TokenURL: "https://issuer.example/token", ClientID: "client-id",
			},
		},
	}); err != nil {
		t.Fatalf("PutCollectionItem() error = %v", err)
	}
	assertMCPAuthInvalidations(t, runtime.invalidated, target, 2)

	if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
		Name:              "linear",
		Target:            TargetConfig,
	}); err != nil {
		t.Fatalf("DeleteCollectionItem() error = %v", err)
	}
	assertMCPAuthInvalidations(t, runtime.invalidated, target, 4)
}

func assertMCPAuthInvalidations(
	t *testing.T,
	invalidated []mcpauth.Target,
	want mcpauth.Target,
	wantCount int,
) {
	t.Helper()
	if len(invalidated) != wantCount {
		t.Fatalf("MCP auth invalidation count = %d, want %d", len(invalidated), wantCount)
	}
	for index, target := range invalidated {
		if target != want {
			t.Fatalf("MCP auth invalidation[%d] = %#v, want %#v", index, target, want)
		}
	}
}
