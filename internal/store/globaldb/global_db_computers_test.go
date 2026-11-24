package globaldb

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/remoteproto"
)

func TestControllerComputers(t *testing.T) {
	t.Parallel()
	t.Run("Should persist an offline linked computer", func(t *testing.T) {
		t.Parallel()
		// UT-030. The owning SQLite registry preserves offline adds across registry restart.
		db := openTestGlobalDB(t)
		accounts := newComputerStoreAccount(t, "account-a")
		cloud := &computerStoreCloud{rows: []remoteproto.AccountComputer{{
			ID: "cmp_offline", AccountID: "account-a", Name: "Offline",
		}}}
		registry := computers.New(t.Context(), db, accounts, cloud)
		t.Cleanup(registry.Close)
		view, err := registry.AddFromAccount(t.Context(), "cmp_offline")
		if err != nil || view.Status != computers.Offline {
			t.Fatalf("offline add = %s %v", view.Status, err)
		}
		registry.Close()
		reopened := computers.New(t.Context(), db, accounts, cloud)
		t.Cleanup(reopened.Close)
		view, err = reopened.Get(t.Context(), "cmp_offline")
		if err != nil || view.Status != computers.Offline || view.AccountID != "account-a" {
			t.Fatalf("restored offline computer = %+v %v", view, err)
		}
	})
	t.Run("Should refuse an account computer with only a hosted route on Free", func(t *testing.T) {
		t.Parallel()
		// UT-031. Plan refusal leaves the durable registry empty.
		db := openTestGlobalDB(t)
		accounts := newComputerStoreAccount(t, "account-a")
		cloud := &computerStoreCloud{rows: []remoteproto.AccountComputer{{
			ID: "cmp_hosted", AccountID: "account-a", Name: "Hosted", HostedHostname: "host.example.test",
		}}}
		registry := computers.New(t.Context(), db, accounts, cloud)
		t.Cleanup(registry.Close)
		_, err := registry.AddFromAccount(t.Context(), "cmp_hosted")
		if !errors.Is(err, computers.ErrPlanRequired) {
			t.Fatalf("hosted-only add = %v", err)
		}
		rows, err := db.ControllerComputers(t.Context(), "account-a")
		if err != nil || len(rows) != 0 {
			t.Fatalf("plan refusal wrote rows: %v %v", rows, err)
		}
	})
	t.Run("Should close the old registry subscription and load only the new account's computers", func(t *testing.T) {
		t.Parallel()
		// UT-093. Safety Invariant 11. A cursor from account A cannot replay into account B.
		db := openTestGlobalDB(t)
		ctx := t.Context()
		const a, b = "account-a", "account-b"
		for _, item := range []computers.Computer{
			{ID: "cmp_a", AccountID: a, Name: "A", AddedAt: time.Unix(1, 0)},
			{ID: "cmp_b", AccountID: b, Name: "B", AddedAt: time.Unix(1, 0)},
		} {
			if err := db.AddComputer(ctx, item); err != nil {
				t.Fatal(err)
			}
		}
		accounts := newComputerStoreAccount(t, a)
		registry := computers.New(ctx, db, accounts, &computerStoreCloud{})
		t.Cleanup(registry.Close)
		old, err := registry.Subscribe(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		defer old.Close()
		if _, err := registry.Update(ctx, "cmp_a", computers.Patch{Name: new("A edited")}); err != nil {
			t.Fatal(err)
		}
		cursor := (<-old.Events).ID
		accounts.switchTo(ctx, b)
		if rows, err := registry.List(ctx); err != nil || len(rows) != 1 || rows[0].ID != "cmp_b" {
			t.Fatalf("new account registry = %v %v", rows, err)
		}
		if _, err := registry.Get(ctx, "cmp_a"); !errors.Is(err, computers.ErrNotFound) {
			t.Fatalf("old computer lookup = %v", err)
		}
		select {
		case <-old.Done:
		default:
			t.Fatal("old account still owns an event subscription")
		}
		sub, err := registry.Subscribe(ctx, cursor)
		if err != nil {
			t.Fatal(err)
		}
		defer sub.Close()
		if len(sub.Replay) != 1 || sub.Replay[0].Event != "stream_reset" {
			t.Fatalf("foreign cursor replay = %+v", sub.Replay)
		}
	})
}

type computerStoreAccount struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	identity account.Installation
}

func newComputerStoreAccount(t *testing.T, owner string) *computerStoreAccount {
	t.Helper()
	a := &computerStoreAccount{}
	a.switchTo(t.Context(), owner)
	t.Cleanup(func() { a.cancel() })
	return a
}

func (a *computerStoreAccount) switchTo(ctx context.Context, owner string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
	a.ctx, a.cancel = context.WithCancel(ctx)
	a.identity = account.Installation{
		State: account.SignedIn, AccountID: owner, AuthGeneration: a.identity.AuthGeneration + 1,
		Profile: remoteproto.AccountSummary{AccountPlan: remoteproto.AccountPlan{Plan: "free"}},
	}
}

func (a *computerStoreAccount) IdentityLifetime() (context.Context, account.Installation) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ctx, a.identity
}

func (*computerStoreAccount) UseDevice(
	ctx context.Context,
	run func(context.Context, cloudclient.Credentials) error,
) error {
	return run(ctx, cloudclient.Credentials{})
}

type computerStoreCloud struct {
	computers.Cloud
	rows []remoteproto.AccountComputer
}

func (c *computerStoreCloud) Computers(
	context.Context,
	cloudclient.Credentials,
) ([]remoteproto.AccountComputer, error) {
	return c.rows, nil
}
