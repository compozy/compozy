//go:build integration

package extensionpkg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/session"
	skillspkg "github.com/compozy/agh/internal/skills"
	storepkg "github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
	transcriptpkg "github.com/compozy/agh/internal/transcript"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type scriptedPromptEvent struct {
	Type            string
	Text            string
	Error           string
	ToolName        string
	ToolCallID      string
	ToolInput       json.RawMessage
	ToolError       bool
	ToolErrorDetail string
	Delay           time.Duration
}

type scriptedPromptDriver struct {
	now       time.Time
	script    []scriptedPromptEvent
	processes map[*session.AgentProcess]*scriptedPromptProcess
	prompts   []acp.PromptRequest
	mu        sync.Mutex
	startSeq  atomic.Int64
}

type scriptedPromptProcess struct {
	done sync.Once
	ch   chan struct{}
}

type deliveryIntegrationEnv struct {
	now           time.Time
	homePaths     aghconfig.HomePaths
	workspace     workspacepkg.ResolvedWorkspace
	workspaces    *hostAPIFakeWorkspaceResolver
	globalDB      *globaldb.GlobalDB
	bridges       *bridgepkg.Service
	sessions      *session.Manager
	manager       *Manager
	broker        *bridgepkg.Broker
	handler       *HostAPIHandler
	checker       *CapabilityChecker
	extensionName string
}

type signalingDeliveryLedgerStore struct {
	bridgepkg.DeliveryLedgerStore
	checkpointed chan bridgepkg.DeliveryLedgerCheckpoint
}

func (s *signalingDeliveryLedgerStore) CheckpointBridgeDelivery(
	ctx context.Context,
	checkpoint bridgepkg.DeliveryLedgerCheckpoint,
) error {
	if err := s.DeliveryLedgerStore.CheckpointBridgeDelivery(ctx, checkpoint); err != nil {
		return err
	}
	select {
	case s.checkpointed <- checkpoint:
	default:
	}
	return nil
}

type durableRestartDeliveryTransport struct {
	mu              sync.Mutex
	remoteMessageID string
	calls           []bridgepkg.DeliveryRequest
}

func (t *durableRestartDeliveryTransport) DeliverBridge(
	_ context.Context,
	_ string,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	t.mu.Lock()
	t.calls = append(t.calls, request)
	remoteMessageID := t.remoteMessageID
	t.mu.Unlock()
	return bridgepkg.DeliveryAck{
		DeliveryID:      request.Event.DeliveryID,
		Seq:             request.Event.Seq,
		RemoteMessageID: remoteMessageID,
	}, nil
}

func (t *durableRestartDeliveryTransport) snapshotCalls() []bridgepkg.DeliveryRequest {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]bridgepkg.DeliveryRequest(nil), t.calls...)
}

func TestBridgeDeliveryIntegrationShouldHandleDeliveryScenarios(t *testing.T) {
	t.Parallel()
	descriptorLookup := newDeliveryIntegrationToolRegistry(t)

	tests := []struct {
		name             string
		now              time.Time
		script           []scriptedPromptEvent
		extensionName    string
		platform         string
		scenario         string
		instanceID       string
		markerFile       string
		messageText      string
		deliveryDefaults json.RawMessage
		brokerOpts       []bridgepkg.DeliveryBrokerOption
		waitFor          func([]managerDeliveryMarker) bool
		assert           func(t *testing.T, markers []managerDeliveryMarker)
		assertContext    func(
			t *testing.T,
			env *deliveryIntegrationEnv,
			driver *scriptedPromptDriver,
			ingest bridgecontract.BridgesMessagesIngestResult,
			markers []managerDeliveryMarker,
		)
	}{
		{
			name: "ShouldProduceOrderedDeliveryStream",
			now:  time.Date(2026, 4, 11, 3, 0, 0, 0, time.UTC),
			script: []scriptedPromptEvent{
				{Type: acp.EventTypeAgentMessage, Text: "hello"},
				{Type: acp.EventTypeAgentMessage, Text: " world"},
				{Type: acp.EventTypeDone},
			},
			extensionName: "ext-bridge-order",
			scenario:      "record_deliveries",
			instanceID:    "brg-order",
			markerFile:    "deliveries.jsonl",
			waitFor: func(markers []managerDeliveryMarker) bool {
				return len(markers) >= 2 &&
					markers[len(markers)-1].Request.Event.EventType == bridgepkg.DeliveryEventTypeFinal
			},
			assert: assertMarkerDeliveryProgress,
		},
		{
			name: "Should deliver ordered redacted tool progress without transcript chrome",
			now:  time.Date(2026, 4, 11, 3, 2, 0, 0, time.UTC),
			script: []scriptedPromptEvent{
				{Type: acp.EventTypeAgentMessage, Text: "working"},
				{
					Type:       acp.EventTypeToolCall,
					ToolName:   "vendor__deploy",
					ToolCallID: "call-progress-integration",
					ToolInput:  json.RawMessage(`{"command":"echo sk-integration-progress-secret"}`),
				},
				{
					Type:       acp.EventTypeToolCall,
					ToolName:   "vendor__deploy",
					ToolCallID: "call-progress-integration",
				},
				{
					Type:            acp.EventTypeToolResult,
					ToolCallID:      "call-progress-integration",
					ToolError:       true,
					ToolErrorDetail: "deployment failed: password=hunter2",
				},
				{Type: acp.EventTypeDone},
			},
			extensionName: "ext-bridge-progress",
			platform:      "teams",
			scenario:      "record_deliveries",
			instanceID:    "brg-progress",
			markerFile:    "progress-deliveries.jsonl",
			deliveryDefaults: json.RawMessage(
				`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
			),
			brokerOpts: []bridgepkg.DeliveryBrokerOption{
				bridgepkg.WithDeliveryBrokerDescriptorLookup(descriptorLookup),
			},
			waitFor: func(markers []managerDeliveryMarker) bool {
				return len(markers) >= 4 &&
					markers[len(markers)-1].Request.Event.EventType == bridgepkg.DeliveryEventTypeFinal
			},
			assert: func(t *testing.T, markers []managerDeliveryMarker) {
				t.Helper()

				assertMarkerEvents(t, markers, []bridgepkg.DeliveryEventType{
					bridgepkg.DeliveryEventTypeStart,
					bridgepkg.DeliveryEventTypeProgress,
					bridgepkg.DeliveryEventTypeProgress,
					bridgepkg.DeliveryEventTypeFinal,
				}, []int64{1, 2, 3, 4})
				for index, marker := range markers[1:3] {
					progress := marker.Request.Event.Progress
					if progress == nil {
						t.Fatalf("progress marker %d payload = nil", index)
					}
					if got, want := progress.ToolID, "vendor__deploy"; got != want {
						t.Fatalf("progress marker %d tool id = %q, want %q", index, got, want)
					}
					if got, want := progress.Label, "Deploying service"; got != want {
						t.Fatalf("progress marker %d label = %q, want %q", index, got, want)
					}
					if got, want := progress.Preview, "echo [REDACTED]"; got != want {
						t.Fatalf("progress marker %d preview = %q, want %q", index, got, want)
					}
				}
				started := markers[1].Request.Event.Progress
				failed := markers[2].Request.Event.Progress
				if started.Phase != bridgepkg.ToolProgressPhaseStarted {
					t.Fatalf("started phase = %q, want %q", started.Phase, bridgepkg.ToolProgressPhaseStarted)
				}
				if failed.Phase != bridgepkg.ToolProgressPhaseFailed {
					t.Fatalf("failed phase = %q, want %q", failed.Phase, bridgepkg.ToolProgressPhaseFailed)
				}
				if got, want := failed.DurationMS, int64(2); got != want {
					t.Fatalf("failed duration = %d, want %d", got, want)
				}
				if strings.Contains(failed.Error, "hunter2") || !strings.Contains(failed.Error, "[REDACTED]") {
					t.Fatalf("failed error = %q, want canonical redaction", failed.Error)
				}
			},
			assertContext: func(
				t *testing.T,
				env *deliveryIntegrationEnv,
				_ *scriptedPromptDriver,
				ingest bridgecontract.BridgesMessagesIngestResult,
				_ []managerDeliveryMarker,
			) {
				t.Helper()

				events, err := env.sessions.Events(
					testutil.Context(t),
					ingest.SessionID,
					storepkg.EventQuery{},
				)
				if err != nil {
					t.Fatalf("sessions.Events() error = %v", err)
				}
				for _, event := range events {
					if event.Type == string(bridgepkg.DeliveryEventTypeProgress) ||
						strings.Contains(event.Content, `"event_type":"progress"`) {
						t.Fatalf("session transcript contains progress chrome: %#v", event)
					}
				}
			},
		},
		{
			name: "ShouldCoalesceIntermediateDeltasForSlowAdapters",
			now:  time.Date(2026, 4, 11, 3, 5, 0, 0, time.UTC),
			script: []scriptedPromptEvent{
				{Type: acp.EventTypeAgentMessage, Text: "h"},
				{Type: acp.EventTypeAgentMessage, Text: "el"},
				{Type: acp.EventTypeAgentMessage, Text: "lo"},
				{Type: acp.EventTypeAgentMessage, Text: "!"},
				{Type: acp.EventTypeDone},
			},
			extensionName: "ext-bridge-slow",
			scenario:      "slow_record_deliveries",
			instanceID:    "brg-slow",
			markerFile:    "slow-deliveries.jsonl",
			brokerOpts:    []bridgepkg.DeliveryBrokerOption{bridgepkg.WithDeliveryBrokerQueueCapacity(2)},
			waitFor: func(markers []managerDeliveryMarker) bool {
				return len(markers) >= 2 &&
					markers[len(markers)-1].Request.Event.EventType == bridgepkg.DeliveryEventTypeFinal
			},
			assert: func(t *testing.T, markers []managerDeliveryMarker) {
				t.Helper()

				if len(markers) >= 5 {
					t.Fatalf(
						"len(delivery markers) = %d, want coalesced stream smaller than 5 projected events",
						len(markers),
					)
				}
				if got := markers[0].Request.Event.EventType; got != bridgepkg.DeliveryEventTypeStart {
					t.Fatalf("first delivery event = %q, want start", got)
				}
				last := markers[len(markers)-1].Request.Event
				if got := last.EventType; got != bridgepkg.DeliveryEventTypeFinal {
					t.Fatalf("last delivery event = %q, want final", got)
				}
				if got, want := last.Seq, int64(5); got != want {
					sequence := make([]string, 0, len(markers))
					for _, marker := range markers {
						event := marker.Request.Event
						sequence = append(sequence, fmt.Sprintf("%d:%s:%q", event.Seq, event.EventType, event.Content.Text))
					}
					t.Fatalf("last delivery seq = %d, want %d; projected sequence = %v", got, want, sequence)
				}
				if got, want := last.Content.Text, "hello!"; got != want {
					t.Fatalf("last delivery content = %q, want %q", got, want)
				}
			},
		},
		{
			name: "ShouldResumeActiveDeliveryAfterRestart",
			now:  time.Date(2026, 4, 11, 3, 10, 0, 0, time.UTC),
			script: []scriptedPromptEvent{
				{Type: acp.EventTypeAgentMessage, Text: "hello"},
				{Type: acp.EventTypeDone},
			},
			extensionName: "ext-bridge-resume",
			scenario:      "exit_once_record_deliveries",
			instanceID:    "brg-resume",
			markerFile:    "resume-deliveries.jsonl",
			brokerOpts: []bridgepkg.DeliveryBrokerOption{
				bridgepkg.WithDeliveryBrokerRetryDelay(20 * time.Millisecond),
			},
			waitFor: func(markers []managerDeliveryMarker) bool {
				for _, marker := range markers {
					if marker.Request.Event.EventType == bridgepkg.DeliveryEventTypeResume {
						return true
					}
				}
				return false
			},
			assert: func(t *testing.T, markers []managerDeliveryMarker) {
				t.Helper()

				if len(markers) < 2 {
					t.Fatalf("len(delivery markers) = %d, want at least start + resume", len(markers))
				}
				if got := markers[0].Request.Event.EventType; got != bridgepkg.DeliveryEventTypeStart {
					t.Fatalf("first delivery event = %q, want start", got)
				}

				resumeIndex := -1
				for idx, marker := range markers {
					if marker.Request.Event.EventType == bridgepkg.DeliveryEventTypeResume {
						resumeIndex = idx
						break
					}
				}
				if resumeIndex < 0 {
					t.Fatalf("delivery markers = %#v, want resume event", markers)
				}
				if markers[resumeIndex].PID == markers[0].PID {
					t.Fatalf(
						"resume marker pid = %d, want restart to use a different process than %d",
						markers[resumeIndex].PID,
						markers[0].PID,
					)
				}
				if markers[resumeIndex].Request.Snapshot == nil {
					t.Fatal("resume request snapshot = nil, want resumable state")
				}
				if got, want := markers[resumeIndex].Request.Snapshot.DeliveryID, markers[0].Request.Event.DeliveryID; got != want {
					t.Fatalf("resume snapshot delivery id = %q, want %q", got, want)
				}
				if got, want := markers[resumeIndex].Request.Snapshot.LatestEventType, bridgepkg.DeliveryEventTypeFinal; got != want {
					t.Fatalf("resume snapshot latest event type = %q, want %q", got, want)
				}
			},
		},
		{
			name: "ShouldRecoverTransientDeliveryFailurePreservingUserContext",
			now:  time.Date(2026, 4, 11, 3, 15, 0, 0, time.UTC),
			script: []scriptedPromptEvent{
				{
					Type: acp.EventTypeAgentMessage,
					Text: "context preserved after transient bridge failure",
				},
				{Type: acp.EventTypeDone},
			},
			extensionName: "ext-bridge-transient",
			scenario:      "transient_fail_once_record_deliveries",
			instanceID:    "brg-transient",
			markerFile:    "transient-deliveries.jsonl",
			messageText:   "please preserve this bridge context while retrying",
			brokerOpts: []bridgepkg.DeliveryBrokerOption{
				bridgepkg.WithDeliveryBrokerRetryDelay(20 * time.Millisecond),
			},
			waitFor: func(markers []managerDeliveryMarker) bool {
				return hasRecoveredTransientDelivery(markers)
			},
			assert: func(t *testing.T, markers []managerDeliveryMarker) {
				t.Helper()

				assertTransientDeliveryRecovery(
					t,
					markers,
					"context preserved after transient bridge failure",
				)
			},
			assertContext: func(
				t *testing.T,
				env *deliveryIntegrationEnv,
				driver *scriptedPromptDriver,
				ingest bridgecontract.BridgesMessagesIngestResult,
				markers []managerDeliveryMarker,
			) {
				t.Helper()

				assertBridgeTurnContextPreserved(
					t,
					env,
					driver,
					ingest,
					markers,
					"please preserve this bridge context while retrying",
					"context preserved after transient bridge failure",
				)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			withDaemonVersion(t, "0.5.0")

			driver := newScriptedPromptDriver(tc.now, tc.script)
			markerPath := filepath.Join(t.TempDir(), tc.markerFile)
			env := newDeliveryIntegrationEnv(
				t,
				driver,
				tc.extensionName,
				tc.scenario,
				markerPath,
				tc.brokerOpts...,
			)

			instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
				ID:               tc.instanceID,
				ExtensionName:    env.extensionName,
				Platform:         tc.platform,
				RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true},
				DeliveryDefaults: tc.deliveryDefaults,
			})
			if tc.platform != "" && instance.Platform != tc.platform {
				t.Fatalf("bridge platform = %q, want low-tier platform %q", instance.Platform, tc.platform)
			}
			messageText := tc.messageText
			if strings.TrimSpace(messageText) == "" {
				messageText = "hello"
			}
			params := map[string]any{
				"bridge_instance_id":  instance.ID,
				"scope":               instance.Scope,
				"workspace_id":        instance.WorkspaceID,
				"peer_id":             "peer-1",
				"platform_message_id": "msg-" + tc.instanceID,
				"received_at":         env.now.Format(time.RFC3339Nano),
				"idempotency_key":     "idem-" + tc.instanceID,
				"content":             map[string]any{"text": messageText},
			}

			result, err := env.callWithContext(
				t,
				env.bridgeContext(instance),
				env.extensionName,
				"bridges/messages/ingest",
				params,
			)
			if err != nil {
				t.Fatalf("Handle(bridges/messages/ingest) error = %v", err)
			}
			var ingest bridgecontract.BridgesMessagesIngestResult
			decodeResult(t, result, &ingest)

			waitForDeliveryMarkers(t, markerPath, tc.waitFor)

			markers := readDeliveryMarkers(t, markerPath)
			tc.assert(t, markers)
			if tc.assertContext != nil {
				tc.assertContext(t, env, driver, ingest, markers)
			}
		})
	}
}

func TestBridgeDeliveryIntegrationShouldReconcileFreshBrokerOverSameStore(t *testing.T) {
	t.Parallel()

	t.Run("Should fail open from a fresh broker over the same GlobalDB", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC)
		homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(testutil.Context(t)); err != nil {
				t.Fatalf("GlobalDB.Close() error = %v", err)
			}
		})

		instance := bridgepkg.BridgeInstance{
			ID:            "brg-durable-restart",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-durable-restart",
			DisplayName:   "Durable restart",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		}
		if err := db.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}
		routingKey := bridgepkg.RoutingKey{
			Scope:            instance.Scope,
			BridgeInstanceID: instance.ID,
			PeerID:           "peer-durable-restart",
		}
		target := bridgepkg.DeliveryTarget{
			BridgeInstanceID: instance.ID,
			PeerID:           routingKey.PeerID,
			Mode:             bridgepkg.DeliveryModeReply,
		}
		checkpointed := make(chan bridgepkg.DeliveryLedgerCheckpoint, 2)
		store := &signalingDeliveryLedgerStore{
			DeliveryLedgerStore: db,
			checkpointed:        checkpointed,
		}
		firstTransport := &durableRestartDeliveryTransport{remoteMessageID: "remote-durable-restart"}
		firstBroker := bridgepkg.NewBroker(
			firstTransport,
			bridgepkg.WithDeliveryLedgerStore(store),
			bridgepkg.WithDeliveryBrokerNow(func() time.Time { return now }),
		)
		t.Cleanup(firstBroker.Close)
		registered, err := firstBroker.RegisterPromptDelivery(ctx, bridgepkg.PromptDeliveryRegistration{
			SessionID:      "sess-durable-restart",
			TurnID:         "turn-durable-restart",
			ExtensionName:  instance.ExtensionName,
			DeliveryID:     "delivery-durable-restart",
			RoutingKey:     routingKey,
			DeliveryTarget: target,
		})
		if err != nil {
			t.Fatalf("RegisterPromptDelivery() error = %v", err)
		}
		if err := firstBroker.Deliver(ctx, bridgepkg.DeliveryEvent{
			DeliveryID:       registered.DeliveryID,
			BridgeInstanceID: instance.ID,
			RoutingKey:       routingKey,
			DeliveryTarget:   target,
			Seq:              1,
			EventType:        bridgepkg.DeliveryEventTypeStart,
			Content:          bridgepkg.MessageContent{Text: "partial answer"},
			Operation:        bridgepkg.DeliveryOperationPost,
		}); err != nil {
			t.Fatalf("Deliver(start) error = %v", err)
		}
		select {
		case checkpoint := <-checkpointed:
			if checkpoint.LastSentSeq != 1 || checkpoint.LastAckedSeq != 0 || checkpoint.RemoteMessageID != "" {
				t.Fatalf("start intent checkpoint = %#v, want write-ahead seq 1 before transport ack", checkpoint)
			}
		case <-ctx.Done():
			t.Fatalf("start intent checkpoint did not persist: %v", ctx.Err())
		}
		select {
		case checkpoint := <-checkpointed:
			if checkpoint.LastSentSeq != 1 || checkpoint.LastAckedSeq != 1 ||
				checkpoint.RemoteMessageID != "remote-durable-restart" {
				t.Fatalf("start ack checkpoint = %#v, want durable remote anchor at seq 1", checkpoint)
			}
		case <-ctx.Done():
			t.Fatalf("start ack checkpoint did not persist: %v", ctx.Err())
		}
		firstBroker.Close()

		active, err := db.ListBridgeDeliveries(ctx, bridgepkg.DeliveryLedgerQuery{
			Scope: instance.Scope,
			State: bridgepkg.DeliveryLedgerStateActive,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveries(active) error = %v", err)
		}
		if got, want := len(active), 1; got != want {
			t.Fatalf("len(active deliveries) = %d, want %d", got, want)
		}
		if got, want := active[0].RemoteMessageID, "remote-durable-restart"; got != want {
			t.Fatalf("active remote_message_id = %q, want %q", got, want)
		}

		restartedTransport := &durableRestartDeliveryTransport{remoteMessageID: "remote-durable-restart"}
		restartedBroker := bridgepkg.NewBroker(
			restartedTransport,
			bridgepkg.WithDeliveryLedgerStore(db),
			bridgepkg.WithDeliveryBrokerRegistrationGate(),
			bridgepkg.WithDeliveryBrokerNow(func() time.Time { return now.Add(time.Minute) }),
		)
		t.Cleanup(restartedBroker.Close)
		query := bridgepkg.DeliveryLedgerQuery{Scope: instance.Scope}
		if err := restartedBroker.LoadDeliveryMetrics(ctx, query); err != nil {
			t.Fatalf("LoadDeliveryMetrics() error = %v", err)
		}
		before := restartedBroker.DeliveryMetrics()[instance.ID]
		if before.LastSuccessAt.IsZero() {
			t.Fatal("rehydrated LastSuccessAt = zero, want first-broker checkpoint metrics")
		}
		if err := restartedBroker.ReconcileDelivery(ctx, active[0], instance.ExtensionName); err != nil {
			t.Fatalf("ReconcileDelivery() error = %v", err)
		}

		calls := restartedTransport.snapshotCalls()
		if got, want := len(calls), 1; got != want {
			t.Fatalf("restarted delivery calls = %d, want %d", got, want)
		}
		resume := calls[0]
		if resume.Event.EventType != bridgepkg.DeliveryEventTypeError || resume.Snapshot != nil {
			t.Fatalf("restart request = %#v, want fail-open terminal post", resume)
		}
		if resume.Event.Operation != bridgepkg.DeliveryOperationPost || resume.Event.Reference != nil {
			t.Fatalf("restart operation/reference = %q/%#v, want universal post", resume.Event.Operation, resume.Event.Reference)
		}
		if !strings.Contains(resume.Event.Content.Text, "session stopped before delivery completed") {
			t.Fatalf("restart content = %q, want visible stopped-session terminal error", resume.Event.Content.Text)
		}

		terminal, err := db.ListBridgeDeliveries(ctx, bridgepkg.DeliveryLedgerQuery{
			Scope: instance.Scope,
			State: bridgepkg.DeliveryLedgerStateTerminalError,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveries(terminal error) error = %v", err)
		}
		if got, want := len(terminal), 1; got != want {
			t.Fatalf("len(terminal deliveries) = %d, want %d", got, want)
		}
		if got, want := terminal[0].RemoteMessageID, "remote-durable-restart"; got != want {
			t.Fatalf("terminal remote_message_id = %q, want %q", got, want)
		}
		after := restartedBroker.DeliveryMetrics()[instance.ID]
		if got, want := after.DeliveryFailuresTotal, before.DeliveryFailuresTotal+1; got != want {
			t.Fatalf("restarted delivery failures = %d, want %d", got, want)
		}
	})
}

func newDeliveryIntegrationToolRegistry(t *testing.T) *toolspkg.RuntimeRegistry {
	t.Helper()

	source := toolspkg.SourceRef{Kind: toolspkg.SourceBuiltin, Owner: "daemon"}
	provider, err := toolspkg.NewNativeProvider(source, toolspkg.NativeTool{
		Descriptor: toolspkg.Descriptor{
			ID:               "vendor__deploy",
			DisplayTitle:     "Deploy service",
			ToolPresentation: toolspkg.NewToolPresentation("Deploying service", "arg:command"),
			Description:      "Deploy one service",
			InputSchema:      json.RawMessage(`{"type":"object"}`),
			Backend: toolspkg.BackendRef{
				Kind:       toolspkg.BackendNativeGo,
				NativeName: "bridge_delivery_metadata",
			},
			Source:          source,
			Visibility:      toolspkg.VisibilityModel,
			Risk:            toolspkg.RiskRead,
			ReadOnly:        true,
			ConcurrencySafe: true,
		},
		Call: func(context.Context, toolspkg.Scope, toolspkg.CallRequest) (toolspkg.ToolResult, error) {
			return toolspkg.ToolResult{Content: []toolspkg.ToolContent{{Type: "text", Text: "unused"}}}, nil
		},
	})
	if err != nil {
		t.Fatalf("tools.NewNativeProvider() error = %v", err)
	}
	registry, err := toolspkg.NewRegistry(toolspkg.WithProviders(provider))
	if err != nil {
		t.Fatalf("tools.NewRegistry() error = %v", err)
	}
	if _, err := registry.OperatorProjection(testutil.Context(t), toolspkg.Scope{Operator: true}); err != nil {
		t.Fatalf("RuntimeRegistry.OperatorProjection() error = %v", err)
	}
	return registry
}

func newDeliveryIntegrationEnv(
	t *testing.T,
	driver session.AgentDriver,
	extensionName string,
	scenario string,
	markerPath string,
	brokerOpts ...bridgepkg.DeliveryBrokerOption,
) *deliveryIntegrationEnv {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", workspaceRoot, err)
	}
	baseNow := time.Date(2026, 4, 11, 3, 0, 0, 0, time.UTC)
	resolvedWorkspace := workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      "ws-bridge-delivery",
			RootDir: workspaceRoot,
			Name:    "bridge-delivery-workspace",
		},
		Config: aghconfig.Config{
			Defaults: aghconfig.DefaultsConfig{Agent: "coder"},
			Providers: map[string]aghconfig.ProviderConfig{
				"fake": {Command: "fake-agent"},
			},
			Permissions: aghconfig.PermissionsConfig{Mode: aghconfig.PermissionModeApproveAll},
		},
		Agents: []aghconfig.AgentDef{{
			Name:        "coder",
			Provider:    "fake",
			Permissions: string(aghconfig.PermissionModeApproveAll),
			Prompt:      "You are a reliable coder.",
		}},
		ResolvedAt: baseNow,
	}

	workspaces := newHostAPIFakeWorkspaceResolver(&resolvedWorkspace)
	globalDB, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("globaldb.OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := globalDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("globaldb.Close() error = %v", err)
		}
	})
	if err := globalDB.InsertWorkspace(testutil.Context(t), resolvedWorkspace.Workspace); err != nil {
		t.Fatalf("globalDB.InsertWorkspace() error = %v", err)
	}

	bridgeRegistry := bridgepkg.NewRegistry(globalDB, bridgepkg.WithNow(func() time.Time { return baseNow }))
	registryEnv := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest(extensionName, managerManifestOptions{
		command:      helperCommand(t),
		args:         helperArgs(),
		withEnv:      helperEnv(scenario, markerPath),
		capabilities: []string{"bridge.adapter"},
		actions: []string{
			"bridges/messages/ingest",
			"bridges/instances/get",
			"bridges/instances/report_state",
		},
		security: []string{"bridge.read", "bridge.write"},
	}), nil)
	installManagerFixture(t, registryEnv.registry, fixture, SourceUser, true)

	manager := NewManager(
		registryEnv.registry,
		WithBridgeRuntimeResolver(&stubBridgeRuntimeResolver{
			runtimes: map[string]*subprocess.InitializeBridgeRuntime{
				extensionName: testScopedBridgeRuntime(extensionName, "runtime-"+extensionName, nil),
			},
		}),
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
		withRestartBackoffMax(10*time.Millisecond),
		withHealthPollBounds(time.Millisecond, 2*time.Millisecond),
	)
	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}

	broker := bridgepkg.NewBroker(manager, brokerOpts...)
	skillsRegistry := skillspkg.NewRegistry(
		skillspkg.RegistryConfig{},
		skillspkg.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)
	checker := &CapabilityChecker{}
	notifier := NewBridgeDeliveryNotifier(broker, nil)
	sessions, err := session.NewManager(
		session.WithHomePaths(homePaths),
		session.WithDriver(driver),
		session.WithNotifier(notifier),
		session.WithWorkspaceResolver(workspaces),
		session.WithStore(func(ctx context.Context, sessionID string, path string) (session.EventRecorder, error) {
			return storeSessionDB(ctx, sessionID, path)
		}),
		session.WithSandboxRegistry(mustLocalSandboxRegistry(t)),
		session.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		session.WithNow(func() time.Time { return baseNow }),
		session.WithSessionIDGenerator(sequentialSessionIDGenerator("sess")),
		session.WithTurnIDGenerator(sequentialSessionIDGenerator("turn")),
	)
	if err != nil {
		t.Fatalf("session.NewManager() error = %v", err)
	}

	handler := NewHostAPIHandler(
		sessions,
		memory.NewStore(homePaths.MemoryDir),
		nil,
		skillsRegistry,
		WithHostAPICapabilityChecker(checker),
		WithHostAPIWorkspaceResolver(workspaces),
		WithHostAPIBridgeRegistry(bridgeRegistry),
		WithHostAPIBridgeDedupStore(globalDB),
		WithHostAPIDeliveryBroker(broker),
		WithHostAPINow(func() time.Time { return baseNow }),
		WithHostAPIBridgeIngressConfig(15*time.Minute, time.Minute),
		WithHostAPIRateLimit(1000, 1000),
	)
	checker.Register(extensionName, SourceUser, &Manifest{
		Actions:  ActionsConfig{Requires: []string{"bridges/messages/ingest"}},
		Security: SecurityConfig{Capabilities: []string{"bridge.write"}},
	})

	env := &deliveryIntegrationEnv{
		now:           baseNow,
		homePaths:     homePaths,
		workspace:     resolvedWorkspace,
		workspaces:    workspaces,
		globalDB:      globalDB,
		bridges:       bridgeRegistry,
		sessions:      sessions,
		manager:       manager,
		broker:        broker,
		handler:       handler,
		checker:       checker,
		extensionName: extensionName,
	}
	t.Cleanup(func() {
		env.stopSessions(t)
		env.broker.Close()
		if err := env.manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Stop() cleanup error = %v", err)
		}
	})

	return env
}

func (e *deliveryIntegrationEnv) callWithContext(
	t testing.TB,
	ctx context.Context,
	extName string,
	method string,
	params any,
) (any, error) {
	t.Helper()

	raw, err := marshalParams(params)
	if err != nil {
		return nil, err
	}
	return e.handler.Handle(ctx, extName, method, raw)
}

func (e *deliveryIntegrationEnv) bridgeContext(instance *bridgepkg.BridgeInstance) context.Context {
	return withHostAPIBridgeRuntime(context.Background(), testScopedBridgeRuntimeForInstance(*instance, nil))
}

func (e *deliveryIntegrationEnv) createBridgeInstance(
	t *testing.T,
	req bridgepkg.CreateInstanceRequest,
) *bridgepkg.BridgeInstance {
	t.Helper()

	if req.Scope == "" {
		req.Scope = bridgepkg.ScopeWorkspace
	}
	if req.WorkspaceID == "" && req.Scope == bridgepkg.ScopeWorkspace {
		req.WorkspaceID = e.workspace.ID
	}
	if req.Platform == "" {
		req.Platform = "telegram"
	}
	if req.ExtensionName == "" {
		req.ExtensionName = e.extensionName
	}
	if req.DisplayName == "" {
		req.DisplayName = "Bridge Delivery Test"
	}
	if req.Status == "" {
		req.Status = bridgepkg.BridgeStatusReady
		req.Enabled = true
	}

	instance, err := e.bridges.CreateInstance(testutil.Context(t), req)
	if err != nil {
		t.Fatalf("bridges.CreateInstance() error = %v", err)
	}
	return instance
}

func (e *deliveryIntegrationEnv) stopSessions(t testing.TB) {
	t.Helper()

	for _, info := range e.sessions.List() {
		if info == nil {
			continue
		}
		if err := e.sessions.Stop(testutil.Context(t), info.ID); err != nil {
			t.Fatalf("sessions.Stop(%q) cleanup error = %v", info.ID, err)
		}
	}
}

func newScriptedPromptDriver(now time.Time, script []scriptedPromptEvent) *scriptedPromptDriver {
	return &scriptedPromptDriver{
		now:       now,
		script:    append([]scriptedPromptEvent(nil), script...),
		processes: make(map[*session.AgentProcess]*scriptedPromptProcess),
	}
}

func (d *scriptedPromptDriver) Start(_ context.Context, opts acp.StartOpts) (*session.AgentProcess, error) {
	seq := d.startSeq.Add(1)
	procState := &scriptedPromptProcess{ch: make(chan struct{})}
	proc := session.NewAgentProcess(session.AgentProcessOptions{
		PID:       int(seq),
		AgentName: opts.AgentName,
		Command:   opts.Command,
		Cwd:       opts.Cwd,
		SessionID: fmt.Sprintf("acp-%d", seq),
		StartedAt: d.now.Add(time.Duration(seq) * time.Millisecond),
		Done:      procState.ch,
		Wait: func() error {
			<-procState.ch
			return nil
		},
	})

	d.mu.Lock()
	d.processes[proc] = procState
	d.mu.Unlock()
	return proc, nil
}

func (d *scriptedPromptDriver) Prompt(
	_ context.Context,
	_ *session.AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	d.prompts = append(d.prompts, req)
	script := append([]scriptedPromptEvent(nil), d.script...)
	startedAt := d.now
	d.mu.Unlock()

	events := make(chan acp.AgentEvent, len(script))
	go func() {
		defer close(events)
		for idx, item := range script {
			if item.Delay > 0 {
				time.Sleep(item.Delay)
			}
			events <- (acp.AgentEvent{
				Type:       item.Type,
				TurnID:     req.TurnID,
				Timestamp:  startedAt.Add(time.Duration(idx+1) * time.Millisecond),
				Text:       item.Text,
				Error:      item.Error,
				ToolCallID: item.ToolCallID,
			}).WithToolDetail(item.ToolName, item.ToolInput, item.ToolError, item.ToolErrorDetail)
		}
	}()
	return events, nil
}

func (d *scriptedPromptDriver) snapshotPrompts() []acp.PromptRequest {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]acp.PromptRequest(nil), d.prompts...)
}

func (d *scriptedPromptDriver) Cancel(context.Context, *session.AgentProcess) error {
	return nil
}

func (d *scriptedPromptDriver) Stop(_ context.Context, proc *session.AgentProcess) error {
	d.mu.Lock()
	state := d.processes[proc]
	d.mu.Unlock()
	if state == nil {
		return nil
	}
	state.done.Do(func() { close(state.ch) })
	return nil
}

func readDeliveryMarkers(t *testing.T, path string) []managerDeliveryMarker {
	t.Helper()

	lines, err := readFileLines(path)
	if err != nil {
		t.Fatalf("readFileLines(%q) error = %v", path, err)
	}

	markers := make([]managerDeliveryMarker, 0, len(lines))
	for _, line := range lines {
		var marker managerDeliveryMarker
		if err := json.Unmarshal([]byte(line), &marker); err != nil {
			t.Fatalf("json.Unmarshal(delivery marker) error = %v; line=%q", err, line)
		}
		markers = append(markers, marker)
	}
	return markers
}

func waitForDeliveryMarkers(t *testing.T, path string, condition func([]managerDeliveryMarker) bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		markers := readDeliveryMarkersOrEmpty(path)
		if condition(markers) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("delivery markers at %q did not satisfy condition before timeout", path)
}

func readDeliveryMarkersOrEmpty(path string) []managerDeliveryMarker {
	lines, err := readFileLines(path)
	if err != nil {
		return nil
	}
	markers := make([]managerDeliveryMarker, 0, len(lines))
	for _, line := range lines {
		var marker managerDeliveryMarker
		if json.Unmarshal([]byte(line), &marker) != nil {
			continue
		}
		markers = append(markers, marker)
	}
	return markers
}

func assertMarkerEvents(
	t *testing.T,
	markers []managerDeliveryMarker,
	wantTypes []bridgepkg.DeliveryEventType,
	wantSeqs []int64,
) {
	t.Helper()

	if len(markers) < len(wantTypes) {
		t.Fatalf("len(markers) = %d, want at least %d", len(markers), len(wantTypes))
	}
	gotTypes := make([]bridgepkg.DeliveryEventType, 0, len(markers))
	gotSeqs := make([]int64, 0, len(markers))
	gotProgress := make([]bridgepkg.ToolProgress, 0, len(markers))
	for _, marker := range markers {
		gotTypes = append(gotTypes, marker.Request.Event.EventType)
		gotSeqs = append(gotSeqs, marker.Request.Event.Seq)
		if marker.Request.Event.Progress != nil {
			gotProgress = append(gotProgress, *marker.Request.Event.Progress)
		}
	}
	if !slices.Equal(gotTypes[:len(wantTypes)], wantTypes) {
		t.Fatalf(
			"marker event types = %#v, want prefix %#v; progress = %#v",
			gotTypes,
			wantTypes,
			gotProgress,
		)
	}
	if !slices.Equal(gotSeqs[:len(wantSeqs)], wantSeqs) {
		t.Fatalf("marker seqs = %#v, want prefix %#v", gotSeqs, wantSeqs)
	}
}

func assertMarkerDeliveryProgress(t *testing.T, markers []managerDeliveryMarker) {
	t.Helper()

	if len(markers) < 2 {
		t.Fatalf("len(markers) = %d, want at least start and final", len(markers))
	}
	if got := markers[0].Request.Event.EventType; got != bridgepkg.DeliveryEventTypeStart {
		t.Fatalf("first marker event = %q, want start", got)
	}
	if got := markers[len(markers)-1].Request.Event.EventType; got != bridgepkg.DeliveryEventTypeFinal {
		t.Fatalf("last marker event = %q, want final", got)
	}

	deliveryID := markers[0].Request.Event.DeliveryID
	lastSeq := int64(0)
	for idx, marker := range markers {
		event := marker.Request.Event
		if event.DeliveryID != deliveryID {
			t.Fatalf("marker delivery_id[%d] = %q, want %q", idx, event.DeliveryID, deliveryID)
		}
		if event.Seq <= lastSeq {
			t.Fatalf("marker seq[%d] = %d, want increasing order after %d", idx, event.Seq, lastSeq)
		}
		lastSeq = event.Seq
	}
}

func hasRecoveredTransientDelivery(markers []managerDeliveryMarker) bool {
	resumeIndex := -1
	for idx, marker := range markers {
		event := marker.Request.Event
		if event.EventType == bridgepkg.DeliveryEventTypeResume &&
			marker.Request.Snapshot != nil &&
			strings.TrimSpace(event.Content.Text) != "" {
			resumeIndex = idx
			if event.Final || marker.Request.Snapshot.Final {
				return true
			}
		}
	}
	if resumeIndex < 0 {
		return false
	}
	for _, marker := range markers[resumeIndex+1:] {
		if marker.Request.Event.EventType == bridgepkg.DeliveryEventTypeFinal {
			return true
		}
	}
	return false
}

func assertTransientDeliveryRecovery(
	t *testing.T,
	markers []managerDeliveryMarker,
	wantRecoveredText string,
) {
	t.Helper()

	if len(markers) < 2 {
		t.Fatalf("len(delivery markers) = %d, want failed start plus recovery", len(markers))
	}
	start := markers[0].Request.Event
	if got := start.EventType; got != bridgepkg.DeliveryEventTypeStart {
		t.Fatalf("first delivery event = %q, want start", got)
	}
	if strings.TrimSpace(start.DeliveryID) == "" {
		t.Fatal("first delivery id = empty, want traceable delivery id")
	}

	resumeIndex := -1
	for idx, marker := range markers {
		event := marker.Request.Event
		if event.DeliveryID != start.DeliveryID {
			t.Fatalf("marker delivery_id[%d] = %q, want %q", idx, event.DeliveryID, start.DeliveryID)
		}
		if event.EventType == bridgepkg.DeliveryEventTypeResume {
			resumeIndex = idx
			break
		}
	}
	if resumeIndex < 0 {
		t.Fatalf("delivery markers = %#v, want resume after transient failure", markers)
	}

	resume := markers[resumeIndex].Request
	if resume.Snapshot == nil {
		t.Fatal("resume request snapshot = nil, want preserved delivery state")
	}
	if got, want := resume.Snapshot.DeliveryID, start.DeliveryID; got != want {
		t.Fatalf("resume snapshot delivery id = %q, want %q", got, want)
	}
	if strings.TrimSpace(resume.Snapshot.SessionID) == "" {
		t.Fatal("resume snapshot session id = empty, want traceable session")
	}
	if strings.TrimSpace(resume.Snapshot.TurnID) == "" {
		t.Fatal("resume snapshot turn id = empty, want traceable turn")
	}
	if got := resume.Event.Resume; got == nil || strings.TrimSpace(string(got.LatestEventType)) == "" {
		t.Fatalf("resume state = %#v, want latest event type", got)
	}
	if !strings.Contains(resume.Event.Content.Text, wantRecoveredText) {
		t.Fatalf("resume content = %q, want %q", resume.Event.Content.Text, wantRecoveredText)
	}
	if !strings.Contains(resume.Snapshot.CurrentContent.Text, wantRecoveredText) {
		t.Fatalf("resume snapshot content = %q, want %q", resume.Snapshot.CurrentContent.Text, wantRecoveredText)
	}

	if resume.Event.Final || resume.Snapshot.Final {
		return
	}
	for _, marker := range markers[resumeIndex+1:] {
		event := marker.Request.Event
		if event.EventType != bridgepkg.DeliveryEventTypeFinal {
			continue
		}
		if !event.Final {
			t.Fatal("final recovery event Final = false, want true")
		}
		if !strings.Contains(event.Content.Text, wantRecoveredText) {
			t.Fatalf("final recovery content = %q, want %q", event.Content.Text, wantRecoveredText)
		}
		return
	}
	t.Fatalf("delivery markers = %#v, want terminal recovery evidence", markers)
}

func assertBridgeTurnContextPreserved(
	t *testing.T,
	env *deliveryIntegrationEnv,
	driver *scriptedPromptDriver,
	ingest bridgecontract.BridgesMessagesIngestResult,
	markers []managerDeliveryMarker,
	wantInboundText string,
	wantOutboundText string,
) {
	t.Helper()

	if strings.TrimSpace(ingest.SessionID) == "" {
		t.Fatal("ingest session_id = empty, want routable bridge session")
	}
	assertBridgePromptCarriesInboundContext(t, driver.snapshotPrompts(), wantInboundText)
	assertResumeMatchesIngestedSession(t, markers, ingest.SessionID)

	messages := waitForSessionTranscriptText(
		t,
		env.sessions,
		ingest.SessionID,
		wantInboundText,
		wantOutboundText,
	)
	visible := transcriptpkg.JoinUIMessageText(messages)
	if !strings.Contains(visible, "Inbound bridge message") {
		t.Fatalf("transcript visible text = %q, want bridge prompt context", visible)
	}
}

func assertBridgePromptCarriesInboundContext(
	t *testing.T,
	prompts []acp.PromptRequest,
	wantInboundText string,
) {
	t.Helper()

	if len(prompts) == 0 {
		t.Fatal("len(prompts) = 0, want bridge prompt recorded")
	}
	first := prompts[0]
	if !strings.Contains(first.Message, wantInboundText) {
		t.Fatalf("prompt message = %q, want inbound text %q", first.Message, wantInboundText)
	}
	if !strings.Contains(first.Message, "Inbound bridge message") {
		t.Fatalf("prompt message = %q, want rendered bridge context", first.Message)
	}
	if first.Meta.Network == nil {
		t.Fatal("prompt network meta = nil, want bridge trace metadata")
	}
	if got, want := first.Meta.Network.MessageID, "msg-brg-transient"; got != want {
		t.Fatalf("prompt network message id = %q, want %q", got, want)
	}
}

func assertResumeMatchesIngestedSession(
	t *testing.T,
	markers []managerDeliveryMarker,
	wantSessionID string,
) {
	t.Helper()

	for _, marker := range markers {
		if marker.Request.Event.EventType != bridgepkg.DeliveryEventTypeResume || marker.Request.Snapshot == nil {
			continue
		}
		if got := marker.Request.Snapshot.SessionID; got != wantSessionID {
			t.Fatalf("resume snapshot session id = %q, want ingest session %q", got, wantSessionID)
		}
		return
	}
	t.Fatalf("delivery markers = %#v, want resume snapshot", markers)
}

func waitForSessionTranscriptText(
	t *testing.T,
	sessions *session.Manager,
	sessionID string,
	wantTexts ...string,
) []transcriptpkg.UIMessage {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var lastVisible string
	var lastErr error
	for time.Now().Before(deadline) {
		page, err := sessions.TranscriptPage(testutil.Context(t), sessionID, transcriptpkg.PageQuery{})
		if err != nil {
			lastErr = err
		} else {
			messages := transcriptpkg.MessagesFromEntries(page.Entries)
			lastVisible = transcriptpkg.JoinUIMessageText(messages)
			if containsAllText(lastVisible, wantTexts) {
				return messages
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("TranscriptPage(%q) error = %v", sessionID, lastErr)
	}
	t.Fatalf("TranscriptPage(%q) visible text = %q, want all %#v", sessionID, lastVisible, wantTexts)
	return nil
}

func containsAllText(haystack string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}
