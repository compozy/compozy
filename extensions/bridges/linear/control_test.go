package main

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func TestLinearControlCheckIsExplicitlyUnsupported(t *testing.T) {
	t.Run("Should return a skipped identity record instead of silent absence", func(t *testing.T) {
		t.Parallel()

		check := linearIdentityCheck()
		if check.Check != "provider.identity" || check.Status != bridgepkg.BridgeCheckStatusSkipped {
			t.Fatalf("linearIdentityCheck() = %#v, want explicit skipped identity", check)
		}
		if check.Remediation == "" {
			t.Fatal("linearIdentityCheck() remediation is empty")
		}
	})
}

func TestLinearControlRuntimeCheck(t *testing.T) {
	t.Run("Should return explicit identity and webhook records for the managed instance", func(t *testing.T) {
		t.Parallel()

		managed := subprocess.InitializeBridgeManagedInstance{Instance: bridgepkg.BridgeInstance{
			ID: "brg-linear-control",
		}}
		cache := bridgesdk.NewInstanceCache(&subprocess.InitializeBridgeRuntime{
			ManagedInstances: []subprocess.InitializeBridgeManagedInstance{managed},
		})
		session := &bridgesdk.Session{}
		field := reflect.ValueOf(session).Elem().FieldByName("cache")
		reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(cache))
		provider, err := newLinearProvider(nil)
		if err != nil {
			t.Fatalf("newLinearProvider() error = %v", err)
		}

		response, err := provider.handleBridgeCheck(context.Background(), session, bridgepkg.BridgeCheckRequest{
			BridgeInstanceID: managed.Instance.ID,
		})
		if err != nil {
			t.Fatalf("handleBridgeCheck() error = %v", err)
		}
		if len(response.Checks) < 2 || response.Checks[0].Status != bridgepkg.BridgeCheckStatusSkipped {
			t.Fatalf("handleBridgeCheck() checks = %#v, want explicit skipped identity", response.Checks)
		}
	})

	t.Run("Should reject a missing control cache and an unmanaged instance", func(t *testing.T) {
		t.Parallel()

		provider, err := newLinearProvider(nil)
		if err != nil {
			t.Fatalf("newLinearProvider() error = %v", err)
		}
		if _, err := provider.handleBridgeCheck(
			context.Background(),
			nil,
			bridgepkg.BridgeCheckRequest{BridgeInstanceID: "missing"},
		); err == nil || !strings.Contains(err.Error(), "cache") {
			t.Fatalf("handleBridgeCheck(nil session) error = %v, want cache", err)
		}

		cache := bridgesdk.NewInstanceCache(&subprocess.InitializeBridgeRuntime{})
		session := &bridgesdk.Session{}
		field := reflect.ValueOf(session).Elem().FieldByName("cache")
		reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(cache))
		if _, err := provider.handleBridgeCheck(
			context.Background(),
			session,
			bridgepkg.BridgeCheckRequest{BridgeInstanceID: "missing"},
		); err == nil || !strings.Contains(err.Error(), "not in the control session") {
			t.Fatalf("handleBridgeCheck(unmanaged) error = %v", err)
		}
	})

	t.Run("Should bind and release the provider listener helper", func(t *testing.T) {
		t.Parallel()

		listener, err := listenLinearWebhook("127.0.0.1:0")
		if err != nil {
			t.Fatalf("listenLinearWebhook() error = %v", err)
		}
		if err := listener.Close(); err != nil {
			t.Fatalf("listener.Close() error = %v", err)
		}
	})
}
