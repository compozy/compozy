package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
	extensionpkg "github.com/compozy/agh/internal/extension"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

var errUnexpectedBridgeRuntimeStoreCall = errors.New("unexpected bridge runtime store stub call")

type recordingBridgeSecretResolver struct {
	values map[string]string
	calls  []bridgepkg.BridgeSecretBinding
	err    error
}

type recordingBridgeSecretRefStore struct {
	values map[string]string
	writes map[string]string
	err    error
}

type bridgeRuntimeStoreStub struct {
	bridgeRuntimeStore
	listBridgeSecretBindingsFn  func(context.Context, string) ([]bridgepkg.BridgeSecretBinding, error)
	putBridgeSecretBindingFn    func(context.Context, bridgepkg.BridgeSecretBinding) error
	deleteBridgeSecretBindingFn func(context.Context, string, string) error
}

type recordingBridgeDeliveryRuntime struct {
	fakeExtensionRuntime

	mu    sync.Mutex
	calls []bridgepkg.DeliveryRequest
}

func (r *recordingBridgeDeliveryRuntime) DeliverBridge(
	_ context.Context,
	_ string,
	req bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	r.mu.Lock()
	r.calls = append(r.calls, req)
	r.mu.Unlock()
	remoteMessageID := ""
	if req.Event.Reference != nil {
		remoteMessageID = req.Event.Reference.RemoteMessageID
	}
	return bridgepkg.DeliveryAck{
		DeliveryID:      req.Event.DeliveryID,
		Seq:             req.Event.Seq,
		RemoteMessageID: remoteMessageID,
	}, nil
}

func (r *recordingBridgeDeliveryRuntime) deliveryCalls() []bridgepkg.DeliveryRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]bridgepkg.DeliveryRequest(nil), r.calls...)
}

func (s bridgeRuntimeStoreStub) ListBridgeSecretBindings(
	ctx context.Context,
	bridgeInstanceID string,
) ([]bridgepkg.BridgeSecretBinding, error) {
	if s.listBridgeSecretBindingsFn != nil {
		return s.listBridgeSecretBindingsFn(ctx, bridgeInstanceID)
	}
	return nil, fmt.Errorf("%w: ListBridgeSecretBindings(%q)", errUnexpectedBridgeRuntimeStoreCall, bridgeInstanceID)
}

func (s bridgeRuntimeStoreStub) PutBridgeSecretBinding(
	ctx context.Context,
	binding bridgepkg.BridgeSecretBinding,
) error {
	if s.putBridgeSecretBindingFn != nil {
		return s.putBridgeSecretBindingFn(ctx, binding)
	}
	return fmt.Errorf(
		"%w: PutBridgeSecretBinding(%q, %q)",
		errUnexpectedBridgeRuntimeStoreCall,
		binding.BridgeInstanceID,
		binding.BindingName,
	)
}

func (s bridgeRuntimeStoreStub) DeleteBridgeSecretBinding(
	ctx context.Context,
	bridgeInstanceID string,
	bindingName string,
) error {
	if s.deleteBridgeSecretBindingFn != nil {
		return s.deleteBridgeSecretBindingFn(ctx, bridgeInstanceID, bindingName)
	}
	return fmt.Errorf(
		"%w: DeleteBridgeSecretBinding(%q, %q)",
		errUnexpectedBridgeRuntimeStoreCall,
		bridgeInstanceID,
		bindingName,
	)
}

func (r *recordingBridgeSecretResolver) ResolveBridgeSecret(
	_ context.Context,
	binding bridgepkg.BridgeSecretBinding,
) (string, error) {
	r.calls = append(r.calls, binding)
	if r.err != nil {
		return "", r.err
	}
	if value, ok := r.values[binding.BindingName]; ok {
		return value, nil
	}
	return "resolved-" + binding.BindingName, nil
}

func (r *recordingBridgeSecretRefStore) ResolveRef(_ context.Context, ref string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if value, ok := r.values[vault.NormalizeRef(ref)]; ok {
		return value, nil
	}
	return "", vault.ErrSecretNotFound
}

func (r *recordingBridgeSecretRefStore) PutSecret(
	_ context.Context,
	ref string,
	_ string,
	plaintext string,
) (vault.Metadata, error) {
	if r.err != nil {
		return vault.Metadata{}, r.err
	}
	normalized := vault.NormalizeRef(ref)
	if r.writes == nil {
		r.writes = make(map[string]string)
	}
	r.writes[normalized] = plaintext
	if r.values == nil {
		r.values = make(map[string]string)
	}
	r.values[normalized] = plaintext
	return vault.Metadata{Ref: normalized, Present: true}, nil
}

func TestComposeBridgeRuntime(t *testing.T) {
	t.Run("ShouldReturnNilWhenRegistryDoesNotSupportBridgePersistence", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		d := newTestDaemon(t, homePaths, &cfg)
		d.now = func() time.Time {
			return time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
		}

		state := &bootState{
			logger:   discardLogger(),
			registry: &recordingRegistry{path: homePaths.DatabaseFile},
		}
		if runtime := d.composeBridgeRuntime(state, &bootCleanup{}); runtime != nil {
			t.Fatalf("composeBridgeRuntime(recordingRegistry) = %#v, want nil", runtime)
		}
	})

	t.Run("ShouldBuildRuntimeWhenRegistrySupportsBridgePersistence", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		d := newTestDaemon(t, homePaths, &cfg)
		d.now = func() time.Time {
			return time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
		}

		db := openDaemonTestGlobalDB(t)
		state := &bootState{
			logger:   discardLogger(),
			registry: db,
		}

		runtime := d.composeBridgeRuntime(state, &bootCleanup{})
		if runtime == nil {
			t.Fatal("composeBridgeRuntime(globaldb) = nil, want non-nil")
			return
		}
		if runtime.Broker() == nil {
			t.Fatal("composeBridgeRuntime(globaldb) broker = nil, want non-nil")
		}
		if runtime.store != db {
			t.Fatalf("composeBridgeRuntime(globaldb) store = %#v, want global db", runtime.store)
		}
		if runtime.registry == nil {
			t.Fatal("composeBridgeRuntime(globaldb) registry = nil, want extension registry")
		}
	})
}

func TestBootBridgeDeliveryReconcileCompletesBeforeRegistration(t *testing.T) {
	t.Parallel()

	t.Run("Should reconcile active rows before opening broker registration", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 12, 18, 0, 0, 0, time.UTC)
		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		instance := bridgepkg.BridgeInstance{
			ID:            "brg-reconcile",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "slack-adapter",
			DisplayName:   "Slack",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		}
		if err := db.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}
		routingKey := bridgepkg.RoutingKey{
			Scope:            instance.Scope,
			WorkspaceID:      instance.WorkspaceID,
			BridgeInstanceID: instance.ID,
			PeerID:           "peer-reconcile",
		}
		if err := db.CreateBridgeDelivery(ctx, bridgepkg.DeliveryLedgerRecord{
			DeliveryID:       "delivery-reconcile",
			SessionID:        "sess-reconcile",
			TurnID:           "turn-reconcile",
			RoutingKey:       routingKey,
			BridgeInstanceID: instance.ID,
			Scope:            instance.Scope,
			WorkspaceID:      instance.WorkspaceID,
			State:            bridgepkg.DeliveryLedgerStateActive,
			CreatedAt:        now.Add(-time.Minute),
			UpdatedAt:        now.Add(-time.Minute),
		}); err != nil {
			t.Fatalf("CreateBridgeDelivery() error = %v", err)
		}
		if err := db.CheckpointBridgeDelivery(ctx, bridgepkg.DeliveryLedgerCheckpoint{
			DeliveryID:      "delivery-reconcile",
			State:           bridgepkg.DeliveryLedgerStateActive,
			LastSentSeq:     3,
			LastAckedSeq:    3,
			RemoteMessageID: "remote-reconcile",
			UpdatedAt:       now.Add(-30 * time.Second),
		}); err != nil {
			t.Fatalf("CheckpointBridgeDelivery() error = %v", err)
		}

		runtime := newBridgeRuntime(
			db,
			discardLogger(),
			func() time.Time { return now },
			nil,
			bridgepkg.WithDeliveryBrokerRegistrationGate(),
		)
		if runtime == nil {
			t.Fatal("newBridgeRuntime() = nil, want durable runtime")
		}
		t.Cleanup(runtime.Close)
		transport := &recordingBridgeDeliveryRuntime{}
		runtime.setExtensionRuntime(transport)

		registration := bridgepkg.PromptDeliveryRegistration{
			SessionID:     "sess-new",
			TurnID:        "turn-new",
			ExtensionName: instance.ExtensionName,
			RoutingKey:    routingKey,
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: instance.ID,
				PeerID:           routingKey.PeerID,
				Mode:             bridgepkg.DeliveryModeReply,
			},
		}
		if _, err := runtime.Broker().RegisterPromptDelivery(ctx, registration); !errors.Is(
			err,
			bridgepkg.ErrDeliveryReconciliationPending,
		) {
			t.Fatalf("RegisterPromptDelivery(before boot reconcile) error = %v, want pending", err)
		}

		d := &Daemon{}
		state := &bootState{bridges: runtime, registry: db, logger: discardLogger()}
		if err := d.bootBridgeDeliveryReconcile(ctx, state); err != nil {
			t.Fatalf("bootBridgeDeliveryReconcile() error = %v", err)
		}
		calls := transport.deliveryCalls()
		if got, want := len(calls), 1; got != want {
			t.Fatalf("reconcile delivery calls = %d, want %d", got, want)
		}
		if calls[0].Event.EventType != bridgepkg.DeliveryEventTypeError || calls[0].Snapshot != nil {
			t.Fatalf("reconcile request = %#v, want fail-open terminal post", calls[0])
		}
		if _, err := runtime.Broker().RegisterPromptDelivery(ctx, registration); err != nil {
			t.Fatalf("RegisterPromptDelivery(after boot reconcile) error = %v", err)
		}
		records, err := db.ListBridgeDeliveries(ctx, bridgepkg.DeliveryLedgerQuery{
			Scope:       instance.Scope,
			WorkspaceID: instance.WorkspaceID,
			State:       bridgepkg.DeliveryLedgerStateTerminalError,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveries(terminal error) error = %v", err)
		}
		if got, want := len(records), 1; got != want {
			t.Fatalf("terminal reconciled records = %d, want %d", got, want)
		}
	})

	t.Run("Should preserve workspace identity across independent reconciliation scopes", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 12, 18, 30, 0, 0, time.UTC)
		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		workspaceIDs := []string{"ws-reconcile-a", "ws-reconcile-b"}
		for _, workspaceID := range workspaceIDs {
			if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
				ID:        workspaceID,
				RootDir:   filepath.Join(t.TempDir(), workspaceID),
				Name:      workspaceID,
				CreatedAt: now.Add(-2 * time.Minute),
				UpdatedAt: now.Add(-2 * time.Minute),
			}); err != nil {
				t.Fatalf("InsertWorkspace(%q) error = %v", workspaceID, err)
			}
			instanceID := "brg-" + workspaceID
			instance := bridgepkg.BridgeInstance{
				ID:            instanceID,
				Scope:         bridgepkg.ScopeWorkspace,
				WorkspaceID:   workspaceID,
				Platform:      "slack",
				ExtensionName: "slack-adapter",
				DisplayName:   workspaceID,
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusReady,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			}
			if err := db.InsertBridgeInstance(ctx, instance); err != nil {
				t.Fatalf("InsertBridgeInstance(%q) error = %v", instanceID, err)
			}
			routingKey := bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      workspaceID,
				BridgeInstanceID: instanceID,
				PeerID:           "peer-" + workspaceID,
			}
			deliveryID := "delivery-" + workspaceID
			if err := db.CreateBridgeDelivery(ctx, bridgepkg.DeliveryLedgerRecord{
				DeliveryID:       deliveryID,
				SessionID:        "session-" + workspaceID,
				TurnID:           "turn-" + workspaceID,
				RoutingKey:       routingKey,
				BridgeInstanceID: instanceID,
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      workspaceID,
				State:            bridgepkg.DeliveryLedgerStateActive,
				CreatedAt:        now.Add(-time.Minute),
				UpdatedAt:        now.Add(-time.Minute),
			}); err != nil {
				t.Fatalf("CreateBridgeDelivery(%q) error = %v", deliveryID, err)
			}
			if err := db.CheckpointBridgeDelivery(ctx, bridgepkg.DeliveryLedgerCheckpoint{
				DeliveryID:      deliveryID,
				State:           bridgepkg.DeliveryLedgerStateActive,
				LastSentSeq:     1,
				LastAckedSeq:    1,
				RemoteMessageID: "remote-" + workspaceID,
				UpdatedAt:       now.Add(-30 * time.Second),
			}); err != nil {
				t.Fatalf("CheckpointBridgeDelivery(%q) error = %v", deliveryID, err)
			}
		}

		runtime := newBridgeRuntime(
			db,
			discardLogger(),
			func() time.Time { return now },
			nil,
			bridgepkg.WithDeliveryBrokerRegistrationGate(),
		)
		if runtime == nil {
			t.Fatal("newBridgeRuntime() = nil, want durable runtime")
		}
		t.Cleanup(runtime.Close)
		transport := &recordingBridgeDeliveryRuntime{}
		runtime.setExtensionRuntime(transport)
		if err := runtime.reconcileBridgeDeliveries(ctx); err != nil {
			t.Fatalf("reconcileBridgeDeliveries() error = %v", err)
		}

		calls := transport.deliveryCalls()
		if got, want := len(calls), len(workspaceIDs); got != want {
			t.Fatalf("reconcile calls = %d, want %d", got, want)
		}
		seen := make(map[string]string, len(calls))
		for _, call := range calls {
			workspaceID := call.Event.RoutingKey.WorkspaceID
			seen[call.Event.DeliveryID] = workspaceID
			if got, want := call.Event.BridgeInstanceID, "brg-"+workspaceID; got != want {
				t.Fatalf("reconciled bridge instance = %q, want %q for workspace %q", got, want, workspaceID)
			}
		}
		for _, workspaceID := range workspaceIDs {
			deliveryID := "delivery-" + workspaceID
			if got, want := seen[deliveryID], workspaceID; got != want {
				t.Fatalf("reconciled workspace for %q = %q, want %q", deliveryID, got, want)
			}
			rows, err := db.ListBridgeDeliveries(ctx, bridgepkg.DeliveryLedgerQuery{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      workspaceID,
				BridgeInstanceID: "brg-" + workspaceID,
				State:            bridgepkg.DeliveryLedgerStateTerminalError,
			})
			if err != nil {
				t.Fatalf("ListBridgeDeliveries(%q) error = %v", workspaceID, err)
			}
			if len(rows) != 1 || rows[0].WorkspaceID != workspaceID {
				t.Fatalf("terminal rows for %q = %#v, want exact workspace row", workspaceID, rows)
			}
		}
	})
}

func TestWithBridgeSecretResolver(t *testing.T) {
	t.Run("ShouldStoreResolverOnDaemon", func(t *testing.T) {
		t.Parallel()

		resolver := &recordingBridgeSecretResolver{}
		d := &Daemon{}

		WithBridgeSecretResolver(resolver)(d)

		if d.bridgeSecretResolver != resolver {
			t.Fatalf("WithBridgeSecretResolver() stored %#v, want %#v", d.bridgeSecretResolver, resolver)
		}
	})
}

func TestDaemonApplyDefaultsBridgeSecretResolver(t *testing.T) {
	t.Run("ShouldLeaveResolverNilWhenUnset", func(t *testing.T) {
		t.Parallel()

		d := &Daemon{}

		d.applyDefaults()
		if d.bridgeSecretResolver != nil {
			t.Fatalf("applyDefaults() bridgeSecretResolver = %#v, want nil", d.bridgeSecretResolver)
		}
	})

	t.Run("ShouldPreserveInjectedResolver", func(t *testing.T) {
		t.Parallel()

		resolver := &recordingBridgeSecretResolver{}
		d := &Daemon{
			getenv:               func(string) string { return "" },
			bridgeSecretResolver: resolver,
		}

		d.applyDefaults()
		if d.bridgeSecretResolver != resolver {
			t.Fatalf("applyDefaults() bridgeSecretResolver = %#v, want %#v", d.bridgeSecretResolver, resolver)
		}
	})
}

func TestVaultBridgeSecretResolver(t *testing.T) {
	t.Run("ShouldValidateResolveAndStoreVaultRefs", func(t *testing.T) {
		t.Parallel()

		store := &recordingBridgeSecretRefStore{
			values: map[string]string{"vault:bridges/brg-vault/bot_token": "resolved-token"},
		}
		resolver := vaultBridgeSecretResolver{service: store}
		binding := bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: "brg-vault",
			BindingName:      "bot_token",
			SecretRef:        " vault:bridges/brg-vault/bot_token ",
			Kind:             "token",
		}

		if err := resolver.ValidateBridgeSecretBinding(binding); err != nil {
			t.Fatalf("ValidateBridgeSecretBinding() error = %v", err)
		}

		value, err := resolver.ResolveBridgeSecret(testutil.Context(t), binding)
		if err != nil {
			t.Fatalf("ResolveBridgeSecret() error = %v", err)
		}
		if got, want := value, "resolved-token"; got != want {
			t.Fatalf("ResolveBridgeSecret() = %q, want %q", got, want)
		}

		if err := resolver.PutBridgeSecretValue(testutil.Context(t), binding, "updated-token"); err != nil {
			t.Fatalf("PutBridgeSecretValue() error = %v", err)
		}
		if got, want := store.writes["vault:bridges/brg-vault/bot_token"], "updated-token"; got != want {
			t.Fatalf("stored bridge secret = %q, want %q", got, want)
		}
	})

	t.Run("ShouldRejectEnvRefs", func(t *testing.T) {
		t.Parallel()

		resolver := vaultBridgeSecretResolver{service: &recordingBridgeSecretRefStore{}}
		binding := bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: "brg-vault-invalid",
			BindingName:      "bot_token",
			SecretRef:        "env:TG_TOKEN",
			Kind:             "token",
		}

		err := resolver.ValidateBridgeSecretBinding(binding)
		if !errors.Is(err, bridgepkg.ErrInvalidBridgeSecretBinding) ||
			!strings.Contains(err.Error(), "unsupported secret ref") {
			t.Fatalf("ValidateBridgeSecretBinding() error = %v, want invalid vault ref validation", err)
		}
	})
}

func TestBootExtensions(t *testing.T) {
	t.Run("ShouldInjectBridgeRuntimeDependencies", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 15, 0, 0, time.UTC)
		bridges := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		if bridges == nil {
			t.Fatal("newBridgeRuntime() = nil, want non-nil")
		}

		manager := &fakeExtensionRuntime{}
		var captured extensionManagerDeps

		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		d := newTestDaemon(t, homePaths, &cfg)
		d.newExtensionManager = func(deps extensionManagerDeps) extensionRuntime {
			captured = deps
			return manager
		}

		state := &bootState{
			logger:            discardLogger(),
			registry:          db,
			sessions:          &fakeSessionManager{},
			observer:          &fakeObserver{},
			workspaceResolver: nil,
			bridges:           bridges,
		}

		if err := d.bootExtensions(testutil.Context(t), state, &bootCleanup{}); err != nil {
			t.Fatalf("bootExtensions() error = %v", err)
		}

		if captured.BridgeRegistry != bridges {
			t.Fatalf("extension deps bridge registry = %#v, want bridge runtime", captured.BridgeRegistry)
		}
		if captured.BridgeDedupStore == nil {
			t.Fatal("extension deps bridge dedup store = nil, want non-nil")
		}
		if captured.BridgeBroker != bridges.Broker() {
			t.Fatalf("extension deps bridge broker = %#v, want runtime broker", captured.BridgeBroker)
		}
		if captured.BridgeRuntime != bridges {
			t.Fatalf("extension deps bridge runtime = %#v, want bridge runtime", captured.BridgeRuntime)
		}
		if manager.startCount != 1 {
			t.Fatalf("extension manager start count = %d, want 1", manager.startCount)
		}
	})
}

func TestBridgeRuntimeStartInstance(t *testing.T) {
	t.Run("ShouldReturnNilRuntimeWhenStoreIsMissing", func(t *testing.T) {
		t.Parallel()

		if runtime := newBridgeRuntime(nil, nil, nil, nil); runtime != nil {
			t.Fatalf("newBridgeRuntime(nil store) = %#v, want nil", runtime)
		}
	})

	t.Run("ShouldHandleNilBrokerAccess", func(t *testing.T) {
		t.Parallel()

		var nilRuntime *bridgeRuntime
		if broker := nilRuntime.Broker(); broker != nil {
			t.Fatalf("(*bridgeRuntime)(nil).Broker() = %#v, want nil", broker)
		}
		nilRuntime.Close()
	})

	t.Run("ShouldTransitionDisabledInstanceToStarting", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		runtime := newBridgeRuntime(db, nil, nil, nil)
		extensions := &fakeExtensionRuntime{}
		runtime.setExtensionRuntime(extensions)

		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-start",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-start",
			DisplayName:   "Start Bridge",
			Enabled:       false,
			Status:        bridgepkg.BridgeStatusDisabled,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})

		updated, err := runtime.StartInstance(testutil.Context(t), instance.ID)
		if err != nil {
			t.Fatalf("StartInstance() error = %v", err)
		}
		if !updated.Enabled {
			t.Fatal("StartInstance() left instance disabled, want enabled")
		}
		if got, want := updated.Status, bridgepkg.BridgeStatusStarting; got != want {
			t.Fatalf("StartInstance().Status = %q, want %q", got, want)
		}
		if got, want := extensions.reloadCount, 1; got != want {
			t.Fatalf("extension reload count = %d, want %d", got, want)
		}
		if runtime.Broker() == nil {
			t.Fatal("newBridgeRuntime() broker = nil, want non-nil")
		}
	})

	t.Run("ShouldRefreshTargetDirectoryOnStaleListAfterSuccessfulStart", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 19, 0, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		extensions := &targetSnapshotExtensionRuntime{
			snapshots: []bridgepkg.BridgeTargetSnapshot{
				{
					CanonicalRoute: "slack:channel:qa-shared",
					DisplayName:    "#qa-shared",
					TargetType:     bridgepkg.BridgeTargetTypeChannel,
					Qualifier:      "slack",
					Capabilities:   []string{string(bridgepkg.DeliveryModeDirectSend)},
				},
			},
		}
		runtime.mu.Lock()
		runtime.extensions = extensions
		runtime.mu.Unlock()

		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-start-targets",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-start-targets",
			DisplayName:   "Start Targets Bridge",
			Enabled:       false,
			Status:        bridgepkg.BridgeStatusDisabled,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})

		if _, err := runtime.StartInstance(testutil.Context(t), instance.ID); err != nil {
			t.Fatalf("StartInstance() error = %v", err)
		}
		if got, want := extensions.reloadCalls, 1; got != want {
			t.Fatalf("reload calls = %d, want %d", got, want)
		}
		if got, want := extensions.snapshotCalls, 0; got != want {
			t.Fatalf("target snapshot calls before list = %d, want %d", got, want)
		}

		result, err := runtime.ListBridgeTargets(testutil.Context(t), bridgepkg.BridgeTargetQuery{
			BridgeID: instance.ID,
		})
		if err != nil {
			t.Fatalf("ListBridgeTargets() error = %v", err)
		}
		if got, want := extensions.snapshotCalls, 1; got != want {
			t.Fatalf("target snapshot calls after stale list = %d, want %d", got, want)
		}
		if got, want := extensions.snapshotExtensionName, "ext-start-targets"; got != want {
			t.Fatalf("snapshot extension name = %q, want %q", got, want)
		}
		if got, want := extensions.snapshotBridgeID, instance.ID; got != want {
			t.Fatalf("snapshot bridge id = %q, want %q", got, want)
		}
		if result.CacheStale {
			t.Fatal("ListBridgeTargets().CacheStale = true, want false after on-demand refresh")
		}
		if got, want := len(result.Items), 1; got != want {
			t.Fatalf("len(ListBridgeTargets().Items) = %d, want %d", got, want)
		}
		if got, want := result.Items[0].CanonicalRoute, "slack:channel:qa-shared"; got != want {
			t.Fatalf("target canonical route = %q, want %q", got, want)
		}
	})

	t.Run("ShouldRefreshTargetDirectoryBeforeResolvingStaleTarget", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 19, 30, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		extensions := &targetSnapshotExtensionRuntime{
			snapshots: []bridgepkg.BridgeTargetSnapshot{
				{
					CanonicalRoute: "slack:channel:qa-shared",
					DisplayName:    "#qa-shared",
					TargetType:     bridgepkg.BridgeTargetTypeChannel,
					Qualifier:      "slack",
					Capabilities:   []string{string(bridgepkg.DeliveryModeDirectSend)},
				},
			},
		}
		runtime.mu.Lock()
		runtime.extensions = extensions
		runtime.mu.Unlock()

		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-resolve-targets",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-resolve-targets",
			DisplayName:   "Resolve Targets Bridge",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})

		result, err := runtime.ResolveBridgeTarget(testutil.Context(t), instance.ID, "qa-shared")
		if err != nil {
			t.Fatalf("ResolveBridgeTarget() error = %v", err)
		}
		if got, want := extensions.snapshotCalls, 1; got != want {
			t.Fatalf("target snapshot calls after stale resolve = %d, want %d", got, want)
		}
		if result.Match == nil {
			t.Fatal("ResolveBridgeTarget().Match = nil, want refreshed match")
		}
		if got, want := result.Match.CanonicalRoute, "slack:channel:qa-shared"; got != want {
			t.Fatalf("resolved canonical route = %q, want %q", got, want)
		}
	})

	t.Run("Should return refresh errors when resolving a stale target directory", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 19, 45, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		extensions := &targetSnapshotExtensionRuntime{
			snapshotErr: errors.New("snapshot failed"),
		}
		runtime.mu.Lock()
		runtime.extensions = extensions
		runtime.mu.Unlock()

		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-resolve-targets-failure",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-resolve-targets-failure",
			DisplayName:   "Resolve Targets Failure Bridge",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})

		_, err := runtime.ResolveBridgeTarget(testutil.Context(t), instance.ID, "qa-shared")
		if err == nil {
			t.Fatal("ResolveBridgeTarget() error = nil, want refresh failure")
		}
		if !strings.Contains(err.Error(), "snapshot failed") {
			t.Fatalf("ResolveBridgeTarget() error = %v, want snapshot failure detail", err)
		}
		if got, want := extensions.snapshotCalls, 1; got != want {
			t.Fatalf("target snapshot calls after stale resolve error = %d, want %d", got, want)
		}
	})
}

func TestBridgeRuntimeCreateInstance(t *testing.T) {
	t.Run("ShouldReloadExtensionsWhenEnabled", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 20, 0, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		extensions := &fakeExtensionRuntime{}
		runtime.setExtensionRuntime(extensions)

		created, err := runtime.CreateInstance(testutil.Context(t), bridgepkg.CreateInstanceRequest{
			ID:            "brg-create",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-create",
			DisplayName:   "Create Bridge",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusStarting,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		if err != nil {
			t.Fatalf("CreateInstance() error = %v", err)
		}
		if created == nil {
			t.Fatal("CreateInstance() = nil, want non-nil")
		}
		if got, want := extensions.reloadCount, 1; got != want {
			t.Fatalf("extension reload count after enabled create = %d, want %d", got, want)
		}
	})

	t.Run("ShouldRollBackToDisabledWhenReloadFails", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 21, 0, 0, time.UTC)
		reloadErr := errors.New("reload boom")
		extensions := &fakeExtensionRuntime{reloadErr: reloadErr}
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		runtime.setExtensionRuntime(extensions)

		_, err := runtime.CreateInstance(testutil.Context(t), bridgepkg.CreateInstanceRequest{
			ID:            "brg-create-rollback",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-create-rollback",
			DisplayName:   "Create Rollback Bridge",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusStarting,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		if !errors.Is(err, reloadErr) {
			t.Fatalf("CreateInstance() error = %v, want wrapped reload failure", err)
		}
		if got, want := extensions.reloadCount, 1; got != want {
			t.Fatalf("extension reload count after failed create = %d, want %d", got, want)
		}

		created, getErr := runtime.GetInstance(testutil.Context(t), "brg-create-rollback")
		if getErr != nil {
			t.Fatalf("GetInstance() error = %v", getErr)
		}
		if created.Enabled {
			t.Fatal("GetInstance() after failed create left instance enabled, want disabled rollback")
		}
		if got, want := created.Status, bridgepkg.BridgeStatusDisabled; got != want {
			t.Fatalf("GetInstance().Status after failed create = %q, want %q", got, want)
		}
	})

	t.Run("ShouldAllowReloadToResolveCurrentManagedInstanceWithoutDeadlock", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 22, 0, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		reload := &resolvingReloadExtensionRuntime{
			runtime:       runtime,
			extensionName: "ext-create-reentrant",
		}
		runtime.setExtensionRuntime(reload)

		done := make(chan struct{})
		var (
			created *bridgepkg.BridgeInstance
			err     error
		)
		go func() {
			created, err = runtime.CreateInstance(testutil.Context(t), bridgepkg.CreateInstanceRequest{
				ID:            "brg-create-reentrant",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "slack",
				ExtensionName: "ext-create-reentrant",
				DisplayName:   "Create Reentrant Bridge",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusStarting,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("CreateInstance() blocked while reload resolved the current managed instance")
		}

		if err != nil {
			t.Fatalf("CreateInstance() error = %v", err)
		}
		if created == nil {
			t.Fatal("CreateInstance() = nil, want non-nil")
		}
		if got, want := reload.reloadCount, 1; got != want {
			t.Fatalf("reload count = %d, want %d", got, want)
		}
		if reload.launch == nil {
			t.Fatal("reload launch = nil, want resolved bridge runtime")
		}
		managed, ok := reload.launch.ManagedInstance(created.ID)
		if !ok {
			t.Fatalf("resolved runtime missing managed instance %q", created.ID)
		}
		if got, want := managed.Instance.ID, created.ID; got != want {
			t.Fatalf("resolved managed instance id = %q, want %q", got, want)
		}
	})
}

func TestBridgeRuntimeCreateInstanceResourceBacked(t *testing.T) {
	t.Run("ShouldRollBackCreatedResourceWhenReconcileFails", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 16, 12, 40, 0, 0, time.UTC)
		triggerErr := errors.New("reconcile boom")
		runtime, resourceStore := newResourceBackedBridgeRuntime(t, now, func(
			context.Context,
			resources.ResourceKind,
			resources.ReconcileReason,
		) error {
			return triggerErr
		})

		_, err := runtime.CreateInstance(testutil.Context(t), bridgepkg.CreateInstanceRequest{
			ID:            "brg-resource-create",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-resource-create",
			DisplayName:   "Resource Create",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusStarting,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		if !errors.Is(err, triggerErr) {
			t.Fatalf("CreateInstance() error = %v, want wrapped reconcile failure", err)
		}

		if _, getErr := resourceStore.Get(
			testutil.Context(t),
			resourceReconcileActor(),
			"brg-resource-create",
		); !errors.Is(
			getErr,
			resources.ErrNotFound,
		) {
			t.Fatalf("resourceStore.Get(after failed create) error = %v, want ErrNotFound", getErr)
		}
		if _, getErr := runtime.GetInstance(testutil.Context(t), "brg-resource-create"); !errors.Is(
			getErr,
			bridgepkg.ErrBridgeInstanceNotFound,
		) {
			t.Fatalf("GetInstance(after failed create) error = %v, want ErrBridgeInstanceNotFound", getErr)
		}
	})
}

func TestBridgeRuntimeListProviders(t *testing.T) {
	t.Run("ShouldProjectInstalledBridgeProvidersFromExtensionRegistry", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 25, 0, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		if runtime == nil {
			t.Fatal("newBridgeRuntime() = nil, want non-nil")
			return
		}
		if runtime.registry == nil {
			t.Fatal("runtime.registry = nil, want extension registry")
		}

		bridgeInfo := mustInstallDaemonExtension(t, runtime.registry, daemonExtensionFixture{
			name:              "telegram-reference",
			description:       "Reference Telegram bridge adapter",
			capabilities:      []string{"bridge.adapter"},
			bridgePlatform:    "telegram",
			bridgeDisplayName: "Telegram",
			bridgeSecretSlots: `
[[bridge.secret_slots]]
name = "bot_token"
description = "Bot API token"
required = true
`,
			bridgeConfigSchema:  "agh.bridge.telegram",
			bridgeConfigVersion: "v1",
			enabled:             true,
		})
		mustInstallDaemonExtension(t, runtime.registry, daemonExtensionFixture{
			name:         "memory-only",
			description:  "Memory backend",
			capabilities: []string{"memory.backend"},
			enabled:      true,
		})

		runtime.setExtensionRuntime(&fakeExtensionRuntime{
			getExt: &extensionpkg.Extension{
				Info: *bridgeInfo,
				Status: extensionpkg.ExtensionStatus{
					Name:          bridgeInfo.Name,
					Version:       bridgeInfo.Version,
					Source:        bridgeInfo.Source,
					Enabled:       bridgeInfo.Enabled,
					Registered:    true,
					Active:        true,
					Healthy:       true,
					HealthMessage: "connected",
					LastStartedAt: now.Add(-time.Minute),
				},
			},
		})

		providers, err := runtime.ListProviders(testutil.Context(t))
		if err != nil {
			t.Fatalf("ListProviders() error = %v", err)
		}
		if got, want := len(providers), 1; got != want {
			t.Fatalf("len(providers) = %d, want %d", got, want)
		}
		if got, want := providers[0].Platform, "telegram"; got != want {
			t.Fatalf("provider platform = %q, want %q", got, want)
		}
		if got, want := providers[0].DisplayName, "Telegram"; got != want {
			t.Fatalf("provider display name = %q, want %q", got, want)
		}
		if got, want := len(providers[0].SecretSlots), 1; got != want {
			t.Fatalf("len(provider secret slots) = %d, want %d", got, want)
		}
		if got, want := providers[0].SecretSlots[0].Name, "bot_token"; got != want {
			t.Fatalf("provider secret slot name = %q, want %q", got, want)
		}
		if providers[0].ConfigSchema == nil {
			t.Fatal("provider config schema = nil, want value")
		}
		if got, want := providers[0].ConfigSchema.Schema, "agh.bridge.telegram"; got != want {
			t.Fatalf("provider config schema id = %q, want %q", got, want)
		}
		if got, want := providers[0].ConfigSchema.Version, "v1"; got != want {
			t.Fatalf("provider config schema version = %q, want %q", got, want)
		}
		if got, want := providers[0].State, "active"; got != want {
			t.Fatalf("provider state = %q, want %q", got, want)
		}
		if got, want := providers[0].Health, "healthy"; got != want {
			t.Fatalf("provider health = %q, want %q", got, want)
		}
		if got, want := providers[0].HealthMessage, "connected"; got != want {
			t.Fatalf("provider health message = %q, want %q", got, want)
		}
	})

	t.Run("ShouldSkipBridgeProvidersWithUnreadableManifestSnapshots", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time {
			return time.Date(2026, 4, 11, 12, 35, 0, 0, time.UTC)
		}, nil)
		if runtime == nil || runtime.registry == nil {
			t.Fatal("newBridgeRuntime() missing registry")
		}

		goodInfo := mustInstallDaemonExtension(t, runtime.registry, daemonExtensionFixture{
			name:              "telegram-reference",
			description:       "Reference Telegram bridge adapter",
			capabilities:      []string{"bridge.adapter"},
			bridgePlatform:    "telegram",
			bridgeDisplayName: "Telegram",
			enabled:           true,
		})
		badInfo := mustInstallDaemonExtension(t, runtime.registry, daemonExtensionFixture{
			name:              "slack-broken",
			description:       "Broken Slack bridge adapter",
			capabilities:      []string{"bridge.adapter"},
			bridgePlatform:    "slack",
			bridgeDisplayName: "Slack",
			enabled:           true,
		})
		if err := os.Remove(badInfo.ManifestPath); err != nil {
			t.Fatalf("os.Remove(%s) error = %v", badInfo.ManifestPath, err)
		}

		providers, err := runtime.ListProviders(testutil.Context(t))
		if err != nil {
			t.Fatalf("ListProviders() error = %v", err)
		}
		if got, want := len(providers), 1; got != want {
			t.Fatalf("len(providers) = %d, want %d", got, want)
		}
		if got, want := providers[0].ExtensionName, goodInfo.Name; got != want {
			t.Fatalf("provider extension_name = %q, want %q", got, want)
		}
	})
}

func TestBridgeRuntimeResolveBridgeRuntime(t *testing.T) {
	t.Run("ShouldResolveBoundSecrets", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 30, 0, 0, time.UTC)
		resolver := &recordingBridgeSecretResolver{
			values: map[string]string{
				"bot_token": "secret-value",
			},
		}

		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, resolver)
		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-secret",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-secret",
			DisplayName:   "Secret Bridge",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		if err := db.PutBridgeSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: instance.ID,
			BindingName:      "bot_token",
			SecretRef:        "vault:bridges/ext-secret/bot-token",
			Kind:             "bot_token",
			CreatedAt:        now,
			UpdatedAt:        now,
		}); err != nil {
			t.Fatalf("PutBridgeSecretBinding() error = %v", err)
		}

		launch, err := runtime.ResolveBridgeRuntime(testutil.Context(t), "ext-secret")
		if err != nil {
			t.Fatalf("ResolveBridgeRuntime() error = %v", err)
		}
		if launch == nil {
			t.Fatal("ResolveBridgeRuntime() = nil, want non-nil")
			return
		}
		managed, ok := launch.ManagedInstance(instance.ID)
		if !ok {
			t.Fatalf("ResolveBridgeRuntime() missing managed instance %q", instance.ID)
		}
		if got, want := managed.Instance.ID, instance.ID; got != want {
			t.Fatalf("ResolveBridgeRuntime().ManagedInstance(%q).Instance.ID = %q, want %q", instance.ID, got, want)
		}
		if got := managed.BoundSecrets; len(got) != 1 || got[0].BindingName != "bot_token" ||
			got[0].Value != "secret-value" {
			t.Fatalf(
				"ResolveBridgeRuntime().ManagedInstance(%q).BoundSecrets = %#v, want resolved bot_token binding",
				instance.ID,
				got,
			)
		}
		if len(resolver.calls) != 1 || resolver.calls[0].BindingName != "bot_token" {
			t.Fatalf("ResolveBridgeSecret() calls = %#v, want bot_token binding", resolver.calls)
		}

		updated, err := runtime.GetInstance(testutil.Context(t), instance.ID)
		if err != nil {
			t.Fatalf("GetInstance() error = %v", err)
		}
		if got, want := updated.Status, bridgepkg.BridgeStatusStarting; got != want {
			t.Fatalf("instance status after launch = %q, want %q", got, want)
		}
	})

	t.Run("ShouldResolveVaultBackedSecrets", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 31, 0, 0, time.UTC)
		refStore := &recordingBridgeSecretRefStore{
			values: map[string]string{
				"vault:bridges/brg-secret-vault/bot_token": "secret-from-vault",
			},
		}
		runtime := newBridgeRuntime(
			db,
			discardLogger(),
			func() time.Time { return now },
			vaultBridgeSecretResolver{service: refStore},
		)
		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-secret-vault",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-secret-vault",
			DisplayName:   "Secret Vault Bridge",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		if err := db.PutBridgeSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: instance.ID,
			BindingName:      "bot_token",
			SecretRef:        "vault:bridges/brg-secret-vault/bot_token",
			Kind:             "bot_token",
			CreatedAt:        now,
			UpdatedAt:        now,
		}); err != nil {
			t.Fatalf("PutBridgeSecretBinding() error = %v", err)
		}

		launch, err := runtime.ResolveBridgeRuntime(testutil.Context(t), instance.ExtensionName)
		if err != nil {
			t.Fatalf("ResolveBridgeRuntime() error = %v", err)
		}

		managed, ok := launch.ManagedInstance(instance.ID)
		if !ok {
			t.Fatalf("ResolveBridgeRuntime() missing managed instance %q", instance.ID)
		}
		if got, want := managed.BoundSecrets[0].Value, "secret-from-vault"; got != want {
			t.Fatalf(
				"ResolveBridgeRuntime().ManagedInstance(%q).BoundSecrets[0].Value = %q, want %q",
				instance.ID,
				got,
				want,
			)
		}
	})

	t.Run("ShouldRequireSecretResolverWhenBindingsExist", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 35, 0, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-secret-missing",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-secret-missing",
			DisplayName:   "Secret Missing",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		if err := db.PutBridgeSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: instance.ID,
			BindingName:      "bot_token",
			SecretRef:        "vault:bridges/ext-secret-missing/bot-token",
			Kind:             "bot_token",
			CreatedAt:        now,
			UpdatedAt:        now,
		}); err != nil {
			t.Fatalf("PutBridgeSecretBinding() error = %v", err)
		}

		_, err := runtime.ResolveBridgeRuntime(testutil.Context(t), instance.ExtensionName)
		if !errors.Is(err, errBridgeSecretResolverRequired) {
			t.Fatalf("ResolveBridgeRuntime() error = %v, want missing secret resolver sentinel", err)
		}
	})

	t.Run("ShouldNotPersistStartingWhenSecretResolutionFails", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 36, 0, 0, time.UTC)
		resolverErr := errors.New("vault boom")
		resolver := &recordingBridgeSecretResolver{err: resolverErr}
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, resolver)

		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-secret-fail",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-secret-fail",
			DisplayName:   "Secret Failure",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		if err := db.PutBridgeSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: instance.ID,
			BindingName:      "bot_token",
			SecretRef:        "vault:bridges/ext-secret-fail/bot-token",
			Kind:             "bot_token",
			CreatedAt:        now,
			UpdatedAt:        now,
		}); err != nil {
			t.Fatalf("PutBridgeSecretBinding() error = %v", err)
		}

		_, err := runtime.ResolveBridgeRuntime(testutil.Context(t), instance.ExtensionName)
		if !errors.Is(err, resolverErr) {
			t.Fatalf("ResolveBridgeRuntime() error = %v, want wrapped resolver error", err)
		}

		updated, getErr := runtime.GetInstance(testutil.Context(t), instance.ID)
		if getErr != nil {
			t.Fatalf("GetInstance() error = %v", getErr)
		}
		if got, want := updated.Status, bridgepkg.BridgeStatusReady; got != want {
			t.Fatalf("instance status after failed secret resolution = %q, want %q", got, want)
		}
	})

	t.Run("ShouldReportMissingVaultSecret", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 36, 30, 0, time.UTC)
		runtime := newBridgeRuntime(
			db,
			discardLogger(),
			func() time.Time { return now },
			vaultBridgeSecretResolver{service: &recordingBridgeSecretRefStore{}},
		)

		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-secret-missing-vault",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-secret-missing-vault",
			DisplayName:   "Secret Missing Vault",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		if err := db.PutBridgeSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: instance.ID,
			BindingName:      "bot_token",
			SecretRef:        "vault:bridges/brg-secret-missing-vault/bot_token",
			Kind:             "bot_token",
			CreatedAt:        now,
			UpdatedAt:        now,
		}); err != nil {
			t.Fatalf("PutBridgeSecretBinding() error = %v", err)
		}

		_, err := runtime.ResolveBridgeRuntime(testutil.Context(t), instance.ExtensionName)
		if !errors.Is(err, bridgepkg.ErrInvalidBridgeSecretBinding) ||
			!strings.Contains(err.Error(), `vault: secret not found`) {
			t.Fatalf("ResolveBridgeRuntime() error = %v, want missing vault error", err)
		}
	})

	t.Run("ShouldResolveMultipleEnabledInstancesForOneExtension", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 37, 0, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)

		first := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-multi-a",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-multi",
			DisplayName:   "Multi A",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		second := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-multi-b",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-multi",
			DisplayName:   "Multi B",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusDegraded,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})

		launch, err := runtime.ResolveBridgeRuntime(testutil.Context(t), "ext-multi")
		if err != nil {
			t.Fatalf("ResolveBridgeRuntime() error = %v", err)
		}
		if launch == nil {
			t.Fatal("ResolveBridgeRuntime() = nil, want non-nil")
			return
		}
		if got, want := launch.ManagedBridgeInstanceIDs(), []string{first.ID, second.ID}; !slices.Equal(got, want) {
			t.Fatalf("ResolveBridgeRuntime().ManagedBridgeInstanceIDs() = %#v, want %#v", got, want)
		}

		for _, instanceID := range []string{first.ID, second.ID} {
			managed, ok := launch.ManagedInstance(instanceID)
			if !ok {
				t.Fatalf("ResolveBridgeRuntime() missing managed instance %q", instanceID)
			}
			if got, want := managed.Instance.Status.Normalize(), bridgecontract.BridgeStatusStarting; got != want {
				t.Fatalf("managed instance %q status = %q, want %q", instanceID, got, want)
			}
		}
	})

	t.Run("ShouldDeferWhenNoEnabledInstanceExistsForExtension", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		runtime := newBridgeRuntime(db, discardLogger(), nil, nil)

		_, err := runtime.ResolveBridgeRuntime(testutil.Context(t), "ext-missing")
		if !errors.Is(err, extensionpkg.ErrBridgeRuntimeDeferred) {
			t.Fatalf("ResolveBridgeRuntime() error = %v, want deferred sentinel", err)
		}
	})
}

func TestBridgeRuntimeSecretBindings(t *testing.T) {
	t.Run("ShouldNormalizeBindingKeysOnWrite", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		refStore := &recordingBridgeSecretRefStore{}
		runtime := newBridgeRuntime(
			db,
			discardLogger(),
			nil,
			vaultBridgeSecretResolver{service: refStore},
		)
		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-secret-binding",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-secret-binding",
			DisplayName:   "Secret Binding",
			Enabled:       false,
			Status:        bridgepkg.BridgeStatusDisabled,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})

		secretValue := "telegram-token"
		err := runtime.PutSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: " " + instance.ID + " ",
			BindingName:      " bot_token ",
			SecretRef:        " vault:bridges/brg-secret-binding/bot_token ",
			Kind:             "token",
		}, &secretValue)
		if err != nil {
			t.Fatalf("PutSecretBinding() error = %v", err)
		}
		if got, want := refStore.writes["vault:bridges/brg-secret-binding/bot_token"], secretValue; got != want {
			t.Fatalf("stored vault secret = %q, want %q", got, want)
		}

		bindings, err := runtime.ListSecretBindings(testutil.Context(t), " "+instance.ID+" ")
		if err != nil {
			t.Fatalf("ListSecretBindings() error = %v", err)
		}
		if got, want := len(bindings), 1; got != want {
			t.Fatalf("len(bindings) = %d, want %d", got, want)
		}
		if got, want := bindings[0].BridgeInstanceID, instance.ID; got != want {
			t.Fatalf("bindings[0].BridgeInstanceID = %q, want %q", got, want)
		}
		if got, want := bindings[0].BindingName, "bot_token"; got != want {
			t.Fatalf("bindings[0].BindingName = %q, want %q", got, want)
		}

		if err := runtime.DeleteSecretBinding(testutil.Context(t), " "+instance.ID+" ", " bot_token "); err != nil {
			t.Fatalf("DeleteSecretBinding() error = %v", err)
		}

		bindings, err = runtime.ListSecretBindings(testutil.Context(t), instance.ID)
		if err != nil {
			t.Fatalf("ListSecretBindings(after delete) error = %v", err)
		}
		if got := len(bindings); got != 0 {
			t.Fatalf("len(bindings after delete) = %d, want 0", got)
		}
	})

	t.Run("ShouldRejectEnvSecretRefs", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		runtime := newBridgeRuntime(
			db,
			discardLogger(),
			nil,
			vaultBridgeSecretResolver{service: &recordingBridgeSecretRefStore{}},
		)
		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-secret-binding-invalid",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-secret-binding-invalid",
			DisplayName:   "Secret Binding Invalid",
			Enabled:       false,
			Status:        bridgepkg.BridgeStatusDisabled,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})

		err := runtime.PutSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: instance.ID,
			BindingName:      "bot_token",
			SecretRef:        "env:TG_TOKEN",
			Kind:             "token",
		}, nil)
		if !errors.Is(err, bridgepkg.ErrInvalidBridgeSecretBinding) ||
			!strings.Contains(err.Error(), "unsupported secret ref") {
			t.Fatalf("PutSecretBinding() error = %v, want invalid bridge ref error", err)
		}
	})

	t.Run("ShouldWrapStoreErrorsWithDaemonContext", func(t *testing.T) {
		t.Parallel()

		listErr := errors.New("list failed")
		putErr := errors.New("put failed")
		deleteErr := errors.New("delete failed")
		runtime := newBridgeRuntime(bridgeRuntimeStoreStub{
			listBridgeSecretBindingsFn: func(context.Context, string) ([]bridgepkg.BridgeSecretBinding, error) {
				return nil, listErr
			},
			putBridgeSecretBindingFn: func(context.Context, bridgepkg.BridgeSecretBinding) error {
				return putErr
			},
			deleteBridgeSecretBindingFn: func(context.Context, string, string) error {
				return deleteErr
			},
		}, discardLogger(), nil, nil)

		_, err := runtime.ListSecretBindings(testutil.Context(t), "brg-1")
		if !errors.Is(err, listErr) || !strings.Contains(err.Error(), "daemon: list bridge secret bindings") {
			t.Fatalf("ListSecretBindings() error = %v, want wrapped list failure", err)
		}
		err = runtime.PutSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: " brg-1 ",
			BindingName:      " token ",
			SecretRef:        "vault:bridges/brg-1/token",
			Kind:             "token",
		}, nil)
		if !errors.Is(err, putErr) || !strings.Contains(err.Error(), "daemon: put bridge secret binding") {
			t.Fatalf("PutSecretBinding() error = %v, want wrapped put failure", err)
		}
		err = runtime.DeleteSecretBinding(testutil.Context(t), " brg-1 ", " token ")
		if !errors.Is(err, deleteErr) || !strings.Contains(err.Error(), "daemon: delete bridge secret binding") {
			t.Fatalf("DeleteSecretBinding() error = %v, want wrapped delete failure", err)
		}
	})
}

func TestBridgeRuntimeStopInstance(t *testing.T) {
	t.Run("ShouldBlockIngressAndPreserveRoutes", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 12, 45, 0, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		extensions := &fakeExtensionRuntime{}
		runtime.setExtensionRuntime(extensions)

		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-stop",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-stop",
			DisplayName:   "Stop Bridge",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		extensions.reloadCount = 0
		route := mustUpsertDaemonBridgeRoute(t, runtime, bridgepkg.BridgeRoute{
			Scope:            bridgepkg.ScopeGlobal,
			BridgeInstanceID: instance.ID,
			PeerID:           "peer-stop",
			SessionID:        "sess-stop",
			AgentName:        "coder",
			LastActivityAt:   now,
		})

		updated, err := runtime.StopInstance(testutil.Context(t), instance.ID)
		if err != nil {
			t.Fatalf("StopInstance() error = %v", err)
		}
		if updated.Enabled {
			t.Fatal("StopInstance() left instance enabled, want disabled")
		}
		if got, want := updated.Status, bridgepkg.BridgeStatusDisabled; got != want {
			t.Fatalf("StopInstance().Status = %q, want %q", got, want)
		}

		routes, err := runtime.ListRoutes(testutil.Context(t), instance.ID)
		if err != nil {
			t.Fatalf("ListRoutes() error = %v", err)
		}
		if len(routes) != 1 || routes[0].RoutingKeyHash != route.RoutingKeyHash {
			t.Fatalf("ListRoutes() = %#v, want preserved route %#v", routes, route)
		}

		if _, err := runtime.ResolveRoute(
			testutil.Context(t),
			route.RoutingKey(),
		); !errors.Is(
			err,
			bridgepkg.ErrBridgeInstanceUnavailable,
		) {
			t.Fatalf("ResolveRoute(disabled) error = %v, want ErrBridgeInstanceUnavailable", err)
		}
	})
}

func TestBridgeRuntimeRestartInstance(t *testing.T) {
	t.Run("Should return state persisted by extension reload", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 16, 13, 5, 0, 0, time.UTC)
		runtime, _ := newResourceBackedBridgeRuntime(t, now, nil)
		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-restart-reloaded-state",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "ext-restart-reloaded-state",
			DisplayName:   "Restart Reloaded State",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		runtime.setExtensionRuntime(&fakeExtensionRuntime{
			onReload: func(ctx context.Context) error {
				_, err := runtime.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
					ID:        instance.ID,
					Enabled:   true,
					Status:    bridgepkg.BridgeStatusAuthRequired,
					UpdatedAt: now.Add(time.Second),
				})
				return err
			},
		})

		updated, err := runtime.RestartInstance(testutil.Context(t), instance.ID)
		if err != nil {
			t.Fatalf("RestartInstance() error = %v", err)
		}
		if got, want := updated.Status, bridgepkg.BridgeStatusAuthRequired; got != want {
			t.Fatalf("RestartInstance().Status = %q, want post-reload %q", got, want)
		}

		persisted, err := runtime.GetInstance(testutil.Context(t), instance.ID)
		if err != nil {
			t.Fatalf("GetInstance() error = %v", err)
		}
		if got, want := persisted.Status, updated.Status; got != want {
			t.Fatalf("GetInstance().Status = %q, want returned status %q", got, want)
		}
	})

	t.Run("ShouldPreserveRoutesAndReloadExtensions", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 13, 0, 0, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		extensions := &fakeExtensionRuntime{}
		runtime.setExtensionRuntime(extensions)

		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-restart",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-restart",
			DisplayName:   "Restart Bridge",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		extensions.reloadCount = 0
		route := mustUpsertDaemonBridgeRoute(t, runtime, bridgepkg.BridgeRoute{
			Scope:            bridgepkg.ScopeGlobal,
			BridgeInstanceID: instance.ID,
			PeerID:           "peer-restart",
			SessionID:        "sess-restart",
			AgentName:        "coder",
			LastActivityAt:   now,
		})

		updated, err := runtime.RestartInstance(testutil.Context(t), instance.ID)
		if err != nil {
			t.Fatalf("RestartInstance() error = %v", err)
		}
		if got, want := extensions.reloadCount, 1; got != want {
			t.Fatalf("extension reload count = %d, want %d", got, want)
		}
		if !updated.Enabled {
			t.Fatal("RestartInstance() disabled the instance, want enabled")
		}
		if got, want := updated.Status, bridgepkg.BridgeStatusStarting; got != want {
			t.Fatalf("RestartInstance().Status = %q, want %q", got, want)
		}

		resolved, err := runtime.ResolveRoute(testutil.Context(t), route.RoutingKey())
		if err != nil {
			t.Fatalf("ResolveRoute(after restart) error = %v", err)
		}
		if got, want := resolved.RoutingKeyHash, route.RoutingKeyHash; got != want {
			t.Fatalf("ResolveRoute(after restart).RoutingKeyHash = %q, want %q", got, want)
		}

		routes, err := runtime.ListRoutes(testutil.Context(t), instance.ID)
		if err != nil {
			t.Fatalf("ListRoutes(after restart) error = %v", err)
		}
		if len(routes) != 1 || routes[0].RoutingKeyHash != route.RoutingKeyHash {
			t.Fatalf("ListRoutes(after restart) = %#v, want preserved route %#v", routes, route)
		}
	})
}

func TestBridgeRuntimeTransition(t *testing.T) {
	t.Run("ShouldRejectTypedNilResourceProjectionPlanWithoutReplacingInstances", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		previous := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-resource-typed-nil",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-resource-typed-nil",
			DisplayName:   "Existing",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			DMPolicy:      bridgepkg.BridgeDMPolicyOpen,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		var plan *bridgepkg.ResourceProjectionPlan

		err := runtime.ApplyBridgeResourceState(testutil.Context(t), plan)
		if err == nil {
			t.Fatal("ApplyBridgeResourceState(typed nil) error = nil, want plan rejection")
		}
		current, err := runtime.GetInstance(testutil.Context(t), previous.ID)
		if err != nil {
			t.Fatalf("GetInstance() error = %v", err)
		}
		if got, want := current.DisplayName, "Existing"; got != want {
			t.Fatalf("GetInstance().DisplayName = %q, want %q", got, want)
		}
	})

	t.Run("ShouldRollBackResourceProjectionWhenReloadFails", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
		previous := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-resource-rollback",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-resource-rollback",
			DisplayName:   "Before",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			DMPolicy:      bridgepkg.BridgeDMPolicyOpen,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})

		reloadErr := errors.New("reload boom")
		extensions := &fakeExtensionRuntime{reloadErr: reloadErr}
		runtime.setExtensionRuntime(extensions)
		plan, err := runtime.BuildBridgeResourceState(
			testutil.Context(t),
			[]resources.Record[bridgepkg.BridgeInstanceSpec]{{
				ID:      previous.ID,
				Version: 2,
				Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				Spec: bridgepkg.BridgeInstanceSpec{
					Scope:         bridgepkg.ScopeGlobal,
					Platform:      "slack",
					ExtensionName: "ext-resource-rollback",
					DisplayName:   "After",
					Source:        bridgepkg.BridgeInstanceSourceDynamic,
					Enabled:       true,
					DMPolicy:      bridgepkg.BridgeDMPolicyOpen,
					RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
				},
				CreatedAt: previous.CreatedAt,
				UpdatedAt: now,
			}},
		)
		if err != nil {
			t.Fatalf("BuildBridgeResourceState() error = %v", err)
		}

		err = runtime.ApplyBridgeResourceState(testutil.Context(t), plan)
		if !errors.Is(err, reloadErr) {
			t.Fatalf("ApplyBridgeResourceState() error = %v, want reload failure", err)
		}
		if got, want := extensions.reloadCount, 1; got != want {
			t.Fatalf("extension reload count = %d, want %d", got, want)
		}
		current, err := runtime.GetInstance(testutil.Context(t), previous.ID)
		if err != nil {
			t.Fatalf("GetInstance() error = %v", err)
		}
		if got, want := current.DisplayName, "Before"; got != want {
			t.Fatalf("GetInstance().DisplayName = %q, want %q", got, want)
		}
		if got, want := current.Status, bridgepkg.BridgeStatusReady; got != want {
			t.Fatalf("GetInstance().Status = %q, want %q", got, want)
		}
	})

	t.Run("ShouldRestorePreviousStateWhenReloadFails", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 11, 13, 5, 0, 0, time.UTC)

		testCases := []struct {
			name       string
			request    bridgepkg.CreateInstanceRequest
			transition func(*bridgeRuntime, context.Context, string) (*bridgepkg.BridgeInstance, error)
			wantState  bridgepkg.BridgeStatus
			wantEnable bool
		}{
			{
				name: "ShouldRollbackStart",
				request: bridgepkg.CreateInstanceRequest{
					ID:            "brg-start-rollback",
					Scope:         bridgepkg.ScopeGlobal,
					Platform:      "slack",
					ExtensionName: "ext-start-rollback",
					DisplayName:   "Start Rollback",
					Enabled:       false,
					Status:        bridgepkg.BridgeStatusDisabled,
					RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
				},
				transition: func(runtime *bridgeRuntime, ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
					return runtime.StartInstance(ctx, id)
				},
				wantState:  bridgepkg.BridgeStatusDisabled,
				wantEnable: false,
			},
			{
				name: "ShouldRollbackStop",
				request: bridgepkg.CreateInstanceRequest{
					ID:            "brg-stop-rollback",
					Scope:         bridgepkg.ScopeGlobal,
					Platform:      "slack",
					ExtensionName: "ext-stop-rollback",
					DisplayName:   "Stop Rollback",
					Enabled:       true,
					Status:        bridgepkg.BridgeStatusReady,
					RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
				},
				transition: func(runtime *bridgeRuntime, ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
					return runtime.StopInstance(ctx, id)
				},
				wantState:  bridgepkg.BridgeStatusReady,
				wantEnable: true,
			},
			{
				name: "ShouldRollbackRestart",
				request: bridgepkg.CreateInstanceRequest{
					ID:            "brg-restart-rollback",
					Scope:         bridgepkg.ScopeGlobal,
					Platform:      "slack",
					ExtensionName: "ext-restart-rollback",
					DisplayName:   "Restart Rollback",
					Enabled:       true,
					Status:        bridgepkg.BridgeStatusReady,
					RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
				},
				transition: func(runtime *bridgeRuntime, ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
					return runtime.RestartInstance(ctx, id)
				},
				wantState:  bridgepkg.BridgeStatusReady,
				wantEnable: true,
			},
		}

		for _, tt := range testCases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				db := openDaemonTestGlobalDB(t)
				reloadErr := errors.New("reload boom")
				runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)

				instance := mustCreateDaemonBridgeInstance(t, runtime, tt.request)
				runtime.setExtensionRuntime(&fakeExtensionRuntime{reloadErr: reloadErr})

				_, err := tt.transition(runtime, testutil.Context(t), instance.ID)
				if !errors.Is(err, reloadErr) {
					t.Fatalf("transition() error = %v, want wrapped reload failure", err)
				}

				updated, getErr := runtime.GetInstance(testutil.Context(t), instance.ID)
				if getErr != nil {
					t.Fatalf("GetInstance() error = %v", getErr)
				}
				if got, want := updated.Enabled, tt.wantEnable; got != want {
					t.Fatalf("GetInstance().Enabled = %t, want %t", got, want)
				}
				if got, want := updated.Status, tt.wantState; got != want {
					t.Fatalf("GetInstance().Status = %q, want %q", got, want)
				}
			})
		}
	})

	t.Run("ShouldSerializeConcurrentLifecycleOperationsDuringReloadRollback", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 13, 10, 0, 0, time.UTC)
		reloadErr := errors.New("reload boom")
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)

		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-race",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-race",
			DisplayName:   "Race Bridge",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		extensions := newBlockingReloadExtensionRuntime(reloadErr)
		runtime.setExtensionRuntime(extensions)

		ctx := testutil.Context(t)
		restartErrCh := make(chan error, 1)
		go func() {
			_, err := runtime.RestartInstance(ctx, instance.ID)
			restartErrCh <- err
		}()

		select {
		case <-extensions.firstStarted:
		case <-time.After(time.Second):
			t.Fatal("RestartInstance() did not enter reload")
		}

		stopErrCh := make(chan error, 1)
		go func() {
			_, err := runtime.StopInstance(ctx, instance.ID)
			stopErrCh <- err
		}()

		select {
		case <-extensions.secondStarted:
			t.Fatal("StopInstance() entered reload before in-flight restart completed")
		case err := <-stopErrCh:
			t.Fatalf("StopInstance() returned before in-flight restart completed: %v", err)
		case <-time.After(200 * time.Millisecond):
		}

		close(extensions.releaseFirst)

		if err := <-restartErrCh; !errors.Is(err, reloadErr) {
			t.Fatalf("RestartInstance() error = %v, want wrapped reload failure", err)
		}
		if err := <-stopErrCh; err != nil {
			t.Fatalf("StopInstance() error = %v", err)
		}

		updated, err := runtime.GetInstance(ctx, instance.ID)
		if err != nil {
			t.Fatalf("GetInstance() error = %v", err)
		}
		if updated.Enabled {
			t.Fatal("GetInstance() after concurrent restart/stop left instance enabled, want disabled")
		}
		if got, want := updated.Status, bridgepkg.BridgeStatusDisabled; got != want {
			t.Fatalf("GetInstance().Status after concurrent restart/stop = %q, want %q", got, want)
		}
	})

	t.Run("ShouldSerializeReloadsAcrossInstancesOfSameExtension", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 4, 11, 13, 12, 0, 0, time.UTC)
		runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)

		first := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-race-a",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-race-shared",
			DisplayName:   "Race Shared A",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		second := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-race-b",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-race-shared",
			DisplayName:   "Race Shared B",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		extensions := newBlockingReloadExtensionRuntime(nil)
		runtime.setExtensionRuntime(extensions)

		ctx := testutil.Context(t)
		firstErrCh := make(chan error, 1)
		go func() {
			_, err := runtime.RestartInstance(ctx, first.ID)
			firstErrCh <- err
		}()

		select {
		case <-extensions.firstStarted:
		case <-time.After(time.Second):
			t.Fatal("RestartInstance(first) did not enter reload")
		}

		secondErrCh := make(chan error, 1)
		go func() {
			_, err := runtime.RestartInstance(ctx, second.ID)
			secondErrCh <- err
		}()

		select {
		case <-extensions.secondStarted:
			t.Fatal("RestartInstance(second) entered reload before same-extension reload completed")
		case err := <-secondErrCh:
			t.Fatalf("RestartInstance(second) returned before same-extension reload completed: %v", err)
		case <-time.After(200 * time.Millisecond):
		}

		close(extensions.releaseFirst)

		if err := <-firstErrCh; err != nil {
			t.Fatalf("RestartInstance(first) error = %v", err)
		}
		if err := <-secondErrCh; err != nil {
			t.Fatalf("RestartInstance(second) error = %v", err)
		}
		if got, want := extensions.reloadCount, 2; got != want {
			t.Fatalf("reload count = %d, want %d", got, want)
		}
	})
}

func TestBridgeRuntimeUpdateInstanceResourceBacked(t *testing.T) {
	t.Run("ShouldRestorePreviousStateWhenReconcileFails", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 16, 12, 50, 0, 0, time.UTC)
		runtime, resourceStore := newResourceBackedBridgeRuntime(t, now, nil)
		created := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-resource-update",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-resource-update",
			DisplayName:   "Before",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		if _, err := runtime.UpdateInstanceState(testutil.Context(t), bridgepkg.UpdateInstanceStateRequest{
			ID:        created.ID,
			Enabled:   true,
			Status:    bridgepkg.BridgeStatusReady,
			UpdatedAt: now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("UpdateInstanceState(setup) error = %v", err)
		}

		triggerErr := errors.New("reconcile boom")
		runtime.resourceTrigger = func(context.Context, resources.ResourceKind, resources.ReconcileReason) error {
			return triggerErr
		}
		updatedName := "After"
		degradation := &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: "token expired",
		}

		_, err := runtime.UpdateInstance(testutil.Context(t), bridgepkg.UpdateInstanceRequest{
			ID:          created.ID,
			DisplayName: &updatedName,
			Degradation: degradation,
		})
		if !errors.Is(err, triggerErr) {
			t.Fatalf("UpdateInstance() error = %v, want wrapped reconcile failure", err)
		}

		record, getErr := resourceStore.Get(testutil.Context(t), resourceReconcileActor(), created.ID)
		if getErr != nil {
			t.Fatalf("resourceStore.Get() error = %v", getErr)
		}
		if got, want := record.Spec.DisplayName, "Before"; got != want {
			t.Fatalf("resourceStore.Get().Spec.DisplayName = %q, want %q", got, want)
		}

		current, getErr := runtime.GetInstance(testutil.Context(t), created.ID)
		if getErr != nil {
			t.Fatalf("GetInstance() error = %v", getErr)
		}
		if got, want := current.DisplayName, "Before"; got != want {
			t.Fatalf("GetInstance().DisplayName = %q, want %q", got, want)
		}
		if got, want := current.Status, bridgepkg.BridgeStatusReady; got != want {
			t.Fatalf("GetInstance().Status = %q, want %q", got, want)
		}
		if current.Degradation != nil {
			t.Fatalf("GetInstance().Degradation = %#v, want nil", current.Degradation)
		}
	})
}

func TestBridgeRuntimeTransitionResourceBacked(t *testing.T) {
	t.Run("Should reload once after projecting the starting state", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 16, 12, 55, 0, 0, time.UTC)
		runtime, _ := newResourceBackedBridgeRuntime(t, now, nil)
		created := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-resource-start",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-resource-start",
			DisplayName:   "Resource Start",
			Enabled:       false,
			Status:        bridgepkg.BridgeStatusDisabled,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		extensions := &fakeExtensionRuntime{
			onReload: func(ctx context.Context) error {
				_, err := runtime.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
					ID:        created.ID,
					Enabled:   true,
					Status:    bridgepkg.BridgeStatusReady,
					UpdatedAt: now.Add(time.Second),
				})
				return err
			},
		}
		runtime.setExtensionRuntime(extensions)

		updated, err := runtime.StartInstance(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("StartInstance() error = %v", err)
		}
		if got, want := extensions.reloadCount, 1; got != want {
			t.Fatalf("extension reload count = %d, want %d", got, want)
		}
		if got, want := updated.Status, bridgepkg.BridgeStatusReady; got != want {
			t.Fatalf("StartInstance().Status = %q, want post-reload %q", got, want)
		}
	})

	t.Run("ShouldRestorePreviousStateWhenReconcileFails", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 16, 13, 0, 0, 0, time.UTC)
		runtime, resourceStore := newResourceBackedBridgeRuntime(t, now, nil)
		created := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-resource-transition",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-resource-transition",
			DisplayName:   "Resource Transition",
			Enabled:       false,
			Status:        bridgepkg.BridgeStatusDisabled,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})

		triggerErr := errors.New("reconcile boom")
		runtime.resourceTrigger = func(context.Context, resources.ResourceKind, resources.ReconcileReason) error {
			return triggerErr
		}

		_, err := runtime.StartInstance(testutil.Context(t), created.ID)
		if !errors.Is(err, triggerErr) {
			t.Fatalf("StartInstance() error = %v, want wrapped reconcile failure", err)
		}

		record, getErr := resourceStore.Get(testutil.Context(t), resourceReconcileActor(), created.ID)
		if getErr != nil {
			t.Fatalf("resourceStore.Get() error = %v", getErr)
		}
		if record.Spec.Enabled {
			t.Fatal("resourceStore.Get().Spec.Enabled = true, want disabled rollback")
		}

		current, getErr := runtime.GetInstance(testutil.Context(t), created.ID)
		if getErr != nil {
			t.Fatalf("GetInstance() error = %v", getErr)
		}
		if current.Enabled {
			t.Fatal("GetInstance().Enabled = true, want disabled rollback")
		}
		if got, want := current.Status, bridgepkg.BridgeStatusDisabled; got != want {
			t.Fatalf("GetInstance().Status = %q, want %q", got, want)
		}
	})
}

func mustCreateDaemonBridgeInstance(
	t *testing.T,
	runtime *bridgeRuntime,
	req bridgepkg.CreateInstanceRequest,
) *bridgepkg.BridgeInstance {
	t.Helper()

	instance, err := runtime.CreateInstance(testutil.Context(t), req)
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}
	return instance
}

func newResourceBackedBridgeRuntime(
	t *testing.T,
	now time.Time,
	trigger func(context.Context, resources.ResourceKind, resources.ReconcileReason) error,
) (*bridgeRuntime, resources.Store[bridgepkg.BridgeInstanceSpec]) {
	t.Helper()

	db := openDaemonTestGlobalDB(t)
	runtime := newBridgeRuntime(db, discardLogger(), func() time.Time { return now }, nil)
	kernel, err := resources.NewKernel(db.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	codecs := resources.NewCodecRegistry()
	codec, err := bridgepkg.NewBridgeInstanceResourceCodec(nil)
	if err != nil {
		t.Fatalf("NewBridgeInstanceResourceCodec() error = %v", err)
	}
	if err := resources.RegisterCodec(codecs, codec); err != nil {
		t.Fatalf("RegisterCodec() error = %v", err)
	}
	resourceStore, err := bridgeInstanceResourceStore(kernel, codecs)
	if err != nil {
		t.Fatalf("bridgeInstanceResourceStore() error = %v", err)
	}
	runtime.setResourceDefinitions(resourceStore, resourceReconcileActor(), trigger)
	return runtime, resourceStore
}

func mustUpsertDaemonBridgeRoute(
	t *testing.T,
	runtime *bridgeRuntime,
	route bridgepkg.BridgeRoute,
) *bridgepkg.BridgeRoute {
	t.Helper()

	resolved, err := runtime.UpsertRoute(testutil.Context(t), route)
	if err != nil {
		t.Fatalf("UpsertRoute() error = %v", err)
	}
	return resolved
}

type blockingReloadExtensionRuntime struct {
	mu             sync.Mutex
	reloadCount    int
	reloadErr      error
	firstStarted   chan struct{}
	secondStarted  chan struct{}
	releaseFirst   chan struct{}
	firstStartOnce sync.Once
}

type resolvingReloadExtensionRuntime struct {
	runtime       *bridgeRuntime
	extensionName string
	reloadCount   int
	launch        *subprocess.InitializeBridgeRuntime
}

type targetSnapshotExtensionRuntime struct {
	fakeExtensionRuntime
	reloadCalls           int
	snapshotCalls         int
	snapshotExtensionName string
	snapshotBridgeID      string
	snapshots             []bridgepkg.BridgeTargetSnapshot
	snapshotErr           error
}

func newBlockingReloadExtensionRuntime(reloadErr error) *blockingReloadExtensionRuntime {
	return &blockingReloadExtensionRuntime{
		reloadErr:     reloadErr,
		firstStarted:  make(chan struct{}),
		secondStarted: make(chan struct{}),
		releaseFirst:  make(chan struct{}),
	}
}

func (r *resolvingReloadExtensionRuntime) Start(context.Context) error {
	return nil
}

func (r *resolvingReloadExtensionRuntime) Stop(context.Context) error {
	return nil
}

func (r *resolvingReloadExtensionRuntime) Reload(ctx context.Context) error {
	r.reloadCount++
	launch, err := r.runtime.ResolveBridgeRuntime(ctx, r.extensionName)
	if err != nil {
		return err
	}
	r.launch = launch
	return nil
}

func (r *resolvingReloadExtensionRuntime) Get(string) (*extensionpkg.Extension, error) {
	return nil, nil
}

func (r *resolvingReloadExtensionRuntime) HookDeclarations(context.Context) ([]hookspkg.HookDecl, error) {
	return nil, nil
}

func (r *targetSnapshotExtensionRuntime) Reload(context.Context) error {
	r.reloadCalls++
	return r.reloadErr
}

func (r *targetSnapshotExtensionRuntime) BridgeTargetSnapshots(
	ctx context.Context,
	extensionName string,
	req bridgepkg.BridgeTargetSnapshotRequest,
) ([]bridgepkg.BridgeTargetSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	r.snapshotCalls++
	r.snapshotExtensionName = extensionName
	r.snapshotBridgeID = req.BridgeInstanceID
	if r.snapshotErr != nil {
		return nil, r.snapshotErr
	}
	return slices.Clone(r.snapshots), nil
}

type daemonExtensionFixture struct {
	name                string
	description         string
	capabilities        []string
	bridgePlatform      string
	bridgeDisplayName   string
	bridgeSecretSlots   string
	bridgeConfigSchema  string
	bridgeConfigVersion string
	enabled             bool
}

func mustInstallDaemonExtension(
	t *testing.T,
	registry *extensionpkg.Registry,
	fixture daemonExtensionFixture,
) *extensionpkg.ExtensionInfo {
	t.Helper()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "extension.toml")
	if err := os.WriteFile(manifestPath, []byte(daemonExtensionManifest(fixture)), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", manifestPath, err)
	}

	manifest, err := extensionpkg.LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest(%s) error = %v", dir, err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%s) error = %v", dir, err)
	}
	if err := registry.Install(manifest, dir, checksum); err != nil {
		t.Fatalf("Install(%s) error = %v", fixture.name, err)
	}
	if !fixture.enabled {
		if err := registry.Disable(fixture.name); err != nil {
			t.Fatalf("Disable(%s) error = %v", fixture.name, err)
		}
	}

	info, err := registry.Get(fixture.name)
	if err != nil {
		t.Fatalf("Get(%s) error = %v", fixture.name, err)
	}
	return info
}

func daemonExtensionManifest(fixture daemonExtensionFixture) string {
	var builder strings.Builder

	fmt.Fprintf(
		&builder,
		"[extension]\nname = %q\nversion = \"0.1.0\"\ndescription = %q\nmin_agh_version = \"0.5.0\"\n\n",
		fixture.name,
		fixture.description,
	)
	if len(fixture.capabilities) > 0 {
		fmt.Fprintf(&builder, "[capabilities]\nprovides = [%s]\n\n", quotedStringList(fixture.capabilities))
	}
	if fixture.bridgePlatform != "" || fixture.bridgeDisplayName != "" {
		fmt.Fprintf(
			&builder,
			"[bridge]\nplatform = %q\ndisplay_name = %q\n",
			fixture.bridgePlatform,
			fixture.bridgeDisplayName,
		)
		if fixture.bridgeSecretSlots != "" {
			builder.WriteString(fixture.bridgeSecretSlots)
		}
		if fixture.bridgeConfigSchema != "" || fixture.bridgeConfigVersion != "" {
			fmt.Fprintf(
				&builder,
				"\n[bridge.config_schema]\nschema = %q\nversion = %q\n",
				fixture.bridgeConfigSchema,
				fixture.bridgeConfigVersion,
			)
		}
	}
	return builder.String()
}

func quotedStringList(values []string) string {
	if len(values) == 0 {
		return ""
	}

	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return strings.Join(quoted, ", ")
}

func (r *blockingReloadExtensionRuntime) Start(context.Context) error {
	return nil
}

func (r *blockingReloadExtensionRuntime) Stop(context.Context) error {
	return nil
}

func (r *blockingReloadExtensionRuntime) Reload(ctx context.Context) error {
	r.mu.Lock()
	r.reloadCount++
	call := r.reloadCount
	r.mu.Unlock()

	switch call {
	case 1:
		r.firstStartOnce.Do(func() {
			close(r.firstStarted)
		})
		select {
		case <-r.releaseFirst:
		case <-ctx.Done():
			return ctx.Err()
		}
		return r.reloadErr
	case 2:
		close(r.secondStarted)
	}

	return nil
}

func (r *blockingReloadExtensionRuntime) Get(string) (*extensionpkg.Extension, error) {
	return nil, nil
}

func (r *blockingReloadExtensionRuntime) HookDeclarations(context.Context) ([]hookspkg.HookDecl, error) {
	return nil, nil
}
