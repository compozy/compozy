package settings

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
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
		sidecarPath := filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName)
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
			MCPServer: &compozyconfig.MCPServer{
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
		sidecarPath := filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName)
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

	t.Run("Should invalidate pending state before deleting durable auth state", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "linear"
transport = "http"
url = "https://original.example/mcp"

[mcp_servers.auth]
registration = "pre_registered"
issuer_url = "https://issuer.example"
client_id = "client-id"
`)
		runtime := &recordingMCPAuthRuntime{}
		service := testService(t, homePaths, Dependencies{MCPAuth: runtime})
		target := mcpauth.Target{Scope: mcpauth.ScopeGlobal, ServerName: "linear"}

		if _, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "linear",
			Target:            TargetConfig,
			MCPServer: &compozyconfig.MCPServer{
				Transport: compozyconfig.MCPServerTransportHTTP,
				URL:       "https://replacement.example/mcp",
				Auth: compozyconfig.MCPAuthConfig{
					Registration: compozyconfig.MCPAuthRegistrationPreRegistered,
					IssuerURL:    "https://issuer.example",
					ClientID:     "client-id",
				},
			},
		}); err != nil {
			t.Fatalf("PutCollectionItem() error = %v", err)
		}
		assertMCPAuthLifecycle(t, runtime.operations, target, 1)

		if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "linear",
			Target:            TargetConfig,
		}); err != nil {
			t.Fatalf("DeleteCollectionItem() error = %v", err)
		}
		assertMCPAuthLifecycle(t, runtime.operations, target, 2)
	})
}

func assertMCPAuthLifecycle(
	t *testing.T,
	operations []recordingMCPAuthOperation,
	want mcpauth.Target,
	wantCycles int,
) {
	t.Helper()
	if got, wantCount := len(operations), wantCycles*2; got != wantCount {
		t.Fatalf("MCP auth lifecycle operation count = %d, want %d", got, wantCount)
	}
	for index, operation := range operations {
		wantKind := recordingMCPAuthInvalidate
		if index%2 == 1 {
			wantKind = recordingMCPAuthDeleteState
		}
		if operation.kind != wantKind || operation.target != want {
			t.Fatalf("MCP auth lifecycle[%d] = %#v, want %s for %#v", index, operation, wantKind, want)
		}
	}
}
