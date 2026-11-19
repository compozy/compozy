package session

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

type budgetHeldReplyAdmission struct {
	store.SessionPromptAdmissionStore
	held atomic.Bool
}

func (s *budgetHeldReplyAdmission) CommitSessionPromptDispatch(
	ctx context.Context, workspaceID, sessionID, key string, now time.Time,
) error {
	if s.held.Load() {
		return &store.DailyBudgetTurnBlockedError{}
	}
	return s.SessionPromptAdmissionStore.CommitSessionPromptDispatch(ctx, workspaceID, sessionID, key, now)
}

func TestDailyBudgetRetryKeepsReplyWatch(t *testing.T) {
	t.Run("Should deliver one completion notice after the same held send retries", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, _ := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		fence := &budgetHeldReplyAdmission{SessionPromptAdmissionStore: db}
		fence.held.Store(true)
		h.manager.promptAdmissionStore = fence
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			return completedSyntheticPromptEvents(req.TurnID), nil
		}
		opts := SendPromptOpts{
			Message:          "query",
			MessageID:        "budget-held-send",
			IdempotencyKey:   "budget-held-key",
			NotifyOnComplete: true,
			Origin: &acp.PromptOriginMeta{
				Kind:        "session",
				SessionID:   sender.ID,
				WorkspaceID: h.workspaceID,
				Hop:         3,
			},
		}
		if _, err := h.manager.SendPrompt(
			t.Context(),
			target.ID,
			opts,
		); !errors.Is(
			err,
			store.ErrDailyBudgetTurnBlocked,
		) {
			t.Fatalf("first send = %v, want budget hold", err)
		}
		id := store.ReplyWatchID(target.ID, opts.MessageID)
		watch, err := db.GetReplyWatch(t.Context(), id)
		if err != nil || watch.State != store.ReplyWatchArmed {
			t.Fatalf("watch after hold = %+v, err=%v", watch, err)
		}
		fence.held.Store(false)
		result, err := h.manager.SendPrompt(t.Context(), target.ID, opts)
		if err != nil || result.ReplyWatch == nil || result.ReplyWatch.ID != id {
			t.Fatalf("retry = %+v, err=%v", result, err)
		}
		collectEvents(t, result.Events)
		if _, err := h.manager.Resume(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		waitForCondition(t, "one budget-held reply delivered", func() bool {
			w, err := db.GetReplyWatch(t.Context(), id)
			return err == nil && w.State == store.ReplyWatchDelivered
		})
		replay, err := h.manager.SendPrompt(t.Context(), target.ID, opts)
		if err != nil || !replay.Replayed || replay.ReplyWatch == nil ||
			replay.ReplyWatch.State != store.ReplyWatchDelivered {
			t.Fatalf("replayed completion = %+v, err=%v", replay, err)
		}
	})
}

func TestDailyBudgetAppGoalControlPreservesPendingReports(t *testing.T) {
	t.Run("Should run Goal status without provider dispatch or consuming a report at cap zero", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openManagerInputQueueStore(t)
		handler := GoalCommandHandlerFunc(func(
			_ context.Context, _, _ string, caller PromptCaller, command GoalCommand,
		) (GoalDispatchDecision, error) {
			if !caller.AppIngress || command.Verb != GoalCommandVerbStatus {
				t.Errorf("Goal control caller=%+v command=%+v", caller, command)
			}
			return GoalDispatchDecision{
				Kind: GoalDispatchRespond, Result: &GoalCommandResult{Outcome: GoalOutcomeStatus},
			}, nil
		})
		h := newHarness(t, WithSessionInputQueueStore(db), WithGoalCommandHandler(handler))
		registerManagerInputQueueWorkspace(t, db, h)
		item := createSession(t, h)
		registerManagerInputQueueSession(t, db, h, item)
		t.Cleanup(func() { reportSessionStop(t, h, item.ID) })
		instance, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID, AgentName: item.AgentName,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().BindMain(ctx, instance.ID, item.ID); err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, instance.ID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(0))}); err != nil {
			t.Fatal(err)
		}
		report, err := db.InstanceReports().AppendReport(ctx, instance.ID, agentinstance.ReportInput{
			SourceKey: "occ:budget-control",
			Kind:      "failure",
			Payload:   []byte(`{"reason":"fixture","detail":"kept for the next provider turn"}`),
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.InstanceReports().MarkReportPublished(ctx, report.ID, item.ID, 1); err != nil {
			t.Fatal(err)
		}
		result, err := h.manager.SendPrompt(ctx, item.ID, SendPromptOpts{
			Message: "/goal status", MessageID: "app-goal-control", IdempotencyKey: "app-goal-control",
			AllowCommands: true, Caller: PromptCaller{Kind: "human", ID: "operator", Source: "http", AppIngress: true},
		})
		if err != nil || result.Goal == nil {
			t.Fatalf("Goal control = %+v, err=%v", result, err)
		}
		h.driver.mu.Lock()
		prompts := len(h.driver.promptCalls)
		h.driver.mu.Unlock()
		after, err := db.Instances().Get(ctx, instance.ID)
		if prompts != 0 || err != nil || after.State != "active" {
			t.Fatalf("provider calls=%d, instance=%+v, err=%v", prompts, after, err)
		}
		pending, err := db.InstanceReports().ListReports(ctx, agentinstance.ReportQuery{
			InstanceID: instance.ID, State: "pending", Limit: 10,
		})
		if err != nil || len(pending) != 1 || pending[0].ID != report.ID {
			t.Fatalf("pending reports = %+v, err=%v", pending, err)
		}
	})
}
