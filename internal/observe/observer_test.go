package observe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	bridgepkg "github.com/compozy/compozy/internal/bridges"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/version"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

func TestOnSessionCreatedTracksSessionSnapshot(t *testing.T) {
	t.Run("Should cache session snapshot on creation", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := newSession("sess-created", session.StateActive, h.workspace, h.now)

		h.observeSessionCreated(t, sess)

		snapshot, ok := h.observer.sessionSnapshot(sess.ID)
		if !ok {
			t.Fatal("sessionSnapshot() cached = false, want true")
		}
		if snapshot.agentName != "coder" || snapshot.workspaceID != h.workspaceID {
			t.Fatalf("sessionSnapshot() = %#v, want coder workspace snapshot", snapshot)
		}
	})
}

func TestOnAgentEventForSessionCachesResolvedAgentByRuntimeIdentity(t *testing.T) {
	t.Run("Should resolve auth once until the runtime selection changes", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		resolver := &countingObserveAgentResolver{agent: compozyconfig.AgentDef{
			Name:     "coder",
			Provider: "claude",
		}}
		h.observer.resolveProviderAuth = defaultProviderAuthModeResolver(
			h.home,
			h.observer.workspaceResolver,
			resolver,
		)
		sess := newSession("sess-runtime-cache", session.StateActive, h.workspace, h.now)
		sess.Model = "claude-test"
		sess.EffectivePermissions = "deny-all"

		h.observeSessionCreated(t, sess)
		for index := range 2 {
			h.observer.OnAgentEventForSession(testutil.Context(t), sess, acp.AgentEvent{
				Type:      "agent_message",
				TurnID:    fmt.Sprintf("turn-cache-%d", index),
				Timestamp: h.now.Add(time.Duration(index+1) * time.Minute),
				Text:      "cached event",
			})
		}
		if got := resolver.calls.Load(); got != 1 {
			t.Fatalf("ResolveAgent() calls = %d, want 1 for one runtime identity", got)
		}
		snapshot, ok := h.observer.sessionSnapshot(sess.ID)
		if !ok || snapshot.permissionMode != "deny-all" {
			t.Fatalf("sessionSnapshot() = %#v, %v, want live deny-all permissions", snapshot, ok)
		}

		sess.RuntimeSelectionRevision++
		sess.Model = "claude-next"
		h.observer.OnAgentEventForSession(testutil.Context(t), sess, acp.AgentEvent{
			Type:      "agent_message",
			TurnID:    "turn-cache-runtime-change",
			Timestamp: h.now.Add(3 * time.Minute),
			Text:      "runtime changed",
		})
		if got := resolver.calls.Load(); got != 2 {
			t.Fatalf("ResolveAgent() calls = %d, want 2 after runtime identity change", got)
		}
	})
}

func TestOnSessionStoppedClearsSessionSnapshot(t *testing.T) {
	t.Run("Should clear session snapshot on stop", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := newSession("sess-stopped", session.StateActive, h.workspace, h.now)

		h.observeSessionCreated(t, sess)
		sess.State = session.StateStopped
		sess.UpdatedAt = h.now.Add(2 * time.Minute)
		h.observer.OnSessionStopped(testutil.Context(t), sess)

		if _, ok := h.observer.sessionSnapshot(sess.ID); ok {
			t.Fatal("sessionSnapshot() cached = true, want false after stop")
		}
	})
}

func TestOnAgentEventWritesEventSummaryToGlobalDB(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sess := newSession("sess-summary", session.StateActive, h.workspace, h.now)
	sess.Lineage = &store.SessionLineage{
		ParentSessionID: "sess-parent",
		RootSessionID:   "sess-root",
		SpawnDepth:      1,
	}
	h.observeSessionCreated(t, sess)

	h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
		Type:      "agent_message",
		TurnID:    "turn-1",
		Timestamp: h.now.Add(time.Minute),
		Text:      "assistant replied with the requested diff",
	})

	events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{SessionID: sess.ID})
	if err != nil {
		t.Fatalf("QueryEvents() error = %v", err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}
	if events[0].Summary != "assistant replied with the requested diff" {
		t.Fatalf("events[0].Summary = %q", events[0].Summary)
	}
	if events[0].ParentSessionID != "sess-parent" ||
		events[0].RootSessionID != "sess-root" ||
		events[0].SpawnDepth != 1 {
		t.Fatalf("events[0] lineage = %#v", events[0])
	}
}

func TestObserverQueryEventsAggregatesMemoryEventSource(t *testing.T) {
	t.Run("Should merge memory events after durable registry events", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := newSession("sess-memory-observe", session.StateActive, h.workspace, h.now)
		h.observeSessionCreated(t, sess)
		h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
			Type:      "agent_message",
			TurnID:    "turn-1",
			Timestamp: h.now.Add(time.Minute),
			Text:      "assistant replied",
		})

		source := &stubMemoryEventSource{
			events: []store.EventSummary{{
				ID:        "memevt-workspace-01",
				Type:      "memory.recall.executed",
				AgentName: "coder",
				Summary:   "workspace recall executed",
				Timestamp: h.now.Add(2 * time.Minute),
			}},
		}
		h.observer.mu.Lock()
		h.observer.memoryEventSource = source
		h.observer.mu.Unlock()

		events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{})
		if err != nil {
			t.Fatalf("QueryEvents() error = %v", err)
		}
		if got, want := len(events), 2; got != want {
			t.Fatalf("len(events) = %d, want %d; events=%#v", got, want, events)
		}
		if got, want := events[0].Type, "agent_message"; got != want {
			t.Fatalf("events[0].Type = %q, want %q", got, want)
		}
		if got, want := events[1].Type, "memory.recall.executed"; got != want {
			t.Fatalf("events[1].Type = %q, want %q", got, want)
		}
		if len(source.workspaces) != 1 || source.workspaces[0] != h.workspace {
			t.Fatalf("memory source workspaces = %#v, want [%q]", source.workspaces, h.workspace)
		}
	})
}

func TestObserverQueryEventsNormalizesMemoryWorkspaceFilter(t *testing.T) {
	t.Run("Should translate public workspace id to memory workspace identity", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		memoryIdentity, err := compozyworkspace.EnsureIdentity(testutil.Context(t), h.workspace)
		if err != nil {
			t.Fatalf("EnsureIdentity(%q) error = %v", h.workspace, err)
		}
		source := &stubMemoryEventSource{
			events: []store.EventSummary{{
				ID:          "memevt-workspace-filter-01",
				Type:        "memory.write.reindex",
				WorkspaceID: memoryIdentity.WorkspaceID,
				Summary:     "scope=workspace indexed=3",
				Timestamp:   h.now.Add(time.Minute),
			}},
		}
		h.observer.mu.Lock()
		h.observer.memoryEventSource = source
		h.observer.mu.Unlock()

		events, err := h.observer.QueryEvents(
			testutil.Context(t),
			store.EventSummaryQuery{WorkspaceID: h.workspaceID, Type: "memory.write.reindex"},
		)
		if err != nil {
			t.Fatalf("QueryEvents(workspace) error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("len(events) = %d, want %d; events=%#v", got, want, events)
		}
		if got, want := source.query.WorkspaceID, memoryIdentity.WorkspaceID; got != want {
			t.Fatalf("memory source query workspace_id = %q, want %q", got, want)
		}
		if len(source.workspaces) != 1 || source.workspaces[0] != h.workspace {
			t.Fatalf("memory source workspaces = %#v, want [%q]", source.workspaces, h.workspace)
		}
	})
}

func TestObserverQueryEventsKeepsSessionScopedEventsNarrow(t *testing.T) {
	t.Run("Should not fan memory source into session-scoped queries yet", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := newSession("sess-memory-filter", session.StateActive, h.workspace, h.now)
		h.observeSessionCreated(t, sess)
		source := &stubMemoryEventSource{
			events: []store.EventSummary{{
				ID:        "memevt-workspace-02",
				Type:      "memory.write.committed",
				SessionID: sess.ID,
				AgentName: "coder",
				Summary:   "workspace write committed",
				Timestamp: h.now.Add(time.Minute),
			}},
		}
		h.observer.mu.Lock()
		h.observer.memoryEventSource = source
		h.observer.mu.Unlock()

		events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{SessionID: sess.ID})
		if err != nil {
			t.Fatalf("QueryEvents(session) error = %v", err)
		}
		if len(events) != 0 {
			t.Fatalf("session scoped events = %#v, want no memory source fan-in yet", events)
		}
		if len(source.workspaces) != 0 {
			t.Fatalf("memory source workspaces = %#v, want source not queried for session scope", source.workspaces)
		}
	})
}

func TestObserverQueryEventsKeepsWorktreeScopedEventsNarrow(t *testing.T) {
	t.Run("Should not merge unsequenced memory events into a worktree replay", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		source := &stubMemoryEventSource{events: []store.EventSummary{{
			ID: "memevt-worktree-01", Type: "memory.write.committed", Sequence: 0,
			Summary: "workspace write committed", Timestamp: h.now.Add(time.Minute),
		}}}
		h.observer.mu.Lock()
		h.observer.memoryEventSource = source
		h.observer.mu.Unlock()

		events, err := h.observer.QueryEvents(
			testutil.Context(t),
			store.EventSummaryQuery{WorkspaceID: h.workspaceID, WorktreeID: "wt-a", Limit: 500},
		)
		if err != nil {
			t.Fatalf("QueryEvents(worktree) error = %v", err)
		}
		if len(events) != 0 {
			t.Fatalf("worktree events = %#v, want no memory source fan-in", events)
		}
		if len(source.workspaces) != 0 {
			t.Fatalf("memory source workspaces = %#v, want source not queried for worktree scope", source.workspaces)
		}
	})
}

func TestOnAgentEventRecoversSessionSnapshot(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		sessionID  string
		state      session.State
		summary    string
		setup      func(t *testing.T, h *harness, sess *session.Session)
		wantCached bool
	}{
		{
			name:      "Should recover session snapshot from live source",
			sessionID: "sess-live-source",
			state:     session.StateActive,
			summary:   "live source event was observed",
			setup: func(t *testing.T, h *harness, sess *session.Session) {
				t.Helper()

				if err := h.registry.RegisterSession(
					testutil.Context(t),
					sessionInfoFromSession(sess.Info()),
				); err != nil {
					t.Fatalf("RegisterSession(live source) error = %v", err)
				}
				h.observer.registry = listSessionsFailingRegistry{Registry: h.registry}
				h.source.sessions = []*session.Info{sess.Info()}
			},
			wantCached: true,
		},
		{
			name:      "Should recover session snapshot from registry",
			sessionID: "sess-registry-source",
			state:     session.StateActive,
			summary:   "registry event was observed",
			setup: func(t *testing.T, h *harness, sess *session.Session) {
				t.Helper()

				if err := h.registry.RegisterSession(
					testutil.Context(t),
					sessionInfoFromSession(sess.Info()),
				); err != nil {
					t.Fatalf("RegisterSession() error = %v", err)
				}
			},
			wantCached: true,
		},
		{
			name:      "Should not cache stopped sessions recovered from registry",
			sessionID: "sess-stopped-registry-source",
			state:     session.StateStopped,
			summary:   "stopped registry event was observed",
			setup: func(t *testing.T, h *harness, sess *session.Session) {
				t.Helper()

				if err := h.registry.RegisterSession(
					testutil.Context(t),
					sessionInfoFromSession(sess.Info()),
				); err != nil {
					t.Fatalf("RegisterSession(stopped) error = %v", err)
				}
			},
			wantCached: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			sess := newSession(tc.sessionID, tc.state, h.workspace, h.now)
			tc.setup(t, h, sess)

			h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
				Type:      "agent_message",
				TurnID:    "turn-" + tc.sessionID,
				Timestamp: h.now.Add(time.Minute),
				Text:      tc.summary,
			})

			events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{SessionID: sess.ID})
			if err != nil {
				t.Fatalf("QueryEvents() error = %v", err)
			}
			if got, want := len(events), 1; got != want {
				t.Fatalf("len(events) = %d, want %d", got, want)
			}
			if events[0].AgentName != "coder" || events[0].Summary != tc.summary {
				t.Fatalf("events[0] = %#v, want recovered event summary %q", events[0], tc.summary)
			}

			_, cached := h.observer.sessionSnapshot(sess.ID)
			if cached != tc.wantCached {
				t.Fatalf("sessionSnapshot(%q) cached = %v, want %v", sess.ID, cached, tc.wantCached)
			}
		})
	}
}

func TestObserverSessionSnapshotRequiresContext(t *testing.T) {
	t.Parallel()

	nilContext := func() context.Context {
		return nil
	}
	testCases := []struct {
		name string
		call func(observer *Observer)
	}{
		{
			name: "Should panic when recovering a session snapshot with nil context",
			call: func(observer *Observer) {
				observer.recoverSessionSnapshot(nilContext(), "sess-nil-context")
			},
		},
		{
			name: "Should panic when building an observed session snapshot with nil context",
			call: func(observer *Observer) {
				observer.observedSessionSnapshot(
					nilContext(),
					"sess-nil-context",
					"coder",
					"",
					"",
					observerWorkspaceID,
					"approve-all",
					0,
					nil,
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			defer func() {
				if recovered := recover(); recovered == nil {
					t.Fatal("snapshot helper panic = nil, want non-nil")
				}
			}()
			tc.call(h.observer)
		})
	}
}

func TestSweepRetentionModes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		retention RetentionConfig
		sessionID string
		record    func(t *testing.T, h *harness, sess *session.Session)
		assert    func(t *testing.T, h *harness, sess *session.Session, health RetentionHealth)
	}{
		{
			name: "Should use configured cutoff",
			retention: RetentionConfig{
				Enabled:       true,
				RetentionDays: 7,
				SweepInterval: time.Hour,
			},
			sessionID: "sess-retention",
			record: func(t *testing.T, h *harness, sess *session.Session) {
				t.Helper()

				healthNow := h.now.Add(time.Hour)
				cutoff := healthNow.AddDate(0, 0, -7)
				h.recordEvent(t, sess.ID, "agent_message", cutoff.Add(-time.Nanosecond), "old event")
				h.recordEvent(t, sess.ID, "agent_message", cutoff, "cutoff event")
				h.recordEvent(t, sess.ID, "agent_message", cutoff.Add(time.Nanosecond), "fresh event")
			},
			assert: func(t *testing.T, h *harness, sess *session.Session, health RetentionHealth) {
				t.Helper()

				cutoff := h.now.Add(time.Hour).AddDate(0, 0, -7)
				if !health.Enabled || health.RetentionDays != 7 || health.LastSweepStatus != retentionSweepStatusOK {
					t.Fatalf("SweepRetention() health = %#v, want enabled ok seven-day retention", health)
				}
				if health.LastCutoffAt == nil || !health.LastCutoffAt.Equal(cutoff) {
					t.Fatalf("SweepRetention().LastCutoffAt = %#v, want %s", health.LastCutoffAt, cutoff)
				}
				if health.DeletedEventSummaries != 1 {
					t.Fatalf("SweepRetention().DeletedEventSummaries = %d, want 1", health.DeletedEventSummaries)
				}
				events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{SessionID: sess.ID})
				if err != nil {
					t.Fatalf("QueryEvents() error = %v", err)
				}
				if got, want := len(events), 2; got != want {
					t.Fatalf("len(events) = %d, want %d: %#v", got, want, events)
				}
				if events[0].Summary != "cutoff event" || events[1].Summary != "fresh event" {
					t.Fatalf("events after retention = %#v, want cutoff and fresh events", events)
				}
			},
		},
		{
			name: "Should keep history when disabled",
			retention: RetentionConfig{
				Enabled:       false,
				RetentionDays: 0,
				SweepInterval: time.Hour,
			},
			sessionID: "sess-keep-history",
			record: func(t *testing.T, h *harness, sess *session.Session) {
				t.Helper()
				h.recordEvent(t, sess.ID, "agent_message", h.now.AddDate(-1, 0, 0), "old retained event")
			},
			assert: func(t *testing.T, h *harness, sess *session.Session, health RetentionHealth) {
				t.Helper()

				if health.Enabled || health.LastSweepStatus != retentionSweepStatusDisabled {
					t.Fatalf("SweepRetention(disabled) health = %#v, want disabled status", health)
				}
				events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{SessionID: sess.ID})
				if err != nil {
					t.Fatalf("QueryEvents() error = %v", err)
				}
				if got, want := len(events), 1; got != want {
					t.Fatalf("len(events) = %d, want %d: %#v", got, want, events)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			h.observer.retention = tc.retention
			h.observer.setRetentionHealth(h.observer.initialRetentionHealth())
			sess := newSession(tc.sessionID, session.StateActive, h.workspace, h.now)
			h.observeSessionCreated(t, sess)
			tc.record(t, h, sess)

			health, err := h.observer.SweepRetention(testutil.Context(t))
			if err != nil {
				t.Fatalf("SweepRetention() error = %v", err)
			}
			tc.assert(t, h, sess, health)
		})
	}
}

func TestRetentionLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve one owned run across a canceled shutdown waiter", func(t *testing.T) {
		t.Parallel()

		source := &blockingRetentionStore{
			started: make(chan struct{}, 1),
			release: make(chan struct{}),
		}
		t.Cleanup(func() {
			select {
			case <-source.release:
			default:
				close(source.release)
			}
		})
		observer := &Observer{
			registry: source,
			retention: RetentionConfig{
				Enabled:       true,
				RetentionDays: 7,
				SweepInterval: time.Hour,
			},
			logger: slog.New(slog.DiscardHandler),
			now:    time.Now,
		}
		var nilCtx context.Context
		if err := observer.ShutdownRetention(nilCtx); err == nil {
			t.Fatal("ShutdownRetention(nil) error = nil, want error")
		}
		if err := observer.StartRetention(t.Context()); err != nil {
			t.Fatalf("StartRetention() error = %v", err)
		}
		select {
		case <-source.started:
		case <-t.Context().Done():
			t.Fatalf("wait for retention sweep: %v", t.Context().Err())
		}

		observer.retentionMu.Lock()
		startedRun := observer.retentionRun
		observer.retentionMu.Unlock()
		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := observer.ShutdownRetention(canceledCtx); !errors.Is(err, context.Canceled) {
			t.Fatalf("ShutdownRetention(canceled) error = %v, want context.Canceled", err)
		}
		if err := observer.StartRetention(t.Context()); err != nil {
			t.Fatalf("StartRetention(while stopping) error = %v", err)
		}
		observer.retentionMu.Lock()
		retainedRun := observer.retentionRun
		observer.retentionMu.Unlock()
		if retainedRun != startedRun {
			t.Fatal("StartRetention() replaced an unfinished owned run")
		}
		if got := source.calls.Load(); got != 1 {
			t.Fatalf("retention sweep calls = %d, want 1", got)
		}

		close(source.release)
		if err := observer.ShutdownRetention(t.Context()); err != nil {
			t.Fatalf("ShutdownRetention(join) error = %v", err)
		}
		observer.retentionMu.Lock()
		finishedRun := observer.retentionRun
		observer.retentionMu.Unlock()
		if finishedRun != nil {
			t.Fatal("retention run remained published after shutdown completion")
		}
	})
}

type blockingRetentionStore struct {
	Registry
	started chan struct{}
	release chan struct{}
	calls   atomic.Int64
}

func (s *blockingRetentionStore) SweepObservability(
	context.Context,
	time.Time,
) (store.ObservabilityRetentionSweepResult, error) {
	s.calls.Add(1)
	select {
	case s.started <- struct{}{}:
	default:
	}
	<-s.release
	return store.ObservabilityRetentionSweepResult{}, nil
}

func TestOnAgentEventUpdatesTokenStatsWithNullableValues(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sess := newSession("sess-usage", session.StateActive, h.workspace, h.now)
	h.observeSessionCreated(t, sess)

	outputTokens := int64(4)
	totalTokens := int64(4)
	h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
		Type:      "done",
		TurnID:    "turn-usage",
		Timestamp: h.now.Add(time.Minute),
		Usage: &acp.TokenUsage{
			TurnID:       "turn-usage",
			OutputTokens: &outputTokens,
			TotalTokens:  &totalTokens,
			Timestamp:    h.now.Add(time.Minute),
		},
	})

	stats, err := h.observer.QueryTokenStats(testutil.Context(t), store.TokenStatsQuery{SessionID: sess.ID})
	if err != nil {
		t.Fatalf("QueryTokenStats() error = %v", err)
	}
	if got, want := len(stats), 1; got != want {
		t.Fatalf("len(stats) = %d, want %d", got, want)
	}
	if stats[0].InputTokens != nil {
		t.Fatalf("InputTokens = %#v, want nil", stats[0].InputTokens)
	}
	if stats[0].OutputTokens == nil || *stats[0].OutputTokens != 4 {
		t.Fatalf("OutputTokens = %#v, want 4", stats[0].OutputTokens)
	}
	if stats[0].TurnCount != 1 {
		t.Fatalf("TurnCount = %d, want 1", stats[0].TurnCount)
	}
}

// Suite: observer token-cost projection
// Invariant: agent cost wins, native CLI is included, and silent bound-secret usage estimates fail closed.
// Boundary IN: one normalized ACP usage event plus the session auth/catalog snapshot.
// Boundary OUT: store aggregation and public task/session rollups.
func TestOnAgentEventProjectsTruthfulCostProvenance(t *testing.T) {
	t.Parallel()

	t.Run("Should persist agent reported cost without consulting the catalog", func(t *testing.T) {
		t.Parallel()

		catalog := &stubCostCatalog{}
		h := newHarness(t)
		h.observer.costCatalog = catalog
		h.observer.resolveProviderAuth = fixedProviderAuthMode(compozyconfig.ProviderAuthModeNativeCLI)
		sess := newSession("sess-cost-actual", session.StateActive, h.workspace, h.now)
		sess.Model = "claude-test"
		h.observeSessionCreated(t, sess)

		amount := 0.25
		currency := "USD"
		h.recordUsage(t, sess.ID, &acp.TokenUsage{CostAmount: &amount, CostCurrency: &currency})

		stat := h.singleTokenStat(t, sess.ID)
		if stat.TotalCost == nil || *stat.TotalCost != amount ||
			stat.CostCurrency == nil || *stat.CostCurrency != currency ||
			stat.CostStatus != "actual" || stat.CostSource != "agent_reported" {
			t.Fatalf("actual cost stat = %#v, want agent_reported USD %.2f", stat, amount)
		}
		if catalog.calls != 0 {
			t.Fatalf("catalog calls = %d, want zero for agent-reported cost", catalog.calls)
		}
	})

	t.Run("Should preserve usage as unknown when agent reported cost has no currency", func(t *testing.T) {
		t.Parallel()

		catalog := &stubCostCatalog{}
		h := newHarness(t)
		h.observer.costCatalog = catalog
		h.observer.resolveProviderAuth = fixedProviderAuthMode(compozyconfig.ProviderAuthModeBoundSecret)
		sess := newSession("sess-cost-actual-missing-currency", session.StateActive, h.workspace, h.now)
		sess.Model = "claude-test"
		h.observeSessionCreated(t, sess)
		amount := 0.25
		total := int64(42)

		h.recordUsage(t, sess.ID, &acp.TokenUsage{TotalTokens: &total, CostAmount: &amount})

		stat := h.singleTokenStat(t, sess.ID)
		if stat.TotalTokens == nil || *stat.TotalTokens != total || stat.TotalCost != nil ||
			stat.CostCurrency != nil || stat.CostStatus != "unknown" || stat.CostSource != "none" {
			t.Fatalf("malformed actual cost stat = %#v, want tokens with unknown/none cost", stat)
		}
		if catalog.calls != 0 {
			t.Fatalf("catalog calls = %d, want zero when an actual amount was reported", catalog.calls)
		}
	})

	t.Run("Should estimate silent bound secret usage from the merged catalog", func(t *testing.T) {
		t.Parallel()

		inputRate := 1.0
		outputRate := 4.0
		catalog := &stubCostCatalog{models: []modelcatalog.Model{{
			ProviderID:           "claude",
			ModelID:              "claude-test",
			CostInputPerMillion:  &inputRate,
			CostOutputPerMillion: &outputRate,
			CostInputSource:      modelcatalog.SourceKindConfig,
			CostOutputSource:     modelcatalog.SourceKindConfig,
		}}}
		h := newHarness(t)
		h.observer.costCatalog = catalog
		h.observer.resolveProviderAuth = fixedProviderAuthMode(compozyconfig.ProviderAuthModeBoundSecret)
		sess := newSession("sess-cost-estimated", session.StateActive, h.workspace, h.now)
		sess.Model = "claude-test"
		h.observeSessionCreated(t, sess)
		input := int64(1_000_000)
		output := int64(1_000_000)

		h.recordUsage(t, sess.ID, &acp.TokenUsage{InputTokens: &input, OutputTokens: &output})

		stat := h.singleTokenStat(t, sess.ID)
		if stat.TotalCost == nil || *stat.TotalCost != 5 ||
			stat.CostCurrency == nil || *stat.CostCurrency != "USD" ||
			stat.CostStatus != "estimated" || stat.CostSource != "catalog_config" {
			t.Fatalf("estimated cost stat = %#v, want catalog_config USD 5", stat)
		}
		if catalog.calls != 1 || catalog.options.ProviderID != "claude" ||
			!catalog.options.SkipRefreshIfEmpty || !catalog.options.IncludeAll || !catalog.options.IncludeStale {
			t.Fatalf(
				"catalog lookup = calls %d options %#v, want one non-refreshing complete lookup",
				catalog.calls,
				catalog.options,
			)
		}
	})

	t.Run("Should refresh the effective runtime before estimating a logical session event", func(t *testing.T) {
		t.Parallel()

		inputRate := 2.0
		outputRate := 6.0
		catalog := &stubCostCatalog{models: []modelcatalog.Model{{
			ProviderID:           "runtime-provider",
			ModelID:              "runtime-model",
			CostInputPerMillion:  &inputRate,
			CostOutputPerMillion: &outputRate,
			CostInputSource:      modelcatalog.SourceKindConfig,
			CostOutputSource:     modelcatalog.SourceKindConfig,
		}}}
		h := newHarness(t)
		h.observer.costCatalog = catalog
		h.observer.resolveProviderAuth = fixedProviderAuthMode(compozyconfig.ProviderAuthModeBoundSecret)
		sess := newSession("sess-cost-runtime-bind", session.StateActive, h.workspace, h.now)
		sess.Provider = "creation-provider"
		sess.Model = "creation-model"
		h.observeSessionCreated(t, sess)

		sess.Provider = "runtime-provider"
		sess.Model = "runtime-model"
		input := int64(1_000_000)
		output := int64(1_000_000)
		h.observer.OnAgentEventForSession(testutil.Context(t), sess, acp.AgentEvent{
			Type:   acp.EventTypeDone,
			TurnID: "turn-runtime-bind",
			Usage:  &acp.TokenUsage{InputTokens: &input, OutputTokens: &output},
		})

		stat := h.singleTokenStat(t, sess.ID)
		if stat.TotalCost == nil || *stat.TotalCost != 8 ||
			stat.CostStatus != "estimated" || stat.CostSource != "catalog_config" {
			t.Fatalf("bound runtime cost stat = %#v, want catalog_config USD 8", stat)
		}
		if catalog.options.ProviderID != "runtime-provider" {
			t.Fatalf(
				"catalog provider = %q, want effective runtime provider",
				catalog.options.ProviderID,
			)
		}
	})

	t.Run("Should persist unknown when the merged model has no usable rates", func(t *testing.T) {
		t.Parallel()

		catalog := &stubCostCatalog{models: []modelcatalog.Model{{
			ProviderID: "claude",
			ModelID:    "claude-test",
		}}}
		h := newHarness(t)
		h.observer.costCatalog = catalog
		h.observer.resolveProviderAuth = fixedProviderAuthMode(compozyconfig.ProviderAuthModeBoundSecret)
		sess := newSession("sess-cost-unknown", session.StateActive, h.workspace, h.now)
		sess.Model = "claude-test"
		h.observeSessionCreated(t, sess)
		output := int64(50)

		h.recordUsage(t, sess.ID, &acp.TokenUsage{OutputTokens: &output})

		stat := h.singleTokenStat(t, sess.ID)
		if stat.TotalCost != nil || stat.CostCurrency != nil ||
			stat.CostStatus != "unknown" || stat.CostSource != "none" {
			t.Fatalf("missing-rate cost stat = %#v, want unknown/none", stat)
		}
	})

	t.Run("Should mark silent native CLI usage as included without consulting rates", func(t *testing.T) {
		t.Parallel()

		catalog := &stubCostCatalog{}
		h := newHarness(t)
		h.observer.costCatalog = catalog
		h.observer.resolveProviderAuth = fixedProviderAuthMode(compozyconfig.ProviderAuthModeNativeCLI)
		sess := newSession("sess-cost-included", session.StateActive, h.workspace, h.now)
		sess.Model = "claude-test"
		h.observeSessionCreated(t, sess)
		output := int64(50)

		h.recordUsage(t, sess.ID, &acp.TokenUsage{OutputTokens: &output})

		stat := h.singleTokenStat(t, sess.ID)
		if stat.TotalCost != nil || stat.CostCurrency != nil ||
			stat.CostStatus != "included" || stat.CostSource != "none" {
			t.Fatalf("native CLI cost stat = %#v, want included/none", stat)
		}
		if catalog.calls != 0 {
			t.Fatalf("catalog calls = %d, want zero for native CLI usage", catalog.calls)
		}
	})
}

func TestOnAgentEventWritesPermissionLog(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sess := newSession("sess-permission", session.StateActive, h.workspace, h.now)
	h.observeSessionCreated(t, sess)

	h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
		Type:      "permission",
		TurnID:    "turn-perm",
		Timestamp: h.now.Add(time.Minute),
		Action:    "session/request_permission",
		Resource:  filepath.Join(h.workspace, "secret.txt"),
		Decision:  "allow",
	})

	entries, err := h.observer.QueryPermissionLog(testutil.Context(t), store.PermissionLogQuery{SessionID: sess.ID})
	if err != nil {
		t.Fatalf("QueryPermissionLog() error = %v", err)
	}
	if got, want := len(entries), 1; got != want {
		t.Fatalf("len(entries) = %d, want %d", got, want)
	}
	if entries[0].PolicyUsed != "approve-all" {
		t.Fatalf("entries[0].PolicyUsed = %q, want approve-all", entries[0].PolicyUsed)
	}
}

func TestOnAgentEventSkipsUnknownSession(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.observer.OnAgentEvent(testutil.Context(t), "missing", acp.AgentEvent{
		Type:      "agent_message",
		TurnID:    "turn-1",
		Timestamp: h.now,
		Text:      "ignored",
	})

	events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{})
	if err != nil {
		t.Fatalf("QueryEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(events))
	}
}

func TestNotifierLifecycleWritesThroughObserver(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sess := newSession("sess-nil-ctx", session.StateActive, h.workspace, h.now)

	h.observeSessionCreated(t, sess)
	h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
		Type:      "tool_result",
		TurnID:    "turn-nil-ctx",
		Timestamp: h.now.Add(time.Minute),
		Title:     "ls",
	})
	sess.State = session.StateStopped
	h.observer.OnSessionStopped(testutil.Context(t), sess)

	events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{SessionID: sess.ID})
	if err != nil {
		t.Fatalf("QueryEvents() error = %v", err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}
}

func TestOnAgentEventGuardBranches(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.observer.OnAgentEvent(testutil.Context(t), "", acp.AgentEvent{
		Type:      "agent_message",
		TurnID:    "turn-empty-session",
		Timestamp: h.now,
	})

	sess := newSession("sess-empty-type", session.StateActive, h.workspace, h.now)
	h.observeSessionCreated(t, sess)
	h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
		TurnID:    "turn-empty-type",
		Timestamp: h.now,
	})

	events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{})
	if err != nil {
		t.Fatalf("QueryEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(events))
	}
}

func TestOnAgentEventPermissionWithoutResolvedPolicySkipsAudit(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sess := newSession("sess-no-policy", session.StateActive, h.workspace, h.now)
	sess.EffectivePermissions = ""
	h.observeSessionCreated(t, sess)
	h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
		Type:      "permission",
		TurnID:    "turn-no-policy",
		Timestamp: h.now.Add(time.Minute),
		Action:    "session/request_permission",
		Resource:  filepath.Join(h.workspace, "secret.txt"),
		Decision:  "deny",
	})

	entries, err := h.observer.QueryPermissionLog(testutil.Context(t), store.PermissionLogQuery{SessionID: sess.ID})
	if err != nil {
		t.Fatalf("QueryPermissionLog() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) = %d, want 0", len(entries))
	}
}

func TestQueryEventsFilterBySessionID(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sessA := newSession("sess-a", session.StateActive, h.workspace, h.now)
	sessB := newSession("sess-b", session.StateActive, h.workspace, h.now.Add(time.Minute))
	h.observeSessionCreated(t, sessA)
	h.observeSessionCreated(t, sessB)

	h.recordEvent(t, sessA.ID, "agent_message", h.now.Add(time.Minute), "a-1")
	h.recordEvent(t, sessB.ID, "agent_message", h.now.Add(2*time.Minute), "b-1")

	events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{SessionID: sessB.ID})
	if err != nil {
		t.Fatalf("QueryEvents() error = %v", err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}
	if events[0].SessionID != sessB.ID {
		t.Fatalf("events[0].SessionID = %q, want %q", events[0].SessionID, sessB.ID)
	}
}

func TestQueryEventsReturnsHarnessLifecycleSummaries(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sess := newSession("sess-harness-observe", session.StateActive, h.workspace, h.now)
	h.observeSessionCreated(t, sess)

	base := h.now.Add(3 * time.Minute)
	summaries := []store.EventSummary{
		{
			SessionID: sess.ID,
			Type:      "harness.context_resolved",
			AgentName: sess.AgentName,
			Summary:   "surface=startup sections=memory|skills|network",
			Timestamp: base,
		},
		{
			SessionID: sess.ID,
			Type:      "harness.section_selected",
			AgentName: sess.AgentName,
			Summary:   "selected=memory|skills|network count=3",
			Timestamp: base.Add(time.Second),
		},
		{
			SessionID: sess.ID,
			Type:      "harness.augmenter_applied",
			AgentName: sess.AgentName,
			Summary:   "augmenter=durable_memory outcome=blank",
			Timestamp: base.Add(2 * time.Second),
		},
	}
	for _, summary := range summaries {
		if err := h.observer.registry.WriteEventSummary(testutil.Context(t), summary); err != nil {
			t.Fatalf("WriteEventSummary(%q) error = %v", summary.Type, err)
		}
	}

	events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{
		SessionID: sess.ID,
		Limit:     3,
	})
	if err != nil {
		t.Fatalf("QueryEvents() error = %v", err)
	}
	if got, want := len(events), 3; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}
	if got, want := events[0].Type, "harness.context_resolved"; got != want {
		t.Fatalf("events[0].Type = %q, want %q", got, want)
	}
	if got, want := events[2].Type, "harness.augmenter_applied"; got != want {
		t.Fatalf("events[2].Type = %q, want %q", got, want)
	}
	if got, want := events[2].Summary, summaries[2].Summary; got != want {
		t.Fatalf("events[2].Summary = %q, want %q", got, want)
	}
}

func TestQueryEventsFilterByEventType(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sess := newSession("sess-type", session.StateActive, h.workspace, h.now)
	h.observeSessionCreated(t, sess)

	h.recordEvent(t, sess.ID, "agent_message", h.now.Add(time.Minute), "msg")
	h.recordEvent(t, sess.ID, "tool_call", h.now.Add(2*time.Minute), "tool")

	events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{Type: "tool_call"})
	if err != nil {
		t.Fatalf("QueryEvents() error = %v", err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}
	if events[0].Type != "tool_call" {
		t.Fatalf("events[0].Type = %q, want tool_call", events[0].Type)
	}
}

func TestQueryEventsFilterByTimeRange(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sess := newSession("sess-since", session.StateActive, h.workspace, h.now)
	h.observeSessionCreated(t, sess)

	oldTs := h.now.Add(time.Minute)
	newTs := h.now.Add(3 * time.Minute)
	h.recordEvent(t, sess.ID, "agent_message", oldTs, "old")
	h.recordEvent(t, sess.ID, "agent_message", newTs, "new")

	events, err := h.observer.QueryEvents(
		testutil.Context(t),
		store.EventSummaryQuery{Since: h.now.Add(2 * time.Minute)},
	)
	if err != nil {
		t.Fatalf("QueryEvents() error = %v", err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}
	if events[0].Summary != "new" {
		t.Fatalf("events[0].Summary = %q, want new", events[0].Summary)
	}
}

func TestQueryEventsLimitReturnsMostRecentRowsInAscendingOrder(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sess := newSession("sess-limit", session.StateActive, h.workspace, h.now)
	h.observeSessionCreated(t, sess)

	h.recordEvent(t, sess.ID, "agent_message", h.now.Add(time.Minute), "one")
	h.recordEvent(t, sess.ID, "agent_message", h.now.Add(2*time.Minute), "two")
	h.recordEvent(t, sess.ID, "agent_message", h.now.Add(3*time.Minute), "three")

	events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{Limit: 2})
	if err != nil {
		t.Fatalf("QueryEvents() error = %v", err)
	}
	if got, want := len(events), 2; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}
	if events[0].Summary != "two" || events[1].Summary != "three" {
		t.Fatalf("events = %#v, want [two three]", events)
	}
}

func TestHealthReturnsCorrectActiveCounts(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	lastActivityAt := h.now.Add(30 * time.Minute)
	turnStartedAt := h.now.Add(20 * time.Minute)
	h.source.sessions = []*session.Info{
		{
			ID:        "sess-active-1",
			AgentName: "coder",
			State:     session.StateActive,
			Liveness: &store.SessionLivenessMeta{
				Activity: &store.SessionActivityMeta{
					TurnID:             "turn-active",
					TurnSource:         "user",
					TurnStartedAt:      &turnStartedAt,
					LastActivityAt:     &lastActivityAt,
					LastActivityKind:   "agent_waiting",
					LastActivityDetail: "waiting for provider",
					CurrentTool:        "delegate_task",
				},
			},
		},
		{ID: "sess-active-2", AgentName: "coder", State: session.StateStopping},
		{ID: "sess-stopped", State: session.StateStopped},
	}

	health, err := h.observer.Health(testutil.Context(t))
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if health.ActiveSessions != 2 || health.ActiveAgents != 1 {
		t.Fatalf("Health() = %#v, want 2 active sessions and 1 active agent", health)
	}
	if health.UptimeSeconds != 3600 {
		t.Fatalf("Health().UptimeSeconds = %d, want 3600", health.UptimeSeconds)
	}
	if health.Version != "1.2.3" {
		t.Fatalf("Health().Version = %q, want 1.2.3", health.Version)
	}
	if health.Persistence.Status != "ok" ||
		health.Persistence.GlobalDBSizeBytes != health.GlobalDBSizeBytes ||
		health.Persistence.SessionDBSizeBytes != health.SessionDBSizeBytes {
		t.Fatalf("Health().Persistence = %#v, want ok status with DB sizes", health.Persistence)
	}
	if health.Retention.LastSweepStatus != retentionSweepStatusDisabled {
		t.Fatalf("Health().Retention = %#v, want disabled default retention state", health.Retention)
	}
	if got, want := len(health.Activities), 1; got != want {
		t.Fatalf("len(Health().Activities) = %d, want %d: %#v", got, want, health.Activities)
	}
	activity := health.Activities[0]
	if activity.SessionID != "sess-active-1" || activity.Status != "active" || activity.CurrentTool != "delegate_task" {
		t.Fatalf("Health().Activities[0] = %#v, want active delegate_task activity", activity)
	}
	if got, want := activity.IdleSeconds, int64(30*60); got != want {
		t.Fatalf("Health().Activities[0].IdleSeconds = %d, want %d", got, want)
	}
}

func TestHealthIncludesLifecycleFailuresAndAgentProbes(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	ctx := testutil.Context(t)
	if err := h.registry.RegisterSession(ctx, store.SessionInfo{
		ID:            "sess-protocol",
		Name:          "Protocol",
		AgentName:     "reviewer",
		Provider:      "codex",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   h.workspaceID,
		State:         string(session.StateStopped),
		StopReason:    store.StopError,
		Failure: &store.SessionFailure{
			Kind:    store.FailureProtocol,
			Summary: "bad ACP frame",
		},
		CreatedAt: h.now,
		UpdatedAt: h.now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("RegisterSession(protocol) error = %v", err)
	}
	if err := h.registry.RegisterSession(ctx, store.SessionInfo{
		ID:            "sess-crash",
		Name:          "Crashed",
		AgentName:     "coder",
		Provider:      "claude",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   h.workspaceID,
		State:         string(session.StateStopped),
		StopReason:    store.StopAgentCrashed,
		Failure: &store.SessionFailure{
			Kind:            store.FailureProcess,
			Summary:         "provider crashed token=super-secret",
			CrashBundlePath: "/tmp/crash-token=super-secret.json",
		},
		CreatedAt: h.now,
		UpdatedAt: h.now.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("RegisterSession(crash) error = %v", err)
	}
	h.observer.agentProbeSource = func(context.Context) ([]acp.ProbeTarget, error) {
		return []acp.ProbeTarget{{
			AgentName: "coder",
			Provider:  "claude",
			Command:   `missing-agent --api-key=super-secret "unterminated`,
		}}, nil
	}

	health, err := h.observer.Health(ctx)
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if got, want := health.Status, "degraded"; got != want {
		t.Fatalf("Health().Status = %q, want %q", got, want)
	}
	if health.Failures.Status != "degraded" ||
		health.Failures.Total != 2 ||
		health.Failures.ByKind[store.FailureProcess] != 1 ||
		health.Failures.ByKind[store.FailureProtocol] != 1 {
		t.Fatalf("Health().Failures = %#v, want classified lifecycle failures", health.Failures)
	}
	if got, want := len(health.Failures.Recent), 2; got != want {
		t.Fatalf("len(Health().Failures.Recent) = %d, want %d", got, want)
	}
	recent := health.Failures.Recent[0]
	if recent.SessionID != "sess-crash" || recent.FailureKind != store.FailureProcess {
		t.Fatalf("Health().Failures.Recent[0] = %#v, want most recent process failure", recent)
	}
	if older := health.Failures.Recent[1]; older.SessionID != "sess-protocol" ||
		older.FailureKind != store.FailureProtocol {
		t.Fatalf("Health().Failures.Recent[1] = %#v, want older protocol failure", older)
	}
	if strings.Contains(recent.Summary, "super-secret") ||
		strings.Contains(recent.CrashBundlePath, "super-secret") {
		t.Fatalf("Health().Failures.Recent[0] = %#v, want redacted diagnostics", recent)
	}
	if !strings.Contains(recent.Summary, "[REDACTED]") ||
		!strings.Contains(recent.CrashBundlePath, "[REDACTED]") {
		t.Fatalf("Health().Failures.Recent[0] = %#v, want redacted markers", recent)
	}
	if got, want := len(health.AgentProbes), 1; got != want {
		t.Fatalf("len(Health().AgentProbes) = %d, want %d", got, want)
	}
	probe := health.AgentProbes[0]
	if probe.Status != acp.ProbeStatusInvalid {
		t.Fatalf("Health().AgentProbes[0] = %#v, want invalid probe", probe)
	}
	if strings.Contains(probe.Command, "super-secret") || strings.Contains(probe.Error, "super-secret") {
		t.Fatalf("Health().AgentProbes[0] = %#v, want redacted probe command and error", probe)
	}
}

func TestHealthStatusDegradesForLifecycleFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should keep top-level status OK for user-canceled sessions", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		ctx := testutil.Context(t)
		if err := h.registry.RegisterSession(ctx, store.SessionInfo{
			ID:            "sess-user-canceled",
			Name:          "User Canceled",
			AgentName:     "coder",
			Provider:      "codex",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   h.workspaceID,
			State:         string(session.StateStopped),
			StopReason:    store.StopUserCanceled,
			Failure: &store.SessionFailure{
				Kind:    store.FailureCanceled,
				Summary: "session canceled by user",
			},
			CreatedAt: h.now,
			UpdatedAt: h.now,
		}); err != nil {
			t.Fatalf("RegisterSession(canceled) error = %v", err)
		}

		health, err := h.observer.Health(ctx)
		if err != nil {
			t.Fatalf("Health() error = %v", err)
		}
		if got, want := health.Failures.Status, observeHealthStatusOK; got != want {
			t.Fatalf("Health().Failures.Status = %q, want %q", got, want)
		}
		if got, want := health.Failures.Total, 1; got != want {
			t.Fatalf("Health().Failures.Total = %d, want %d", got, want)
		}
		if got, want := health.Failures.ByKind[store.FailureCanceled], 1; got != want {
			t.Fatalf("Health().Failures.ByKind[cancellation] = %d, want %d", got, want)
		}
		if got, want := len(health.Failures.Recent), 1; got != want {
			t.Fatalf("len(Health().Failures.Recent) = %d, want %d", got, want)
		}
		if got, want := health.Status, observeHealthStatusOK; got != want {
			t.Fatalf("Health().Status = %q, want %q", got, want)
		}
	})

	t.Run("Should degrade top-level status when only failure health is degraded", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		ctx := testutil.Context(t)
		if err := h.registry.RegisterSession(ctx, store.SessionInfo{
			ID:            "sess-failure-only",
			Name:          "Failure Only",
			AgentName:     "coder",
			Provider:      "codex",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   h.workspaceID,
			State:         string(session.StateStopped),
			StopReason:    store.StopError,
			Failure: &store.SessionFailure{
				Kind:    store.FailureProtocol,
				Summary: "bad frame",
			},
			CreatedAt: h.now,
			UpdatedAt: h.now,
		}); err != nil {
			t.Fatalf("RegisterSession(failure) error = %v", err)
		}

		health, err := h.observer.Health(ctx)
		if err != nil {
			t.Fatalf("Health() error = %v", err)
		}
		if got, want := health.Failures.Status, observeHealthStatusDegraded; got != want {
			t.Fatalf("Health().Failures.Status = %q, want %q", got, want)
		}
		if got, want := health.Status, observeHealthStatusDegraded; got != want {
			t.Fatalf("Health().Status = %q, want %q", got, want)
		}
	})
}

type harness struct {
	observer    *Observer
	registry    *globaldb.GlobalDB
	bridges     *observeBridgeSource
	home        compozyconfig.HomePaths
	source      *stubSessionSource
	now         time.Time
	workspaceID string
	workspace   string
}

func (h *harness) observeSessionCreated(t *testing.T, sess *session.Session) {
	t.Helper()
	if err := h.registry.RegisterSession(testutil.Context(t), sessionInfoFromSession(sess.Info())); err != nil {
		t.Fatalf("RegisterSession(%q) error = %v", sess.ID, err)
	}
	h.observer.OnSessionCreated(testutil.Context(t), sess)
}

const observerWorkspaceID = "ws-observe-workspace"

type stubSessionSource struct {
	sessions []*session.Info
}

type listSessionsFailingRegistry struct {
	Registry
}

type stubMemoryEventSource struct {
	events     []store.EventSummary
	workspaces []string
	query      store.EventSummaryQuery
}

type stubCostCatalog struct {
	models  []modelcatalog.Model
	err     error
	calls   int
	options modelcatalog.ListOptions
}

func (s *stubCostCatalog) ListModels(
	_ context.Context,
	options modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	s.calls++
	s.options = options
	return append([]modelcatalog.Model(nil), s.models...), s.err
}

func fixedProviderAuthMode(mode compozyconfig.ProviderAuthMode) ProviderAuthModeResolver {
	return func(context.Context, string, string, string, string) (compozyconfig.ProviderAuthMode, error) {
		return mode, nil
	}
}

func (r listSessionsFailingRegistry) ListSessions(
	context.Context,
	store.SessionListQuery,
) ([]store.SessionInfo, error) {
	return nil, errors.New("registry fallback disabled")
}

type observeBridgeSource struct {
	*bridgepkg.Service
	broker *bridgepkg.Broker
}

func (s *stubSessionSource) List() []*session.Info {
	return s.sessions
}

func (s *stubMemoryEventSource) ListMemoryEventSummaries(
	_ context.Context,
	workspaces []string,
	query store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	s.workspaces = append([]string(nil), workspaces...)
	s.query = query
	return append([]store.EventSummary(nil), s.events...), nil
}

func (s *observeBridgeSource) DeliveryMetrics() map[string]bridgepkg.BridgeDeliveryMetrics {
	if s == nil || s.broker == nil {
		return nil
	}
	return s.broker.DeliveryMetrics()
}

func (s *observeBridgeSource) DeliveryMetricsFor(
	bridgeInstanceIDs []string,
) (map[string]bridgepkg.BridgeDeliveryMetrics, error) {
	if s == nil || s.broker == nil {
		return nil, nil
	}
	return s.broker.DeliveryMetricsFor(bridgeInstanceIDs)
}

func (s *observeBridgeSource) CountBridgeRoutes(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string]int, error) {
	counts := make(map[string]int, len(bridgeInstanceIDs))
	for _, bridgeInstanceID := range bridgeInstanceIDs {
		routes, err := s.ListRoutes(ctx, bridgeInstanceID)
		if err != nil {
			return nil, err
		}
		counts[bridgeInstanceID] = len(routes)
	}
	return counts, nil
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := observeTestContext(t)

	home, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(home); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	if err := observeTestGlobalSeed.Clone(home.DatabaseFile); err != nil {
		t.Fatalf("global store seed Clone() error = %v", err)
	}
	registry, err := globaldb.OpenGlobalDB(ctx, home.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := registry.Close(observeTestContext(t)); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	now := time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)
	source := &stubSessionSource{}
	bridges := &observeBridgeSource{
		Service: bridgepkg.NewRegistry(registry, bridgepkg.WithNow(func() time.Time { return now })),
		broker:  bridgepkg.NewBroker(nil, bridgepkg.WithDeliveryBrokerNow(func() time.Time { return now })),
	}
	t.Cleanup(bridges.broker.Close)
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}
	cfg := compozyconfig.DefaultWithHome(home)
	workspaceResolver := &fakeObserveWorkspaceResolver{
		expectedRef: observerWorkspaceID,
		resolved: compozyworkspace.ResolvedWorkspace{
			Workspace: compozyworkspace.Workspace{
				ID:      observerWorkspaceID,
				RootDir: workspace,
				Name:    "observe-workspace",
			},
			Config: cfg,
			Agents: []compozyconfig.AgentDef{{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			}},
		},
	}
	if err := registry.InsertWorkspace(ctx, compozyworkspace.Workspace{
		ID:        observerWorkspaceID,
		RootDir:   workspace,
		Name:      "observe-workspace",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}

	observer, err := New(ctx,
		WithRegistry(registry),
		WithHomePaths(home),
		WithSessionSource(source),
		WithBridgeSource(bridges),
		WithWorkspaceResolver(workspaceResolver),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithNow(func() time.Time { return now.Add(time.Hour) }),
		WithStartTime(now),
		WithVersionSource(func() version.Info {
			return version.Info{Version: "1.2.3"}
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return &harness{
		observer:    observer,
		registry:    registry,
		bridges:     bridges,
		home:        home,
		source:      source,
		now:         now,
		workspaceID: observerWorkspaceID,
		workspace:   workspace,
	}
}

func observeTestContext(t testing.TB) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func (h *harness) recordEvent(t *testing.T, sessionID string, eventType string, timestamp time.Time, text string) {
	t.Helper()

	h.observer.OnAgentEvent(testutil.Context(t), sessionID, acp.AgentEvent{
		Type:      eventType,
		TurnID:    "turn-" + strings.ReplaceAll(text, " ", "-"),
		Timestamp: timestamp,
		Text:      text,
	})
}

func (h *harness) recordUsage(t *testing.T, sessionID string, usage *acp.TokenUsage) {
	t.Helper()

	h.observer.OnAgentEvent(testutil.Context(t), sessionID, acp.AgentEvent{
		Type:      "done",
		TurnID:    "turn-" + sessionID,
		Timestamp: h.now.Add(time.Minute),
		Usage:     usage,
	})
}

func (h *harness) singleTokenStat(t *testing.T, sessionID string) store.TokenStats {
	t.Helper()

	stats, err := h.observer.QueryTokenStats(
		testutil.Context(t),
		store.TokenStatsQuery{SessionID: sessionID},
	)
	if err != nil {
		t.Fatalf("QueryTokenStats(%q) error = %v", sessionID, err)
	}
	if len(stats) != 1 {
		t.Fatalf("len(QueryTokenStats(%q)) = %d, want 1: %#v", sessionID, len(stats), stats)
	}
	return stats[0]
}

func newSession(id string, state session.State, workspace string, now time.Time) *session.Session {
	return &session.Session{
		ID:                   id,
		Name:                 strings.ToUpper(id),
		AgentName:            "coder",
		Provider:             "claude",
		EffectivePermissions: "approve-all",
		RuntimeStatus:        session.RuntimeStatusReady,
		WorkspaceID:          observerWorkspaceID,
		Workspace:            workspace,
		State:                state,
		ACPSessionID:         "acp-" + id,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

type countingObserveAgentResolver struct {
	calls atomic.Int64
	agent compozyconfig.AgentDef
}

func (r *countingObserveAgentResolver) ResolveAgent(
	_ string,
	_ *compozyworkspace.ResolvedWorkspace,
) (compozyconfig.AgentDef, error) {
	r.calls.Add(1)
	return r.agent, nil
}
