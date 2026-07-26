// Suite: Teams provider control runtime
// Invariant: a disabled Teams instance answers bridges/check through the control handshake with explicit identity and webhook results.
// Boundary IN: provider runtime peer, purpose=control handshake, bound credentials, and fake Bot Framework API.
// Boundary OUT: real Microsoft endpoints, owned by release QA.
// Canonical suite: extensions/bridges/teams/control_test.go.
package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func TestTeamsIdentityChecksReportTokenAcquisition(t *testing.T) {
	t.Run("Should pass when Bot Framework token acquisition succeeds", func(t *testing.T) {
		t.Parallel()

		checks := teamsIdentityChecks(nil)
		if len(checks) != 1 || checks[0].Status != bridgepkg.BridgeCheckStatusPass {
			t.Fatalf("teamsIdentityChecks() = %#v, want one pass", checks)
		}
	})

	t.Run("Should name the credential bindings when token acquisition fails", func(t *testing.T) {
		t.Parallel()

		checks := teamsIdentityChecks(errors.New("unauthorized"))
		if len(checks) != 1 || checks[0].Status != bridgepkg.BridgeCheckStatusFail {
			t.Fatalf("teamsIdentityChecks() = %#v, want one fail", checks)
		}
		if !strings.Contains(checks[0].Remediation, "app_id") ||
			!strings.Contains(checks[0].Remediation, "app_password") {
			t.Fatalf("remediation = %q, want Teams credential slots", checks[0].Remediation)
		}
	})
}

func TestTeamsControlRuntimeCheck(t *testing.T) {
	t.Run("Should answer through the control peer with identity pass and disabled webhook skip", func(t *testing.T) {
		t.Parallel()

		runtime, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		runtime.apiFactory = func(resolvedInstanceConfig) teamsAPI { return &fakeTeamsAPI{} }

		now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
		managed := testTeamsManagedInstance(t, now, "brg-teams-control", map[string]any{
			"webhook": map[string]any{
				"public_url": "https://bridge.example.test/teams/control",
			},
		})
		managed.Instance.Enabled = false
		managed.Instance.Status = bridgepkg.BridgeStatusDisabled
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
		assertTeamsControlCheckStatus(
			t,
			response.Checks,
			"provider.identity",
			bridgepkg.BridgeCheckStatusPass,
		)
		assertTeamsControlCheckStatus(
			t,
			response.Checks,
			"webhook.reachability",
			bridgepkg.BridgeCheckStatusSkipped,
		)
	})

	for _, test := range []struct {
		name                    string
		configure               func(*subprocess.InitializeBridgeManagedInstance)
		wantCheck               string
		wantRemediationContains string
	}{
		{
			name: "Should return app id remediation through the control peer when its binding is missing",
			configure: func(managed *subprocess.InitializeBridgeManagedInstance) {
				managed.BoundSecrets = slices.DeleteFunc(
					managed.BoundSecrets,
					func(secret subprocess.InitializeBridgeBoundSecret) bool {
						return secret.BindingName == "app_id"
					},
				)
			},
			wantCheck:               "provider.identity",
			wantRemediationContains: "app_id",
		},
		{
			name: "Should return app password remediation through the control peer when its binding is missing",
			configure: func(managed *subprocess.InitializeBridgeManagedInstance) {
				managed.BoundSecrets = slices.DeleteFunc(
					managed.BoundSecrets,
					func(secret subprocess.InitializeBridgeBoundSecret) bool {
						return secret.BindingName == "app_password"
					},
				)
			},
			wantCheck:               "provider.identity",
			wantRemediationContains: "app_password",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			runtime, hostPeer, cleanup := newRuntimePeerPair(t)
			defer cleanup()
			apiFactoryCalls := 0
			runtime.apiFactory = func(resolvedInstanceConfig) teamsAPI {
				apiFactoryCalls++
				return &fakeTeamsAPI{}
			}

			now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
			managed := testTeamsManagedInstance(t, now, "brg-teams-control-invalid", map[string]any{
				"webhook": map[string]any{
					"public_url": "https://bridge.example.test/teams/control",
				},
			})
			managed.Instance.Enabled = false
			managed.Instance.Status = bridgepkg.BridgeStatusDisabled
			test.configure(&managed)
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
			if apiFactoryCalls != 0 {
				t.Fatalf("provider API factory calls = %d, want 0 for invalid local credentials", apiFactoryCalls)
			}

			var matched *bridgepkg.BridgeCheckRecord
			for index := range response.Checks {
				if response.Checks[index].Check == test.wantCheck {
					matched = &response.Checks[index]
					break
				}
			}
			if matched == nil {
				t.Fatalf("check %q missing from %#v", test.wantCheck, response.Checks)
			}
			if matched.Status != bridgepkg.BridgeCheckStatusFail ||
				!strings.Contains(matched.Remediation, test.wantRemediationContains) {
				t.Fatalf(
					"check %q = %#v, want fail remediation containing %q",
					test.wantCheck,
					*matched,
					test.wantRemediationContains,
				)
			}
		})
	}
}

func assertTeamsControlCheckStatus(
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
