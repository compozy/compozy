//go:build integration

package store_test

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestCloudStore(t *testing.T) {
	t.Parallel()

	t.Run("Should count only linked enabled computers and active reservations (UT-004)", func(t *testing.T) {
		t.Parallel()
		db, conn, cloud := openCloudStore(t)
		ctx := t.Context()
		now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
		seedPlanUsage(t, conn, now)
		if err := cloud.Transaction(ctx, func(q *store.Queries) error {
			if err := q.LockPlanAccount(ctx, "acc_a"); err != nil {
				return err
			}
			usage, err := q.PlanUsage(ctx, "acc_a", now)
			if err != nil {
				return err
			}
			if usage != (plan.Usage{}) {
				t.Errorf("inactive rows used slots: %+v", usage)
			}
			return plan.CanReserve(plan.Free, usage)
		}); err != nil {
			t.Fatalf("reserve with inactive usage: %v", err)
		}

		if _, err := conn.Exec(ctx, "UPDATE computers SET plan_limited = false WHERE id = 'cmp_limited'"); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(
			ctx,
			"UPDATE slot_reservations SET released_at = NULL WHERE id = 'rsv_released'",
		); err != nil {
			t.Fatal(err)
		}
		usage, err := cloud.PlanUsage(ctx, "acc_a", now)
		if err != nil || usage.EnabledComputers != 1 || usage.ActiveReservations != 1 {
			t.Fatalf("counted usage = %+v, %v for schema %s", usage, err, db.Schema)
		}
		if err := plan.CanReserve(plan.Free, usage); !errors.Is(err, plan.ErrLimitReached) {
			t.Fatalf("Free plan accepted active usage: %v", err)
		}
	})

	t.Run("Should enforce append-only audit storage and compact expired rows (UT-139)", func(t *testing.T) {
		t.Parallel()
		_, conn, cloud := openCloudStore(t)
		ctx := t.Context()
		now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
		old := now.AddDate(-1, 0, -1)
		safe := map[string]any{"device_name": "laptop", "location": "Lisbon, PT"}
		for _, event := range []store.SecurityEvent{
			{Type: "device_enrolled", Actor: "account:dev_fixture", At: old, Details: safe},
			{Type: "device_enrolled", Actor: "account:dev_fixture", At: old, Details: safe},
			{Type: "device_revoked", Actor: "system", At: old},
			{Type: "gate_policy_changed", Actor: "admin:operator@example.test", At: now.AddDate(-1, 0, 0)},
		} {
			if err := cloud.AppendSecurityEvent(ctx, event); err != nil {
				t.Fatal(err)
			}
		}
		for _, key := range []string{"token", "secret", "refresh_token", "password", "ip"} {
			event := store.SecurityEvent{
				Type: "device_enrolled", Actor: "system", At: now,
				Details: map[string]any{"nested": []any{map[string]any{key: "unsafe"}}},
			}
			if err := cloud.AppendSecurityEvent(ctx, event); !errors.Is(err, store.ErrSecurityEventDetails) {
				t.Errorf("accepted %s details: %v", key, err)
			}
		}
		if _, err := conn.Exec(ctx, "SET compozy.security_event_retention = 'on'"); err != nil {
			t.Fatal(err)
		}
		for _, query := range []string{
			"UPDATE security_events SET type = 'changed'",
			"DELETE FROM security_events",
			"TRUNCATE security_events",
		} {
			if _, err := conn.Exec(ctx, query); err == nil {
				t.Errorf("%s bypassed append-only trigger", query)
			}
		}
		for range 2 {
			if err := cloud.CompactSecurityEvents(ctx, now); err != nil {
				t.Fatalf("compact old audit records: %v", err)
			}
		}
		assertAuditRetention(t, conn)
	})
}

func openCloudStore(t *testing.T) (*remotetest.Database, *pgx.Conn, *store.Store) {
	t.Helper()
	db := remotetest.NewDatabase(t)
	db.Migrate(t)
	cfg := db.Config(t, "store")
	cloud, err := store.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cloud.Close)
	conn, err := cloud.ConnectDirect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := conn.Close(ctx); err != nil {
			t.Errorf("close Postgres connection: %v", err)
		}
	})
	return db, conn, cloud
}

func seedPlanUsage(t *testing.T, conn *pgx.Conn, now time.Time) {
	t.Helper()
	ctx := t.Context()
	queries := []string{
		`INSERT INTO accounts (id, email, name, first_sign_in_provider)
		  VALUES ('acc_a', 'a@example.test', 'A', 'github'), ('acc_b', 'b@example.test', 'B', 'google')`,
		`INSERT INTO devices (id, account_id, name, kind) VALUES ('dev_a', 'acc_a', 'A', 'cli')`,
		`INSERT INTO computers (id, account_id, name, platform, version, computer_jwk, tls_root_sha256,
		                        remote_enabled, plan_limited, unlinked_at)
		  VALUES ('cmp_limited', 'acc_a', 'Limited', 'linux_x86_64', '0.8.0', '{}', 'root', true, true, NULL),
		         ('cmp_disabled', 'acc_a', 'Disabled', 'linux_x86_64', '0.8.0', '{}', 'root', false, false, NULL),
		         ('cmp_unlinked', 'acc_a', 'Unlinked', 'linux_x86_64', '0.8.0', '{}', 'root', true, false, now()),
		         ('cmp_other', 'acc_b', 'Other', 'linux_x86_64', '0.8.0', '{}', 'root', true, false, NULL)`,
	}
	for _, query := range queries {
		if _, err := conn.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(ctx, `INSERT INTO slot_reservations
	    (id, account_id, device_id, expires_at, consumed_at, released_at)
	    VALUES ('rsv_released', 'acc_a', 'dev_a', $2, NULL, $1),
	           ('rsv_consumed', 'acc_a', 'dev_a', $2, $1, NULL),
	           ('rsv_expired', 'acc_a', 'dev_a', $1, NULL, NULL)`, now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
}

func assertAuditRetention(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	var types []string
	if err := conn.QueryRow(t.Context(), "SELECT array_agg(type) FROM security_events").Scan(&types); err != nil {
		t.Fatal(err)
	}
	if len(types) != 1 || types[0] != "gate_policy_changed" {
		t.Fatalf("remaining audit rows = %v", types)
	}
	rows, err := conn.Query(t.Context(), "SELECT type, count FROM security_event_counts")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	counts := map[string]int64{}
	for rows.Next() {
		var typ string
		var count int64
		if err := rows.Scan(&typ, &count); err != nil {
			t.Fatal(err)
		}
		counts[typ] = count
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(counts, map[string]int64{"device_enrolled": 2, "device_revoked": 1}) {
		t.Fatalf("compacted counts = %v", counts)
	}
}
