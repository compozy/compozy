//go:build integration && !windows

package daemon

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	extensiontest "github.com/compozy/agh/internal/extensiontest"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
)

const bridgeIngressFixtureAgentName = "mock-bridge-runner"

const bridgeIngressFixtureSecret = "sk-e2e-progress-secret"

func TestDaemonE2EBridgeIngressCreatesAndReusesRouteThroughOptedInLowTierContractMock(t *testing.T) {
	t.Run(
		"Should round-trip ordered progress through an opted-in low-tier contract mock",
		testDaemonE2EBridgeIngressCreatesAndReusesRouteThroughOptedInLowTierContractMock,
	)
}

func TestDaemonE2EBridgeDeliveryReconcilesAfterRestart(t *testing.T) {
	t.Run("Should post a visible terminal error before startup accepts new work", func(t *testing.T) {
		acpmock.RequireDriver(t)

		repoRoot := daemonBridgeRuntimeRepoRoot(t)
		extensionDir := prepareDaemonTelegramReferenceExtension(t, repoRoot)
		markers := extensiontest.NewTempMarkerPaths(t)
		env := markers.Env()
		delete(env, extensiontest.EnvCrashOncePath)
		env["AGH_TEST_TELEGRAM_TOKEN"] = "telegram-bot-token"
		homePaths := e2etest.NewHomePaths(t)
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		options := e2etest.RuntimeHarnessOptions{
			HomePaths: homePaths,
			ConfigSeed: e2etest.ConfigSeedOptions{
				DefaultAgent:   bridgeIngressFixtureAgentName,
				PermissionMode: aghconfig.PermissionModeApproveAll,
				Mutate:         allowUnverifiedBridgeExtensionInstalls,
			},
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  mockFixturePath(t, "bridge_ingress_fixture.json"),
				FixtureAgent: "bridge-runner",
				AgentName:    bridgeIngressFixtureAgentName,
			}},
			Workspace: e2etest.WorkspaceSeedOptions{Root: workspaceRoot},
			Env:       env,
		}
		first := e2etest.StartRuntimeHarness(t, &options)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		checksum, err := extensionpkg.ComputeDirectoryChecksum(extensionDir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", extensionDir, err)
		}
		if _, err := first.InstallExtension(ctx, aghcontract.InstallExtensionRequest{
			Path:            extensionDir,
			Checksum:        checksum,
			AllowUnverified: true,
		}); err != nil {
			t.Fatalf("InstallExtension(%q) error = %v", extensionDir, err)
		}
		waitForRuntimeCondition(t, "telegram restart extension registered", 10*time.Second, func() bool {
			ext, err := first.GetExtension(ctx, "telegram-reference")
			return err == nil && ext.Enabled
		})
		created, err := first.CreateBridge(ctx, aghcontract.CreateBridgeRequest{
			Scope:         bridgepkg.ScopeWorkspace,
			WorkspaceID:   first.WorkspaceID,
			Platform:      "telegram",
			ExtensionName: "telegram-reference",
			DisplayName:   "Telegram restart contract",
			Enabled:       false,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		if err != nil {
			t.Fatalf("CreateBridge() error = %v", err)
		}
		bridgeID := created.Bridge.ID
		secretValue := "telegram-bot-token"
		if _, err := first.PutBridgeSecretBinding(
			ctx,
			bridgeID,
			"bot_token",
			aghcontract.PutBridgeSecretBindingRequest{
				SecretRef:   "vault:bridges/" + bridgeID + "/bot_token",
				Kind:        "token",
				SecretValue: &secretValue,
			},
		); err != nil {
			t.Fatalf("PutBridgeSecretBinding() error = %v", err)
		}
		if _, err := first.EnableBridge(ctx, bridgeID); err != nil {
			t.Fatalf("EnableBridge(%q) error = %v", bridgeID, err)
		}
		waitForRuntimeCondition(t, "restart bridge ready", 10*time.Second, func() bool {
			bridge, err := first.GetBridge(ctx, bridgeID)
			return err == nil && bridge.Health.Status.Normalize() == bridgepkg.BridgeStatusReady
		})

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := first.Stop(stopCtx); err != nil {
			stopCancel()
			t.Fatalf("Stop(first daemon) error = %v", err)
		}
		stopCancel()

		seedCtx, seedCancel := context.WithTimeout(context.Background(), 20*time.Second)
		db, err := globaldb.OpenGlobalDB(seedCtx, homePaths.DatabaseFile)
		if err != nil {
			seedCancel()
			t.Fatalf("OpenGlobalDB(seed restart delivery) error = %v", err)
		}
		now := time.Now().UTC()
		routingKey := bridgepkg.RoutingKey{
			Scope:            bridgepkg.ScopeWorkspace,
			WorkspaceID:      first.WorkspaceID,
			BridgeInstanceID: bridgeID,
			PeerID:           "777",
		}
		if err := db.CreateBridgeDelivery(seedCtx, bridgepkg.DeliveryLedgerRecord{
			DeliveryID:       "delivery-daemon-restart",
			SessionID:        "session-daemon-restart",
			TurnID:           "turn-daemon-restart",
			RoutingKey:       routingKey,
			BridgeInstanceID: bridgeID,
			Scope:            bridgepkg.ScopeWorkspace,
			WorkspaceID:      first.WorkspaceID,
			State:            bridgepkg.DeliveryLedgerStateActive,
			CreatedAt:        now.Add(-time.Minute),
			UpdatedAt:        now.Add(-time.Minute),
		}); err != nil {
			if closeErr := db.Close(seedCtx); closeErr != nil {
				t.Errorf("GlobalDB.Close(after create failure) error = %v", closeErr)
			}
			seedCancel()
			t.Fatalf("CreateBridgeDelivery(seed) error = %v", err)
		}
		if err := db.CheckpointBridgeDelivery(seedCtx, bridgepkg.DeliveryLedgerCheckpoint{
			DeliveryID:      "delivery-daemon-restart",
			State:           bridgepkg.DeliveryLedgerStateActive,
			LastSentSeq:     1,
			LastAckedSeq:    1,
			RemoteMessageID: "remote-before-daemon-restart",
			UpdatedAt:       now.Add(-30 * time.Second),
		}); err != nil {
			if closeErr := db.Close(seedCtx); closeErr != nil {
				t.Errorf("GlobalDB.Close(after checkpoint failure) error = %v", closeErr)
			}
			seedCancel()
			t.Fatalf("CheckpointBridgeDelivery(seed) error = %v", err)
		}
		if err := db.Close(seedCtx); err != nil {
			seedCancel()
			t.Fatalf("GlobalDB.Close(seed) error = %v", err)
		}
		seedCancel()

		restarted := e2etest.StartRuntimeHarness(t, &options)
		deliveries := extensiontest.WaitForDeliveryMarkers(
			t,
			markers,
			15*time.Second,
			func(records []extensiontest.DeliveryRecord) bool {
				for _, record := range records {
					if record.Request.Event.DeliveryID == "delivery-daemon-restart" &&
						record.Request.Event.EventType == bridgepkg.DeliveryEventTypeError {
						return true
					}
				}
				return false
			},
		)
		var reconciled *extensiontest.DeliveryRecord
		for index := range deliveries {
			if deliveries[index].Request.Event.DeliveryID == "delivery-daemon-restart" {
				reconciled = &deliveries[index]
				break
			}
		}
		if reconciled == nil {
			t.Fatalf("deliveries = %#v, want reconciled restart delivery", deliveries)
		}
		if reconciled.Request.Event.Operation != bridgepkg.DeliveryOperationPost ||
			reconciled.Request.Event.Reference != nil || reconciled.Request.Snapshot != nil {
			t.Fatalf("reconciled request = %#v, want universal terminal post", reconciled.Request)
		}
		if got, want := reconciled.Request.Event.Content.Text, "session stopped before delivery completed"; got != want {
			t.Fatalf("reconciled content = %q, want %q", got, want)
		}

		restartStopCtx, restartStopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := restarted.Stop(restartStopCtx); err != nil {
			restartStopCancel()
			t.Fatalf("Stop(restarted daemon) error = %v", err)
		}
		restartStopCancel()
		verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer verifyCancel()
		verifiedDB, err := globaldb.OpenGlobalDB(verifyCtx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB(verify restart) error = %v", err)
		}
		defer func() {
			if err := verifiedDB.Close(verifyCtx); err != nil {
				t.Errorf("GlobalDB.Close(verify restart) error = %v", err)
			}
		}()
		terminal, err := verifiedDB.ListBridgeDeliveries(verifyCtx, bridgepkg.DeliveryLedgerQuery{
			Scope:            bridgepkg.ScopeWorkspace,
			WorkspaceID:      first.WorkspaceID,
			BridgeInstanceID: bridgeID,
			State:            bridgepkg.DeliveryLedgerStateTerminalError,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveries(terminal restart) error = %v", err)
		}
		if len(terminal) != 1 || terminal[0].DeliveryID != "delivery-daemon-restart" {
			t.Fatalf("terminal restart rows = %#v, want seeded delivery terminalized", terminal)
		}
	})
}

func testDaemonE2EBridgeIngressCreatesAndReusesRouteThroughOptedInLowTierContractMock(t *testing.T) {
	t.Helper()

	acpmock.RequireDriver(t)

	repoRoot := daemonBridgeRuntimeRepoRoot(t)
	extensionDir := prepareDaemonTelegramReferenceExtension(t, repoRoot)
	rewriteDaemonBridgeContractPlatform(t, extensionDir, "telegram", "teams")

	markers := extensiontest.NewTempMarkerPaths(t)
	env := markers.Env()
	delete(env, extensiontest.EnvCrashOncePath)
	env["AGH_TEST_TELEGRAM_TOKEN"] = "telegram-bot-token"

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent:   bridgeIngressFixtureAgentName,
			PermissionMode: aghconfig.PermissionModeApproveAll,
			Mutate:         allowUnverifiedBridgeExtensionInstalls,
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "bridge_ingress_fixture.json"),
			FixtureAgent: "bridge-runner",
			AgentName:    bridgeIngressFixtureAgentName,
		}},
		Env: env,
	})

	registration, ok := harness.MockAgentRegistration(bridgeIngressFixtureAgentName)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) = missing, want present", bridgeIngressFixtureAgentName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var (
		bridgeID  string
		sessionID string
	)
	registerBridgeExtensionArtifacts(t, harness, markers, registration, &bridgeID, &sessionID)

	checksum, err := extensionpkg.ComputeDirectoryChecksum(extensionDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", extensionDir, err)
	}

	installed, err := harness.InstallExtension(ctx, aghcontract.InstallExtensionRequest{
		Path:            extensionDir,
		Checksum:        checksum,
		AllowUnverified: true,
	})
	if err != nil {
		t.Fatalf("InstallExtension(%q) error = %v", extensionDir, err)
	}
	if got, want := installed.Name, "telegram-reference"; got != want {
		t.Fatalf("installed.Name = %q, want %q", got, want)
	}

	waitForRuntimeCondition(t, "telegram extension registered", 10*time.Second, func() bool {
		ext, err := harness.GetExtension(ctx, "telegram-reference")
		return err == nil && ext.Enabled
	})

	createdBridge, err := harness.CreateBridge(ctx, aghcontract.CreateBridgeRequest{
		Scope:         bridgepkg.ScopeWorkspace,
		WorkspaceID:   harness.WorkspaceID,
		Platform:      "teams",
		ExtensionName: "telegram-reference",
		DisplayName:   "Teams Contract Mock Runtime E2E",
		Enabled:       false,
		RoutingPolicy: bridgepkg.RoutingPolicy{
			IncludePeer:   true,
			IncludeThread: true,
		},
		DeliveryDefaults: aghcontract.BridgeDeliveryDefaultsPayload(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		),
	})
	if err != nil {
		t.Fatalf("CreateBridge() error = %v", err)
	}
	bridgeID = createdBridge.Bridge.ID
	if got, want := createdBridge.Bridge.Status.Normalize(), bridgepkg.BridgeStatusDisabled; got != want {
		t.Fatalf("created bridge status = %q, want %q", got, want)
	}
	if createdBridge.Bridge.Enabled {
		t.Fatalf("created bridge enabled = %v, want false before explicit start", createdBridge.Bridge.Enabled)
	}

	secretValue := "telegram-bot-token"
	secretRef := "vault:bridges/" + bridgeID + "/bot_token"
	binding, err := harness.PutBridgeSecretBinding(
		ctx,
		bridgeID,
		"bot_token",
		aghcontract.PutBridgeSecretBindingRequest{
			SecretRef:   secretRef,
			Kind:        "token",
			SecretValue: &secretValue,
		},
	)
	if err != nil {
		t.Fatalf("PutBridgeSecretBinding() error = %v", err)
	}
	if got, want := binding.BindingName, "bot_token"; got != want {
		t.Fatalf("binding.BindingName = %q, want %q", got, want)
	}

	if _, err := harness.EnableBridge(ctx, bridgeID); err != nil {
		t.Fatalf("EnableBridge(%q) error = %v", bridgeID, err)
	}

	handshake := extensiontest.WaitForHandshakeMarker(t, markers, 15*time.Second)
	if handshake.Request.Runtime.Bridge == nil {
		t.Fatal("initialize runtime.bridge = nil, want bridge runtime metadata")
	}
	if got, want := handshake.Request.Runtime.Bridge.Platform, "teams"; got != want {
		t.Fatalf("initialize runtime.bridge.platform = %q, want %q", got, want)
	}
	managed, ok := handshake.Request.Runtime.Bridge.ManagedInstance(bridgeID)
	if !ok || managed == nil {
		t.Fatalf("initialize runtime missing managed instance %q", bridgeID)
	}
	if got, want := managed.Instance.ExtensionName, "telegram-reference"; got != want {
		t.Fatalf("managed.Instance.ExtensionName = %q, want %q", got, want)
	}
	if got, want := managed.Instance.Platform, "teams"; got != want {
		t.Fatalf("managed.Instance.Platform = %q, want %q", got, want)
	}
	baseline := managed.Instance
	baseline.DeliveryDefaults = nil
	if got, want := bridgecontract.ResolveProgressConfig(&baseline, baseline.Platform).ToolProgress,
		bridgecontract.ProgressModeOff; got != want {
		t.Fatalf("default low-tier progress mode = %q, want %q", got, want)
	}
	effectiveProgress := bridgecontract.ResolveProgressConfig(&managed.Instance, managed.Instance.Platform)
	if got, want := effectiveProgress.ToolProgress, bridgecontract.ProgressModeAll; got != want {
		t.Fatalf("managed progress mode = %q, want %q", got, want)
	}
	if got, want := effectiveProgress.Grouping, bridgecontract.ProgressGroupingAccumulate; got != want {
		t.Fatalf("managed progress grouping = %q, want %q", got, want)
	}
	if len(managed.BoundSecrets) == 0 {
		t.Fatal("managed.BoundSecrets = empty, want resolved bot_token binding")
	}
	if got, want := managed.BoundSecrets[0].BindingName, "bot_token"; got != want {
		t.Fatalf("managed.BoundSecrets[0].BindingName = %q, want %q", got, want)
	}

	states := extensiontest.WaitForStateMarkers(
		t,
		markers,
		15*time.Second,
		func(records []extensiontest.StateRecord) bool {
			return len(records) > 0 && records[len(records)-1].Status.Normalize() == bridgepkg.BridgeStatusReady
		},
	)
	if got, want := states[len(states)-1].BridgeInstanceID, bridgeID; got != want {
		t.Fatalf("last state bridge_instance_id = %q, want %q", got, want)
	}

	waitForRuntimeCondition(t, "telegram extension active", 10*time.Second, func() bool {
		ext, err := harness.GetExtension(ctx, "telegram-reference")
		return err == nil && ext.State == "active" && ext.Health == "healthy"
	})

	waitForRuntimeCondition(t, "bridge ready before ingress", 10*time.Second, func() bool {
		bridge, err := harness.GetBridge(ctx, bridgeID)
		return err == nil &&
			bridge.Health.Status.Normalize() == bridgepkg.BridgeStatusReady &&
			bridge.Health.RouteCount == 0
	})

	originalMessageAt := time.Now().UTC().Truncate(time.Second)
	extensiontest.AppendInboundUpdateMarker(
		t,
		markers,
		telegramRuntimeInboundUpdate(originalMessageAt, 9001, 321, "Need a runtime bridge summary"),
	)

	firstIngests := extensiontest.WaitForIngestMarkers(
		t,
		markers,
		15*time.Second,
		func(records []extensiontest.IngestRecord) bool {
			return len(records) >= 1 && strings.TrimSpace(records[0].Result.SessionID) != ""
		},
	)
	firstIngest := firstIngests[0]
	sessionID = firstIngest.Result.SessionID

	firstHandling, err := classifyBridgeSessionHandling(firstIngest.Result, "")
	if err != nil {
		t.Fatalf("classifyBridgeSessionHandling(first) error = %v", err)
	}
	if got, want := firstHandling, bridgeSessionHandlingCreated; got != want {
		t.Fatalf("first bridge session handling = %q, want %q", got, want)
	}

	firstDeliveries := extensiontest.WaitForDeliveryMarkers(
		t,
		markers,
		15*time.Second,
		func(records []extensiontest.DeliveryRecord) bool {
			return countDeliveryEvents(records, bridgepkg.DeliveryEventTypeFinal) >= 1
		},
	)
	if !hasDeliveryEventType(firstDeliveries, bridgepkg.DeliveryEventTypeStart) {
		t.Fatalf("first deliveries = %#v, want start event", firstDeliveries)
	}
	progressDelivery := findDeliveryEvent(firstDeliveries, bridgepkg.DeliveryEventTypeProgress)
	if progressDelivery == nil || progressDelivery.Request.Event.Progress == nil {
		t.Fatalf("first deliveries = %#v, want typed progress event", firstDeliveries)
	}
	if got, want := progressDelivery.Request.Event.Progress.ToolID, "agh__terminal"; got != want {
		t.Fatalf("progress tool_id = %q, want %q", got, want)
	}
	if got, want := progressDelivery.Request.Event.Progress.Phase, bridgepkg.ToolProgressPhaseStarted; got != want {
		t.Fatalf("progress phase = %q, want %q", got, want)
	}
	if got, want := progressDelivery.Request.Event.Progress.Emoji, "⚙️"; got != want {
		t.Fatalf("progress emoji = %q, want %q", got, want)
	}
	if got, want := progressDelivery.Request.Event.Progress.Label, "agh__terminal:"; got != want {
		t.Fatalf("progress label = %q, want %q", got, want)
	}
	if got, want := progressDelivery.Request.Event.Progress.Preview, `"echo"`; got != want {
		t.Fatalf("progress preview = %q, want %q", got, want)
	}
	if strings.Contains(progressDelivery.Request.Event.Progress.Preview, bridgeIngressFixtureSecret) {
		t.Fatalf("progress preview = %q, contains raw fixture secret", progressDelivery.Request.Event.Progress.Preview)
	}
	if progressDelivery.Ack.RemoteMessageID != "" || progressDelivery.Ack.ReplaceRemoteMessageID != "" {
		t.Fatalf("progress ack = %#v, want explicit no-op acknowledgement", progressDelivery.Ack)
	}
	progressRecords := deliveryEvents(firstDeliveries, bridgepkg.DeliveryEventTypeProgress)
	wantProgressLifecycle := []struct {
		toolID string
		phase  bridgepkg.ToolProgressPhase
	}{
		{toolID: "agh__terminal", phase: bridgepkg.ToolProgressPhaseStarted},
		{toolID: "agh__terminal", phase: bridgepkg.ToolProgressPhaseCompleted},
		{toolID: "agh__tool_list", phase: bridgepkg.ToolProgressPhaseStarted},
		{toolID: "agh__tool_list", phase: bridgepkg.ToolProgressPhaseCompleted},
		{toolID: "agh__skill_list", phase: bridgepkg.ToolProgressPhaseStarted},
		{toolID: "agh__skill_list", phase: bridgepkg.ToolProgressPhaseFailed},
	}
	if got, want := len(progressRecords), len(wantProgressLifecycle); got != want {
		t.Fatalf("progress delivery count = %d, want %d: %#v", got, want, progressRecords)
	}
	lastProgressIndex := 0
	for index, want := range wantProgressLifecycle {
		record := progressRecords[index]
		progress := record.Request.Event.Progress
		if progress == nil {
			t.Fatalf("progress[%d] payload = nil", index)
		}
		if progress.ToolID != want.toolID || progress.Phase != want.phase {
			t.Fatalf(
				"progress[%d] lifecycle = %s/%s, want %s/%s",
				index,
				progress.ToolID,
				progress.Phase,
				want.toolID,
				want.phase,
			)
		}
		if progress.Index <= lastProgressIndex {
			t.Fatalf(
				"progress[%d] index = %d, want greater than %d",
				index,
				progress.Index,
				lastProgressIndex,
			)
		}
		lastProgressIndex = progress.Index
		if record.Ack.RemoteMessageID != "" || record.Ack.ReplaceRemoteMessageID != "" {
			t.Fatalf("progress[%d] ack = %#v, want lifecycle-only ids", index, record.Ack)
		}
	}

	waitForRuntimeCondition(t, "first bridge transcript", 15*time.Second, func() bool {
		return sessionTranscriptHasNeedle(ctx, harness, sessionID, "Need a runtime bridge summary") &&
			sessionTranscriptHasNeedle(ctx, harness, sessionID, "Bridge summary: initial route handled.")
	})

	waitForRuntimeCondition(t, "bridge route created after first ingress", 15*time.Second, func() bool {
		bridge, err := harness.GetBridge(ctx, bridgeID)
		return err == nil &&
			bridge.Health.RouteCount == 1 &&
			bridge.Health.LastSuccessAt != nil
	})

	editedAt := originalMessageAt.Add(time.Minute)
	extensiontest.AppendInboundUpdateMarker(
		t,
		markers,
		telegramRuntimeEditedInboundUpdate(
			originalMessageAt,
			editedAt,
			9002,
			321,
			"Need an edited runtime bridge summary",
		),
	)

	editIngests := extensiontest.WaitForIngestMarkers(
		t,
		markers,
		15*time.Second,
		func(records []extensiontest.IngestRecord) bool {
			return len(records) >= 2 && strings.TrimSpace(records[1].Result.SessionID) != ""
		},
	)
	editIngest := editIngests[1]
	editHandling, err := classifyBridgeSessionHandling(editIngest.Result, firstIngest.Result.SessionID)
	if err != nil {
		t.Fatalf("classifyBridgeSessionHandling(edit) error = %v", err)
	}
	if got, want := editHandling, bridgeSessionHandlingReused; got != want {
		t.Fatalf("edit bridge session handling = %q, want %q", got, want)
	}
	if got, want := editIngest.Result.SessionID, firstIngest.Result.SessionID; got != want {
		t.Fatalf("edit ingest session_id = %q, want reused %q", got, want)
	}
	if got, want := editIngest.Result.RoutingKey, firstIngest.Result.RoutingKey; got != want {
		t.Fatalf("edit ingest routing_key = %#v, want %#v", got, want)
	}
	if got, want := editIngest.Envelope.EventFamily, bridgepkg.InboundEventFamilyEdit; got != want {
		t.Fatalf("edit envelope event_family = %q, want %q", got, want)
	}
	if editIngest.Envelope.Edit == nil {
		t.Fatal("edit envelope payload = nil, want typed edit")
	}
	if got, want := editIngest.Envelope.Edit.MessageID, "321"; got != want {
		t.Fatalf("edit envelope message_id = %q, want %q", got, want)
	}
	if got, want := editIngest.Envelope.Edit.NewText, "Need an edited runtime bridge summary"; got != want {
		t.Fatalf("edit envelope new_text = %q, want %q", got, want)
	}
	if got, want := editIngest.Envelope.Edit.OriginalTimestamp, originalMessageAt; !got.Equal(want) {
		t.Fatalf("edit envelope original_timestamp = %s, want %s", got, want)
	}

	waitForRuntimeCondition(t, "edited bridge transcript", 15*time.Second, func() bool {
		return sessionTranscriptHasNeedle(ctx, harness, sessionID, "Need an edited runtime bridge summary") &&
			sessionTranscriptHasNeedle(ctx, harness, sessionID, "Bridge summary: edit reused the existing session.")
	})

	extensiontest.AppendInboundUpdateMarker(
		t,
		markers,
		telegramRuntimeReplyInboundUpdate(
			editedAt.Add(time.Minute),
			9003,
			322,
			"Provide a follow-up runtime bridge summary",
			321,
			"Need an edited runtime bridge summary",
		),
	)

	ingests := extensiontest.WaitForIngestMarkers(
		t,
		markers,
		15*time.Second,
		func(records []extensiontest.IngestRecord) bool {
			return len(records) >= 3 && strings.TrimSpace(records[2].Result.SessionID) != ""
		},
	)
	replyIngest := ingests[2]

	replyHandling, err := classifyBridgeSessionHandling(replyIngest.Result, firstIngest.Result.SessionID)
	if err != nil {
		t.Fatalf("classifyBridgeSessionHandling(reply) error = %v", err)
	}
	if got, want := replyHandling, bridgeSessionHandlingReused; got != want {
		t.Fatalf("reply bridge session handling = %q, want %q", got, want)
	}
	if got, want := replyIngest.Result.SessionID, firstIngest.Result.SessionID; got != want {
		t.Fatalf("reply ingest session_id = %q, want reused %q", got, want)
	}
	if got, want := replyIngest.Result.RoutingKey, firstIngest.Result.RoutingKey; got != want {
		t.Fatalf("reply ingest routing_key = %#v, want %#v", got, want)
	}
	if got, want := replyIngest.Envelope.ReplyToText, "Need an edited runtime bridge summary"; got != want {
		t.Fatalf("reply envelope reply_to_text = %q, want %q", got, want)
	}
	if got, want := replyIngest.Envelope.ReplyToAuthorID, "888"; got != want {
		t.Fatalf("reply envelope reply_to_author_id = %q, want %q", got, want)
	}
	if got, want := replyIngest.Envelope.ReplyToAuthorName, "Alice Example"; got != want {
		t.Fatalf("reply envelope reply_to_author_name = %q, want %q", got, want)
	}

	deliveries := extensiontest.WaitForDeliveryMarkers(
		t,
		markers,
		15*time.Second,
		func(records []extensiontest.DeliveryRecord) bool {
			return countDeliveryEvents(records, bridgepkg.DeliveryEventTypeFinal) >= 3
		},
	)
	if got, want := uniqueDeliveryCount(deliveries), 3; got < want {
		t.Fatalf("unique delivery IDs = %d, want at least %d", got, want)
	}
	if !hasDeliveryEventType(deliveries, bridgepkg.DeliveryEventTypeStart) {
		t.Fatalf("deliveries = %#v, want start event", deliveries)
	}
	if !hasDeliveryEventType(deliveries, bridgepkg.DeliveryEventTypeFinal) {
		t.Fatalf("deliveries = %#v, want final event", deliveries)
	}

	routes, err := harness.ListBridgeRoutes(ctx, bridgeID)
	if err != nil {
		t.Fatalf("ListBridgeRoutes(%q) error = %v", bridgeID, err)
	}
	if got, want := len(routes), 1; got != want {
		t.Fatalf("len(routes) = %d, want %d", got, want)
	}
	route := routes[0]
	if got, want := route.SessionID, firstIngest.Result.SessionID; got != want {
		t.Fatalf("route.SessionID = %q, want %q", got, want)
	}
	if got, want := route.AgentName, bridgeIngressFixtureAgentName; got != want {
		t.Fatalf("route.AgentName = %q, want %q", got, want)
	}
	routeKey, err := route.RoutingKey().Serialize()
	if err != nil {
		t.Fatalf("route.RoutingKey().Serialize() error = %v", err)
	}
	ingressKey, err := firstIngest.Result.RoutingKey.Serialize()
	if err != nil {
		t.Fatalf("firstIngest.Result.RoutingKey.Serialize() error = %v", err)
	}
	if routeKey != ingressKey {
		t.Fatalf("route routing key = %q, want %q", routeKey, ingressKey)
	}

	bindings, err := harness.ListBridgeSecretBindings(ctx, bridgeID)
	if err != nil {
		t.Fatalf("ListBridgeSecretBindings(%q) error = %v", bridgeID, err)
	}
	if got, want := len(bindings), 1; got != want {
		t.Fatalf("len(bindings) = %d, want %d", got, want)
	}
	if got, want := bindings[0].SecretRef, secretRef; got != want {
		t.Fatalf("bindings[0].SecretRef = %q, want %q", got, want)
	}

	bridge, err := harness.GetBridge(ctx, bridgeID)
	if err != nil {
		t.Fatalf("GetBridge(%q) error = %v", bridgeID, err)
	}
	if got, want := bridge.Health.RouteCount, 1; got != want {
		t.Fatalf("bridge.Health.RouteCount = %d, want %d", got, want)
	}
	if bridge.Health.LastSuccessAt == nil {
		t.Fatal("bridge.Health.LastSuccessAt = nil, want successful delivery progression")
	}

	transcript := mustSessionTranscript(t, ctx, harness, sessionID)
	transcriptContent := joinTranscriptContent(sessionTranscriptMessages(transcript))
	for _, needle := range []string{
		"Need a runtime bridge summary",
		"Need an edited runtime bridge summary",
		"Provide a follow-up runtime bridge summary",
		"Bridge summary: initial route handled.",
		"Bridge summary: edit reused the existing session.",
		"Bridge summary: follow-up reused the existing session.",
	} {
		if !strings.Contains(transcriptContent, needle) {
			t.Fatalf("transcript = %q, want %q", transcriptContent, needle)
		}
	}

	diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(%q) error = %v", registration.DiagnosticsPath, err)
	}
	promptDiagnostics := acpmock.PromptDiagnostics(diagnostics)
	editDiagnostic := findBridgePromptDiagnostic(promptDiagnostics, "edit", "321")
	if editDiagnostic == nil {
		t.Fatalf("prompt diagnostics = %#v, want edit/321 metadata", promptDiagnostics)
	}
	for _, needle := range []string{
		"Inbound bridge message edited",
		"Message ID: 321",
		"Operation: updated",
		"Original timestamp: " + originalMessageAt.Format(time.RFC3339Nano),
		"New text:\nNeed an edited runtime bridge summary",
	} {
		if !strings.Contains(editDiagnostic.Prompt, needle) {
			t.Fatalf("edit prompt = %q, want %q", editDiagnostic.Prompt, needle)
		}
	}

	replyDiagnostic := findBridgePromptDiagnostic(promptDiagnostics, "message", "322")
	if replyDiagnostic == nil {
		t.Fatalf("prompt diagnostics = %#v, want reply message/322 metadata", promptDiagnostics)
	}
	for _, needle := range []string{
		"Reply context:",
		`Author: "Alice Example" (id="888")`,
		"Quoted text (untrusted historical context):\n> Need an edited runtime bridge summary",
		"Provide a follow-up runtime bridge summary",
	} {
		if !strings.Contains(replyDiagnostic.Prompt, needle) {
			t.Fatalf("reply prompt = %q, want %q", replyDiagnostic.Prompt, needle)
		}
	}

	report := extensiontest.ReportFromMarkers(t, markers)
	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "telegram-reference",
		Platform:                  "teams",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          bridgeID,
			ExtensionName:       "telegram-reference",
			BoundSecretNames:    []string{"bot_token"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	if report.Ownership == nil {
		t.Fatal("report.Ownership = nil, want provider ownership markers")
	}
	if got, want := len(report.Ownership.Listed), 1; got != want {
		t.Fatalf("len(report.Ownership.Listed) = %d, want %d", got, want)
	}
	if got, want := report.Ownership.Listed[0].ID, bridgeID; got != want {
		t.Fatalf("report.Ownership.Listed[0].ID = %q, want %q", got, want)
	}
	if got, want := len(report.Ownership.Fetched), 1; got != want {
		t.Fatalf("len(report.Ownership.Fetched) = %d, want %d", got, want)
	}
	if got, want := report.Ownership.Fetched[0].ID, bridgeID; got != want {
		t.Fatalf("report.Ownership.Fetched[0].ID = %q, want %q", got, want)
	}
	if got, want := len(report.Ingests), 3; got < want {
		t.Fatalf("len(report.Ingests) = %d, want at least %d", got, want)
	}
	if report.Ingests[0].Result.SessionID != route.SessionID ||
		report.Ingests[len(report.Ingests)-1].Result.SessionID != route.SessionID {
		t.Fatalf("ingest session IDs = %#v, want route session %q", report.Ingests, route.SessionID)
	}
	if got, want := uniqueDeliveryCount(report.Deliveries), 3; got < want {
		t.Fatalf("unique delivery IDs = %d, want at least %d", got, want)
	}
	if !hasDeliveryEventType(report.Deliveries, bridgepkg.DeliveryEventTypeStart) {
		t.Fatalf("report.Deliveries = %#v, want start event", report.Deliveries)
	}
	if !hasDeliveryEventType(report.Deliveries, bridgepkg.DeliveryEventTypeFinal) {
		t.Fatalf("report.Deliveries = %#v, want final event", report.Deliveries)
	}
	if report.Deliveries[len(report.Deliveries)-1].Request.Event.BridgeInstanceID != bridgeID {
		t.Fatalf(
			"last delivery bridge_instance_id = %q, want %q",
			report.Deliveries[len(report.Deliveries)-1].Request.Event.BridgeInstanceID,
			bridgeID,
		)
	}
}

func registerBridgeExtensionArtifacts(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	markers extensiontest.MarkerPaths,
	registration acpmock.Registration,
	bridgeID *string,
	sessionID *string,
) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		trimmedBridgeID := strings.TrimSpace(derefStringValue(bridgeID))
		if trimmedBridgeID != "" {
			if err := harness.CaptureBridgeHealth(ctx, trimmedBridgeID); err != nil {
				t.Logf("CaptureBridgeHealth(%q) error = %v", trimmedBridgeID, err)
			}
			if err := harness.CaptureBridgeRoutes(ctx, trimmedBridgeID); err != nil {
				t.Logf("CaptureBridgeRoutes(%q) error = %v", trimmedBridgeID, err)
			}
			if err := harness.CaptureBridgeDeliveryState(ctx, trimmedBridgeID); err != nil {
				t.Logf("CaptureBridgeDeliveryState(%q) error = %v", trimmedBridgeID, err)
			}
			if err := harness.CaptureBridgeSecretBindings(ctx, trimmedBridgeID); err != nil {
				t.Logf("CaptureBridgeSecretBindings(%q) error = %v", trimmedBridgeID, err)
			}
		}

		trimmedSessionID := strings.TrimSpace(derefStringValue(sessionID))
		if trimmedSessionID != "" {
			if err := harness.CaptureSessionTranscript(ctx, trimmedSessionID); err != nil {
				t.Logf("CaptureSessionTranscript(%q) error = %v", trimmedSessionID, err)
			}
			if err := harness.CaptureSessionEvents(ctx, trimmedSessionID); err != nil {
				t.Logf("CaptureSessionEvents(%q) error = %v", trimmedSessionID, err)
			}
			if err := harness.CaptureSessionSandbox(ctx, trimmedSessionID); err != nil {
				t.Logf("CaptureSessionSandbox(%q) error = %v", trimmedSessionID, err)
			}
		}

		payload := map[string]any{
			"provider_markers": extensiontest.ReportFromMarkers(t, markers),
		}
		if records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath); err == nil {
			payload["mock_agents"] = map[string]any{
				registration.AgentName: records,
			}
		} else {
			t.Logf("ReadDiagnostics(%q) error = %v", registration.AgentName, err)
		}
		if err := harness.CaptureProviderCallsJSON(payload); err != nil {
			t.Logf("CaptureProviderCallsJSON() error = %v", err)
		}
	})
}

func daemonBridgeRuntimeRepoRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func allowUnverifiedBridgeExtensionInstalls(cfg *aghconfig.Config) {
	cfg.Extensions.Marketplace.AllowUnverified = true
}

func daemonTelegramReferenceExtensionDir(repoRoot string) string {
	return filepath.Join(repoRoot, "sdk", "examples", "telegram-reference")
}

func prepareDaemonTelegramReferenceExtension(t *testing.T, repoRoot string) string {
	t.Helper()

	sourceDir := daemonTelegramReferenceExtensionDir(repoRoot)
	targetDir := filepath.Join(t.TempDir(), "telegram-reference")
	if err := copyDirectory(sourceDir, targetDir); err != nil {
		t.Fatalf("copyDirectory(%q, %q) error = %v", sourceDir, targetDir, err)
	}
	relaxDaemonTelegramReferenceMinVersion(t, targetDir)
	buildDaemonTelegramReferenceAdapter(t, repoRoot, targetDir)
	return targetDir
}

func relaxDaemonTelegramReferenceMinVersion(t *testing.T, extensionDir string) {
	t.Helper()

	manifestPath := filepath.Join(extensionDir, "extension.toml")
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read telegram reference manifest %q error = %v", manifestPath, err)
	}
	rewriteMinVersion := regexp.MustCompile(`(?m)^(\s*min_agh_version\s*=\s*).*$`)
	updated := rewriteMinVersion.ReplaceAllString(string(contents), `${1}"0.0.0"`)
	if updated == string(contents) {
		t.Fatalf("telegram reference manifest %q did not contain a min_agh_version assignment", manifestPath)
	}
	if err := os.WriteFile(manifestPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write telegram reference manifest %q error = %v", manifestPath, err)
	}
}

func rewriteDaemonBridgeContractPlatform(
	t *testing.T,
	extensionDir string,
	from string,
	to string,
) {
	t.Helper()

	manifestPath := filepath.Join(extensionDir, "extension.toml")
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read bridge contract manifest %q error = %v", manifestPath, err)
	}
	source := `platform = "` + strings.TrimSpace(from) + `"`
	if got, want := strings.Count(string(contents), source), 1; got != want {
		t.Fatalf("bridge contract manifest %q platform assignments = %d, want %d for %q", manifestPath, got, want, source)
	}
	target := `platform = "` + strings.TrimSpace(to) + `"`
	updated := strings.Replace(string(contents), source, target, 1)
	if err := os.WriteFile(manifestPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write bridge contract manifest %q error = %v", manifestPath, err)
	}
}

func buildDaemonTelegramReferenceAdapter(t *testing.T, repoRoot string, extensionDir string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"go",
		"build",
		"-o", filepath.Join(extensionDir, "bin", "telegram-reference"),
		"./sdk/examples/telegram-reference",
	)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"go build telegram-reference from %q into %q: %v\n%s",
			repoRoot,
			extensionDir,
			err,
			string(output),
		)
	}
}

func copyDirectory(sourceDir string, targetDir string) error {
	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("rel %q from %q: %w", path, sourceDir, err)
		}
		targetPath := filepath.Join(targetDir, relativePath)

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat %q: %w", path, err)
		}

		if entry.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}

		bytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %q: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("mkdir %q: %w", filepath.Dir(targetPath), err)
		}
		if err := os.WriteFile(targetPath, bytes, info.Mode().Perm()); err != nil {
			return fmt.Errorf("write %q: %w", targetPath, err)
		}
		return nil
	})
}

func telegramRuntimeInboundUpdate(now time.Time, updateID int64, messageID int64, text string) map[string]any {
	return map[string]any{
		"update_id": updateID,
		"message": map[string]any{
			"message_id":        messageID,
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
			"text": text,
		},
	}
}

func telegramRuntimeEditedInboundUpdate(
	originalAt time.Time,
	editedAt time.Time,
	updateID int64,
	messageID int64,
	text string,
) map[string]any {
	return map[string]any{
		"update_id": updateID,
		"edited_message": map[string]any{
			"message_id":        messageID,
			"message_thread_id": 654,
			"date":              originalAt.Unix(),
			"edit_date":         editedAt.Unix(),
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
			"text": text,
		},
	}
}

func telegramRuntimeReplyInboundUpdate(
	now time.Time,
	updateID int64,
	messageID int64,
	text string,
	parentMessageID int64,
	parentText string,
) map[string]any {
	return map[string]any{
		"update_id": updateID,
		"message": map[string]any{
			"message_id":        messageID,
			"message_thread_id": 654,
			"date":              now.Unix(),
			"chat": map[string]any{
				"id":    777,
				"type":  "supergroup",
				"title": "ops",
			},
			"from": map[string]any{
				"id":         999,
				"username":   "bob",
				"first_name": "Bob",
			},
			"text": text,
			"reply_to_message": map[string]any{
				"message_id": parentMessageID,
				"date":       now.Add(-time.Minute).Unix(),
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
				"text": parentText,
			},
		},
	}
}

func findBridgePromptDiagnostic(
	records []acpmock.DiagnosticsRecord,
	kind string,
	messageID string,
) *acpmock.DiagnosticsRecord {
	for index := range records {
		network := records[index].PromptMeta.Network
		if network == nil {
			continue
		}
		if network.Kind == kind && network.MessageID == messageID {
			return &records[index]
		}
	}
	return nil
}

func countDeliveryEvents(records []extensiontest.DeliveryRecord, want bridgepkg.DeliveryEventType) int {
	total := 0
	for _, record := range records {
		if normalizeBridgeDeliveryEventType(record.Request.Event.EventType) == normalizeBridgeDeliveryEventType(want) {
			total++
		}
	}
	return total
}

func deliveryEvents(
	records []extensiontest.DeliveryRecord,
	want bridgepkg.DeliveryEventType,
) []extensiontest.DeliveryRecord {
	filtered := make([]extensiontest.DeliveryRecord, 0, len(records))
	for _, record := range records {
		if normalizeBridgeDeliveryEventType(record.Request.Event.EventType) ==
			normalizeBridgeDeliveryEventType(want) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func hasDeliveryEventType(records []extensiontest.DeliveryRecord, want bridgepkg.DeliveryEventType) bool {
	return countDeliveryEvents(records, want) > 0
}

func findDeliveryEvent(
	records []extensiontest.DeliveryRecord,
	want bridgepkg.DeliveryEventType,
) *extensiontest.DeliveryRecord {
	for i := range records {
		if normalizeBridgeDeliveryEventType(records[i].Request.Event.EventType) ==
			normalizeBridgeDeliveryEventType(want) {
			return &records[i]
		}
	}
	return nil
}

func uniqueDeliveryCount(records []extensiontest.DeliveryRecord) int {
	ids := make(map[string]struct{}, len(records))
	for _, record := range records {
		deliveryID := strings.TrimSpace(record.Request.Event.DeliveryID)
		if deliveryID == "" {
			continue
		}
		ids[deliveryID] = struct{}{}
	}
	return len(ids)
}

func normalizeBridgeDeliveryEventType(value bridgepkg.DeliveryEventType) bridgepkg.DeliveryEventType {
	return bridgepkg.DeliveryEventType(strings.ToLower(strings.TrimSpace(string(value))))
}

func derefStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
