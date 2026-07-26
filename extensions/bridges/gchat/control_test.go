// Suite: Google Chat provider control runtime
// Invariant: a disabled Google Chat instance answers bridges/check through the control handshake with explicit identity and webhook results.
// Boundary IN: provider runtime peer, purpose=control handshake, bound service-account credentials, and fake Chat API.
// Boundary OUT: real Google endpoints, owned by release QA.
// Canonical suite: extensions/bridges/gchat/control_test.go.
package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func TestGChatIdentityChecksReportCredentialValidation(t *testing.T) {
	t.Run("Should pass when service-account token acquisition succeeds", func(t *testing.T) {
		t.Parallel()

		checks := gchatIdentityChecks(nil)
		if len(checks) != 1 || checks[0].Status != bridgepkg.BridgeCheckStatusPass {
			t.Fatalf("gchatIdentityChecks() = %#v, want one pass", checks)
		}
	})

	t.Run("Should name credentials json when authentication fails", func(t *testing.T) {
		t.Parallel()

		checks := gchatIdentityChecks(errors.New("invalid assertion"))
		if len(checks) != 1 || checks[0].Status != bridgepkg.BridgeCheckStatusFail {
			t.Fatalf("gchatIdentityChecks() = %#v, want one fail", checks)
		}
		if !strings.Contains(checks[0].Remediation, "credentials_json") {
			t.Fatalf("remediation = %q, want credentials_json", checks[0].Remediation)
		}
	})
}

func TestGChatControlRuntimeCheck(t *testing.T) {
	t.Run("Should answer through the control peer with identity pass and disabled webhook skip", func(t *testing.T) {
		t.Parallel()

		runtime, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		runtime.apiFactory = func(*resolvedInstanceConfig) gchatAPI { return &fakeGChatAPI{} }

		now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
		managed := testBridgeRuntime(t, now, "brg-gchat-control")
		managed.Instance.Enabled = false
		managed.Instance.Status = bridgepkg.BridgeStatusDisabled
		managed.Instance.ProviderConfig = []byte(
			`{"webhook":{"public_url":"https://bridge.example.test/gchat/control"}}`,
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
			t.Fatalf("hostPeer.Call(initialize control) error = %v", err)
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
			t.Fatalf("BridgeCheckResponse.Validate() error = %v; response=%#v", err, response)
		}
		if len(response.Checks) == 0 {
			t.Fatal("bridges/check response is empty")
		}
		assertGChatControlCheckStatus(
			t,
			response.Checks,
			"provider.identity",
			bridgepkg.BridgeCheckStatusPass,
		)
		assertGChatControlCheckStatus(
			t,
			response.Checks,
			"webhook.reachability",
			bridgepkg.BridgeCheckStatusSkipped,
		)
	})
}

func assertGChatControlCheckStatus(
	t *testing.T,
	checks []bridgepkg.BridgeCheckRecord,
	name string,
	want bridgepkg.BridgeCheckStatus,
) {
	t.Helper()

	for _, check := range checks {
		if check.Check != name {
			continue
		}
		if check.Status != want {
			t.Fatalf("check %q status = %q, want %q; checks=%#v", name, check.Status, want, checks)
		}
		return
	}
	t.Fatalf("check %q missing from %#v", name, checks)
}
