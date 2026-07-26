// Suite: WhatsApp provider control runtime
// Invariant: a disabled WhatsApp instance answers bridges/check through the control handshake with explicit identity and webhook results.
// Boundary IN: provider runtime peer, purpose=control handshake, bound Meta credentials, and fake Graph API.
// Boundary OUT: real Meta endpoints, owned by release QA.
// Canonical suite: extensions/bridges/whatsapp/control_test.go.
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

func TestWhatsAppIdentityChecksValidatePhoneNumberAccess(t *testing.T) {
	t.Run("Should pass when Graph returns the configured phone number", func(t *testing.T) {
		t.Parallel()

		checks := whatsappIdentityChecks(&whatsappPhoneNumber{ID: "123456789012345"}, nil)
		if len(checks) != 1 || checks[0].Status != bridgepkg.BridgeCheckStatusPass {
			t.Fatalf("whatsappIdentityChecks() = %#v, want one pass", checks)
		}
	})

	t.Run("Should name the access token binding when Graph rejects identity", func(t *testing.T) {
		t.Parallel()

		checks := whatsappIdentityChecks(nil, errors.New("invalid token"))
		if len(checks) != 1 || checks[0].Status != bridgepkg.BridgeCheckStatusFail {
			t.Fatalf("whatsappIdentityChecks() = %#v, want one fail", checks)
		}
		if !strings.Contains(checks[0].Remediation, "access_token") {
			t.Fatalf("remediation = %q, want access_token", checks[0].Remediation)
		}
	})
}

func TestWhatsAppControlRuntimeCheck(t *testing.T) {
	t.Run("Should answer through the control peer with identity pass and disabled webhook skip", func(t *testing.T) {
		t.Parallel()

		runtime, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		runtime.apiFactory = func(resolvedInstanceConfig) whatsappAPI { return &fakeWhatsAppAPI{} }

		now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
		managed := testBridgeRuntime(now, "brg-whatsapp-control")
		managed.Instance.Enabled = false
		managed.Instance.Status = bridgepkg.BridgeStatusDisabled
		managed.Instance.ProviderConfig = []byte(`{
			"phone_number_id":"123456789012345",
			"webhook":{"public_url":"https://bridge.example.test/whatsapp/control"}
		}`)
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
		assertWhatsAppControlCheckStatus(
			t,
			response.Checks,
			"provider.identity",
			bridgepkg.BridgeCheckStatusPass,
		)
		assertWhatsAppControlCheckStatus(
			t,
			response.Checks,
			"webhook.reachability",
			bridgepkg.BridgeCheckStatusSkipped,
		)
	})
}

func assertWhatsAppControlCheckStatus(
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
