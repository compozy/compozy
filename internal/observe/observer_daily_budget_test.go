package observe

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

type failFirstCanonicalReceiptRegistry struct {
	*globaldb.GlobalDB
	fail atomic.Bool
}

type blockingFailedReceiptRegistry struct {
	*globaldb.GlobalDB
	entered chan struct{}
	release chan struct{}
}

func (r *blockingFailedReceiptRegistry) RecordTokenUsageOnce(
	ctx context.Context, _ store.TokenStatsUpdate, _ store.TokenUsageDailyUpdate,
) (bool, error) {
	close(r.entered)
	select {
	case <-r.release:
		return false, errors.New("global receipt write failed after durable Done")
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func TestObserverDailyBudgetAdmissionCannotPassAnOverlappingFailedReceipt(t *testing.T) {
	t.Run("Should serialize final admission with failure marking and durable recovery", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		clock := time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)
		h := newHarness(t, globaldb.WithClock(func() time.Time { return clock }))
		instance, _, err := h.registry.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
			AgentName: "coder", CreatedByKind: "human", PermissionCeiling: "{}",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		sess := newSession("budget-overlapping-failure", session.StateActive, h.workspace, clock)
		sess.InstanceID = instance.ID
		h.observeSessionCreated(t, sess)
		if _, err := h.registry.Instances().BindMain(ctx, instance.ID, sess.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.registry.Instances().SetDailyBudgetLimits(ctx, instance.ID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(85))}); err != nil {
			t.Fatal(err)
		}
		usage := &acp.TokenUsage{TurnID: "overlapping-done", InputTokens: new(int64(85)), Timestamp: clock}
		event := acp.AgentEvent{Type: acp.EventTypeDone, TurnID: usage.TurnID, Usage: usage, Timestamp: clock}
		content, err := transcript.MarshalAgentEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		ledger, err := sessiondb.OpenSessionDB(ctx,
			store.SessionDBOwner{SessionID: sess.ID, WorkspaceID: h.workspaceID},
			store.SessionDBFile(filepath.Join(h.home.SessionsDir, sess.ID)))
		if err != nil {
			t.Fatal(err)
		}
		_, _, appendErr := ledger.AppendEventIfAbsent(ctx, store.SessionEvent{
			ID: "overlapping-event", SessionID: sess.ID, TurnID: event.TurnID,
			Type: event.Type, AgentName: sess.AgentName, Content: content, Timestamp: clock,
		})
		if err := errors.Join(appendErr, ledger.Close(ctx)); err != nil {
			t.Fatal(err)
		}
		registry := &blockingFailedReceiptRegistry{
			GlobalDB: h.registry, entered: make(chan struct{}), release: make(chan struct{}),
		}
		h.observer.registry = registry
		var workers sync.WaitGroup
		var release sync.Once
		releaseReceipt := func() { release.Do(func() { close(registry.release) }) }
		defer func() {
			cancel()
			releaseReceipt()
			workers.Wait()
		}()
		observed := make(chan struct{})
		workers.Go(func() {
			h.observer.OnAgentEvent(ctx, sess.ID, event)
			close(observed)
		})
		select {
		case <-registry.entered:
		case <-ctx.Done():
			t.Fatal("provider Done never reached receipt projection")
		}
		admitted := make(chan struct {
			allowed bool
			err     error
		}, 1)
		if h.observer.budgetAdmissionMu.TryRLock() {
			h.observer.budgetAdmissionMu.RUnlock()
			t.Fatal("receipt projection is not fenced against admission")
		}
		workers.Go(func() {
			var allowed bool
			err := h.observer.WithDailyBudgetAdmission(ctx, sess.ID, func(ctx context.Context) error {
				var commitErr error
				allowed, _, commitErr = h.registry.Instances().AdmitDailyBudgetTurn(ctx, sess.ID)
				return commitErr
			})
			admitted <- struct {
				allowed bool
				err     error
			}{allowed: allowed, err: err}
		})
		releaseReceipt()
		select {
		case <-observed:
		case <-ctx.Done():
			t.Fatal("failed observer did not exit")
		}
		select {
		case result := <-admitted:
			if result.err != nil || result.allowed {
				t.Fatalf("admission after failed receipt=%+v, want recovered cap block", result)
			}
		case <-ctx.Done():
			t.Fatal("budget admission did not recover after failed Done")
		}
	})
}

func TestObserverDailyBudgetPreDeletePreservesUnbudgetedSpendForLaterActivation(t *testing.T) {
	for _, ledgerPresent := range []bool{true, false} {
		name := "Should backfill an unbudgeted owned session before deletion"
		if !ledgerPresent {
			name = "Should allow opt-out cleanup but fence retroactive caps after ledger loss"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.Context(t)
			clock := time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)
			h := newHarness(t, globaldb.WithClock(func() time.Time { return clock }))
			instance, _, err := h.registry.Instances().AdmitCreate(ctx, agentinstance.Instance{
				ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
				AgentName: "coder", CreatedByKind: "human", PermissionCeiling: "{}",
			}, 10)
			if err != nil {
				t.Fatal(err)
			}
			sess := newSession("budget-pre-delete", session.StateStopped, h.workspace, clock)
			sess.InstanceID = instance.ID
			h.observeSessionCreated(t, sess)
			if _, err := h.registry.Instances().BindMain(ctx, instance.ID, sess.ID); err != nil {
				t.Fatal(err)
			}
			if ledgerPresent {
				usage := &acp.TokenUsage{
					TurnID:      "unnoticed-before-delete",
					InputTokens: new(int64(85)),
					Timestamp:   clock,
				}
				event := acp.AgentEvent{Type: acp.EventTypeDone, TurnID: usage.TurnID, Usage: usage, Timestamp: clock}
				content, err := transcript.MarshalAgentEvent(event)
				if err != nil {
					t.Fatal(err)
				}
				ledger, err := sessiondb.OpenSessionDB(ctx,
					store.SessionDBOwner{SessionID: sess.ID, WorkspaceID: h.workspaceID},
					store.SessionDBFile(filepath.Join(h.home.SessionsDir, sess.ID)))
				if err != nil {
					t.Fatal(err)
				}
				if _, _, err := ledger.AppendEventIfAbsent(ctx, store.SessionEvent{
					ID: "unnoticed-pre-delete", SessionID: sess.ID, TurnID: event.TurnID,
					Type: event.Type, AgentName: sess.AgentName, Content: content, Timestamp: clock,
				}); err != nil {
					t.Fatal(err)
				}
				if err := ledger.Close(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if err := h.observer.ReconcileBudgetSessionBeforeDeletion(ctx, sess.ID); err != nil {
				t.Fatalf("opt-out history cleanup: %v", err)
			}
			limit := agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}
			err = h.registry.Instances().SetDailyBudgetLimits(ctx, instance.ID, limit)
			if !ledgerPresent {
				if !errors.Is(err, agentinstance.ErrBudgetHistoryUncertain) {
					t.Fatalf("retroactive limit after lost history=%v, want conflict", err)
				}
				clock = clock.AddDate(0, 0, 1)
				if err := h.registry.Instances().SetDailyBudgetLimits(ctx, instance.ID, limit); err != nil {
					t.Fatalf("future day should be eligible: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			used, err := h.registry.Instances().ReadDailyBudgetUsage(ctx, instance.ID, clock)
			if err != nil || used.Tokens != 85 {
				t.Fatalf("pre-delete backfill usage=%+v err=%v, want 85", used, err)
			}
		})
	}
}

func TestObserverDailyBudgetRecoversYesterdayAfterMidnight(t *testing.T) {
	t.Run("Should write a missing previous-day receipt once and advance the durable replay cursor", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		today := time.Date(2026, 4, 4, 1, 0, 0, 0, time.Local)
		yesterday := today.Add(-2 * time.Hour)
		clock := yesterday.Add(-time.Hour)
		h := newHarness(t, globaldb.WithClock(func() time.Time { return clock }))
		instance, _, err := h.registry.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
			AgentName: "coder", CreatedByKind: "human", PermissionCeiling: "{}",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		sess := newSession("budget-after-midnight", session.StateStopped, h.workspace, yesterday)
		sess.InstanceID = instance.ID
		h.observeSessionCreated(t, sess)
		if _, err := h.registry.Instances().BindMain(ctx, instance.ID, sess.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.registry.Instances().SetDailyBudgetLimits(ctx, instance.ID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}); err != nil {
			t.Fatal(err)
		}
		clock = today
		usage := &acp.TokenUsage{TurnID: "yesterday-unseen", InputTokens: new(int64(85)), Timestamp: yesterday}
		event := acp.AgentEvent{Type: acp.EventTypeDone, TurnID: usage.TurnID, Usage: usage, Timestamp: yesterday}
		content, err := transcript.MarshalAgentEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		ledger, err := sessiondb.OpenSessionDB(ctx,
			store.SessionDBOwner{SessionID: sess.ID, WorkspaceID: h.workspaceID},
			store.SessionDBFile(filepath.Join(h.home.SessionsDir, sess.ID)))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := ledger.AppendEventIfAbsent(ctx, store.SessionEvent{
			ID: "yesterday-event", SessionID: sess.ID, TurnID: event.TurnID,
			Type: event.Type, AgentName: sess.AgentName, Content: content, Timestamp: yesterday,
		}); err != nil {
			t.Fatal(err)
		}
		if err := ledger.Close(ctx); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := h.observer.ReconcileDailyBudgetReceipts(ctx); err != nil {
				t.Fatal(err)
			}
		}
		previous, err := h.registry.Instances().ReadDailyBudgetUsage(ctx, instance.ID, yesterday)
		if err != nil || previous.Tokens != 85 {
			t.Fatalf("yesterday receipt=%+v err=%v", previous, err)
		}
		current, err := h.registry.Instances().ReadDailyBudgetUsage(ctx, instance.ID, today)
		if err != nil || current.Tokens != 0 {
			t.Fatalf("fresh day usage=%+v err=%v, want zero", current, err)
		}
		cursor, err := h.registry.DailyBudgetRecoveryCursor(ctx, sess.ID)
		if err != nil || cursor < 1 {
			t.Fatalf("durable replay sequence=%d err=%v", cursor, err)
		}
		var receipts int
		if err := h.registry.DB().QueryRowContext(ctx, `SELECT count(*) FROM token_usage_receipts
			WHERE session_id=?`, sess.ID).Scan(&receipts); err != nil || receipts != 1 {
			t.Fatalf("past-day receipts=%d err=%v, want one", receipts, err)
		}
	})
}

type dirtyDuringBudgetReconcileRegistry struct {
	*globaldb.GlobalDB
	observer *Observer
	calls    atomic.Int32
	always   bool
}

func (r *dirtyDuringBudgetReconcileRegistry) ListDailyBudgetReconcileSessions(
	ctx context.Context, instanceID string,
) ([]store.DailyBudgetReconcileSession, error) {
	items, err := r.GlobalDB.ListDailyBudgetReconcileSessions(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	count := r.calls.Add(1)
	if count == 1 || r.always {
		r.observer.mu.Lock()
		r.observer.dailyBudgetDirtyGeneration++
		r.observer.mu.Unlock()
	}
	return items, nil
}

func TestObserverDailyBudgetReadinessRetriesOverlappingReceiptFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		always bool
	}{
		{name: "Should rescan receipts that failed during an earlier recovery"},
		{name: "Should fail closed when new failures continuously race recovery", always: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			registry := &dirtyDuringBudgetReconcileRegistry{
				GlobalDB: h.registry, observer: h.observer, always: tc.always,
			}
			h.observer.registry = registry
			h.observer.mu.Lock()
			h.observer.dailyBudgetDirtyGeneration++
			h.observer.mu.Unlock()
			err := h.observer.EnsureDailyBudgetReady(testutil.Context(t), "")
			if tc.always {
				if err == nil || !strings.Contains(err.Error(), "daily budget receipts changed during recovery") ||
					registry.calls.Load() != 3 {
					t.Fatalf("unstable recovery err=%v scans=%d, want fail closed after 3", err, registry.calls.Load())
				}
			} else if err != nil || registry.calls.Load() != 2 {
				t.Fatalf("racing recovery err=%v scans=%d, want two scans", err, registry.calls.Load())
			}
		})
	}
}

func (r *failFirstCanonicalReceiptRegistry) RecordTokenUsageOnce(
	ctx context.Context, stats store.TokenStatsUpdate, daily store.TokenUsageDailyUpdate,
) (bool, error) {
	if r.fail.CompareAndSwap(true, false) {
		return false, errors.New("temporary global observer write failure")
	}
	return r.GlobalDB.RecordTokenUsageOnce(ctx, stats, daily)
}

func TestObserverCanonicalUsageDoesNotRecountReplayedDoneEvent(t *testing.T) {
	t.Run("Should count a replayed Done only once", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		sess := newSession("budget-usage-replay", session.StateActive, h.workspace, h.now)
		h.observeSessionCreated(t, sess)
		usage := &acp.TokenUsage{
			TurnID: "budget-turn", InputTokens: new(int64(75)),
			OutputTokens: new(int64(25)), Timestamp: h.now,
		}
		event := acp.AgentEvent{
			Type: acp.EventTypeDone, TurnID: "budget-turn", Usage: usage, Timestamp: h.now,
		}
		for range 3 {
			h.observer.OnAgentEvent(testutil.Context(t), sess.ID, event)
		}
		stat := h.singleTokenStat(t, sess.ID)
		if stat.TurnCount != 1 || stat.InputTokens == nil || *stat.InputTokens != 75 ||
			stat.OutputTokens == nil || *stat.OutputTokens != 25 {
			t.Fatalf("replayed usage aggregate = %+v, want exactly 100 tokens in one turn", stat)
		}
		var receipts int
		if err := h.registry.DB().QueryRowContext(testutil.Context(t),
			`SELECT count(*) FROM token_usage_receipts WHERE session_id = ? AND turn_id = ?`,
			sess.ID, event.TurnID).Scan(&receipts); err != nil || receipts != 1 {
			t.Fatalf("canonical receipt count = %d, err = %v", receipts, err)
		}
		stats, err := h.observer.QueryTokenStats(testutil.Context(t), store.TokenStatsQuery{SessionID: sess.ID})
		if err != nil || len(stats) != 1 {
			t.Fatalf("replayed session projection = %+v, err = %v", stats, err)
		}
	})
}

func TestObserverDailyBudgetRebuildsReceiptFromDurableDoneAfterRestart(t *testing.T) {
	t.Run("Should restore a priced receipt after restart", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		clock := time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)
		h := newHarness(t, globaldb.WithClock(func() time.Time { return clock }))
		oldRate, currentRate := 1.0, 10.0
		h.observer.costCatalog = &stubCostCatalog{models: []modelcatalog.Model{
			{ProviderID: "claude", ModelID: "old-priced", CostInputPerMillion: &oldRate,
				CostInputSource: modelcatalog.SourceKindConfig},
			{ProviderID: "claude", ModelID: "current-priced", CostInputPerMillion: &currentRate,
				CostInputSource: modelcatalog.SourceKindConfig},
		}}
		h.observer.resolveProviderAuth = fixedProviderAuthMode(compozyconfig.ProviderAuthModeBoundSecret)
		instance, _, err := h.registry.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
			AgentName: "coder", CreatedByKind: "human", PermissionCeiling: "{}",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		sess := newSession("budget-restart-ledger", session.StateActive, h.workspace, h.now)
		sess.Model = "current-priced" // session metadata changed after the old turn completed
		sess.InstanceID = instance.ID
		h.observeSessionCreated(t, sess)
		if _, err := h.registry.Instances().BindMain(ctx, instance.ID, sess.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.registry.Instances().SetDailyBudgetLimits(ctx, instance.ID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}); err != nil {
			t.Fatal(err)
		}
		usage := &acp.TokenUsage{TurnID: "crash-window", InputTokens: new(int64(85)), Timestamp: h.now}
		event := (acp.AgentEvent{Type: acp.EventTypeDone, TurnID: usage.TurnID,
			Usage: usage, Timestamp: h.now}).WithPromptRuntime(&acp.PromptRuntime{
			Provider: "claude", Model: "old-priced", MeteringAuthMode: string(compozyconfig.ProviderAuthModeNone),
		})
		content, err := transcript.MarshalAgentEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		ledger, err := sessiondb.OpenSessionDB(ctx,
			store.SessionDBOwner{SessionID: sess.ID, WorkspaceID: h.workspaceID},
			store.SessionDBFile(filepath.Join(h.home.SessionsDir, sess.ID)))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := ledger.AppendEventIfAbsent(ctx, store.SessionEvent{
			ID: "crash-event", SessionID: sess.ID, TurnID: event.TurnID,
			Type: event.Type, AgentName: sess.AgentName, Content: content, Timestamp: h.now,
		}); err != nil {
			t.Fatal(err)
		}
		if err := ledger.Close(ctx); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := h.observer.ReconcileDailyBudgetReceipts(ctx); err != nil {
				t.Fatal(err)
			}
		}
		h.observer.OnAgentEvent(ctx, sess.ID, event) // delayed notifier delivery after recovered event
		h.observer.resolveProviderAuth = func(context.Context, string, string, string, string) (
			compozyconfig.ProviderAuthMode, error,
		) {
			return "", errors.New("historical provider definition removed")
		}
		if _, err := h.registry.DB().
			ExecContext(ctx, `DELETE FROM daily_budget_recovery_cursors WHERE session_id = ?`, sess.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.observer.ReconcileDailyBudgetReceipts(ctx); err != nil {
			t.Fatalf("accepted replay must survive removed provider definition: %v", err)
		}
		read, err := h.registry.Instances().ReadDailyBudgetUsage(ctx, instance.ID, h.now)
		if err != nil || read.Tokens != 85 {
			t.Fatalf("recovered budget usage=%+v err=%v, want 85 exactly once", read, err)
		}
		if read.USD == nil || read.USD.FloatString(6) != "0.000085" {
			t.Fatalf("recovered USD=%v, want per-event old model cost 0.000085", read.USD)
		}
		var receipts, notices int
		if err := h.registry.DB().QueryRowContext(ctx,
			`SELECT count(*) FROM token_usage_receipts WHERE session_id=?`, sess.ID).Scan(&receipts); err != nil {
			t.Fatal(err)
		}
		if err := h.registry.DB().QueryRowContext(ctx,
			`SELECT count(*) FROM agent_instance_daily_budget_notices WHERE instance_id=?`, instance.ID).
			Scan(&notices); err != nil {
			t.Fatal(err)
		}
		if receipts != 1 || notices != 1 {
			t.Fatalf("recovery receipts=%d notices=%d, want exactly one each", receipts, notices)
		}
	})
}

func TestObserverDailyBudgetMissingHistoricalAuthCannotInventEstimatedUSD(t *testing.T) {
	t.Run("Should preserve unpriced tokens but accept explicit provider cost", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		clock := time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)
		h := newHarness(t, globaldb.WithClock(func() time.Time { return clock }))
		rate := 10.0
		h.observer.costCatalog = &stubCostCatalog{models: []modelcatalog.Model{{
			ProviderID: "claude", ModelID: "historical-native-cli", CostInputPerMillion: &rate,
			CostInputSource: modelcatalog.SourceKindConfig,
		}}}
		h.observer.resolveProviderAuth = func(context.Context, string, string, string, string) (
			compozyconfig.ProviderAuthMode, error,
		) {
			return compozyconfig.ProviderAuthModeNativeCLI, nil
		}
		instance, _, err := h.registry.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
			AgentName: "coder", CreatedByKind: "human", PermissionCeiling: "{}",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		sess := newSession("budget-historical-unknown-auth", session.StateActive, h.workspace, clock)
		sess.InstanceID = instance.ID
		sess.Provider, sess.Model = "claude", "historical-native-cli"
		h.observeSessionCreated(t, sess)
		if _, err := h.registry.Instances().BindMain(ctx, instance.ID, sess.ID); err != nil {
			t.Fatal(err)
		}
		limit := 0.05
		if err := h.registry.Instances().SetDailyBudgetLimits(ctx, instance.ID,
			agentinstance.DailyBudgetLimits{USD: &limit}); err != nil {
			t.Fatal(err)
		}
		reported := 0.01
		currency := "USD"
		for index, usage := range []*acp.TokenUsage{
			{TurnID: "unpriced-historical", InputTokens: new(int64(85)), Timestamp: clock},
			{TurnID: "replay-unpriced-historical", InputTokens: new(int64(9)), Timestamp: clock},
			{TurnID: "actual-historical", InputTokens: new(int64(1)), CostAmount: &reported,
				CostCurrency: &currency, Timestamp: clock},
		} {
			event := (acp.AgentEvent{Type: acp.EventTypeDone, TurnID: usage.TurnID,
				Usage: usage, Timestamp: clock}).WithPromptRuntime(&acp.PromptRuntime{
				Provider: "claude", Model: "historical-native-cli",
			})
			if index == 0 {
				h.observer.OnAgentEvent(ctx, sess.ID, event)
				used, err := h.registry.Instances().ReadDailyBudgetUsage(ctx, instance.ID, clock)
				if err != nil || used.Tokens != 85 || !used.HasUnpricedUsage {
					t.Fatalf("live historical usage=%+v err=%v, want 85 unpriced tokens", used, err)
				}
			}
			content, err := transcript.MarshalAgentEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			ledger, err := sessiondb.OpenSessionDB(ctx,
				store.SessionDBOwner{SessionID: sess.ID, WorkspaceID: h.workspaceID},
				store.SessionDBFile(filepath.Join(h.home.SessionsDir, sess.ID)))
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := ledger.AppendEventIfAbsent(ctx, store.SessionEvent{
				ID: "historical-event-" + usage.TurnID, SessionID: sess.ID, TurnID: usage.TurnID,
				Type: event.Type, AgentName: sess.AgentName, Content: content,
				Timestamp: clock.Add(time.Duration(index) * time.Second),
			}); err != nil {
				t.Fatal(err)
			}
			if err := ledger.Close(ctx); err != nil {
				t.Fatal(err)
			}
		}
		if err := h.observer.ReconcileDailyBudgetReceipts(ctx); err != nil {
			t.Fatal(err)
		}
		used, err := h.registry.Instances().ReadDailyBudgetUsage(ctx, instance.ID, clock)
		if err != nil || used.Tokens != 95 || !used.HasUnpricedUsage ||
			used.USD == nil || used.USD.FloatString(6) != "0.010000" {
			t.Fatalf("historical receipt usage=%+v err=%v; USD must include actual only", used, err)
		}
		var recoveredStatus string
		if err := h.registry.DB().QueryRowContext(ctx, `SELECT cost_status FROM token_usage_receipts
			WHERE session_id=? AND turn_id='replay-unpriced-historical'`, sess.ID).
			Scan(&recoveredStatus); err != nil || recoveredStatus != string(modelcatalog.CostStatusUnknown) {
			t.Fatalf("recovered historical cost status=%q err=%v, want unknown", recoveredStatus, err)
		}
		var notices int
		if err := h.registry.DB().QueryRowContext(ctx,
			`SELECT count(*) FROM agent_instance_daily_budget_notices WHERE instance_id=?`, instance.ID).
			Scan(&notices); err != nil || notices != 0 {
			t.Fatalf("invented price warning count=%d err=%v, want zero", notices, err)
		}
	})
}

func TestObserverLiveDailyBudgetUsesRecordedPromptRuntimePricing(t *testing.T) {
	t.Run("Should price a delayed Done using its original model instead of current metadata", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		oldRate, currentRate := 1.0, 10.0
		h.observer.costCatalog = &stubCostCatalog{models: []modelcatalog.Model{
			{ProviderID: "claude", ModelID: "old-priced", CostInputPerMillion: &oldRate,
				CostInputSource: modelcatalog.SourceKindConfig},
			{ProviderID: "claude", ModelID: "current-priced", CostInputPerMillion: &currentRate,
				CostInputSource: modelcatalog.SourceKindConfig},
		}}
		h.observer.resolveProviderAuth = fixedProviderAuthMode(compozyconfig.ProviderAuthModeBoundSecret)
		sess := newSession("budget-live-historical-model", session.StateActive, h.workspace, h.now)
		sess.Model = "current-priced"
		h.observeSessionCreated(t, sess)
		usage := &acp.TokenUsage{TurnID: "old-runtime-turn", InputTokens: new(int64(85)), Timestamp: h.now}
		event := (acp.AgentEvent{Type: acp.EventTypeDone, TurnID: usage.TurnID,
			Usage: usage, Timestamp: h.now}).WithPromptRuntime(&acp.PromptRuntime{
			Provider: "claude", Model: "old-priced",
			MeteringAuthMode: string(compozyconfig.ProviderAuthModeBoundSecret),
		})
		h.observer.OnAgentEvent(testutil.Context(t), sess.ID, event)
		stat := h.singleTokenStat(t, sess.ID)
		if stat.TotalCost == nil || *stat.TotalCost != 0.000085 {
			t.Fatalf("delayed Done cost=%v; want historical model's $0.000085", stat.TotalCost)
		}
	})
}

func TestObserverDailyBudgetQuarantinesLostLedgerWithoutBlockingOtherInstances(t *testing.T) {
	t.Run("Should quarantine one broken instance without blocking another", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		clock := time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)
		h := newHarness(t, globaldb.WithClock(func() time.Time { return clock }))
		for _, name := range []string{"unreadable", "healthy", "archived", "starting"} {
			instance, _, err := h.registry.Instances().AdmitCreate(ctx, agentinstance.Instance{
				ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
				AgentName: name, CreatedByKind: "human", PermissionCeiling: "{}",
			}, 10)
			if err != nil {
				t.Fatal(err)
			}
			id := "budget-quarantine-" + name
			if _, err := h.registry.RegisterInstanceSession(ctx, &store.SessionInfo{
				ID: id, ProfileID: store.DefaultProfileID,
				WorkspaceID: h.workspaceID, AgentName: name, Provider: "claude",
				RuntimeStatus: store.SessionRuntimeUnbound,
				State:         string(session.StateStopped), CreatedAt: h.now, UpdatedAt: h.now,
			}, instance.ID, nil); err != nil {
				t.Fatal(err)
			}
			if name != "starting" {
				if _, err := h.registry.Instances().BindMain(ctx, instance.ID, id); err != nil {
					t.Fatal(err)
				}
			}
			if err := h.registry.Instances().SetDailyBudgetLimits(ctx, instance.ID,
				agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}); err != nil {
				t.Fatal(err)
			}
			if name == "archived" {
				if _, err := h.registry.Instances().Archive(ctx, instance.ID); err != nil {
					t.Fatal(err)
				}
			}
			if name == "healthy" {
				usage := &acp.TokenUsage{TurnID: "healthy-turn", InputTokens: new(int64(85)), Timestamp: h.now}
				event := acp.AgentEvent{Type: acp.EventTypeDone, TurnID: usage.TurnID, Usage: usage, Timestamp: h.now}
				content, err := transcript.MarshalAgentEvent(event)
				if err != nil {
					t.Fatal(err)
				}
				ledger, err := sessiondb.OpenSessionDB(ctx,
					store.SessionDBOwner{SessionID: id, WorkspaceID: h.workspaceID},
					store.SessionDBFile(filepath.Join(h.home.SessionsDir, id)))
				if err != nil {
					t.Fatal(err)
				}
				if _, _, err := ledger.AppendEventIfAbsent(ctx, store.SessionEvent{
					ID: "healthy-recovery-event", SessionID: id, TurnID: event.TurnID,
					Type: event.Type, AgentName: name, Content: content, Timestamp: h.now,
				}); err != nil {
					t.Fatal(err)
				}
				if err := ledger.Close(ctx); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := h.observer.ReconcileDailyBudgetReceipts(ctx); err != nil {
			t.Fatalf("one lost ledger must not abort global recovery: %v", err)
		}
		instances, err := h.registry.Instances().List(ctx, agentinstance.ListQuery{
			ProfileID: store.DefaultProfileID, Limit: 100, AllWorkspaces: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		var quarantined, counted, startingQuarantined bool
		for _, instance := range instances {
			switch instance.AgentName {
			case "unreadable":
				quarantined = instance.State == "paused" && instance.PauseReason == "daily_budget_recovery"
			case "starting":
				generation, err := h.registry.DailyBudgetRecoveryGeneration(ctx, instance.ID)
				if err != nil {
					t.Fatal(err)
				}
				startingQuarantined = instance.State == "starting" && instance.MainSessionID == "" && generation > 0
			case "healthy":
				usage, usageErr := h.registry.Instances().ReadDailyBudgetUsage(ctx, instance.ID, clock)
				counted = usageErr == nil && usage.Tokens == 85
			}
		}
		if !quarantined || !counted || !startingQuarantined {
			t.Fatalf("lost-ledger quarantine=%t starting-quarantine=%t healthy-accounted=%t",
				quarantined, startingQuarantined, counted)
		}
	})
}

func TestObserverReconcilesCurrentDayBeforeEnablingInstanceBudget(t *testing.T) {
	t.Run("Should reconcile today's usage despite stale catalog metadata", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		clock := time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)
		h := newHarness(t, globaldb.WithClock(func() time.Time { return clock }))
		instance, _, err := h.registry.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
			AgentName: "coder", CreatedByKind: "human", PermissionCeiling: "{}",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		sess := newSession("budget-enable-midday", session.StateActive, h.workspace, h.now)
		sess.InstanceID = instance.ID
		h.observeSessionCreated(t, sess)
		if _, err := h.registry.Instances().BindMain(ctx, instance.ID, sess.ID); err != nil {
			t.Fatal(err)
		}
		usage := &acp.TokenUsage{TurnID: "prior-unobserved", InputTokens: new(int64(85)), Timestamp: h.now}
		event := acp.AgentEvent{Type: acp.EventTypeDone, TurnID: usage.TurnID, Usage: usage, Timestamp: h.now}
		content, err := transcript.MarshalAgentEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		ledger, err := sessiondb.OpenSessionDB(ctx,
			store.SessionDBOwner{SessionID: sess.ID, WorkspaceID: h.workspaceID},
			store.SessionDBFile(filepath.Join(h.home.SessionsDir, sess.ID)))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := ledger.AppendEventIfAbsent(ctx, store.SessionEvent{
			ID: "midday-event", SessionID: sess.ID, TurnID: event.TurnID,
			Type: event.Type, AgentName: sess.AgentName, Content: content, Timestamp: h.now,
		}); err != nil {
			t.Fatal(err)
		}
		if err := ledger.Close(ctx); err != nil {
			t.Fatal(err)
		}
		// A crash can occur after Done is durable but before the global session
		// catalog's updated_at is refreshed. That metadata must not filter out a
		// current-day receipt on activation or restart.
		if _, err := h.registry.DB().ExecContext(ctx, `UPDATE sessions SET updated_at=? WHERE id=?`,
			store.FormatTimestamp(clock.Add(-24*time.Hour)), sess.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.observer.ReconcileDailyBudgetInstance(ctx, instance.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.registry.Instances().SetDailyBudgetLimits(ctx, instance.ID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}); err != nil {
			t.Fatal(err)
		}
		read, err := h.registry.Instances().ReadDailyBudgetUsage(ctx, instance.ID, clock)
		if err != nil || read.Tokens != 85 {
			t.Fatalf("midday budget activation missed prior usage: %+v err=%v", read, err)
		}
	})
}

func TestObserverFailedLiveReceiptReconcilesBeforeNextAutonomousTurn(t *testing.T) {
	t.Run("Should recover a failed live receipt before another turn", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		clock := time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)
		h := newHarness(t, globaldb.WithClock(func() time.Time { return clock }))
		instance, _, err := h.registry.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
			AgentName: "coder", CreatedByKind: "human", PermissionCeiling: "{}",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		sess := newSession("budget-live-write-failure", session.StateActive, h.workspace, h.now)
		sess.InstanceID = instance.ID
		h.observeSessionCreated(t, sess)
		if _, err := h.registry.Instances().BindMain(ctx, instance.ID, sess.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.registry.Instances().SetDailyBudgetLimits(ctx, instance.ID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}); err != nil {
			t.Fatal(err)
		}
		usage := &acp.TokenUsage{TurnID: "failed-observer-write", InputTokens: new(int64(85)), Timestamp: h.now}
		event := acp.AgentEvent{Type: acp.EventTypeDone, TurnID: usage.TurnID, Usage: usage, Timestamp: h.now}
		content, err := transcript.MarshalAgentEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		ledger, err := sessiondb.OpenSessionDB(ctx,
			store.SessionDBOwner{SessionID: sess.ID, WorkspaceID: h.workspaceID},
			store.SessionDBFile(filepath.Join(h.home.SessionsDir, sess.ID)))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := ledger.AppendEventIfAbsent(ctx, store.SessionEvent{
			ID: "failed-observer-event", SessionID: sess.ID, TurnID: event.TurnID,
			Type: event.Type, AgentName: sess.AgentName, Content: content, Timestamp: h.now,
		}); err != nil {
			t.Fatal(err)
		}
		if err := ledger.Close(ctx); err != nil {
			t.Fatal(err)
		}
		registry := &failFirstCanonicalReceiptRegistry{GlobalDB: h.registry}
		registry.fail.Store(true)
		h.observer.registry = registry
		h.observer.OnAgentEvent(ctx, sess.ID, event)
		read, err := h.registry.Instances().ReadDailyBudgetUsage(ctx, instance.ID, clock)
		if err != nil || read.Tokens != 0 {
			t.Fatalf("fault-injected observer already committed usage=%+v err=%v", read, err)
		}
		if err := h.observer.EnsureDailyBudgetReady(ctx, sess.ID); err != nil {
			t.Fatalf("new-turn budget preflight did not recover: %v", err)
		}
		read, err = h.registry.Instances().ReadDailyBudgetUsage(ctx, instance.ID, clock)
		if err != nil || read.Tokens != 85 {
			t.Fatalf("new turn would see stale usage=%+v err=%v", read, err)
		}
		if err := h.observer.EnsureDailyBudgetReady(ctx, sess.ID); err != nil {
			t.Fatalf("idempotent budget preflight: %v", err)
		}
	})
}

func TestObserverDailyBudgetDirtyRecoveryLeavesStandaloneWorkAvailable(t *testing.T) {
	t.Run("Should avoid budget recovery for standalone admission", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		sess := newSession("unbudgeted-recovery-isolation", session.StateActive, h.workspace, h.now)
		h.observeSessionCreated(t, sess)
		registry := &dirtyDuringBudgetReconcileRegistry{
			GlobalDB: h.registry, observer: h.observer, always: true,
		}
		h.observer.registry = registry
		h.observer.markDailyBudgetReceiptPending()
		committed := false
		err := h.observer.WithDailyBudgetAdmission(t.Context(), sess.ID, func(context.Context) error {
			committed = true
			return nil
		})
		if err != nil || !committed || registry.calls.Load() != 0 {
			t.Fatalf("standalone commit=%t err=%v budget scans=%d", committed, err, registry.calls.Load())
		}
	})
}
