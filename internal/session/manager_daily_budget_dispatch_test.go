package session

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

type budgetReadinessTestNotifier struct {
	*fakeNotifier
	check func(context.Context, string) error
}

func (n *budgetReadinessTestNotifier) EnsureDailyBudgetReady(ctx context.Context, sessionID string) error {
	return n.check(ctx, sessionID)
}

func TestQueuedDailyBudgetInputRecoversPendingUsageBeforeClaim(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail bool
	}{
		{name: "Should recover a failed Done receipt before claiming the queued turn"},
		{name: "Should preserve the queued turn when receipt recovery is unavailable", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openManagerInputQueueStore(t)
			h := newHarness(t, WithSessionInputQueueStore(db))
			registerManagerInputQueueWorkspace(t, db, h)
			sess := createSession(t, h)
			registerManagerInputQueueSession(t, db, h, sess)
			t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
			instance, err := db.Instances().Adopt(t.Context(), agentinstance.Instance{
				ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: h.workspaceID,
				PermissionCeiling: "{}", CreatedByKind: "human",
			}, sess.ID, 10)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Instances().SetDailyBudgetLimits(t.Context(), instance.ID,
				agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}); err != nil {
				t.Fatal(err)
			}
			at := time.Now().UTC()
			entry, _, err := db.EnqueueSessionInput(t.Context(), store.SessionInputQueueInsert{
				ID: "budget-queued-after-done", SessionID: sess.ID, Text: "autonomous next turn",
				QueueCap: 2, Now: at,
			})
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			h.manager.notifier = &budgetReadinessTestNotifier{fakeNotifier: h.notifier,
				check: func(ctx context.Context, sessionID string) error {
					attempt := calls.Add(1)
					if tc.fail {
						return errors.New("canonical Done receipt unavailable")
					}
					if attempt > 1 {
						return nil
					}
					count := int64(100)
					stats := store.TokenStatsUpdate{SessionID: sessionID, TurnID: "unobserved-done",
						AgentName: "coder", TotalTokens: &count, CostStatus: "unknown",
						CostSource: "none", UpdatedAt: at}
					daily := store.TokenUsageDailyUpdate{ProfileID: store.DefaultProfileID,
						Day: store.LocalDay(at), AgentName: "coder", TotalTokens: &count,
						CostStatus: "unknown", CostSource: "none", UpdatedAt: at}
					_, err := db.RecordTokenUsageOnce(ctx, stats, daily)
					return err
				},
			}
			h.manager.startNextQueuedInputPrompt(sess.ID)
			stored, err := db.GetSessionInputQueueEntry(t.Context(), sess.ID, entry.ID)
			if err != nil || stored.Status != store.SessionInputQueueStatusQueued {
				t.Fatalf("unstarted budget input=%+v err=%v, must stay queued", stored, err)
			}
			if got := len(managerPromptCalls(h)); got != 0 {
				t.Fatalf("driver invoked %d times before recovery/admission", got)
			}
			if calls.Load() == 0 {
				t.Fatal("budget recovery was not attempted before queue claim")
			}
			if !tc.fail {
				paused, err := db.Instances().Get(t.Context(), instance.ID)
				if err != nil || paused.PauseReason != "daily_budget" {
					t.Fatalf("recovered cap pause=%+v err=%v", paused, err)
				}
			}
		})
	}
}

func TestDaemonLocalPromptChecksInstanceDailyBudgetBeforeDriver(t *testing.T) {
	t.Run("Should block daemon-local prompts before driver dispatch", func(t *testing.T) {
		t.Parallel()
		db := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(db))
		registerManagerInputQueueWorkspace(t, db, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, db, h, sess)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		instance, err := db.Instances().Adopt(t.Context(), agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: h.workspaceID,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, sess.ID, 10)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(t.Context(), instance.ID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(0))}); err != nil {
			t.Fatal(err)
		}
		var pausedFor string
		h.manager.SetInstancePauseHandler(func(_ context.Context, trip store.InstancePauseTrip) error {
			pausedFor = trip.InstanceID
			return nil
		})
		if _, err := h.manager.Prompt(
			t.Context(),
			sess.ID,
			"proactive heartbeat",
		); !errors.Is(
			err,
			store.ErrDailyBudgetTurnBlocked,
		) {
			t.Fatalf("daemon-local prompt error = %v, want budget rejection", err)
		}
		if got := len(managerPromptCalls(h)); got != 0 {
			t.Fatalf("driver invoked %d times despite the cap", got)
		}
		paused, err := db.Instances().Get(t.Context(), instance.ID)
		if err != nil || paused.State != "paused" || paused.PauseReason != "daily_budget" ||
			pausedFor != instance.ID {
			t.Fatalf("committed pause / effect: %+v, effect=%q, err=%v", paused, pausedFor, err)
		}
	})
}

func TestNativeCompactionChecksInstanceDailyBudgetBeforeDriver(t *testing.T) {
	t.Run("Should block native compaction before driver dispatch", func(t *testing.T) {
		t.Parallel()
		db := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(db))
		registerManagerInputQueueWorkspace(t, db, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, db, h, sess)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		sess.replaceAdvertisedCommands([]store.SessionAdvertisedCommand{{Name: "compact"}}, h.manager.now())
		instance, err := db.Instances().Adopt(t.Context(), agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: h.workspaceID,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, sess.ID, 10)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(t.Context(), instance.ID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(0))}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := h.manager.RequestCompaction(
			t.Context(),
			sess.ID,
		); !errors.Is(
			err,
			store.ErrDailyBudgetTurnBlocked,
		) {
			t.Fatalf("native compaction error = %v, want budget block", err)
		}
		if got := len(managerPromptCalls(h)); got != 0 {
			t.Fatalf("compaction invoked provider %d times while exhausted", got)
		}
	})
}

func TestOperatorCompactionDoesNotConsumeAutonomousInstanceBudget(t *testing.T) {
	t.Run("Should allow authenticated operator maintenance with an exhausted budget", func(t *testing.T) {
		t.Parallel()
		db := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(db))
		registerManagerInputQueueWorkspace(t, db, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, db, h, sess)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		sess.replaceAdvertisedCommands([]store.SessionAdvertisedCommand{{Name: "compact"}}, h.manager.now())
		instance, err := db.Instances().Adopt(t.Context(), agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: h.workspaceID,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, sess.ID, 10)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(t.Context(), instance.ID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(0))}); err != nil {
			t.Fatal(err)
		}
		ctx := WithCompactionRequestedBy(WithOperatorCompaction(t.Context()), "cli")
		_, events, err := h.manager.RequestCompaction(ctx, sess.ID)
		if err != nil {
			t.Fatalf("operator maintenance failed behind daily budget: %v", err)
		}
		collectEvents(t, events)
		if calls := len(managerPromptCalls(h)); calls != 1 {
			t.Fatalf("operator maintenance provider calls=%d, want one", calls)
		}
	})
}
