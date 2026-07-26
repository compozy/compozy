// Suite: Slack bridge control runtime
// Invariant: identity probes send bound tokens only to operator-owned API destinations and report bot/scope results safely.
// Boundary IN: isolated control peer, process endpoint override, bound secrets, and Slack identity response.
// Boundary OUT: the real Slack API, owned by release QA.
// not parallel: the operator-owned endpoint regression changes the process-level Slack API environment.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func TestSlackIdentityChecksReportBotAndScopeFailures(t *testing.T) {
	t.Run("Should reject a user token before reporting scopes", func(t *testing.T) {
		checks := slackIdentityChecks(&slackAuthIdentity{
			UserID:        "U123",
			GrantedScopes: bridgepkg.SlackBotScopes(),
		}, nil)
		if len(checks) == 0 {
			t.Fatal("slackIdentityChecks() returned no checks")
		}
		identity := checks[0]
		if identity.Status != bridgepkg.BridgeCheckStatusFail {
			t.Fatalf("identity status = %q, want fail", identity.Status)
		}
		if !strings.Contains(identity.Remediation, "user token, expected bot token") {
			t.Fatalf("identity remediation = %q, want bot-token diagnosis", identity.Remediation)
		}
	})

	t.Run("Should name every missing scope and require reinstall", func(t *testing.T) {
		granted := slices.DeleteFunc(
			bridgepkg.SlackBotScopes(),
			func(scope string) bool { return scope == "mpim:history" },
		)
		checks := slackIdentityChecks(&slackAuthIdentity{
			BotID:         "B123",
			UserID:        "U123",
			GrantedScopes: granted,
		}, nil)

		var missing *bridgepkg.BridgeCheckRecord
		for index := range checks {
			if checks[index].Check == "scope.mpim:history" {
				missing = &checks[index]
				break
			}
		}
		if missing == nil {
			t.Fatalf("slackIdentityChecks() = %#v, want mpim:history result", checks)
		}
		if missing.Status != bridgepkg.BridgeCheckStatusFail {
			t.Fatalf("missing scope status = %q, want fail", missing.Status)
		}
		if !strings.Contains(missing.Remediation, "mpim:history") ||
			!strings.Contains(strings.ToLower(missing.Remediation), "reinstall") {
			t.Fatalf("missing scope remediation = %q, want scope and reinstall", missing.Remediation)
		}
	})
}

func TestSlackControlRuntimeReturnsProviderAndWebhookChecks(t *testing.T) {
	t.Run("Should answer the isolated check contract for a disabled instance", func(t *testing.T) {
		provider, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		provider.apiFactory = func(*resolvedInstanceConfig) slackAPI {
			return &slackControlAPI{fakeSlackAPI: &fakeSlackAPI{}}
		}
		now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
		managed := testBridgeRuntime(now, "brg-slack-control")
		managed.Instance.Enabled = false
		managed.Instance.Status = bridgepkg.BridgeStatusDisabled
		managed.Instance.ProviderConfig = []byte(
			`{"webhook":{"public_url":"https://bridge.example.test/slack/support"}}`,
		)
		if err := hostPeer.Call(
			context.Background(),
			"initialize",
			slackControlInitializeRequest(now, managed),
			nil,
		); err != nil {
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
		assertSlackControlCheck(t, response.Checks, "provider.identity", bridgepkg.BridgeCheckStatusPass)
		assertSlackControlCheck(t, response.Checks, "webhook.configuration", bridgepkg.BridgeCheckStatusPass)
		assertSlackControlCheck(t, response.Checks, "webhook.reachability", bridgepkg.BridgeCheckStatusSkipped)
	})
}

func TestSlackControlCredentialDestinationIsOperatorOwned(t *testing.T) {
	t.Run("Should reject a provider configured API URL without dialing or sending the token", func(t *testing.T) {
		requests := make(chan string, 1)
		attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			requests <- request.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(`{"ok":true,"user_id":"U_ATTACKER","bot_id":"B_ATTACKER"}`)); err != nil {
				t.Errorf("Write() error = %v", err)
			}
		}))
		defer attacker.Close()

		_, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		now := time.Date(2026, 7, 12, 12, 15, 0, 0, time.UTC)
		managed := testBridgeRuntime(now, "brg-slack-untrusted-endpoint")
		managed.Instance.Enabled = false
		managed.Instance.Status = bridgepkg.BridgeStatusDisabled
		managed.Instance.ProviderConfig = fmt.Appendf(nil,
			`{"api_base_url":%q,"webhook":{"public_url":"https://bridge.example.test/slack/support"}}`,
			attacker.URL,
		)

		err := hostPeer.Call(
			context.Background(),
			"initialize",
			slackControlInitializeRequest(now, managed),
			nil,
		)
		if err == nil {
			t.Fatal("hostPeer.Call(initialize) error = nil, want operator-owned endpoint rejection")
		}
		if strings.Contains(err.Error(), "xoxb-slack-token") {
			t.Fatalf("initialize error leaked bot token: %q", err)
		}
		select {
		case authorization := <-requests:
			t.Fatalf("attacker received Authorization %q, want zero requests", authorization)
		default:
		}
	})

	t.Run("Should send the token only to the process configured API URL", func(t *testing.T) {
		requests := make(chan string, 1)
		trusted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/auth.test" {
				t.Errorf("request path = %q, want /auth.test", request.URL.Path)
			}
			requests <- request.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-OAuth-Scopes", strings.Join(bridgepkg.SlackBotScopes(), ","))
			if _, err := w.Write([]byte(`{"ok":true,"user_id":"U_BOT","bot_id":"B_BOT"}`)); err != nil {
				t.Errorf("Write() error = %v", err)
			}
		}))
		defer trusted.Close()
		t.Setenv(slackAPIBaseEnv, trusted.URL)

		_, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		now := time.Date(2026, 7, 12, 12, 20, 0, 0, time.UTC)
		managed := testBridgeRuntime(now, "brg-slack-operator-endpoint")
		managed.Instance.Enabled = false
		managed.Instance.Status = bridgepkg.BridgeStatusDisabled
		managed.Instance.ProviderConfig = []byte(
			`{"webhook":{"public_url":"https://bridge.example.test/slack/support"}}`,
		)
		if err := hostPeer.Call(
			context.Background(),
			"initialize",
			slackControlInitializeRequest(now, managed),
			nil,
		); err != nil {
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
		select {
		case authorization := <-requests:
			if authorization != "Bearer xoxb-slack-token" {
				t.Fatalf("Authorization = %q, want bound bot token", authorization)
			}
		default:
			t.Fatal("operator-owned API endpoint received no identity request")
		}
	})
}

type slackControlAPI struct{ *fakeSlackAPI }

func (a *slackControlAPI) AuthTest(context.Context) (*slackAuthIdentity, error) {
	return &slackAuthIdentity{
		UserID:        "U_BOT",
		BotID:         "B_BOT",
		GrantedScopes: bridgepkg.SlackBotScopes(),
	}, nil
}

func slackControlInitializeRequest(
	now time.Time,
	managed subprocess.InitializeBridgeManagedInstance,
) subprocess.InitializeRequest {
	request := testInitializeRequest(now, managed)
	request.Capabilities = subprocess.InitializeCapabilities{}
	request.Methods = subprocess.InitializeMethods{
		DaemonRequests:    []string{"shutdown"},
		ExtensionServices: []string{string(bridgepkg.ControlMethodCheck)},
	}
	request.Runtime.Bridge.Purpose = subprocess.BridgeRuntimePurposeControl
	request.Runtime.Bridge.AllowedMethods = []string{string(bridgepkg.ControlMethodCheck)}
	return request
}

func assertSlackControlCheck(
	t *testing.T,
	checks []bridgepkg.BridgeCheckRecord,
	name string,
	status bridgepkg.BridgeCheckStatus,
) {
	t.Helper()
	for _, check := range checks {
		if check.Check == name {
			if check.Status != status {
				t.Fatalf("check %q status = %q, want %q", name, check.Status, status)
			}
			return
		}
	}
	t.Fatalf("checks = %#v, want %q", checks, name)
}
