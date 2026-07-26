//go:build integration

package extensionpkg_test

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensiontest "github.com/compozy/agh/internal/extensiontest"
	observepkg "github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/subprocess"
)

var (
	buildTelegramReferenceOnce sync.Once
	buildTelegramReferenceErr  error
)

func TestTelegramReferenceAdapterLaunchNegotiatesBridgeRuntime(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildTelegramReferenceAdapter(t, repoRoot)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: telegramReferenceExtensionDir(repoRoot),
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
		},
		StartTime: time.Date(2026, 4, 11, 6, 0, 0, 0, time.UTC),
	})

	handshake := harness.WaitForHandshake(t, 10*time.Second)
	states := harness.WaitForStates(t, 10*time.Second, func(states []extensiontest.StateRecord) bool {
		return len(states) > 0
	})
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("last adapter state = %q (error=%q), want %q", got, states[len(states)-1].Error, want)
	}
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "telegram-reference",
		Platform:                  "telegram",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "telegram-reference",
			BoundSecretNames:    []string{"bot_token"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	if handshake.Request.Runtime.Bridge == nil {
		t.Fatal("initialize runtime.bridge = nil, want bound bridge launch metadata")
	}
	managed, ok := handshake.Request.Runtime.Bridge.ManagedInstance(harness.Instances[0].ID)
	if !ok || managed == nil {
		t.Fatalf("handshake.Request.Runtime.Bridge.ManagedInstance(%q) missing", harness.Instances[0].ID)
	}
	if got, want := managed.Instance.ID, harness.Instances[0].ID; got != want {
		t.Fatalf("initialize runtime bridge instance = %q, want %q", got, want)
	}
	if got, want := managed.Instance.ExtensionName, "telegram-reference"; got != want {
		t.Fatalf("initialize runtime bridge extension = %q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(managed.BoundSecrets[0].Value), "telegram-bot-token"; got != want {
		t.Fatalf("initialize bound bot token = %q, want %q", got, want)
	}
	if report.Ownership == nil {
		t.Fatal("ownership marker = nil, want provider ownership metadata")
	}
	if got, want := len(report.Ownership.Fetched), 1; got != want {
		t.Fatalf("len(report.Ownership.Fetched) = %d, want %d", got, want)
	}
	if got, want := report.Ownership.Fetched[0].ID, harness.Instances[0].ID; got != want {
		t.Fatalf("ownership fetched id = %q, want %q", got, want)
	}

	row := waitForBridgeHealth(t, 10*time.Second, harness, func(health observepkg.BridgeInstanceHealth) bool {
		return health.Status.Normalize() == bridgepkg.BridgeStatusReady
	})
	if got, want := row.RouteCount, 0; got != want {
		t.Fatalf("bridge health route_count = %d, want %d before ingress", got, want)
	}

	health := harness.ObserveHealth(t)
	if got, want := health.Bridges.StatusCounts.Ready, 1; got != want {
		t.Fatalf("observe.Health().Bridges.StatusCounts.Ready = %d, want %d", got, want)
	}
}

func TestTelegramReferenceAdapterIngressAndDeliveryConformance(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildTelegramReferenceAdapter(t, repoRoot)

	startTime := time.Date(2026, 4, 11, 6, 5, 0, 0, time.UTC)
	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: telegramReferenceExtensionDir(repoRoot),
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
		},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeAgentMessage, Text: " world"},
			{Type: acp.EventTypeDone},
		}),
		StartTime: startTime,
	})

	harness.WaitForHandshake(t, 10*time.Second)
	states := harness.WaitForStates(t, 10*time.Second, func(states []extensiontest.StateRecord) bool {
		return len(states) > 0
	})
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("last adapter state = %q (error=%q), want %q", got, states[len(states)-1].Error, want)
	}
	harness.AppendInboundUpdate(t, telegramInboundUpdate(startTime))

	ingests := harness.WaitForIngests(t, 10*time.Second, func(records []extensiontest.IngestRecord) bool {
		return len(records) > 0 && strings.TrimSpace(records[len(records)-1].Result.SessionID) != ""
	})
	deliveries := harness.WaitForDeliveries(t, 10*time.Second, func(records []extensiontest.DeliveryRecord) bool {
		return len(records) > 0 &&
			normalizeDeliveryEventType(
				records[len(records)-1].Request.Event.EventType,
			) == bridgepkg.DeliveryEventTypeFinal
	})
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "telegram-reference",
		Platform:                  "telegram",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "telegram-reference",
			BoundSecretNames:    []string{"bot_token"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	if got, want := len(ingests), 1; got != want {
		t.Fatalf("len(ingests) = %d, want %d", got, want)
	}
	if got, want := ingests[0].Envelope.Content.Text, "Need a summary"; got != want {
		t.Fatalf("ingest envelope text = %q, want %q", got, want)
	}
	if len(deliveries) < 2 {
		t.Fatalf("len(deliveries) = %d, want at least 2", len(deliveries))
	}
	if got, want := normalizeDeliveryEventType(
		deliveries[0].Request.Event.EventType,
	), bridgepkg.DeliveryEventTypeStart; got != want {
		t.Fatalf("first delivery event type = %q, want %q", got, want)
	}
	if got, want := normalizeDeliveryEventType(
		deliveries[len(deliveries)-1].Request.Event.EventType,
	), bridgepkg.DeliveryEventTypeFinal; got != want {
		t.Fatalf("last delivery event type = %q, want %q", got, want)
	}

	row := waitForBridgeHealth(t, 10*time.Second, harness, func(health observepkg.BridgeInstanceHealth) bool {
		return health.Status.Normalize() == bridgepkg.BridgeStatusReady && health.RouteCount == 1
	})
	if got, want := row.RouteCount, 1; got != want {
		t.Fatalf("bridge health route_count = %d, want %d", got, want)
	}
}

func TestTelegramReferenceAdapterRestartResumesActiveDelivery(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildTelegramReferenceAdapter(t, repoRoot)

	startTime := time.Date(2026, 4, 11, 6, 10, 0, 0, time.UTC)
	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: telegramReferenceExtensionDir(repoRoot),
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
		},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeDone},
		}),
		StartTime:                startTime,
		CrashOnceOnFirstDelivery: true,
		BrokerOptions: []bridgepkg.DeliveryBrokerOption{
			bridgepkg.WithDeliveryBrokerRetryDelay(20 * time.Millisecond),
		},
	})

	harness.WaitForHandshake(t, 10*time.Second)
	states := harness.WaitForStates(t, 10*time.Second, func(states []extensiontest.StateRecord) bool {
		return len(states) > 0
	})
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("last adapter state = %q (error=%q), want %q", got, states[len(states)-1].Error, want)
	}
	harness.AppendInboundUpdate(t, telegramInboundUpdate(startTime))

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
		Provider:                  "telegram-reference",
		Platform:                  "telegram",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		RequireResume:             true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "telegram-reference",
			BoundSecretNames:    []string{"bot_token"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	resume := findDeliveryRecord(t, deliveries, bridgepkg.DeliveryEventTypeResume)
	if resume.Request.Snapshot == nil {
		t.Fatal("resume delivery snapshot = nil, want resumable state")
	}
	// The crashed process can exit before its first delivery marker is flushed,
	// so the stable restart proof is the resumed delivery plus its snapshot.
	if resume.PID <= 0 {
		t.Fatalf("resume pid = %d, want resumed delivery to record a live adapter process", resume.PID)
	}
}

func TestTelegramReferenceAdapterAuthRequiredHealthSurface(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildTelegramReferenceAdapter(t, repoRoot)

	startTime := time.Date(2026, 4, 11, 6, 15, 0, 0, time.UTC)
	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: telegramReferenceExtensionDir(repoRoot),
		StartTime:    startTime,
	})

	harness.WaitForHandshake(t, 10*time.Second)
	states := harness.WaitForStates(t, 10*time.Second, func(states []extensiontest.StateRecord) bool {
		return len(states) > 0
	})
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("last adapter state = %q (error=%q), want %q", got, states[len(states)-1].Error, want)
	}
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "telegram-reference",
		Platform:                  "telegram",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "telegram-reference",
			ExpectedFinalStatus: bridgepkg.BridgeStatusAuthRequired,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	row := waitForBridgeHealth(t, 10*time.Second, harness, func(health observepkg.BridgeInstanceHealth) bool {
		return health.Status.Normalize() == bridgepkg.BridgeStatusAuthRequired && health.AuthFailuresTotal > 0
	})
	if row.AuthFailuresTotal <= 0 {
		t.Fatalf("bridge auth_failures_total = %d, want > 0", row.AuthFailuresTotal)
	}

	health := harness.ObserveHealth(t)
	if got, want := health.Bridges.StatusCounts.AuthRequired, 1; got != want {
		t.Fatalf("observe.Health().Bridges.StatusCounts.AuthRequired = %d, want %d", got, want)
	}
	if health.Bridges.AuthFailuresTotal <= 0 {
		t.Fatalf("observe.Health().Bridges.AuthFailuresTotal = %d, want > 0", health.Bridges.AuthFailuresTotal)
	}
}

func telegramReferenceRepoRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func telegramReferenceExtensionDir(repoRoot string) string {
	return filepath.Join(repoRoot, "sdk", "examples", "telegram-reference")
}

func buildTelegramReferenceAdapter(t *testing.T, repoRoot string) {
	t.Helper()

	buildTelegramReferenceOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(
			ctx,
			"go",
			"build",
			"-o",
			"./sdk/examples/telegram-reference/bin/telegram-reference",
			"./sdk/examples/telegram-reference",
		)
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildTelegramReferenceErr = fmt.Errorf("go build telegram-reference: %w\n%s", err, string(output))
		}
	})
	if buildTelegramReferenceErr != nil {
		t.Fatal(buildTelegramReferenceErr)
	}
}

func telegramInboundUpdate(now time.Time) map[string]any {
	return map[string]any{
		"update_id": 9001,
		"message": map[string]any{
			"message_id":        321,
			"message_thread_id": 654,
			"date":              now.Unix(),
			"chat": map[string]any{
				"id":    777,
				"type":  "supergroup",
				"title": "ops",
			},
			"from": map[string]any{
				"id":         888,
				"username":   "alice",
				"first_name": "Alice",
				"last_name":  "Example",
			},
			"text": "Need a summary",
		},
	}
}

func waitForBridgeHealth(
	t *testing.T,
	timeout time.Duration,
	harness *extensiontest.Harness,
	predicate func(observepkg.BridgeInstanceHealth) bool,
) observepkg.BridgeInstanceHealth {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rows := harness.QueryBridgeHealth(t)
		for _, row := range rows {
			if row.BridgeInstanceID == harness.Instances[0].ID && predicate(row) {
				return row
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("bridge health for %q did not satisfy predicate before timeout", harness.Instances[0].ID)
	return observepkg.BridgeInstanceHealth{}
}

func findDeliveryRecord(
	t *testing.T,
	records []extensiontest.DeliveryRecord,
	eventType bridgepkg.DeliveryEventType,
) extensiontest.DeliveryRecord {
	t.Helper()

	want := normalizeDeliveryEventType(eventType)
	for _, record := range records {
		if normalizeDeliveryEventType(record.Request.Event.EventType) == want {
			return record
		}
	}
	t.Fatalf("delivery records did not contain event type %q", eventType)
	return extensiontest.DeliveryRecord{}
}

func normalizeDeliveryEventType(value bridgepkg.DeliveryEventType) bridgepkg.DeliveryEventType {
	return bridgepkg.DeliveryEventType(strings.ToLower(strings.TrimSpace(string(value))))
}
