package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

// Suite: bridge setup commands
// Invariant: setup validates platform inputs, stores secrets write-only, and returns safe actionable output.
// Boundary IN: Cobra setup commands, validators, provider-config assembly, and daemon-client orchestration.
// Boundary OUT: daemon persistence and provider APIs, owned by the bridge API and provider integration suites.

func TestBridgeSetupWhatsAppValidators(t *testing.T) {
	t.Parallel()

	t.Run("Should accept valid WhatsApp field shapes", func(t *testing.T) {
		t.Parallel()

		for _, phoneNumberID := range []string{"123456789012345", "12345678901234567"} {
			if err := validateWhatsAppPhoneNumberID(phoneNumberID); err != nil {
				t.Fatalf("validateWhatsAppPhoneNumberID(%q) error = %v", phoneNumberID, err)
			}
		}
		if err := validateWhatsAppAccessToken("EAA" + strings.Repeat("a", 100)); err != nil {
			t.Fatalf("validateWhatsAppAccessToken() error = %v", err)
		}
		if err := validateWhatsAppAppSecret("0123456789abcdef0123456789abcdef"); err != nil {
			t.Fatalf("validateWhatsAppAppSecret() error = %v", err)
		}
	})

	t.Run("Should reject a phone number with the From dropdown remediation", func(t *testing.T) {
		t.Parallel()

		err := validateWhatsAppPhoneNumberID("15556422442")
		if err == nil || !strings.Contains(err.Error(), "look below the From dropdown") {
			t.Fatalf("validateWhatsAppPhoneNumberID(phone) error = %v, want From dropdown remediation", err)
		}
	})

	for _, test := range []struct {
		name    string
		value   string
		product string
	}{
		{name: "Should name an OpenAI key", value: "sk-not-meta", product: "OpenAI"},
		{name: "Should name a Slack token", value: "xoxb-not-meta", product: "Slack"},
		{name: "Should name a GitHub token", value: "ghp_not_meta", product: "GitHub"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateWhatsAppAccessToken(test.value)
			if err == nil || !strings.Contains(err.Error(), test.product) {
				t.Fatalf("validateWhatsAppAccessToken(%q) error = %v, want %s", test.value, err, test.product)
			}
		})
	}

	for _, test := range []struct {
		name     string
		validate func(string) error
		value    string
	}{
		{
			name:     "Should reject a fourteen-digit phone number id",
			validate: validateWhatsAppPhoneNumberID,
			value:    "12345678901234",
		},
		{
			name:     "Should reject a non-hex app secret",
			validate: validateWhatsAppAppSecret,
			value:    "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
		},
		{
			name:     "Should reject a short Meta access token",
			validate: validateWhatsAppAccessToken,
			value:    "EAAshort",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if err := test.validate(test.value); err == nil {
				t.Fatalf("validator(%q) error = nil, want validation failure", test.value)
			}
		})
	}
}

func TestBridgeSetupTelegramTokenValidator(t *testing.T) {
	t.Parallel()

	t.Run("Should accept a valid BotFather token", func(t *testing.T) {
		t.Parallel()

		if err := validateTelegramBotToken("123456789:" + strings.Repeat("A", 30)); err != nil {
			t.Fatalf("validateTelegramBotToken(valid) error = %v", err)
		}
	})

	t.Run("Should reject a truncated BotFather token", func(t *testing.T) {
		t.Parallel()

		if err := validateTelegramBotToken("123456789:short"); err == nil {
			t.Fatal("validateTelegramBotToken(truncated) error = nil, want validation failure")
		}
	})
}

func TestBridgeSetupWizardRunner(t *testing.T) {
	t.Parallel()

	t.Run("Should reprompt after a validator remediation", func(t *testing.T) {
		t.Parallel()

		var output bytes.Buffer
		selection, err := runBridgeSetupWizard(t.Context(), bridgeSetupWizardInput{
			Platform: bridgeSetupPlatformWhatsApp,
			Prompts: []bridgeSetupPrompt{
				{
					Key:      bridgeSetupPhoneNumberIDKey,
					Label:    "Meta Phone Number ID",
					Required: true,
					Validate: validateWhatsAppPhoneNumberID,
				},
			},
			Defaults: map[string]string{},
			Input:    strings.NewReader("15556422442\n123456789012345\n"),
			Output:   &output,
		})
		if err != nil {
			t.Fatalf("runBridgeSetupWizard() error = %v", err)
		}
		if selection.Values[bridgeSetupPhoneNumberIDKey] != "123456789012345" {
			t.Fatalf(
				"wizard phone_number_id = %q, want corrected Meta ID",
				selection.Values[bridgeSetupPhoneNumberIDKey],
			)
		}
		if !strings.Contains(output.String(), "look below the From dropdown") {
			t.Fatalf("wizard output missing validator remediation: %s", output.String())
		}
	})

	t.Run("Should never echo a secret read from redirected input", func(t *testing.T) {
		t.Parallel()

		const secret = "operator-supplied-secret"
		var output bytes.Buffer
		selection, err := runBridgeSetupWizard(t.Context(), bridgeSetupWizardInput{
			Platform: bridgeSetupPlatformDiscord,
			Prompts: []bridgeSetupPrompt{
				{
					Key:      bridgeSetupBotTokenKey,
					Label:    "Discord bot token",
					Secret:   true,
					Required: true,
				},
			},
			Defaults: map[string]string{},
			Input:    strings.NewReader(secret + "\n"),
			Output:   &output,
		})
		if err != nil {
			t.Fatalf("runBridgeSetupWizard() error = %v", err)
		}
		if selection.Values[bridgeSetupBotTokenKey] != secret {
			t.Fatalf("wizard bot token = %q, want supplied value", selection.Values[bridgeSetupBotTokenKey])
		}
		if strings.Contains(output.String(), secret) {
			t.Fatalf("wizard output leaked redirected secret: %s", output.String())
		}
	})

	t.Run("Should retain a masked existing binding on empty input", func(t *testing.T) {
		t.Parallel()

		var output bytes.Buffer
		selection, err := runBridgeSetupWizard(t.Context(), bridgeSetupWizardInput{
			Platform: bridgeSetupPlatformTelegram,
			Prompts: []bridgeSetupPrompt{
				{
					Key:      bridgeSetupBotTokenKey,
					Label:    "BotFather bot token",
					Secret:   true,
					Required: true,
				},
			},
			Defaults: map[string]string{bridgeSetupBotTokenKey: bridgeSetupMaskedSecret},
			Input:    strings.NewReader("\n"),
			Output:   &output,
		})
		if err != nil {
			t.Fatalf("runBridgeSetupWizard() error = %v", err)
		}
		if selection.Values[bridgeSetupBotTokenKey] != bridgeSetupMaskedSecret {
			t.Fatalf("wizard bot token default = %q, want mask", selection.Values[bridgeSetupBotTokenKey])
		}
	})
}

func TestBridgeSetupJSONCreatesWhatsAppInstanceAndMasksSecrets(t *testing.T) {
	t.Parallel()

	t.Run("Should create bind and return structured output without invoking prompts", func(t *testing.T) {
		t.Parallel()

		const (
			bridgeID      = "brg-whatsapp"
			accessToken   = "EAA" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			appSecret     = "0123456789abcdef0123456789abcdef"
			verifyToken   = "operator-supplied-verify-token"
			phoneNumberID = "123456789012345"
		)
		bound := make(map[string]string)
		client := &bridgeSetupStubClient{stubClient: &stubClient{
			createBridgeFn: func(_ context.Context, request CreateBridgeRequest) (BridgeRecord, error) {
				if request.Enabled {
					t.Fatal("CreateBridge() enabled = true, want disabled setup instance")
				}
				if request.Platform != "whatsapp" || request.ExtensionName != "whatsapp" {
					t.Fatalf(
						"CreateBridge() identity = %q/%q, want whatsapp/whatsapp",
						request.Platform,
						request.ExtensionName,
					)
				}
				var providerConfig struct {
					PhoneNumberID string `json:"phone_number_id"`
					Webhook       struct {
						Path      string `json:"path"`
						PublicURL string `json:"public_url"`
					} `json:"webhook"`
				}
				if err := json.Unmarshal(request.ProviderConfig, &providerConfig); err != nil {
					t.Fatalf("json.Unmarshal(provider config) error = %v", err)
				}
				if providerConfig.PhoneNumberID != phoneNumberID ||
					providerConfig.Webhook.Path != "/whatsapp/support" ||
					providerConfig.Webhook.PublicURL != "https://bridge.example.com/whatsapp/support" {
					t.Fatalf("provider config = %#v, want phone id and webhook coordinates", providerConfig)
				}
				return BridgeRecord{
					ID:             bridgeID,
					Platform:       request.Platform,
					ExtensionName:  request.ExtensionName,
					DisplayName:    request.DisplayName,
					ProviderConfig: json.RawMessage(request.ProviderConfig),
					Status:         bridgepkg.BridgeStatusDisabled,
				}, nil
			},
			putBridgeSecretBindingFn: func(
				_ context.Context,
				id string,
				bindingName string,
				request BridgeSecretBindingRequest,
			) (BridgeSecretBindingRecord, error) {
				if id != bridgeID || request.SecretValue == nil {
					t.Fatalf("PutBridgeSecretBinding() = id %q request %#v", id, request)
				}
				bound[bindingName] = *request.SecretValue
				return BridgeSecretBindingRecord{
					BridgeInstanceID: id,
					BindingName:      bindingName,
					SecretRef:        request.SecretRef,
					Kind:             request.Kind,
				}, nil
			},
		}}
		deps := newTestDeps(t, client)
		deps.runBridgeSetupWizard = func(
			context.Context,
			bridgeSetupWizardInput,
		) (bridgeSetupWizardSelection, error) {
			t.Fatal("interactive bridge setup runner called in --json mode")
			return bridgeSetupWizardSelection{}, nil
		}
		stdin := mustJSON(t, map[string]any{
			"scope":              "global",
			"display_name":       "WhatsApp support",
			"extension_name":     "whatsapp",
			"phone_number_id":    phoneNumberID,
			"access_token":       accessToken,
			"app_secret":         appSecret,
			"verify_token":       verifyToken,
			"webhook_public_url": "https://bridge.example.com/whatsapp/support",
			"webhook_path":       "/whatsapp/support",
		})

		stdout, _, err := executeRootCommandWithInput(
			t,
			deps,
			string(stdin),
			"bridge", "setup", "whatsapp", "--json",
		)
		if err != nil {
			t.Fatalf("bridge setup whatsapp --json error = %v", err)
		}
		if bound["access_token"] != accessToken || bound["app_secret"] != appSecret ||
			bound["verify_token"] != verifyToken {
			t.Fatalf("bound secrets = %#v, want supplied values", bound)
		}
		for _, secret := range []string{accessToken, appSecret, verifyToken} {
			if strings.Contains(stdout, secret) {
				t.Fatalf("bridge setup stdout leaked secret %q: %s", secret, stdout)
			}
		}
		var result bridgeSetupResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("json.Unmarshal(setup result) error = %v", err)
		}
		if result.BridgeInstanceID != bridgeID || result.Platform != "whatsapp" ||
			result.NextCommand != "agh bridge verify "+bridgeID {
			t.Fatalf("setup result = %#v, want bridge identity and exact verify command", result)
		}
		for _, slot := range []string{"access_token", "app_secret", "verify_token"} {
			if result.SecretSlots[slot] != bridgeSetupMaskedSecret {
				t.Fatalf("setup result slot %q = %q, want mask", slot, result.SecretSlots[slot])
			}
		}
		if !strings.Contains(result.VerificationCommand, "${WHATSAPP_VERIFY_TOKEN}") {
			t.Fatalf("verification command = %q, want safe token placeholder", result.VerificationCommand)
		}
	})
}

func TestBridgeSetupRevealsOnlyNewlyGeneratedSecretsWithExplicitConsent(t *testing.T) {
	t.Parallel()

	t.Run("Should reveal the generated verify token but never supplied credentials", func(t *testing.T) {
		t.Parallel()

		const (
			bridgeID    = "brg-whatsapp-reveal"
			accessToken = "EAA" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			appSecret   = "0123456789abcdef0123456789abcdef"
			generated   = "one-shot-generated-verify-token"
		)
		client := &bridgeSetupStubClient{stubClient: &stubClient{
			createBridgeFn: func(_ context.Context, request CreateBridgeRequest) (BridgeRecord, error) {
				return BridgeRecord{
					ID:             bridgeID,
					Scope:          bridgepkg.ScopeGlobal,
					Platform:       bridgeSetupPlatformWhatsApp,
					ExtensionName:  bridgeSetupPlatformWhatsApp,
					DisplayName:    request.DisplayName,
					ProviderConfig: json.RawMessage(request.ProviderConfig),
					Status:         bridgepkg.BridgeStatusDisabled,
				}, nil
			},
			putBridgeSecretBindingFn: func(
				_ context.Context,
				id string,
				bindingName string,
				request BridgeSecretBindingRequest,
			) (BridgeSecretBindingRecord, error) {
				return BridgeSecretBindingRecord{
					BridgeInstanceID: id,
					BindingName:      bindingName,
					SecretRef:        request.SecretRef,
					Kind:             request.Kind,
				}, nil
			},
		}}
		deps := newTestDeps(t, client)
		deps.generateBridgeSetupSecret = func() (string, error) { return generated, nil }
		stdin := mustJSON(t, map[string]any{
			"scope":              "global",
			"phone_number_id":    "123456789012345",
			"access_token":       accessToken,
			"app_secret":         appSecret,
			"webhook_public_url": "https://bridge.example.com/whatsapp/reveal",
			"webhook_path":       "/whatsapp/reveal",
		})

		stdout, _, err := executeRootCommandWithInput(
			t,
			deps,
			string(stdin),
			"bridge", "setup", "whatsapp", "--json", "--reveal-generated-secrets",
		)
		if err != nil {
			t.Fatalf("bridge setup whatsapp reveal error = %v", err)
		}
		var result bridgeSetupResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("json.Unmarshal(revealed setup result) error = %v", err)
		}
		if len(result.GeneratedSecrets) != 1 ||
			result.GeneratedSecrets[bridgeSetupVerifyTokenKey] != generated {
			t.Fatalf("GeneratedSecrets = %#v, want only the new verify token", result.GeneratedSecrets)
		}
		if !strings.Contains(result.VerificationCommand, generated) {
			t.Fatalf("VerificationCommand = %q, want one-shot generated token", result.VerificationCommand)
		}
		for _, supplied := range []string{accessToken, appSecret} {
			if strings.Contains(stdout, supplied) {
				t.Fatalf("revealed setup output leaked supplied credential %q: %s", supplied, stdout)
			}
		}
	})

	t.Run("Should reject a hidden generated verify token before writing the bridge", func(t *testing.T) {
		t.Parallel()

		const (
			accessToken = "EAA" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			appSecret   = "0123456789abcdef0123456789abcdef"
		)
		createCalls := 0
		client := &bridgeSetupStubClient{stubClient: &stubClient{
			createBridgeFn: func(context.Context, CreateBridgeRequest) (BridgeRecord, error) {
				createCalls++
				return BridgeRecord{}, nil
			},
		}}
		deps := newTestDeps(t, client)
		deps.generateBridgeSetupSecret = func() (string, error) {
			return "hidden-generated-verify-token", nil
		}
		stdin := mustJSON(t, map[string]any{
			"scope":              "global",
			"phone_number_id":    "123456789012345",
			"access_token":       accessToken,
			"app_secret":         appSecret,
			"webhook_public_url": "https://bridge.example.com/whatsapp/hidden",
			"webhook_path":       "/whatsapp/hidden",
		})

		stdout, _, err := executeRootCommandWithInput(
			t,
			deps,
			string(stdin),
			"bridge", "setup", "whatsapp", "--json",
		)
		if err == nil || !strings.Contains(err.Error(), "generated verify_token would become write-only") {
			t.Fatalf("bridge setup whatsapp hidden generation error = %v", err)
		}
		if createCalls != 0 {
			t.Fatalf("CreateBridge() calls = %d, want 0 before secret disclosure", createCalls)
		}
		if stdout != "" {
			t.Fatalf("bridge setup whatsapp hidden generation stdout = %q, want empty", stdout)
		}
	})
}

func TestBridgeSetupRerunKeepsMaskedSecretDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve existing bindings when masked defaults are accepted", func(t *testing.T) {
		t.Parallel()

		const bridgeID = "brg-existing"
		existing := BridgeRecord{
			ID:            bridgeID,
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "whatsapp",
			ExtensionName: "whatsapp",
			DisplayName:   "Existing WhatsApp",
			ProviderConfig: json.RawMessage(`{
				"phone_number_id":"123456789012345",
				"webhook":{
					"path":"/whatsapp/existing",
					"public_url":"https://bridge.example.com/whatsapp/existing"
				}
			}`),
			Status: bridgepkg.BridgeStatusDisabled,
		}
		bindings := []BridgeSecretBindingRecord{
			{
				BridgeInstanceID: bridgeID,
				BindingName:      bridgeSetupAccessTokenKey,
				SecretRef:        "vault:bridges/brg-existing/access_token",
				Kind:             bridgeSetupSecretKindToken,
			},
			{
				BridgeInstanceID: bridgeID,
				BindingName:      bridgeSetupAppSecretKey,
				SecretRef:        "vault:bridges/brg-existing/app_secret",
				Kind:             bridgeSetupSecretKindToken,
			},
			{
				BridgeInstanceID: bridgeID,
				BindingName:      bridgeSetupVerifyTokenKey,
				SecretRef:        "vault:bridges/brg-existing/verify_token",
				Kind:             bridgeSetupSecretKindToken,
			},
		}
		client := &bridgeSetupStubClient{stubClient: &stubClient{
			getBridgeFn: func(_ context.Context, id string) (BridgeRecord, error) {
				if id != bridgeID {
					t.Fatalf("GetBridge() id = %q, want %q", id, bridgeID)
				}
				return existing, nil
			},
			listBridgeSecretBindingsFn: func(_ context.Context, id string) ([]BridgeSecretBindingRecord, error) {
				if id != bridgeID {
					t.Fatalf("ListBridgeSecretBindings() id = %q, want %q", id, bridgeID)
				}
				return bindings, nil
			},
			putBridgeSecretBindingFn: func(
				context.Context,
				string,
				string,
				BridgeSecretBindingRequest,
			) (BridgeSecretBindingRecord, error) {
				t.Fatal("PutBridgeSecretBinding() called for an accepted masked default")
				return BridgeSecretBindingRecord{}, nil
			},
		}}
		deps := newTestDeps(t, client)
		deps.runBridgeSetupWizard = func(
			_ context.Context,
			input bridgeSetupWizardInput,
		) (bridgeSetupWizardSelection, error) {
			for _, slot := range []string{"access_token", "app_secret", "verify_token"} {
				if input.Defaults[slot] != bridgeSetupMaskedSecret {
					t.Fatalf("wizard default %q = %q, want mask", slot, input.Defaults[slot])
				}
			}
			return bridgeSetupWizardSelection{Values: input.Defaults}, nil
		}

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"bridge", "setup", "whatsapp", "--instance", bridgeID, "-o", "json",
		)
		if err != nil {
			t.Fatalf("bridge setup whatsapp rerun error = %v", err)
		}
		if strings.Contains(stdout, "vault:bridges") {
			t.Fatalf("bridge setup rerun output exposed secret reference details: %s", stdout)
		}
		var result bridgeSetupResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("json.Unmarshal(setup rerun) error = %v", err)
		}
		if result.NextCommand != "agh bridge verify "+bridgeID {
			t.Fatalf("NextCommand = %q, want exact verify command", result.NextCommand)
		}

		revealedStdout, _, revealErr := executeRootCommand(
			t,
			deps,
			"bridge", "setup", "whatsapp", "--instance", bridgeID,
			"-o", "json", "--reveal-generated-secrets",
		)
		if revealErr == nil || !strings.Contains(revealErr.Error(), "requires a secret generated during this setup") {
			t.Fatalf("bridge setup rerun reveal error = %v, want generated-only rejection", revealErr)
		}
		if revealedStdout != "" {
			t.Fatalf("bridge setup rerun reveal stdout = %q, want no disclosure", revealedStdout)
		}
	})
}

func TestBridgeSetupTelegramRoutingShapes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		policy bridgepkg.RoutingPolicy
	}{
		{
			name:   "Should persist a private chat route from JSON input",
			policy: bridgepkg.RoutingPolicy{IncludePeer: true},
		},
		{
			name:   "Should persist an ordinary group route from JSON input",
			policy: bridgepkg.RoutingPolicy{IncludeGroup: true},
		},
		{
			name: "Should preserve the forum route default through explicit JSON input",
			policy: bridgepkg.RoutingPolicy{
				IncludeGroup:  true,
				IncludeThread: true,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var created CreateBridgeRequest
			client := telegramRoutingSetupClient(t, &created)
			deps := newTestDeps(t, client)
			deps.generateBridgeSetupSecret = func() (string, error) {
				return "generated-webhook-secret", nil
			}
			stdin := mustJSON(t, map[string]any{
				"scope":              "global",
				"display_name":       "Telegram routing",
				"bot_token":          "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZabcd",
				"routing_policy":     test.policy,
				"webhook_public_url": "https://bridge.example.com/telegram/routing",
				"webhook_path":       "/telegram/routing",
			})

			if _, _, err := executeRootCommandWithInput(
				t,
				deps,
				string(stdin),
				"bridge", "setup", "telegram", "--json",
			); err != nil {
				t.Fatalf("bridge setup telegram error = %v", err)
			}
			if created.RoutingPolicy != test.policy {
				t.Fatalf("CreateBridge() routing policy = %#v, want %#v", created.RoutingPolicy, test.policy)
			}
		})
	}

	t.Run("Should persist a private chat route selected by the guided wizard", func(t *testing.T) {
		t.Parallel()

		var created CreateBridgeRequest
		client := telegramRoutingSetupClient(t, &created)
		deps := newTestDeps(t, client)
		deps.runBridgeSetupWizard = func(
			_ context.Context,
			input bridgeSetupWizardInput,
		) (bridgeSetupWizardSelection, error) {
			if input.Defaults["routing_shape"] != "forum" {
				t.Fatalf("wizard routing default = %q, want forum", input.Defaults["routing_shape"])
			}
			return bridgeSetupWizardSelection{Values: map[string]string{
				"scope":              "global",
				"display_name":       "Telegram private",
				"extension_name":     "telegram",
				"webhook_public_url": "https://bridge.example.com/telegram/private",
				"webhook_path":       "/telegram/private",
				"routing_shape":      "private",
				"bot_token":          "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZabcd",
				"webhook_secret":     "operator-webhook-secret",
			}}, nil
		}

		if _, _, err := executeRootCommand(
			t,
			deps,
			"bridge", "setup", "telegram", "-o", "json",
		); err != nil {
			t.Fatalf("guided bridge setup telegram error = %v", err)
		}
		want := bridgepkg.RoutingPolicy{IncludePeer: true}
		if created.RoutingPolicy != want {
			t.Fatalf("CreateBridge() routing policy = %#v, want %#v", created.RoutingPolicy, want)
		}
	})

	t.Run("Should reject a mixed Telegram route shape before creating an instance", func(t *testing.T) {
		t.Parallel()

		createCalls := 0
		client := &bridgeSetupStubClient{stubClient: &stubClient{
			createBridgeFn: func(context.Context, CreateBridgeRequest) (BridgeRecord, error) {
				createCalls++
				return BridgeRecord{}, nil
			},
		}}
		deps := newTestDeps(t, client)
		stdin := mustJSON(t, map[string]any{
			"scope":              "global",
			"bot_token":          "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZabcd",
			"routing_policy":     bridgepkg.RoutingPolicy{IncludePeer: true, IncludeGroup: true},
			"webhook_public_url": "https://bridge.example.com/telegram/mixed",
			"webhook_path":       "/telegram/mixed",
		})

		_, _, err := executeRootCommandWithInput(
			t,
			deps,
			string(stdin),
			"bridge", "setup", "telegram", "--json",
		)
		if err == nil || !strings.Contains(err.Error(), "private, group, or forum") {
			t.Fatalf("bridge setup Telegram mixed routing error = %v, want closed-shape remediation", err)
		}
		if createCalls != 0 {
			t.Fatalf("CreateBridge() calls = %d, want 0", createCalls)
		}
	})
}

func telegramRoutingSetupClient(
	t *testing.T,
	created *CreateBridgeRequest,
) *bridgeSetupStubClient {
	t.Helper()

	const bridgeID = "brg-telegram-routing"
	return &bridgeSetupStubClient{
		stubClient: &stubClient{
			createBridgeFn: func(_ context.Context, request CreateBridgeRequest) (BridgeRecord, error) {
				*created = request
				return BridgeRecord{
					ID:             bridgeID,
					Platform:       bridgeSetupPlatformTelegram,
					ExtensionName:  bridgeSetupPlatformTelegram,
					DisplayName:    request.DisplayName,
					ProviderConfig: json.RawMessage(request.ProviderConfig),
					RoutingPolicy:  request.RoutingPolicy,
					Status:         bridgepkg.BridgeStatusDisabled,
				}, nil
			},
			putBridgeSecretBindingFn: func(
				_ context.Context,
				id string,
				bindingName string,
				request BridgeSecretBindingRequest,
			) (BridgeSecretBindingRecord, error) {
				return BridgeSecretBindingRecord{
					BridgeInstanceID: id,
					BindingName:      bindingName,
					SecretRef:        request.SecretRef,
					Kind:             request.Kind,
				}, nil
			},
		},
		registerBridgeWebhookFn: func(
			context.Context,
			string,
		) (BridgeWebhookRegistrationRecord, error) {
			return BridgeWebhookRegistrationRecord{Status: bridgepkg.BridgeCheckStatusPass}, nil
		},
	}
}

func TestBridgeSetupTelegramRegistersWebhookOrPrintsSafeTemplate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name             string
		printOnly        bool
		wantRegisterCall int
	}{
		{name: "Should register the webhook through the daemon", printOnly: false, wantRegisterCall: 1},
		{name: "Should print a safe curl template without registering", printOnly: true, wantRegisterCall: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			const (
				bridgeID      = "brg-telegram"
				botToken      = "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZabcd"
				webhookSecret = "generated-webhook-secret"
			)
			registerCalls := 0
			client := &bridgeSetupStubClient{
				stubClient: &stubClient{
					createBridgeFn: func(_ context.Context, request CreateBridgeRequest) (BridgeRecord, error) {
						return BridgeRecord{
							ID:             bridgeID,
							Platform:       "telegram",
							ExtensionName:  "telegram",
							DisplayName:    request.DisplayName,
							ProviderConfig: json.RawMessage(request.ProviderConfig),
							Status:         bridgepkg.BridgeStatusDisabled,
						}, nil
					},
					putBridgeSecretBindingFn: func(
						_ context.Context,
						id string,
						bindingName string,
						request BridgeSecretBindingRequest,
					) (BridgeSecretBindingRecord, error) {
						return BridgeSecretBindingRecord{
							BridgeInstanceID: id,
							BindingName:      bindingName,
							SecretRef:        request.SecretRef,
							Kind:             request.Kind,
						}, nil
					},
				},
				registerBridgeWebhookFn: func(_ context.Context, id string) (BridgeWebhookRegistrationRecord, error) {
					registerCalls++
					if id != bridgeID {
						t.Fatalf("RegisterBridgeWebhook() id = %q, want %q", id, bridgeID)
					}
					return BridgeWebhookRegistrationRecord{Status: bridgepkg.BridgeCheckStatusPass}, nil
				},
			}
			deps := newTestDeps(t, client)
			deps.generateBridgeSetupSecret = func() (string, error) { return webhookSecret, nil }
			input := map[string]any{
				"scope":              "global",
				"display_name":       "Telegram support",
				"bot_token":          botToken,
				"webhook_public_url": "https://bridge.example.com/telegram/support",
				"webhook_path":       "/telegram/support",
			}
			if test.printOnly {
				input[bridgeSetupWebhookSecretKey] = webhookSecret
			}
			stdin := mustJSON(t, input)
			args := []string{"bridge", "setup", "telegram", "--json"}
			if test.printOnly {
				args = append(args, "--print-only")
			}

			stdout, _, err := executeRootCommandWithInput(t, deps, string(stdin), args...)
			if err != nil {
				t.Fatalf("bridge setup telegram error = %v", err)
			}
			if registerCalls != test.wantRegisterCall {
				t.Fatalf("RegisterBridgeWebhook() calls = %d, want %d", registerCalls, test.wantRegisterCall)
			}
			if strings.Contains(stdout, botToken) || strings.Contains(stdout, webhookSecret) {
				t.Fatalf("bridge setup telegram stdout leaked a secret: %s", stdout)
			}
			var result bridgeSetupResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("json.Unmarshal(telegram setup result) error = %v", err)
			}
			if test.printOnly {
				if result.WebhookCommand == "" ||
					!strings.Contains(result.WebhookCommand, "${TELEGRAM_BOT_TOKEN}") ||
					!strings.Contains(result.WebhookCommand, "${TELEGRAM_WEBHOOK_SECRET}") {
					t.Fatalf("WebhookCommand = %q, want safe exact curl template", result.WebhookCommand)
				}
			} else if result.WebhookRegistration == nil ||
				result.WebhookRegistration.Status != bridgepkg.BridgeCheckStatusPass {
				t.Fatalf("WebhookRegistration = %#v, want pass", result.WebhookRegistration)
			}
		})
	}

	t.Run("Should reject a hidden generated webhook secret before print-only writes", func(t *testing.T) {
		t.Parallel()

		createCalls := 0
		client := &bridgeSetupStubClient{stubClient: &stubClient{
			createBridgeFn: func(context.Context, CreateBridgeRequest) (BridgeRecord, error) {
				createCalls++
				return BridgeRecord{}, nil
			},
		}}
		deps := newTestDeps(t, client)
		deps.generateBridgeSetupSecret = func() (string, error) {
			return "hidden-generated-webhook-secret", nil
		}
		stdin := mustJSON(t, map[string]any{
			"scope":              "global",
			"bot_token":          "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZabcd",
			"webhook_public_url": "https://bridge.example.com/telegram/hidden",
			"webhook_path":       "/telegram/hidden",
		})

		stdout, _, err := executeRootCommandWithInput(
			t,
			deps,
			string(stdin),
			"bridge", "setup", "telegram", "--json", "--print-only",
		)
		if err == nil || !strings.Contains(err.Error(), "generated webhook_secret cannot be used with --print-only") {
			t.Fatalf("bridge setup Telegram hidden generation error = %v", err)
		}
		if createCalls != 0 {
			t.Fatalf("CreateBridge() calls = %d, want 0 before secret disclosure", createCalls)
		}
		if stdout != "" {
			t.Fatalf("bridge setup Telegram hidden generation stdout = %q, want empty", stdout)
		}
	})
}

func TestDiscordInviteURLUsesConfiguredScopesAndPermissions(t *testing.T) {
	t.Parallel()

	t.Run("Should build the Discord invite URL from instance config", func(t *testing.T) {
		t.Parallel()

		const applicationID = "111122223333444455"
		details, err := buildBridgeSetupProviderDetails("discord", bridgeSetupJSONInput{
			ProviderConfig: json.RawMessage(`{
				"application_id":"111122223333444455",
				"webhook":{"path":"/discord/support","public_url":"https://bridge.example.com/discord/support"},
				"invite":{"scopes":["applications.commands","bot"],"permissions":68672}
			}`),
		}, nil)
		if err != nil {
			t.Fatalf("buildBridgeSetupProviderDetails() error = %v", err)
		}
		inviteURL := details.InviteURL
		parsed, err := url.Parse(inviteURL)
		if err != nil {
			t.Fatalf("url.Parse(invite URL) error = %v", err)
		}
		if parsed.Scheme != "https" || parsed.Host != "discord.com" ||
			parsed.Path != "/oauth2/authorize" {
			t.Fatalf("invite URL endpoint = %s, want Discord OAuth authorize", inviteURL)
		}
		query := parsed.Query()
		if query.Get("client_id") != applicationID || query.Get("permissions") != "68672" ||
			query.Get("scope") != "applications.commands bot" {
			t.Fatalf("invite URL query = %#v, want configured client/scopes/permissions", query)
		}
	})

	t.Run("Should reject an invite without an application id", func(t *testing.T) {
		t.Parallel()

		if _, err := buildDiscordInviteURL("", []string{"bot"}, 68672); err == nil {
			t.Fatal("buildDiscordInviteURL(empty application id) error = nil, want validation failure")
		}
	})
}

type bridgeSetupStubClient struct {
	*stubClient
	registerBridgeWebhookFn func(context.Context, string) (BridgeWebhookRegistrationRecord, error)
}

func (s *bridgeSetupStubClient) RegisterBridgeWebhook(
	ctx context.Context,
	id string,
) (BridgeWebhookRegistrationRecord, error) {
	if s.registerBridgeWebhookFn != nil {
		return s.registerBridgeWebhookFn(ctx, id)
	}
	return BridgeWebhookRegistrationRecord{}, nil
}
