package main

import (
	"context"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func TestGitHubControlCheckIsExplicitlyUnsupported(t *testing.T) {
	t.Run("Should return a skipped identity record instead of silent absence", func(t *testing.T) {
		t.Parallel()

		check := githubIdentityCheck()
		if check.Check != "provider.identity" || check.Status != bridgepkg.BridgeCheckStatusSkipped {
			t.Fatalf("githubIdentityCheck() = %#v, want explicit skipped identity", check)
		}
		if check.Remediation == "" {
			t.Fatal("githubIdentityCheck() remediation is empty")
		}
	})
}

func TestGitHubControlRuntimeReturnsExplicitChecks(t *testing.T) {
	t.Run("Should answer the isolated check contract without issue side effects", func(t *testing.T) {
		t.Parallel()

		_, hostPeer, cleanup := newGitHubRuntimePeerPair(t)
		defer cleanup()
		managed := githubProgressManagedInstance()
		managed.Instance.Enabled = false
		managed.Instance.Status = bridgepkg.BridgeStatusDisabled
		managed.Instance.ProviderConfig = []byte(
			`{"webhook":{"public_url":"https://bridge.example.test/github/app"}}`,
		)
		initialize := githubRuntimeInitializeRequest(managed)
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
	})
}
