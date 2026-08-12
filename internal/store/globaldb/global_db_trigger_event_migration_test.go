package globaldb

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

// Suite: global automation trigger event hard cut
// Invariant: upgrading removes only trigger definitions no runtime producer can activate.
// Boundary IN: a v59 global database containing producer-backed and impossible trigger events.
// Boundary OUT: runtime registration, which consumes the migrated definitions.
func TestGlobalDBTriggerEventHardCutMigration(t *testing.T) {
	t.Run(
		"Should delete impossible triggers and their overlays while preserving supported definitions and run history",
		func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), GlobalDatabaseName)
			prefixDB, err := openGlobalMigrationPrefixDatabase(
				t,
				path,
				globalMigrationPrefixBefore(t, "00060_trigger_event_hard_cut.sql"),
			)
			if err != nil {
				t.Fatalf("OpenSQLiteDatabase(v59 prefix) error = %v", err)
			}
			ctx := testutil.Context(t)
			seedTriggerEventHardCutFixture(ctx, t, prefixDB)
			if err := prefixDB.Close(); err != nil {
				t.Fatalf("prefixDB.Close() error = %v", err)
			}

			upgraded, err := openGlobalMigrationUpgrade(t, path)
			if err != nil {
				t.Fatalf("OpenGlobalDB(v60 upgrade) error = %v", err)
			}
			assertTriggerEventHardCutState(ctx, t, upgraded.db)
			status, err := store.Status(ctx, upgraded.db, MigrationStream())
			if err != nil {
				t.Fatalf("Status(upgraded) error = %v", err)
			}
			assertCompleteMigrationStream(t, status, MigrationStream())
			if err := upgraded.Close(ctx); err != nil {
				t.Fatalf("Close(upgraded) error = %v", err)
			}

			reopened, err := OpenGlobalDB(testutil.Context(t), path)
			if err != nil {
				t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
			}
			t.Cleanup(func() {
				if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
					t.Errorf("Close(reopened) error = %v", closeErr)
				}
			})
			assertTriggerEventHardCutState(testutil.Context(t), t, reopened.db)
		},
	)
}

func seedTriggerEventHardCutFixture(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	events := []struct {
		id    string
		event string
	}{
		{id: "trg-session", event: "session.stopped"},
		{id: "trg-memory", event: "memory.consolidated"},
		{id: "trg-hook-internal", event: "hook.release review.v2.completed"},
		{id: "trg-extension-free-form", event: "ext. release"},
		{id: "trg-unknown", event: "loop.terminal"},
		{id: "trg-hook-prefix-padding", event: "hook. release.completed"},
		{id: "trg-hook-suffix-padding", event: "hook.release .completed"},
		{id: "trg-surrounding-padding", event: " session.stopped"},
		{id: "trg-empty-extension", event: "ext.\u2003"},
	}
	for _, fixture := range events {
		if _, err := db.ExecContext(ctx, `INSERT INTO automation_triggers (
			id, scope, name, agent_name, prompt, event, enabled, retry, fire_limit,
			source, created_at, updated_at
		) VALUES (?, 'global', ?, 'reviewer', 'Review the event.', ?, 1,
			'{"strategy":"none","max_retries":0,"base_delay":""}',
			'{"max":12,"window":"1h"}', 'dynamic',
			'2026-08-12T12:00:00Z', '2026-08-12T12:00:00Z')`, fixture.id, fixture.id, fixture.event); err != nil {
			t.Fatalf("seed trigger %q error = %v", fixture.id, err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO automation_trigger_overlays (
			trigger_id, enabled_override, updated_at
		) VALUES (?, 0, '2026-08-12T12:01:00Z')`, fixture.id); err != nil {
			t.Fatalf("seed trigger overlay %q error = %v", fixture.id, err)
		}
	}
	for _, triggerID := range []string{"trg-session", "trg-unknown"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO automation_trigger_catalog_entries (
			trigger_id, scope, event, source, source_rank, name, enabled,
			search_name, search_agent_name, search_prompt, search_scope,
			search_source, search_event, search_endpoint_slug, search_webhook_id
		) VALUES (?, 'global', 'session.stopped', 'dynamic', 1, ?, 1,
			?, 'reviewer', 'review the event', 'global', 'dynamic',
			'session.stopped', '', '')`, triggerID, triggerID, triggerID); err != nil {
			t.Fatalf("seed trigger catalog entry %q error = %v", triggerID, err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO automation_trigger_catalog_filter_terms (
			trigger_id, value
		) VALUES (?, 'reviewer')`, triggerID); err != nil {
			t.Fatalf("seed trigger catalog filter term %q error = %v", triggerID, err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO automation_runs (
		id, trigger_id, status, attempt, started_at, ended_at, metadata_json
	) VALUES (
		'run-invalid-history', 'trg-unknown', 'completed', 1,
		'2026-08-12T12:02:00Z', '2026-08-12T12:03:00Z', '{}'
	)`); err != nil {
		t.Fatalf("seed invalid trigger run history error = %v", err)
	}
	seedTriggerEventHardCutResource(ctx, t, db, "trg-resource-valid", "ext. release")
	seedTriggerEventHardCutResource(ctx, t, db, "trg-resource-invalid", "loop.terminal")
	seedTriggerEventHardCutMalformedResource(ctx, t, db)
	if _, err := db.ExecContext(ctx, `INSERT INTO automation_trigger_overlays (
		trigger_id, enabled_override, updated_at
	) VALUES
		('trg-resource-valid', 0, '2026-08-12T12:01:00Z'),
		('trg-resource-invalid', 0, '2026-08-12T12:01:00Z'),
		('trg-resource-malformed', 0, '2026-08-12T12:01:00Z')`); err != nil {
		t.Fatalf("seed trigger resource overlays error = %v", err)
	}
	for _, triggerID := range []string{"trg-unknown", "trg-resource-invalid"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO gateway_ingress_bindings (
			subject_kind, subject_id, scope_kind, endpoint_generation, confirmed_at
		) VALUES (
			'webhook_trigger', ?, 'global', 1, '2026-08-12T12:04:00Z'
		)`, triggerID); err != nil {
			t.Fatalf("seed invalid trigger ingress binding %q error = %v", triggerID, err)
		}
		ref := "vault:automation/triggers/" + triggerID + "/webhook-secret"
		if _, err := db.ExecContext(ctx, `INSERT INTO vault_secrets (
			ref, kind, encrypted_value, created_at, updated_at
		) VALUES (?, 'webhook_secret', 'encrypted',
			'2026-08-12T12:04:00Z', '2026-08-12T12:04:00Z')`, ref); err != nil {
			t.Fatalf("seed invalid trigger secret %q error = %v", triggerID, err)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE automation_triggers
		SET webhook_secret_ref = 'vault:automation/triggers/trg-unknown/webhook-secret'
		WHERE id = 'trg-unknown'`); err != nil {
		t.Fatalf("bind invalid legacy trigger secret error = %v", err)
	}
}

func seedTriggerEventHardCutMalformedResource(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.ExecContext(ctx, `INSERT INTO resource_records (
		kind, id, version, scope_kind, owner_kind, owner_id, source_kind, source_id,
		spec_json, created_at, updated_at
	) VALUES (
		'automation.trigger', 'trg-resource-malformed', 1, 'global',
		'operator', 'operator', 'dynamic', 'trg-resource-malformed', '{',
		'2026-08-12T12:00:00Z', '2026-08-12T12:00:00Z'
	)`); err != nil {
		t.Fatalf("seed malformed trigger resource error = %v", err)
	}
}

func seedTriggerEventHardCutResource(
	ctx context.Context,
	t *testing.T,
	db *sql.DB,
	triggerID string,
	event string,
) {
	t.Helper()

	secretRef := "vault:automation/triggers/" + triggerID + "/webhook-secret"
	if _, err := db.ExecContext(ctx, `INSERT INTO resource_records (
		kind, id, version, scope_kind, owner_kind, owner_id, source_kind, source_id,
		spec_json, created_at, updated_at
	) VALUES (
		'automation.trigger', ?, 1, 'global', 'operator', 'operator', 'dynamic', ?,
		json_object(
			'id', ?, 'scope', 'global', 'name', ?, 'agent_name', 'reviewer',
			'prompt', 'Review the event.', 'event', ?, 'enabled', 1,
			'source', 'dynamic', 'webhook_secret_ref', ?
		), '2026-08-12T12:00:00Z', '2026-08-12T12:00:00Z'
	)`, triggerID, triggerID, triggerID, triggerID, event, secretRef); err != nil {
		t.Fatalf("seed trigger resource %q error = %v", triggerID, err)
	}
}

func assertTriggerEventHardCutState(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	wantTriggerIDs := []string{
		"trg-extension-free-form",
		"trg-hook-internal",
		"trg-memory",
		"trg-session",
	}
	if got := triggerEventHardCutIDs(
		ctx,
		t,
		db,
		"automation_triggers",
		"id",
	); !testutil.EqualStringSlices(got, wantTriggerIDs) {
		t.Fatalf("automation trigger ids = %#v, want %#v", got, wantTriggerIDs)
	}
	wantOverlayIDs := []string{
		"trg-extension-free-form",
		"trg-hook-internal",
		"trg-memory",
		"trg-resource-valid",
		"trg-session",
	}
	if got := triggerEventHardCutIDs(
		ctx,
		t,
		db,
		"automation_trigger_overlays",
		"trigger_id",
	); !testutil.EqualStringSlices(got, wantOverlayIDs) {
		t.Fatalf("automation trigger overlay ids = %#v, want %#v", got, wantOverlayIDs)
	}
	if got := triggerEventHardCutIDs(ctx, t, db, "automation_runs", "trigger_id"); !testutil.EqualStringSlices(got, []string{"trg-unknown"}) {
		t.Fatalf("automation run trigger ids = %#v, want preserved invalid history", got)
	}
	if got := triggerEventHardCutIDs(
		ctx,
		t,
		db,
		"automation_trigger_catalog_entries",
		"trigger_id",
	); !testutil.EqualStringSlices(got, []string{"trg-session"}) {
		t.Fatalf("automation trigger catalog ids = %#v, want supported trigger only", got)
	}
	if got := triggerEventHardCutIDs(
		ctx,
		t,
		db,
		"automation_trigger_catalog_filter_terms",
		"trigger_id",
	); !testutil.EqualStringSlices(got, []string{"trg-session"}) {
		t.Fatalf("automation trigger catalog filter ids = %#v, want supported trigger only", got)
	}
	if got := triggerEventHardCutIDsWhere(
		ctx,
		t,
		db,
		"resource_records",
		"id",
		"kind = 'automation.trigger'",
	); !testutil.EqualStringSlices(got, []string{"trg-resource-valid"}) {
		t.Fatalf("automation trigger resource ids = %#v, want supported resource", got)
	}
	if got := triggerEventHardCutIDsWhere(
		ctx,
		t,
		db,
		"gateway_ingress_bindings",
		"subject_id",
		"subject_kind = 'webhook_trigger'",
	); len(got) != 0 {
		t.Fatalf("invalid trigger ingress bindings = %#v, want none", got)
	}
	if got := triggerEventHardCutIDsWhere(
		ctx,
		t,
		db,
		"vault_secrets",
		"ref",
		"kind = 'webhook_secret'",
	); len(got) != 0 {
		t.Fatalf("invalid trigger secrets = %#v, want none", got)
	}
}

func triggerEventHardCutIDs(ctx context.Context, t *testing.T, db *sql.DB, table string, column string) []string {
	t.Helper()
	return triggerEventHardCutIDsWhere(ctx, t, db, table, column, "1 = 1")
}

func triggerEventHardCutIDsWhere(
	ctx context.Context,
	t *testing.T,
	db *sql.DB,
	table string,
	column string,
	predicate string,
) []string {
	t.Helper()

	query := "SELECT " + column + " FROM " + table + " WHERE " + predicate + " ORDER BY " + column
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		t.Fatalf("query %s.%s error = %v", table, column, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Errorf("close %s.%s rows error = %v", table, column, closeErr)
		}
	}()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan %s.%s error = %v", table, column, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s.%s error = %v", table, column, err)
	}
	return ids
}
