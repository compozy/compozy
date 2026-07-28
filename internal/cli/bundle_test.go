package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBundleCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should activate bundle presets through the daemon client", func(t *testing.T) {
		t.Parallel()

		var captured ActivateBundleRequest
		deps := newTestDeps(t, &stubClient{
			activateBundleFn: func(_ context.Context, request ActivateBundleRequest) (BundleActivationRecord, error) {
				captured = request
				return sampleBundleActivationRecord(), nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"bundle",
			"activate",
			"--extension",
			"marketing-team",
			"--bundle",
			"marketing",
			"--profile",
			"default",
			"--scope",
			"workspace",
			"--workspace",
			"ws-marketing",
			"--confirm-network-requirement",
			"--json",
		)
		if err != nil {
			t.Fatalf("bundle activate error = %v", err)
		}
		if captured.ExtensionName != "marketing-team" ||
			captured.BundleName != "marketing" ||
			captured.ProfileName != "default" ||
			captured.Scope != "workspace" ||
			captured.Workspace != "ws-marketing" ||
			!captured.ConfirmNetworkRequirement {
			t.Fatalf("captured request = %#v", captured)
		}

		var payload BundleActivationRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(stdout) error = %v\nstdout=%s", err, stdout)
		}
		if got, want := len(payload.Agents), 1; got != want {
			t.Fatalf("len(payload.Agents) = %d, want %d", got, want)
		}
		if got, want := len(payload.Layouts), 1; got != want {
			t.Fatalf("len(payload.Layouts) = %d, want %d", got, want)
		}
		if !payload.Agents[0].HasSoul || !payload.Agents[0].HasHeartbeat {
			t.Fatalf("payload.Agents[0] sidecar flags = %#v", payload.Agents[0])
		}
		if !strings.Contains(stdout, `"resource_kind": "agent.soul"`) {
			t.Fatalf("stdout = %s, want agent.soul inventory", stdout)
		}
	})

	t.Run("Should preview bundle presets without activation", func(t *testing.T) {
		t.Parallel()

		var captured ActivateBundleRequest
		deps := newTestDeps(t, &stubClient{
			previewBundleActivationFn: func(
				_ context.Context,
				request ActivateBundleRequest,
			) (BundleActivationRecord, error) {
				captured = request
				item := sampleBundleActivationRecord()
				item.ID = "preview_marketing"
				return item, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"bundle",
			"preview",
			"--extension",
			"marketing-team",
			"--bundle",
			"marketing",
			"--profile",
			"default",
			"--scope",
			"global",
			"--json",
		)
		if err != nil {
			t.Fatalf("bundle preview error = %v", err)
		}
		if captured.ExtensionName != "marketing-team" ||
			captured.BundleName != "marketing" ||
			captured.ProfileName != "default" ||
			captured.Scope != "global" ||
			captured.ConfirmNetworkRequirement {
			t.Fatalf("captured preview request = %#v", captured)
		}

		var payload BundleActivationRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(stdout) error = %v\nstdout=%s", err, stdout)
		}
		if got, want := payload.ID, "preview_marketing"; got != want {
			t.Fatalf("payload.ID = %q, want %q", got, want)
		}
		if got, want := len(payload.Inventory), 4; got != want {
			t.Fatalf("len(payload.Inventory) = %d, want %d", got, want)
		}
	})

	t.Run("Should list catalog profiles with agent counts", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			listBundleCatalogFn: func(context.Context) ([]BundleCatalogRecord, error) {
				return []BundleCatalogRecord{{
					ExtensionName: "marketing-team",
					BundleName:    "marketing",
					Profiles: []BundleProfileCatalogRecord{{
						Name:        "default",
						LayoutCount: 1,
						AgentCount:  1,
						JobCount:    1,
					}},
				}}, nil
			},
		})
		stdout, _, err := executeRootCommand(t, deps, "bundle", "catalog", "--json")
		if err != nil {
			t.Fatalf("bundle catalog error = %v", err)
		}
		if !strings.Contains(stdout, `"agent_count": 1`) {
			t.Fatalf("stdout = %s, want agent_count", stdout)
		}
		if !strings.Contains(stdout, `"layout_count": 1`) {
			t.Fatalf("stdout = %s, want layout_count", stdout)
		}
	})

	t.Run("Should list active bundle activations", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			listBundleActivationsFn: func(context.Context) ([]BundleActivationRecord, error) {
				item := sampleBundleActivationRecord()
				item.SpecDrift = true
				return []BundleActivationRecord{item}, nil
			},
		})
		stdout, _, err := executeRootCommand(t, deps, "bundle", "list", "--json")
		if err != nil {
			t.Fatalf("bundle list error = %v", err)
		}

		var payload struct {
			Activations []BundleActivationRecord `json:"activations"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(stdout) error = %v\nstdout=%s", err, stdout)
		}
		if got, want := len(payload.Activations), 1; got != want {
			t.Fatalf("len(payload.Activations) = %d, want %d", got, want)
		}
		if got, want := len(payload.Activations[0].Agents), 1; got != want {
			t.Fatalf("len(payload.Activations[0].Agents) = %d, want %d", got, want)
		}
		if got, want := len(payload.Activations[0].Layouts), 1; got != want {
			t.Fatalf("len(payload.Activations[0].Layouts) = %d, want %d", got, want)
		}
		if !payload.Activations[0].SpecDrift {
			t.Fatal("payload.Activations[0].SpecDrift = false, want true")
		}
	})

	t.Run("Should get one bundle activation by id", func(t *testing.T) {
		t.Parallel()

		var capturedID string
		deps := newTestDeps(t, &stubClient{
			getBundleActivationFn: func(_ context.Context, id string) (BundleActivationRecord, error) {
				capturedID = id
				return sampleBundleActivationRecord(), nil
			},
		})
		stdout, _, err := executeRootCommand(t, deps, "bundle", "get", "act_marketing", "--json")
		if err != nil {
			t.Fatalf("bundle get error = %v", err)
		}
		if capturedID != "act_marketing" {
			t.Fatalf("captured activation id = %q, want act_marketing", capturedID)
		}

		var payload BundleActivationRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(stdout) error = %v\nstdout=%s", err, stdout)
		}
		if got, want := payload.ID, "act_marketing"; got != want {
			t.Fatalf("payload.ID = %q, want %q", got, want)
		}
		human, _, err := executeRootCommand(t, deps, "bundle", "get", "act_marketing")
		if err != nil {
			t.Fatalf("bundle get human error = %v", err)
		}
		if !strings.Contains(human, "Layouts") || !strings.Contains(human, "Marketing two-up") {
			t.Fatalf("human output = %q, want window layout section", human)
		}
	})

	t.Run("Should confirm network requirement on bundle update", func(t *testing.T) {
		t.Parallel()

		var captured UpdateBundleActivationRequest
		deps := newTestDeps(t, &stubClient{
			updateBundleActivationFn: func(
				_ context.Context,
				id string,
				request UpdateBundleActivationRequest,
			) (BundleActivationRecord, error) {
				if id != "act_marketing" {
					t.Fatalf("activation id = %q, want act_marketing", id)
				}
				captured = request
				item := sampleBundleActivationRecord()
				item.NetworkRequirementConfirmedBy = "operator"
				return item, nil
			},
		})
		if _, _, err := executeRootCommand(
			t,
			deps,
			"bundle",
			"update",
			"act_marketing",
			"--confirm-network-requirement",
			"--expected-version",
			"7",
			"--json",
		); err != nil {
			t.Fatalf("bundle update confirm error = %v", err)
		}
		if !captured.ConfirmNetworkRequirement || captured.ExpectedVersion != 7 {
			t.Fatalf("captured request = %#v, want confirmation at version 7", captured)
		}
	})

	t.Run("Should require expected-version before calling the client", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		_, _, err := executeRootCommand(
			t,
			deps,
			"bundle",
			"update",
			"act_marketing",
			"--confirm-network-requirement",
		)
		if err == nil || !strings.Contains(err.Error(), "--expected-version") {
			t.Fatalf("bundle update error = %v, want expected-version guidance", err)
		}
	})

	t.Run("Should reapply bundle spec without confirming an unchanged network requirement", func(t *testing.T) {
		t.Parallel()

		var captured UpdateBundleActivationRequest
		deps := newTestDeps(t, &stubClient{
			updateBundleActivationFn: func(
				_ context.Context,
				_ string,
				request UpdateBundleActivationRequest,
			) (BundleActivationRecord, error) {
				captured = request
				return sampleBundleActivationRecord(), nil
			},
		})
		if _, _, err := executeRootCommand(
			t,
			deps,
			"bundle",
			"update",
			"act_marketing",
			"--expected-version",
			"7",
		); err != nil {
			t.Fatalf("bundle update reapply error = %v", err)
		}
		if captured.ConfirmNetworkRequirement || captured.ExpectedVersion != 7 {
			t.Fatalf("captured request = %#v, want reapply at version 7 without confirmation", captured)
		}
	})

	t.Run("Should deactivate one bundle activation by id", func(t *testing.T) {
		t.Parallel()

		var capturedID string
		deps := newTestDeps(t, &stubClient{
			deactivateBundleFn: func(_ context.Context, id string) error {
				capturedID = id
				return nil
			},
		})
		stdout, _, err := executeRootCommand(t, deps, "bundle", "deactivate", "act_marketing", "--json")
		if err != nil {
			t.Fatalf("bundle deactivate error = %v", err)
		}
		if capturedID != "act_marketing" {
			t.Fatalf("captured activation id = %q, want act_marketing", capturedID)
		}

		var payload map[string]string
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(stdout) error = %v\nstdout=%s", err, stdout)
		}
		if got, want := payload["deactivated"], "act_marketing"; got != want {
			t.Fatalf("payload[deactivated] = %q, want %q", got, want)
		}
	})

	t.Run("Should expose bundle network settings", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			bundleNetworkSettingsFn: func(context.Context) (BundleNetworkSettingsRecord, error) {
				return BundleNetworkSettingsRecord{
					DeclaredChannels: []DeclaredNetworkChannelRecord{{
						ActivationID: "act_marketing",
						Name:         "marketing",
						Primary:      true,
					}},
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(t, deps, "bundle", "network-settings", "-o", "toon")
		if err != nil {
			t.Fatalf("bundle network-settings error = %v", err)
		}
		if !strings.Contains(stdout, "declared_channels") || !strings.Contains(stdout, "marketing") {
			t.Fatalf("stdout = %q, want declared channels toon output", stdout)
		}
	})
}

func sampleBundleActivationRecord() BundleActivationRecord {
	return BundleActivationRecord{
		ID:                 "act_marketing",
		Version:            7,
		ExtensionName:      "marketing-team",
		BundleName:         "marketing",
		BundleDescription:  "Marketing team bundle",
		ProfileName:        "default",
		ProfileDescription: "Default profile",
		Scope:              "workspace",
		WorkspaceID:        "ws-marketing",
		Channels: []BundleChannelRecord{{
			Name:        "marketing",
			Description: "Marketing coordination",
			Primary:     true,
		}},
		Layouts: []BundleLayoutRecord{{
			ID:               "lay_two_up",
			DisplayName:      "Marketing two-up",
			AspectVariant:    "landscape",
			ParticipantSlots: []string{"marketer", "research"},
			OverflowPolicy:   "stack",
		}},
		Agents: []BundleAgentRecord{{
			ID:           "agt_marketer",
			Name:         "marketer",
			Provider:     "claude",
			Model:        "sonnet",
			HasSoul:      true,
			HasHeartbeat: true,
		}},
		Jobs: []BundleJobRecord{{
			ID:        "job_daily",
			Name:      "daily-sync",
			AgentName: "marketer",
			Enabled:   true,
		}},
		Triggers: []BundleTriggerRecord{{
			ID:        "trg_session",
			Name:      "session-opened",
			AgentName: "marketer",
			Event:     "session.created",
			Enabled:   true,
		}},
		Bridges: []BundleBridgeRecord{{
			ID:            "bri_linear",
			Name:          "linear-main",
			ExtensionName: "linear",
			Platform:      "linear",
			DisplayName:   "Linear",
		}},
		Inventory: []BundleInventoryRecord{
			{ResourceKind: "window_layout", ResourceID: "lay_two_up", ResourceName: "Marketing two-up"},
			{ResourceKind: "agent", ResourceID: "agt_marketer", ResourceName: "marketer"},
			{ResourceKind: "agent.soul", ResourceID: "sol_marketer", ResourceName: "marketer"},
			{ResourceKind: "agent.heartbeat", ResourceID: "hbt_marketer", ResourceName: "marketer"},
		},
		CreatedAt: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC),
	}
}
