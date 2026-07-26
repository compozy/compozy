package contract_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/loop/dsl"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/windowmanager"
)

func TestWindowManagerReturnAnchorContract(t *testing.T) {
	t.Run("Should preserve an exact source group through the public layout round trip", func(t *testing.T) {
		t.Parallel()

		axis := windowmanager.AxisHorizontal
		zoomedWindowID := windowmanager.WindowID("window:zoomed")
		peerWindowID := windowmanager.WindowID("window:peer")
		activeWindowID := windowmanager.WindowID("window:active")
		sourceGroup := windowmanager.LayoutGroup{
			ID:    "group:source",
			Frame: windowmanager.NormalizedRect{X: 0.1, Y: 0.2, Width: 0.8, Height: 0.6},
			Root: windowmanager.LayoutNode{
				ID: "split:source", Kind: windowmanager.NodeKindSplit, Axis: &axis,
				Children: []windowmanager.LayoutNode{
					{
						ID: "leaf:zoomed", Kind: windowmanager.NodeKindLeaf,
						WindowID: &zoomedWindowID, Children: []windowmanager.LayoutNode{},
					},
					{
						ID: "stack:peers", Kind: windowmanager.NodeKindStack,
						WindowIDs: []windowmanager.WindowID{peerWindowID, activeWindowID},
						ActiveID:  &activeWindowID,
						Children:  []windowmanager.LayoutNode{},
					},
				},
				Weights: []float64{0.37, 0.63},
			},
		}
		document := windowmanager.LayoutDocument{
			Version: windowmanager.SnapshotVersion, WorkspaceID: "workspace:test",
			Desktops: []windowmanager.Desktop{{
				ID: "desktop:main", Name: "Main", Purpose: windowmanager.DesktopPurposeStandard,
				Groups: []windowmanager.LayoutGroup{}, Floating: []windowmanager.WindowID{zoomedWindowID},
			}},
			Windows: map[windowmanager.WindowID]windowmanager.Window{
				zoomedWindowID: {
					ID: zoomedWindowID, App: "tasks",
					Route:        windowmanager.RouteIntent{Pathname: "/tasks", Search: windowmanager.RouteSearch{}},
					Placement:    windowmanager.WindowPlacementFloating,
					DesktopID:    "desktop:main",
					FloatingRect: windowmanager.NormalizedRect{X: 0.2, Y: 0.2, Width: 0.6, Height: 0.6},
					ReturnAnchor: &windowmanager.ReturnAnchor{
						DesktopID: "desktop:main", SourceRevision: 17, SourceGroup: &sourceGroup,
					},
				},
			},
		}

		wireDocument, err := contract.WindowManagerLayoutFromDomain(document)
		if err != nil {
			t.Fatalf("WindowManagerLayoutFromDomain() error = %v", err)
		}
		data, err := json.Marshal(wireDocument)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		var decoded contract.WindowManagerLayoutDocument
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		anchor := decoded.Domain().Windows[zoomedWindowID].ReturnAnchor
		if anchor == nil || anchor.SourceGroup == nil {
			t.Fatalf("round-trip return anchor = %#v, want exact source group", anchor)
		}
		if !reflect.DeepEqual(*anchor.SourceGroup, sourceGroup) {
			t.Fatalf("round-trip source group = %#v, want %#v", *anchor.SourceGroup, sourceGroup)
		}
	})

	t.Run("Should omit a source group when the return anchor has no exact capture", func(t *testing.T) {
		t.Parallel()

		anchor := contract.WindowManagerReturnAnchor{
			DesktopID: "desktop:main", SourceRevision: 9,
		}
		data, err := json.Marshal(anchor)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if strings.Contains(string(data), `"source_group"`) {
			t.Fatalf("return anchor JSON includes omitted source_group: %s", data)
		}
		var decoded contract.WindowManagerReturnAnchor
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.SourceGroup != nil {
			t.Fatalf("decoded source group = %#v, want nil", decoded.SourceGroup)
		}
	})

	t.Run("Should isolate mutable layout values in both conversion directions", func(t *testing.T) {
		t.Parallel()
		axis := windowmanager.AxisHorizontal
		windowID := windowmanager.WindowID("window:a")
		focusOwner := windowID
		historyLimit := 20
		document := windowmanager.LayoutDocument{
			Version: windowmanager.SnapshotVersion, WorkspaceID: "workspace:test",
			Desktops: []windowmanager.Desktop{{
				ID: "desktop:main", Name: "Main", Purpose: windowmanager.DesktopPurposeStandard,
				FocusOwner: &focusOwner,
				Groups: []windowmanager.LayoutGroup{{
					ID: "group:main", Frame: windowmanager.NormalizedRect{Width: 1, Height: 1},
					Root: windowmanager.LayoutNode{
						ID: "split:main", Kind: windowmanager.NodeKindSplit, Axis: &axis,
						Weights: []float64{0.4, 0.6},
						Children: []windowmanager.LayoutNode{
							{ID: "leaf:a", Kind: windowmanager.NodeKindLeaf, WindowID: &windowID},
							{ID: "stack:b", Kind: windowmanager.NodeKindStack,
								WindowIDs: []windowmanager.WindowID{"window:b", "window:c"}},
						},
					},
				}},
				Floating: []windowmanager.WindowID{"window:floating"},
			}},
			Windows: map[windowmanager.WindowID]windowmanager.Window{},
			Overrides: windowmanager.WorkspaceConfig{
				HistoryLimit: &historyLimit,
				Snap:         &windowmanager.SnapConfig{RepeatRatios: []float64{0.5, 0.75}},
				Shortcuts:    map[string]string{"layout.balance": "Meta+Shift+KeyB"},
			},
		}

		wire, err := contract.WindowManagerLayoutFromDomain(document)
		if err != nil {
			t.Fatalf("WindowManagerLayoutFromDomain() error = %v", err)
		}
		wire.Desktops[0].FocusOwner = new(windowmanager.WindowID)
		wire.Desktops[0].Floating[0] = "wire-mutated"
		wire.Desktops[0].Groups[0].Root.Weights[0] = 0.9
		wire.Desktops[0].Groups[0].Root.Children[0].WindowID = new(windowmanager.WindowID)
		*wire.Overrides.HistoryLimit = 99
		wire.Overrides.Snap.RepeatRatios[0] = 0.25
		wire.Overrides.Shortcuts["layout.balance"] = "Alt+KeyB"
		if *document.Desktops[0].FocusOwner != windowID ||
			document.Desktops[0].Floating[0] != "window:floating" ||
			document.Desktops[0].Groups[0].Root.Weights[0] != 0.4 ||
			*document.Desktops[0].Groups[0].Root.Children[0].WindowID != windowID ||
			*document.Overrides.HistoryLimit != 20 ||
			document.Overrides.Snap.RepeatRatios[0] != 0.5 ||
			document.Overrides.Shortcuts["layout.balance"] != "Meta+Shift+KeyB" {
			t.Fatalf("wire conversion aliases domain layout: %#v", document.Desktops[0])
		}

		domain := wire.Domain()
		domain.Desktops[0].Floating[0] = "domain-mutated"
		domain.Desktops[0].Groups[0].Root.Weights[0] = 0.2
		*domain.Overrides.HistoryLimit = 8
		domain.Overrides.Snap.RepeatRatios[0] = 0.1
		domain.Overrides.Shortcuts["layout.balance"] = "Control+KeyB"
		if wire.Desktops[0].Floating[0] != "wire-mutated" ||
			wire.Desktops[0].Groups[0].Root.Weights[0] != 0.9 ||
			*wire.Overrides.HistoryLimit != 99 ||
			wire.Overrides.Snap.RepeatRatios[0] != 0.25 ||
			wire.Overrides.Shortcuts["layout.balance"] != "Alt+KeyB" {
			t.Fatalf("domain conversion aliases wire layout: %#v", wire.Desktops[0])
		}
	})
}

func TestLoopDefinitionDocumentPreservesWatchEvents(t *testing.T) {
	t.Run("Should preserve watch-events subscriptions across the public DTO boundary", func(t *testing.T) {
		t.Parallel()

		const raw = `{
			"definition": {
				"apiVersion": "agh.loop/v1",
				"kind": "Loop",
				"meta": {"name": "watch-events-contract", "catalog": {}},
				"contract": {
					"goal": "",
					"definition_of_done": "Create one follow-up task.",
					"iteration_cap": 0,
					"no_progress": {"window": 0},
					"budget": {"tokens": 0, "wall_clock_sec": 0}
				},
				"graph": {
					"nodes": [{
						"id": "on_parent_completed",
						"class": "source",
						"kind": "watch-events",
						"events": [{
							"kind": "task.status_changed",
							"filter": "event.task_id == 'task-parent' && event.payload.to_status == 'completed'"
						}]
					}],
					"edges": []
				}
			}
		}`

		var request contract.ValidateLoopRequest
		if err := json.Unmarshal([]byte(raw), &request); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		var definition dsl.Definition
		if err := request.Definition.Decode(&definition); err != nil {
			t.Fatalf("LoopDefinitionDocument.Decode() error = %v", err)
		}
		if len(definition.Graph.Nodes) != 1 {
			t.Fatalf("decoded nodes = %d, want 1", len(definition.Graph.Nodes))
		}
		if len(definition.Graph.Nodes[0].Events) != 1 {
			t.Fatalf("decoded watch-events subscriptions = %d, want 1", len(definition.Graph.Nodes[0].Events))
		}
		subscription := definition.Graph.Nodes[0].Events[0]
		if subscription.Kind != "task.status_changed" {
			t.Fatalf("subscription.Kind = %q, want task.status_changed", subscription.Kind)
		}
		if subscription.Filter != "event.task_id == 'task-parent' && event.payload.to_status == 'completed'" {
			t.Fatalf("subscription.Filter = %q, want authored CEL filter", subscription.Filter)
		}
	})
}

func TestLoopDefinitionDocumentPreservesNetworkParticipation(t *testing.T) {
	t.Run("Should preserve definition participation across the public DTO boundary", func(t *testing.T) {
		t.Parallel()

		mode := participation.ModeLive
		strategy := participation.StrategyNamed
		channelID := "release-room"
		definition := dsl.Definition{
			APIVersion: dsl.APIVersion,
			Kind:       dsl.KindLoop,
			Meta:       dsl.Meta{Name: "network-contract"},
			DefinitionExtensionState: &dsl.DefinitionExtensionState{
				NetworkParticipation: &participation.Request{
					Mode:            &mode,
					ChannelStrategy: &strategy,
					ChannelID:       &channelID,
				},
			},
		}

		document, err := contract.NewLoopDefinitionDocument(definition)
		if err != nil {
			t.Fatalf("NewLoopDefinitionDocument() error = %v", err)
		}
		if document.NetworkParticipation == nil {
			t.Fatal("document.NetworkParticipation = nil, want authored Live request")
		}
		if document.NetworkParticipation.Mode == nil || *document.NetworkParticipation.Mode != mode {
			t.Fatalf("document.NetworkParticipation.Mode = %v, want %q", document.NetworkParticipation.Mode, mode)
		}
		if document.NetworkParticipation.ChannelStrategy == nil ||
			*document.NetworkParticipation.ChannelStrategy != strategy {
			t.Fatalf(
				"document.NetworkParticipation.ChannelStrategy = %v, want %q",
				document.NetworkParticipation.ChannelStrategy,
				strategy,
			)
		}
		if document.NetworkParticipation.ChannelID == nil || *document.NetworkParticipation.ChannelID != channelID {
			t.Fatalf(
				"document.NetworkParticipation.ChannelID = %v, want %q",
				document.NetworkParticipation.ChannelID,
				channelID,
			)
		}

		var decoded dsl.Definition
		if err := document.Decode(&decoded); err != nil {
			t.Fatalf("LoopDefinitionDocument.Decode() error = %v", err)
		}
		if decoded.NetworkParticipation == nil || decoded.NetworkParticipation.ChannelID == nil ||
			*decoded.NetworkParticipation.ChannelID != channelID {
			t.Fatalf("decoded.NetworkParticipation = %#v, want channel %q", decoded.NetworkParticipation, channelID)
		}

		encoded, err := json.Marshal(document)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		var publicShape map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &publicShape); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if _, ok := publicShape["network_participation"]; !ok {
			t.Fatalf("encoded document keys = %v, want network_participation", publicShape)
		}
	})
}

func TestSessionPayloadJSONShape(t *testing.T) {
	t.Run("Should preserve session payload JSON shape", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 7, 10, 30, 0, 0, time.UTC)
		ttl := now.Add(time.Hour)
		payload := core.SessionPayloadFromInfo(&session.Info{
			ID:              "sess-1",
			Name:            "demo",
			AgentName:       "coder",
			Provider:        "fake",
			Model:           "gpt-test",
			ReasoningEffort: "high",
			WorkspaceID:     "ws_alpha",
			Workspace:       "/workspace",
			State:           session.StateActive,
			ACPSessionID:    "acp-123",
			Lineage: &store.SessionLineage{
				RootSessionID:    "sess-1",
				SpawnDepth:       0,
				TTLExpiresAt:     &ttl,
				SpawnBudget:      store.SessionSpawnBudget{TTLSeconds: 3600},
				PermissionPolicy: store.SessionPermissionPolicy{Tools: []string{"read"}},
			},
			Sandbox: &store.SessionSandboxMeta{
				SandboxID:  "env-json",
				Backend:    "local",
				Profile:    "local",
				State:      "prepared",
				InstanceID: "instance-json",
			},
			CreatedAt: now,
			UpdatedAt: now,
			ACPCaps: acp.Caps{
				SupportsLoadSession: true,
				SupportedModes:      []string{"chat"},
				ConfigOptions: []acp.SessionConfigOption{
					{
						ID:      "model",
						Label:   "Model",
						Kind:    acp.SessionConfigOptionKindSelect,
						Current: "gpt-test",
						Values: []acp.SessionConfigOptionValue{
							{Value: "gpt-test", Label: "GPT Test"},
						},
					},
				},
			},
		})

		var got map[string]any
		marshalJSON(t, payload, &got)

		if got["agent_name"] != "coder" ||
			got["provider"] != "fake" ||
			got["model"] != "gpt-test" ||
			got["reasoning_effort"] != "high" ||
			got["workspace_id"] != "ws_alpha" ||
			got["workspace_path"] != "/workspace" {
			t.Fatalf("session JSON = %#v", got)
		}
		if _, exists := got["stop_reason"]; exists {
			t.Fatalf("session JSON should omit empty stop_reason: %#v", got)
		}
		if _, exists := got["stop_detail"]; exists {
			t.Fatalf("session JSON should omit empty stop_detail: %#v", got)
		}
		if _, exists := got["acp_session_id"]; !exists {
			t.Fatalf("session JSON missing acp_session_id: %#v", got)
		}
		lineage, ok := got["lineage"].(map[string]any)
		if !ok {
			t.Fatalf("lineage type = %T, want object", got["lineage"])
		}
		if lineage["root_session_id"] != "sess-1" || lineage["spawn_depth"] != float64(0) {
			t.Fatalf("lineage JSON = %#v", lineage)
		}
		if _, exists := lineage["permission_policy_json"]; exists {
			t.Fatalf("lineage JSON leaked raw policy storage: %#v", lineage)
		}
		acpCaps, ok := got["acp_caps"].(map[string]any)
		if !ok {
			t.Fatalf("acp_caps type = %T, want object", got["acp_caps"])
		}
		if acpCaps["supports_load_session"] != true {
			t.Fatalf("acp_caps JSON = %#v", acpCaps)
		}
		configOptions, ok := acpCaps["config_options"].([]any)
		if !ok || len(configOptions) != 1 {
			t.Fatalf("config_options JSON = %#v", acpCaps["config_options"])
		}
		configOption, ok := configOptions[0].(map[string]any)
		if !ok {
			t.Fatalf("config option type = %T, want object", configOptions[0])
		}
		if configOption["id"] != "model" || configOption["kind"] != "select" || configOption["current"] != "gpt-test" {
			t.Fatalf("config option JSON = %#v", configOption)
		}
		values, ok := configOption["values"].([]any)
		if !ok || len(values) != 1 {
			t.Fatalf("config option values JSON = %#v", configOption["values"])
		}
		firstValue, ok := values[0].(map[string]any)
		if !ok {
			t.Fatalf("config option value type = %T, want object", values[0])
		}
		if firstValue["value"] != "gpt-test" || firstValue["label"] != "GPT Test" {
			t.Fatalf("config option value JSON = %#v", firstValue)
		}
		sandboxPayload, ok := got["sandbox"].(map[string]any)
		if !ok {
			t.Fatalf("sandbox type = %T, want object", got["sandbox"])
		}
		if sandboxPayload["sandbox_id"] != "env-json" ||
			sandboxPayload["backend"] != "local" ||
			sandboxPayload["instance_id"] != "instance-json" {
			t.Fatalf("sandbox JSON = %#v", sandboxPayload)
		}
	})
}

func TestNetworkSendRequestRejectsLegacyConversationFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "Should reject unknown fields",
			raw: `{
				"session_id":"sess-a",
				"channel":"builders",
				"surface":"thread",
				"thread_id":"thread_launch_db",
				"kind":"say",
				"legacy":true,
				"body":{"text":"hello"}
			}`,
			want: "unknown field",
		},
		{
			name: "Should reject interaction id",
			raw: `{
				"session_id":"sess-a",
				"channel":"builders",
				"surface":"thread",
				"thread_id":"thread_launch_db",
				"kind":"say",
				"interaction_id":"legacy",
				"body":{"text":"hello"}
			}`,
			want: "interaction_id",
		},
		{
			name: "Should reject direct kind",
			raw: `{
				"session_id":"sess-a",
				"channel":"builders",
				"surface":"direct",
				"direct_id":"direct_99401d24bee62651d189e5a561785466",
				"kind":"direct",
				"body":{"text":"hello"}
			}`,
			want: "kind direct",
		},
		{
			name: "Should reject caller supplied verified-format identity",
			raw: `{
				"session_id":"sess-a",
				"channel":"builders",
				"surface":"thread",
				"thread_id":"thread_launch_db",
				"kind":"say",
				"from":"alice@39f713d0a644253f04529421b9f51b9b",
				"body":{"text":"hello"}
			}`,
			want: "sender identity and proof are daemon-derived",
		},
		{
			name: "Should reject caller supplied proof",
			raw: `{
				"session_id":"sess-a",
				"channel":"builders",
				"surface":"thread",
				"thread_id":"thread_launch_db",
				"kind":"say",
				"proof":{"profile":"agh-network.trust.ed25519-jcs/v1"},
				"body":{"text":"hello"}
			}`,
			want: "sender identity and proof are daemon-derived",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var req contract.NetworkSendRequest
			err := json.Unmarshal([]byte(tt.raw), &req)
			if err == nil {
				t.Fatalf("json.Unmarshal() error = nil, want rejection containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("json.Unmarshal() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRuntimeActivityJSONPreservesZeroMetrics(t *testing.T) {
	t.Run("Should preserve zero metrics in runtime activity payload", func(t *testing.T) {
		t.Parallel()

		var got map[string]any
		marshalJSON(t, contract.RuntimeActivityPayload{}, &got)

		assertZeroMetricField(t, got, "iteration_current")
		assertZeroMetricField(t, got, "iteration_max")
		assertZeroMetricField(t, got, "idle_seconds")
		assertZeroMetricField(t, got, "elapsed_seconds")
	})

	t.Run("Should preserve zero metrics in session activity health payload", func(t *testing.T) {
		t.Parallel()

		var got map[string]any
		marshalJSON(t, contract.SessionActivityHealthPayload{
			SessionID: "sess-health",
			Status:    "active",
		}, &got)

		assertZeroMetricField(t, got, "iteration_current")
		assertZeroMetricField(t, got, "iteration_max")
		assertZeroMetricField(t, got, "idle_seconds")
		assertZeroMetricField(t, got, "elapsed_seconds")
	})
}

func TestCreateSessionRequestJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should decode optional provider when present", func(t *testing.T) {
		t.Parallel()

		var req contract.CreateSessionRequest
		if err := json.Unmarshal(
			[]byte(`{"agent_name":"coder","provider":"fake","workspace":"alpha"}`),
			&req,
		); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if req.AgentName != "coder" || req.Provider != "fake" || req.Workspace != "alpha" {
			t.Fatalf("request = %#v", req)
		}
	})

	t.Run("Should omit provider cleanly when absent", func(t *testing.T) {
		t.Parallel()

		var req contract.CreateSessionRequest
		if err := json.Unmarshal(
			[]byte(`{"agent_name":"coder","workspace_path":"/workspace"}`),
			&req,
		); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if req.Provider != "" {
			t.Fatalf("request.Provider = %q, want empty", req.Provider)
		}
		if req.WorkspacePath != "/workspace" {
			t.Fatalf("request = %#v", req)
		}
	})

	t.Run("Should round-trip model and reasoning_effort overrides", func(t *testing.T) {
		t.Parallel()

		req := contract.CreateSessionRequest{
			AgentName:       "coder",
			Provider:        "codex",
			Model:           "gpt-5.4",
			ReasoningEffort: "high",
			Workspace:       "alpha",
		}
		raw, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		var decoded contract.CreateSessionRequest
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.Model != "gpt-5.4" || decoded.ReasoningEffort != "high" {
			t.Fatalf("decoded = %#v", decoded)
		}
		var shape map[string]any
		if err := json.Unmarshal(raw, &shape); err != nil {
			t.Fatalf("json.Unmarshal(map) error = %v", err)
		}
		if shape["model"] != "gpt-5.4" || shape["reasoning_effort"] != "high" {
			t.Fatalf("shape = %#v", shape)
		}
	})

	t.Run("Should round-trip the optional initial prompt", func(t *testing.T) {
		t.Parallel()

		req := contract.CreateSessionRequest{
			AgentName: "coder",
			Workspace: "alpha",
			Prompt:    "Investigate the failing build",
		}
		raw, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		var decoded contract.CreateSessionRequest
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.Prompt != req.Prompt {
			t.Fatalf("decoded.Prompt = %q, want %q", decoded.Prompt, req.Prompt)
		}
		var shape map[string]any
		if err := json.Unmarshal(raw, &shape); err != nil {
			t.Fatalf("json.Unmarshal(map) error = %v", err)
		}
		if shape["prompt"] != req.Prompt {
			t.Fatalf("shape = %#v, want prompt key %q", shape, req.Prompt)
		}
	})

	t.Run("Should omit model and reasoning_effort cleanly when absent", func(t *testing.T) {
		t.Parallel()

		req := contract.CreateSessionRequest{AgentName: "coder", Workspace: "alpha"}
		raw, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if strings.Contains(string(raw), "model") || strings.Contains(string(raw), "reasoning_effort") {
			t.Fatalf("raw = %s", string(raw))
		}
	})

	t.Run("Should omit prompt cleanly when absent", func(t *testing.T) {
		t.Parallel()

		raw, err := json.Marshal(contract.CreateSessionRequest{AgentName: "coder", Workspace: "alpha"})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if strings.Contains(string(raw), "prompt") {
			t.Fatalf("raw = %s, want prompt omitted", string(raw))
		}
	})
}

func TestMemoryV2PublicContractJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should expose scope and agent tier without legacy workspace field", func(t *testing.T) {
		t.Parallel()

		req := contract.MemoryCreateRequest{
			Scope:       memcontract.ScopeAgent,
			WorkspaceID: "ws_01HXYZ",
			AgentName:   "reviewer",
			AgentTier:   memcontract.AgentTierWorkspace,
			Origin:      memcontract.OriginHTTP,
			Type:        memcontract.TypeFeedback,
			Name:        "Reviewer preference",
			Content:     "Prefer terse PR feedback.",
		}

		var got map[string]any
		marshalJSON(t, req, &got)

		if got["scope"] != "agent" || got["agent_tier"] != "workspace" || got["workspace_id"] != "ws_01HXYZ" {
			t.Fatalf("memory create JSON = %#v", got)
		}
		assertJSONFieldAbsent(t, got, "workspace")
	})

	t.Run("Should not leak replay material or raw LLM response in decisions", func(t *testing.T) {
		t.Parallel()

		decision := contract.MemoryDecisionPayload{
			ID:              "dec_01",
			CandidateHash:   "sha256:candidate",
			Op:              contract.MemoryDecisionOpUpdate,
			Scope:           memcontract.ScopeWorkspace,
			WorkspaceID:     "ws_01HXYZ",
			TargetFilename:  "feedback_reviewer.md",
			Frontmatter:     memcontract.Header{Name: "Reviewer preference", Type: memcontract.TypeFeedback},
			PostContentHash: "sha256:post",
			Confidence:      0.93,
			Source:          memcontract.SourceLLM,
			LLMTrace: &contract.MemoryLLMTracePayload{
				Model:         "haiku",
				PromptVersion: "v1",
				LatencyMs:     37,
			},
			DecidedAt: time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
		}

		var got map[string]any
		marshalJSON(t, decision, &got)

		assertJSONFieldAbsent(t, got, "post_content")
		assertJSONFieldAbsent(t, got, "prior_content")
		llmTrace, ok := got["llm_trace"].(map[string]any)
		if !ok {
			t.Fatalf("llm_trace = %#v, want object", got["llm_trace"])
		}
		assertJSONFieldAbsent(t, llmTrace, "raw_response")
		if llmTrace["latency_ms"] != float64(37) {
			t.Fatalf("llm_trace latency_ms = %#v, want 37", llmTrace["latency_ms"])
		}
	})

	t.Run("Should expose deterministic memory error envelope", func(t *testing.T) {
		t.Parallel()

		var got map[string]any
		marshalJSON(t, contract.MemoryErrorPayload{
			Code:    "memory.scope.workspace_required",
			Message: "workspace_id is required for workspace scope",
			Details: map[string]any{
				"scope": "workspace",
			},
		}, &got)

		if got["code"] != "memory.scope.workspace_required" || got["message"] == "" {
			t.Fatalf("memory error JSON = %#v", got)
		}
		assertJSONFieldAbsent(t, got, "error")
	})
}

func TestSessionPayloadJSONIncludesSessionStopFields(t *testing.T) {
	t.Run("Should include session stop fields in JSON", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 7, 10, 30, 0, 0, time.UTC)
		payload := core.SessionPayloadFromInfo(&session.Info{
			ID:          "sess-stopped",
			Name:        "demo",
			AgentName:   "coder",
			WorkspaceID: "ws_alpha",
			Workspace:   "/workspace",
			State:       session.StateStopped,
			StopReason:  store.StopUserCanceled,
			StopDetail:  "requested by API",
			Failure: &store.SessionFailure{
				Kind:    store.FailureCanceled,
				Summary: "requested by API",
			},
			CreatedAt: now,
			UpdatedAt: now,
		})

		var got map[string]any
		marshalJSON(t, payload, &got)

		if got["stop_reason"] != string(store.StopUserCanceled) {
			t.Fatalf("stop_reason = %#v, want %q", got["stop_reason"], store.StopUserCanceled)
		}
		if got["stop_detail"] != "requested by API" {
			t.Fatalf("stop_detail = %#v, want %q", got["stop_detail"], "requested by API")
		}
		failure, ok := got["failure"].(map[string]any)
		if !ok {
			t.Fatalf("failure = %#v, want object", got["failure"])
		}
		if failure["kind"] != string(store.FailureCanceled) || failure["summary"] != "requested by API" {
			t.Fatalf("failure JSON = %#v", failure)
		}
	})
}

func TestWorkspacePayloadPreservesOmitEmptyBehavior(t *testing.T) {
	t.Run("Should preserve workspace omit-empty behavior", func(t *testing.T) {
		t.Parallel()

		payload := contract.WorkspacePayload{
			ID:        "ws_alpha",
			RootDir:   "/workspace",
			AddDirs:   []string{},
			Name:      "alpha",
			CreatedAt: time.Date(2026, 4, 7, 10, 30, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 7, 11, 30, 0, 0, time.UTC),
		}

		var got map[string]any
		marshalJSON(t, payload, &got)

		if _, exists := got["default_agent"]; exists {
			t.Fatalf("default_agent should be omitted: %#v", got)
		}
		addDirs, ok := got["add_dirs"].([]any)
		if !ok {
			t.Fatalf("add_dirs type = %T, want array", got["add_dirs"])
		}
		if len(addDirs) != 0 {
			t.Fatalf("add_dirs length = %d, want 0", len(addDirs))
		}
	})
}

func TestWorkspaceSandboxRefJSONFields(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize create workspace sandbox_ref", func(t *testing.T) {
		t.Parallel()

		payload := contract.CreateWorkspaceRequest{
			RootDir:    "/workspace",
			SandboxRef: "daytona-dev",
		}

		var got map[string]any
		marshalJSON(t, payload, &got)

		if got["sandbox_ref"] != "daytona-dev" {
			t.Fatalf("sandbox_ref = %#v, want daytona-dev", got["sandbox_ref"])
		}
	})

	t.Run("Should include workspace payload sandbox_ref", func(t *testing.T) {
		t.Parallel()

		payload := contract.WorkspacePayload{
			ID:         "ws_alpha",
			RootDir:    "/workspace",
			AddDirs:    []string{},
			Name:       "alpha",
			SandboxRef: "daytona-dev",
			CreatedAt:  time.Date(2026, 4, 7, 10, 30, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2026, 4, 7, 11, 30, 0, 0, time.UTC),
		}

		var got map[string]any
		marshalJSON(t, payload, &got)

		if got["sandbox_ref"] != "daytona-dev" {
			t.Fatalf("sandbox_ref = %#v, want daytona-dev", got["sandbox_ref"])
		}
	})
}

func TestAgentEventPayloadRoundTripsThroughJSON(t *testing.T) {
	t.Parallel()

	inputTokens := int64(12)
	event := acp.AgentEvent{
		Type:      acp.EventTypePermission,
		SessionID: "sess-1",
		TurnID:    "turn-1",
		RequestID: "req-1",
		Timestamp: time.Date(2026, 4, 7, 10, 30, 0, 0, time.UTC),
		Action:    "fs/read_text_file",
		Resource:  "/tmp/file.txt",
		Decision:  "pending",
		Error:     "",
		Usage: &acp.TokenUsage{
			TurnID:      "turn-1",
			InputTokens: &inputTokens,
			Timestamp:   time.Date(2026, 4, 7, 10, 30, 1, 0, time.UTC),
		},
		Raw: []byte(`{"ok":true}`),
	}

	payload := core.AgentEventPayloadFromEvent(event)
	var roundTrip contract.AgentEventPayload
	marshalJSON(t, payload, &roundTrip)

	if roundTrip.Type != event.Type || roundTrip.RequestID != event.RequestID || roundTrip.Action != event.Action {
		t.Fatalf("roundTrip payload = %#v", roundTrip)
	}
	if roundTrip.Usage == nil || roundTrip.Usage.InputTokens == nil || *roundTrip.Usage.InputTokens != inputTokens {
		t.Fatalf("usage payload = %#v", roundTrip.Usage)
	}
	if string(roundTrip.Raw) != `{"ok":true}` {
		t.Fatalf("raw payload = %s", string(roundTrip.Raw))
	}
}

func TestAutomationJobPayloadJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve automation job JSON shape", func(t *testing.T) {
		t.Parallel()

		nextRun := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
		liveMode := participation.ModeLive
		namedStrategy := participation.StrategyNamed
		channelID := "ops-automation"
		payload := contract.JobPayload{
			ID:          "job-1",
			Scope:       automationpkg.AutomationScopeWorkspace,
			Name:        "nightly-review",
			AgentName:   "coder",
			WorkspaceID: "ws-alpha",
			Prompt:      "review repo",
			Schedule: &automationpkg.ScheduleSpec{
				Mode:                automationpkg.ScheduleModeEvery,
				Interval:            "1h",
				CatchUpPolicy:       automationpkg.SchedulerCatchUpPolicyRunOnce,
				MisfireGraceSeconds: 120,
			},
			Task: &automationpkg.JobTaskConfig{
				Title: "Review findings",
				Owner: &taskpkg.Ownership{
					Kind: taskpkg.OwnerKindAutomation,
					Ref:  "rule:nightly-review",
				},
				NetworkParticipation: &participation.Request{
					Mode:            &liveMode,
					ChannelStrategy: &namedStrategy,
					ChannelID:       &channelID,
				},
			},
			Enabled: true,
			Retry: automationpkg.RetryConfig{
				Strategy:   automationpkg.RetryStrategyBackoff,
				MaxRetries: 2,
				BaseDelay:  "1m",
			},
			FireLimit: automationpkg.FireLimitConfig{
				Max:    5,
				Window: "24h",
			},
			Source:    automationpkg.JobSourceDynamic,
			CreatedAt: time.Date(2026, 4, 11, 11, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 11, 11, 30, 0, 0, time.UTC),
			NextRun:   &nextRun,
		}

		var got map[string]any
		marshalJSON(t, payload, &got)

		if got["scope"] != string(automationpkg.AutomationScopeWorkspace) {
			t.Fatalf("scope = %#v, want %q", got["scope"], automationpkg.AutomationScopeWorkspace)
		}
		if got["workspace_id"] != "ws-alpha" {
			t.Fatalf("workspace_id = %#v, want %q", got["workspace_id"], "ws-alpha")
		}
		if got["source"] != string(automationpkg.JobSourceDynamic) {
			t.Fatalf("source = %#v, want %q", got["source"], automationpkg.JobSourceDynamic)
		}
		scheduleValue, ok := got["schedule"].(map[string]any)
		if !ok ||
			scheduleValue["catch_up_policy"] != string(automationpkg.SchedulerCatchUpPolicyRunOnce) ||
			scheduleValue["misfire_grace_seconds"] != float64(120) {
			t.Fatalf("schedule = %#v, want run once catch up with 120 second grace", got["schedule"])
		}
		taskValue, ok := got["task"].(map[string]any)
		if !ok || taskValue["title"] != "Review findings" {
			t.Fatalf("task = %#v, want populated task config", got["task"])
		}
		participationValue, ok := taskValue["network_participation"].(map[string]any)
		if !ok ||
			participationValue["mode"] != string(participation.ModeLive) ||
			participationValue["channel_strategy"] != string(participation.StrategyNamed) ||
			participationValue["channel_id"] != channelID {
			t.Fatalf("task.network_participation = %#v, want live named channel %q", participationValue, channelID)
		}
		if _, exists := taskValue["network_channel"]; exists {
			t.Fatalf("task contains removed network_channel: %#v", taskValue)
		}
		if _, exists := got["next_run"]; !exists {
			t.Fatalf("job payload missing next_run: %#v", got)
		}
	})
}

func TestAutomationTriggerPayloadJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve automation trigger JSON shape", func(t *testing.T) {
		t.Parallel()

		payload := contract.TriggerPayload{
			ID:                   "trigger-1",
			Scope:                automationpkg.AutomationScopeWorkspace,
			Name:                 "deploy-review",
			AgentName:            "coder",
			WorkspaceID:          "ws-alpha",
			Prompt:               `review {{ index .Data "payload" }}`,
			Event:                "webhook",
			Filter:               map[string]string{"branch": "main"},
			Enabled:              true,
			Retry:                automationpkg.DefaultRetryConfig(),
			FireLimit:            automationpkg.DefaultFireLimitConfig(),
			Source:               automationpkg.JobSourceDynamic,
			WebhookID:            "wbh_123",
			EndpointSlug:         "deploy-review",
			WebhookSecretPresent: true,
			WebhookSecretHash:    "sha256:redacted",
			CreatedAt:            time.Date(2026, 4, 11, 11, 0, 0, 0, time.UTC),
			UpdatedAt:            time.Date(2026, 4, 11, 11, 30, 0, 0, time.UTC),
		}

		var got map[string]any
		marshalJSON(t, payload, &got)

		if got["scope"] != string(automationpkg.AutomationScopeWorkspace) {
			t.Fatalf("scope = %#v, want %q", got["scope"], automationpkg.AutomationScopeWorkspace)
		}
		if got["workspace_id"] != "ws-alpha" {
			t.Fatalf("workspace_id = %#v, want %q", got["workspace_id"], "ws-alpha")
		}
		if got["source"] != string(automationpkg.JobSourceDynamic) {
			t.Fatalf("source = %#v, want %q", got["source"], automationpkg.JobSourceDynamic)
		}
		if got["endpoint_slug"] != "deploy-review" {
			t.Fatalf("endpoint_slug = %#v, want %q", got["endpoint_slug"], "deploy-review")
		}
		if got["webhook_id"] != "wbh_123" {
			t.Fatalf("webhook_id = %#v, want %q", got["webhook_id"], "wbh_123")
		}
		if got["webhook_secret_present"] != true || got["webhook_secret_hash"] != "sha256:redacted" {
			t.Fatalf(
				"webhook secret metadata = %#v/%#v, want redacted metadata",
				got["webhook_secret_present"],
				got["webhook_secret_hash"],
			)
		}
		if _, exists := got["webhook_secret_ref"]; exists {
			t.Fatalf("trigger payload includes webhook_secret_ref: %#v", got)
		}
	})

	t.Run("Should derive redacted webhook secret metadata from internal triggers", func(t *testing.T) {
		t.Parallel()

		sourceFilter := map[string]string{"branch": "main"}
		payload := contract.TriggerPayloadFromTrigger(automationpkg.Trigger{
			ID:               "trigger-1",
			Scope:            automationpkg.AutomationScopeWorkspace,
			Name:             "deploy-review",
			AgentName:        "coder",
			WorkspaceID:      "ws-alpha",
			Prompt:           `review {{ index .Data "payload" }}`,
			Event:            "webhook",
			Filter:           sourceFilter,
			Enabled:          true,
			Retry:            automationpkg.DefaultRetryConfig(),
			FireLimit:        automationpkg.DefaultFireLimitConfig(),
			Source:           automationpkg.JobSourceDynamic,
			WebhookID:        "wbh_123",
			EndpointSlug:     "deploy-review",
			WebhookSecretRef: "vault:automation/triggers/deploy-review/webhook-secret",
			CreatedAt:        time.Date(2026, 4, 11, 11, 0, 0, 0, time.UTC),
			UpdatedAt:        time.Date(2026, 4, 11, 11, 30, 0, 0, time.UTC),
		})
		sourceFilter["branch"] = "mutated"

		if !payload.WebhookSecretPresent || !strings.HasPrefix(payload.WebhookSecretHash, "sha256:") {
			t.Fatalf("webhook secret metadata = %#v, want redacted metadata", payload)
		}
		if got, want := payload.Filter["branch"], "main"; got != want {
			t.Fatalf("payload.Filter[branch] = %q, want %q", got, want)
		}
	})
}

func TestAutomationUpdateRequestsHasChanges(t *testing.T) {
	t.Parallel()

	name := "updated"
	prompt := "updated prompt"
	schedule := automationpkg.ScheduleSpec{
		Mode:     automationpkg.ScheduleModeEvery,
		Interval: "1h",
	}
	retry := automationpkg.RetryConfig{
		Strategy:   automationpkg.RetryStrategyBackoff,
		MaxRetries: 2,
		BaseDelay:  "1m",
	}
	fireLimit := automationpkg.FireLimitConfig{
		Max:    3,
		Window: "15m",
	}
	event := "session.created"
	filter := map[string]string{"kind": "session"}
	webhookID := "wbh_123"
	endpointSlug := "deploy-review"
	secret := "secret"
	disabled := false

	t.Run("Should report changes for automation job update requests", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name string
			req  contract.UpdateJobRequest
			want bool
		}{
			{
				name: "Should return false for an empty job update",
				req:  contract.UpdateJobRequest{},
				want: false,
			},
			{
				name: "Should return true when the job name is set",
				req:  contract.UpdateJobRequest{Name: &name},
				want: true,
			},
			{
				name: "Should return true when the job prompt is set",
				req:  contract.UpdateJobRequest{Prompt: &prompt},
				want: true,
			},
			{
				name: "Should return true when the job schedule is set",
				req:  contract.UpdateJobRequest{Schedule: &schedule},
				want: true,
			},
			{
				name: "Should return true when the job enabled flag is set",
				req:  contract.UpdateJobRequest{Enabled: &disabled},
				want: true,
			},
			{
				name: "Should return true when the job retry policy is set",
				req:  contract.UpdateJobRequest{Retry: &retry},
				want: true,
			},
			{
				name: "Should return true when the job fire limit is set",
				req:  contract.UpdateJobRequest{FireLimit: &fireLimit},
				want: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if got := tc.req.HasChanges(); got != tc.want {
					t.Fatalf("UpdateJobRequest.HasChanges() = %v, want %v", got, tc.want)
				}
			})
		}
	})

	t.Run("Should report changes for automation trigger update requests", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name string
			req  contract.UpdateTriggerRequest
			want bool
		}{
			{
				name: "Should return false for an empty trigger update",
				req:  contract.UpdateTriggerRequest{},
				want: false,
			},
			{
				name: "Should return true when the trigger name is set",
				req:  contract.UpdateTriggerRequest{Name: &name},
				want: true,
			},
			{
				name: "Should return true when the trigger prompt is set",
				req:  contract.UpdateTriggerRequest{Prompt: &prompt},
				want: true,
			},
			{
				name: "Should return true when the trigger event is set",
				req:  contract.UpdateTriggerRequest{Event: &event},
				want: true,
			},
			{
				name: "Should return true when the trigger filter is set",
				req:  contract.UpdateTriggerRequest{Filter: filter},
				want: true,
			},
			{
				name: "Should return true when the webhook secret value is set",
				req:  contract.UpdateTriggerRequest{WebhookSecretValue: &secret},
				want: true,
			},
			{
				name: "Should return true when the trigger enabled flag is set",
				req:  contract.UpdateTriggerRequest{Enabled: &disabled},
				want: true,
			},
			{
				name: "Should return true when the trigger retry policy is set",
				req:  contract.UpdateTriggerRequest{Retry: &retry},
				want: true,
			},
			{
				name: "Should return true when the trigger fire limit is set",
				req:  contract.UpdateTriggerRequest{FireLimit: &fireLimit},
				want: true,
			},
			{
				name: "Should return true when the trigger webhook id is set",
				req:  contract.UpdateTriggerRequest{WebhookID: &webhookID},
				want: true,
			},
			{
				name: "Should return true when the trigger endpoint slug is set",
				req:  contract.UpdateTriggerRequest{EndpointSlug: &endpointSlug},
				want: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if got := tc.req.HasChanges(); got != tc.want {
					t.Fatalf("UpdateTriggerRequest.HasChanges() = %v, want %v", got, tc.want)
				}
			})
		}
	})
}

func TestTaskPayloadJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should marshal task payload JSON shape", func(t *testing.T) {
		t.Parallel()

		payload := contract.TaskPayload{
			ID:           "task-1",
			Identifier:   "TASK-1",
			Scope:        taskpkg.ScopeWorkspace,
			WorkspaceID:  "ws-alpha",
			ParentTaskID: "task-root",
			ResolvedNetworkParticipation: &participation.Spec{
				Version:   participation.SpecVersion,
				Mode:      participation.ModeLive,
				ChannelID: "builders",
				Source:    participation.SourceExplicitRequest,
			},
			Title:       "Review task",
			Description: "Check the API layer",
			Status:      taskpkg.TaskStatusInProgress,
			Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "reviewers"},
			CreatedBy:   taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
			Origin:      taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.create"},
			CreatedAt:   time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 4, 14, 10, 5, 0, 0, time.UTC),
			Metadata:    json.RawMessage(`{"priority":"high"}`),
		}

		var got map[string]any
		marshalJSON(t, payload, &got)

		if got["workspace_id"] != "ws-alpha" || resolvedChannelFromJSON(got) != "builders" {
			t.Fatalf("task JSON = %#v", got)
		}
		createdBy, ok := got["created_by"].(map[string]any)
		if !ok || createdBy["kind"] != string(taskpkg.ActorKindHuman) || createdBy["ref"] != "local-user" {
			t.Fatalf("created_by JSON = %#v", got["created_by"])
		}
		origin, ok := got["origin"].(map[string]any)
		if !ok || origin["kind"] != string(taskpkg.OriginKindHTTP) || origin["ref"] != "tasks.create" {
			t.Fatalf("origin JSON = %#v", got["origin"])
		}
		owner, ok := got["owner"].(map[string]any)
		if !ok || owner["kind"] != string(taskpkg.OwnerKindPool) || owner["ref"] != "reviewers" {
			t.Fatalf("owner JSON = %#v", got["owner"])
		}
		if _, exists := got["metadata"]; !exists {
			t.Fatalf("task JSON missing metadata: %#v", got)
		}
	})

	t.Run("Should omit zero-valued optional task timestamps", func(t *testing.T) {
		t.Parallel()

		payload := contract.TaskPayload{
			ID:        "task-1",
			Scope:     taskpkg.ScopeGlobal,
			Title:     "Review task",
			Status:    taskpkg.TaskStatusReady,
			CreatedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.create"},
			CreatedAt: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 14, 10, 5, 0, 0, time.UTC),
		}

		var got map[string]any
		marshalJSON(t, payload, &got)

		if _, exists := got["closed_at"]; exists {
			t.Fatalf("task JSON unexpectedly included closed_at: %#v", got)
		}
	})
}

func TestTaskRunPayloadJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should marshal task run payload JSON shape", func(t *testing.T) {
		t.Parallel()

		startedAt := time.Date(2026, 4, 14, 10, 1, 0, 0, time.UTC)
		payload := contract.TaskRunPayload{
			ID:             "run-1",
			TaskID:         "task-1",
			Status:         taskpkg.TaskRunStatusRunning,
			Attempt:        2,
			ClaimedBy:      &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
			SessionID:      "sess-1",
			Origin:         taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.start_run"},
			IdempotencyKey: "key-1",
			ResolvedNetworkParticipation: &participation.Spec{
				Version:   participation.SpecVersion,
				Mode:      participation.ModeLive,
				ChannelID: "builders",
				Source:    participation.SourceExplicitRequest,
			},
			QueuedAt:  time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
			StartedAt: &startedAt,
			Result:    json.RawMessage(`{"ok":true}`),
		}

		var got map[string]any
		marshalJSON(t, payload, &got)

		if got["session_id"] != "sess-1" || got["idempotency_key"] != "key-1" {
			t.Fatalf("task run JSON = %#v", got)
		}
		if resolvedChannelFromJSON(got) != "builders" || got["status"] != taskpkg.TaskRunStatusRunning.String() {
			t.Fatalf("task run JSON = %#v", got)
		}
	})

	t.Run("Should omit zero-valued optional run timestamps", func(t *testing.T) {
		t.Parallel()

		payload := contract.TaskRunPayload{
			ID:       "run-1",
			TaskID:   "task-1",
			Status:   taskpkg.TaskRunStatusQueued,
			Attempt:  1,
			Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.enqueue_run"},
			QueuedAt: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
		}

		var got map[string]any
		marshalJSON(t, payload, &got)

		for _, field := range []string{"claimed_at", "started_at", "ended_at"} {
			if _, exists := got[field]; exists {
				t.Fatalf("task run JSON unexpectedly included %s: %#v", field, got)
			}
		}
	})
}

func TestUpdateTaskRequestHasChanges(t *testing.T) {
	t.Parallel()

	title := "updated"
	channel := "builders"
	owner := &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "reviewers"}
	metadata := json.RawMessage(`{"priority":"high"}`)

	testCases := []struct {
		name string
		req  contract.UpdateTaskRequest
		want bool
	}{
		{name: "Should return false when no task changes are set", req: contract.UpdateTaskRequest{}, want: false},
		{name: "Should return true when title is set", req: contract.UpdateTaskRequest{Title: &title}, want: true},
		{
			name: "Should return true when network participation is set",
			req: contract.UpdateTaskRequest{
				NetworkParticipation: &participation.Request{
					ChannelID: &channel,
				},
			},
			want: true,
		},
		{name: "Should return true when owner is set", req: contract.UpdateTaskRequest{Owner: owner}, want: true},
		{
			name: "Should return true when metadata is set",
			req:  contract.UpdateTaskRequest{Metadata: &metadata},
			want: true,
		},
		{
			name: "Should return true when clear owner is set",
			req:  contract.UpdateTaskRequest{ClearOwner: true},
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.req.HasChanges(); got != tc.want {
				t.Fatalf("UpdateTaskRequest.HasChanges() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNetworkPeerPayloadJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize peer-card capabilities as typed brief objects", func(t *testing.T) {
		t.Parallel()

		payload := contract.NetworkPeerPayload{
			PeerID:        "reviewer.sess-a",
			DisplayName:   "Reviewer",
			Channel:       "builders",
			Local:         true,
			PresenceState: contract.NetworkPresenceLocal,
			PeerCard: contract.NetworkPeerCardPayload{
				PeerID: "reviewer.sess-a",
				Capabilities: []contract.NetworkCapabilityBriefPayload{{
					ID:      "review-pr",
					Summary: "Review pull requests",
				}},
				ProfilesSupported:   []string{"agh-network/v0"},
				ArtifactsSupported:  []string{"capability"},
				TrustModesSupported: []string{"untrusted"},
				Ext: map[string]json.RawMessage{
					"agh.workflow_id": json.RawMessage(`"wf-1"`),
				},
			},
		}

		var got map[string]any
		marshalJSON(t, payload, &got)
		if got["presence_state"] != contract.NetworkPresenceLocal {
			t.Fatalf("presence_state = %#v, want local", got["presence_state"])
		}

		peerCard, ok := got["peer_card"].(map[string]any)
		if !ok {
			t.Fatalf("peer_card type = %T, want object", got["peer_card"])
		}
		capabilities, ok := peerCard["capabilities"].([]any)
		if !ok || len(capabilities) != 1 {
			t.Fatalf("peer_card.capabilities = %#v, want one object entry", peerCard["capabilities"])
		}
		firstCapability, ok := capabilities[0].(map[string]any)
		if !ok {
			t.Fatalf("capability entry type = %T, want object", capabilities[0])
		}
		if firstCapability["id"] != "review-pr" || firstCapability["summary"] != "Review pull requests" {
			t.Fatalf("capability brief JSON = %#v", firstCapability)
		}
		if _, isString := capabilities[0].(string); isString {
			t.Fatalf("capability brief JSON should be object, got string: %#v", capabilities[0])
		}
	})
}

func TestNetworkPeerDetailPayloadJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize rich capability catalogs as structured payloads", func(t *testing.T) {
		t.Parallel()

		payload := contract.NetworkPeerDetailPayload{
			PeerID:        "reviewer.sess-a",
			DisplayName:   "Reviewer",
			Channel:       "builders",
			Local:         true,
			PresenceState: contract.NetworkPresenceLocal,
			PeerCard: contract.NetworkPeerCardPayload{
				PeerID: "reviewer.sess-a",
				Capabilities: []contract.NetworkCapabilityBriefPayload{{
					ID:      "review-pr",
					Summary: "Review pull requests",
				}},
				ProfilesSupported:   []string{"agh-network/v0"},
				ArtifactsSupported:  []string{"capability"},
				TrustModesSupported: []string{"untrusted"},
			},
			CapabilityCatalog: &contract.NetworkCapabilityCatalogPayload{
				Capabilities: []contract.NetworkCapabilityPayload{{
					ID:                "review-pr",
					Summary:           "Review pull requests",
					Outcome:           "Actionable review findings",
					Version:           "1.0.0",
					Digest:            "sha256:review-pr-v1",
					ContextNeeded:     []string{"pull request link"},
					ArtifactsExpected: []string{"review summary"},
					Requirements:      []string{"workspace-read"},
				}},
			},
		}

		var got map[string]any
		marshalJSON(t, payload, &got)
		if got["presence_state"] != contract.NetworkPresenceLocal {
			t.Fatalf("detail presence_state = %#v, want local", got["presence_state"])
		}
		if _, exists := got["last_seen_age_seconds"]; exists {
			t.Fatalf("detail should omit local last_seen_age_seconds: %#v", got)
		}

		catalog, ok := got["capability_catalog"].(map[string]any)
		if !ok {
			t.Fatalf("capability_catalog type = %T, want object", got["capability_catalog"])
		}
		capabilities, ok := catalog["capabilities"].([]any)
		if !ok || len(capabilities) != 1 {
			t.Fatalf("capability_catalog.capabilities = %#v, want one object entry", catalog["capabilities"])
		}
		firstCapability, ok := capabilities[0].(map[string]any)
		if !ok {
			t.Fatalf("rich capability entry type = %T, want object", capabilities[0])
		}
		if firstCapability["digest"] != "sha256:review-pr-v1" ||
			firstCapability["outcome"] != "Actionable review findings" {
			t.Fatalf("rich capability JSON = %#v", firstCapability)
		}
		requirements, ok := firstCapability["requirements"].([]any)
		if !ok || len(requirements) != 1 || requirements[0] != "workspace-read" {
			t.Fatalf("requirements JSON = %#v, want workspace-read", firstCapability["requirements"])
		}
	})
}

func marshalJSON[T any](t *testing.T, value any, target *T) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}

func assertZeroMetricField(t *testing.T, payload map[string]any, field string) {
	t.Helper()

	value, exists := payload[field]
	if !exists {
		t.Fatalf("payload missing zero metric field %q: %#v", field, payload)
	}
	if value != float64(0) {
		t.Fatalf("payload[%q] = %#v, want JSON zero", field, value)
	}
}

func assertJSONFieldAbsent(t *testing.T, payload map[string]any, field string) {
	t.Helper()

	if _, exists := payload[field]; exists {
		t.Fatalf("payload should not include %q: %#v", field, payload)
	}
}
