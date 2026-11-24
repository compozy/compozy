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
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb"
	"github.com/compozy/compozy/internal/testutil"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

func TestGlobalDBIngressLifecycle(t *testing.T) {
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
		endpoint, changed, err := syncIngressEndpointForTest(ctx, first,
			"https://gateway.example.test", at,
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
		stable, changed, err := syncIngressEndpointForTest(ctx, reopened,
			endpoint.URL, at.Add(time.Minute),
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

		first, changed, err := syncIngressEndpointForTest(ctx, database,
			"https://gateway.example.test", at,
		)
		if err != nil || !changed || first.Generation != 1 {
			t.Fatalf("SyncIngressEndpoint(first) = (%#v, %t, %v), want generation 1 changed", first, changed, err)
		}
		stable, changed, err := syncIngressEndpointForTest(ctx, database,
			"https://gateway.example.test", at.Add(time.Minute),
		)
		if err != nil || changed || stable.Generation != 1 {
			t.Fatalf("SyncIngressEndpoint(stable) = (%#v, %t, %v), want generation 1 unchanged", stable, changed, err)
		}
		moved, changed, err := syncIngressEndpointForTest(ctx, database,
			"https://gateway-two.example.test", at.Add(2*time.Minute),
		)
		if err != nil || !changed || moved.Generation != 2 {
			t.Fatalf("SyncIngressEndpoint(changed) = (%#v, %t, %v), want generation 2 changed", moved, changed, err)
		}
	})

	t.Run("Should resolve webhook ownership and delete its binding atomically [UT-095, UT-097]", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		insertIngressTestWorkspace(ctx, t, database, "workspace-owner")
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
		insertIngressTestWorkspace(ctx, t, database, "workspace-resource-trigger")
		trigger := automationWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"gateway-resource-webhook",
			"workspace-resource-trigger",
			automation.JobSourceDynamic,
		)
		trigger.ID = "trigger-gateway-resource"
		trigger.WebhookID = "wbh_gateway_resource"
		trigger.EndpointSlug = "gateway-resource"
		insertIngressResourceRecord(
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
		updateIngressResourceRecord(ctx, t, database, "automation.trigger", trigger.ID, trigger)
		if _, err := database.GetIngressBinding(ctx, ref); err != nil {
			t.Fatalf("GetIngressBinding(after display-only resource update) error = %v", err)
		}

		trigger.EndpointSlug = "gateway-resource-moved"
		updateIngressResourceRecord(ctx, t, database, "automation.trigger", trigger.ID, trigger)
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
		deleteIngressResourceRecord(ctx, t, database, "automation.trigger", trigger.ID)
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
		insertIngressTestWorkspace(ctx, t, database, "workspace-before")
		insertIngressTestWorkspace(ctx, t, database, "workspace-after")
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
			`INSERT INTO ingress_bindings (
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
		insertIngressTestWorkspace(ctx, t, database, "workspace-cascade")
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

func insertIngressTestWorkspace(ctx context.Context, t *testing.T, database *GlobalDB, id string) {
	t.Helper()
	at := time.Date(2026, 8, 6, 11, 0, 0, 0, time.UTC)
	if err := database.InsertWorkspace(ctx, compozyworkspace.Workspace{
		ID: id, RootDir: t.TempDir(), Name: id, CreatedAt: at, UpdatedAt: at,
	}); err != nil {
		t.Fatalf("InsertWorkspace(%q) error = %v", id, err)
	}
}

func insertIngressResourceRecord(
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

func updateIngressResourceRecord(
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

func deleteIngressResourceRecord(
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
		`SELECT count(*) FROM ingress_bindings WHERE subject_kind = ? AND subject_id = ?`,
		string(ref.Kind),
		ref.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count ingress bindings: %v", err)
	}
	if count != 0 {
		t.Fatalf("ingress binding count = %d, want 0", count)
	}
}

func syncIngressEndpointForTest(
	ctx context.Context, database *GlobalDB, url string, at time.Time,
) (ingress.EndpointState, bool, error) {
	previous, err := database.GetIngressEndpoint(ctx)
	if err != nil && !errors.Is(err, ingress.ErrEndpointUnverified) {
		return ingress.EndpointState{}, false, err
	}
	if err := database.SyncRemoteIngressEndpoint(
		ctx,
		ingress.RemoteEndpoint{Base: url, Managed: true, Reachable: true},
		at,
	); err != nil {
		return ingress.EndpointState{}, false, err
	}
	current, err := database.GetIngressEndpoint(ctx)
	return current, current.URL != previous.URL, err
}

// IT-002. Reopen a 0.7.8 database with live webhook bindings and exercise the migrated owners.
func TestIngressBindingGatewayUpgrade(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve binding generations and enforce the resource lifecycle after upgrade", func(t *testing.T) {
		t.Parallel()
		ctx := globalMigrationTestContext(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prior, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixThrough(t, "00218_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		const base = "https://hooks.example.test"
		const created = "2026-11-24T18:00:00Z"
		insertPrefixRow(t, prior, "gateway_endpoint_state", map[string]any{
			"tier": "public", "endpoint_url": base, "endpoint_generation": 3, "reachable": 1, "updated_at": created,
		})
		for _, id := range []string{"trigger-delete", "trigger-keep"} {
			insertPrefixRow(t, prior, "resource_records", map[string]any{
				"kind": "automation.trigger", "id": id, "version": 1, "scope_kind": "user",
				"owner_kind": "operator", "owner_id": "test", "source_kind": "dynamic", "source_id": "test",
				"spec_json":  `{"event":"webhook","endpoint_slug":"deploy","webhook_id":"wbh_existing"}`,
				"created_at": created, "updated_at": created,
			})
			insertPrefixRow(t, prior, "gateway_ingress_bindings", map[string]any{
				"subject_kind": "webhook_trigger", "subject_id": id, "scope_kind": "global",
				"endpoint_generation": 3, "confirmed_at": created,
			})
		}
		telemetry, err := telemetrydb.Open(ctx, telemetrydb.Path(path))
		if err != nil {
			t.Fatal(err)
		}
		bindErr := bindTelemetryDestination(ctx, prior, telemetry.DB())
		if err := errors.Join(bindErr, telemetry.Close(ctx), prior.Close()); err != nil {
			t.Fatal(err)
		}
		database, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := database.Close(context.WithoutCancel(ctx)); err != nil {
				t.Error(err)
			}
		})
		endpoint, err := database.GetIngressEndpoint(ctx)
		if err != nil || endpoint.URL != base || endpoint.Generation != 3 || !endpoint.Reachable {
			t.Fatalf("migrated endpoint = %+v, %v", endpoint, err)
		}
		bindings, err := database.ListIngressBindings(ctx)
		if err != nil || len(bindings) != 2 {
			t.Fatalf("migrated bindings = %+v, %v", bindings, err)
		}
		for i, want := range []string{"trigger-delete", "trigger-keep"} {
			got := bindings[i]
			if got.Subject.ID != want || got.EndpointGeneration != 3 || got.Scope != ingress.ScopeGlobal ||
				got.WorkspaceID != "" || got.ConfirmedAt.Format(time.RFC3339) != created {
				t.Fatalf("migrated binding %d = %+v", i, got)
			}
		}
		var triggerCount int
		if err := database.ReadDB().QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema
			WHERE type = 'trigger' AND name IN ('ingress_resource_delete', 'ingress_trigger_resource_identity_update')`).
			Scan(&triggerCount); err != nil || triggerCount != 2 {
			t.Fatalf("retained ingress triggers = %d, %v", triggerCount, err)
		}
		deleteIngressResourceRecord(ctx, t, database, "automation.trigger", "trigger-delete")
		assertNoIngressBindingRow(
			ctx,
			t,
			database,
			ingress.SubjectRef{Kind: ingress.SubjectWebhookTrigger, ID: "trigger-delete"},
		)

		manager, err := ingress.NewManager(database, nil, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		keep := ingress.SubjectRef{Kind: ingress.SubjectWebhookTrigger, ID: "trigger-keep"}
		if err := manager.AdmitDelivery(ctx, base, keep); err != nil {
			t.Fatalf("migrated binding refused at the same address: %v", err)
		}
	})
}
