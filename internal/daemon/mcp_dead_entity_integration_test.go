//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/deadentity"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"
)

const (
	deadEntityMCPStatePathEnv    = "AGH_TEST_DEAD_ENTITY_MCP_STATE_PATH"
	deadEntityMCPStateReady      = "ready"
	deadEntityMCPStateTerminated = "terminated"
)

func TestDaemonIntegrationMCPDeadEntityRecoversWithoutRestart(t *testing.T) {
	t.Run("Should retain unavailable tool diagnostics and recover the real sidecar after the due probe", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		now := time.Date(2026, 7, 15, 20, 0, 0, 0, time.UTC)
		workspaceID := "ws-dead-mcp-integration"
		globalDB, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), store.GlobalDatabaseName))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := globalDB.Close(context.Background()); err != nil {
				t.Errorf("GlobalDB.Close() error = %v", err)
			}
		})
		if err := globalDB.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID:        workspaceID,
			Name:      "Dead MCP integration",
			RootDir:   t.TempDir(),
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}

		statePath := filepath.Join(t.TempDir(), "sidecar-state")
		writeDeadEntityMCPState(t, statePath, deadEntityMCPStateReady)
		server := aghconfig.MCPServer{
			Name:      "recovery-fixture",
			Transport: aghconfig.MCPServerTransportStdio,
			Command:   os.Args[0],
			Args:      []string{"-test.run=^TestDeadEntityMCPStdioHelperProcess$"},
			Env: map[string]string{
				deadEntityMCPHelperEnv:    "1",
				deadEntityMCPStatePathEnv: statePath,
			},
		}
		catalog := newResourceCatalog(cloneDaemonMCPServer)
		catalog.Replace(1, []resources.Record[aghconfig.MCPServer]{{
			Kind:    aghconfig.MCPServerResourceKind,
			ID:      "mcp-server.recovery-fixture",
			Version: 1,
			Scope: resources.ResourceScope{
				Kind: resources.ResourceScopeKindWorkspace,
				ID:   workspaceID,
			},
			Spec: server,
		}})
		state := &bootState{
			cfg:              aghconfig.Config{},
			registry:         globalDB,
			mcpServerCatalog: catalog,
			deadEntities: deadentity.New(
				globalDB,
				deadentity.WithClock(func() time.Time { return now }),
			),
		}
		daemon := &Daemon{getenv: os.Getenv}
		provider, _, err := daemon.newDaemonMCPToolProvider(state)
		if err != nil {
			t.Fatalf("newDaemonMCPToolProvider() error = %v", err)
		}
		registry, err := toolspkg.NewRegistry(toolspkg.WithProviders(provider))
		if err != nil {
			t.Fatalf("tools.NewRegistry() error = %v", err)
		}
		scope := toolspkg.Scope{WorkspaceID: workspaceID, Operator: true}
		const toolID toolspkg.ToolID = "mcp__recovery_fixture__lookup"

		ready := requireDeadEntityMCPToolView(t, ctx, registry, scope, toolID)
		if !ready.Availability.Executable {
			t.Fatalf("initial availability = %#v, want executable", ready.Availability)
		}
		if ready.Descriptor.Source.WorkspaceID != workspaceID {
			t.Fatalf(
				"initial descriptor workspace_id = %q, want %q",
				ready.Descriptor.Source.WorkspaceID,
				workspaceID,
			)
		}

		writeDeadEntityMCPState(t, statePath, deadEntityMCPStateTerminated)
		var dead toolspkg.ToolView
		for attempt := 1; attempt <= deadentity.DefaultPermanentFailureThreshold; attempt++ {
			dead = requireDeadEntityMCPToolView(t, ctx, registry, scope, toolID)
			if slices.Contains(dead.Availability.ReasonCodes, toolspkg.ReasonBackendDead) {
				break
			}
		}
		if dead.Availability.Executable ||
			!slices.Contains(dead.Availability.ReasonCodes, toolspkg.ReasonBackendDead) {
			t.Fatalf("dead availability = %#v, want unavailable with backend_dead", dead.Availability)
		}
		listed, err := globalDB.ListDeadEntities(ctx, workspaceID)
		if err != nil {
			t.Fatalf("ListDeadEntities(marked) error = %v", err)
		}
		if len(listed) != 1 || listed[0].Kind != store.DeadEntityKindMCPSidecar ||
			listed[0].EntityID != server.Name {
			t.Fatalf("dead entities = %#v, want recovery-fixture MCP sidecar", listed)
		}

		writeDeadEntityMCPState(t, statePath, deadEntityMCPStateReady)
		now = now.Add(deadentity.DefaultRecoveryInterval)
		recovered := requireDeadEntityMCPToolView(t, ctx, registry, scope, toolID)
		if !recovered.Availability.Executable || len(recovered.Availability.ReasonCodes) != 0 {
			t.Fatalf("recovered availability = %#v, want executable without reasons", recovered.Availability)
		}
		listed, err = globalDB.ListDeadEntities(ctx, workspaceID)
		if err != nil {
			t.Fatalf("ListDeadEntities(recovered) error = %v", err)
		}
		if len(listed) != 0 {
			t.Fatalf("dead entities after recovery = %#v, want none", listed)
		}
	})
}

func TestDeadEntityMCPStdioHelperProcess(t *testing.T) {
	if os.Getenv(deadEntityMCPHelperEnv) != "1" {
		return
	}
	stateBytes, err := os.ReadFile(os.Getenv(deadEntityMCPStatePathEnv))
	if err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(3)
		}
		os.Exit(2)
	}
	if strings.TrimSpace(string(stateBytes)) == deadEntityMCPStateTerminated {
		os.Exit(17)
	}
	if err := mcpsrv.ServeStdio(newDeadEntityMCPFixtureServer()); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(3)
		}
		os.Exit(2)
	}
	os.Exit(0)
}

func newDeadEntityMCPFixtureServer() *mcpsrv.MCPServer {
	server := mcpsrv.NewMCPServer("dead-entity-fixture", "1.0.0", mcpsrv.WithToolCapabilities(true))
	server.AddTool(
		mcpsdk.NewTool(
			"lookup",
			mcpsdk.WithDescription("Look up a recovery fixture value"),
			mcpsdk.WithString("query"),
			mcpsdk.WithRawOutputSchema(json.RawMessage(
				`{"type":"object","properties":{"answer":{"type":"string"}}}`,
			)),
			mcpsdk.WithReadOnlyHintAnnotation(true),
		),
		func(context.Context, mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			return mcpsdk.NewToolResultText("ready"), nil
		},
	)
	return server
}

func requireDeadEntityMCPToolView(
	t *testing.T,
	ctx context.Context,
	registry *toolspkg.RuntimeRegistry,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
) toolspkg.ToolView {
	t.Helper()
	views, err := registry.List(ctx, scope)
	if err != nil {
		t.Fatalf("registry.List() error = %v", err)
	}
	for i := range views {
		if views[i].Descriptor.ID == toolID {
			return views[i]
		}
	}
	t.Fatalf("registry.List() = %#v, want retained tool %q", views, toolID)
	return toolspkg.ToolView{}
}

func writeDeadEntityMCPState(t *testing.T, path string, state string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(state), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}
