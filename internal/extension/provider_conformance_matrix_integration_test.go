//go:build integration

package extensionpkg_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensiontest "github.com/compozy/agh/internal/extensiontest"
	"github.com/compozy/agh/internal/subprocess"
)

func TestRepresentativeProviderConformanceMatrix(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	summaries := make([]extensiontest.ProviderConformanceSummary, 0, 5)
	for _, tc := range []struct {
		name string
		run  func(*testing.T, string) extensiontest.ProviderConformanceSummary
	}{
		{name: "GitHubMultiInstance", run: runGitHubMultiInstanceMatrixCase},
		{name: "TelegramRestartRecovery", run: runTelegramRestartRecoveryMatrixCase},
		{name: "WhatsAppDMPolicy", run: runWhatsAppDMPolicyMatrixCase},
		{name: "TelegramAuthDegradation", run: runTelegramAuthDegradationMatrixCase},
		{name: "WhatsAppRateLimitRecovery", run: runWhatsAppRateLimitMatrixCase},
	} {
		var summary extensiontest.ProviderConformanceSummary
		ok := t.Run(tc.name, func(t *testing.T) {
			summary = tc.run(t, repoRoot)
		})
		if ok {
			summaries = append(summaries, summary)
		}
	}

	matrix := extensiontest.BuildConformanceMatrix(summaries...)

	if got, want := len(matrix), 3; got != want {
		t.Fatalf("len(matrix) = %d, want %d", got, want)
	}
	if err := extensiontest.ValidateConformanceMatrix(matrix,
		extensiontest.CoverageTargetMultiInstance,
		extensiontest.CoverageTargetRestartRecovery,
		extensiontest.CoverageTargetDMPolicy,
		extensiontest.CoverageTargetAuthDegradation,
		extensiontest.CoverageTargetRateLimitRecovery,
	); err != nil {
		t.Fatalf("ValidateConformanceMatrix() error = %v", err)
	}
}

func TestAutoDiscoveredProviderRuntimeConformance(t *testing.T) {
	t.Run("Should discover and round-trip every in-tree bridge provider", func(t *testing.T) {
		// Provider builds stay sequential because they share the Go build cache and this is an integration lane.
		repoRoot := telegramReferenceRepoRoot(t)
		providers, err := discoverBridgeProviders(filepath.Join(repoRoot, "extensions", "bridges"))
		if err != nil {
			t.Fatalf("discoverBridgeProviders() error = %v", err)
		}
		if got, want := len(providers), 8; got != want {
			t.Fatalf("len(discovered providers) = %d, want %d", got, want)
		}
		for _, provider := range providers {
			provider := provider
			t.Run("Should conform "+provider.Name, func(t *testing.T) {
				contract, err := validateDiscoveredProviderManifest(provider)
				if err != nil {
					t.Fatalf("validateDiscoveredProviderManifest(%q) error = %v", provider.Name, err)
				}
				if err := runDiscoveredProviderRuntime(t, repoRoot, provider, contract); err != nil {
					t.Fatalf("runDiscoveredProviderRuntime(%q) error = %v", provider.Name, err)
				}
			})
		}
	})

	t.Run("Should discover a synthetic provider and reject required slot drift", func(t *testing.T) {
		repoRoot := telegramReferenceRepoRoot(t)
		providersRoot := createSyntheticProviderRoot(t, repoRoot)
		providers, err := discoverBridgeProviders(providersRoot)
		if err != nil {
			t.Fatalf("discoverBridgeProviders(synthetic) error = %v", err)
		}
		if got, want := len(providers), 1; got != want {
			t.Fatalf("len(synthetic providers) = %d, want %d", got, want)
		}
		provider := providers[0]
		contract, err := validateDiscoveredProviderManifest(provider)
		if err != nil {
			t.Fatalf("validateDiscoveredProviderManifest(synthetic) error = %v", err)
		}
		if err := runDiscoveredProviderRuntime(t, repoRoot, provider, contract); err != nil {
			t.Fatalf("runDiscoveredProviderRuntime(synthetic) error = %v", err)
		}

		drifted := cloneProviderManifest(provider.Manifest)
		drifted.Bridge.SecretSlots = nil
		if err := validateProviderSchemaContract(drifted, contract); err == nil ||
			!strings.Contains(err.Error(), "token") {
			t.Fatalf("validateProviderSchemaContract(slot drift) error = %v, want missing token", err)
		}
	})
}

func runGitHubMultiInstanceMatrixCase(t *testing.T, repoRoot string) extensiontest.ProviderConformanceSummary {
	t.Helper()

	buildGitHubProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newGitHubProviderAPIServer(t)
	privateKey := githubProviderTestPrivateKey(t)
	startTime := time.Date(2026, 4, 16, 0, 5, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: githubProviderExtensionDir(repoRoot),
		Platform:     "github",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{
			githubPATManagedInstance(listenAddr),
			githubAppManagedInstance(listenAddr, privateKey),
		},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeAgentMessage, Text: " world"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			githubProviderListenAddrEnv: listenAddr,
			githubProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: startTime,
	})

	waitForGitHubReadyStates(t, harness, []string{"brg-github-pat", "brg-github-app"})

	webhookURL := fmt.Sprintf("http://%s/github", listenAddr)
	postGitHubProviderWebhook(
		t,
		webhookURL,
		githubProviderWebhookSecret,
		"issue_comment",
		githubIssueCommentWebhookPayload(startTime),
	)
	postGitHubProviderWebhook(
		t,
		webhookURL,
		githubProviderWebhookSecret,
		"pull_request_review_comment",
		githubReviewCommentWebhookPayload(startTime),
	)

	ingests := harness.WaitForIngests(t, 10*time.Second, func(records []extensiontest.IngestRecord) bool {
		if len(records) < 2 {
			return false
		}
		seen := map[string]bool{}
		for _, record := range records {
			if record.Result.SessionID == "" {
				continue
			}
			seen[record.Envelope.BridgeInstanceID] = true
		}
		return seen["brg-github-pat"] && seen["brg-github-app"]
	})
	deliveries := harness.WaitForDeliveries(t, 10*time.Second, func(records []extensiontest.DeliveryRecord) bool {
		finals := 0
		for _, record := range records {
			if normalizeDeliveryEventType(record.Request.Event.EventType) == bridgepkg.DeliveryEventTypeFinal {
				finals++
			}
		}
		return finals >= 2
	})
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "github",
		Platform:                  "github",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{
			{
				InstanceID:          "brg-github-pat",
				ExtensionName:       "github",
				BoundSecretNames:    []string{"webhook_secret", "token"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			},
			{
				InstanceID:          "brg-github-app",
				ExtensionName:       "github",
				BoundSecretNames:    []string{"webhook_secret", "app_id", "private_key"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			},
		},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	patIngest := githubFindIngestByInstance(t, ingests, "brg-github-pat")
	if got, want := patIngest.Envelope.GroupID, "acme/app-one"; got != want {
		t.Fatalf("PAT ingest group id = %q, want %q", got, want)
	}
	if got, want := patIngest.Envelope.ThreadID, "github:acme/app-one:issue:42"; got != want {
		t.Fatalf("PAT ingest thread id = %q, want %q", got, want)
	}

	appIngest := githubFindIngestByInstance(t, ingests, "brg-github-app")
	if got, want := appIngest.Envelope.GroupID, "acme/app-two"; got != want {
		t.Fatalf("App ingest group id = %q, want %q", got, want)
	}
	if got, want := appIngest.Envelope.ThreadID, "github:acme/app-two:7:rc:300"; got != want {
		t.Fatalf("App ingest thread id = %q, want %q", got, want)
	}
	if len(deliveries) < 4 {
		t.Fatalf("len(deliveries) = %d, want at least 4", len(deliveries))
	}

	calls := mockAPI.Calls()
	if !githubProviderCallsContain(calls, httpMethodPost, "/repos/acme/app-one/issues/42/comments") {
		t.Fatalf("mock api calls = %#v, want PAT issue comment POST", calls)
	}
	if !githubProviderCallsContain(calls, httpMethodPost, "/repos/acme/app-two/pulls/7/comments/300/replies") {
		t.Fatalf("mock api calls = %#v, want App review reply POST", calls)
	}

	return extensiontest.SummarizeConformanceReport(
		"github",
		"github",
		report,
		extensiontest.CoverageTargetMultiInstance,
	)
}

func runTelegramRestartRecoveryMatrixCase(t *testing.T, repoRoot string) extensiontest.ProviderConformanceSummary {
	t.Helper()

	buildTelegramProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newTelegramProviderAPIServer(t)
	startTime := time.Date(2026, 4, 16, 0, 10, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: telegramProviderExtensionDir(repoRoot),
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:            "brg-telegram-restart",
			DisplayName:   "Telegram Restart",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludeThread: true, IncludeGroup: true},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
				{BindingName: "webhook_secret", Kind: "token", Value: "top-secret"},
			},
		}},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeDone},
		}),
		StartTime:                startTime,
		CrashOnceOnFirstDelivery: true,
		BrokerOptions: []bridgepkg.DeliveryBrokerOption{
			bridgepkg.WithDeliveryBrokerRetryDelay(20 * time.Millisecond),
		},
		ExtraEnv: map[string]string{
			telegramProviderListenAddrEnv: listenAddr,
			telegramProviderAPIBaseEnv:    mockAPI.URL(),
		},
	})

	harness.WaitForHandshake(t, 10*time.Second)
	waitForBridgeAdapterState(t, harness, bridgepkg.BridgeStatusReady)
	postTelegramProviderWebhook(
		t,
		fmt.Sprintf("http://%s/telegram/%s", listenAddr, harness.Instances[0].ID),
		"top-secret",
		telegramProviderInboundUpdate(startTime),
	)

	deliveries := harness.WaitForDeliveries(t, 10*time.Second, func(records []extensiontest.DeliveryRecord) bool {
		for _, record := range records {
			if normalizeDeliveryEventType(record.Request.Event.EventType) == bridgepkg.DeliveryEventTypeResume {
				return true
			}
		}
		return false
	})
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "telegram",
		Platform:                  "telegram",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		RequireResume:             true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "telegram",
			BoundSecretNames:    []string{"bot_token", "webhook_secret"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	resume := findDeliveryRecord(t, deliveries, bridgepkg.DeliveryEventTypeResume)
	if resume.Request.Snapshot == nil {
		t.Fatal("resume delivery snapshot = nil, want resumable state")
	}
	if resume.PID == deliveries[0].PID {
		t.Fatalf("resume pid = %d, want a restarted provider process different from %d", resume.PID, deliveries[0].PID)
	}
	if !mockAPI.ContainsMethod("sendMessage") {
		t.Fatal("mock telegram api did not record sendMessage after restart")
	}

	return extensiontest.SummarizeConformanceReport(
		"telegram",
		"telegram",
		report,
		extensiontest.CoverageTargetRestartRecovery,
	)
}

func runWhatsAppDMPolicyMatrixCase(t *testing.T, repoRoot string) extensiontest.ProviderConformanceSummary {
	t.Helper()

	buildWhatsAppProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newWhatsAppProviderAPIServer(t, whatsappProviderAPIServerConfig{})
	startTime := time.Date(2026, 4, 16, 0, 15, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: whatsappProviderExtensionDir(repoRoot),
		Platform:     "whatsapp",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:          "brg-whatsapp-dm",
			DisplayName: "WhatsApp DM Policy",
			DMPolicy:    bridgepkg.BridgeDMPolicyAllowlist,
			RoutingPolicy: bridgepkg.RoutingPolicy{
				IncludePeer: true,
			},
			ProviderConfig: map[string]any{
				"phone_number_id": "123456789",
				"dm": map[string]any{
					"allow_user_ids": []string{"15551234567"},
				},
			},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "access_token", Kind: "token", Value: "access-token"},
				{BindingName: "app_secret", Kind: "token", Value: "app-secret"},
				{BindingName: "verify_token", Kind: "token", Value: "verify-token"},
			},
		}},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			whatsappProviderListenAddrEnv: listenAddr,
			whatsappProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: startTime,
	})

	harness.WaitForHandshake(t, 10*time.Second)
	waitForBridgeAdapterState(t, harness, bridgepkg.BridgeStatusReady)

	webhookURL := fmt.Sprintf("http://%s/whatsapp/%s", listenAddr, harness.Instances[0].ID)
	postWhatsAppProviderWebhook(t, webhookURL, "app-secret", whatsappProviderMixedDMWebhook("123456789"))

	ingests := harness.WaitForIngests(t, 10*time.Second, func(records []extensiontest.IngestRecord) bool {
		return len(records) == 1 && records[0].Result.SessionID != ""
	})
	harness.WaitForDeliveries(t, 10*time.Second, func(records []extensiontest.DeliveryRecord) bool {
		return len(records) > 0 &&
			normalizeDeliveryEventType(
				records[len(records)-1].Request.Event.EventType,
			) == bridgepkg.DeliveryEventTypeFinal
	})
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "whatsapp",
		Platform:                  "whatsapp",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "whatsapp",
			BoundSecretNames:    []string{"access_token", "app_secret", "verify_token"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	if got, want := len(ingests), 1; got != want {
		t.Fatalf("len(ingests) = %d, want %d", got, want)
	}
	if got, want := ingests[0].Envelope.Sender.ID, "15551234567"; got != want {
		t.Fatalf("allowed ingest sender id = %q, want %q", got, want)
	}
	if got, want := ingests[0].Envelope.Content.Text, "hello"; got != want {
		t.Fatalf("allowed ingest text = %q, want %q", got, want)
	}

	for _, call := range mockAPI.Calls() {
		if to, ok := call.Body["to"].(string); ok && to == "16667778888" {
			t.Fatalf("blocked direct message leaked into outbound delivery: %#v", call.Body)
		}
	}

	return extensiontest.SummarizeConformanceReport(
		"whatsapp",
		"whatsapp",
		report,
		extensiontest.CoverageTargetDMPolicy,
	)
}

func runTelegramAuthDegradationMatrixCase(t *testing.T, repoRoot string) extensiontest.ProviderConformanceSummary {
	t.Helper()

	buildTelegramProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newTelegramProviderAPIServer(t)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: telegramProviderExtensionDir(repoRoot),
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:            "brg-telegram-auth",
			DisplayName:   "Telegram Auth",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludeThread: true, IncludeGroup: true},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "webhook_secret", Kind: "token", Value: "top-secret"},
			},
		}},
		ExtraEnv: map[string]string{
			telegramProviderListenAddrEnv: listenAddr,
			telegramProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: time.Date(2026, 4, 16, 0, 20, 0, 0, time.UTC),
	})

	harness.WaitForHandshake(t, 10*time.Second)
	states := harness.WaitForStates(t, 10*time.Second, func(records []extensiontest.StateRecord) bool {
		return len(records) > 0
	})
	last := states[len(states)-1]
	if got, want := last.Status.Normalize(), bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("last adapter state = %q (error=%q), want %q", got, last.Error, want)
	}
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "telegram",
		Platform:                  "telegram",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "telegram",
			BoundSecretNames:    []string{"webhook_secret"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusAuthRequired,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	var instance *bridgepkg.BridgeInstance
	waitForCondition(t, 10*time.Second, "bridge instance auth required after missing token", func() bool {
		loaded, err := harness.Bridges.GetInstance(context.Background(), harness.Instances[0].ID)
		if err != nil {
			return false
		}
		instance = loaded
		return loaded.Status.Normalize() == bridgepkg.BridgeStatusAuthRequired &&
			loaded.Degradation != nil &&
			loaded.Degradation.Reason == bridgepkg.BridgeDegradationReasonAuthFailed
	})
	if instance == nil {
		t.Fatal("auth-required bridge instance = nil, want persisted auth failure state")
	}
	if err := extensiontest.ValidateClassifiedOutcome(
		extensiontest.ClassifiedOutcome{
			Provider:       "telegram",
			Classification: extensiontest.OutcomeClassAuthFailure,
			Status:         instance.Status,
			Reason:         instance.Degradation.Reason,
			Retryable:      false,
		},
		extensiontest.ClassifiedOutcomeExpectation{
			Classification: extensiontest.OutcomeClassAuthFailure,
			Status:         bridgepkg.BridgeStatusAuthRequired,
			Reason:         bridgepkg.BridgeDegradationReasonAuthFailed,
			Retryable:      false,
		},
	); err != nil {
		t.Fatalf("ValidateClassifiedOutcome(auth) error = %v", err)
	}

	return extensiontest.SummarizeConformanceReport(
		"telegram",
		"telegram",
		report,
		extensiontest.CoverageTargetAuthDegradation,
	)
}

func runWhatsAppRateLimitMatrixCase(t *testing.T, repoRoot string) extensiontest.ProviderConformanceSummary {
	t.Helper()

	buildWhatsAppProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newWhatsAppProviderAPIServer(t, whatsappProviderAPIServerConfig{FailFirstSendWith429: true})
	startTime := time.Date(2026, 4, 16, 0, 25, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: whatsappProviderExtensionDir(repoRoot),
		Platform:     "whatsapp",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:          "brg-whatsapp-rate-limit",
			DisplayName: "WhatsApp Rate Limit",
			RoutingPolicy: bridgepkg.RoutingPolicy{
				IncludePeer: true,
			},
			ProviderConfig: map[string]any{
				"phone_number_id": "123456789",
			},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "access_token", Kind: "token", Value: "access-token"},
				{BindingName: "app_secret", Kind: "token", Value: "app-secret"},
				{BindingName: "verify_token", Kind: "token", Value: "verify-token"},
			},
		}},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			whatsappProviderListenAddrEnv: listenAddr,
			whatsappProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: startTime,
	})

	harness.WaitForHandshake(t, 10*time.Second)
	waitForBridgeAdapterState(t, harness, bridgepkg.BridgeStatusReady)
	webhookURL := fmt.Sprintf("http://%s/whatsapp/%s", listenAddr, harness.Instances[0].ID)
	postWhatsAppProviderWebhook(
		t,
		webhookURL,
		"app-secret",
		whatsappProviderInboundWebhook("123456789", "Trigger rate limit"),
	)

	states := harness.WaitForStates(t, 10*time.Second, func(records []extensiontest.StateRecord) bool {
		return stateRecordsContainDegradation(
			records,
			bridgepkg.BridgeStatusDegraded,
			bridgepkg.BridgeDegradationReasonRateLimited,
		)
	})
	degraded, ok := findStateRecordWithDegradation(
		states,
		bridgepkg.BridgeStatusDegraded,
		bridgepkg.BridgeDegradationReasonRateLimited,
	)
	if !ok {
		t.Fatalf("state markers = %#v, want degraded rate-limited state report", states)
	}

	report := harness.Report(t)
	reportForValidation := report
	reportForValidation.Deliveries = nil
	if err := extensiontest.ValidateConformance(reportForValidation, extensiontest.ConformanceExpectation{
		Provider:                  "whatsapp",
		Platform:                  "whatsapp",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:       harness.Instances[0].ID,
			ExtensionName:    "whatsapp",
			BoundSecretNames: []string{"access_token", "app_secret", "verify_token"},
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	if err := extensiontest.ValidateClassifiedOutcome(
		extensiontest.ClassifiedOutcome{
			Provider:       "whatsapp",
			Classification: extensiontest.OutcomeClassRateLimit,
			Status:         degraded.Status,
			Reason:         degraded.Instance.Degradation.Reason,
			Retryable:      true,
		},
		extensiontest.ClassifiedOutcomeExpectation{
			Classification: extensiontest.OutcomeClassRateLimit,
			Status:         bridgepkg.BridgeStatusDegraded,
			Reason:         bridgepkg.BridgeDegradationReasonRateLimited,
			Retryable:      true,
		},
	); err != nil {
		t.Fatalf("ValidateClassifiedOutcome(rate limit) error = %v", err)
	}

	return extensiontest.SummarizeConformanceReport(
		"whatsapp",
		"whatsapp",
		report,
		extensiontest.CoverageTargetRateLimitRecovery,
	)
}

func whatsappProviderMixedDMWebhook(phoneNumberID string) map[string]any {
	return map[string]any{
		"object": "whatsapp_business_account",
		"entry": []map[string]any{{
			"id": "waba-dm",
			"changes": []map[string]any{{
				"field": "messages",
				"value": map[string]any{
					"messaging_product": "whatsapp",
					"metadata": map[string]any{
						"display_phone_number": "+15551234567",
						"phone_number_id":      phoneNumberID,
					},
					"contacts": []map[string]any{
						{
							"profile": map[string]any{"name": "Alice Example"},
							"wa_id":   "15551234567",
						},
						{
							"profile": map[string]any{"name": "Blocked User"},
							"wa_id":   "16667778888",
						},
					},
					"messages": []map[string]any{
						{
							"from":      "15551234567",
							"id":        "wamid.allowed",
							"timestamp": "1775866800",
							"type":      "text",
							"text":      map[string]any{"body": "hello"},
						},
						{
							"from":      "16667778888",
							"id":        "wamid.blocked",
							"timestamp": "1775866801",
							"type":      "text",
							"text":      map[string]any{"body": "blocked"},
						},
					},
				},
			}},
		}},
	}
}

func waitForBridgeAdapterState(
	t *testing.T,
	harness *extensiontest.Harness,
	want bridgepkg.BridgeStatus,
) []extensiontest.StateRecord {
	t.Helper()

	states := harness.WaitForStates(t, 10*time.Second, func(records []extensiontest.StateRecord) bool {
		if len(records) == 0 {
			return false
		}
		return records[len(records)-1].Status.Normalize() == want
	})
	if got := states[len(states)-1].Status.Normalize(); got != want {
		t.Fatalf("last adapter state = %q (error=%q), want %q", got, states[len(states)-1].Error, want)
	}
	return states
}

const httpMethodPost = "POST"

func stateRecordsContainDegradation(
	records []extensiontest.StateRecord,
	status bridgepkg.BridgeStatus,
	reason bridgepkg.BridgeDegradationReason,
) bool {
	_, ok := findStateRecordWithDegradation(records, status, reason)
	return ok
}

func findStateRecordWithDegradation(
	records []extensiontest.StateRecord,
	status bridgepkg.BridgeStatus,
	reason bridgepkg.BridgeDegradationReason,
) (extensiontest.StateRecord, bool) {
	for _, record := range records {
		if record.Status.Normalize() != status.Normalize() {
			continue
		}
		if record.Instance.Degradation == nil {
			continue
		}
		if record.Instance.Degradation.Reason.Normalize() == reason.Normalize() {
			return record, true
		}
	}
	return extensiontest.StateRecord{}, false
}
