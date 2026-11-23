package globaldb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/apptoken"
	"github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/remotehost"
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
			if err := reopened.Close(testutil.Context(t)); err != nil {
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
		database := openTestGlobalDB(t)
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
		insertGatewayResourceRecord(ctx, t, database, "automation.trigger", trigger.ID, "user", "", trigger)
		ref := ingress.SubjectRef{Kind: ingress.SubjectWebhookTrigger, ID: trigger.ID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject() error = %v", err)
		}
		fencedCtx := store.ContextWithMutationCommitFence(ctx, func(context.Context) error {
			return remotehost.ErrDeviceRevoked
		})

		if _, err := database.Transition(fencedCtx, gateway.TransitionRequest{
			Target: gateway.TargetSurface, Tier: gateway.TierPublic,
			Surface: gateway.SurfaceOperatorUI, Desired: gateway.DesiredEnabled, Consent: true,
		}, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)); !errors.Is(err, remotehost.ErrDeviceRevoked) {
			t.Fatalf("Transition(fenced) error = %v, want ErrDeviceRevoked", err)
		}
		if _, _, err := database.SyncIngressEndpoint(
			fencedCtx, "https://gateway.example.test",
			time.Date(2026, 8, 6, 12, 1, 0, 0, time.UTC),
		); !errors.Is(err, remotehost.ErrDeviceRevoked) {
			t.Fatalf("SyncIngressEndpoint(fenced) error = %v, want ErrDeviceRevoked", err)
		}
		if _, _, err := database.PutIngressBinding(
			fencedCtx,
			subject,
			1,
			time.Date(2026, 8, 6, 12, 2, 0, 0, time.UTC),
		); !errors.Is(err, remotehost.ErrDeviceRevoked) {
			t.Fatalf("PutIngressBinding(fenced) error = %v, want ErrDeviceRevoked", err)
		}

		snapshot, err := database.Snapshot(ctx)
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		if len(snapshot.Surfaces) != 0 {
			t.Fatalf("snapshot surfaces = %#v, want rollback", snapshot.Surfaces)
		}
		if _, err := database.GetIngressEndpoint(ctx); !errors.Is(
			err,
			ingress.ErrEndpointUnverified,
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
			ctx, "https://gateway.example.test", at,
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
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		persisted, err := reopened.GetIngressEndpoint(ctx)
		if err != nil {
			t.Fatalf("GetIngressEndpoint(reopened) error = %v", err)
		}
		if persisted.URL != endpoint.URL || persisted.Generation != 1 || !persisted.Reachable {
			t.Fatalf("reopened endpoint = %#v, want stable generation 1", persisted)
		}
		stable, changed, err := reopened.SyncIngressEndpoint(
			ctx, endpoint.URL, at.Add(time.Minute),
		)
		if err != nil || changed || stable.Generation != 1 {
			t.Fatalf("SyncIngressEndpoint(reopened stable) = (%#v, %t, %v)", stable, changed, err)
		}
	})

	t.Run("Should preserve generation for a stable address and advance it on change [UT-094]", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		at := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

		first, changed, err := database.SyncIngressEndpoint(
			ctx, "https://gateway.example.test", at,
		)
		if err != nil || !changed || first.Generation != 1 {
			t.Fatalf("SyncIngressEndpoint(first) = (%#v, %t, %v), want generation 1 changed", first, changed, err)
		}
		stable, changed, err := database.SyncIngressEndpoint(
			ctx, "https://gateway.example.test", at.Add(time.Minute),
		)
		if err != nil || changed || stable.Generation != 1 {
			t.Fatalf("SyncIngressEndpoint(stable) = (%#v, %t, %v), want generation 1 unchanged", stable, changed, err)
		}
		moved, changed, err := database.SyncIngressEndpoint(
			ctx, "https://gateway-two.example.test", at.Add(2*time.Minute),
		)
		if err != nil || !changed || moved.Generation != 2 {
			t.Fatalf("SyncIngressEndpoint(changed) = (%#v, %t, %v), want generation 2 changed", moved, changed, err)
		}
	})

	t.Run("Should resolve webhook ownership and delete its binding atomically [UT-095, UT-097]", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
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
		ref := ingress.SubjectRef{Kind: ingress.SubjectWebhookTrigger, ID: created.ID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject() error = %v", err)
		}
		if subject.Scope != ingress.ScopeWorkspace || subject.WorkspaceID != "workspace-owner" ||
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
		if _, err := database.GetIngressBinding(ctx, ref); !errors.Is(err, ingress.ErrSubjectNotFound) {
			t.Fatalf("GetIngressBinding(after delete) error = %v, want ErrIngressSubjectNotFound", err)
		}
	})

	t.Run("Should bind canonical trigger resources and invalidate only ingress identity changes", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
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

		ref := ingress.SubjectRef{Kind: ingress.SubjectWebhookTrigger, ID: trigger.ID}
		subject, err := database.ResolveIngressSubject(ctx, ref)
		if err != nil {
			t.Fatalf("ResolveIngressSubject(resource trigger) error = %v", err)
		}
		if subject.Scope != ingress.ScopeWorkspace ||
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
				database := openTestGlobalDB(t)
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
				ref := ingress.SubjectRef{Kind: ingress.SubjectWebhookTrigger, ID: created.ID}
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

	t.Run("Should invalidate a webhook binding when its workspace owner changes", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
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
		ref := ingress.SubjectRef{Kind: ingress.SubjectWebhookTrigger, ID: created.ID}
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

	t.Run("Should hide and sweep a binding whose subject vanished", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		ref := ingress.SubjectRef{Kind: ingress.SubjectWebhookTrigger, ID: "trigger-orphan"}
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
		if _, err := database.GetIngressBinding(ctx, ref); !errors.Is(err, ingress.ErrSubjectNotFound) {
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
		database := openTestGlobalDB(t)
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
		ref := ingress.SubjectRef{Kind: ingress.SubjectWebhookTrigger, ID: created.ID}
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
	ref ingress.SubjectRef,
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

func TestIngressEndpointFailsClosedAcrossRestart(t *testing.T) {
	t.Parallel()
	t.Run("Should mark the endpoint unavailable when restart reconciliation fails", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		database := openTestGlobalDB(t)
		const baseURL = "https://gateway.example.test"
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		initial, _, err := database.SyncIngressEndpoint(ctx, baseURL, now)
		if err != nil || !initial.Reachable || initial.Generation != 1 {
			t.Fatalf("initial endpoint = (%#v, %v), want reachable generation 1", initial, err)
		}

		reconciler := gateway.NewReconciler(database.GatewayRepo, nil)
		t.Cleanup(func() {
			if err := reconciler.Close(context.WithoutCancel(ctx)); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		err = reconciler.Reconcile(ctx, gateway.ExposurePlan{Tiers: []gateway.TierPlan{{
			Tier: gateway.TierPublic, Enabled: true,
		}}})
		if err == nil {
			t.Fatal("Reconcile(missing provider) error = nil")
		}
		endpoint, err := database.GetIngressEndpoint(ctx)
		if err != nil || endpoint.Reachable {
			t.Fatalf("endpoint after failed restart reconcile = (%#v, %v), want unavailable", endpoint, err)
		}

		restored, changed, err := database.SyncIngressEndpoint(ctx, baseURL, now.Add(3*time.Hour))
		if err != nil || changed || restored.Generation != 1 || !restored.Reachable {
			t.Fatalf(
				"stable endpoint restore = (%#v, %t, %v), want reachable generation 1 unchanged [IT-051]",
				restored, changed, err,
			)
		}
	})
}

func TestGlobalDBGatewayDeviceLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should persist hash-only credentials and order the complete device inventory", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		database := openTestGlobalDB(t)
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
		database := openTestGlobalDB(t)
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
		); !errors.Is(err, remotehost.ErrDeviceRevoked) {
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
		if !errors.Is(err, remotehost.ErrDeviceRevoked) {
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

	t.Run("Should delete a device record only under a live actor fence", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		database := openTestGlobalDB(t)
		createdAt := time.Date(2026, 8, 6, 14, 0, 0, 0, time.UTC)
		actor := gateway.StoredDeviceSession{
			Session: gateway.DeviceSession{
				ID: "delete-actor", Name: "Actor", ActorKind: gateway.ActorKindOperatorDevice,
				PairingOrigin: "local", CreatedAt: createdAt,
			},
			TokenHash: strings.Repeat("e", 64),
		}
		target := gateway.StoredDeviceSession{
			Session: gateway.DeviceSession{
				ID: "delete-target", Name: "Target", ActorKind: gateway.ActorKindCLIProfile,
				PairingOrigin: "private", CreatedAt: createdAt,
			},
			TokenHash: strings.Repeat("f", 64),
		}
		for _, record := range []gateway.StoredDeviceSession{actor, target} {
			if err := database.CreateDevice(ctx, record); err != nil {
				t.Fatalf("CreateDevice(%q) error = %v", record.Session.ID, err)
			}
		}
		deleted, err := database.DeleteDeviceForActor(
			ctx,
			actor.Session.ID,
			actor.Session.RevokeEpoch,
			target.Session.ID,
		)
		if err != nil || deleted.ID != target.Session.ID || deleted.Name != "Target" {
			t.Fatalf("DeleteDeviceForActor() = (%#v, %v)", deleted, err)
		}
		if _, err := database.GetDevice(ctx, target.Session.ID); !errors.Is(err, gateway.ErrDeviceNotFound) {
			t.Fatalf("GetDevice(deleted) error = %v, want ErrDeviceNotFound", err)
		}
		if _, err := database.DeleteDevice(ctx, target.Session.ID); !errors.Is(
			err,
			gateway.ErrDeviceNotFound,
		) {
			t.Fatalf("DeleteDevice(absent) error = %v, want ErrDeviceNotFound", err)
		}
		if _, _, err := database.RevokeDevice(ctx, actor.Session.ID, createdAt.Add(time.Minute)); err != nil {
			t.Fatalf("RevokeDevice(actor) error = %v", err)
		}
		if _, err := database.DeleteDeviceForActor(
			ctx,
			actor.Session.ID,
			actor.Session.RevokeEpoch,
			actor.Session.ID,
		); !errors.Is(err, remotehost.ErrDeviceRevoked) {
			t.Fatalf("DeleteDeviceForActor(stale actor) error = %v, want ErrDeviceRevoked", err)
		}
		if _, err := database.GetDevice(ctx, actor.Session.ID); err != nil {
			t.Fatalf("GetDevice(actor) error = %v, want retained record", err)
		}
	})
}

// Invariant: app credentials remain hash-only, isolated, origin-scoped and revocable; owner: app token service with SQLite.
func TestGlobalDBGatewayAppTokenLifecycle(t *testing.T) {
	t.Parallel()
	t.Run("Should keep app token authentication separate from device inventory and last use", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		database := openTestGlobalDB(t)
		service := appTokenServiceForTest(t, database)
		deviceService, err := gateway.NewDeviceService(database)
		if err != nil {
			t.Fatal(err)
		}
		pairing, err := deviceService.MintPairing(ctx, gateway.PairingRequest{Source: gateway.PairingSourceLocal})
		if err != nil {
			t.Fatal(err)
		}
		device, err := deviceService.RedeemPairing(ctx, gateway.RedeemRequest{
			Artifact: pairing.Artifact,
			Name:     "Operator",
			Kind:     gateway.ActorKindOperatorDevice,
			Source:   gateway.PairingSourceLocal,
		})
		if err != nil {
			t.Fatal(err)
		}
		issued, err := service.CreateAppToken(
			ctx,
			"  Dashboard  ",
			[]string{"http://A.test:80", "http://a.test", "https://b.test"},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^cpz_app_[A-Za-z0-9_-]{43}$`).MatchString(issued.Secret) ||
			!strings.HasPrefix(issued.Token.ID, "apt_") || issued.Token.Name != "Dashboard" ||
			!reflect.DeepEqual(issued.Token.Origins, []string{"http://a.test", "https://b.test"}) {
			t.Fatalf("unexpected issued token metadata: %#v", issued.Token)
		}
		digest := sha256.Sum256([]byte(issued.Secret))
		var storedHash string
		if err := database.db.QueryRowContext(ctx, "SELECT token_hash FROM gateway_device_sessions WHERE id = ?", issued.Token.ID).
			Scan(&storedHash); err != nil {
			t.Fatal(err)
		}
		if storedHash != hex.EncodeToString(digest[:]) || storedHash == issued.Secret {
			t.Fatal("app token secret was not stored exclusively as its hash")
		}
		token, err := service.AuthenticateAppToken(ctx, issued.Secret)
		if err != nil || !token.LastSeenAt.IsZero() {
			t.Fatalf("read-only authentication = (%#v, %v)", token, err)
		}
		if _, err := service.AuthenticateAppToken(
			ctx,
			device.Credential,
		); !errors.Is(
			err,
			apptoken.ErrUnauthenticated,
		) {
			t.Fatalf("device credential authenticated as app token: %v", err)
		}
		if _, err := deviceService.Authenticate(ctx, issued.Secret); !errors.Is(err, gateway.ErrDeviceUnauthenticated) {
			t.Fatalf("app token authenticated as device: %v", err)
		}
		for _, mutate := range []func() error{
			func() error { _, err := deviceService.RenameDevice(ctx, token.ID, "No"); return err },
			func() error { _, err := deviceService.RevokeDevice(ctx, token.ID); return err },
		} {
			if err := mutate(); !errors.Is(err, gateway.ErrDeviceNotFound) {
				t.Fatalf("device mutation reached app token: %v", err)
			}
		}
		devices, err := deviceService.ListDevices(ctx)
		if err != nil || len(devices) != 1 || devices[0].ID != device.Device.ID {
			t.Fatalf("device inventory = (%#v, %v)", devices, err)
		}
		snapshot, err := database.Snapshot(ctx)
		if err != nil || len(snapshot.Devices) != 1 || snapshot.Devices[0].ID != device.Device.ID {
			t.Fatalf("gateway snapshot = (%#v, %v)", snapshot.Devices, err)
		}
		count, err := database.GatewayRepo.queries.CountActiveGatewayDeviceSessions(ctx)
		if err != nil || count != 1 {
			t.Fatalf("active device count = %d, %v; want 1", count, err)
		}
		activeDevices, err := database.GatewayRepo.queries.ListActiveGatewayDeviceSessions(ctx)
		if err != nil || len(activeDevices) != 1 || activeDevices[0].ID != device.Device.ID {
			t.Fatalf("active device inventory = %#v, %v", activeDevices, err)
		}
		if err := service.TouchAppTokenLastSeen(ctx, token.ID, token.RevokeEpoch); err != nil {
			t.Fatal(err)
		}
		tokens, err := service.ListAppTokens(ctx, "")
		if err != nil || len(tokens) != 1 || tokens[0].LastSeenAt.IsZero() {
			t.Fatalf("last use was not recorded after authorization: (%#v, %v)", tokens, err)
		}
	})

	t.Run(
		"Should rebuild origins on create update revoke and startup while retaining revoked audit rows",
		func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			database := openTestGlobalDB(t)
			service := appTokenServiceForTest(t, database)
			issued, err := service.CreateAppToken(ctx, "Dashboard", []string{"http://a.test"})
			if err != nil || !service.AppTokenOriginAllowed("http://a.test") {
				t.Fatalf("create origin union: %v", err)
			}
			restarted := appTokenServiceForTest(t, database)
			if !restarted.AppTokenOriginAllowed("http://a.test") {
				t.Fatal("startup did not load persisted origin union")
			}
			origins := []string{"https://b.test"}
			updated, err := service.UpdateAppToken(
				ctx,
				issued.Token.ID,
				apptoken.Patch{Name: new("Production"), Origins: &origins},
			)
			if err != nil || updated.Name != "Production" || service.AppTokenOriginAllowed("http://a.test") ||
				!service.AppTokenOriginAllowed("https://b.test") {
				t.Fatalf("update origin union: (%#v, %v)", updated, err)
			}
			updated, err = service.UpdateAppToken(
				ctx,
				issued.Token.ID,
				apptoken.Patch{Origins: new([]string{})},
			)
			if err != nil || len(updated.Origins) != 0 || updated.Name != "Production" ||
				service.AppTokenOriginAllowed("https://b.test") {
				t.Fatalf("clear origins = (%#v, %v)", updated, err)
			}
			updated, err = service.UpdateAppToken(
				ctx,
				issued.Token.ID,
				apptoken.Patch{Origins: &origins},
			)
			if err != nil || updated.Name != "Production" {
				t.Fatalf("replace origins = (%#v, %v)", updated, err)
			}
			stream, cancel := context.WithCancel(ctx)
			defer cancel()
			service.Connections().RegisterWaitable(issued.Token.ID, cancel, stream.Done())
			revoked, err := service.RevokeAppToken(ctx, issued.Token.ID)
			if err != nil || !revoked.Changed || revoked.Canceled != 1 || stream.Err() == nil ||
				service.AppTokenOriginAllowed("https://b.test") {
				t.Fatalf("revocation = (%#v, %v)", revoked, err)
			}
			again, err := service.RevokeAppToken(ctx, issued.Token.ID)
			if err != nil || again.Changed || again.Canceled != 0 ||
				again.Token.RevokedAt != revoked.Token.RevokedAt {
				t.Fatalf("repeat revocation = (%#v, %v)", again, err)
			}
			if _, err := service.AuthenticateAppToken(ctx, issued.Secret); !errors.Is(err, apptoken.ErrRevoked) {
				t.Fatalf("revoked token authentication: %v", err)
			}
			if err := service.TouchAppTokenLastSeen(
				ctx,
				issued.Token.ID,
				0,
			); !errors.Is(
				err,
				apptoken.ErrRevoked,
			) {
				t.Fatalf("revoked token last use: %v", err)
			}
			if _, err := service.UpdateAppToken(
				ctx,
				issued.Token.ID,
				apptoken.Patch{Name: new("No"), Origins: new([]string{})},
			); !errors.Is(
				err,
				apptoken.ErrRevoked,
			) {
				t.Fatalf("revoked token update: %v", err)
			}
			for _, state := range []apptoken.State{apptoken.StateAll, apptoken.StateRevoked} {
				tokens, err := service.ListAppTokens(ctx, state)
				if err != nil || len(tokens) != 1 || !reflect.DeepEqual(tokens[0], again.Token) {
					t.Fatalf("retained revoked inventory(%s) = (%#v, %v)", state, tokens, err)
				}
			}
			active, err := service.ListAppTokens(ctx, apptoken.StateActive)
			if err != nil || len(active) != 0 {
				t.Fatalf("active inventory = (%#v, %v)", active, err)
			}
			if _, err := service.ListAppTokens(ctx, "expired"); !errors.Is(err, apptoken.ErrStateInvalid) {
				t.Fatalf("invalid state filter: %v", err)
			}
		},
	)

	t.Run("Should list active tokens before revoked tokens in newest-first order", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
		service, err := apptoken.NewService(
			database,
			remotehost.NewMutationGate(database.RevalidateDeviceEpoch),
			remotehost.NewConnectionRegistry(),
			func() time.Time { return now },
		)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, 3)
		for index := range ids {
			issued, err := service.CreateAppToken(t.Context(), fmt.Sprintf("App %d", index), nil)
			if err != nil {
				t.Fatal(err)
			}
			ids[index] = issued.Token.ID
			now = now.Add(time.Minute)
		}
		if _, err := service.RevokeAppToken(t.Context(), ids[1]); err != nil {
			t.Fatal(err)
		}
		tokens, err := service.ListAppTokens(t.Context(), apptoken.StateAll)
		if err != nil || len(tokens) != 3 || tokens[0].ID != ids[2] || tokens[1].ID != ids[0] ||
			tokens[2].ID != ids[1] {
			t.Fatalf("ordered inventory = %#v, %v", tokens, err)
		}
		if _, err := service.RevokeAppToken(t.Context(), "apt_missing"); !errors.Is(err, apptoken.ErrNotFound) {
			t.Fatalf("unknown token revoke error = %v", err)
		}
	})

	t.Run("Should revoke an attributed local request without waiting on its own lease", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		service := appTokenServiceForTest(t, openTestGlobalDB(t))
		issued, err := service.CreateAppToken(ctx, "Self", nil)
		if err != nil {
			t.Fatal(err)
		}
		lease, err := service.AcquireMutation(ctx, issued.Token.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer remotehost.ReleaseMutation(lease)
		fenced := store.ContextWithMutationCommitFence(lease, func(commitCtx context.Context) error {
			return service.RevalidateForCommit(commitCtx, issued.Token.ID, 0)
		})
		result, err := service.RevokeAppToken(fenced, issued.Token.ID)
		if err != nil || !result.Changed {
			t.Fatalf("self revoke = %#v, %v", result, err)
		}
	})

	t.Run("Should drain app mutation leases before committing revocation", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		service := appTokenServiceForTest(t, openTestGlobalDB(t))
		issued, err := service.CreateAppToken(ctx, "Leased", nil)
		if err != nil {
			t.Fatal(err)
		}
		lease, err := service.AcquireMutation(ctx, issued.Token.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer remotehost.ReleaseMutation(lease)
		done := make(chan error, 1)
		go func() {
			_, err := service.RevokeAppToken(ctx, issued.Token.ID)
			done <- err
		}()
		select {
		case <-lease.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("revocation did not cancel the active mutation")
		}
		select {
		case err := <-done:
			t.Fatalf("revocation finished before lease drain: %v", err)
		default:
		}
		updateDone := make(chan error, 1)
		go func() {
			_, err := service.UpdateAppToken(
				lease,
				issued.Token.ID,
				apptoken.Patch{Name: new("Canceled")},
			)
			updateDone <- err
		}()
		select {
		case err := <-updateDone:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled mutation error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("revocation held a lock needed by the draining mutation")
		}
		remotehost.ReleaseMutation(lease)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if _, err := service.AcquireMutation(
			ctx,
			issued.Token.ID,
			0,
		); !errors.Is(
			err,
			remotehost.ErrDeviceRevoked,
		) {
			t.Fatalf("revoked token acquired mutation lease: %v", err)
		}
	})
}

// Invariant: limits are enforced at validation and atomically at persistence; owner: gateway/SQLite.
func TestGlobalDBGatewayAppTokenLimits(t *testing.T) {
	t.Parallel()
	t.Run("Should validate names and count only distinct normalized origins", func(t *testing.T) {
		t.Parallel()
		service := appTokenServiceForTest(t, openTestGlobalDB(t))
		for _, name := range []string{"   ", strings.Repeat("a", 65)} {
			if _, err := service.CreateAppToken(
				t.Context(),
				name,
				nil,
			); !errors.Is(
				err,
				apptoken.ErrNameInvalid,
			) {
				t.Fatalf("invalid token name: %v", err)
			}
		}
		origins := make([]string, 16)
		for index := range origins {
			origins[index] = fmt.Sprintf("https://app-%d.test", index)
		}
		issued, err := service.CreateAppToken(
			t.Context(),
			strings.Repeat("a", 64),
			append(origins, "https://APP-0.test:443"),
		)
		if err != nil || len(issued.Token.Origins) != 16 {
			t.Fatalf("distinct-origin boundary = (%#v, %v)", issued.Token, err)
		}
		if _, err := service.CreateAppToken(
			t.Context(),
			"Too many",
			append(origins, "https://another.test"),
		); !errors.Is(
			err,
			apptoken.ErrOriginLimit,
		) {
			t.Fatalf("origin limit: %v", err)
		}
	})

	t.Run(
		"Should reserve the final active slot across concurrent creators and release it on revoke",
		func(t *testing.T) {
			t.Parallel()
			database := openTestGlobalDB(t)
			service := appTokenServiceForTest(t, database)
			var firstID string
			for index := range 60 {
				issued, err := service.CreateAppToken(t.Context(), fmt.Sprintf("App %d", index), nil)
				if err != nil {
					t.Fatal(err)
				}
				firstID = issued.Token.ID
			}
			start := make(chan struct{})
			results := make(chan error, 10)
			var workers sync.WaitGroup
			for range 10 {
				creator := appTokenServiceForTest(t, database)
				workers.Go(func() {
					<-start
					_, err := creator.CreateAppToken(t.Context(), "Concurrent", nil)
					results <- err
				})
			}
			close(start)
			workers.Wait()
			close(results)
			var success, limited int
			for err := range results {
				switch {
				case err == nil:
					success++
				case errors.Is(err, apptoken.ErrLimit):
					limited++
				default:
					t.Fatalf("concurrent create: %v", err)
				}
			}
			if success != 4 || limited != 6 {
				t.Fatalf("concurrent outcomes = (%d successes, %d limited)", success, limited)
			}
			count, err := database.GatewayRepo.queries.CountActiveAppTokens(t.Context())
			if err != nil || count != 64 {
				t.Fatalf("active token count = %d, %v; want 64", count, err)
			}
			if _, err := service.RevokeAppToken(t.Context(), firstID); err != nil {
				t.Fatal(err)
			}
			if _, err := service.CreateAppToken(t.Context(), "Replacement", nil); err != nil {
				t.Fatalf("revoked token still occupied an active slot: %v", err)
			}
		},
	)
}

func appTokenServiceForTest(t *testing.T, database *GlobalDB) *apptoken.Service {
	t.Helper()
	service, err := apptoken.NewService(
		database,
		remotehost.NewMutationGate(database.RevalidateDeviceEpoch),
		remotehost.NewConnectionRegistry(),
		func() time.Time { return time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.LoadAppTokenOrigins(t.Context()); err != nil {
		t.Fatal(err)
	}
	return service
}
