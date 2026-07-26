package doctor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/subprocess"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Should reject duplicate probe identifiers", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		probe := &ProbeFunc{ProbeID: "doctor.a", ProbeCategory: contract.CategoryDaemon}
		if err := registry.Register(probe); err != nil {
			t.Fatalf("Register(first) error = %v", err)
		}
		if err := registry.Register(probe); err == nil {
			t.Fatal("Register(duplicate) error = nil, want duplicate failure")
		}
	})

	t.Run("Should return probes sorted by identifier", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		for _, probe := range []*ProbeFunc{
			{ProbeID: "doctor.z", ProbeCategory: contract.CategoryDaemon},
			{ProbeID: "doctor.a", ProbeCategory: contract.CategoryDaemon},
		} {
			if err := registry.Register(probe); err != nil {
				t.Fatalf("Register(%s) error = %v", probe.ProbeID, err)
			}
		}
		probes := registry.Probes()
		if got, want := probes[0].ID(), "doctor.a"; got != want {
			t.Fatalf("first probe ID = %q, want %q", got, want)
		}
	})
}

func TestDeadEntityProbe(t *testing.T) {
	t.Parallel()

	t.Run("Should emit redacted read-only diagnostics for the selected runtime family", func(t *testing.T) {
		t.Parallel()

		workspace := workspacepkg.Workspace{ID: "ws-reliability", Name: "Reliability"}
		source := deadEntityProbeSource{entities: map[string][]store.DeadEntity{
			workspace.ID: {
				{
					DeadEntityKey: store.DeadEntityKey{
						WorkspaceID: workspace.ID,
						Kind:        store.DeadEntityKindMCPSidecar,
						EntityID:    "github",
					},
					Reason:   "invalid api_key=super-secret",
					MarkedAt: time.Date(2026, 7, 15, 19, 0, 0, 0, time.UTC),
				},
				{
					DeadEntityKey: store.DeadEntityKey{
						WorkspaceID: workspace.ID,
						Kind:        store.DeadEntityKindBridge,
						EntityID:    "telegram",
					},
					Reason:   "bridge unavailable",
					MarkedAt: time.Date(2026, 7, 15, 19, 1, 0, 0, time.UTC),
				},
			},
		}}
		registry := NewRegistry()
		if err := registry.Register(&DeadEntityProbe{
			Source:     source,
			Workspaces: deadEntityProbeWorkspaceSource{workspaces: []workspacepkg.Workspace{workspace}},
			Kind:       store.DeadEntityKindMCPSidecar,
		}); err != nil {
			t.Fatalf("Register(dead entity probe) error = %v", err)
		}
		runner, err := NewRunner(registry)
		if err != nil {
			t.Fatalf("NewRunner() error = %v", err)
		}
		items, err := runner.Run(context.Background(), RunOptions{Only: []string{contract.CategoryMCP}})
		if err != nil {
			t.Fatalf("Run(mcp) error = %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("Run(mcp) items = %#v, want one MCP diagnostic", items)
		}
		item := items[0]
		if item.Code != contract.CodeMCPServerUnavailable || item.Category != contract.CategoryMCP {
			t.Fatalf("dead MCP diagnostic = %#v, want MCP unavailable", item)
		}
		if item.SuggestedCommand != "" {
			t.Fatalf("SuggestedCommand = %q, want read-only diagnosis without revive command", item.SuggestedCommand)
		}
		reason, ok := item.Evidence["reason"].(string)
		if !ok || strings.Contains(reason, "super-secret") || !strings.Contains(reason, "[REDACTED]") {
			t.Fatalf("diagnostic reason = %#v, want redacted evidence", item.Evidence["reason"])
		}
	})

	t.Run("Should return no diagnostic when no durable mark matches the kind", func(t *testing.T) {
		t.Parallel()

		probe := &DeadEntityProbe{
			Source:     deadEntityProbeSource{},
			Workspaces: deadEntityProbeWorkspaceSource{workspaces: []workspacepkg.Workspace{{ID: "ws-empty"}}},
			Kind:       store.DeadEntityKindExtension,
		}
		items, err := probe.Run(context.Background(), &ProbeEnv{})
		if err != nil {
			t.Fatalf("Run(empty) error = %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("Run(empty) = %#v, want no noise", items)
		}
	})
}

type deadEntityProbeSource struct {
	entities map[string][]store.DeadEntity
}

func (s deadEntityProbeSource) List(
	_ context.Context,
	workspaceID string,
) ([]store.DeadEntity, error) {
	return append([]store.DeadEntity(nil), s.entities[workspaceID]...), nil
}

type deadEntityProbeWorkspaceSource struct {
	workspaces []workspacepkg.Workspace
}

func (s deadEntityProbeWorkspaceSource) List(context.Context) ([]workspacepkg.Workspace, error) {
	return append([]workspacepkg.Workspace(nil), s.workspaces...), nil
}

func TestRunner(t *testing.T) {
	t.Parallel()

	t.Run("Should sanitize returned probe diagnostics", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		if err := registry.Register(&ProbeFunc{
			ProbeID:       "doctor.provider.auth",
			ProbeCategory: contract.CategoryProvider,
			RunFunc: func(context.Context, *ProbeEnv) ([]contract.DiagnosticItem, error) {
				return []contract.DiagnosticItem{
					diagnostics.NewItem(
						"doctor.provider.auth",
						contract.CodeProviderNotAuthenticated,
						contract.CategoryProvider,
						"Provider auth",
						"stderr token=probe-secret",
						contract.SeverityWarn,
						contract.FreshnessLive,
						diagnostics.WithEvidence(map[string]any{"access_token": "secret-value"}),
					),
				}, nil
			},
		}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		runner, err := NewRunner(registry)
		if err != nil {
			t.Fatalf("NewRunner() error = %v", err)
		}

		items, err := runner.Run(context.Background(), RunOptions{})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("Run() item count = %d, want 1", len(items))
		}
		if strings.Contains(items[0].Message, "probe-secret") {
			t.Fatalf("Diagnostic message = %q leaked secret", items[0].Message)
		}
		if items[0].Evidence["access_token"] != "[REDACTED]" {
			t.Fatalf("Evidence[access_token] = %#v, want redacted marker", items[0].Evidence["access_token"])
		}
	})

	t.Run("Should return structured diagnostic when probe fails", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		if err := registry.Register(&ProbeFunc{
			ProbeID:       "doctor.provider.cli",
			ProbeCategory: contract.CategoryProvider,
			RunFunc: func(context.Context, *ProbeEnv) ([]contract.DiagnosticItem, error) {
				return nil, errors.New("provider stderr token=raw-secret")
			},
		}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		runner, err := NewRunner(registry)
		if err != nil {
			t.Fatalf("NewRunner() error = %v", err)
		}

		items, err := runner.Run(context.Background(), RunOptions{})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("Run() item count = %d, want 1", len(items))
		}
		if items[0].Code != contract.CodeProbeFailed {
			t.Fatalf("failure Code = %q, want %q", items[0].Code, contract.CodeProbeFailed)
		}
		if strings.Contains(items[0].Message, "raw-secret") {
			t.Fatalf("failure Message = %q leaked raw error", items[0].Message)
		}
	})

	t.Run("Should classify probe timeouts as timeout diagnostics", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		if err := registry.Register(&ProbeFunc{
			ProbeID:       "doctor.daemon.timeout",
			ProbeCategory: contract.CategoryDaemon,
			RunFunc: func(ctx context.Context, _ *ProbeEnv) ([]contract.DiagnosticItem, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		runner, err := NewRunner(registry)
		if err != nil {
			t.Fatalf("NewRunner() error = %v", err)
		}

		items, err := runner.Run(context.Background(), RunOptions{ProbeTimeout: time.Millisecond})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("Run() item count = %d, want 1", len(items))
		}
		if items[0].Code != contract.CodeProbeTimeout {
			t.Fatalf("timeout Code = %q, want %q", items[0].Code, contract.CodeProbeTimeout)
		}
	})
}

func TestRuntimeMemoryProbe(t *testing.T) {
	t.Parallel()

	t.Run("Should return a populated periodic snapshot", func(t *testing.T) {
		t.Parallel()

		capturedAt := time.Date(2026, 7, 15, 12, 30, 0, 0, time.UTC)
		probe := &RuntimeMemoryProbe{Source: runtimeMemorySnapshotSourceStub{
			snapshot: RuntimeMemorySnapshot{
				Enabled:                 true,
				Active:                  true,
				CapturedAt:              capturedAt,
				Phase:                   "periodic",
				ResidentMemoryBytes:     4096,
				ResidentMemoryKind:      "current",
				ResidentMemoryAvailable: true,
				HeapAllocBytes:          2048,
				HeapInUseBytes:          3072,
				Goroutines:              9,
				UptimeSeconds:           60,
			},
		}}
		items, err := probe.Run(context.Background(), &ProbeEnv{})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("Run() item count = %d, want 1", len(items))
		}
		item := items[0]
		if item.ID != RuntimeMemoryProbeID || item.Code != contract.CodeDaemonStatusOK ||
			item.Severity != contract.SeverityOK {
			t.Fatalf("Run() item = %#v, want healthy runtime memory diagnostic", item)
		}
		if item.Evidence["phase"] != "periodic" ||
			item.Evidence["captured_at"] != capturedAt.Format(time.RFC3339Nano) ||
			item.Evidence["resident_memory_bytes"] != uint64(4096) ||
			item.Evidence["heap_alloc_bytes"] != uint64(2048) ||
			item.Evidence["goroutines"] != 9 {
			t.Fatalf("Run() evidence = %#v, want populated periodic snapshot", item.Evidence)
		}
	})

	t.Run("Should report interval zero as disabled deterministically", func(t *testing.T) {
		t.Parallel()

		probe := &RuntimeMemoryProbe{Source: runtimeMemorySnapshotSourceStub{
			snapshot: RuntimeMemorySnapshot{Phase: "disabled"},
		}}
		items, err := probe.Run(context.Background(), &ProbeEnv{})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("Run() item count = %d, want 1", len(items))
		}
		item := items[0]
		if item.Code != contract.CodeDaemonStatusOK || item.Severity != contract.SeverityOK ||
			item.Evidence["enabled"] != false || item.Evidence["active"] != false ||
			item.Evidence["phase"] != "disabled" || item.Evidence["captured_at"] != "" {
			t.Fatalf("Run() disabled item = %#v, want deterministic disabled diagnostic", item)
		}
	})
}

func TestSubprocessHealthProbe(t *testing.T) {
	t.Parallel()

	t.Run("Should report an unhealthy verdict as degraded with a redacted reason", func(t *testing.T) {
		t.Parallel()

		checkedAt := time.Date(2026, 7, 16, 9, 30, 0, 0, time.UTC)
		probe := &SubprocessHealthProbe{Source: subprocessHealthSnapshotSourceStub{
			snapshots: []session.SubprocessHealthSnapshot{{
				SessionID:   "sess-health",
				WorkspaceID: "ws-health",
				AgentName:   "coder",
				Health: subprocess.HealthState{
					Healthy:             false,
					Message:             "api_key=super-secret probe failed",
					LastCheckedAt:       checkedAt,
					ConsecutiveFailures: 3,
					LastError:           "context deadline exceeded",
				},
			}},
		}}

		items, err := probe.Run(context.Background(), &ProbeEnv{})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := len(items), 1; got != want {
			t.Fatalf("Run() item count = %d, want %d", got, want)
		}
		item := diagnostics.RedactItem(items[0])
		if item.ID != SubprocessHealthProbeID || item.Severity != contract.SeverityWarn ||
			item.Code != contract.CodeDaemonHealthUnavailable {
			t.Fatalf("Run() item = %#v, want degraded subprocess diagnostic", item)
		}
		if !strings.Contains(item.Message, "probe failed") || strings.Contains(item.Message, "super-secret") {
			t.Fatalf("Run() message = %q, want redacted verdict reason", item.Message)
		}
		if item.Evidence["session_id"] != "sess-health" ||
			item.Evidence["workspace_id"] != "ws-health" ||
			item.Evidence["consecutive_failures"] != 3 {
			t.Fatalf("Run() evidence = %#v, want session/workspace/failure correlation", item.Evidence)
		}
	})
}

func TestBridgeProbeCategoryFilter(t *testing.T) {
	t.Run("Should skip bridge checks without an explicit bridge filter", func(t *testing.T) {
		t.Parallel()

		source := &bridgeProbeSourceStub{
			instances: []bridgepkg.BridgeInstance{{
				ID:            "brg-slack",
				Platform:      "slack",
				ExtensionName: "slack",
			}},
		}
		registry := NewRegistry()
		if err := registry.Register(&BridgeProbe{Source: source}); err != nil {
			t.Fatalf("Register(bridge) error = %v", err)
		}
		runner, err := NewRunner(registry)
		if err != nil {
			t.Fatalf("NewRunner() error = %v", err)
		}

		items, err := runner.Run(context.Background(), RunOptions{})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if source.listCalls != 0 || source.checkCalls != 0 {
			t.Fatalf("bridge source calls = list:%d check:%d, want 0/0", source.listCalls, source.checkCalls)
		}
		if len(items) != 0 {
			t.Fatalf("bridge diagnostic count = %d, want 0", len(items))
		}
	})

	t.Run("Should run only bridge checks and preserve failing secret remediation", func(t *testing.T) {
		t.Parallel()

		source := &bridgeProbeSourceStub{
			instances: []bridgepkg.BridgeInstance{{
				ID:            "brg-slack",
				Platform:      "slack",
				ExtensionName: "slack",
			}},
			response: bridgepkg.BridgeCheckResponse{Checks: []bridgepkg.BridgeCheckRecord{{
				Check:       "provider.identity",
				Status:      bridgepkg.BridgeCheckStatusFail,
				Remediation: "Bind the required bot_token bridge secret and retry.",
			}}},
		}
		otherProbeCalls := 0
		registry := NewRegistry()
		if err := registry.Register(&ProbeFunc{
			ProbeID:       "runtime.providers",
			ProbeCategory: contract.CategoryProvider,
			RunFunc: func(context.Context, *ProbeEnv) ([]contract.DiagnosticItem, error) {
				otherProbeCalls++
				return nil, nil
			},
		}); err != nil {
			t.Fatalf("Register(provider) error = %v", err)
		}
		if err := registry.Register(&BridgeProbe{Source: source}); err != nil {
			t.Fatalf("Register(bridge) error = %v", err)
		}
		runner, err := NewRunner(registry)
		if err != nil {
			t.Fatalf("NewRunner() error = %v", err)
		}

		items, err := runner.Run(context.Background(), RunOptions{Only: []string{contract.CategoryBridge}})
		if err != nil {
			t.Fatalf("Run(bridge only) error = %v", err)
		}
		if otherProbeCalls != 0 {
			t.Fatalf("provider probe calls = %d, want 0", otherProbeCalls)
		}
		if source.listCalls != 1 || source.checkCalls != 1 {
			t.Fatalf("bridge source calls = list:%d check:%d, want 1/1", source.listCalls, source.checkCalls)
		}
		if len(items) != 1 {
			t.Fatalf("bridge diagnostic count = %d, want 1", len(items))
		}
		item := items[0]
		if item.Category != contract.CategoryBridge || item.Severity != contract.SeverityError ||
			!strings.Contains(item.Message, "bot_token") {
			t.Fatalf("bridge diagnostic = %#v, want failed bot_token remediation", item)
		}
		if item.SuggestedCommand != "agh bridge verify brg-slack" {
			t.Fatalf("SuggestedCommand = %q, want bridge verify follow-up", item.SuggestedCommand)
		}
	})

	t.Run("Should never describe a failed check without remediation as passed", func(t *testing.T) {
		t.Parallel()

		item := bridgeCheckDiagnosticItem(
			bridgepkg.BridgeInstance{ID: "brg-slack", Platform: "slack", ExtensionName: "slack"},
			bridgepkg.BridgeCheckRecord{Check: "provider.identity", Status: bridgepkg.BridgeCheckStatusFail},
		)
		if item.Severity != contract.SeverityError || strings.Contains(strings.ToLower(item.Message), "passed") ||
			!strings.Contains(strings.ToLower(item.Message), "no remediation") {
			t.Fatalf("bridge failure diagnostic = %#v, want explicit missing-remediation error", item)
		}
	})
}

type bridgeProbeSourceStub struct {
	instances  []bridgepkg.BridgeInstance
	response   bridgepkg.BridgeCheckResponse
	listCalls  int
	checkCalls int
}

type runtimeMemorySnapshotSourceStub struct {
	snapshot RuntimeMemorySnapshot
}

type subprocessHealthSnapshotSourceStub struct {
	snapshots []session.SubprocessHealthSnapshot
}

func (s runtimeMemorySnapshotSourceStub) RuntimeMemorySnapshot() RuntimeMemorySnapshot {
	return s.snapshot
}

func (s subprocessHealthSnapshotSourceStub) SubprocessHealthSnapshots() []session.SubprocessHealthSnapshot {
	return append([]session.SubprocessHealthSnapshot(nil), s.snapshots...)
}

func (s *bridgeProbeSourceStub) ListInstances(context.Context) ([]bridgepkg.BridgeInstance, error) {
	s.listCalls++
	return append([]bridgepkg.BridgeInstance(nil), s.instances...), nil
}

func (s *bridgeProbeSourceStub) CheckBridge(
	_ context.Context,
	extensionName string,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	s.checkCalls++
	if extensionName != "slack" || request.BridgeInstanceID != "brg-slack" {
		return bridgepkg.BridgeCheckResponse{}, errors.New("unexpected bridge check request")
	}
	return s.response, nil
}
