package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
)

var (
	errTriggerFailure  = errors.New("trigger failure")
	errProviderFailure = errors.New("provider failure")
)

func testToolSpec(id toolspkg.ToolID) toolspkg.Tool {
	return toolspkg.Tool{
		ID:           id,
		DisplayTitle: "lookup",
		Description:  "Search extension data",
		Backend: toolspkg.BackendRef{
			Kind:        toolspkg.BackendExtensionHost,
			ExtensionID: "linear",
			Handler:     "lookup",
		},
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Source: toolspkg.SourceRef{
			Kind:        toolspkg.SourceExtension,
			Owner:       "linear",
			RawToolName: "lookup",
		},
		Visibility: toolspkg.VisibilityOperator,
		Risk:       toolspkg.RiskRead,
		ReadOnly:   true,
	}
}

func TestResourceCatalogProjectorBuildAndApply(t *testing.T) {
	t.Parallel()

	t.Run("Should build a projection plan and apply a defensive catalog snapshot", func(t *testing.T) {
		t.Parallel()

		catalog := newResourceCatalog(cloneToolSpec)
		projector := newToolProjector(catalog)
		if projector == nil {
			t.Fatal("newToolProjector() = nil, want projector")
		}
		if got, want := projector.Kind(), toolspkg.ToolResourceKind; got != want {
			t.Fatalf("projector.Kind() = %q, want %q", got, want)
		}
		if got := projector.DependsOn(); got != nil {
			t.Fatalf("projector.DependsOn() = %#v, want nil", got)
		}

		records := []resources.Record[toolspkg.Tool]{{
			ID:      "lookup",
			Version: 3,
			Scope: resources.ResourceScope{
				Kind: resources.ResourceScopeKindGlobal,
			},
			Spec: testToolSpec("ext__linear__lookup"),
		}}

		plan, err := projector.Build(context.Background(), records)
		if err != nil {
			t.Fatalf("projector.Build() error = %v", err)
		}
		if got, want := plan.Kind(), toolspkg.ToolResourceKind; got != want {
			t.Fatalf("plan.Kind() = %q, want %q", got, want)
		}
		if got, want := plan.Revision(), int64(3); got != want {
			t.Fatalf("plan.Revision() = %d, want %d", got, want)
		}
		if got, want := plan.OperationCount(), 1; got != want {
			t.Fatalf("plan.OperationCount() = %d, want %d", got, want)
		}

		if err := projector.Apply(context.Background(), plan); err != nil {
			t.Fatalf("projector.Apply() error = %v", err)
		}
		if got, want := catalog.Revision(), int64(3); got != want {
			t.Fatalf("catalog.Revision() = %d, want %d", got, want)
		}

		snapshot := catalog.Snapshot()
		if got, want := len(snapshot), 1; got != want {
			t.Fatalf("len(snapshot) = %d, want %d", got, want)
		}
		snapshot[0].Spec.ID = "ext__linear__mutated"
		if got, want := catalog.Snapshot()[0].Spec.ID, toolspkg.ToolID("ext__linear__lookup"); got != want {
			t.Fatalf("catalog.Snapshot()[0].Spec.ID = %q, want %q", got, want)
		}
	})
}

func TestToolMCPComparisonAndNilHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should handle nil catalog and projector helpers", func(t *testing.T) {
		t.Parallel()

		if got := newToolProjector(nil); got != nil {
			t.Fatalf("newToolProjector(nil) = %#v, want nil", got)
		}
		if got := newMCPServerProjector(nil); got != nil {
			t.Fatalf("newMCPServerProjector(nil) = %#v, want nil", got)
		}

		var nilCatalog *resourceCatalog[toolspkg.Tool]
		nilCatalog.Replace(9, []resources.Record[toolspkg.Tool]{{ID: "ignored"}})
		if got := nilCatalog.Snapshot(); got != nil {
			t.Fatalf("nilCatalog.Snapshot() = %#v, want nil", got)
		}
		if got := nilCatalog.Revision(); got != 0 {
			t.Fatalf("nilCatalog.Revision() = %d, want 0", got)
		}

		var nilPlan *resourceCatalogProjectionPlan[toolspkg.Tool]
		if got := nilPlan.Kind(); got != "" {
			t.Fatalf("nilPlan.Kind() = %q, want empty", got)
		}
		if got := nilPlan.Revision(); got != 0 {
			t.Fatalf("nilPlan.Revision() = %d, want 0", got)
		}
		if got := nilPlan.OperationCount(); got != 0 {
			t.Fatalf("nilPlan.OperationCount() = %d, want 0", got)
		}

		var nilProjector *resourceCatalogProjector[toolspkg.Tool]
		if got := nilProjector.Kind(); got != "" {
			t.Fatalf("nilProjector.Kind() = %q, want empty", got)
		}
		if got := nilProjector.DependsOn(); got != nil {
			t.Fatalf("nilProjector.DependsOn() = %#v, want nil", got)
		}
		if _, err := nilProjector.Build(context.Background(), nil); err == nil {
			t.Fatal("nilProjector.Build() error = nil, want non-nil")
		}
		if err := nilProjector.Apply(
			context.Background(),
			&resourceCatalogProjectionPlan[toolspkg.Tool]{},
		); err == nil {
			t.Fatal("nilProjector.Apply() error = nil, want non-nil")
		}
		if got := newToolMCPSourceSyncer(nil, nil, nil, nil, nil, resources.MutationActor{}, nil, nil); got != nil {
			t.Fatalf("newToolMCPSourceSyncer(nil deps) = %#v, want nil", got)
		}
	})

	t.Run("Should compare encoded tool and MCP resources", func(t *testing.T) {
		t.Parallel()

		toolCodec, err := toolspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
		}
		mcpCodec, err := aghconfig.NewMCPServerResourceCodec()
		if err != nil {
			t.Fatalf("aghconfig.NewMCPServerResourceCodec() error = %v", err)
		}

		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		workspaceScope := resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"}

		toolSpec := testToolSpec("ext__linear__lookup")
		toolEncoded, err := toolCodec.Encode(toolSpec)
		if err != nil {
			t.Fatalf("toolCodec.Encode() error = %v", err)
		}
		toolRecord := resources.RawRecord{
			Scope:    globalScope,
			SpecJSON: toolEncoded,
		}
		if !sameManagedRawRecord(toolRecord, globalScope, toolEncoded) {
			t.Fatal("sameManagedRawRecord(tool) = false, want true for matching scope and spec")
		}
		if sameManagedRawRecord(toolRecord, workspaceScope, toolEncoded) {
			t.Fatal("sameManagedRawRecord(tool) = true, want false for mismatched scope")
		}
		if sameManagedRawRecord(toolRecord, globalScope, []byte(`{"bad":true}`)) {
			t.Fatal("sameManagedRawRecord(tool) = true, want false for mismatched encoding")
		}

		mcpSpec := aghconfig.MCPServer{
			Name:    "git",
			Command: "npx",
			Args:    []string{"@modelcontextprotocol/server-git"},
		}
		mcpEncoded, err := mcpCodec.Encode(mcpSpec)
		if err != nil {
			t.Fatalf("mcpCodec.Encode() error = %v", err)
		}
		mcpRecord := resources.RawRecord{
			Scope:    globalScope,
			SpecJSON: mcpEncoded,
		}
		if !sameManagedRawRecord(mcpRecord, globalScope, mcpEncoded) {
			t.Fatal("sameManagedRawRecord(mcp) = false, want true for matching scope and spec")
		}
		if sameManagedRawRecord(mcpRecord, workspaceScope, mcpEncoded) {
			t.Fatal("sameManagedRawRecord(mcp) = true, want false for mismatched scope")
		}
		if sameManagedRawRecord(mcpRecord, globalScope, []byte(`{"bad":true}`)) {
			t.Fatal("sameManagedRawRecord(mcp) = true, want false for mismatched encoding")
		}
	})

	t.Run("Should treat publisher helpers as noop or passthrough", func(t *testing.T) {
		t.Parallel()

		var nilPublisher toolMCPPublisherFunc
		if err := nilPublisher.Sync(context.Background()); err != nil {
			t.Fatalf("nilPublisher.Sync() error = %v", err)
		}
		called := false
		publisher := toolMCPPublisherFunc(func(context.Context) error {
			called = true
			return nil
		})
		if err := publisher.Sync(context.Background()); err != nil {
			t.Fatalf("publisher.Sync() error = %v", err)
		}
		if !called {
			t.Fatal("publisher.Sync() did not invoke wrapped function")
		}
	})

	t.Run("Should build syncer even when logger is nil", func(t *testing.T) {
		t.Parallel()

		raw, toolStore, toolCodec, mcpStore, mcpCodec := toolMCPSyncStores(t)
		syncerWithNilLogger := newToolMCPSourceSyncer(
			raw,
			toolStore,
			toolCodec,
			mcpStore,
			mcpCodec,
			toolMCPSyncActor(),
			nil,
			nil,
		)
		if syncerWithNilLogger == nil {
			t.Fatal("newToolMCPSourceSyncer(nil logger) = nil, want syncer")
		}
		concreteSyncer, ok := syncerWithNilLogger.(*toolMCPSourceSyncer)
		if !ok {
			t.Fatalf("syncerWithNilLogger type = %T, want *toolMCPSourceSyncer", syncerWithNilLogger)
		}
		if err := concreteSyncer.Sync(context.Background()); err != nil {
			t.Fatalf("syncerWithNilLogger.Sync() error = %v", err)
		}
	})
}

func TestToolMCPSourceSyncerHandlesNilReceiverAndTriggerFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should treat nil receiver as noop and return trigger failures", func(t *testing.T) {
		t.Parallel()

		var nilSyncer *toolMCPSourceSyncer
		if err := nilSyncer.Sync(context.Background()); err != nil {
			t.Fatalf("nilSyncer.Sync() error = %v", err)
		}

		raw, toolStore, toolCodec, mcpStore, mcpCodec := toolMCPSyncStores(t)
		syncer := newToolMCPSourceSyncer(
			raw,
			toolStore,
			toolCodec,
			mcpStore,
			mcpCodec,
			toolMCPSyncActor(),
			discardLogger(),
			func(context.Context, resources.ResourceKind, resources.ReconcileReason) error {
				return errTriggerFailure
			},
			func(context.Context) (toolMCPDesiredResources, error) {
				return toolMCPDesiredResources{
					tools: []toolPublicationInput{{
						sourceKey: "test/tool/lookup",
						scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
						spec:      testToolSpec("ext__linear__lookup"),
					}},
				}, nil
			},
		)

		err := syncer.Sync(context.Background())
		if err == nil {
			t.Fatal("syncer.Sync() error = nil, want trigger failure")
		}
		if !errors.Is(err, errTriggerFailure) {
			t.Fatalf("syncer.Sync() error = %v, want %v", err, errTriggerFailure)
		}
	})
}

func TestToolMCPSourceSyncerSyncPropagatesProviderFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should propagate provider failures", func(t *testing.T) {
		t.Parallel()

		raw, toolStore, toolCodec, mcpStore, mcpCodec := toolMCPSyncStores(t)
		syncer := newToolMCPSourceSyncer(
			raw,
			toolStore,
			toolCodec,
			mcpStore,
			mcpCodec,
			toolMCPSyncActor(),
			discardLogger(),
			nil,
			func(context.Context) (toolMCPDesiredResources, error) {
				return toolMCPDesiredResources{}, errProviderFailure
			},
		)

		err := syncer.Sync(context.Background())
		if err == nil {
			t.Fatal("syncer.Sync() error = nil, want provider failure")
		}
		if !errors.Is(err, errProviderFailure) {
			t.Fatalf("syncer.Sync() error = %v, want %v", err, errProviderFailure)
		}
	})
}

func TestToolMCPSourceSyncerReplacesCanonicalSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("Should replace canonical snapshots and avoid noop triggers", func(t *testing.T) {
		t.Parallel()

		raw, toolStore, toolCodec, mcpStore, mcpCodec := toolMCPSyncStores(t)
		desired := toolMCPDesiredResources{
			tools: []toolPublicationInput{{
				sourceKey: "test/tool/lookup",
				scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				spec:      testToolSpec("ext__linear__lookup"),
			}},
			mcpServers: []mcpServerPublicationInput{{
				sourceKey: "test/mcp/git",
				scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				spec: aghconfig.MCPServer{
					Name:    "git",
					Command: "npx",
					Args:    []string{"@modelcontextprotocol/server-git"},
				},
			}},
		}
		triggered := make(map[resources.ResourceKind]int)
		syncer := newToolMCPSourceSyncer(
			raw,
			toolStore,
			toolCodec,
			mcpStore,
			mcpCodec,
			toolMCPSyncActor(),
			discardLogger(),
			func(_ context.Context, kind resources.ResourceKind, _ resources.ReconcileReason) error {
				triggered[kind]++
				return nil
			},
			func(context.Context) (toolMCPDesiredResources, error) {
				return desired, nil
			},
		)

		if err := syncer.Sync(context.Background()); err != nil {
			t.Fatalf("Sync() error = %v", err)
		}
		assertToolMCPStoreCounts(t, toolStore, mcpStore, 1, 1)
		if triggered[toolspkg.ToolResourceKind] != 1 || triggered[aghconfig.MCPServerResourceKind] != 1 {
			t.Fatalf("triggered = %#v, want one trigger per resource kind", triggered)
		}

		if err := syncer.Sync(context.Background()); err != nil {
			t.Fatalf("second Sync() error = %v", err)
		}
		if triggered[toolspkg.ToolResourceKind] != 1 || triggered[aghconfig.MCPServerResourceKind] != 1 {
			t.Fatalf("triggered after no-op = %#v, want no additional triggers", triggered)
		}

		desired.tools = nil
		desired.mcpServers[0].spec.Command = "node"
		if err := syncer.Sync(context.Background()); err != nil {
			t.Fatalf("third Sync() error = %v", err)
		}
		assertToolMCPStoreCounts(t, toolStore, mcpStore, 0, 1)
		if triggered[toolspkg.ToolResourceKind] != 2 || triggered[aghconfig.MCPServerResourceKind] != 2 {
			t.Fatalf("triggered after replacement = %#v, want delete/update triggers", triggered)
		}
	})
}

func TestNewToolMCPPublisherFallsBackToNoopWithoutResourceRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should fall back to noop publisher without resource runtime", func(t *testing.T) {
		t.Parallel()

		daemon := &Daemon{}

		publisher, err := daemon.newToolMCPPublisher(nil, nil)
		if err != nil {
			t.Fatalf("newToolMCPPublisher(nil state) error = %v", err)
		}
		if publisher == nil {
			t.Fatal("newToolMCPPublisher(nil state) = nil, want no-op publisher")
		}
		if err := publisher.Sync(context.Background()); err != nil {
			t.Fatalf("publisher.Sync(nil state) error = %v", err)
		}

		publisher, err = daemon.newToolMCPPublisher(&bootState{}, nil)
		if err != nil {
			t.Fatalf("newToolMCPPublisher(empty state) error = %v", err)
		}
		if publisher == nil {
			t.Fatal("newToolMCPPublisher(empty state) = nil, want no-op publisher")
		}
		if err := publisher.Sync(context.Background()); err != nil {
			t.Fatalf("publisher.Sync(empty state) error = %v", err)
		}
	})
}

func TestNewToolMCPPublisherBuildsSyncerWhenResourceRuntimeIsReady(t *testing.T) {
	t.Parallel()

	t.Run("Should build syncer when resource runtime is ready", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		kernel, err := resources.NewKernel(db.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}

		toolCodec, err := toolspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
		}
		mcpCodec, err := aghconfig.NewMCPServerResourceCodec()
		if err != nil {
			t.Fatalf("aghconfig.NewMCPServerResourceCodec() error = %v", err)
		}

		codecs := resources.NewCodecRegistry()
		if err := resources.RegisterCodec(codecs, toolCodec); err != nil {
			t.Fatalf("RegisterCodec(tool) error = %v", err)
		}
		if err := resources.RegisterCodec(codecs, mcpCodec); err != nil {
			t.Fatalf("RegisterCodec(mcp) error = %v", err)
		}

		daemon := &Daemon{getenv: func(string) string { return "" }}
		publisher, err := daemon.newToolMCPPublisher(&bootState{
			logger:         discardLogger(),
			resourceKernel: kernel,
			resourceCodecs: codecs,
		}, nil)
		if err != nil {
			t.Fatalf("newToolMCPPublisher(ready state) error = %v", err)
		}
		if publisher == nil {
			t.Fatal("newToolMCPPublisher(ready state) = nil, want syncer")
		}
		if err := publisher.Sync(context.Background()); err != nil {
			t.Fatalf("publisher.Sync(ready state) error = %v", err)
		}
	})
}

func TestNewToolMCPPublisherReturnsCodecResolutionErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should return codec resolution errors", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		kernel, err := resources.NewKernel(db.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}

		daemon := &Daemon{}
		emptyCodecs := resources.NewCodecRegistry()
		_, err = daemon.newToolMCPPublisher(&bootState{
			logger:         discardLogger(),
			resourceKernel: kernel,
			resourceCodecs: emptyCodecs,
		}, nil)
		if err == nil {
			t.Fatal("newToolMCPPublisher(empty codecs) error = nil, want tool codec failure")
		}
		if !errors.Is(err, resources.ErrCodecNotFound) {
			t.Fatalf("newToolMCPPublisher(empty codecs) error = %v, want %v", err, resources.ErrCodecNotFound)
		}

		toolCodec, err := toolspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
		}
		toolOnlyCodecs := resources.NewCodecRegistry()
		if err := resources.RegisterCodec(toolOnlyCodecs, toolCodec); err != nil {
			t.Fatalf("RegisterCodec(tool) error = %v", err)
		}

		_, err = daemon.newToolMCPPublisher(&bootState{
			logger:         discardLogger(),
			resourceKernel: kernel,
			resourceCodecs: toolOnlyCodecs,
		}, nil)
		if err == nil {
			t.Fatal("newToolMCPPublisher(tool-only codecs) error = nil, want mcp codec failure")
		}
		if !errors.Is(err, resources.ErrCodecNotFound) {
			t.Fatalf("newToolMCPPublisher(tool-only codecs) error = %v, want %v", err, resources.ErrCodecNotFound)
		}
	})
}

func TestValidateAndEncodeToolAndMCPServer(t *testing.T) {
	t.Parallel()

	t.Run("Should validate and encode tool and MCP resources", func(t *testing.T) {
		t.Parallel()

		toolCodec, err := toolspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
		}

		toolScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		toolSpec := testToolSpec("ext__linear__lookup")
		toolSpec.Description = " Search extension data "
		toolSpec.InputSchema = json.RawMessage(`{"required":["query"],"type":"object"}`)
		toolEncodedSpec, toolEncoded, err := validateAndEncodeTool(context.Background(), toolCodec, toolScope, toolSpec)
		if err != nil {
			t.Fatalf("validateAndEncodeTool(valid) error = %v", err)
		}
		if got, want := toolEncodedSpec.ID, toolspkg.ToolID("ext__linear__lookup"); got != want {
			t.Fatalf("toolSpec.ID = %q, want %q", got, want)
		}
		var toolPayload struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Source      struct {
				Kind string `json:"kind"`
			} `json:"source"`
			ReadOnly bool `json:"read_only"`
		}
		if err := json.Unmarshal(toolEncoded, &toolPayload); err != nil {
			t.Fatalf("json.Unmarshal(toolEncoded) error = %v", err)
		}
		if got, want := toolPayload.ID, "ext__linear__lookup"; got != want {
			t.Fatalf("toolPayload.ID = %#v, want %#v", got, want)
		}
		if got, want := toolPayload.Description, "Search extension data"; got != want {
			t.Fatalf("toolPayload.Description = %#v, want %#v", got, want)
		}
		if got, want := toolPayload.Source.Kind, "extension"; got != want {
			t.Fatalf("toolPayload.Source.Kind = %#v, want %#v", got, want)
		}
		if got, want := toolPayload.ReadOnly, true; got != want {
			t.Fatalf("toolPayload.ReadOnly = %#v, want %#v", got, want)
		}

		_, _, err = validateAndEncodeTool(context.Background(), toolCodec, toolScope, toolspkg.Tool{
			ID:          "ext__linear__lookup",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		})
		if err == nil {
			t.Fatal("validateAndEncodeTool(invalid) error = nil, want validation failure")
		}
		if !errors.Is(err, resources.ErrValidation) {
			t.Fatalf("validateAndEncodeTool(invalid) error = %v, want %v", err, resources.ErrValidation)
		}
		if !strings.Contains(err.Error(), "backend.kind") {
			t.Fatalf("validateAndEncodeTool(invalid) error = %v, want backend.kind validation context", err)
		}

		mcpCodec, err := aghconfig.NewMCPServerResourceCodec()
		if err != nil {
			t.Fatalf("aghconfig.NewMCPServerResourceCodec() error = %v", err)
		}

		mcpSpec, mcpEncoded, err := validateAndEncodeMCPServer(
			context.Background(),
			mcpCodec,
			toolScope,
			aghconfig.MCPServer{
				Name:    " git ",
				Command: " npx ",
				Args:    []string{" --stdio "},
				SecretEnv: map[string]string{
					" TOKEN ": " env:MCP_TOKEN ",
				},
			},
		)
		if err != nil {
			t.Fatalf("validateAndEncodeMCPServer(valid) error = %v", err)
		}
		if got, want := mcpSpec.Name, "git"; got != want {
			t.Fatalf("mcpSpec.Name = %q, want %q", got, want)
		}
		var mcpPayload struct {
			Name    string
			Command string
		}
		if err := json.Unmarshal(mcpEncoded, &mcpPayload); err != nil {
			t.Fatalf("json.Unmarshal(mcpEncoded) error = %v", err)
		}
		if got, want := mcpPayload.Name, "git"; got != want {
			t.Fatalf("mcpPayload.Name = %#v, want %#v", got, want)
		}
		if got, want := mcpPayload.Command, "npx"; got != want {
			t.Fatalf("mcpPayload.Command = %#v, want %#v", got, want)
		}

		_, _, err = validateAndEncodeMCPServer(
			context.Background(),
			mcpCodec,
			toolScope,
			aghconfig.MCPServer{Name: "git"},
		)
		if err == nil {
			t.Fatal("validateAndEncodeMCPServer(invalid) error = nil, want validation failure")
		}
		if !strings.Contains(err.Error(), "config: validate mcp resource spec") {
			t.Fatalf("validateAndEncodeMCPServer(invalid) error = %v, want mcp resource spec context", err)
		}
		if !strings.Contains(err.Error(), "mcp_server.command is required") {
			t.Fatalf("validateAndEncodeMCPServer(invalid) error = %v, want missing command validation", err)
		}
	})
}

func toolMCPSyncStores(
	t *testing.T,
) (
	resources.RawStore,
	resources.Store[toolspkg.Tool],
	resources.KindCodec[toolspkg.Tool],
	resources.Store[aghconfig.MCPServer],
	resources.KindCodec[aghconfig.MCPServer],
) {
	t.Helper()

	db := openDaemonTestGlobalDB(t)
	kernel, err := resources.NewKernel(db.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}

	toolCodec, err := toolspkg.NewResourceCodec()
	if err != nil {
		t.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
	}
	toolStore, err := resources.NewStore(kernel, toolCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(tool) error = %v", err)
	}

	mcpCodec, err := aghconfig.NewMCPServerResourceCodec()
	if err != nil {
		t.Fatalf("aghconfig.NewMCPServerResourceCodec() error = %v", err)
	}
	mcpStore, err := resources.NewStore(kernel, mcpCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(mcp) error = %v", err)
	}

	return kernel, toolStore, toolCodec, mcpStore, mcpCodec
}

func assertToolMCPStoreCounts(
	t *testing.T,
	toolStore resources.Store[toolspkg.Tool],
	mcpStore resources.Store[aghconfig.MCPServer],
	wantTools int,
	wantMCPServers int,
) {
	t.Helper()

	source := toolMCPSyncActor().Source
	tools, err := toolStore.List(testutil.Context(t), toolMCPSyncActor(), resources.ResourceFilter{Source: &source})
	if err != nil {
		t.Fatalf("toolStore.List() error = %v", err)
	}
	if got := len(tools); got != wantTools {
		t.Fatalf("len(toolStore.List()) = %d, want %d", got, wantTools)
	}

	mcpServers, err := mcpStore.List(testutil.Context(t), toolMCPSyncActor(), resources.ResourceFilter{Source: &source})
	if err != nil {
		t.Fatalf("mcpStore.List() error = %v", err)
	}
	if got := len(mcpServers); got != wantMCPServers {
		t.Fatalf("len(mcpStore.List()) = %d, want %d", got, wantMCPServers)
	}
}
