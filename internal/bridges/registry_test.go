package bridges_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/testutil"
)

type stubRegistryStore struct {
	insertBridgeInstanceFn     func(context.Context, bridgepkg.BridgeInstance) error
	updateBridgeInstanceFn     func(context.Context, bridgepkg.BridgeInstance) error
	deleteBridgeInstanceFn     func(context.Context, string) error
	getBridgeInstanceFn        func(context.Context, string) (bridgepkg.BridgeInstance, error)
	listBridgeInstancesFn      func(context.Context) ([]bridgepkg.BridgeInstance, error)
	listBridgeCatalogRecordsFn func(
		context.Context,
		bridgepkg.BridgeCatalogQuery,
	) ([]bridgepkg.BridgeCatalogRecord, error)
	listBridgeInstancesByIDsFn func(
		context.Context,
		[]string,
	) ([]bridgepkg.BridgeInstance, error)
	putBridgeRouteFn     func(context.Context, bridgepkg.BridgeRoute) error
	resolveBridgeRouteFn func(context.Context, bridgepkg.RoutingKey) (bridgepkg.BridgeRoute, error)
	listBridgeRoutesFn   func(context.Context, string) ([]bridgepkg.BridgeRoute, error)
}

func (s stubRegistryStore) InsertBridgeInstance(ctx context.Context, instance bridgepkg.BridgeInstance) error {
	if s.insertBridgeInstanceFn != nil {
		return s.insertBridgeInstanceFn(ctx, instance)
	}
	return nil
}

func (s stubRegistryStore) UpdateBridgeInstance(ctx context.Context, instance bridgepkg.BridgeInstance) error {
	if s.updateBridgeInstanceFn != nil {
		return s.updateBridgeInstanceFn(ctx, instance)
	}
	return nil
}

func (s stubRegistryStore) DeleteBridgeInstance(ctx context.Context, id string) error {
	if s.deleteBridgeInstanceFn != nil {
		return s.deleteBridgeInstanceFn(ctx, id)
	}
	return nil
}

func (s stubRegistryStore) GetBridgeInstance(ctx context.Context, id string) (bridgepkg.BridgeInstance, error) {
	if s.getBridgeInstanceFn != nil {
		return s.getBridgeInstanceFn(ctx, id)
	}
	return bridgepkg.BridgeInstance{}, bridgepkg.ErrBridgeInstanceNotFound
}

func (s stubRegistryStore) ListBridgeInstances(ctx context.Context) ([]bridgepkg.BridgeInstance, error) {
	if s.listBridgeInstancesFn != nil {
		return s.listBridgeInstancesFn(ctx)
	}
	return nil, nil
}

func (s stubRegistryStore) ListBridgeCatalogRecords(
	ctx context.Context,
	query bridgepkg.BridgeCatalogQuery,
) ([]bridgepkg.BridgeCatalogRecord, error) {
	if s.listBridgeCatalogRecordsFn != nil {
		return s.listBridgeCatalogRecordsFn(ctx, query)
	}
	return nil, nil
}

func (s stubRegistryStore) ListBridgeInstancesByIDs(
	ctx context.Context,
	bridgeInstanceIDs []string,
) ([]bridgepkg.BridgeInstance, error) {
	if s.listBridgeInstancesByIDsFn != nil {
		return s.listBridgeInstancesByIDsFn(ctx, bridgeInstanceIDs)
	}
	return nil, nil
}

func (s stubRegistryStore) PutBridgeRoute(ctx context.Context, route bridgepkg.BridgeRoute) error {
	if s.putBridgeRouteFn != nil {
		return s.putBridgeRouteFn(ctx, route)
	}
	return nil
}

func (s stubRegistryStore) ResolveBridgeRoute(
	ctx context.Context,
	key bridgepkg.RoutingKey,
) (bridgepkg.BridgeRoute, error) {
	if s.resolveBridgeRouteFn != nil {
		return s.resolveBridgeRouteFn(ctx, key)
	}
	return bridgepkg.BridgeRoute{}, bridgepkg.ErrBridgeRouteNotFound
}

func (s stubRegistryStore) ListBridgeRoutes(
	ctx context.Context,
	bridgeInstanceID string,
) ([]bridgepkg.BridgeRoute, error) {
	if s.listBridgeRoutesFn != nil {
		return s.listBridgeRoutesFn(ctx, bridgeInstanceID)
	}
	return nil, nil
}

type memoryRegistryStore struct {
	mu         sync.RWMutex
	instances  map[string]bridgepkg.BridgeInstance
	routes     map[string]bridgepkg.BridgeRoute
	workspaces map[string]struct{}
}

func newMemoryRegistryStore() *memoryRegistryStore {
	return &memoryRegistryStore{
		instances:  make(map[string]bridgepkg.BridgeInstance),
		routes:     make(map[string]bridgepkg.BridgeRoute),
		workspaces: make(map[string]struct{}),
	}
}

func (s *memoryRegistryStore) InsertBridgeInstance(_ context.Context, instance bridgepkg.BridgeInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureWorkspaceLocked(instance.Scope, instance.WorkspaceID); err != nil {
		return err
	}
	s.instances[instance.ID] = cloneBridgeInstanceForTest(instance)
	return nil
}

func (s *memoryRegistryStore) UpdateBridgeInstance(_ context.Context, instance bridgepkg.BridgeInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureWorkspaceLocked(instance.Scope, instance.WorkspaceID); err != nil {
		return err
	}
	if _, ok := s.instances[instance.ID]; !ok {
		return bridgepkg.ErrBridgeInstanceNotFound
	}
	s.instances[instance.ID] = cloneBridgeInstanceForTest(instance)
	return nil
}

func (s *memoryRegistryStore) GetBridgeInstance(_ context.Context, id string) (bridgepkg.BridgeInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instance, ok := s.instances[id]
	if !ok {
		return bridgepkg.BridgeInstance{}, bridgepkg.ErrBridgeInstanceNotFound
	}
	return cloneBridgeInstanceForTest(instance), nil
}

func (s *memoryRegistryStore) ListBridgeInstances(_ context.Context) ([]bridgepkg.BridgeInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instances := make([]bridgepkg.BridgeInstance, 0, len(s.instances))
	for _, instance := range s.instances {
		instances = append(instances, cloneBridgeInstanceForTest(instance))
	}
	return instances, nil
}

func (s *memoryRegistryStore) ListBridgeCatalogRecords(
	_ context.Context,
	_ bridgepkg.BridgeCatalogQuery,
) ([]bridgepkg.BridgeCatalogRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]bridgepkg.BridgeCatalogRecord, 0, len(s.instances))
	for _, instance := range s.instances {
		records = append(records, bridgepkg.BridgeCatalogRecordFromInstance(instance))
	}
	return records, nil
}

func (s *memoryRegistryStore) ListBridgeInstancesByIDs(
	_ context.Context,
	bridgeInstanceIDs []string,
) ([]bridgepkg.BridgeInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instances := make([]bridgepkg.BridgeInstance, 0, len(bridgeInstanceIDs))
	for _, id := range bridgeInstanceIDs {
		if instance, ok := s.instances[id]; ok {
			instances = append(instances, cloneBridgeInstanceForTest(instance))
		}
	}
	return instances, nil
}

func (s *memoryRegistryStore) PutBridgeRoute(_ context.Context, route bridgepkg.BridgeRoute) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.routes[route.RoutingKeyHash] = cloneBridgeRouteForTest(route)
	return nil
}

func (s *memoryRegistryStore) ResolveBridgeRoute(
	_ context.Context,
	key bridgepkg.RoutingKey,
) (bridgepkg.BridgeRoute, error) {
	hash, err := key.Hash()
	if err != nil {
		return bridgepkg.BridgeRoute{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	route, ok := s.routes[hash]
	if !ok {
		return bridgepkg.BridgeRoute{}, bridgepkg.ErrBridgeRouteNotFound
	}
	return cloneBridgeRouteForTest(route), nil
}

func (s *memoryRegistryStore) ListBridgeRoutes(
	_ context.Context,
	bridgeInstanceID string,
) ([]bridgepkg.BridgeRoute, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routes := make([]bridgepkg.BridgeRoute, 0, len(s.routes))
	for _, route := range s.routes {
		if route.BridgeInstanceID == bridgeInstanceID {
			routes = append(routes, cloneBridgeRouteForTest(route))
		}
	}
	return routes, nil
}

func (s *memoryRegistryStore) InsertWorkspace(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workspaces[id] = struct{}{}
}

func (s *memoryRegistryStore) ensureWorkspaceLocked(scope bridgepkg.Scope, workspaceID string) error {
	if scope != bridgepkg.ScopeWorkspace {
		return nil
	}
	if _, ok := s.workspaces[workspaceID]; !ok {
		return errors.New("workspace not found")
	}
	return nil
}

func cloneBridgeInstanceForTest(instance bridgepkg.BridgeInstance) bridgepkg.BridgeInstance {
	cloned := instance
	cloned.ProviderConfig = append(json.RawMessage(nil), instance.ProviderConfig...)
	cloned.DeliveryDefaults = append(json.RawMessage(nil), instance.DeliveryDefaults...)
	if instance.Degradation != nil {
		degradation := *instance.Degradation
		cloned.Degradation = &degradation
	}
	return cloned
}

func cloneBridgeRouteForTest(route bridgepkg.BridgeRoute) bridgepkg.BridgeRoute {
	return route
}

func TestValidateInstanceStateTransitionRejectsReadyFromDisabledWithoutEnablePath(t *testing.T) {
	t.Parallel()

	current := bridgepkg.BridgeInstance{
		ID:            "brg-disabled",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Disabled",
		Enabled:       false,
		Status:        bridgepkg.BridgeStatusDisabled,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	}

	err := bridgepkg.ValidateInstanceStateTransition(current, true, bridgepkg.BridgeStatusReady)
	if !errors.Is(err, bridgepkg.ErrInvalidBridgeStateTransition) {
		t.Fatalf("ValidateInstanceStateTransition() error = %v, want ErrInvalidBridgeStateTransition", err)
	}
}

func TestPlatformDimensionMappingValidate(t *testing.T) {
	t.Parallel()

	mapping := bridgepkg.PlatformDimensionMapping{
		Platform:        "telegram",
		PeerIDConcept:   "chat or user id",
		ThreadIDConcept: "forum topic id",
		GroupIDConcept:  "group or bridge id",
	}
	if err := mapping.Validate(); err != nil {
		t.Fatalf("PlatformDimensionMapping.Validate(valid) error = %v", err)
	}

	if err := (bridgepkg.PlatformDimensionMapping{Platform: "telegram"}).Validate(); err == nil {
		t.Fatal("PlatformDimensionMapping.Validate(no concepts) error = nil, want non-nil")
	}
}

func TestCanonicalizeRoutingKeyRejectsBaseMismatch(t *testing.T) {
	t.Parallel()

	instance := bridgepkg.BridgeInstance{
		ID:            "brg-base",
		Scope:         bridgepkg.ScopeWorkspace,
		WorkspaceID:   "ws-1",
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Base",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	}

	_, err := bridgepkg.CanonicalizeRoutingKey(instance, bridgepkg.RoutingKey{
		Scope:            bridgepkg.ScopeGlobal,
		WorkspaceID:      "ws-2",
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
	})
	if err == nil {
		t.Fatal("CanonicalizeRoutingKey() error = nil, want non-nil")
	}
}

func TestValidateInstanceStateTransitionAllowedPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		current     bridgepkg.BridgeInstance
		nextEnabled bool
		nextStatus  bridgepkg.BridgeStatus
	}{
		{
			name: "starting to ready",
			current: bridgepkg.BridgeInstance{
				ID:            "brg-starting",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "telegram-adapter",
				DisplayName:   "Starting",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusStarting,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			},
			nextEnabled: true,
			nextStatus:  bridgepkg.BridgeStatusReady,
		},
		{
			name: "ready to starting",
			current: bridgepkg.BridgeInstance{
				ID:            "brg-ready",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "telegram-adapter",
				DisplayName:   "Ready",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusReady,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			},
			nextEnabled: true,
			nextStatus:  bridgepkg.BridgeStatusStarting,
		},
		{
			name: "degraded to ready",
			current: bridgepkg.BridgeInstance{
				ID:            "brg-degraded",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "telegram-adapter",
				DisplayName:   "Degraded",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusDegraded,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			},
			nextEnabled: true,
			nextStatus:  bridgepkg.BridgeStatusReady,
		},
		{
			name: "auth required to starting",
			current: bridgepkg.BridgeInstance{
				ID:            "brg-auth",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "telegram-adapter",
				DisplayName:   "Auth",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusAuthRequired,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			},
			nextEnabled: true,
			nextStatus:  bridgepkg.BridgeStatusStarting,
		},
		{
			name: "error to starting",
			current: bridgepkg.BridgeInstance{
				ID:            "brg-error",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "telegram-adapter",
				DisplayName:   "Error",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusError,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			},
			nextEnabled: true,
			nextStatus:  bridgepkg.BridgeStatusStarting,
		},
		{
			name: "ready to disabled",
			current: bridgepkg.BridgeInstance{
				ID:            "brg-disable",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "telegram-adapter",
				DisplayName:   "Disable",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusReady,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			},
			nextEnabled: false,
			nextStatus:  bridgepkg.BridgeStatusDisabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := bridgepkg.ValidateInstanceStateTransition(tt.current, tt.nextEnabled, tt.nextStatus); err != nil {
				t.Fatalf("ValidateInstanceStateTransition() error = %v", err)
			}
		})
	}
}

func TestCreateInstanceRequestValidate(t *testing.T) {
	t.Parallel()

	req := bridgepkg.CreateInstanceRequest{
		Scope:          bridgepkg.ScopeGlobal,
		Platform:       "telegram",
		ExtensionName:  "telegram-adapter",
		DisplayName:    "Validate",
		Enabled:        true,
		Status:         bridgepkg.BridgeStatusStarting,
		DMPolicy:       bridgepkg.BridgeDMPolicyPairing,
		RoutingPolicy:  bridgepkg.RoutingPolicy{IncludePeer: true},
		ProviderConfig: json.RawMessage(`{"mode":"bot"}`),
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("CreateInstanceRequest.Validate() error = %v", err)
	}

	req.DeliveryDefaults = json.RawMessage(`{"thread_id":{"nested":"bad"}}`)
	if err := req.Validate(); err == nil {
		t.Fatal("CreateInstanceRequest.Validate() error = nil, want invalid delivery defaults failure")
	}
}

func TestUpdateInstanceRequestValidate(t *testing.T) {
	t.Parallel()

	displayName := "Updated"
	dmPolicy := bridgepkg.BridgeDMPolicyAllowlist
	providerConfig := json.RawMessage(`{"mode":"bot","tenant":"ws-alpha"}`)
	deliveryDefaults := json.RawMessage(`{"peer_id":"peer-default","mode":"reply"}`)
	req := bridgepkg.UpdateInstanceRequest{
		ID:               "brg-update",
		DisplayName:      &displayName,
		DMPolicy:         &dmPolicy,
		RoutingPolicy:    &bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
		ProviderConfig:   &providerConfig,
		DeliveryDefaults: &deliveryDefaults,
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("UpdateInstanceRequest.Validate() error = %v", err)
	}

	invalidDeliveryDefaults := json.RawMessage(`{"mode":true}`)
	req.DeliveryDefaults = &invalidDeliveryDefaults
	if err := req.Validate(); err == nil {
		t.Fatal("UpdateInstanceRequest.Validate() error = nil, want invalid delivery defaults failure")
	}
}

func TestRegistryCreateGetAndUpdateInstanceState(t *testing.T) {
	t.Parallel()

	registry, _ := newRegistryTestHarness(t)
	created, err := registry.CreateInstance(testutil.Context(t), bridgepkg.CreateInstanceRequest{
		ID:            "brg-state",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Lifecycle",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusStarting,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}
	if created.Status != bridgepkg.BridgeStatusStarting {
		t.Fatalf("CreateInstance().Status = %q, want starting", created.Status)
	}

	loaded, err := registry.GetInstance(testutil.Context(t), created.ID)
	if err != nil {
		t.Fatalf("GetInstance() error = %v", err)
	}
	if loaded.ID != created.ID {
		t.Fatalf("GetInstance().ID = %q, want %q", loaded.ID, created.ID)
	}

	updated, err := registry.UpdateInstanceState(testutil.Context(t), bridgepkg.UpdateInstanceStateRequest{
		ID:      created.ID,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusReady,
	})
	if err != nil {
		t.Fatalf("UpdateInstanceState() error = %v", err)
	}
	if updated.Status != bridgepkg.BridgeStatusReady {
		t.Fatalf("UpdateInstanceState().Status = %q, want ready", updated.Status)
	}

	instances, err := registry.ListInstances(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListInstances() error = %v", err)
	}
	if got, want := len(instances), 1; got != want {
		t.Fatalf("len(instances) = %d, want %d", got, want)
	}
}

func TestRegistryUpdateInstanceStateAppliesAndClearsDegradation(t *testing.T) {
	t.Parallel()

	registry, _ := newRegistryTestHarness(t)
	created := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            "brg-state-degradation",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Degradation",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusStarting,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

	degraded, err := registry.UpdateInstanceState(testutil.Context(t), bridgepkg.UpdateInstanceStateRequest{
		ID:      created.ID,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusAuthRequired,
		Degradation: &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: "token expired",
		},
	})
	if err != nil {
		t.Fatalf("UpdateInstanceState(set degradation) error = %v", err)
	}
	if degraded.Degradation == nil || degraded.Degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("UpdateInstanceState(set degradation) = %#v, want auth_failed degradation", degraded.Degradation)
	}

	ready, err := registry.UpdateInstanceState(testutil.Context(t), bridgepkg.UpdateInstanceStateRequest{
		ID:      created.ID,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusStarting,
	})
	if err != nil {
		t.Fatalf("UpdateInstanceState(clear on status change) error = %v", err)
	}
	if ready.Degradation != nil {
		t.Fatalf("UpdateInstanceState(clear on status change).Degradation = %#v, want nil", ready.Degradation)
	}
}

func TestUpdateInstanceStateRequestValidateRejectsConflictingDegradationFlags(t *testing.T) {
	t.Parallel()

	err := (bridgepkg.UpdateInstanceStateRequest{
		ID:      "brg-conflict",
		Enabled: true,
		Status:  bridgepkg.BridgeStatusDegraded,
		Degradation: &bridgepkg.BridgeDegradation{
			Reason: bridgepkg.BridgeDegradationReasonRateLimited,
		},
		ClearDegradation: true,
	}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want conflict error")
	}
	if !strings.Contains(err.Error(), "cannot clear and set degradation together") {
		t.Fatalf("Validate() error = %v, want degradation conflict", err)
	}
}

func TestRegistryCreateInstanceAutoGeneratedIDUsesBridgePrefix(t *testing.T) {
	t.Parallel()

	registry, _ := newRegistryTestHarness(t)
	created, err := registry.CreateInstance(testutil.Context(t), bridgepkg.CreateInstanceRequest{
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Lifecycle",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusStarting,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}
	if !strings.HasPrefix(created.ID, "brg-") {
		t.Fatalf("CreateInstance().ID = %q, want brg- prefix", created.ID)
	}
}

func TestRegistryUpdateInstanceMutatesBridgeConfigAndDefaults(t *testing.T) {
	t.Parallel()

	registry, _ := newRegistryTestHarness(t)
	instance := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            "brg-update",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Original",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

	displayName := "Updated"
	dmPolicy := bridgepkg.BridgeDMPolicyAllowlist
	providerConfig := json.RawMessage(`{"mode":"bot","tenant":"ws-alpha"}`)
	deliveryDefaults := json.RawMessage(`{"peer_id":"peer-default","mode":"reply"}`)
	updated, err := registry.UpdateInstance(testutil.Context(t), bridgepkg.UpdateInstanceRequest{
		ID:               instance.ID,
		DisplayName:      &displayName,
		DMPolicy:         &dmPolicy,
		RoutingPolicy:    &bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
		ProviderConfig:   &providerConfig,
		DeliveryDefaults: &deliveryDefaults,
	})
	if err != nil {
		t.Fatalf("UpdateInstance() error = %v", err)
	}
	if updated.DisplayName != "Updated" {
		t.Fatalf("UpdateInstance().DisplayName = %q, want Updated", updated.DisplayName)
	}
	if !updated.RoutingPolicy.IncludeThread {
		t.Fatalf("UpdateInstance().RoutingPolicy = %#v, want thread routing enabled", updated.RoutingPolicy)
	}
	if got, want := updated.DMPolicy, bridgepkg.BridgeDMPolicyAllowlist; got != want {
		t.Fatalf("UpdateInstance().DMPolicy = %q, want %q", got, want)
	}
	if got := string(updated.ProviderConfig); got != `{"mode":"bot","tenant":"ws-alpha"}` {
		t.Fatalf("UpdateInstance().ProviderConfig = %s, want compact JSON", got)
	}
	if got := string(updated.DeliveryDefaults); got != `{"mode":"reply","peer_id":"peer-default"}` {
		t.Fatalf("UpdateInstance().DeliveryDefaults = %s, want compact JSON", got)
	}
}

func TestRegistryCreateInstanceWrapsInsertErrors(t *testing.T) {
	t.Parallel()

	insertErr := errors.New("insert failed")
	registry := bridgepkg.NewRegistry(stubRegistryStore{
		insertBridgeInstanceFn: func(context.Context, bridgepkg.BridgeInstance) error {
			return insertErr
		},
	})

	_, err := registry.CreateInstance(testutil.Context(t), bridgepkg.CreateInstanceRequest{
		ID:            "brg-wrap-create",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Wrapped Create",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusStarting,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	if !errors.Is(err, insertErr) {
		t.Fatalf("CreateInstance() error = %v, want wrapped %v", err, insertErr)
	}
	if !strings.Contains(err.Error(), "create bridge instance") || !strings.Contains(err.Error(), "insert") {
		t.Fatalf("CreateInstance() error = %q, want contextual insert failure", err)
	}
}

func TestRegistryListInstancesReturnsClonedDeliveryDefaults(t *testing.T) {
	t.Parallel()

	stored := []bridgepkg.BridgeInstance{{
		ID:               "brg-list-clone",
		Scope:            bridgepkg.ScopeGlobal,
		Platform:         "telegram",
		ExtensionName:    "telegram-adapter",
		DisplayName:      "List Clone",
		Enabled:          true,
		Status:           bridgepkg.BridgeStatusReady,
		RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true},
		DeliveryDefaults: json.RawMessage(`{"mode":"reply"}`),
	}}
	registry := bridgepkg.NewRegistry(stubRegistryStore{
		listBridgeInstancesFn: func(context.Context) ([]bridgepkg.BridgeInstance, error) {
			return stored, nil
		},
	})

	first, err := registry.ListInstances(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListInstances(first) error = %v", err)
	}
	first[0].DeliveryDefaults[0] = 'x'
	if got := string(stored[0].DeliveryDefaults); got != `{"mode":"reply"}` {
		t.Fatalf("stored delivery defaults = %s, want original JSON", got)
	}

	second, err := registry.ListInstances(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListInstances(second) error = %v", err)
	}
	if got := string(second[0].DeliveryDefaults); got != `{"mode":"reply"}` {
		t.Fatalf("ListInstances(second).DeliveryDefaults = %s, want original JSON", got)
	}
}

func TestRegistryListInstancesByIDsIsBoundedAndOrdered(t *testing.T) {
	t.Parallel()

	t.Run("Should request one batch and return cloned instances in caller order", func(t *testing.T) {
		t.Parallel()

		stored := []bridgepkg.BridgeInstance{
			{ID: "brg-b", DeliveryDefaults: json.RawMessage(`{"mode":"reply"}`)},
			{ID: "brg-a", DeliveryDefaults: json.RawMessage(`{"mode":"direct-send"}`)},
		}
		var capturedIDs []string
		registry := bridgepkg.NewRegistry(stubRegistryStore{
			listBridgeInstancesByIDsFn: func(
				_ context.Context,
				ids []string,
			) ([]bridgepkg.BridgeInstance, error) {
				capturedIDs = append([]string(nil), ids...)
				return stored, nil
			},
			listBridgeInstancesFn: func(context.Context) ([]bridgepkg.BridgeInstance, error) {
				t.Fatal("ListBridgeInstances() must not back a bounded lookup")
				return nil, nil
			},
		})

		instances, err := registry.ListInstancesByIDs(testutil.Context(t), []string{"brg-a", "brg-b", "brg-a"})
		if err != nil {
			t.Fatalf("ListInstancesByIDs() error = %v", err)
		}
		if got, want := strings.Join(capturedIDs, ","), "brg-a,brg-b"; got != want {
			t.Fatalf("store ids = %q, want %q", got, want)
		}
		if got, want := len(instances), 2; got != want {
			t.Fatalf("len(ListInstancesByIDs()) = %d, want %d", got, want)
		}
		if instances[0].ID != "brg-a" || instances[1].ID != "brg-b" {
			t.Fatalf("ListInstancesByIDs() order = %#v, want brg-a then brg-b", instances)
		}
		instances[0].DeliveryDefaults[0] = 'x'
		if got, want := string(stored[1].DeliveryDefaults), `{"mode":"direct-send"}`; got != want {
			t.Fatalf("stored delivery defaults = %s, want %s", got, want)
		}
	})

	t.Run("Should reject empty and oversized batches before persistence", func(t *testing.T) {
		t.Parallel()

		registry := bridgepkg.NewRegistry(stubRegistryStore{
			listBridgeInstancesByIDsFn: func(context.Context, []string) ([]bridgepkg.BridgeInstance, error) {
				t.Fatal("ListBridgeInstancesByIDs() should not run for invalid cardinality")
				return nil, nil
			},
		})
		if _, err := registry.ListInstancesByIDs(testutil.Context(t), nil); err == nil {
			t.Fatal("ListInstancesByIDs(nil) error = nil, want non-nil")
		}
		ids := make([]string, bridgepkg.MaxBridgeCatalogLimit+1)
		for index := range ids {
			ids[index] = fmt.Sprintf("brg-%03d", index)
		}
		if _, err := registry.ListInstancesByIDs(testutil.Context(t), ids); err == nil {
			t.Fatal("ListInstancesByIDs(oversized) error = nil, want non-nil")
		}
	})
}

func TestRegistryListRoutesReturnsClonedRoutes(t *testing.T) {
	t.Parallel()

	stored := []bridgepkg.BridgeRoute{{
		Scope:            bridgepkg.ScopeGlobal,
		BridgeInstanceID: "brg-route-clone",
		PeerID:           "peer-1",
		SessionID:        "sess-1",
		AgentName:        "coder",
	}}
	registry := bridgepkg.NewRegistry(stubRegistryStore{
		getBridgeInstanceFn: func(context.Context, string) (bridgepkg.BridgeInstance, error) {
			return bridgepkg.BridgeInstance{
				ID:            "brg-route-clone",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "telegram-adapter",
				DisplayName:   "Route Clone",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusReady,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			}, nil
		},
		listBridgeRoutesFn: func(context.Context, string) ([]bridgepkg.BridgeRoute, error) {
			return stored, nil
		},
	})

	first, err := registry.ListRoutes(testutil.Context(t), "brg-route-clone")
	if err != nil {
		t.Fatalf("ListRoutes(first) error = %v", err)
	}
	first[0].SessionID = "sess-mutated"
	if got, want := stored[0].SessionID, "sess-1"; got != want {
		t.Fatalf("stored route session id = %q, want %q", got, want)
	}

	second, err := registry.ListRoutes(testutil.Context(t), "brg-route-clone")
	if err != nil {
		t.Fatalf("ListRoutes(second) error = %v", err)
	}
	if got, want := second[0].SessionID, "sess-1"; got != want {
		t.Fatalf("ListRoutes(second).SessionID = %q, want %q", got, want)
	}
}

func TestRegistryResolveOrCreateRouteReusesStoredSession(t *testing.T) {
	t.Parallel()

	registry, _ := newRegistryTestHarness(t)
	instance := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            "brg-route-reuse",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Route Reuse",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
	})

	first, created, err := registry.ResolveOrCreateRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		SessionID:        "sess-1",
		AgentName:        "coder",
	})
	if err != nil {
		t.Fatalf("ResolveOrCreateRoute(first) error = %v", err)
	}
	if !created {
		t.Fatal("ResolveOrCreateRoute(first) created = false, want true")
	}

	second, created, err := registry.ResolveOrCreateRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		SessionID:        "sess-2",
		AgentName:        "reviewer",
	})
	if err != nil {
		t.Fatalf("ResolveOrCreateRoute(second) error = %v", err)
	}
	if created {
		t.Fatal("ResolveOrCreateRoute(second) created = true, want false")
	}
	if second.SessionID != first.SessionID {
		t.Fatalf("ResolveOrCreateRoute(second).SessionID = %q, want %q", second.SessionID, first.SessionID)
	}

	routes, err := registry.ListRoutes(testutil.Context(t), instance.ID)
	if err != nil {
		t.Fatalf("ListRoutes() error = %v", err)
	}
	if got, want := len(routes), 1; got != want {
		t.Fatalf("len(routes) = %d, want %d", got, want)
	}
}

func TestRegistryBuildResolveAndUpsertRoute(t *testing.T) {
	t.Parallel()

	registry, db := newRegistryTestHarness(t)
	workspaceID := registerWorkspaceForBridgesTests(t, db, "ws-build-route", "build-route")
	instance := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            "brg-build-route",
		Scope:         bridgepkg.ScopeWorkspace,
		WorkspaceID:   workspaceID,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Build Route",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
	})

	key, err := registry.BuildRoutingKey(testutil.Context(t), bridgepkg.RoutingKey{
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		GroupID:          "ignored-group",
	})
	if err != nil {
		t.Fatalf("BuildRoutingKey() error = %v", err)
	}
	if key.Scope != bridgepkg.ScopeWorkspace || key.WorkspaceID != workspaceID {
		t.Fatalf("BuildRoutingKey() = %#v, want workspace scope %q", key, workspaceID)
	}
	if key.GroupID != "" {
		t.Fatalf("BuildRoutingKey().GroupID = %q, want empty", key.GroupID)
	}

	route, err := registry.UpsertRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		SessionID:        "sess-1",
		AgentName:        "coder",
	})
	if err != nil {
		t.Fatalf("UpsertRoute(first) error = %v", err)
	}
	if route.SessionID != "sess-1" {
		t.Fatalf("UpsertRoute(first).SessionID = %q, want sess-1", route.SessionID)
	}

	resolved, err := registry.ResolveRoute(testutil.Context(t), bridgepkg.RoutingKey{
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		GroupID:          "ignored-group",
	})
	if err != nil {
		t.Fatalf("ResolveRoute() error = %v", err)
	}
	if resolved.RoutingKeyHash != route.RoutingKeyHash {
		t.Fatalf("ResolveRoute().RoutingKeyHash = %q, want %q", resolved.RoutingKeyHash, route.RoutingKeyHash)
	}

	rebound, err := registry.UpsertRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		SessionID:        "sess-2",
		AgentName:        "reviewer",
	})
	if err != nil {
		t.Fatalf("UpsertRoute(second) error = %v", err)
	}
	if rebound.SessionID != "sess-2" {
		t.Fatalf("UpsertRoute(second).SessionID = %q, want sess-2", rebound.SessionID)
	}
}

func TestRegistryResolveRouteRejectsDisabledInstance(t *testing.T) {
	t.Parallel()

	registry, _ := newRegistryTestHarness(t)
	instance := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            "brg-disabled-route",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Disabled Route",
		Enabled:       false,
		Status:        bridgepkg.BridgeStatusDisabled,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

	_, err := registry.ResolveRoute(testutil.Context(t), bridgepkg.RoutingKey{
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
	})
	if !errors.Is(err, bridgepkg.ErrBridgeInstanceUnavailable) {
		t.Fatalf("ResolveRoute(disabled) error = %v, want ErrBridgeInstanceUnavailable", err)
	}
}

func TestRegistryGuardClauses(t *testing.T) {
	t.Parallel()

	var nilRegistry *bridgepkg.Service
	if _, err := nilRegistry.ListInstances(testutil.Context(t)); err == nil {
		t.Fatal("nilRegistry.ListInstances() error = nil, want non-nil")
	}

	missingStore := bridgepkg.NewRegistry(nil)
	if _, err := missingStore.ListInstances(testutil.Context(t)); err == nil {
		t.Fatal("NewRegistry(nil).ListInstances() error = nil, want non-nil")
	}

	registry, _ := newRegistryTestHarness(t)
	if _, err := registry.ListInstances(nilContextForBridgesTests()); err == nil {
		t.Fatal("ListInstances(nil ctx) error = nil, want non-nil")
	}
}

func TestBuildBridgeCatalogPage(t *testing.T) {
	t.Parallel()

	t.Run("Should filter the full authority before the page cut and count exact facets", func(t *testing.T) {
		t.Parallel()

		instances := make([]bridgepkg.BridgeInstance, 0, 58)
		for index := range 55 {
			instances = append(instances, bridgeCatalogTestInstance(
				fmt.Sprintf("brg-prefix-%03d", index),
				bridgepkg.ScopeGlobal,
				"",
				"telegram",
				fmt.Sprintf("Prefix %03d", index),
				bridgepkg.BridgeStatusReady,
			))
		}
		instances = append(instances,
			bridgeCatalogTestInstance(
				"brg-needle-c", bridgepkg.ScopeWorkspace, "ws-alpha", "Slack", "Needle C", bridgepkg.BridgeStatusReady,
			),
			bridgeCatalogTestInstance(
				"brg-needle-a", bridgepkg.ScopeGlobal, "", "slack", "Needle A", bridgepkg.BridgeStatusReady,
			),
			bridgeCatalogTestInstance(
				"brg-needle-b", bridgepkg.ScopeWorkspace, "ws-alpha", "SLACK", "Needle B", bridgepkg.BridgeStatusReady,
			),
		)

		page, err := bridgepkg.BuildBridgeCatalogPage(instances, map[string]bridgepkg.BridgeStatus{
			"brg-needle-a": bridgepkg.BridgeStatusError,
			"brg-needle-b": bridgepkg.BridgeStatusError,
			"brg-needle-c": bridgepkg.BridgeStatusError,
		}, bridgepkg.BridgeCatalogQuery{
			Scope:       "all",
			WorkspaceID: "ws-alpha",
			Search:      "  NEEDLE  ",
			Platform:    " SlAcK ",
			Status:      bridgepkg.BridgeStatusError,
			Sort:        bridgepkg.BridgeCatalogSortName,
			Limit:       2,
		})
		if err != nil {
			t.Fatalf("BuildBridgeCatalogPage() error = %v", err)
		}
		if got, want := len(page.Items), 2; got != want {
			t.Fatalf("len(Items) = %d, want %d", got, want)
		}
		if got, want := page.Total, 3; got != want {
			t.Fatalf("Total = %d, want %d", got, want)
		}
		if !page.HasMore || page.NextCursor == "" || page.Limit != 2 {
			t.Fatalf("page metadata = %#v, want bounded continuation", page)
		}
		if got, want := page.Items[0].Instance.ID, "brg-needle-a"; got != want {
			t.Fatalf("first bridge id = %q, want %q", got, want)
		}
		if got, want := page.Items[1].Instance.ID, "brg-needle-b"; got != want {
			t.Fatalf("second bridge id = %q, want %q", got, want)
		}
		if got, want := page.Facets.Platforms["slack"], 3; got != want {
			t.Fatalf("platform facet = %d, want %d", got, want)
		}
		if got, want := page.Facets.Statuses[bridgepkg.BridgeStatusError], 3; got != want {
			t.Fatalf("status facet = %d, want %d", got, want)
		}
	})

	t.Run("Should walk stable tied pages without gaps after the anchor mutates", func(t *testing.T) {
		t.Parallel()

		instances := make([]bridgepkg.BridgeInstance, 0, 205)
		for index := range 205 {
			instances = append(instances, bridgeCatalogTestInstance(
				fmt.Sprintf("brg-%03d", index),
				bridgepkg.ScopeGlobal,
				"",
				"telegram",
				"Same name",
				bridgepkg.BridgeStatusReady,
			))
		}
		query := bridgepkg.BridgeCatalogQuery{Sort: bridgepkg.BridgeCatalogSortName, Limit: 50}
		first, err := bridgepkg.BuildBridgeCatalogPage(instances, nil, query)
		if err != nil {
			t.Fatalf("BuildBridgeCatalogPage(first) error = %v", err)
		}
		instances[49].DisplayName = "A name before the captured anchor"
		query.Cursor = first.NextCursor

		seen := make(map[string]struct{}, len(instances))
		for _, item := range first.Items {
			seen[item.Instance.ID] = struct{}{}
		}
		for {
			page, pageErr := bridgepkg.BuildBridgeCatalogPage(instances, nil, query)
			if pageErr != nil {
				t.Fatalf("BuildBridgeCatalogPage(next) error = %v", pageErr)
			}
			for _, item := range page.Items {
				if _, duplicate := seen[item.Instance.ID]; duplicate {
					t.Fatalf("duplicate bridge id %q across pages", item.Instance.ID)
				}
				seen[item.Instance.ID] = struct{}{}
			}
			if !page.HasMore {
				break
			}
			query.Cursor = page.NextCursor
		}
		if got, want := len(seen), len(instances); got != want {
			t.Fatalf("walked bridge count = %d, want %d", got, want)
		}
	})

	t.Run("Should reject a cursor reused across query or workspace boundaries", func(t *testing.T) {
		t.Parallel()

		instances := []bridgepkg.BridgeInstance{
			bridgeCatalogTestInstance("brg-a", bridgepkg.ScopeGlobal, "", "telegram", "A", bridgepkg.BridgeStatusReady),
			bridgeCatalogTestInstance("brg-b", bridgepkg.ScopeGlobal, "", "telegram", "B", bridgepkg.BridgeStatusReady),
		}
		page, err := bridgepkg.BuildBridgeCatalogPage(instances, nil, bridgepkg.BridgeCatalogQuery{
			Scope:       "all",
			WorkspaceID: "ws-alpha",
			Limit:       1,
		})
		if err != nil {
			t.Fatalf("BuildBridgeCatalogPage() error = %v", err)
		}
		cases := []bridgepkg.BridgeCatalogQuery{
			{Scope: "all", WorkspaceID: "ws-beta", Cursor: page.NextCursor, Limit: 1},
			{
				Scope: "all", WorkspaceID: "ws-alpha", Search: "different", Cursor: page.NextCursor, Limit: 1,
			},
		}
		for _, query := range cases {
			_, cursorErr := bridgepkg.BuildBridgeCatalogPage(instances, nil, query)
			if !errors.Is(cursorErr, bridgepkg.ErrBridgeCatalogCursorInvalid) {
				t.Fatalf(
					"BuildBridgeCatalogPage(mismatched cursor) error = %v, want ErrBridgeCatalogCursorInvalid",
					cursorErr,
				)
			}
		}

		for _, field := range []string{"id", "display_name", "scope"} {
			forged := forgeBridgeCatalogCursor(t, page.NextCursor, field)
			_, cursorErr := bridgepkg.BuildBridgeCatalogPage(
				instances,
				nil,
				bridgepkg.BridgeCatalogQuery{
					Scope:       "all",
					WorkspaceID: "ws-alpha",
					Cursor:      forged,
					Limit:       1,
				},
			)
			if !errors.Is(cursorErr, bridgepkg.ErrBridgeCatalogCursorInvalid) {
				t.Fatalf(
					"BuildBridgeCatalogPage(forged %s) error = %v, want ErrBridgeCatalogCursorInvalid",
					field,
					cursorErr,
				)
			}
		}
	})

	t.Run("Should preserve disabled and auth-required status over stale runtime overlays", func(t *testing.T) {
		t.Parallel()

		instances := []bridgepkg.BridgeInstance{
			bridgeCatalogTestInstance(
				"brg-disabled",
				bridgepkg.ScopeGlobal,
				"",
				"telegram",
				"Disabled",
				bridgepkg.BridgeStatusDisabled,
			),
			bridgeCatalogTestInstance(
				"brg-auth",
				bridgepkg.ScopeGlobal,
				"",
				"telegram",
				"Auth",
				bridgepkg.BridgeStatusAuthRequired,
			),
		}
		page, err := bridgepkg.BuildBridgeCatalogPage(instances, map[string]bridgepkg.BridgeStatus{
			"brg-disabled": bridgepkg.BridgeStatusReady,
			"brg-auth":     bridgepkg.BridgeStatusReady,
		}, bridgepkg.BridgeCatalogQuery{})
		if err != nil {
			t.Fatalf("BuildBridgeCatalogPage() error = %v", err)
		}
		statuses := make(map[string]bridgepkg.BridgeStatus, len(page.Items))
		for _, item := range page.Items {
			statuses[item.Instance.ID] = item.EffectiveStatus
		}
		if statuses["brg-disabled"] != bridgepkg.BridgeStatusDisabled ||
			statuses["brg-auth"] != bridgepkg.BridgeStatusAuthRequired {
			t.Fatalf("effective statuses = %#v, want persisted safety precedence", statuses)
		}
	})

	t.Run("Should return explicit zero counts and reject unsupported sorts", func(t *testing.T) {
		t.Parallel()

		page, err := bridgepkg.BuildBridgeCatalogPage(nil, nil, bridgepkg.BridgeCatalogQuery{})
		if err != nil {
			t.Fatalf("BuildBridgeCatalogPage(empty) error = %v", err)
		}
		if page.Total != 0 || len(page.Items) != 0 || page.Limit != bridgepkg.DefaultBridgeCatalogLimit {
			t.Fatalf("empty page = %#v, want explicit zero total and default limit", page)
		}
		_, err = bridgepkg.BuildBridgeCatalogPage(nil, nil, bridgepkg.BridgeCatalogQuery{Sort: "recent"})
		if err == nil {
			t.Fatal("BuildBridgeCatalogPage(recent) error = nil, want unsupported sort error")
		}
	})
}

func bridgeCatalogTestInstance(
	id string,
	scope bridgepkg.Scope,
	workspaceID string,
	platform string,
	displayName string,
	status bridgepkg.BridgeStatus,
) bridgepkg.BridgeInstance {
	return bridgepkg.BridgeInstance{
		ID:            id,
		Scope:         scope,
		WorkspaceID:   workspaceID,
		Platform:      platform,
		ExtensionName: platform + "-extension",
		DisplayName:   displayName,
		Enabled:       status != bridgepkg.BridgeStatusDisabled,
		Status:        status,
		CreatedAt:     time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
	}
}

func forgeBridgeCatalogCursor(t *testing.T, cursor string, field string) string {
	t.Helper()

	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatalf("DecodeString(cursor) error = %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(decoded, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(cursor) error = %v", err)
	}
	position, ok := envelope["p"].(map[string]any)
	if !ok {
		t.Fatalf("cursor position = %#v, want object", envelope["p"])
	}
	if field == "scope" {
		position[field] = "unknown"
	} else {
		position[field] = ""
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("json.Marshal(cursor) error = %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func newRegistryTestHarness(t *testing.T) (*bridgepkg.Service, *memoryRegistryStore) {
	t.Helper()

	store := newMemoryRegistryStore()
	now := time.Date(2026, 4, 10, 16, 0, 0, 0, time.UTC)
	registry := bridgepkg.NewRegistry(store, bridgepkg.WithNow(func() time.Time { return now }))
	return registry, store
}

func createTestBridgeInstance(
	t *testing.T,
	registry *bridgepkg.Service,
	req bridgepkg.CreateInstanceRequest,
) *bridgepkg.BridgeInstance {
	t.Helper()

	instance, err := registry.CreateInstance(testutil.Context(t), req)
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}
	return instance
}

func registerWorkspaceForBridgesTests(t *testing.T, store *memoryRegistryStore, id string, _ string) string {
	t.Helper()

	store.InsertWorkspace(id)
	return id
}

func nilContextForBridgesTests() context.Context {
	return nil
}
