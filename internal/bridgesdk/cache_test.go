package bridgesdk

import (
	"context"
	"encoding/json"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func TestInstanceCacheSyncPreservesBoundSecrets(t *testing.T) {
	t.Parallel()

	cache := NewInstanceCache(testManagedRuntime("brg-1"))
	host := NewHostAPIClientFromCall(func(_ context.Context, method string, _ any, result any) error {
		switch method {
		case "bridges/instances/list":
			target := result.(*[]bridgepkg.BridgeInstance)
			*target = []bridgepkg.BridgeInstance{
				func() bridgepkg.BridgeInstance {
					instance := testBridgeInstance("brg-1")
					instance.Status = bridgepkg.BridgeStatusDegraded
					return instance
				}(),
				testBridgeInstance("brg-2"),
			}
			return nil
		default:
			t.Fatalf("unexpected method = %q", method)
			return nil
		}
	})

	items, err := cache.Sync(context.Background(), host)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if got, want := len(items), 2; got != want {
		t.Fatalf("len(Sync()) = %d, want %d", got, want)
	}

	if value, ok := cache.BoundSecretValue("brg-1", "bot_token"); !ok || value != "secret-brg-1" {
		t.Fatalf("BoundSecretValue(brg-1, bot_token) = (%q, %v), want (secret-brg-1, true)", value, ok)
	}
	if _, ok := cache.BoundSecretValue("brg-2", "bot_token"); ok {
		t.Fatal("BoundSecretValue(brg-2, bot_token) ok = true, want false")
	}
}

func TestInstanceCacheSnapshotAndListReturnClones(t *testing.T) {
	t.Parallel()

	runtime := testManagedRuntime("brg-1")
	runtime.ManagedInstances[0].Instance.ProviderConfig = json.RawMessage(`{"mode":"bot"}`)
	runtime.ManagedInstances[0].Instance.DeliveryDefaults = json.RawMessage(`{"peer_id":"peer-1"}`)
	runtime.ManagedInstances[0].Instance.Degradation = &bridgepkg.BridgeDegradation{
		Reason: bridgepkg.BridgeDegradationReasonRateLimited,
	}

	cache := NewInstanceCache(runtime)
	snapshot := cache.Snapshot()
	listed := cache.List()

	snapshot.ManagedInstances[0].BoundSecrets[0].Value = "changed"
	listed[0].Instance.ProviderConfig = json.RawMessage(`{"mode":"changed"}`)
	listed[0].Instance.DeliveryDefaults = json.RawMessage(`{"peer_id":"changed"}`)
	listed[0].Instance.Degradation.Reason = bridgepkg.BridgeDegradationReasonAuthFailed

	value, ok := cache.BoundSecretValue("brg-1", "bot_token")
	if !ok || value != "secret-brg-1" {
		t.Fatalf("BoundSecretValue(brg-1, bot_token) = (%q, %v), want (secret-brg-1, true)", value, ok)
	}

	fresh, ok := cache.Get("brg-1")
	if !ok {
		t.Fatal("cache.Get(brg-1) ok = false, want true")
	}
	if got, want := string(fresh.Instance.ProviderConfig), `{"mode":"bot"}`; got != want {
		t.Fatalf("fresh.Instance.ProviderConfig = %s, want %s", got, want)
	}
	if got, want := string(fresh.Instance.DeliveryDefaults), `{"peer_id":"peer-1"}`; got != want {
		t.Fatalf("fresh.Instance.DeliveryDefaults = %s, want %s", got, want)
	}
	if fresh.Instance.Degradation == nil ||
		fresh.Instance.Degradation.Reason != bridgepkg.BridgeDegradationReasonRateLimited {
		t.Fatalf("fresh.Instance.Degradation = %#v, want rate_limited", fresh.Instance.Degradation)
	}
}
