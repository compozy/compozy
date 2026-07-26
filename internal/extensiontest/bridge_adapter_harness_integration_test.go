//go:build integration

package extensiontest

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/subprocess"
)

var (
	buildHarnessTelegramReferenceOnce sync.Once
	buildHarnessTelegramReferenceErr  error
)

func TestHarnessIntegrationTelegramReferenceConformance(t *testing.T) {
	repoRoot := harnessRepoRoot(t)
	buildHarnessTelegramReferenceAdapter(t, repoRoot)

	t.Run("Should ready_delivery", func(t *testing.T) {
		startTime := time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC)
		harness := NewHarness(t, HarnessConfig{
			ExtensionDir: filepath.Join(repoRoot, "sdk", "examples", "telegram-reference"),
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
			},
			Driver: NewScriptedPromptDriver(startTime, []ScriptedPromptEvent{
				{Type: acp.EventTypeAgentMessage, Text: "hello"},
				{Type: acp.EventTypeAgentMessage, Text: " world"},
				{Type: acp.EventTypeDone},
			}),
			StartTime: startTime,
		})

		harness.WaitForHandshake(t, 10*time.Second)
		harness.WaitForStates(t, 10*time.Second, func(states []StateRecord) bool {
			return len(states) > 0 && states[len(states)-1].Status.Normalize() == bridgepkg.BridgeStatusReady
		})
		harness.AppendInboundUpdate(t, map[string]any{
			"update_id": 9001,
			"message": map[string]any{
				"message_id":        321,
				"message_thread_id": 654,
				"date":              startTime.Unix(),
				"chat": map[string]any{
					"id": 777,
				},
				"from": map[string]any{
					"id": 888,
				},
				"text": "Need a summary",
			},
		})

		harness.WaitForIngests(t, 10*time.Second, func(records []IngestRecord) bool {
			return len(records) > 0 && records[len(records)-1].Result.SessionID != ""
		})
		harness.WaitForDeliveries(t, 10*time.Second, func(records []DeliveryRecord) bool {
			return len(records) > 0 &&
				normalizeEventType(records[len(records)-1].Request.Event.EventType) == bridgepkg.DeliveryEventTypeFinal
		})

		report := harness.Report(t)
		if err := ValidateConformance(report, ConformanceExpectation{
			Provider:                  "telegram-reference",
			Platform:                  "telegram",
			RequireOwnedInstanceList:  true,
			RequireOwnedInstanceFetch: true,
			RequireStateReport:        true,
			RequireDelivery:           true,
			ManagedInstances: []ManagedInstanceExpectation{{
				InstanceID:          harness.Instances[0].ID,
				ExtensionName:       "telegram-reference",
				BoundSecretNames:    []string{"bot_token"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			}},
		}); err != nil {
			t.Fatalf("ValidateConformance() error = %v", err)
		}

		rows := harness.QueryBridgeHealth(t)
		if len(rows) == 0 {
			t.Fatal("QueryBridgeHealth() = empty, want bridge health rows")
		}
	})

	t.Run("Should resume_delivery", func(t *testing.T) {
		startTime := time.Date(2026, 4, 11, 8, 5, 0, 0, time.UTC)
		harness := NewHarness(t, HarnessConfig{
			ExtensionDir: filepath.Join(repoRoot, "sdk", "examples", "telegram-reference"),
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
			},
			Driver: NewScriptedPromptDriver(startTime, []ScriptedPromptEvent{
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
		harness.WaitForStates(t, 10*time.Second, func(states []StateRecord) bool {
			return len(states) > 0 && states[len(states)-1].Status.Normalize() == bridgepkg.BridgeStatusReady
		})
		harness.AppendInboundUpdate(t, map[string]any{
			"update_id": 9002,
			"message": map[string]any{
				"message_id": 222,
				"date":       startTime.Unix(),
				"chat": map[string]any{
					"id": 999,
				},
				"from": map[string]any{
					"id": 111,
				},
				"text": "retry me",
			},
		})

		harness.WaitForDeliveries(t, 10*time.Second, func(records []DeliveryRecord) bool {
			for _, record := range records {
				if normalizeEventType(record.Request.Event.EventType) == bridgepkg.DeliveryEventTypeResume {
					return true
				}
			}
			return false
		})
		report := harness.Report(t)
		if err := ValidateConformance(report, ConformanceExpectation{
			Provider:                  "telegram-reference",
			Platform:                  "telegram",
			RequireOwnedInstanceList:  true,
			RequireOwnedInstanceFetch: true,
			RequireStateReport:        true,
			RequireDelivery:           true,
			RequireResume:             true,
			ManagedInstances: []ManagedInstanceExpectation{{
				InstanceID:          harness.Instances[0].ID,
				ExtensionName:       "telegram-reference",
				BoundSecretNames:    []string{"bot_token"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			}},
		}); err != nil {
			t.Fatalf("ValidateConformance() error = %v", err)
		}
		waitForCondition(t, 10*time.Second, "observer ready bridge status count", func() bool {
			return harness.ObserveHealth(t).Bridges.StatusCounts.Ready == 1
		})
		if got := harness.ObserveHealth(t).Bridges.StatusCounts.Ready; got != 1 {
			t.Fatalf("ObserveHealth().Bridges.StatusCounts.Ready = %d, want 1", got)
		}
	})

	t.Run("Should auth_required_health", func(t *testing.T) {
		startTime := time.Date(2026, 4, 11, 8, 10, 0, 0, time.UTC)
		harness := NewHarness(t, HarnessConfig{
			ExtensionDir: filepath.Join(repoRoot, "sdk", "examples", "telegram-reference"),
			StartTime:    startTime,
		})

		harness.WaitForHandshake(t, 10*time.Second)
		harness.WaitForStates(t, 10*time.Second, func(states []StateRecord) bool {
			return len(states) > 0 && states[len(states)-1].Status.Normalize() == bridgepkg.BridgeStatusAuthRequired
		})

		report := harness.Report(t)
		if err := ValidateConformance(report, ConformanceExpectation{
			Provider:                  "telegram-reference",
			Platform:                  "telegram",
			RequireOwnedInstanceList:  true,
			RequireOwnedInstanceFetch: true,
			RequireStateReport:        true,
			ManagedInstances: []ManagedInstanceExpectation{{
				InstanceID:          harness.Instances[0].ID,
				ExtensionName:       "telegram-reference",
				ExpectedFinalStatus: bridgepkg.BridgeStatusAuthRequired,
			}},
		}); err != nil {
			t.Fatalf("ValidateConformance() error = %v", err)
		}
		waitForCondition(t, 10*time.Second, "observer auth-required bridge status count", func() bool {
			return harness.ObserveHealth(t).Bridges.StatusCounts.AuthRequired == 1
		})
		if got := harness.ObserveHealth(t).Bridges.StatusCounts.AuthRequired; got != 1 {
			t.Fatalf("ObserveHealth().Bridges.StatusCounts.AuthRequired = %d, want 1", got)
		}
	})

	t.Run("Should multi_instance_provider_scope", func(t *testing.T) {
		startTime := time.Date(2026, 4, 11, 8, 15, 0, 0, time.UTC)
		harness := NewHarness(t, HarnessConfig{
			ExtensionDir: filepath.Join(repoRoot, "sdk", "examples", "telegram-reference"),
			ManagedInstances: []ManagedInstanceConfig{
				{
					ID:          "brg-telegram-reference-a",
					DisplayName: "Telegram Reference A",
					BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
						{BindingName: "bot_token", Kind: "token", Value: "token-a"},
					},
				},
				{
					ID:          "brg-telegram-reference-b",
					DisplayName: "Telegram Reference B",
					BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
						{BindingName: "bot_token", Kind: "token", Value: "token-b"},
					},
				},
			},
			Driver: NewScriptedPromptDriver(startTime, []ScriptedPromptEvent{
				{Type: acp.EventTypeAgentMessage, Text: "hello"},
				{Type: acp.EventTypeDone},
			}),
			StartTime: startTime,
		})

		handshake := harness.WaitForHandshake(t, 10*time.Second)
		harness.WaitForStates(t, 10*time.Second, func(states []StateRecord) bool {
			seen := make(map[string]bridgepkg.BridgeStatus, len(states))
			for _, state := range states {
				seen[state.BridgeInstanceID] = state.Status.Normalize()
			}
			return seen["brg-telegram-reference-a"] == bridgepkg.BridgeStatusReady &&
				seen["brg-telegram-reference-b"] == bridgepkg.BridgeStatusReady
		})

		harness.AppendInboundUpdate(t, map[string]any{
			"bridge_instance_id": "brg-telegram-reference-a",
			"update_id":          9101,
			"message": map[string]any{
				"message_id": 111,
				"date":       startTime.Unix(),
				"chat":       map[string]any{"id": 1001},
				"from":       map[string]any{"id": 2001},
				"text":       "route a",
			},
		})
		harness.AppendInboundUpdate(t, map[string]any{
			"bridge_instance_id": "brg-telegram-reference-b",
			"update_id":          9102,
			"message": map[string]any{
				"message_id": 112,
				"date":       startTime.Unix(),
				"chat":       map[string]any{"id": 1002},
				"from":       map[string]any{"id": 2002},
				"text":       "route b",
			},
		})

		harness.WaitForDeliveries(t, 10*time.Second, func(records []DeliveryRecord) bool {
			seen := make(map[string]bool)
			for _, record := range records {
				seen[record.Request.Event.BridgeInstanceID] = true
			}
			return seen["brg-telegram-reference-a"] && seen["brg-telegram-reference-b"]
		})

		report := harness.Report(t)
		if err := ValidateConformance(report, ConformanceExpectation{
			Provider:                  "telegram-reference",
			Platform:                  "telegram",
			RequireOwnedInstanceList:  true,
			RequireOwnedInstanceFetch: true,
			RequireStateReport:        true,
			RequireDelivery:           true,
			ManagedInstances: []ManagedInstanceExpectation{
				{
					InstanceID:          "brg-telegram-reference-a",
					ExtensionName:       "telegram-reference",
					BoundSecretNames:    []string{"bot_token"},
					ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
				},
				{
					InstanceID:          "brg-telegram-reference-b",
					ExtensionName:       "telegram-reference",
					BoundSecretNames:    []string{"bot_token"},
					ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
				},
			},
		}); err != nil {
			t.Fatalf("ValidateConformance() error = %v", err)
		}

		if _, err := handshake.Request.Runtime.Bridge.SingleManagedInstance(); err == nil {
			t.Fatal("SingleManagedInstance() error = nil, want legacy single-instance expectation failure")
		}
	})
}

func TestScriptedPromptDriverPromptStopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	driver := NewScriptedPromptDriver(
		time.Date(2026, 4, 11, 9, 0, 0, 0, time.UTC),
		[]ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "late event", Delay: 50 * time.Millisecond},
			{Type: acp.EventTypeDone},
		},
	)

	proc, err := driver.Start(context.Background(), acp.StartOpts{AgentName: "coder"})
	if err != nil {
		t.Fatalf("ScriptedPromptDriver.Start() error = %v", err)
	}
	defer func() {
		if err := driver.Stop(context.Background(), proc); err != nil {
			t.Fatalf("ScriptedPromptDriver.Stop() error = %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	events, err := driver.Prompt(ctx, proc, acp.PromptRequest{TurnID: "turn-cancel"})
	if err != nil {
		t.Fatalf("ScriptedPromptDriver.Prompt() error = %v", err)
	}
	cancel()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	select {
	case event, ok := <-events:
		if ok {
			t.Fatalf("Prompt() event after cancellation = %#v, want closed bridge", event)
		}
	case <-timer.C:
		t.Fatal("Prompt() bridge did not close after cancellation")
	}
}

func harnessRepoRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func buildHarnessTelegramReferenceAdapter(t *testing.T, repoRoot string) {
	t.Helper()

	buildHarnessTelegramReferenceOnce.Do(func() {
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
			buildHarnessTelegramReferenceErr = fmt.Errorf("go build telegram-reference: %w\n%s", err, string(output))
		}
	})
	if buildHarnessTelegramReferenceErr != nil {
		t.Fatal(buildHarnessTelegramReferenceErr)
	}
}
