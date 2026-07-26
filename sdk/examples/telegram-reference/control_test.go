package main

import (
	"context"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func TestReferenceAdapterControlCheckIsExplicit(t *testing.T) {
	t.Run("Should return an actionable skipped identity result", func(t *testing.T) {
		t.Parallel()

		check := referenceIdentityCheck()
		if check.Status != bridgepkg.BridgeCheckStatusSkipped || check.Remediation == "" {
			t.Fatalf("referenceIdentityCheck() = %#v, want actionable skip", check)
		}
	})

	t.Run("Should reject a missing control session", func(t *testing.T) {
		t.Parallel()

		runtime := &telegramReferenceRuntime{}
		if err := runtime.healthCheck(); err != nil {
			t.Fatalf("healthCheck(empty) error = %v", err)
		}
		_, err := runtime.handleBridgeCheck(
			context.Background(),
			nil,
			bridgepkg.BridgeCheckRequest{BridgeInstanceID: "brg-reference"},
		)
		if err == nil {
			t.Fatal("handleBridgeCheck(nil session) error = nil")
		}
	})
}

func TestReferenceAdapterControlRuntimeReturnsExplicitChecks(t *testing.T) {
	t.Run("Should answer the isolated check contract without provider API calls", func(t *testing.T) {
		t.Parallel()

		_, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
		managed := testBridgeRuntime(now, "brg-reference-control")
		managed.Instance.Enabled = false
		managed.Instance.Status = bridgepkg.BridgeStatusDisabled
		managed.Instance.ProviderConfig = []byte(
			`{"webhook":{"public_url":"https://bridge.example.test/telegram/reference"}}`,
		)
		initialize := testInitializeRequest(now, managed)
		initialize.Capabilities = subprocess.InitializeCapabilities{}
		initialize.Methods = subprocess.InitializeMethods{
			DaemonRequests:    []string{"shutdown"},
			ExtensionServices: []string{string(bridgepkg.ControlMethodCheck)},
		}
		initialize.Runtime.Bridge.Purpose = subprocess.BridgeRuntimePurposeControl
		initialize.Runtime.Bridge.AllowedMethods = []string{string(bridgepkg.ControlMethodCheck)}
		if err := hostPeer.Call(context.Background(), "initialize", initialize, nil); err != nil {
			t.Fatalf("hostPeer.Call(initialize) error = %v", err)
		}

		var response bridgepkg.BridgeCheckResponse
		if err := hostPeer.Call(
			context.Background(),
			string(bridgepkg.ControlMethodCheck),
			bridgepkg.BridgeCheckRequest{BridgeInstanceID: managed.Instance.ID},
			&response,
		); err != nil {
			t.Fatalf("hostPeer.Call(bridges/check) error = %v", err)
		}
		if err := response.Validate(); err != nil {
			t.Fatalf("BridgeCheckResponse.Validate() error = %v", err)
		}
		if response.Checks[0].Status != bridgepkg.BridgeCheckStatusSkipped ||
			response.Checks[len(response.Checks)-1].Status != bridgepkg.BridgeCheckStatusSkipped {
			t.Fatalf("checks = %#v, want explicit identity and reachability skips", response.Checks)
		}
		var missing bridgepkg.BridgeCheckResponse
		err := hostPeer.Call(
			context.Background(),
			string(bridgepkg.ControlMethodCheck),
			bridgepkg.BridgeCheckRequest{BridgeInstanceID: "brg-missing"},
			&missing,
		)
		if err == nil {
			t.Fatal("missing instance check error = nil, want control-session rejection")
		}
	})
}
