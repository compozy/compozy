// Suite: Telegram bridge control runtime
// Invariant: identity verification and webhook registration preserve safe endpoint, secret, and provider-error semantics.
// Boundary IN: isolated control peer, operator-owned API transport, bound secrets, and provider outcomes.
// Boundary OUT: the real Telegram Bot API, owned by release QA.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func TestTelegramControlOperationsUseTheBotAPI(t *testing.T) {
	t.Run("Should register the configured public webhook and secret", func(t *testing.T) {
		var captured telegramSetWebhookRequest
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got, want := r.Method, http.MethodPost; got != want {
				t.Errorf("method = %q, want %q", got, want)
			}
			if got, want := r.URL.Path, "/bot123456:abcdefghijklmnopqrstuvwxyzABCDE/setWebhook"; got != want {
				t.Errorf("path = %q, want %q", got, want)
			}
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Errorf("Decode() error = %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(`{"ok":true,"result":true}`)); err != nil {
				t.Errorf("Write() error = %v", err)
			}
		}))
		defer server.Close()

		client := &telegramBotClient{
			baseURL:    server.URL,
			botToken:   "123456:abcdefghijklmnopqrstuvwxyzABCDE",
			httpClient: server.Client(),
		}
		err := client.SetWebhook(context.Background(), telegramSetWebhookRequest{
			URL:                "https://bridge.example.test/telegram/support",
			SecretToken:        "generated-webhook-secret",
			DropPendingUpdates: true,
		})
		if err != nil {
			t.Fatalf("SetWebhook() error = %v", err)
		}
		if got, want := captured.URL, "https://bridge.example.test/telegram/support"; got != want {
			t.Fatalf("captured URL = %q, want %q", got, want)
		}
		if got, want := captured.SecretToken, "generated-webhook-secret"; got != want {
			t.Fatalf("captured SecretToken = %q, want %q", got, want)
		}
		if !captured.DropPendingUpdates {
			t.Fatal("captured DropPendingUpdates = false, want true")
		}
	})

	t.Run("Should map an invalid identity to the bot token binding", func(t *testing.T) {
		t.Parallel()

		checks := telegramIdentityChecks(nil, errors.New("invalid bot token"))
		if len(checks) != 1 {
			t.Fatalf("telegramIdentityChecks() = %#v, want one check", checks)
		}
		if checks[0].Status != bridgepkg.BridgeCheckStatusFail {
			t.Fatalf("status = %q, want fail", checks[0].Status)
		}
		if !strings.Contains(checks[0].Remediation, "bot_token") {
			t.Fatalf("remediation = %q, want bot_token", checks[0].Remediation)
		}
	})

	t.Run("Should register through the isolated runtime control contract", func(t *testing.T) {
		t.Parallel()

		provider, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		api := &recordingTelegramControlAPI{fakeTelegramAPI: &fakeTelegramAPI{nextMessageID: 1}}
		provider.apiFactory = func(*resolvedInstanceConfig) telegramAPI { return api }

		now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
		managed := testBridgeRuntime(now, "brg-telegram-control")
		managed.Instance.Enabled = false
		managed.Instance.Status = bridgepkg.BridgeStatusDisabled
		managed.Instance.ProviderConfig = []byte(
			`{"webhook":{"public_url":"https://bridge.example.test/telegram/support"}}`,
		)
		if err := hostPeer.Call(
			context.Background(),
			"initialize",
			telegramControlInitializeRequest(managed, bridgepkg.ControlMethodWebhookRegister),
			nil,
		); err != nil {
			t.Fatalf("hostPeer.Call(initialize) error = %v", err)
		}

		var response bridgepkg.BridgeWebhookRegistrationResponse
		if err := hostPeer.Call(
			context.Background(),
			string(bridgepkg.ControlMethodWebhookRegister),
			bridgepkg.BridgeWebhookRegistrationRequest{BridgeInstanceID: managed.Instance.ID},
			&response,
		); err != nil {
			t.Fatalf("hostPeer.Call(webhook register) error = %v", err)
		}
		if response.Status != bridgepkg.BridgeCheckStatusPass {
			t.Fatalf("registration response = %#v, want pass", response)
		}
		if api.webhook.URL != "https://bridge.example.test/telegram/support" ||
			api.webhook.SecretToken != "top-secret" || !api.webhook.DropPendingUpdates {
			t.Fatalf("captured webhook = %#v", api.webhook)
		}
	})

	t.Run("Should return identity and webhook checks through the isolated runtime", func(t *testing.T) {
		t.Parallel()

		provider, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		api := &recordingTelegramControlAPI{fakeTelegramAPI: &fakeTelegramAPI{nextMessageID: 1}}
		provider.apiFactory = func(*resolvedInstanceConfig) telegramAPI { return api }
		now := time.Date(2026, 7, 12, 12, 5, 0, 0, time.UTC)
		managed := testBridgeRuntime(now, "brg-telegram-check")
		managed.Instance.Enabled = false
		managed.Instance.Status = bridgepkg.BridgeStatusDisabled
		managed.Instance.ProviderConfig = []byte(
			`{"webhook":{"public_url":"https://bridge.example.test/telegram/check"}}`,
		)
		if err := hostPeer.Call(
			context.Background(),
			"initialize",
			telegramControlInitializeRequest(managed, bridgepkg.ControlMethodCheck),
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
		if !telegramCheckHasStatus(response.Checks, "provider.identity", bridgepkg.BridgeCheckStatusPass) ||
			!telegramCheckHasStatus(response.Checks, "webhook.reachability", bridgepkg.BridgeCheckStatusSkipped) {
			t.Fatalf("checks = %#v, want identity pass and reachability skipped", response.Checks)
		}
	})

	t.Run("Should refuse webhook replacement while the bridge is enabled", func(t *testing.T) {
		t.Parallel()

		provider, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		provider.apiFactory = func(*resolvedInstanceConfig) telegramAPI {
			return &recordingTelegramControlAPI{fakeTelegramAPI: &fakeTelegramAPI{nextMessageID: 1}}
		}
		now := time.Date(2026, 7, 12, 12, 10, 0, 0, time.UTC)
		managed := testBridgeRuntime(now, "brg-telegram-enabled")
		if err := hostPeer.Call(
			context.Background(),
			"initialize",
			telegramControlInitializeRequest(managed, bridgepkg.ControlMethodWebhookRegister),
			nil,
		); err != nil {
			t.Fatalf("hostPeer.Call(initialize) error = %v", err)
		}

		var response bridgepkg.BridgeWebhookRegistrationResponse
		if err := hostPeer.Call(
			context.Background(),
			string(bridgepkg.ControlMethodWebhookRegister),
			bridgepkg.BridgeWebhookRegistrationRequest{BridgeInstanceID: managed.Instance.ID},
			&response,
		); err != nil {
			t.Fatalf("hostPeer.Call(webhook register) error = %v", err)
		}
		if response.Status != bridgepkg.BridgeCheckStatusFail ||
			!strings.Contains(response.Remediation, "Disable") {
			t.Fatalf("registration response = %#v, want disabled-state remediation", response)
		}
	})

	t.Run("Should not expose the token-bearing request URL on transport failure", func(t *testing.T) {
		t.Parallel()

		const token = "123456:abcdefghijklmnopqrstuvwxyzABCDE"
		client := &telegramBotClient{
			baseURL:  "https://api.telegram.test",
			botToken: token,
			httpClient: &http.Client{Transport: telegramRoundTripperFunc(func(
				request *http.Request,
			) (*http.Response, error) {
				return nil, errors.New(request.URL.String())
			})},
		}
		_, err := client.GetMe(context.Background())
		if err == nil {
			t.Fatal("GetMe() error = nil, want transport failure")
		}
		if strings.Contains(err.Error(), token) {
			t.Fatalf("GetMe() error = %q, want token redacted", err)
		}
	})
}

func TestTelegramControlIdentityPreservesProviderErrorClasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bridgepkg.BridgeCheckStatus
	}{
		{
			name: "Should fail an authentication error",
			err:  &bridgesdk.AuthError{Err: errors.New("invalid leaked-telegram-token")},
			want: bridgepkg.BridgeCheckStatusFail,
		},
		{
			name: "Should fail a permanent error",
			err:  &bridgesdk.PermanentError{Err: errors.New("bad leaked-telegram-token configuration")},
			want: bridgepkg.BridgeCheckStatusFail,
		},
		{
			name: "Should warn on rate limiting",
			err:  &bridgesdk.RateLimitError{Err: errors.New("leaked-telegram-token rate limited")},
			want: bridgepkg.BridgeCheckStatusWarn,
		},
		{
			name: "Should warn on timeout",
			err:  context.DeadlineExceeded,
			want: bridgepkg.BridgeCheckStatusWarn,
		},
		{
			name: "Should warn on a transient error",
			err:  &bridgesdk.TransientError{Err: errors.New("leaked-telegram-token unavailable")},
			want: bridgepkg.BridgeCheckStatusWarn,
		},
		{
			name: "Should warn when canceled",
			err:  context.Canceled,
			want: bridgepkg.BridgeCheckStatusWarn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider, hostPeer, cleanup := newRuntimePeerPair(t)
			defer cleanup()
			provider.apiFactory = func(*resolvedInstanceConfig) telegramAPI {
				return fakeTelegramAPIError{err: tt.err}
			}
			now := time.Date(2026, 7, 12, 12, 25, 0, 0, time.UTC)
			managed := testBridgeRuntime(now, "brg-telegram-identity-outcome")
			managed.Instance.Enabled = false
			managed.Instance.Status = bridgepkg.BridgeStatusDisabled
			managed.Instance.ProviderConfig = []byte(
				`{"webhook":{"public_url":"https://bridge.example.test/telegram/support"}}`,
			)
			if err := hostPeer.Call(
				context.Background(),
				"initialize",
				telegramControlInitializeRequest(managed, bridgepkg.ControlMethodCheck),
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
			if !telegramCheckHasStatus(response.Checks, "provider.identity", tt.want) {
				t.Fatalf("checks = %#v, want provider.identity %q", response.Checks, tt.want)
			}
			for _, check := range response.Checks {
				if strings.Contains(check.Remediation, "leaked-telegram-token") {
					t.Fatalf("remediation leaked provider error text: %q", check.Remediation)
				}
			}
		})
	}
}

func TestTelegramWebhookRegistrationReportsSafeOutcomes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configure func(*subprocess.InitializeBridgeManagedInstance)
		api       telegramAPI
		want      bridgepkg.BridgeCheckStatus
		wantErr   bool
	}{
		{
			name: "Should reject an internal public URL before calling Telegram",
			configure: func(managed *subprocess.InitializeBridgeManagedInstance) {
				managed.Instance.ProviderConfig = []byte(
					`{"webhook":{"public_url":"https://169.254.169.254/latest/meta-data"}}`,
				)
			},
			api:  &recordingTelegramControlAPI{fakeTelegramAPI: &fakeTelegramAPI{}},
			want: bridgepkg.BridgeCheckStatusFail,
		},
		{
			name: "Should fail safely when the webhook secret is missing",
			configure: func(managed *subprocess.InitializeBridgeManagedInstance) {
				managed.BoundSecrets = slices.DeleteFunc(
					managed.BoundSecrets,
					func(secret subprocess.InitializeBridgeBoundSecret) bool {
						return secret.BindingName == "webhook_secret"
					},
				)
			},
			api:  &recordingTelegramControlAPI{fakeTelegramAPI: &fakeTelegramAPI{}},
			want: bridgepkg.BridgeCheckStatusFail,
		},
		{
			name: "Should warn when Telegram times out after registration",
			api: &recordingTelegramControlAPI{
				fakeTelegramAPI: &fakeTelegramAPI{},
				err:             context.DeadlineExceeded,
			},
			want: bridgepkg.BridgeCheckStatusWarn,
		},
		{
			name: "Should fail a permanent Telegram registration error",
			api: &recordingTelegramControlAPI{
				fakeTelegramAPI: &fakeTelegramAPI{},
				err:             errors.New("registration rejected"),
			},
			want: bridgepkg.BridgeCheckStatusFail,
		},
		{
			name:    "Should reject a provider API without webhook registration support",
			api:     &fakeTelegramAPI{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider, hostPeer, cleanup := newRuntimePeerPair(t)
			defer cleanup()
			provider.apiFactory = func(*resolvedInstanceConfig) telegramAPI { return tt.api }
			now := time.Date(2026, 7, 12, 12, 30, 0, 0, time.UTC)
			managed := testBridgeRuntime(now, "brg-telegram-registration-outcome")
			managed.Instance.Enabled = false
			managed.Instance.Status = bridgepkg.BridgeStatusDisabled
			managed.Instance.ProviderConfig = []byte(
				`{"webhook":{"public_url":"https://bridge.example.test/telegram/support"}}`,
			)
			if tt.configure != nil {
				tt.configure(&managed)
			}
			if err := hostPeer.Call(
				context.Background(),
				"initialize",
				telegramControlInitializeRequest(managed, bridgepkg.ControlMethodWebhookRegister),
				nil,
			); err != nil {
				t.Fatalf("hostPeer.Call(initialize) error = %v", err)
			}

			var response bridgepkg.BridgeWebhookRegistrationResponse
			err := hostPeer.Call(
				context.Background(),
				string(bridgepkg.ControlMethodWebhookRegister),
				bridgepkg.BridgeWebhookRegistrationRequest{BridgeInstanceID: managed.Instance.ID},
				&response,
			)
			if tt.wantErr {
				if err == nil {
					t.Fatal("hostPeer.Call(webhook register) error = nil, want unsupported API error")
				}
				return
			}
			if err != nil {
				t.Fatalf("hostPeer.Call(webhook register) error = %v", err)
			}
			if response.Status != tt.want {
				t.Fatalf("registration response = %#v, want status %q", response, tt.want)
			}
		})
	}
}

type telegramRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f telegramRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type recordingTelegramControlAPI struct {
	*fakeTelegramAPI
	webhook telegramSetWebhookRequest
	err     error
}

func (a *recordingTelegramControlAPI) SetWebhook(
	_ context.Context,
	request telegramSetWebhookRequest,
) error {
	a.webhook = request
	return a.err
}

func telegramControlInitializeRequest(
	managed subprocess.InitializeBridgeManagedInstance,
	method bridgepkg.ControlMethod,
) subprocess.InitializeRequest {
	return subprocess.InitializeRequest{
		ProtocolVersion:          "1",
		SupportedProtocolVersion: []string{"1"},
		AGHVersion:               "test",
		SessionNonce:             "nonce-telegram-control",
		Extension: subprocess.InitializeExtension{
			Name:       "telegram",
			Version:    "0.1.0",
			SourceTier: "bundled",
		},
		Methods: subprocess.InitializeMethods{
			DaemonRequests:    []string{"shutdown"},
			ExtensionServices: []string{string(method)},
		},
		Runtime: subprocess.InitializeRuntime{
			HealthCheckIntervalMS: 1_000,
			HealthCheckTimeoutMS:  1_000,
			ShutdownTimeoutMS:     1_000,
			DefaultHookTimeoutMS:  1_000,
			Bridge: &subprocess.InitializeBridgeRuntime{
				RuntimeVersion:   subprocess.InitializeBridgeRuntimeVersion2,
				Purpose:          subprocess.BridgeRuntimePurposeControl,
				Provider:         "telegram",
				Platform:         "telegram",
				AllowedMethods:   []string{string(method)},
				ManagedInstances: []subprocess.InitializeBridgeManagedInstance{managed},
			},
		},
	}
}

func telegramCheckHasStatus(
	checks []bridgepkg.BridgeCheckRecord,
	name string,
	status bridgepkg.BridgeCheckStatus,
) bool {
	for _, check := range checks {
		if check.Check == name && check.Status == status {
			return true
		}
	}
	return false
}
