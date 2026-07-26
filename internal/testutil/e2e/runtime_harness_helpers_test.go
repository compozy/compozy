package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/network/participation"
	sessionpkg "github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	harnessNetworkThreadID = "thread_builders_main"
	harnessNetworkDirectID = "direct_0123456789abcdef0123456789abcdef"
	harnessNetworkWorkID   = "work_patch_42"
)

func harnessBuildersParticipationSpec() participation.Spec {
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     "ws-1",
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       "builders",
		Source:          participation.SourceExplicitRequest,
	}
}

func TestRuntimeHarnessCaptureHelpersPersistArtifacts(t *testing.T) {
	t.Parallel()

	server := newHarnessTestServer(t)

	homePaths := NewHomePaths(t)
	auditTime := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	auditEntry, err := json.Marshal(store.NetworkAuditEntry{
		ID:        "naud-1",
		SessionID: "sess-1",
		Direction: "sent",
		Kind:      "say",
		Channel:   "builders",
		Surface:   "thread",
		ThreadID:  harnessNetworkThreadID,
		WorkID:    harnessNetworkWorkID,
		PeerFrom:  "coder.sess-1",
		MessageID: "msg-1",
		Size:      42,
		Timestamp: auditTime,
	})
	if err != nil {
		t.Fatalf("json.Marshal(auditEntry) error = %v", err)
	}
	if err := os.WriteFile(homePaths.NetworkAuditFile, append(auditEntry, '\n'), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", homePaths.NetworkAuditFile, err)
	}

	harness := &RuntimeHarness{
		HomePaths:   homePaths,
		Artifacts:   NewArtifactCollector(t),
		HTTPBaseURL: server.URL,
		HTTPClient:  server.Client(),
		UDSBaseURL:  server.URL,
		UDSClient:   server.Client(),
	}

	metaPath := store.SessionMetaFile(filepath.Join(homePaths.SessionsDir, "sess-1"))
	if err := store.WriteSessionMeta(metaPath, store.SessionMeta{
		ID:                   "sess-1",
		AgentName:            "coder",
		WorkspaceID:          "ws-1",
		NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
		State:                "stopped",
		Sandbox: &store.SessionSandboxMeta{
			SandboxID:      "env-1",
			Backend:        "local",
			Profile:        "local-sandbox",
			State:          "stopped",
			RuntimeRootDir: "/workspace",
			RuntimeAdditionalDirs: []string{
				"/workspace/shared",
			},
			ProviderState: json.RawMessage(`{"sandbox":"local"}`),
		},
	}); err != nil {
		t.Fatalf("WriteSessionMeta(%q) error = %v", metaPath, err)
	}

	workspace, err := harness.ResolveWorkspace(testContext(t), "/workspace")
	if err != nil {
		t.Fatalf("ResolveWorkspace() error = %v", err)
	}
	if got, want := workspace.ID, "ws-1"; got != want {
		t.Fatalf("workspace.ID = %q, want %q", got, want)
	}

	gotWorkspace, err := harness.GetWorkspace(testContext(t), "ws-1")
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if got, want := gotWorkspace.RootDir, "/workspace"; got != want {
		t.Fatalf("gotWorkspace.RootDir = %q, want %q", got, want)
	}

	extensions, err := harness.ListExtensions(testContext(t))
	if err != nil {
		t.Fatalf("ListExtensions() error = %v", err)
	}
	if got, want := len(extensions), 1; got != want {
		t.Fatalf("len(extensions) = %d, want %d", got, want)
	}

	installedExtension, err := harness.InstallExtension(testContext(t), aghcontract.InstallExtensionRequest{
		Path:     "/extensions/telegram-reference",
		Checksum: "sha256-demo",
	})
	if err != nil {
		t.Fatalf("InstallExtension() error = %v", err)
	}
	if got, want := installedExtension.Name, "telegram-reference"; got != want {
		t.Fatalf("installedExtension.Name = %q, want %q", got, want)
	}

	gotExtension, err := harness.GetExtension(testContext(t), "telegram-reference")
	if err != nil {
		t.Fatalf("GetExtension() error = %v", err)
	}
	if got, want := gotExtension.State, "registered"; got != want {
		t.Fatalf("gotExtension.State = %q, want %q", got, want)
	}

	enabledExtension, err := harness.EnableExtension(testContext(t), "telegram-reference")
	if err != nil {
		t.Fatalf("EnableExtension() error = %v", err)
	}
	if got, want := enabledExtension.Health, "healthy"; got != want {
		t.Fatalf("enabledExtension.Health = %q, want %q", got, want)
	}

	disabledExtension, err := harness.DisableExtension(testContext(t), "telegram-reference")
	if err != nil {
		t.Fatalf("DisableExtension() error = %v", err)
	}
	if disabledExtension.Enabled {
		t.Fatalf("disabledExtension.Enabled = %v, want false", disabledExtension.Enabled)
	}

	session, err := harness.CreateSession(testContext(t), aghcontract.CreateSessionRequest{
		AgentName:     "coder",
		WorkspacePath: "/workspace",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if got, want := session.ID, "sess-1"; got != want {
		t.Fatalf("session.ID = %q, want %q", got, want)
	}

	gotSession, err := harness.GetSession(testContext(t), "sess-1")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got, want := gotSession.Sandbox.Profile, "local"; got != want {
		t.Fatalf("gotSession.Sandbox.Profile = %q, want %q", got, want)
	}
	resumedSession, err := harness.ResumeSession(testContext(t), "sess-1")
	if err != nil {
		t.Fatalf("ResumeSession() error = %v", err)
	}
	if got, want := resumedSession.State, sessionpkg.StateActive; got != want {
		t.Fatalf("resumedSession.State = %q, want %q", got, want)
	}
	if resumedSession.ResolvedNetworkParticipation == nil ||
		resumedSession.ResolvedNetworkParticipation.ChannelID != "builders" {
		t.Fatalf(
			"resumedSession.ResolvedNetworkParticipation = %#v, want channel builders",
			resumedSession.ResolvedNetworkParticipation,
		)
	}

	stream, err := harness.PromptSession(testContext(t), "sess-1", "hello world")
	if err != nil {
		t.Fatalf("PromptSession() error = %v", err)
	}
	if got, want := len(stream), 2; got != want {
		t.Fatalf("len(stream) = %d, want %d", got, want)
	}
	if got, want := stream[0].Event, "agent_message"; got != want {
		t.Fatalf("stream[0].Event = %q, want %q", got, want)
	}

	if _, err := harness.SessionTranscript(testContext(t), "sess-1"); err != nil {
		t.Fatalf("SessionTranscript() error = %v", err)
	}
	if _, err := harness.SessionEvents(testContext(t), "sess-1"); err != nil {
		t.Fatalf("SessionEvents() error = %v", err)
	}
	if _, err := harness.NetworkStatus(testContext(t)); err != nil {
		t.Fatalf("NetworkStatus() error = %v", err)
	}
	if _, err := harness.NetworkPeers(testContext(t), "builders"); err != nil {
		t.Fatalf("NetworkPeers() error = %v", err)
	}
	if _, err := harness.NetworkChannels(testContext(t)); err != nil {
		t.Fatalf("NetworkChannels() error = %v", err)
	}
	if _, err := harness.NetworkChannel(testContext(t), "builders"); err != nil {
		t.Fatalf("NetworkChannel() error = %v", err)
	}
	if _, err := harness.NetworkChannelMessages(testContext(t), "builders"); err != nil {
		t.Fatalf("NetworkChannelMessages() error = %v", err)
	}
	threads, err := harness.NetworkThreads(testContext(t), "builders")
	if err != nil {
		t.Fatalf("NetworkThreads() error = %v", err)
	}
	if got, want := threads[0].ThreadID, harnessNetworkThreadID; got != want {
		t.Fatalf("NetworkThreads()[0].ThreadID = %q, want %q", got, want)
	}
	thread, err := harness.NetworkThread(testContext(t), "builders", harnessNetworkThreadID)
	if err != nil {
		t.Fatalf("NetworkThread() error = %v", err)
	}
	if got, want := thread.ThreadID, harnessNetworkThreadID; got != want {
		t.Fatalf("NetworkThread().ThreadID = %q, want %q", got, want)
	}
	threadMessages, err := harness.NetworkThreadMessages(testContext(t), "builders", harnessNetworkThreadID)
	if err != nil {
		t.Fatalf("NetworkThreadMessages() error = %v", err)
	}
	if got, want := threadMessages[0].Surface, "thread"; got != want {
		t.Fatalf("NetworkThreadMessages()[0].Surface = %q, want %q", got, want)
	}
	directs, err := harness.NetworkDirectRooms(testContext(t), "builders")
	if err != nil {
		t.Fatalf("NetworkDirectRooms() error = %v", err)
	}
	if got, want := directs[0].DirectID, harnessNetworkDirectID; got != want {
		t.Fatalf("NetworkDirectRooms()[0].DirectID = %q, want %q", got, want)
	}
	resolvedDirect, err := harness.NetworkDirectResolve(
		testContext(t),
		"builders",
		aghcontract.NetworkDirectResolveRequest{
			SessionID: "sess-1",
			PeerID:    "reviewer.sess-2",
		},
	)
	if err != nil {
		t.Fatalf("NetworkDirectResolve() error = %v", err)
	}
	if got, want := resolvedDirect.DirectID, harnessNetworkDirectID; got != want {
		t.Fatalf("NetworkDirectResolve().DirectID = %q, want %q", got, want)
	}
	direct, err := harness.NetworkDirectRoom(testContext(t), "builders", harnessNetworkDirectID)
	if err != nil {
		t.Fatalf("NetworkDirectRoom() error = %v", err)
	}
	if got, want := direct.DirectID, harnessNetworkDirectID; got != want {
		t.Fatalf("NetworkDirectRoom().DirectID = %q, want %q", got, want)
	}
	directMessages, err := harness.NetworkDirectRoomMessages(testContext(t), "builders", harnessNetworkDirectID)
	if err != nil {
		t.Fatalf("NetworkDirectRoomMessages() error = %v", err)
	}
	if got, want := directMessages[0].Surface, "direct"; got != want {
		t.Fatalf("NetworkDirectRoomMessages()[0].Surface = %q, want %q", got, want)
	}
	work, err := harness.NetworkWork(testContext(t), harnessNetworkWorkID)
	if err != nil {
		t.Fatalf("NetworkWork() error = %v", err)
	}
	if got, want := work.WorkID, harnessNetworkWorkID; got != want {
		t.Fatalf("NetworkWork().WorkID = %q, want %q", got, want)
	}
	if _, err := harness.NetworkInbox(testContext(t), "sess-1"); err != nil {
		t.Fatalf("NetworkInbox() error = %v", err)
	}
	if _, err := harness.NetworkSend(testContext(t), aghcontract.NetworkSendRequest{
		SessionID:   "sess-1",
		Channel:     "builders",
		Surface:     "thread",
		ThreadID:    harnessNetworkThreadID,
		Kind:        "say",
		WorkID:      harnessNetworkWorkID,
		ReplyTo:     "msg-1",
		TraceID:     "trace_ops_patch_42",
		CausationID: "msg-1",
		Body:        json.RawMessage(`{"text":"hello"}`),
	}); err != nil {
		t.Fatalf("NetworkSend() error = %v", err)
	}
	if _, err := harness.CreateNetworkChannel(testContext(t), aghcontract.CreateNetworkChannelRequest{
		Channel:     "builders",
		WorkspaceID: "ws-1",
		AgentNames:  []string{"coder"},
	}); err != nil {
		t.Fatalf("CreateNetworkChannel() error = %v", err)
	}
	if err := harness.StopSession(testContext(t), "sess-1"); err != nil {
		t.Fatalf("StopSession() error = %v", err)
	}
	if err := harness.CaptureNetworkArtifacts(testContext(t), "builders"); err != nil {
		t.Fatalf("CaptureNetworkArtifacts() error = %v", err)
	}

	createdBridge, err := harness.CreateBridge(testContext(t), aghcontract.CreateBridgeRequest{
		Scope:         bridgepkg.ScopeWorkspace,
		WorkspaceID:   "ws-1",
		Platform:      "telegram",
		ExtensionName: "telegram-reference",
		DisplayName:   "Telegram Runtime",
		Enabled:       false,
	})
	if err != nil {
		t.Fatalf("CreateBridge() error = %v", err)
	}
	if got, want := createdBridge.Bridge.ID, "brg-1"; got != want {
		t.Fatalf("createdBridge.Bridge.ID = %q, want %q", got, want)
	}

	gotBridge, err := harness.GetBridge(testContext(t), "brg-1")
	if err != nil {
		t.Fatalf("GetBridge() error = %v", err)
	}
	if got, want := gotBridge.Health.RouteCount, 1; got != want {
		t.Fatalf("gotBridge.Health.RouteCount = %d, want %d", got, want)
	}

	enabledBridge, err := harness.EnableBridge(testContext(t), "brg-1")
	if err != nil {
		t.Fatalf("EnableBridge() error = %v", err)
	}
	if !enabledBridge.Bridge.Enabled {
		t.Fatal("enabledBridge.Bridge.Enabled = false, want true")
	}

	restartedBridge, err := harness.RestartBridge(testContext(t), "brg-1")
	if err != nil {
		t.Fatalf("RestartBridge() error = %v", err)
	}
	if got, want := restartedBridge.Health.DeliveryBacklog, 0; got != want {
		t.Fatalf("restartedBridge.Health.DeliveryBacklog = %d, want %d", got, want)
	}

	routes, err := harness.ListBridgeRoutes(testContext(t), "brg-1")
	if err != nil {
		t.Fatalf("ListBridgeRoutes() error = %v", err)
	}
	if got, want := len(routes), 1; got != want {
		t.Fatalf("len(routes) = %d, want %d", got, want)
	}

	binding, err := harness.PutBridgeSecretBinding(
		testContext(t),
		"brg-1",
		"bot_token",
		aghcontract.PutBridgeSecretBindingRequest{
			SecretRef:   "vault:bridges/brg-1/bot_token",
			SecretValue: new("telegram-bot-token"),
			Kind:        "token",
		},
	)
	if err != nil {
		t.Fatalf("PutBridgeSecretBinding() error = %v", err)
	}
	if got, want := binding.BindingName, "bot_token"; got != want {
		t.Fatalf("binding.BindingName = %q, want %q", got, want)
	}

	bindings, err := harness.ListBridgeSecretBindings(testContext(t), "brg-1")
	if err != nil {
		t.Fatalf("ListBridgeSecretBindings() error = %v", err)
	}
	if got, want := len(bindings), 1; got != want {
		t.Fatalf("len(bindings) = %d, want %d", got, want)
	}

	if err := harness.CaptureSessionTranscript(testContext(t), "sess-1"); err != nil {
		t.Fatalf("CaptureSessionTranscript() error = %v", err)
	}
	if err := harness.CaptureSessionEvents(testContext(t), "sess-1"); err != nil {
		t.Fatalf("CaptureSessionEvents() error = %v", err)
	}
	if err := harness.CaptureSessionSandbox(testContext(t), "sess-1"); err != nil {
		t.Fatalf("CaptureSessionSandbox() error = %v", err)
	}
	if err := harness.CaptureNetworkArtifacts(testContext(t), "builders"); err != nil {
		t.Fatalf("CaptureNetworkArtifacts() error = %v", err)
	}
	if err := harness.CaptureAutomationRuns(testContext(t), url.Values{"status": {"completed"}}); err != nil {
		t.Fatalf("CaptureAutomationRuns() error = %v", err)
	}
	if err := harness.CaptureTasks(testContext(t), url.Values{"limit": {"10"}}); err != nil {
		t.Fatalf("CaptureTasks() error = %v", err)
	}
	if err := harness.CaptureTaskRuns(testContext(t), "task-1", url.Values{"status": {"completed"}}); err != nil {
		t.Fatalf("CaptureTaskRuns() error = %v", err)
	}
	if err := harness.CaptureBridgeHealth(testContext(t), "brg-1"); err != nil {
		t.Fatalf("CaptureBridgeHealth() error = %v", err)
	}
	if err := harness.CaptureBridgeRoutes(testContext(t), "brg-1"); err != nil {
		t.Fatalf("CaptureBridgeRoutes() error = %v", err)
	}
	if err := harness.CaptureBridgeDeliveryState(testContext(t), "brg-1"); err != nil {
		t.Fatalf("CaptureBridgeDeliveryState() error = %v", err)
	}
	if err := harness.CaptureBridgeSecretBindings(testContext(t), "brg-1"); err != nil {
		t.Fatalf("CaptureBridgeSecretBindings() error = %v", err)
	}
	if err := harness.CaptureToolHostDiagnosticsJSON(ToolHostDiagnosticsArtifact{
		SessionID: "sess-1",
		Operations: []ToolHostOperationDiagnostic{{
			Operation:        "write_text_file",
			Path:             "toolhost/output.txt",
			Outcome:          ToolHostOutcomeAllowed,
			SideEffectPath:   "/workspace/toolhost/output.txt",
			SideEffectExists: true,
		}},
	}); err != nil {
		t.Fatalf("CaptureToolHostDiagnosticsJSON() error = %v", err)
	}
	if err := harness.CaptureCombinedFlowJSON(CombinedFlowArtifact{
		Scenario:          "bridge-sandbox-network",
		SessionID:         "sess-1",
		Channel:           "builders",
		AutomationRunID:   "run-1",
		TaskID:            "task-1",
		TaskRunID:         "task-run-1",
		BridgeID:          "brg-1",
		NetworkMessageIDs: []string{"msg-1"},
		SideEffectPaths:   []string{"/workspace/toolhost/output.txt"},
	}); err != nil {
		t.Fatalf("CaptureCombinedFlowJSON() error = %v", err)
	}

	providerLog := filepath.Join(t.TempDir(), "provider.log")
	if err := os.WriteFile(providerLog, []byte("provider call"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", providerLog, err)
	}
	if err := harness.CaptureProviderCallsFile(providerLog, "text/plain"); err != nil {
		t.Fatalf("CaptureProviderCallsFile() error = %v", err)
	}

	tracePath := filepath.Join(t.TempDir(), "trace.zip")
	if err := os.WriteFile(tracePath, []byte("trace-bytes"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", tracePath, err)
	}
	if err := harness.CaptureBrowserTraceFile(tracePath); err != nil {
		t.Fatalf("CaptureBrowserTraceFile() error = %v", err)
	}

	screenshotOne := filepath.Join(t.TempDir(), "screen-1.png")
	screenshotTwo := filepath.Join(t.TempDir(), "screen-2.png")
	for _, item := range []struct {
		path string
		data string
	}{
		{path: screenshotOne, data: "one"},
		{path: screenshotTwo, data: "two"},
	} {
		if err := os.WriteFile(item.path, []byte(item.data), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", item.path, err)
		}
	}
	if err := harness.CaptureBrowserScreenshots([]string{screenshotOne, screenshotTwo}); err != nil {
		t.Fatalf("CaptureBrowserScreenshots() error = %v", err)
	}
	if err := harness.CaptureBrowserConsoleJSON([]map[string]string{{"level": "error"}}); err != nil {
		t.Fatalf("CaptureBrowserConsoleJSON() error = %v", err)
	}
	if err := harness.CaptureBrowserNetworkJSON([]map[string]string{{"url": "/api/demo"}}); err != nil {
		t.Fatalf("CaptureBrowserNetworkJSON() error = %v", err)
	}
	if err := harness.StopSession(testContext(t), "sess-1"); err != nil {
		t.Fatalf("StopSession() error = %v", err)
	}

	manifest := harness.Artifacts.Manifest()
	if got, wantMin := len(manifest.Artifacts), 22; got < wantMin {
		t.Fatalf("len(manifest.Artifacts) = %d, want at least %d", got, wantMin)
	}

	manifestPath := harness.Artifacts.ManifestPath()
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", manifestPath, err)
	}
	if !strings.Contains(string(manifestBytes), "bridge_health.json") {
		t.Fatalf("manifest = %s, want bridge_health.json entry", string(manifestBytes))
	}
	if !strings.Contains(string(manifestBytes), "bridge_routes.json") {
		t.Fatalf("manifest = %s, want bridge_routes.json entry", string(manifestBytes))
	}
	if !strings.Contains(string(manifestBytes), "bridge_delivery_state.json") {
		t.Fatalf("manifest = %s, want bridge_delivery_state.json entry", string(manifestBytes))
	}
	if !strings.Contains(string(manifestBytes), "bridge_secret_bindings.json") {
		t.Fatalf("manifest = %s, want bridge_secret_bindings.json entry", string(manifestBytes))
	}
	for _, want := range []string{
		"network_threads.json",
		"network_direct_rooms.json",
		"network_work.json",
	} {
		if !strings.Contains(string(manifestBytes), want) {
			t.Fatalf("manifest = %s, want %s entry", string(manifestBytes), want)
		}
	}

	networkAuditPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindNetworkAudit)
	if !ok {
		t.Fatal("ArtifactPath(network_audit) = missing, want present")
	}
	networkAuditBytes, err := os.ReadFile(networkAuditPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", networkAuditPath, err)
	}
	if strings.Contains(string(networkAuditBytes), "\n{\"id\":") {
		t.Fatalf(
			"network audit artifact = %s, want stable JSON snapshot instead of raw ndjson",
			string(networkAuditBytes),
		)
	}
	if !strings.Contains(string(networkAuditBytes), "\"Direction\": \"sent\"") {
		t.Fatalf("network audit artifact = %s, want decoded audit entry", string(networkAuditBytes))
	}
	if !strings.Contains(string(networkAuditBytes), "\"Surface\": \"thread\"") ||
		!strings.Contains(string(networkAuditBytes), "\"ThreadID\": \""+harnessNetworkThreadID+"\"") ||
		!strings.Contains(string(networkAuditBytes), "\"WorkID\": \""+harnessNetworkWorkID+"\"") {
		t.Fatalf("network audit artifact = %s, want final conversation metadata", string(networkAuditBytes))
	}

	networkThreadsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindNetworkThreads)
	if !ok {
		t.Fatal("ArtifactPath(network_threads) = missing, want present")
	}
	networkThreadsBytes, err := os.ReadFile(networkThreadsPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", networkThreadsPath, err)
	}
	if !strings.Contains(string(networkThreadsBytes), `"thread_id": "`+harnessNetworkThreadID+`"`) {
		t.Fatalf("network threads artifact = %s, want thread_id", string(networkThreadsBytes))
	}

	networkDirectRoomsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindNetworkDirectRooms)
	if !ok {
		t.Fatal("ArtifactPath(network_direct_rooms) = missing, want present")
	}
	networkDirectRoomsBytes, err := os.ReadFile(networkDirectRoomsPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", networkDirectRoomsPath, err)
	}
	if !strings.Contains(string(networkDirectRoomsBytes), `"direct_id": "`+harnessNetworkDirectID+`"`) {
		t.Fatalf("network direct rooms artifact = %s, want direct_id", string(networkDirectRoomsBytes))
	}

	networkWorkPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindNetworkWork)
	if !ok {
		t.Fatal("ArtifactPath(network_work) = missing, want present")
	}
	networkWorkBytes, err := os.ReadFile(networkWorkPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", networkWorkPath, err)
	}
	if !strings.Contains(string(networkWorkBytes), `"work_id": "`+harnessNetworkWorkID+`"`) {
		t.Fatalf("network work artifact = %s, want work_id", string(networkWorkBytes))
	}

	automationRunsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindAutomationRuns)
	if !ok {
		t.Fatal("ArtifactPath(automation_runs) = missing, want present")
	}
	automationRunsBytes, err := os.ReadFile(automationRunsPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", automationRunsPath, err)
	}
	if !strings.Contains(string(automationRunsBytes), `"session_id": "sess-1"`) {
		t.Fatalf("automation runs artifact = %s, want linked session id", string(automationRunsBytes))
	}

	tasksPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindTasks)
	if !ok {
		t.Fatal("ArtifactPath(tasks) = missing, want present")
	}
	tasksBytes, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", tasksPath, err)
	}
	if !strings.Contains(string(tasksBytes), `"ref": "run:run-1"`) {
		t.Fatalf("tasks artifact = %s, want automation origin linkage", string(tasksBytes))
	}

	taskRunsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindTaskRuns)
	if !ok {
		t.Fatal("ArtifactPath(task_runs) = missing, want present")
	}
	taskRunsBytes, err := os.ReadFile(taskRunsPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", taskRunsPath, err)
	}
	if !strings.Contains(string(taskRunsBytes), `"session_id": "sess-1"`) ||
		!strings.Contains(string(taskRunsBytes), `"idempotency_key": "automation-run:run-1"`) {
		t.Fatalf("task runs artifact = %s, want session linkage and idempotency key", string(taskRunsBytes))
	}

	sessionSandboxPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindSessionSandbox)
	if !ok {
		t.Fatal("ArtifactPath(session_sandbox) = missing, want present")
	}
	sessionSandboxBytes, err := os.ReadFile(sessionSandboxPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", sessionSandboxPath, err)
	}
	if !strings.Contains(string(sessionSandboxBytes), `"runtime_root_dir": "/workspace"`) {
		t.Fatalf(
			"session sandbox artifact = %s, want persisted runtime root metadata",
			string(sessionSandboxBytes),
		)
	}
	if !strings.Contains(string(sessionSandboxBytes), `"session_state": "stopped"`) {
		t.Fatalf(
			"session sandbox artifact = %s, want session-level stop visibility",
			string(sessionSandboxBytes),
		)
	}

	bridgeRoutesPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindBridgeRoutes)
	if !ok {
		t.Fatal("ArtifactPath(bridge_routes) = missing, want present")
	}
	bridgeRoutesBytes, err := os.ReadFile(bridgeRoutesPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", bridgeRoutesPath, err)
	}
	if !strings.Contains(string(bridgeRoutesBytes), `"session_id": "sess-1"`) {
		t.Fatalf("bridge routes artifact = %s, want route session linkage", string(bridgeRoutesBytes))
	}

	bridgeStatePath, ok := harness.Artifacts.ArtifactPath(ArtifactKindBridgeDeliveryState)
	if !ok {
		t.Fatal("ArtifactPath(bridge_delivery_state) = missing, want present")
	}
	bridgeStateBytes, err := os.ReadFile(bridgeStatePath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", bridgeStatePath, err)
	}
	if !strings.Contains(string(bridgeStateBytes), `"delivery_backlog": 0`) {
		t.Fatalf("bridge delivery state artifact = %s, want delivery health snapshot", string(bridgeStateBytes))
	}

	bridgeBindingsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindBridgeSecretBindings)
	if !ok {
		t.Fatal("ArtifactPath(bridge_secret_bindings) = missing, want present")
	}
	bridgeBindingsBytes, err := os.ReadFile(bridgeBindingsPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", bridgeBindingsPath, err)
	}
	if !strings.Contains(string(bridgeBindingsBytes), `"secret_ref": "vault:bridges/brg-1/bot_token"`) {
		t.Fatalf("bridge secret bindings artifact = %s, want stable binding snapshot", string(bridgeBindingsBytes))
	}

	combinedFlowPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindCombinedFlow)
	if !ok {
		t.Fatal("ArtifactPath(combined_flow) = missing, want present")
	}
	combinedFlowBytes, err := os.ReadFile(combinedFlowPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", combinedFlowPath, err)
	}
	if !strings.Contains(string(combinedFlowBytes), `"scenario": "bridge-sandbox-network"`) ||
		!strings.Contains(string(combinedFlowBytes), `"bridge_id": "brg-1"`) {
		t.Fatalf("combined flow artifact = %s, want cross-domain scenario summary", string(combinedFlowBytes))
	}

	toolHostDiagnosticsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindToolHostDiagnostics)
	if !ok {
		t.Fatal("ArtifactPath(tool_host_diagnostics) = missing, want present")
	}
	toolHostDiagnosticsBytes, err := os.ReadFile(toolHostDiagnosticsPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", toolHostDiagnosticsPath, err)
	}
	if !strings.Contains(string(toolHostDiagnosticsBytes), `"operation": "write_text_file"`) {
		t.Fatalf(
			"tool host diagnostics artifact = %s, want tool-host operation diagnostics",
			string(toolHostDiagnosticsBytes),
		)
	}

	providerCallsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindProviderCalls)
	if !ok {
		t.Fatal("ArtifactPath(provider_calls) = missing, want present")
	}
	providerCallsBytes, err := os.ReadFile(providerCallsPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", providerCallsPath, err)
	}
	if !strings.Contains(string(providerCallsBytes), "provider call") {
		t.Fatalf("provider calls artifact = %s, want provider log capture", string(providerCallsBytes))
	}

	bridgeHealthPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindBridgeHealth)
	if !ok {
		t.Fatal("ArtifactPath(bridge_health) = missing, want present")
	}
	if _, err := os.Stat(bridgeHealthPath); err != nil {
		t.Fatalf("os.Stat(%q) error = %v", bridgeHealthPath, err)
	}
}

func TestRuntimeHarnessBridgeAndExtensionHelpersSurfaceTransportErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should surface bridge and extension transport errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/bridges/health/stream":
				if got := r.URL.Query().Get("bridge_ids"); got != "brg-1" {
					http.Error(w, "bridge_ids is required", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				if _, err := fmt.Fprint(w, "event: bridge_health\ndata: not-json\n\n"); err != nil {
					t.Errorf("fmt.Fprint(bridge health stream) error = %v", err)
				}
			default:
				http.Error(w, "boom", http.StatusInternalServerError)
			}
		}))
		defer server.Close()

		harness := &RuntimeHarness{
			Artifacts:   NewArtifactCollector(t),
			HTTPBaseURL: server.URL,
			HTTPClient:  server.Client(),
			UDSBaseURL:  server.URL,
			UDSClient:   server.Client(),
		}

		ctx := testContext(t)

		_, err := harness.GetExtension(ctx, "telegram-reference")
		assertErrorContains(t, err, "/api/extensions/telegram-reference status 500: boom")
		if _, err := harness.CreateBridge(ctx, aghcontract.CreateBridgeRequest{
			Scope:         bridgepkg.ScopeWorkspace,
			WorkspaceID:   "ws-1",
			Platform:      "telegram",
			ExtensionName: "telegram-reference",
			DisplayName:   "Telegram Runtime",
			Enabled:       false,
		}); err != nil {
			assertErrorContains(t, err, "/api/bridges status 500: boom")
		} else {
			t.Fatal("CreateBridge() error = nil, want transport error")
		}
		if _, err := harness.PutBridgeSecretBinding(
			ctx,
			"brg-1",
			"bot_token",
			aghcontract.PutBridgeSecretBindingRequest{
				SecretRef:   "vault:bridges/brg-1/bot_token",
				SecretValue: new("telegram-bot-token"),
				Kind:        "token",
			},
		); err != nil {
			assertErrorContains(t, err, "/api/bridges/brg-1/secret-bindings/bot_token status 500: boom")
		} else {
			t.Fatal("PutBridgeSecretBinding() error = nil, want transport error")
		}
		assertErrorContains(t, harness.CaptureBridgeRoutes(ctx, "brg-1"), "/api/bridges/brg-1/routes status 500: boom")
		assertErrorContains(
			t,
			harness.CaptureBridgeSecretBindings(ctx, "brg-1"),
			"/api/bridges/brg-1/secret-bindings status 500: boom",
		)
		assertErrorContains(t, harness.CaptureBridgeHealth(ctx, "brg-1"), "decode bridge health snapshot")
	})
}

func TestRuntimeHarnessBridgeHelpersRejectBlankBridgeID(t *testing.T) {
	t.Parallel()

	harness := &RuntimeHarness{}
	ctx := testContext(t)

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "get bridge",
			call: func() error {
				_, err := harness.GetBridge(ctx, "   ")
				return err
			},
		},
		{
			name: "enable bridge",
			call: func() error {
				_, err := harness.EnableBridge(ctx, "")
				return err
			},
		},
		{
			name: "restart bridge",
			call: func() error {
				_, err := harness.RestartBridge(ctx, "")
				return err
			},
		},
		{
			name: "list routes",
			call: func() error {
				_, err := harness.ListBridgeRoutes(ctx, "")
				return err
			},
		},
		{
			name: "put secret binding",
			call: func() error {
				_, err := harness.PutBridgeSecretBinding(
					ctx,
					"",
					"bot_token",
					aghcontract.PutBridgeSecretBindingRequest{},
				)
				return err
			},
		},
		{
			name: "list secret bindings",
			call: func() error {
				_, err := harness.ListBridgeSecretBindings(ctx, "")
				return err
			},
		},
		{
			name: "capture routes",
			call: func() error {
				return harness.CaptureBridgeRoutes(ctx, "")
			},
		},
		{
			name: "capture delivery state",
			call: func() error {
				return harness.CaptureBridgeDeliveryState(ctx, "")
			},
		},
		{
			name: "capture secret bindings",
			call: func() error {
				return harness.CaptureBridgeSecretBindings(ctx, "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertErrorContains(t, tt.call(), "bridge id is required")
		})
	}
}

type harnessTestServer struct {
	*httptest.Server
	handlerErrs chan error
}

func newHarnessTestServer(t testing.TB) *harnessTestServer {
	t.Helper()

	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	routeTime := now.Add(5 * time.Minute)
	handlerErrs := make(chan error, 32)
	mux := http.NewServeMux()

	mux.HandleFunc("/api/workspaces/resolve", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.WorkspaceResponse{
			Workspace: aghcontract.WorkspacePayload{
				ID:        "ws-1",
				RootDir:   "/workspace",
				Name:      "workspace",
				CreatedAt: now,
				UpdatedAt: now,
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.WorkspaceResponse{
			Workspace: aghcontract.WorkspacePayload{
				ID:        "ws-1",
				RootDir:   "/workspace",
				Name:      "workspace",
				CreatedAt: now,
				UpdatedAt: now,
			},
		})
	})
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, aghcontract.SessionResponse{
			Session: aghcontract.SessionPayload{
				ID:            "sess-1",
				AgentName:     "coder",
				WorkspaceID:   "ws-1",
				WorkspacePath: "/workspace",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, aghcontract.SessionResponse{
			Session: aghcontract.SessionPayload{
				ID:            "sess-1",
				AgentName:     "coder",
				WorkspaceID:   "ws-1",
				WorkspacePath: "/workspace",
				State:         "stopped",
				StopReason:    store.StopCompleted,
				Sandbox: &aghcontract.SessionSandboxPayload{
					SandboxID: "env-1",
					Backend:   "local",
					Profile:   "local",
					State:     "ready",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1/attach", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, aghcontract.SessionAttachResponse{
			Session: aghcontract.SessionPayload{
				ID:                           "sess-1",
				AgentName:                    "coder",
				WorkspaceID:                  "ws-1",
				WorkspacePath:                "/workspace",
				ResolvedNetworkParticipation: participation.CloneSpec(harnessBuildersParticipationSpec()),
				State:                        "active",
				Badge:                        "idle",
				Sandbox: &aghcontract.SessionSandboxPayload{
					SandboxID: "env-1",
					Backend:   "local",
					Profile:   "local",
					State:     "ready",
				},
				CreatedAt: now,
				UpdatedAt: routeTime,
			},
			Attach: aghcontract.SessionAttachPayload{
				SessionID:       "sess-1",
				AttachedTo:      "e2e:test",
				AttachExpiresAt: routeTime.Add(time.Minute),
				AttachedAt:      routeTime,
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1/transcript", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"messages": []map[string]any{
				{"role": "user", "content": "hello world"},
				{"role": "assistant", "content": "echo: hello world"},
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1/events", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"events": []map[string]any{
				{"id": "evt-1", "type": "agent_message"},
				{"id": "evt-2", "type": "session_stopped"},
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1/prompt", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := fmt.Fprint(
			w,
			"event: agent_message\n"+
				"data: {\"type\":\"text-delta\"}\n\n"+
				"event: done\n"+
				"data: [DONE]\n\n",
		); err != nil {
			reportHarnessHandlerError(w, handlerErrs, http.StatusInternalServerError, "write prompt stream: %v", err)
		}
	})
	mux.HandleFunc(
		"/api/workspaces/ws-1/network/channels/builders/threads",
		func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, aghcontract.NetworkThreadsResponse{
				Threads: []aghcontract.NetworkThreadSummaryPayload{{
					Channel:            "builders",
					ThreadID:           harnessNetworkThreadID,
					RootMessageID:      "msg-1",
					Title:              "Migration test repair",
					OpenedByPeerID:     "coder.sess-1",
					OpenedSessionID:    "sess-1",
					OpenedAt:           &now,
					LastActivityAt:     &now,
					MessageCount:       1,
					ParticipantCount:   1,
					OpenWorkCount:      1,
					LastMessagePreview: "hello",
				}},
			})
		},
	)
	mux.HandleFunc("/api/workspaces/ws-1/network/channels/builders/threads/"+harnessNetworkThreadID, func(
		w http.ResponseWriter,
		_ *http.Request,
	) {
		writeJSON(w, aghcontract.NetworkThreadResponse{
			Thread: aghcontract.NetworkThreadSummaryPayload{
				Channel:            "builders",
				ThreadID:           harnessNetworkThreadID,
				RootMessageID:      "msg-1",
				Title:              "Migration test repair",
				OpenedByPeerID:     "coder.sess-1",
				OpenedSessionID:    "sess-1",
				OpenedAt:           &now,
				LastActivityAt:     &now,
				MessageCount:       1,
				ParticipantCount:   1,
				OpenWorkCount:      1,
				LastMessagePreview: "hello",
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/network/channels/builders/threads/"+harnessNetworkThreadID+"/messages", func(
		w http.ResponseWriter,
		_ *http.Request,
	) {
		writeJSON(w, aghcontract.NetworkThreadMessagesResponse{
			Messages: []aghcontract.NetworkConversationMessagePayload{{
				MessageID:   "msg-1",
				Channel:     "builders",
				Surface:     "thread",
				ThreadID:    harnessNetworkThreadID,
				Kind:        "say",
				Direction:   "sent",
				PeerFrom:    "peer-1",
				WorkID:      harnessNetworkWorkID,
				TraceID:     "trace_ops_patch_42",
				Text:        "hello",
				PreviewText: "hello",
				Body:        json.RawMessage(`{"text":"hello"}`),
				Timestamp:   now,
			}},
		})
	})
	mux.HandleFunc(
		"/api/workspaces/ws-1/network/channels/builders/directs",
		func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, aghcontract.NetworkDirectRoomsResponse{
				Directs: []aghcontract.NetworkDirectRoomPayload{{
					Channel:            "builders",
					DirectID:           harnessNetworkDirectID,
					SessionA:           "sess-1",
					SessionB:           "sess-2",
					OpenedAt:           &now,
					LastActivityAt:     &now,
					MessageCount:       1,
					OpenWorkCount:      1,
					LastMessagePreview: "direct hello",
				}},
			})
		},
	)
	mux.HandleFunc("/api/workspaces/ws-1/network/channels/builders/directs/resolve", func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request aghcontract.NetworkDirectResolveRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			reportHarnessHandlerError(w, handlerErrs, http.StatusBadRequest, "decode direct resolve: %v", err)
			return
		}
		if got, want := request.SessionID, "sess-1"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"direct resolve session_id = %q, want %q",
				got,
				want,
			)
			return
		}
		if got, want := request.PeerID, "reviewer.sess-2"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"direct resolve peer_id = %q, want %q",
				got,
				want,
			)
			return
		}
		writeJSON(w, aghcontract.NetworkDirectRoomResponse{
			Direct: aghcontract.NetworkDirectRoomPayload{
				Channel:        "builders",
				DirectID:       harnessNetworkDirectID,
				SessionA:       "sess-1",
				SessionB:       "sess-2",
				OpenedAt:       &now,
				LastActivityAt: &now,
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/network/channels/builders/directs/"+harnessNetworkDirectID, func(
		w http.ResponseWriter,
		_ *http.Request,
	) {
		writeJSON(w, aghcontract.NetworkDirectRoomResponse{
			Direct: aghcontract.NetworkDirectRoomPayload{
				Channel:            "builders",
				DirectID:           harnessNetworkDirectID,
				SessionA:           "sess-1",
				SessionB:           "sess-2",
				OpenedAt:           &now,
				LastActivityAt:     &now,
				MessageCount:       1,
				OpenWorkCount:      1,
				LastMessagePreview: "direct hello",
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/network/channels/builders/directs/"+harnessNetworkDirectID+"/messages", func(
		w http.ResponseWriter,
		_ *http.Request,
	) {
		writeJSON(w, aghcontract.NetworkDirectRoomMessagesResponse{
			Messages: []aghcontract.NetworkConversationMessagePayload{{
				MessageID:   "msg-direct-1",
				Channel:     "builders",
				Surface:     "direct",
				DirectID:    harnessNetworkDirectID,
				Kind:        "say",
				Direction:   "sent",
				PeerFrom:    "coder.sess-1",
				PeerTo:      "reviewer.sess-2",
				WorkID:      harnessNetworkWorkID,
				ReplyTo:     "msg-1",
				TraceID:     "trace_ops_patch_42",
				CausationID: "msg-1",
				Text:        "direct hello",
				PreviewText: "direct hello",
				Body:        json.RawMessage(`{"text":"direct hello"}`),
				Timestamp:   now,
			}},
		})
	})
	mux.HandleFunc(
		"/api/workspaces/ws-1/network/work/"+harnessNetworkWorkID,
		func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, aghcontract.NetworkWorkResponse{
				Work: aghcontract.NetworkWorkPayload{
					WorkID:          harnessNetworkWorkID,
					Channel:         "builders",
					Surface:         "thread",
					ThreadID:        harnessNetworkThreadID,
					OpenedSessionID: "sess-1",
					TargetSessionID: "sess-2",
					State:           "running",
					OpenedAt:        &now,
					LastActivityAt:  &now,
				},
			})
		},
	)
	mux.HandleFunc("/api/network/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.NetworkStatusResponse{
			Network: aghcontract.NetworkStatusPayload{
				Enabled:           true,
				Status:            "running",
				LocalPeers:        1,
				Channels:          1,
				MessagesSent:      3,
				MessagesDelivered: 2,
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/network/peers", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("channel"), "builders"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"network peers query channel = %q, want %q",
				got,
				want,
			)
			return
		}
		writeJSON(w, aghcontract.NetworkPeersResponse{
			Peers: []aghcontract.NetworkPeerPayload{{
				SessionID:   new("sess-1"),
				PeerID:      "coder.sess-1",
				DisplayName: "coder",
				Channel:     "builders",
				Local:       true,
				PeerCard: aghcontract.NetworkPeerCardPayload{
					PeerID:            "coder.sess-1",
					ProfilesSupported: []string{"agh-network/v0"},
					Capabilities: []aghcontract.NetworkCapabilityBriefPayload{{
						ID:      "chat.review",
						Summary: "Reviews chat requests.",
					}},
					ArtifactsSupported: []string{"capability"},
				},
			}},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/network/channels", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, aghcontract.CreateNetworkChannelResponse{
				Channel: aghcontract.NetworkChannelDetailPayload{
					Channel:   "builders",
					PeerCount: 1,
					Sessions: []aghcontract.SessionPayload{{
						ID:                           "sess-1",
						AgentName:                    "coder",
						ResolvedNetworkParticipation: participation.CloneSpec(harnessBuildersParticipationSpec()),
						State:                        "active",
					}},
				},
			})
			return
		}
		writeJSON(w, aghcontract.NetworkChannelsResponse{
			Channels: []aghcontract.NetworkChannelPayload{{
				Channel:        "builders",
				PeerCount:      1,
				LocalPeerCount: 1,
				SessionCount:   1,
				MessageCount:   1,
				LastActivityAt: &now,
			}},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/network/channels/builders", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.NetworkChannelResponse{
			Channel: aghcontract.NetworkChannelDetailPayload{
				Channel:   "builders",
				PeerCount: 1,
				Sessions: []aghcontract.SessionPayload{{
					ID:                           "sess-1",
					AgentName:                    "coder",
					ResolvedNetworkParticipation: participation.CloneSpec(harnessBuildersParticipationSpec()),
					State:                        "active",
				}},
				Peers: []aghcontract.NetworkPeerPayload{{
					SessionID:   new("sess-1"),
					PeerID:      "coder.sess-1",
					DisplayName: "coder",
					Channel:     "builders",
					Local:       true,
					PeerCard: aghcontract.NetworkPeerCardPayload{
						PeerID:            "coder.sess-1",
						ProfilesSupported: []string{"agh-network/v0"},
						Capabilities: []aghcontract.NetworkCapabilityBriefPayload{{
							ID:      "chat.review",
							Summary: "Reviews chat requests.",
						}},
						ArtifactsSupported: []string{"capability"},
					},
				}},
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/network/inbox", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("session_id"), "sess-1"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"network inbox query session_id = %q, want %q",
				got,
				want,
			)
			return
		}
		writeJSON(w, aghcontract.NetworkInboxResponse{
			Messages: []aghcontract.NetworkEnvelopePayload{{
				Protocol: "agh-network/v0",
				ID:       "msg-inbox-1",
				Kind:     "say",
				Channel:  "builders",
				Surface:  new("direct"),
				DirectID: new(harnessNetworkDirectID),
				From:     "peer-1",
				To:       new("coder.sess-1"),
				WorkID:   new(harnessNetworkWorkID),
				ReplyTo:  new("msg-1"),
				TraceID:  new("trace_ops_patch_42"),
				TS:       now.Unix(),
				Body:     json.RawMessage(`{"text":"review this"}`),
			}},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/network/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request aghcontract.NetworkSendRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			reportHarnessHandlerError(w, handlerErrs, http.StatusBadRequest, "decode network send: %v", err)
			return
		}
		for _, check := range []struct {
			label string
			got   string
			want  string
		}{
			{label: "session_id", got: request.SessionID, want: "sess-1"},
			{label: "channel", got: request.Channel, want: "builders"},
			{label: "surface", got: request.Surface, want: "thread"},
			{label: "thread_id", got: request.ThreadID, want: harnessNetworkThreadID},
			{label: "kind", got: request.Kind, want: "say"},
			{label: "work_id", got: request.WorkID, want: harnessNetworkWorkID},
			{label: "reply_to", got: request.ReplyTo, want: "msg-1"},
			{label: "trace_id", got: request.TraceID, want: "trace_ops_patch_42"},
			{label: "causation_id", got: request.CausationID, want: "msg-1"},
		} {
			if check.got != check.want {
				reportHarnessHandlerError(
					w,
					handlerErrs,
					http.StatusBadRequest,
					"network send %s = %q, want %q",
					check.label,
					check.got,
					check.want,
				)
				return
			}
		}
		writeJSON(w, aghcontract.NetworkSendResponse{
			Message: aghcontract.NetworkSendPayload{
				ID:          "msg-send-1",
				SessionID:   "sess-1",
				Channel:     "builders",
				Surface:     "thread",
				ThreadID:    harnessNetworkThreadID,
				Kind:        "say",
				WorkID:      harnessNetworkWorkID,
				ReplyTo:     "msg-1",
				TraceID:     "trace_ops_patch_42",
				CausationID: "msg-1",
			},
		})
	})
	mux.HandleFunc("/api/automation/runs", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("status"), "completed"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"automation runs query status = %q, want %q",
				got,
				want,
			)
			return
		}
		writeJSON(w, aghcontract.RunsResponse{
			Runs: []aghcontract.RunPayload{{
				ID:        "run-1",
				SessionID: "sess-1",
				Status:    "completed",
				Attempt:   1,
			}},
		})
	})
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("limit"), "10"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"tasks query limit = %q, want %q",
				got,
				want,
			)
			return
		}
		writeJSON(w, aghcontract.TasksResponse{
			Tasks: []aghcontract.TaskCatalogItemPayload{{
				ID:        "task-1",
				Scope:     "workspace",
				Title:     "demo",
				Status:    "in_progress",
				CreatedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAutomation, Ref: "job-1"},
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindAutomation, Ref: "run:run-1"},
				CreatedAt: now,
				UpdatedAt: now,
			}},
		})
	})
	mux.HandleFunc("/api/tasks/task-1/runs", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("status"), "completed"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"task runs query status = %q, want %q",
				got,
				want,
			)
			return
		}
		writeJSON(w, aghcontract.TaskRunsResponse{
			Runs: []aghcontract.TaskRunPayload{{
				ID:             "task-run-1",
				TaskID:         "task-1",
				Status:         taskpkg.TaskRunStatusCompleted,
				Attempt:        1,
				SessionID:      "sess-1",
				Origin:         taskpkg.Origin{Kind: taskpkg.OriginKindAutomation, Ref: "run:run-1"},
				IdempotencyKey: "automation-run:run-1",
				QueuedAt:       now,
			}},
		})
	})
	mux.HandleFunc("/api/extensions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, aghcontract.ExtensionsResponse{
				Extensions: []aghcontract.ExtensionPayload{{
					Name:          "telegram-reference",
					Version:       "0.1.0",
					Type:          "local",
					Source:        "user",
					Enabled:       true,
					State:         "registered",
					Capabilities:  []string{"bridge.adapter"},
					Actions:       []string{"bridges/messages/ingest", "bridges/instances/report_state"},
					Health:        "healthy",
					DaemonRunning: true,
				}},
			})
		case http.MethodPost:
			var request aghcontract.InstallExtensionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				reportHarnessHandlerError(
					w,
					handlerErrs,
					http.StatusBadRequest,
					"json.Decode(install extension) error = %v",
					err,
				)
				return
			}
			if got, want := request.Path, "/extensions/telegram-reference"; got != want {
				reportHarnessHandlerError(
					w,
					handlerErrs,
					http.StatusBadRequest,
					"install extension path = %q, want %q",
					got,
					want,
				)
				return
			}
			if got, want := request.Checksum, "sha256-demo"; got != want {
				reportHarnessHandlerError(
					w,
					handlerErrs,
					http.StatusBadRequest,
					"install extension checksum = %q, want %q",
					got,
					want,
				)
				return
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, aghcontract.ExtensionResponse{
				Extension: aghcontract.ExtensionPayload{
					Name:          "telegram-reference",
					Version:       "0.1.0",
					Type:          "local",
					Source:        "user",
					Enabled:       true,
					State:         "registered",
					Capabilities:  []string{"bridge.adapter"},
					Actions:       []string{"bridges/messages/ingest", "bridges/instances/report_state"},
					Health:        "healthy",
					DaemonRunning: true,
				},
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/extensions/telegram-reference", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.ExtensionResponse{
			Extension: aghcontract.ExtensionPayload{
				Name:          "telegram-reference",
				Version:       "0.1.0",
				Type:          "local",
				Source:        "user",
				Enabled:       true,
				State:         "registered",
				Capabilities:  []string{"bridge.adapter"},
				Actions:       []string{"bridges/messages/ingest", "bridges/instances/report_state"},
				Health:        "healthy",
				DaemonRunning: true,
			},
		})
	})
	mux.HandleFunc("/api/extensions/telegram-reference/enable", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.ExtensionResponse{
			Extension: aghcontract.ExtensionPayload{
				Name:          "telegram-reference",
				Version:       "0.1.0",
				Type:          "local",
				Source:        "user",
				Enabled:       true,
				State:         "active",
				Capabilities:  []string{"bridge.adapter"},
				Actions:       []string{"bridges/messages/ingest", "bridges/instances/report_state"},
				Health:        "healthy",
				DaemonRunning: true,
			},
		})
	})
	mux.HandleFunc("/api/extensions/telegram-reference/disable", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.ExtensionResponse{
			Extension: aghcontract.ExtensionPayload{
				Name:          "telegram-reference",
				Version:       "0.1.0",
				Type:          "local",
				Source:        "user",
				Enabled:       false,
				State:         "registered",
				Capabilities:  []string{"bridge.adapter"},
				Actions:       []string{"bridges/messages/ingest", "bridges/instances/report_state"},
				Health:        "idle",
				DaemonRunning: true,
			},
		})
	})
	mux.HandleFunc("/api/bridges", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request aghcontract.CreateBridgeRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"json.Decode(create bridge) error = %v",
				err,
			)
			return
		}
		if got, want := request.ExtensionName, "telegram-reference"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"create bridge extension = %q, want %q",
				got,
				want,
			)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, aghcontract.BridgeResponse{
			Bridge: aghcontract.BridgePayload{
				ID:            "brg-1",
				Scope:         bridgepkg.ScopeWorkspace,
				WorkspaceID:   "ws-1",
				Platform:      "telegram",
				ExtensionName: "telegram-reference",
				DisplayName:   "Telegram Runtime",
				Enabled:       false,
				Status:        bridgepkg.BridgeStatusDisabled,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			Health: aghcontract.BridgeHealthPayload{
				BridgeInstanceID: "brg-1",
				Status:           bridgepkg.BridgeStatusDisabled,
			},
		})
	})
	mux.HandleFunc("/api/bridges/brg-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.BridgeResponse{
			Bridge: aghcontract.BridgePayload{
				ID:            "brg-1",
				Scope:         bridgepkg.ScopeWorkspace,
				WorkspaceID:   "ws-1",
				Platform:      "telegram",
				ExtensionName: "telegram-reference",
				DisplayName:   "Telegram Runtime",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusReady,
				CreatedAt:     now,
				UpdatedAt:     routeTime,
			},
			Health: aghcontract.BridgeHealthPayload{
				BridgeInstanceID: "brg-1",
				Status:           bridgepkg.BridgeStatusReady,
				RouteCount:       1,
				DeliveryBacklog:  0,
			},
		})
	})
	mux.HandleFunc("/api/bridges/brg-1/enable", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.BridgeResponse{
			Bridge: aghcontract.BridgePayload{
				ID:            "brg-1",
				Scope:         bridgepkg.ScopeWorkspace,
				WorkspaceID:   "ws-1",
				Platform:      "telegram",
				ExtensionName: "telegram-reference",
				DisplayName:   "Telegram Runtime",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusStarting,
				CreatedAt:     now,
				UpdatedAt:     routeTime,
			},
			Health: aghcontract.BridgeHealthPayload{
				BridgeInstanceID: "brg-1",
				Status:           bridgepkg.BridgeStatusStarting,
			},
		})
	})
	mux.HandleFunc("/api/bridges/brg-1/restart", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.BridgeResponse{
			Bridge: aghcontract.BridgePayload{
				ID:            "brg-1",
				Scope:         bridgepkg.ScopeWorkspace,
				WorkspaceID:   "ws-1",
				Platform:      "telegram",
				ExtensionName: "telegram-reference",
				DisplayName:   "Telegram Runtime",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusReady,
				CreatedAt:     now,
				UpdatedAt:     routeTime,
			},
			Health: aghcontract.BridgeHealthPayload{
				BridgeInstanceID: "brg-1",
				Status:           bridgepkg.BridgeStatusReady,
				RouteCount:       1,
				DeliveryBacklog:  0,
			},
		})
	})
	mux.HandleFunc("/api/bridges/brg-1/routes", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.BridgeRoutesResponse{
			Routes: []bridgepkg.BridgeRoute{{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-1",
				BridgeInstanceID: "brg-1",
				PeerID:           "telegram:chat:777:user:888",
				ThreadID:         "654",
				SessionID:        "sess-1",
				AgentName:        "coder",
				LastActivityAt:   routeTime,
				CreatedAt:        routeTime,
				UpdatedAt:        routeTime,
			}},
		})
	})
	mux.HandleFunc("/api/bridges/brg-1/secret-bindings", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, aghcontract.BridgeSecretBindingsResponse{
			Bindings: []bridgepkg.BridgeSecretBinding{{
				BridgeInstanceID: "brg-1",
				BindingName:      "bot_token",
				SecretRef:        "vault:bridges/brg-1/bot_token",
				Kind:             "token",
				CreatedAt:        now,
				UpdatedAt:        routeTime,
			}},
		})
	})
	mux.HandleFunc("/api/bridges/brg-1/secret-bindings/bot_token", func(w http.ResponseWriter, r *http.Request) {
		var request aghcontract.PutBridgeSecretBindingRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"json.Decode(put bridge secret binding) error = %v",
				err,
			)
			return
		}
		if got, want := request.SecretRef, "vault:bridges/brg-1/bot_token"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"secret binding secret_ref = %q, want %q",
				got,
				want,
			)
			return
		}
		if request.SecretValue == nil || *request.SecretValue != "telegram-bot-token" {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"secret binding secret_value = %v, want write-only token",
				request.SecretValue,
			)
			return
		}
		if got, want := request.Kind, "token"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"secret binding kind = %q, want %q",
				got,
				want,
			)
			return
		}
		writeJSON(w, aghcontract.BridgeSecretBindingResponse{
			Binding: bridgepkg.BridgeSecretBinding{
				BridgeInstanceID: "brg-1",
				BindingName:      "bot_token",
				SecretRef:        request.SecretRef,
				Kind:             request.Kind,
				CreatedAt:        now,
				UpdatedAt:        routeTime,
			},
		})
	})
	mux.HandleFunc("/api/bridges/health/stream", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("bridge_ids"); got != "brg-1" {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"bridge_ids query = %q, want brg-1",
				got,
			)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		payload, err := json.Marshal(aghcontract.BridgeHealthStreamPayload{
			GeneratedAt: now,
			BridgeHealth: map[string]aghcontract.BridgeHealthPayload{
				"brg-1": {
					BridgeInstanceID: "brg-1",
					Status:           "ready",
					RouteCount:       1,
					DeliveryBacklog:  0,
				},
			},
		})
		if err != nil {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusInternalServerError,
				"json.Marshal(bridge health) error = %v",
				err,
			)
			return
		}
		if _, err := fmt.Fprintf(w, "event: bridge_health\ndata: %s\n\n", payload); err != nil {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusInternalServerError,
				"write bridge health stream: %v",
				err,
			)
		}
	})

	server := &harnessTestServer{
		Server:      httptest.NewServer(mux),
		handlerErrs: handlerErrs,
	}
	t.Cleanup(func() {
		server.assertNoHandlerErrors(t)
	})
	t.Cleanup(server.Close)
	return server
}

func writeJSON(w http.ResponseWriter, value any) {
	if err := writeJSONResponse(w, value); err != nil {
		panic(err)
	}
}

func writeJSONResponse(w http.ResponseWriter, value any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(value)
}

func testContext(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func (s *harnessTestServer) assertNoHandlerErrors(t testing.TB) {
	t.Helper()

	close(s.handlerErrs)

	messages := make([]string, 0)
	for err := range s.handlerErrs {
		messages = append(messages, err.Error())
	}
	if len(messages) > 0 {
		t.Fatalf("handler validation errors:\n%s", strings.Join(messages, "\n"))
	}
}

func reportHarnessHandlerError(
	w http.ResponseWriter,
	errCh chan<- error,
	status int,
	format string,
	args ...any,
) {
	err := fmt.Errorf(format, args...)
	errCh <- err
	http.Error(w, err.Error(), status)
}

func assertErrorContains(t testing.TB, err error, want string) {
	t.Helper()

	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want substring %q", err, want)
	}
}
