package bridges_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/testutil"
)

func TestRegistryContextRefacs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func(*bridgepkg.Service, context.Context) error
	}{
		{
			name: "Should reject canceled ListInstances before calling the store",
			call: func(registry *bridgepkg.Service, ctx context.Context) error {
				_, err := registry.ListInstances(ctx)
				return err
			},
		},
		{
			name: "Should reject canceled BuildRoutingKey before calling the store",
			call: func(registry *bridgepkg.Service, ctx context.Context) error {
				_, err := registry.BuildRoutingKey(ctx, bridgepkg.RoutingKey{BridgeInstanceID: "brg-canceled"})
				return err
			},
		},
		{
			name: "Should reject canceled ResolveDeliveryTarget before calling the store",
			call: func(registry *bridgepkg.Service, ctx context.Context) error {
				_, err := registry.ResolveDeliveryTarget(ctx, bridgepkg.ResolveDeliveryTargetRequest{
					BridgeInstanceID: "brg-canceled",
				})
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			storeCalled := false
			registry := bridgepkg.NewRegistry(stubRegistryStore{
				getBridgeInstanceFn: func(context.Context, string) (bridgepkg.BridgeInstance, error) {
					storeCalled = true
					return bridgepkg.BridgeInstance{}, errors.New("store should not be called")
				},
				listBridgeInstancesFn: func(context.Context) ([]bridgepkg.BridgeInstance, error) {
					storeCalled = true
					return nil, errors.New("store should not be called")
				},
			})
			ctx, cancel := context.WithCancel(testutil.Context(t))
			cancel()

			if err := tc.call(registry, ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("registry call error = %v, want %v", err, context.Canceled)
			}
			if storeCalled {
				t.Fatal("registry call reached store after context cancellation")
			}
		})
	}
}

func TestBridgeProviderConfigRefacs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		config json.RawMessage
	}{
		{name: "Should reject scalar provider config", config: json.RawMessage(`"bot"`)},
		{name: "Should reject array provider config", config: json.RawMessage(`["bot"]`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := providerConfigRefacCreateRequest(tc.config)
			registry, _ := newRegistryTestHarness(t)
			_, err := registry.CreateInstance(testutil.Context(t), req)
			requireProviderConfigShapeError(t, err)

			validReq := providerConfigRefacCreateRequest(json.RawMessage(`{"mode":"bot"}`))
			validReq.ID = "brg-provider-update"
			created := createTestBridgeInstance(t, registry, validReq)
			update := bridgepkg.UpdateInstanceRequest{
				ID:             created.ID,
				ProviderConfig: &tc.config,
			}
			_, err = registry.UpdateInstance(testutil.Context(t), update)
			requireProviderConfigShapeError(t, err)
		})
	}
}

func TestBridgeProviderConfigRejectsOperatorOwnedDestinations(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a nested operator-owned URL before persistence", func(t *testing.T) {
		t.Parallel()

		const key = "token_url"
		request := providerConfigRefacCreateRequest(
			json.RawMessage(`{"auth":{"token_url":"https://attacker.example/token"}}`),
		)
		registry, store := newRegistryTestHarness(t)
		_, err := registry.CreateInstance(testutil.Context(t), request)
		if err == nil {
			t.Fatalf("CreateInstance() error = nil, want rejection for %q", key)
		}
		if !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "operator-owned") {
			t.Fatalf("CreateInstance() error = %q, want %q operator-owned rejection", err, key)
		}
		if len(store.instances) != 0 {
			t.Fatalf("persisted instances = %d, want 0", len(store.instances))
		}
	})
}

func providerConfigRefacCreateRequest(config json.RawMessage) bridgepkg.CreateInstanceRequest {
	return bridgepkg.CreateInstanceRequest{
		ID:             "brg-provider-create",
		Scope:          bridgepkg.ScopeGlobal,
		Platform:       "slack",
		ExtensionName:  "slack-adapter",
		DisplayName:    "Slack Provider",
		Enabled:        true,
		Status:         bridgepkg.BridgeStatusReady,
		RoutingPolicy:  bridgepkg.RoutingPolicy{IncludePeer: true},
		ProviderConfig: config,
	}
}

func requireProviderConfigShapeError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("provider config validation error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "bridge instance provider config must be a JSON object or null") {
		t.Fatalf("provider config validation error = %v, want JSON object shape error", err)
	}
}
