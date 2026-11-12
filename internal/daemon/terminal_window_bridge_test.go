package daemon

// Invariant: agent terminal lifecycle cannot litter durable desktops; this bridge suite owns event composition.
import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/windowmanager"
)

const bridgeTestWorkspace = "workspace-a"

type staticWindowManagerProvider struct {
	manager  *windowmanager.Manager
	err      error
	failures atomic.Int32
}

func (p *staticWindowManagerProvider) For(string) (*windowmanager.Manager, error) {
	if p.failures.Add(-1) >= 0 {
		return nil, errors.New("transient provider failure")
	}
	if p.err != nil {
		return nil, p.err
	}
	return p.manager, nil
}

func newBridgeTestManager(t *testing.T) *windowmanager.Manager {
	t.Helper()
	manager, err := windowmanager.NewService(
		windowmanager.NewMemoryRepository(),
		windowmanager.NewMemoryWorkspaceResolver(bridgeTestWorkspace),
		nil,
		windowmanager.DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := manager.Close(); closeErr != nil {
			t.Errorf("Manager.Close() error = %v", closeErr)
		}
	})
	return manager
}

func TestAttachTerminalWindowBridge(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		kind      terminalpkg.EventKind
		origin    terminalpkg.ActorKind
		remaining int
	}{
		{terminalpkg.EventKindClosed, terminalpkg.ActorKindAgent, 0},
		{terminalpkg.EventKindDeleted, terminalpkg.ActorKindAgent, 0},
		{terminalpkg.EventKindClosed, terminalpkg.ActorKindHuman, 1},
	} {
		t.Run("Should reconcile "+string(testCase.origin)+" windows on "+string(testCase.kind), func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			manager := newBridgeTestManager(t)
			notifier := terminalpkg.NewNotifier(discardLogger())
			terminals, err := terminalpkg.NewManager(
				terminalpkg.WithNotifier(notifier),
				terminalpkg.WithJournal(nativeTerminalJournalStub{}),
			)
			if err != nil {
				t.Fatal(err)
			}
			provider := &staticWindowManagerProvider{manager: manager}
			worker := attachTerminalWindowBridge(
				ctx,
				terminals,
				provider,
				discardLogger(),
			)
			t.Cleanup(func() {
				if err := stopOwnedWorker(context.Background(), "test terminal windows", worker); err != nil {
					t.Errorf("stop bridge: %v", err)
				}
			})
			before, err := manager.Snapshot(ctx, bridgeTestWorkspace)
			if err != nil {
				t.Fatal(err)
			}
			for range 5 {
				notifier.Notify(
					ctx,
					terminalpkg.Event{
						Kind:        terminalpkg.EventKindOpened,
						WorkspaceID: bridgeTestWorkspace,
						ProfileID:   "profile-a",
						Origin:      terminalpkg.Actor{Kind: testCase.origin},
						Actor:       terminalpkg.Actor{Kind: terminalpkg.ActorKindAgent},
						Detail:      &terminalpkg.EventDetail{Mode: terminalpkg.ModePTY},
					},
				)
			}
			after, err := manager.Snapshot(ctx, bridgeTestWorkspace)
			if err != nil {
				t.Fatal(err)
			}
			if after.Revision != before.Revision || len(after.Windows) != 0 {
				t.Fatal("agent opens changed desktop")
			}
			id := "term-0000000000ab"
			_, err = manager.Execute(
				ctx,
				windowmanager.CommandRequest{
					WorkspaceID:      bridgeTestWorkspace,
					ExpectedRevision: after.Revision,
					Actor:            windowmanager.Actor{Kind: "system", ID: "test"},
					Payload: windowmanager.OpenWindowCommand{
						Window: windowmanager.WindowSpec{
							App:         "terminal",
							InstanceKey: &id,
							Route: windowmanager.RouteIntent{
								Pathname: "/terminal/" + id,
								Search:   windowmanager.RouteSearch{},
							},
						},
					},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			var updates <-chan windowmanager.SubscriptionUpdate
			if testCase.remaining == 0 {
				seeded, err := manager.Snapshot(ctx, bridgeTestWorkspace)
				if err != nil {
					t.Fatal(err)
				}
				subscription, err := manager.Subscribe(
					ctx,
					windowmanager.SubscriptionRequest{WorkspaceID: bridgeTestWorkspace, AfterRevision: seeded.Revision},
				)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := subscription.Close(); err != nil {
						t.Errorf("Close subscription: %v", err)
					}
				})
				updates = subscription.Updates()
				provider.failures.Store(1)
			}
			notifier.Notify(
				ctx,
				terminalpkg.Event{
					Kind:        testCase.kind,
					WorkspaceID: bridgeTestWorkspace,
					ProfileID:   "profile-a",
					TerminalID:  terminalpkg.ID(id),
					Origin:      terminalpkg.Actor{Kind: testCase.origin},
					Actor:       terminalpkg.Actor{Kind: terminalpkg.ActorKindAgent},
				},
			)
			if updates != nil {
				waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				select {
				case update := <-updates:
					if update.Event == nil {
						t.Fatal("missing retirement event")
					}
				case <-waitCtx.Done():
					t.Fatal("retirement was not retried")
				}
			}
			after, err = manager.Snapshot(ctx, bridgeTestWorkspace)
			if err != nil {
				t.Fatal(err)
			}
			if len(after.Windows) != testCase.remaining {
				t.Fatalf("stale windows: %+v", after.Windows)
			}
			if testCase.remaining != 0 {
				return
			}
			for _, entries := range [][]windowmanager.HistoryEntry{after.History.Undo, after.History.Redo} {
				for _, entry := range entries {
					if len(entry.Before.Windows) != 0 || len(entry.After.Windows) != 0 {
						t.Fatal("history can resurrect terminal")
					}
				}
			}
		})
	}
}
