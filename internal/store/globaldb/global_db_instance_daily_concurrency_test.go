package globaldb

import (
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

func TestDailyBudgetConcurrentOwnedTurnsAndReceipts(t *testing.T) {
	t.Run("Should count overlapping owned turns once and fence subsequent dispatch and resume", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return at }
		main := budgetBoundSession(t, db, at)
		const child = "budget-concurrent-child"
		const instanceID = "daily-dispatch-instance"
		stamp := store.FormatTimestamp(at)
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO sessions
			(id, profile_id, scope, agent_name, workspace_id, session_type, state,
			 parent_session_id, root_session_id, lineage_kind, created_at, updated_at)
			SELECT ?, profile_id, scope, agent_name, workspace_id, session_type, state,
			 id, id, 'spawn', ?, ? FROM sessions WHERE id=?`, child, stamp, stamp, main); err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, instanceID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(110))}); err != nil {
			t.Fatal(err)
		}
		type admissionResult struct {
			allowed bool
			paused  bool
			err     error
		}
		admitBoth := func() [2]admissionResult {
			start := make(chan struct{})
			results := make(chan admissionResult, 2)
			for _, sessionID := range []string{main, child} {
				go func() {
					<-start
					allowed, trip, err := db.Instances().AdmitDailyBudgetTurn(ctx, sessionID)
					results <- admissionResult{allowed: allowed, paused: trip != nil, err: err}
				}()
			}
			close(start)
			return [2]admissionResult{<-results, <-results}
		}
		for _, result := range admitBoth() {
			if result.err != nil || !result.allowed || result.paused {
				t.Fatalf("turn admitted at 100 of 110 tokens = %+v", result)
			}
		}
		startReceipts := make(chan struct{})
		receiptErrors := make(chan error, 4)
		for _, sessionID := range []string{main, child, child, main} {
			go func() {
				<-startReceipts
				stats := store.TokenStatsUpdate{SessionID: sessionID, TurnID: "overlapping-done",
					AgentName: "coder", TotalTokens: new(int64(7)), CostStatus: "unknown",
					CostSource: "none", Turns: 1, UpdatedAt: at}
				daily := store.TokenUsageDailyUpdate{ProfileID: store.DefaultProfileID, Day: store.LocalDay(at),
					AgentName: stats.AgentName, TotalTokens: stats.TotalTokens, CostStatus: stats.CostStatus,
					CostSource: stats.CostSource, Turns: 1, UpdatedAt: at}
				_, err := db.RecordTokenUsageOnce(ctx, stats, daily)
				receiptErrors <- err
			}()
		}
		close(startReceipts)
		for range 4 {
			if err := <-receiptErrors; err != nil {
				t.Fatal(err)
			}
		}
		usage, err := db.Instances().ReadDailyBudgetUsage(ctx, instanceID, at)
		if err != nil || usage.Tokens != 114 {
			t.Fatalf("confirmed usage=%+v err=%v, want 114 after exactly two 7-token turns", usage, err)
		}
		held := admitBoth()
		if held[0].err != nil || held[1].err != nil || held[0].allowed || held[1].allowed ||
			held[0].paused == held[1].paused {
			t.Fatalf("subsequent admissions=%+v, want both held with one pause transition", held)
		}
		startResume := make(chan struct{})
		resumeErrors := make(chan error, 2)
		for range 2 {
			go func() {
				<-startResume
				_, err := db.Instances().Resume(ctx, instanceID, 10, true)
				resumeErrors <- err
			}()
		}
		close(startResume)
		for range 2 {
			if err := <-resumeErrors; !errors.Is(err, agentinstance.ErrPaused) {
				t.Fatalf("parallel operator resume=%v, want exhausted pause", err)
			}
		}
	})
}
