//go:build integration

package control_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestCloudFanout(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should deliver committed events across replicas and recover the direct listener (IT-105)",
		func(t *testing.T) {
			t.Parallel()
			db := remotetest.NewDatabase(t)
			db.Migrate(t)
			first := remotetest.StartReplica(t, cloud.Config{Postgres: db.Config(t, "first")}, nil)
			second := remotetest.StartReplica(t, cloud.Config{Postgres: db.Config(t, "second")}, nil)
			one := subscribe(t, first.App.Control, "acc_a", "cmp_one")
			two := subscribe(t, second.App.Control, "acc_a", "cmp_two")
			foreign := subscribe(t, second.App.Control, "acc_b", "cmp_foreign")

			revoke := store.ControlEvent{
				Type: remoteproto.ControlRevokeDevice, AccountID: "acc_a", SecurityEpoch: 8, ResourceID: "dev_gone",
			}
			rolledBack := errors.New("rollback audit and notification")
			err := second.App.Store.Transaction(t.Context(), func(q *store.Queries) error {
				at := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
				if err := q.AppendSecurityEvent(t.Context(), store.SecurityEvent{
					AccountID: "acc_a", Actor: "system", At: at, Type: "device_revoked",
				}); err != nil {
					return err
				}
				if err := q.NotifyControlEvent(t.Context(), revoke); err != nil {
					return err
				}
				return rolledBack
			})
			if !errors.Is(err, rolledBack) {
				t.Fatalf("rollback = %v", err)
			}

			revoke.SecurityEpoch++
			commitEvent(t, second.App.Store, revoke)
			expectEvent(t, one, revoke)
			expectEvent(t, two, revoke)

			approval := store.ControlEvent{
				Type: remoteproto.ControlApprovalDecided, AccountID: "acc_a", ComputerID: "cmp_two",
				ResourceID: "apr_one", SecurityEpoch: 9,
			}
			commitEvent(t, first.App.Store, approval)
			expectEvent(t, two, approval)

			peers := store.ControlEvent{Type: remoteproto.ControlPeers, AccountID: "acc_a"}
			commitEvent(t, first.App.Store, peers)
			expectEvent(t, one, peers)
			expectEvent(t, two, peers)

			keys := store.ControlEvent{Type: remoteproto.ControlJWKS}
			commitEvent(t, first.App.Store, keys)
			expectEvent(t, one, keys)
			expectEvent(t, two, keys)
			expectEvent(t, foreign, keys)

			oldPID := directListenerPID(t, db, "first")
			var stopped bool
			err = db.Pool.QueryRow(t.Context(), "SELECT pg_terminate_backend($1)", oldPID).Scan(&stopped)
			if err != nil || !stopped {
				t.Fatalf("terminate LISTEN backend = %t, %v", stopped, err)
			}
			select {
			case reason := <-one.Closing:
				if reason != control.Resync {
					t.Fatalf("listener disconnect closed link with %q", reason)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("subscriber was not told to resync after losing LISTEN")
			}
			ready, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if err := first.App.Control.WaitReady(ready); err != nil {
				t.Fatal(err)
			}
			if newPID := directListenerPID(t, db, "first"); newPID == oldPID {
				t.Fatalf("listener reused terminated backend %d", oldPID)
			}
			after := subscribe(t, first.App.Control, "acc_a", "cmp_one")
			snapshot := store.ControlEvent{Type: remoteproto.ControlSnapshot, AccountID: "acc_a", SecurityEpoch: 10}
			commitEvent(t, second.App.Store, snapshot)
			expectEvent(t, after, snapshot)
			assertPooledConnections(t, db)
		},
	)
}

func commitEvent(t *testing.T, db *store.Store, event store.ControlEvent) {
	t.Helper()
	ctx := t.Context()
	if err := db.Transaction(
		ctx,
		func(q *store.Queries) error { return q.NotifyControlEvent(ctx, event) },
	); err != nil {
		t.Fatalf("publish %+v: %v", event, err)
	}
}

func subscribe(t *testing.T, hub *control.Hub, account string, computer remoteproto.ComputerID) *control.Subscription {
	t.Helper()
	sub, ok := hub.Subscribe(account, computer)
	if !ok {
		t.Fatalf("subscribe %s/%s", account, computer)
	}
	t.Cleanup(func() { hub.Release(sub) })
	return sub
}

func expectEvent(t *testing.T, sub *control.Subscription, want store.ControlEvent) {
	t.Helper()
	select {
	case event := <-sub.Events:
		if event != want {
			t.Fatalf("event = %+v, want %+v", event, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("event %+v did not arrive within 2s", want)
	}
}

func directListenerPID(t *testing.T, db *remotetest.Database, replica string) int32 {
	t.Helper()
	var pid int32
	err := db.Pool.QueryRow(t.Context(),
		"SELECT pid FROM pg_stat_activity WHERE application_name = $1 AND query = 'LISTEN cloud_events'",
		db.ApplicationName(replica, "direct"),
	).Scan(&pid)
	if err != nil {
		t.Fatalf("direct LISTEN session: %v", err)
	}
	return pid
}

func assertPooledConnections(t *testing.T, db *remotetest.Database) {
	t.Helper()
	var names int
	err := db.Pool.QueryRow(t.Context(),
		"SELECT count(DISTINCT application_name) FROM pg_stat_activity WHERE application_name IN ($1, $2)",
		db.ApplicationName("first", "pooled"), db.ApplicationName("second", "pooled"),
	).Scan(&names)
	if err != nil || names != 2 {
		t.Fatalf("pooled request sessions = %d, %v", names, err)
	}
}
