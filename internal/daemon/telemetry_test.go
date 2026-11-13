package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/udsapi"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/session/sessionid"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/telemetry"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/version"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
)

// stubTelemetryRuntime is a started export with a fixed status.
type stubTelemetryRuntime struct {
	status     telemetry.Status
	shutdownFn func(context.Context) error
}

func (r *stubTelemetryRuntime) Status() telemetry.Status { return r.status }

func (r *stubTelemetryRuntime) ProviderEnv() []string { return nil }

func (r *stubTelemetryRuntime) Shutdown(ctx context.Context) error {
	if r.shutdownFn == nil {
		return nil
	}
	return r.shutdownFn(ctx)
}

// routedServer is a fake server that reports mounted route templates.
type routedServer struct {
	fakeServer
	routes []string
}

func (s *routedServer) RouteTemplates() []string { return s.routes }

func TestBootTelemetryStartsAfterTheServersMountTheirRoutes(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should register mounted routes, ACP methods, hook events, memberships, and owner validators",
		func(t *testing.T) {
			t.Parallel()

			runtime := &stubTelemetryRuntime{status: telemetry.Status{State: telemetry.StateOK}}
			var registries telemetry.Registries
			var refs telemetry.SecretResolver
			d := &Daemon{initTelemetry: func(
				_ context.Context,
				_ compozyconfig.ObservabilityConfig,
				resolver telemetry.SecretResolver,
				reg telemetry.Registries,
				_ version.Info,
				_ *slog.Logger,
			) (telemetry.Runtime, error) {
				registries, refs = reg, resolver
				return runtime, nil
			}}
			state := &bootState{
				logger:     slog.New(slog.DiscardHandler),
				httpServer: &routedServer{routes: []string{"/api/status"}},
				udsServer:  &routedServer{routes: []string{"/api/sessions/:session_id"}},
				telemetry:  &telemetrySlot{},
				cfg: compozyconfig.Config{
					Providers: map[string]compozyconfig.ProviderConfig{"team-proxy": {}},
				},
				agentCatalog:       newResourceCatalog(cloneAgentDef),
				nativeToolProvider: nativeToolListStub{ids: []toolspkg.ToolID{"compozy__session_list"}},
			}
			state.agentCatalog.Replace(1, []resources.Record[compozyconfig.AgentDef]{
				{ID: "reviewer", Spec: compozyconfig.AgentDef{Name: "reviewer"}},
			})
			cleanup := &bootCleanup{}
			// An OK runtime installs the process-wide store write observer.
			t.Cleanup(func() { store.SetWriteObserver(nil) })

			if err := d.bootTelemetry(t.Context(), state, cleanup); err != nil {
				t.Fatalf("bootTelemetry() error = %v", err)
			}
			if want := []string{
				"/api/status",
				"/api/sessions/:session_id",
			}; !slices.Equal(
				registries.RouteTemplates,
				want,
			) {
				t.Fatalf("Registries.RouteTemplates = %v, want %v", registries.RouteTemplates, want)
			}
			if len(registries.HookEvents) != len(hookspkg.AllHookEvents()) ||
				registries.HookEvents[0] != hookspkg.AllHookEvents()[0].String() {
				t.Fatalf("Registries.HookEvents = %v, want the hook event taxonomy", registries.HookEvents)
			}
			assertOwnerValidators(t, registries.IDs)
			if !slices.Equal(registries.ACPMethods, acp.TracedMethods()) {
				t.Fatalf("Registries.ACPMethods = %v, want the traced ACP methods", registries.ACPMethods)
			}
			assertMembership(t, "Providers", registries.Providers, []string{"claude", "team-proxy"}, "/home/ana/agent")
			assertMembership(t, "Agents", registries.Agents, []string{"reviewer"}, "unregistered")
			assertMembership(t, "ToolIDs", registries.ToolIDs, []string{"compozy__session_list"}, "mcp__github__echo")
			if refs != nil {
				t.Fatalf("secret resolver = %T, want none without a provider vault", refs)
			}
			if status, ok := state.telemetry.Status(); !ok || status.State != telemetry.StateOK {
				t.Fatalf("telemetry status = %+v, %t; want the published runtime's status", status, ok)
			}
			if len(cleanup.fns) != 1 {
				t.Fatalf("boot cleanups = %d, want the telemetry shutdown", len(cleanup.fns))
			}
		},
	)

	t.Run("Should refuse to start before the servers mount their routes", func(t *testing.T) {
		t.Parallel()

		d := &Daemon{initTelemetry: func(
			context.Context,
			compozyconfig.ObservabilityConfig,
			telemetry.SecretResolver,
			telemetry.Registries,
			version.Info,
			*slog.Logger,
		) (telemetry.Runtime, error) {
			t.Fatal("initTelemetry called before the servers mounted their routes")
			return nil, errors.New("unreachable")
		}}
		state := &bootState{logger: slog.New(slog.DiscardHandler), telemetry: &telemetrySlot{}}

		if err := d.bootTelemetry(t.Context(), state, &bootCleanup{}); err == nil {
			t.Fatal("bootTelemetry() error = nil, want the server ordering error")
		}
		if _, ok := state.telemetry.Status(); ok {
			t.Fatal("telemetry status published without a runtime")
		}
	})
}

// assertMembership checks that a membership registry admits the registered names and rejects an unknown value.
func assertMembership(t *testing.T, name string, member func(string) bool, registered []string, unknown string) {
	t.Helper()
	if member == nil {
		t.Fatalf("Registries.%s = nil, want a membership registry", name)
	}
	for _, value := range registered {
		if !member(value) {
			t.Fatalf("Registries.%s(%q) = false, want registered", name, value)
		}
	}
	if member(unknown) {
		t.Fatalf("Registries.%s(%q) = true, want unknown", name, unknown)
	}
}

// The tests above fake the servers; this one boots the daemon over the real HTTP and UDS servers, so a broken link in
// the composition root (route registry feed, status source, shutdown publication) fails here instead of staying
// silent: a missing `otel` reads as "unknown", and a flush that never runs loses the final spans.
func TestBootedDaemonWiresTelemetryThroughBothTransports(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should feed mounted routes to the registries, serve status on HTTP and UDS, and flush at shutdown",
		func(t *testing.T) {
			t.Parallel()

			var (
				mu          sync.Mutex
				order       []string
				flushWindow time.Duration
			)
			record := func(step string) func() {
				return func() {
					mu.Lock()
					defer mu.Unlock()
					order = append(order, step)
				}
			}
			runtime := &stubTelemetryRuntime{
				status: telemetry.Status{
					State:        telemetry.StateDegraded,
					Protocol:     "grpc",
					EndpointHost: "collector.local:4317",
					SampleRatio:  0.25,
					Traces:       telemetry.SignalStatus{Enabled: true},
					Metrics:      telemetry.SignalStatus{Enabled: true, LastErrorClass: "http_401", FailedExports: 12},
				},
				shutdownFn: func(ctx context.Context) error {
					record("telemetry")()
					if deadline, ok := ctx.Deadline(); ok {
						mu.Lock()
						flushWindow = time.Until(deadline)
						mu.Unlock()
					}
					return nil
				},
			}

			home := testHomePaths(t)
			cfg := testConfig(t, home)
			d := newTestDaemon(t, home, &cfg)
			d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
				return &fakeSessionManager{}, nil
			}
			d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) { return &fakeObserver{}, nil }
			var registries telemetry.Registries
			d.initTelemetry = func(
				_ context.Context,
				_ compozyconfig.ObservabilityConfig,
				_ telemetry.SecretResolver,
				reg telemetry.Registries,
				_ version.Info,
				_ *slog.Logger,
			) (telemetry.Runtime, error) {
				registries = reg
				return runtime, nil
			}
			engines := make(map[string]*gin.Engine, 2)
			routes := make(map[string][]string, 2)
			d.httpFactory = func(_ context.Context, deps RuntimeDeps) (Server, error) {
				engine := gin.New()
				server, err := httpapi.New(append(httpServerOptions(&deps), httpapi.WithEngine(engine))...)
				if err != nil {
					return nil, err
				}
				engines["HTTP"], routes["HTTP"] = engine, server.RouteTemplates()
				return &routedServer{name: "http", onShutdown: record("http"), routes: routes["HTTP"]}, nil
			}
			d.udsFactory = func(_ context.Context, deps RuntimeDeps) (Server, error) {
				engine := gin.New()
				options := append(
					udsServerOptions(&deps),
					udsapi.WithEngine(engine),
					udsapi.WithSocketPath(shortSocketPath(t)),
				)
				server, err := udsapi.New(options...)
				if err != nil {
					return nil, err
				}
				engines["UDS"], routes["UDS"] = engine, server.RouteTemplates()
				return &routedServer{name: "uds", onShutdown: record("uds"), routes: routes["UDS"]}, nil
			}

			if err := d.boot(t.Context()); err != nil {
				t.Fatalf("boot() error = %v", err)
			}
			t.Cleanup(func() {
				if err := d.Shutdown(context.Background()); err != nil {
					t.Errorf("Shutdown() cleanup error = %v", err)
				}
			})

			const promptRoute = "/api/workspaces/:workspace_id/sessions/:session_id/prompt"
			for transport, templates := range routes {
				if !slices.Contains(templates, "/api/status") || !slices.Contains(templates, promptRoute) {
					t.Fatalf(
						"%s RouteTemplates() = %v, want the mounted /api/status and prompt templates",
						transport,
						templates,
					)
				}
			}
			if want := slices.Concat(routes["HTTP"], routes["UDS"]); !slices.Equal(registries.RouteTemplates, want) {
				t.Fatalf(
					"Registries.RouteTemplates = %v, want both mounted servers' templates %v",
					registries.RouteTemplates,
					want,
				)
			}
			for transport, engine := range engines {
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(
					recorder,
					httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/status", http.NoBody),
				)
				if recorder.Code != http.StatusOK {
					t.Fatalf(
						"%s GET /api/status = %d, want %d; body=%s",
						transport,
						recorder.Code,
						http.StatusOK,
						recorder.Body.String(),
					)
				}
				var status compozycontract.StatusPayload
				if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
					t.Fatalf("%s decode status error = %v; body=%s", transport, err, recorder.Body.String())
				}
				if otel := status.OTel; otel == nil || otel.State != "degraded" || otel.Protocol != "grpc" ||
					otel.EndpointHost != "collector.local:4317" || otel.Metrics.LastErrorClass != "http_401" ||
					otel.Metrics.FailedExports != 12 {
					t.Fatalf("%s status otel = %+v, want the telemetry runtime's status", transport, otel)
				}
			}

			if err := d.Shutdown(t.Context()); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
			mu.Lock()
			defer mu.Unlock()
			if want := []string{"http", "uds", "telemetry"}; !slices.Equal(order, want) {
				t.Fatalf("shutdown order = %v, want the telemetry flush once, after both servers stop: %v", order, want)
			}
			if flushWindow <= 0 || flushWindow > telemetryShutdownTimeout {
				t.Fatalf("telemetry flush window = %s, want at most %s", flushWindow, telemetryShutdownTimeout)
			}
		},
	)
}

func assertOwnerValidators(t *testing.T, ids map[attribute.Key]func(string) bool) {
	t.Helper()
	generated := map[attribute.Key]string{
		telemetry.AttrProfileID:    store.DefaultProfileID,
		telemetry.AttrWorkspaceID:  "ws_0123456789abcdef",
		telemetry.AttrSessionID:    "sess-0123456789abcdef",
		telemetry.AttrTurnID:       "turn-0123456789abcdef",
		telemetry.AttrRunID:        "run-0123456789abcdef",
		telemetry.AttrLoopID:       "looprun-0123456789abcdef",
		telemetry.AttrAutomationID: "job-0123456789abcdef",
	}
	if len(ids) != len(generated) {
		t.Fatalf("Registries.IDs has %d validators, want one per identity attribute", len(ids))
	}
	for key, id := range generated {
		valid := ids[key]
		if valid == nil || !valid(id) || valid("free text/../x") {
			t.Fatalf("Registries.IDs[%s] does not accept %q and reject free text", key, id)
		}
	}
}

// A daemon that exports nothing must not list the model catalog for telemetry.
func TestTelemetryModelClassifierOnlyWhenExportIsRequested(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		obsEnabled  bool
		otelEnabled bool
		traces      bool
		metrics     bool
		want        bool
	}{
		{name: "Should build no classifier when export is disabled", obsEnabled: true, traces: true},
		{name: "Should build no classifier when observability is off", otelEnabled: true, traces: true},
		{name: "Should build no classifier when both signals are off", obsEnabled: true, otelEnabled: true},
		{
			name:       "Should build the classifier for traces only",
			obsEnabled: true, otelEnabled: true, traces: true, want: true,
		},
		{
			name:       "Should build the classifier for metrics only",
			obsEnabled: true, otelEnabled: true, metrics: true, want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			state := &bootState{modelCatalog: &modelCatalogRuntime{}}
			state.cfg.Observability.Enabled = tc.obsEnabled
			state.cfg.Observability.OTel.Enabled = tc.otelEnabled
			state.cfg.Observability.OTel.Traces = tc.traces
			state.cfg.Observability.OTel.Metrics = tc.metrics
			if got := telemetryModelClassifier(state) != nil; got != tc.want {
				t.Fatalf("telemetryModelClassifier() built = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestTelemetryModelClassesFollowTheCatalog(t *testing.T) {
	t.Parallel()

	row := func(provider, model string, kinds ...modelcatalog.SourceKind) modelcatalog.Model {
		sources := make([]modelcatalog.SourceRef, 0, len(kinds))
		for _, kind := range kinds {
			sources = append(sources, modelcatalog.SourceRef{SourceKind: kind})
		}
		return modelcatalog.Model{ProviderID: provider, ModelID: model, Sources: sources}
	}

	t.Run("Should export catalog names, configured models as custom, and the rest as other", func(t *testing.T) {
		t.Parallel()

		catalog := &telemetryModelCatalogStub{models: []modelcatalog.Model{
			row("claude", "claude-sonnet-4-5", modelcatalog.SourceKindBuiltin),
			row("opencode", "org/model", modelcatalog.SourceKindModelsDev, modelcatalog.SourceKindProviderLive),
			row("local", "/home/ana/private/model.gguf", modelcatalog.SourceKindConfig),
			row("claude", "live-only-model", modelcatalog.SourceKindProviderLive),
		}}
		classes := newTestTelemetryModelClasses(catalog, time.Now)
		if got := classes.Classify("claude", "claude-sonnet-4-5"); got != telemetry.ModelUnknown {
			t.Fatalf("Classify() before the first build = %v, want ModelUnknown", got)
		}
		if err := classes.rebuild(t.Context()); err != nil {
			t.Fatalf("rebuild() error = %v", err)
		}

		for _, tc := range []struct {
			provider, model string
			want            telemetry.ModelClass
		}{
			{"claude", "claude-sonnet-4-5", telemetry.ModelCatalog},
			{"opencode", "org/model", telemetry.ModelCatalog},
			{"local", "/home/ana/private/model.gguf", telemetry.ModelCustom},
			{"claude", "live-only-model", telemetry.ModelUnknown},
			{"claude", "my model v2", telemetry.ModelUnknown},
			{"opencode", "claude-sonnet-4-5", telemetry.ModelUnknown},
		} {
			if got := classes.Classify(tc.provider, tc.model); got != tc.want {
				t.Fatalf("Classify(%q, %q) = %v, want %v", tc.provider, tc.model, got, tc.want)
			}
		}
	})

	t.Run("Should rebuild soon after a lookup misses, before the snapshot expires", func(t *testing.T) {
		t.Parallel()

		var now atomic.Pointer[time.Time]
		start := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
		now.Store(&start)
		catalog := &telemetryModelCatalogStub{listed: make(chan struct{}, 4)}
		classes := newTestTelemetryModelClasses(catalog, func() time.Time { return *now.Load() })
		if err := classes.rebuild(t.Context()); err != nil {
			t.Fatalf("rebuild() error = %v", err)
		}
		<-catalog.listed
		catalog.set([]modelcatalog.Model{row("claude", "claude-opus-5-5", modelcatalog.SourceKindModelsDev)})

		classes.Classify("claude", "claude-opus-5-5")
		select {
		case <-catalog.listed:
			t.Fatal("a miss on a fresh snapshot rebuilt it; want rebuilds bounded by telemetryModelMissRebuildAfter")
		default:
		}
		later := start.Add(telemetryModelMissRebuildAfter)
		now.Store(&later)
		classes.Classify("claude", "claude-opus-5-5")
		select {
		case <-catalog.listed:
		case <-time.After(5 * time.Second):
			t.Fatal("a miss on a snapshot older than the miss window did not rebuild")
		}
		deadline := time.Now().Add(5 * time.Second)
		for classes.Classify("claude", "claude-opus-5-5") != telemetry.ModelCatalog {
			if time.Now().After(deadline) {
				t.Fatal("Classify() never saw the catalog row after the miss rebuild")
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	t.Run("Should pick up a new catalog row once the snapshot expires", func(t *testing.T) {
		t.Parallel()

		var now atomic.Pointer[time.Time]
		start := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
		now.Store(&start)
		catalog := &telemetryModelCatalogStub{listed: make(chan struct{}, 4)}
		classes := newTestTelemetryModelClasses(catalog, func() time.Time { return *now.Load() })
		if err := classes.rebuild(t.Context()); err != nil {
			t.Fatalf("rebuild() error = %v", err)
		}
		<-catalog.listed
		catalog.set([]modelcatalog.Model{row("claude", "claude-opus-5", modelcatalog.SourceKindBuiltin)})

		if got := classes.Classify("claude", "claude-opus-5"); got != telemetry.ModelUnknown {
			t.Fatalf("Classify() within the snapshot TTL = %v, want the cached ModelUnknown", got)
		}
		expired := start.Add(telemetryModelSnapshotTTL)
		now.Store(&expired)
		classes.Classify("claude", "claude-opus-5")
		select {
		case <-catalog.listed:
		case <-time.After(5 * time.Second):
			t.Fatal("expired snapshot did not rebuild")
		}
		deadline := time.Now().Add(5 * time.Second)
		for classes.Classify("claude", "claude-opus-5") != telemetry.ModelCatalog {
			if time.Now().After(deadline) {
				t.Fatal("Classify() never saw the new catalog row")
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
}

type telemetryModelCatalogStub struct {
	mu     sync.Mutex
	models []modelcatalog.Model
	listed chan struct{}
}

func (s *telemetryModelCatalogStub) set(models []modelcatalog.Model) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.models = models
}

func (s *telemetryModelCatalogStub) ListModels(
	context.Context,
	modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listed != nil {
		select {
		case s.listed <- struct{}{}:
		default:
		}
	}
	return slices.Clone(s.models), nil
}

func newTestTelemetryModelClasses(catalog telemetryModelCatalog, now func() time.Time) *telemetryModelClasses {
	return &telemetryModelClasses{
		catalog: catalog,
		begin:   func() (func(), bool) { return func() {}, true },
		now:     now,
	}
}

// The turn span exports compozy.session_id and compozy.turn_id only when the owner validators accept them, so the
// synthetic identities the daemon hands the session runtime must keep matching sessionid.
func TestDaemonSyntheticIdentitiesMatchTelemetryValidators(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		id    string
		valid func(string) bool
	}{
		{
			name: "Should accept Loop-bound session IDs",
			id: deterministicBindingValue(
				"sess",
				looppkg.ActionSessionBindRequest{LoopRunID: "looprun-1", NodeID: "n"},
				1,
			),
			valid: sessionid.IsSessionID,
		},
		{
			name:  "Should accept task claim handoff turn IDs",
			id:    taskClaimHandoffTurnID("sess-1", "run-1"),
			valid: sessionid.IsTurnID,
		},
		{
			name:  "Should accept task role activation turn IDs",
			id:    taskRolePromptTurnID("sess-1", "run-1"),
			valid: sessionid.IsTurnID,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !tc.valid(tc.id) {
				t.Fatalf("owner validator rejected daemon identity %q", tc.id)
			}
		})
	}
}

// nativeToolListStub is the daemon's native tool provider as telemetry membership reads it: List only.
type nativeToolListStub struct {
	toolspkg.Provider
	ids []toolspkg.ToolID
}

func (s nativeToolListStub) List(context.Context, toolspkg.Scope) ([]toolspkg.Descriptor, error) {
	descriptors := make([]toolspkg.Descriptor, 0, len(s.ids))
	for _, id := range s.ids {
		descriptors = append(descriptors, toolspkg.Descriptor{ID: id})
	}
	return descriptors, nil
}
