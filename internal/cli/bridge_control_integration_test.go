//go:build integration

// Suite: bridge setup and control CLI integration
// Invariant: setup, webhook registration, verify, and real-send round-trip through daemon UDS and persisted state.
// Boundary IN: Cobra CLI, unix client, shared API handlers, SQLite bridge registry, and control/delivery transports.
// Boundary OUT: live provider credentials, replaced by explicit fake provider boundaries.
package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func TestBridgeSetupVerifyAndSendIntegration(t *testing.T) {
	t.Run("Should persist headless setup and round trip provider control without lifecycle mutation", func(t *testing.T) {
		t.Parallel()

		h := newIntegrationHarness(t)
		h.runner.bridgeProviders = []bridgepkg.BridgeProvider{{
			Platform:      "telegram",
			ExtensionName: "telegram",
			DisplayName:   "Telegram",
		}}
		mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
		defer stopBridgeControlIntegrationDaemon(t, &h)

		const botToken = "123456:abcdefghijklmnopqrstuvwxyzABCDE"
		const webhookSecret = "telegram-webhook-secret-value"
		setupInput := `{
  "scope": "global",
  "webhook_public_url": "https://bridge.example.test/telegram/support",
  "webhook_path": "/telegram/support",
  "bot_token": "` + botToken + `",
  "webhook_secret": "` + webhookSecret + `"
}`
		setupOut, setupErr, err := executeRootCommandWithInput(
			t,
			h.deps,
			setupInput,
			"bridge", "setup", "telegram", "--json", "--print-only",
		)
		if err != nil {
			t.Fatalf("bridge setup error = %v; stderr=%s", err, setupErr)
		}
		if strings.Contains(setupOut, botToken) || strings.Contains(setupOut, webhookSecret) {
			t.Fatalf("bridge setup output leaked supplied secret: %s", setupOut)
		}
		var setup bridgeSetupResult
		if err := json.Unmarshal([]byte(setupOut), &setup); err != nil {
			t.Fatalf("json.Unmarshal(setup) error = %v; output=%s", err, setupOut)
		}
		if setup.Status != bridgepkg.BridgeStatusDisabled || setup.WebhookCommand == "" ||
			setup.NextCommand != "agh bridge verify "+setup.BridgeInstanceID {
			t.Fatalf("setup result = %#v", setup)
		}

		getOut := mustExecuteRoot(t, h.deps, "bridge", "get", setup.BridgeInstanceID, "--json")
		var before BridgeRecord
		if err := json.Unmarshal([]byte(getOut), &before); err != nil {
			t.Fatalf("json.Unmarshal(bridge get) error = %v", err)
		}
		if before.Enabled || before.Status != bridgepkg.BridgeStatusDisabled {
			t.Fatalf("bridge before verify = %#v, want disabled", before)
		}

		bindingsOut := mustExecuteRoot(
			t,
			h.deps,
			"bridge", "secret-bindings", "list", setup.BridgeInstanceID, "--json",
		)
		var bindings struct {
			Bindings []BridgeSecretBindingRecord `json:"bindings"`
		}
		if err := json.Unmarshal([]byte(bindingsOut), &bindings); err != nil {
			t.Fatalf("json.Unmarshal(bindings) error = %v", err)
		}
		if len(bindings.Bindings) != 2 {
			t.Fatalf("bindings = %#v, want bot_token and webhook_secret", bindings.Bindings)
		}

		fakeTelegramCalls := 0
		fakeTelegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			fakeTelegramCalls++
			if request.Method != http.MethodPost || request.URL.Path != "/setWebhook" {
				t.Errorf("fake Telegram request = %s %s", request.Method, request.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			if _, err := io.WriteString(w, `{"ok":true,"result":true}`); err != nil {
				t.Errorf("fake Telegram response write error = %v", err)
			}
		}))
		defer fakeTelegram.Close()
		h.runner.bridges.registerWebhookFn = func(
			ctx context.Context,
			extensionName string,
			request bridgepkg.BridgeWebhookRegistrationRequest,
		) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
			if extensionName != "telegram" || request.BridgeInstanceID != setup.BridgeInstanceID {
				t.Fatalf("RegisterBridgeWebhook() extension=%q request=%#v", extensionName, request)
			}
			httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, fakeTelegram.URL+"/setWebhook", http.NoBody)
			if err != nil {
				return bridgepkg.BridgeWebhookRegistrationResponse{}, err
			}
			response, err := fakeTelegram.Client().Do(httpRequest)
			if err != nil {
				return bridgepkg.BridgeWebhookRegistrationResponse{}, err
			}
			if closeErr := response.Body.Close(); closeErr != nil {
				return bridgepkg.BridgeWebhookRegistrationResponse{}, closeErr
			}
			return bridgepkg.BridgeWebhookRegistrationResponse{Status: bridgepkg.BridgeCheckStatusPass}, nil
		}
		client, err := clientFromDeps(h.deps)
		if err != nil {
			t.Fatalf("clientFromDeps() error = %v", err)
		}
		registrationClient, ok := client.(bridgeSetupClient)
		if !ok {
			t.Fatal("daemon client does not implement bridgeSetupClient")
		}
		registration, err := registrationClient.RegisterBridgeWebhook(t.Context(), setup.BridgeInstanceID)
		if err != nil {
			t.Fatalf("RegisterBridgeWebhook() error = %v", err)
		}
		if registration.Status != bridgepkg.BridgeCheckStatusPass || fakeTelegramCalls != 1 {
			t.Fatalf("registration=%#v fake calls=%d, want pass/1", registration, fakeTelegramCalls)
		}

		h.runner.bridges.checkBridgeFn = func(
			_ context.Context,
			extensionName string,
			request bridgepkg.BridgeCheckRequest,
		) (bridgepkg.BridgeCheckResponse, error) {
			if extensionName != "telegram" || request.BridgeInstanceID != setup.BridgeInstanceID {
				t.Fatalf("CheckBridge() extension=%q request=%#v", extensionName, request)
			}
			return bridgepkg.BridgeCheckResponse{Checks: []bridgepkg.BridgeCheckRecord{
				{Check: "provider.identity", Status: bridgepkg.BridgeCheckStatusPass},
				{
					Check: "webhook.reachability", Status: bridgepkg.BridgeCheckStatusSkipped,
					Remediation: "Enable the bridge before testing reachability.",
				},
			}}, nil
		}
		verifyOut := mustExecuteRoot(
			t,
			h.deps,
			"bridge", "verify", setup.BridgeInstanceID, "--json",
		)
		var verified BridgeVerifyRecord
		if err := json.Unmarshal([]byte(verifyOut), &verified); err != nil {
			t.Fatalf("json.Unmarshal(verify) error = %v", err)
		}
		if len(verified.Checks) != 2 || verified.Checks[1].Status != bridgepkg.BridgeCheckStatusSkipped {
			t.Fatalf("verify response = %#v", verified)
		}

		afterOut := mustExecuteRoot(t, h.deps, "bridge", "get", setup.BridgeInstanceID, "--json")
		var after BridgeRecord
		if err := json.Unmarshal([]byte(afterOut), &after); err != nil {
			t.Fatalf("json.Unmarshal(bridge after verify) error = %v", err)
		}
		if after.Enabled != before.Enabled || after.Status != before.Status {
			t.Fatalf("verify changed lifecycle: before=%#v after=%#v", before, after)
		}

		platformSends := 0
		h.runner.bridges.deliverBridgeFn = func(
			_ context.Context,
			extensionName string,
			request bridgepkg.DeliveryRequest,
		) (bridgepkg.DeliveryAck, error) {
			platformSends++
			if extensionName != "telegram" || request.Event.Content.Text != "Connection check" {
				t.Fatalf("DeliverBridge() extension=%q request=%#v", extensionName, request)
			}
			return bridgepkg.DeliveryAck{
				DeliveryID:      request.Event.DeliveryID,
				Seq:             request.Event.Seq,
				RemoteMessageID: "remote-integration",
			}, nil
		}
		mustExecuteRoot(t, h.deps, "bridge", "enable", setup.BridgeInstanceID, "--json")
		sendOut := mustExecuteRoot(
			t,
			h.deps,
			"bridge", "send-test", setup.BridgeInstanceID,
			"--message", "Connection check",
			"--group-id", "-100123",
			"--thread-id", "42",
			"--mode", "direct-send",
			"--json",
		)
		var sent BridgeSendTestRecord
		if err := json.Unmarshal([]byte(sendOut), &sent); err != nil {
			t.Fatalf("json.Unmarshal(send-test) error = %v", err)
		}
		if platformSends != 1 || sent.RemoteMessageID != "remote-integration" {
			t.Fatalf("platform sends=%d response=%#v", platformSends, sent)
		}
	})
}

func stopBridgeControlIntegrationDaemon(t *testing.T, h *integrationHarness) {
	t.Helper()

	_, stderr, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
	if err != nil {
		t.Errorf("daemon stop error = %v; stderr=%s", err, stderr)
	}
	if err := h.runner.waitForExit(); err != nil {
		t.Errorf("integration daemon wait error = %v", err)
	}
}
