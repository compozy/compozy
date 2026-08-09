package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

func TestGlobalDBGatewayTierKeysSurviveReopen(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve both tiers and enforce one active provider per tier [UT-161]", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		provider := gateway.ProviderIdentity{
			Name: "connectivity-test", InstallSource: "bundled",
			DigestConfirmed: "sha256:test", ConfirmedAt: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC),
		}
		for _, tier := range []gateway.Tier{gateway.TierPrivate, gateway.TierPublic} {
			changed, transitionErr := first.Transition(ctx, gateway.TransitionRequest{
				Target: gateway.TargetProvider, Tier: tier, Desired: gateway.DesiredEnabled, Provider: provider,
			}, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
			if transitionErr != nil || !changed {
				t.Fatalf("Transition(provider %s) = (%t, %v), want changed", tier, changed, transitionErr)
			}
			changed, transitionErr = first.Transition(ctx, gateway.TransitionRequest{
				Target: gateway.TargetSurface, Tier: tier, Surface: gateway.SurfaceOperatorUI,
				Desired: gateway.DesiredEnabled, Consent: tier == gateway.TierPublic,
			}, time.Date(2026, 8, 6, 12, 1, 0, 0, time.UTC))
			if transitionErr != nil || !changed {
				t.Fatalf("Transition(surface %s) = (%t, %v), want changed", tier, changed, transitionErr)
			}
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopened) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(ctx); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		snapshot, err := reopened.Snapshot(ctx)
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		if len(snapshot.Providers) != 2 || len(snapshot.Surfaces) != 2 {
			t.Fatalf("reopened snapshot = %#v, want two provider and two surface tier keys", snapshot)
		}
		for _, tier := range []gateway.Tier{gateway.TierPrivate, gateway.TierPublic} {
			assertGatewayTierKey(t, snapshot, tier)
		}

		changed, err := reopened.Transition(ctx, gateway.TransitionRequest{
			Target: gateway.TargetProvider, Tier: gateway.TierPrivate,
			Desired: gateway.DesiredDisabled, ExpectedGeneration: 1,
			Provider: gateway.ProviderIdentity{Name: "connectivity-test"},
		}, time.Date(2026, 8, 6, 12, 2, 0, 0, time.UTC))
		if err != nil || !changed {
			t.Fatalf("Transition(disable provider) = (%t, %v), want changed", changed, err)
		}
		disabledSnapshot, err := reopened.Snapshot(ctx)
		if err != nil {
			t.Fatalf("Snapshot(disabled provider) error = %v", err)
		}
		assertGatewayProviderState(
			t,
			disabledSnapshot,
			gateway.TierPrivate,
			gateway.DesiredDisabled,
			2,
		)

		_, err = reopened.Transition(ctx, gateway.TransitionRequest{
			Target: gateway.TargetProvider, Tier: gateway.TierPublic, Desired: gateway.DesiredEnabled,
			Provider: gateway.ProviderIdentity{Name: "second-provider", InstallSource: "marketplace"},
		}, time.Date(2026, 8, 6, 12, 3, 0, 0, time.UTC))
		if !errors.Is(err, gateway.ErrTierProviderConflict) {
			t.Fatalf("Transition(second provider) error = %v, want ErrTierProviderConflict", err)
		}

		for _, table := range []string{
			"gateway_device_sessions",
			"gateway_providers",
			"gateway_provider_activations",
			"gateway_surface_exposure",
			"gateway_endpoint_state",
		} {
			columns, columnsErr := tableColumns(ctx, reopened.db, table)
			if columnsErr != nil {
				t.Fatalf("tableColumns(%q) error = %v", table, columnsErr)
			}
			if _, exists := columns["workspace_id"]; exists {
				t.Fatalf("operator-global table %q contains workspace_id", table)
			}
		}
		var indexSQL string
		if err := reopened.db.QueryRowContext(
			ctx,
			`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'gateway_active_provider_per_tier'`,
		).Scan(&indexSQL); err != nil {
			t.Fatalf("query partial gateway provider index: %v", err)
		}
		if !strings.Contains(strings.ToLower(indexSQL), "where desired_state = 'enabled'") {
			t.Fatalf("gateway provider index = %q, want enabled-only partial predicate", indexSQL)
		}
	})
}

func TestGlobalDBGatewayMutationCommitFence(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back surface and ingress writes rejected at commit", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		trigger := automationWebhookTriggerForTest(
			automation.AutomationScopeGlobal,
			"gateway-fenced-webhook",
			"",
			automation.JobSourceDynamic,
		)
		trigger.ID = "trigger-gateway-fenced"
		trigger.WebhookID = "wbh_gateway_fenced"
		trigger.EndpointSlug = "gateway-fenced"
		insertGatewayResourceRecord(ctx, t, database, "automation.trigger", trigger.ID, "global", "", trigger)
		ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: trigger.ID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject() error = %v", err)
		}
		fencedCtx := store.ContextWithMutationCommitFence(ctx, func(context.Context) error {
			return gateway.ErrDeviceRevoked
		})

		if _, err := database.Transition(fencedCtx, gateway.TransitionRequest{
			Target: gateway.TargetSurface, Tier: gateway.TierPublic,
			Surface: gateway.SurfaceOperatorUI, Desired: gateway.DesiredEnabled, Consent: true,
		}, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)); !errors.Is(err, gateway.ErrDeviceRevoked) {
			t.Fatalf("Transition(fenced) error = %v, want ErrDeviceRevoked", err)
		}
		if _, _, err := database.SyncIngressEndpoint(
			fencedCtx,
			gateway.TierPublic,
			"https://gateway.example.test",
			time.Date(2026, 8, 6, 12, 1, 0, 0, time.UTC),
		); !errors.Is(err, gateway.ErrDeviceRevoked) {
			t.Fatalf("SyncIngressEndpoint(fenced) error = %v, want ErrDeviceRevoked", err)
		}
		if _, _, err := database.PutIngressBinding(
			fencedCtx,
			subject,
			1,
			time.Date(2026, 8, 6, 12, 2, 0, 0, time.UTC),
		); !errors.Is(err, gateway.ErrDeviceRevoked) {
			t.Fatalf("PutIngressBinding(fenced) error = %v, want ErrDeviceRevoked", err)
		}

		snapshot, err := database.Snapshot(ctx)
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		if len(snapshot.Surfaces) != 0 {
			t.Fatalf("snapshot surfaces = %#v, want rollback", snapshot.Surfaces)
		}
		if _, err := database.GetIngressEndpoint(ctx, gateway.TierPublic); !errors.Is(
			err,
			gateway.ErrEndpointUnverified,
		) {
			t.Fatalf("GetIngressEndpoint() error = %v, want ErrEndpointUnverified", err)
		}
		assertNoIngressBindingRow(ctx, t, database, ref)
	})
}

func TestGlobalDBGatewayIngressLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve a stable public address across a real database restart [IT-051]", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		at := time.Date(2026, 8, 6, 11, 0, 0, 0, time.UTC)
		endpoint, changed, err := first.SyncIngressEndpoint(
			ctx, gateway.TierPublic, "https://gateway.example.test", at,
		)
		if err != nil || !changed || endpoint.Generation != 1 || !endpoint.Reachable {
			t.Fatalf("SyncIngressEndpoint(first) = (%#v, %t, %v)", endpoint, changed, err)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopened) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(ctx); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		persisted, err := reopened.GetIngressEndpoint(ctx, gateway.TierPublic)
		if err != nil {
			t.Fatalf("GetIngressEndpoint(reopened) error = %v", err)
		}
		if persisted.URL != endpoint.URL || persisted.Generation != 1 || !persisted.Reachable {
			t.Fatalf("reopened endpoint = %#v, want stable generation 1", persisted)
		}
		stable, changed, err := reopened.SyncIngressEndpoint(
			ctx, gateway.TierPublic, endpoint.URL, at.Add(time.Minute),
		)
		if err != nil || changed || stable.Generation != 1 {
			t.Fatalf("SyncIngressEndpoint(reopened stable) = (%#v, %t, %v)", stable, changed, err)
		}
	})

	t.Run("Should preserve generation for a stable address and advance it on change [UT-094]", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		at := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

		first, changed, err := database.SyncIngressEndpoint(
			ctx, gateway.TierPublic, "https://gateway.example.test", at,
		)
		if err != nil || !changed || first.Generation != 1 {
			t.Fatalf("SyncIngressEndpoint(first) = (%#v, %t, %v), want generation 1 changed", first, changed, err)
		}
		stable, changed, err := database.SyncIngressEndpoint(
			ctx, gateway.TierPublic, "https://gateway.example.test", at.Add(time.Minute),
		)
		if err != nil || changed || stable.Generation != 1 {
			t.Fatalf("SyncIngressEndpoint(stable) = (%#v, %t, %v), want generation 1 unchanged", stable, changed, err)
		}
		moved, changed, err := database.SyncIngressEndpoint(
			ctx, gateway.TierPublic, "https://gateway-two.example.test", at.Add(2*time.Minute),
		)
		if err != nil || !changed || moved.Generation != 2 {
			t.Fatalf("SyncIngressEndpoint(changed) = (%#v, %t, %v), want generation 2 changed", moved, changed, err)
		}
	})

	t.Run("Should resolve webhook ownership and delete its binding atomically [UT-095, UT-097]", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		insertGatewayTestWorkspace(ctx, t, database, "workspace-owner")
		trigger := automationWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"gateway-owned-webhook",
			"workspace-owner",
			automation.JobSourceDynamic,
		)
		trigger.ID = "trigger-gateway-owned"
		trigger.WebhookID = "wbh_gateway_owned"
		trigger.EndpointSlug = "gateway-owned"
		created, err := database.CreateTrigger(ctx, trigger)
		if err != nil {
			t.Fatalf("CreateTrigger() error = %v", err)
		}
		ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: created.ID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject() error = %v", err)
		}
		if subject.Scope != gateway.IngressScopeWorkspace || subject.WorkspaceID != "workspace-owner" ||
			!strings.Contains(subject.Path, "/workspaces/workspace-owner/gateway-owned--wbh_gateway_owned") {
			t.Fatalf("resolved subject = %#v, want persisted workspace owner and formatted path", subject)
		}
		if _, changed, err := database.PutIngressBinding(
			ctx, subject, 3, time.Date(2026, 8, 6, 12, 30, 0, 0, time.UTC),
		); err != nil || !changed {
			t.Fatalf("PutIngressBinding() = changed:%t error:%v", changed, err)
		}

		if err := database.DeleteTrigger(ctx, created.ID); err != nil {
			t.Fatalf("DeleteTrigger() error = %v", err)
		}
		assertNoIngressBindingRow(ctx, t, database, ref)
		if _, err := database.GetIngressBinding(ctx, ref); !errors.Is(err, gateway.ErrIngressSubjectNotFound) {
			t.Fatalf("GetIngressBinding(after delete) error = %v, want ErrIngressSubjectNotFound", err)
		}
	})

	t.Run("Should bind canonical trigger resources and invalidate only ingress identity changes", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		insertGatewayTestWorkspace(ctx, t, database, "workspace-resource-trigger")
		trigger := automationWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"gateway-resource-webhook",
			"workspace-resource-trigger",
			automation.JobSourceDynamic,
		)
		trigger.ID = "trigger-gateway-resource"
		trigger.WebhookID = "wbh_gateway_resource"
		trigger.EndpointSlug = "gateway-resource"
		insertGatewayResourceRecord(
			ctx,
			t,
			database,
			"automation.trigger",
			trigger.ID,
			"workspace",
			"workspace-resource-trigger",
			trigger,
		)

		ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: trigger.ID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject(resource trigger) error = %v", err)
		}
		if subject.Scope != gateway.IngressScopeWorkspace ||
			subject.WorkspaceID != "workspace-resource-trigger" ||
			!strings.HasSuffix(subject.Path, "/gateway-resource--wbh_gateway_resource") {
			t.Fatalf("resource trigger subject = %#v", subject)
		}
		if _, changed, err := database.PutIngressBinding(
			ctx,
			subject,
			3,
			time.Date(2026, 8, 6, 15, 30, 0, 0, time.UTC),
		); err != nil || !changed {
			t.Fatalf("PutIngressBinding(resource trigger) = changed:%t error:%v", changed, err)
		}

		trigger.Name = "Renamed without ingress changes"
		updateGatewayResourceRecord(ctx, t, database, "automation.trigger", trigger.ID, trigger)
		if _, err := database.GetIngressBinding(ctx, ref); err != nil {
			t.Fatalf("GetIngressBinding(after display-only resource update) error = %v", err)
		}

		trigger.EndpointSlug = "gateway-resource-moved"
		updateGatewayResourceRecord(ctx, t, database, "automation.trigger", trigger.ID, trigger)
		assertNoIngressBindingRow(ctx, t, database, ref)

		subject, err = database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject(updated resource trigger) error = %v", err)
		}
		if _, _, err := database.PutIngressBinding(
			ctx,
			subject,
			3,
			time.Date(2026, 8, 6, 15, 31, 0, 0, time.UTC),
		); err != nil {
			t.Fatalf("PutIngressBinding(updated resource trigger) error = %v", err)
		}
		deleteGatewayResourceRecord(ctx, t, database, "automation.trigger", trigger.ID)
		assertNoIngressBindingRow(ctx, t, database, ref)
	})

	t.Run("Should bind canonical bridge resources and clear confirmation on target changes", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		bridge := bridges.BridgeInstanceSpec{
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-adapter",
			DisplayName:   "Gateway resource bridge",
			Enabled:       true,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			ProviderConfig: json.RawMessage(
				`{"webhook":{"listen_addr":"127.0.0.1:43128","path":"/telegram"}}`,
			),
		}
		const bridgeID = "bridge-gateway-resource"
		insertGatewayResourceRecord(
			ctx, t, database, "bridge.instance", bridgeID, "global", "", bridge,
		)

		ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectBridgeInstance, ID: bridgeID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject(resource bridge) error = %v", err)
		}
		if _, changed, err := database.PutIngressBinding(
			ctx,
			subject,
			4,
			time.Date(2026, 8, 6, 15, 40, 0, 0, time.UTC),
		); err != nil || !changed {
			t.Fatalf("PutIngressBinding(resource bridge) = changed:%t error:%v", changed, err)
		}

		bridge.DisplayName = "Renamed without target changes"
		updateGatewayResourceRecord(ctx, t, database, "bridge.instance", bridgeID, bridge)
		if _, err := database.GetIngressBinding(ctx, ref); err != nil {
			t.Fatalf("GetIngressBinding(after display-only bridge update) error = %v", err)
		}

		bridge.ProviderConfig = json.RawMessage(
			`{"webhook":{"public_url":"https://external.example.test/telegram"}}`,
		)
		updateGatewayResourceRecord(ctx, t, database, "bridge.instance", bridgeID, bridge)
		assertNoIngressBindingRow(ctx, t, database, ref)

		bridge.ProviderConfig = json.RawMessage(
			`{"webhook":{"listen_addr":"127.0.0.1:43128","path":"/telegram"}}`,
		)
		updateGatewayResourceRecord(ctx, t, database, "bridge.instance", bridgeID, bridge)
		subject, err = database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject(restored resource bridge) error = %v", err)
		}
		if _, _, err := database.PutIngressBinding(
			ctx,
			subject,
			4,
			time.Date(2026, 8, 6, 15, 41, 0, 0, time.UTC),
		); err != nil {
			t.Fatalf("PutIngressBinding(restored resource bridge) error = %v", err)
		}
		deleteGatewayResourceRecord(ctx, t, database, "bridge.instance", bridgeID)
		assertNoIngressBindingRow(ctx, t, database, ref)
	})

	t.Run("Should delete and recreate a bridge without preserving its confirmation [IT-050]", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		insertGatewayTestWorkspace(ctx, t, database, "workspace-owner")
		instance := bridges.BridgeInstance{
			ID:            "bridge-gateway-owned",
			Scope:         bridges.ScopeWorkspace,
			WorkspaceID:   "workspace-owner",
			Platform:      "telegram",
			ExtensionName: "telegram-adapter",
			DisplayName:   "Gateway bridge",
			Enabled:       true,
			Status:        bridges.BridgeStatusReady,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			ProviderConfig: json.RawMessage(
				`{"webhook":{"listen_addr":"127.0.0.1:43123","path":"/telegram"}}`,
			),
		}
		if err := database.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}
		ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectBridgeInstance, ID: instance.ID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject() error = %v", err)
		}
		if _, changed, err := database.PutIngressBinding(
			ctx, subject, 4, time.Date(2026, 8, 6, 13, 0, 0, 0, time.UTC),
		); err != nil || !changed {
			t.Fatalf("PutIngressBinding() = changed:%t error:%v", changed, err)
		}
		if err := database.DeleteBridgeInstance(ctx, instance.ID); err != nil {
			t.Fatalf("DeleteBridgeInstance() error = %v", err)
		}
		assertNoIngressBindingRow(ctx, t, database, ref)

		if err := database.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance(recreate) error = %v", err)
		}
		if _, err := database.GetIngressBinding(ctx, ref); !errors.Is(err, gateway.ErrIngressSubjectNotFound) {
			t.Fatalf("GetIngressBinding(recreated) error = %v, want fresh confirmation", err)
		}
	})

	t.Run("Should invalidate legacy webhook confirmation when delivery identity changes", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name   string
			mutate func(*automation.Trigger)
		}{
			{
				name: "Should invalidate an endpoint slug change",
				mutate: func(trigger *automation.Trigger) {
					trigger.EndpointSlug = "gateway-legacy-moved"
				},
			},
			{
				name: "Should invalidate a webhook id change",
				mutate: func(trigger *automation.Trigger) {
					trigger.WebhookID = "wbh_gateway_legacy_moved"
				},
			},
		}
		for index, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				database := openFreshTestGlobalDB(t)
				ctx := testutil.Context(t)
				trigger := automationWebhookTriggerForTest(
					automation.AutomationScopeGlobal,
					fmt.Sprintf("gateway-legacy-%d", index),
					"",
					automation.JobSourceDynamic,
				)
				trigger.ID = fmt.Sprintf("trigger-gateway-legacy-%d", index)
				trigger.WebhookID = fmt.Sprintf("wbh_gateway_legacy_%d", index)
				trigger.EndpointSlug = fmt.Sprintf("gateway-legacy-%d", index)
				created, err := database.CreateTrigger(ctx, trigger)
				if err != nil {
					t.Fatalf("CreateTrigger() error = %v", err)
				}
				ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: created.ID}
				subject, err := database.ResolveIngressSubject(ctx, ref)
				if err != nil {
					t.Fatalf("ResolveIngressSubject() error = %v", err)
				}
				if _, _, err := database.PutIngressBinding(
					ctx, subject, 1, time.Date(2026, 8, 6, 13, 30, 0, 0, time.UTC),
				); err != nil {
					t.Fatalf("PutIngressBinding() error = %v", err)
				}
				tc.mutate(&created)
				created.UpdatedAt = created.UpdatedAt.Add(time.Minute)
				if _, err := database.UpdateTrigger(ctx, created); err != nil {
					t.Fatalf("UpdateTrigger() error = %v", err)
				}
				assertNoIngressBindingRow(ctx, t, database, ref)
			})
		}
	})

	t.Run("Should invalidate legacy bridge confirmation when the local target changes", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name           string
			providerConfig json.RawMessage
		}{
			{
				name:           "Should invalidate a listen address change",
				providerConfig: json.RawMessage(`{"webhook":{"listen_addr":"127.0.0.1:43126","path":"/telegram"}}`),
			},
			{
				name: "Should invalidate a callback path change",
				providerConfig: json.RawMessage(
					`{"webhook":{"listen_addr":"127.0.0.1:43125","path":"/telegram-moved"}}`,
				),
			},
		}
		for index, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				database := openFreshTestGlobalDB(t)
				ctx := testutil.Context(t)
				instance := bridges.BridgeInstance{
					ID:    fmt.Sprintf("bridge-gateway-target-%d", index),
					Scope: bridges.ScopeGlobal, Platform: "telegram", ExtensionName: "telegram-adapter",
					DisplayName: "Gateway target bridge", Enabled: true, Status: bridges.BridgeStatusReady,
					RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
					ProviderConfig: json.RawMessage(
						`{"webhook":{"listen_addr":"127.0.0.1:43125","path":"/telegram"}}`,
					),
				}
				if err := database.InsertBridgeInstance(ctx, instance); err != nil {
					t.Fatalf("InsertBridgeInstance() error = %v", err)
				}
				ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectBridgeInstance, ID: instance.ID}
				subject, err := database.ResolveIngressSubject(ctx, ref)
				if err != nil {
					t.Fatalf("ResolveIngressSubject() error = %v", err)
				}
				if _, _, err := database.PutIngressBinding(
					ctx, subject, 1, time.Date(2026, 8, 6, 13, 40, 0, 0, time.UTC),
				); err != nil {
					t.Fatalf("PutIngressBinding() error = %v", err)
				}
				instance.ProviderConfig = tc.providerConfig
				instance.UpdatedAt = time.Date(2026, 8, 6, 13, 41, 0, 0, time.UTC)
				if err := database.UpdateBridgeInstance(ctx, instance); err != nil {
					t.Fatalf("UpdateBridgeInstance() error = %v", err)
				}
				assertNoIngressBindingRow(ctx, t, database, ref)
			})
		}
	})

	t.Run("Should invalidate a webhook binding when its workspace owner changes", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		insertGatewayTestWorkspace(ctx, t, database, "workspace-before")
		insertGatewayTestWorkspace(ctx, t, database, "workspace-after")
		trigger := automationWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"gateway-reassigned-webhook",
			"workspace-before",
			automation.JobSourceDynamic,
		)
		trigger.ID = "trigger-gateway-reassigned"
		trigger.WebhookID = "wbh_gateway_reassigned"
		trigger.EndpointSlug = "gateway-reassigned"
		created, err := database.CreateTrigger(ctx, trigger)
		if err != nil {
			t.Fatalf("CreateTrigger() error = %v", err)
		}
		ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: created.ID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject() error = %v", err)
		}
		if _, _, err := database.PutIngressBinding(
			ctx,
			subject,
			1,
			time.Date(2026, 8, 6, 14, 0, 0, 0, time.UTC),
		); err != nil {
			t.Fatalf("PutIngressBinding() error = %v", err)
		}

		created.WorkspaceID = "workspace-after"
		created.UpdatedAt = created.UpdatedAt.Add(time.Minute)
		if _, err := database.UpdateTrigger(ctx, created); err != nil {
			t.Fatalf("UpdateTrigger(reassign) error = %v", err)
		}
		assertNoIngressBindingRow(ctx, t, database, ref)
	})

	t.Run("Should invalidate a bridge binding when replacement changes its workspace owner", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		insertGatewayTestWorkspace(ctx, t, database, "workspace-before")
		insertGatewayTestWorkspace(ctx, t, database, "workspace-after")
		instance := bridges.BridgeInstance{
			ID:            "bridge-gateway-reassigned",
			Scope:         bridges.ScopeWorkspace,
			WorkspaceID:   "workspace-before",
			Platform:      "telegram",
			ExtensionName: "telegram-adapter",
			DisplayName:   "Reassigned gateway bridge",
			Enabled:       true,
			Status:        bridges.BridgeStatusReady,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			ProviderConfig: json.RawMessage(
				`{"webhook":{"listen_addr":"127.0.0.1:43124","path":"/telegram"}}`,
			),
		}
		if err := database.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}
		ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectBridgeInstance, ID: instance.ID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject() error = %v", err)
		}
		if _, _, err := database.PutIngressBinding(
			ctx,
			subject,
			1,
			time.Date(2026, 8, 6, 14, 0, 0, 0, time.UTC),
		); err != nil {
			t.Fatalf("PutIngressBinding() error = %v", err)
		}

		instance.WorkspaceID = "workspace-after"
		instance.UpdatedAt = time.Date(2026, 8, 6, 14, 1, 0, 0, time.UTC)
		if err := database.ReplaceBridgeInstances(ctx, []bridges.BridgeInstance{instance}); err != nil {
			t.Fatalf("ReplaceBridgeInstances(reassign) error = %v", err)
		}
		assertNoIngressBindingRow(ctx, t, database, ref)
	})

	t.Run("Should keep an external-proxy-only bridge outside gateway ingress", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		instance := bridges.BridgeInstance{
			ID:            "bridge-external-proxy",
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-adapter",
			DisplayName:   "External proxy bridge",
			Status:        bridges.BridgeStatusDisabled,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			ProviderConfig: json.RawMessage(
				`{"webhook":{"public_url":"https://external.example.test/telegram"}}`,
			),
		}
		if err := database.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}
		_, err := database.ResolveIngressSubject(ctx, gateway.IngressSubjectRef{
			Kind: gateway.IngressSubjectBridgeInstance,
			ID:   instance.ID,
		})
		if !errors.Is(err, gateway.ErrIngressSubjectNotFound) {
			t.Fatalf("ResolveIngressSubject(external proxy) error = %v, want ErrIngressSubjectNotFound", err)
		}
	})

	t.Run("Should distinguish an invalid local bridge target from external proxy mode", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		instance := bridges.BridgeInstance{
			ID:            "bridge-invalid-local-target",
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-adapter",
			DisplayName:   "Invalid local target",
			Status:        bridges.BridgeStatusDisabled,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			ProviderConfig: json.RawMessage(
				`{"webhook":{"listen_addr":"0.0.0.0:43124","path":"/telegram"}}`,
			),
		}
		if err := database.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}
		subject, err := database.ResolveIngressSubject(ctx, gateway.IngressSubjectRef{
			Kind: gateway.IngressSubjectBridgeInstance,
			ID:   instance.ID,
		})
		if err != nil {
			t.Fatalf("ResolveIngressSubject(invalid local target) error = %v", err)
		}
		if !subject.TargetUnavailable {
			t.Fatalf("ResolveIngressSubject(invalid local target) = %#v, want unavailable target", subject)
		}
	})

	t.Run("Should treat a missing bridge provider config as an unavailable local target", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		instance := bridges.BridgeInstance{
			ID:            "bridge-missing-provider-config",
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-adapter",
			DisplayName:   "Missing provider config",
			Status:        bridges.BridgeStatusDisabled,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
		}
		if err := database.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}
		resource := bridges.BridgeInstanceSpec{
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-adapter",
			DisplayName:   "Missing resource provider config",
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
		}
		const resourceID = "bridge-resource-missing-provider-config"
		insertGatewayResourceRecord(ctx, t, database, "bridge.instance", resourceID, "global", "", resource)

		for _, bridgeID := range []string{instance.ID, resourceID} {
			subject, err := database.ResolveIngressSubject(ctx, gateway.IngressSubjectRef{
				Kind: gateway.IngressSubjectBridgeInstance,
				ID:   bridgeID,
			})
			if err != nil {
				t.Fatalf("ResolveIngressSubject(%q) error = %v", bridgeID, err)
			}
			if !subject.TargetUnavailable {
				t.Fatalf("ResolveIngressSubject(%q) = %#v, want unavailable target", bridgeID, subject)
			}
		}
	})

	t.Run("Should require fresh confirmation after a bridge switches through external proxy mode", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		instance := bridges.BridgeInstance{
			ID:            "bridge-mode-switch",
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-adapter",
			DisplayName:   "Mode switch bridge",
			Enabled:       true,
			Status:        bridges.BridgeStatusReady,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			ProviderConfig: json.RawMessage(
				`{"webhook":{"listen_addr":"127.0.0.1:43125","path":"/telegram"}}`,
			),
		}
		if err := database.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}
		ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectBridgeInstance, ID: instance.ID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject() error = %v", err)
		}
		if _, _, err := database.PutIngressBinding(
			ctx,
			subject,
			2,
			time.Date(2026, 8, 6, 14, 30, 0, 0, time.UTC),
		); err != nil {
			t.Fatalf("PutIngressBinding() error = %v", err)
		}

		instance.ProviderConfig = json.RawMessage(
			`{"webhook":{"public_url":"https://external.example.test/telegram"}}`,
		)
		instance.UpdatedAt = time.Date(2026, 8, 6, 14, 31, 0, 0, time.UTC)
		if err := database.UpdateBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("UpdateBridgeInstance(external) error = %v", err)
		}
		assertNoIngressBindingRow(ctx, t, database, ref)

		instance.ProviderConfig = json.RawMessage(
			`{"webhook":{"listen_addr":"127.0.0.1:43125","path":"/telegram"}}`,
		)
		instance.UpdatedAt = instance.UpdatedAt.Add(time.Minute)
		if err := database.UpdateBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("UpdateBridgeInstance(local again) error = %v", err)
		}
		if _, err := database.GetIngressBinding(ctx, ref); !errors.Is(err, gateway.ErrIngressSubjectNotFound) {
			t.Fatalf("GetIngressBinding(local again) error = %v, want fresh confirmation", err)
		}
	})

	t.Run("Should hide and sweep a binding whose subject vanished", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: "trigger-orphan"}
		if _, err := database.db.ExecContext(
			ctx,
			`INSERT INTO gateway_ingress_bindings (
				subject_kind, subject_id, scope_kind, workspace_id, endpoint_generation, confirmed_at
			) VALUES (?, ?, 'global', NULL, 1, ?)`,
			string(ref.Kind),
			ref.ID,
			store.FormatTimestamp(time.Date(2026, 8, 6, 13, 30, 0, 0, time.UTC)),
		); err != nil {
			t.Fatalf("insert orphan binding: %v", err)
		}
		if _, err := database.GetIngressBinding(ctx, ref); !errors.Is(err, gateway.ErrIngressSubjectNotFound) {
			t.Fatalf("GetIngressBinding(orphan) error = %v, want ErrIngressSubjectNotFound", err)
		}
		removed, err := database.SweepOrphanedIngressBindings(ctx)
		if err != nil || removed != 1 {
			t.Fatalf("SweepOrphanedIngressBindings() = (%d, %v), want (1, nil)", removed, err)
		}
		assertNoIngressBindingRow(ctx, t, database, ref)
	})

	t.Run("Should delete workspace-owned bindings before cascading their subjects", func(t *testing.T) {
		t.Parallel()
		database := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		insertGatewayTestWorkspace(ctx, t, database, "workspace-cascade")
		trigger := automationWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"gateway-workspace-cascade",
			"workspace-cascade",
			automation.JobSourceDynamic,
		)
		trigger.ID = "trigger-workspace-cascade"
		trigger.WebhookID = "wbh_workspace_cascade"
		trigger.EndpointSlug = "workspace-cascade"
		created, err := database.CreateTrigger(ctx, trigger)
		if err != nil {
			t.Fatalf("CreateTrigger() error = %v", err)
		}
		ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: created.ID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject() error = %v", err)
		}
		if _, _, err := database.PutIngressBinding(
			ctx,
			subject,
			1,
			time.Date(2026, 8, 6, 15, 0, 0, 0, time.UTC),
		); err != nil {
			t.Fatalf("PutIngressBinding() error = %v", err)
		}
		if err := database.DeleteWorkspace(ctx, "workspace-cascade"); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v", err)
		}
		assertNoIngressBindingRow(ctx, t, database, ref)
	})
}

func insertGatewayTestWorkspace(ctx context.Context, t *testing.T, database *GlobalDB, id string) {
	t.Helper()
	at := time.Date(2026, 8, 6, 11, 0, 0, 0, time.UTC)
	if err := database.InsertWorkspace(ctx, compozyworkspace.Workspace{
		ID: id, RootDir: t.TempDir(), Name: id, CreatedAt: at, UpdatedAt: at,
	}); err != nil {
		t.Fatalf("InsertWorkspace(%q) error = %v", id, err)
	}
}

func insertGatewayResourceRecord(
	ctx context.Context,
	t *testing.T,
	database *GlobalDB,
	kind string,
	id string,
	scopeKind string,
	scopeID string,
	spec any,
) {
	t.Helper()
	payload, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("json.Marshal(%s resource) error = %v", kind, err)
	}
	var nullableScopeID any
	if strings.TrimSpace(scopeID) != "" {
		nullableScopeID = scopeID
	}
	at := store.FormatTimestamp(time.Date(2026, 8, 6, 15, 20, 0, 0, time.UTC))
	if _, err := database.db.ExecContext(
		ctx,
		`INSERT INTO resource_records (
			kind, id, version, scope_kind, scope_id, owner_kind, owner_id,
			source_kind, source_id, spec_json, created_at, updated_at
		) VALUES (?, ?, 1, ?, ?, 'operator', 'gateway-test', 'dynamic', 'gateway-test', ?, ?, ?)`,
		kind,
		id,
		scopeKind,
		nullableScopeID,
		string(payload),
		at,
		at,
	); err != nil {
		t.Fatalf("insert %s resource %q error = %v", kind, id, err)
	}
}

func updateGatewayResourceRecord(
	ctx context.Context,
	t *testing.T,
	database *GlobalDB,
	kind string,
	id string,
	spec any,
) {
	t.Helper()
	payload, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("json.Marshal(updated %s resource) error = %v", kind, err)
	}
	result, err := database.db.ExecContext(
		ctx,
		`UPDATE resource_records SET version = version + 1, spec_json = ?, updated_at = ?
		 WHERE kind = ? AND id = ?`,
		string(payload),
		store.FormatTimestamp(time.Date(2026, 8, 6, 15, 21, 0, 0, time.UTC)),
		kind,
		id,
	)
	if err != nil {
		t.Fatalf("update %s resource %q error = %v", kind, id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected(update %s resource %q) error = %v", kind, id, err)
	}
	if rows != 1 {
		t.Fatalf("RowsAffected(update %s resource %q) = %d, want 1", kind, id, rows)
	}
}

func deleteGatewayResourceRecord(
	ctx context.Context,
	t *testing.T,
	database *GlobalDB,
	kind string,
	id string,
) {
	t.Helper()
	result, err := database.db.ExecContext(
		ctx,
		`DELETE FROM resource_records WHERE kind = ? AND id = ?`,
		kind,
		id,
	)
	if err != nil {
		t.Fatalf("delete %s resource %q error = %v", kind, id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected(delete %s resource %q) error = %v", kind, id, err)
	}
	if rows != 1 {
		t.Fatalf("RowsAffected(delete %s resource %q) = %d, want 1", kind, id, rows)
	}
}

func assertNoIngressBindingRow(
	ctx context.Context,
	t *testing.T,
	database *GlobalDB,
	ref gateway.IngressSubjectRef,
) {
	t.Helper()
	var count int
	if err := database.db.QueryRowContext(
		ctx,
		`SELECT count(*) FROM gateway_ingress_bindings WHERE subject_kind = ? AND subject_id = ?`,
		string(ref.Kind),
		ref.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count ingress bindings: %v", err)
	}
	if count != 0 {
		t.Fatalf("ingress binding count = %d, want 0", count)
	}
}

func assertGatewayProviderState(
	t *testing.T,
	snapshot gateway.Snapshot,
	tier gateway.Tier,
	desired gateway.DesiredState,
	generation uint64,
) {
	t.Helper()
	for _, provider := range snapshot.Providers {
		if provider.ProviderName == "connectivity-test" && provider.Tier == tier {
			if provider.Desired != desired || provider.Generation != generation {
				t.Fatalf(
					"provider %s state = (%s, %d), want (%s, %d)",
					tier,
					provider.Desired,
					provider.Generation,
					desired,
					generation,
				)
			}
			return
		}
	}
	t.Fatalf("snapshot providers = %#v, want connectivity-test on %s", snapshot.Providers, tier)
}

func assertGatewayTierKey(t *testing.T, snapshot gateway.Snapshot, tier gateway.Tier) {
	t.Helper()
	providerFound := false
	for _, provider := range snapshot.Providers {
		if provider.ProviderName == "connectivity-test" && provider.Tier == tier &&
			provider.Desired == gateway.DesiredEnabled && provider.Generation == 1 {
			providerFound = true
		}
	}
	if !providerFound {
		t.Fatalf("snapshot providers = %#v, want enabled connectivity-test on %s", snapshot.Providers, tier)
	}
	surfaceFound := false
	for _, surface := range snapshot.Surfaces {
		if surface.Surface == gateway.SurfaceOperatorUI && surface.Tier == tier &&
			surface.Desired == gateway.DesiredEnabled && surface.Generation == 1 {
			surfaceFound = true
		}
	}
	if !surfaceFound {
		t.Fatalf("snapshot surfaces = %#v, want enabled operator UI on %s", snapshot.Surfaces, tier)
	}
}

func TestGlobalDBGatewayDeviceLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should persist hash-only credentials and order the complete device inventory", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		database := openFreshTestGlobalDB(t)
		createdAt := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		first := gateway.StoredDeviceSession{
			Session: gateway.DeviceSession{
				ID: "device-first", Name: "First", ActorKind: gateway.ActorKindOperatorDevice,
				PairingOrigin: "local", CreatedAt: createdAt,
			},
			TokenHash: strings.Repeat("a", 64),
		}
		second := gateway.StoredDeviceSession{
			Session: gateway.DeviceSession{
				ID: "device-second", Name: "Second", ActorKind: gateway.ActorKindCLIProfile,
				PairingOrigin: "private", CreatedAt: createdAt.Add(time.Minute),
			},
			TokenHash: strings.Repeat("b", 64),
		}
		for _, record := range []gateway.StoredDeviceSession{first, second} {
			if err := database.CreateDevice(ctx, record); err != nil {
				t.Fatalf("CreateDevice(%q) error = %v", record.Session.ID, err)
			}
		}
		seenAt := createdAt.Add(2 * time.Minute)
		found, err := database.FindDeviceByTokenHash(ctx, first.TokenHash, seenAt)
		if err != nil {
			t.Fatalf("FindDeviceByTokenHash() error = %v", err)
		}
		if found.Session.ID != first.Session.ID ||
			found.TokenHash != first.TokenHash ||
			found.Session.LastSeenAt != seenAt {
			t.Fatalf("FindDeviceByTokenHash() = %#v", found)
		}
		devices, err := database.ListDevices(ctx)
		if err != nil {
			t.Fatalf("ListDevices() error = %v", err)
		}
		if len(devices) != 2 || devices[0].ID != first.Session.ID || devices[1].ID != second.Session.ID {
			t.Fatalf("ListDevices() = %#v, want last-seen ordering", devices)
		}

		duplicate := second
		duplicate.Session.ID = "device-duplicate-hash"
		duplicate.TokenHash = first.TokenHash
		if err := database.CreateDevice(ctx, duplicate); err == nil {
			t.Fatal("CreateDevice(duplicate token hash) error = nil, want uniqueness failure")
		}
	})

	t.Run("Should fence mutations with the actor epoch and revoke idempotently", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		database := openFreshTestGlobalDB(t)
		createdAt := time.Date(2026, 8, 6, 13, 0, 0, 0, time.UTC)
		actor := gateway.StoredDeviceSession{
			Session: gateway.DeviceSession{
				ID: "actor-device", Name: "Actor", ActorKind: gateway.ActorKindOperatorDevice,
				PairingOrigin: "local", CreatedAt: createdAt,
			},
			TokenHash: strings.Repeat("c", 64),
		}
		target := gateway.StoredDeviceSession{
			Session: gateway.DeviceSession{
				ID: "target-device", Name: "Target", ActorKind: gateway.ActorKindCLIProfile,
				PairingOrigin: "private", CreatedAt: createdAt.Add(time.Minute),
			},
			TokenHash: strings.Repeat("d", 64),
		}
		for _, record := range []gateway.StoredDeviceSession{actor, target} {
			if err := database.CreateDevice(ctx, record); err != nil {
				t.Fatalf("CreateDevice(%q) error = %v", record.Session.ID, err)
			}
		}
		renamed, err := database.RenameDeviceForActor(
			ctx,
			actor.Session.ID,
			actor.Session.RevokeEpoch,
			target.Session.ID,
			"Renamed",
		)
		if err != nil || renamed.Name != "Renamed" {
			t.Fatalf("RenameDeviceForActor() = (%#v, %v)", renamed, err)
		}

		revoked, changed, err := database.RevokeDevice(ctx, actor.Session.ID, createdAt.Add(2*time.Minute))
		if err != nil || !changed || revoked.RevokeEpoch != 1 || revoked.RevokedAt.IsZero() {
			t.Fatalf("RevokeDevice(first) = (%#v, %t, %v)", revoked, changed, err)
		}
		revokedAgain, changedAgain, err := database.RevokeDevice(
			ctx,
			actor.Session.ID,
			createdAt.Add(3*time.Minute),
		)
		if err != nil || changedAgain || revokedAgain.RevokeEpoch != 1 || revokedAgain.RevokedAt != revoked.RevokedAt {
			t.Fatalf("RevokeDevice(second) = (%#v, %t, %v)", revokedAgain, changedAgain, err)
		}
		if err := database.RevalidateDeviceEpoch(
			ctx,
			actor.Session.ID,
			actor.Session.RevokeEpoch,
		); !errors.Is(err, gateway.ErrDeviceRevoked) {
			t.Fatalf("RevalidateDeviceEpoch() error = %v, want ErrDeviceRevoked", err)
		}
		snapshot, err := database.Snapshot(ctx)
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		if len(snapshot.Devices) != 2 {
			t.Fatalf("Snapshot().Devices = %#v, want complete retained inventory", snapshot.Devices)
		}
		var retained gateway.DeviceSession
		for _, device := range snapshot.Devices {
			if device.ID == actor.Session.ID {
				retained = device
				break
			}
		}
		if retained.ID == "" || retained.RevokedAt.IsZero() || retained.RevokeEpoch != 1 {
			t.Fatalf("Snapshot().Devices = %#v, want retained revoked actor", snapshot.Devices)
		}
		_, err = database.RenameDeviceForActor(
			ctx,
			actor.Session.ID,
			actor.Session.RevokeEpoch,
			target.Session.ID,
			"Must not commit",
		)
		if !errors.Is(err, gateway.ErrDeviceRevoked) {
			t.Fatalf("RenameDeviceForActor(stale actor) error = %v, want ErrDeviceRevoked", err)
		}
		unchanged, err := database.GetDevice(ctx, target.Session.ID)
		if err != nil {
			t.Fatalf("GetDevice(target) error = %v", err)
		}
		if unchanged.Name != "Renamed" {
			t.Fatalf("stale actor committed target name %q", unchanged.Name)
		}
	})
}
