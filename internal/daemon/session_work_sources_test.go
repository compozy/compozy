package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspace"
)

// Suite: daemon session work sources
// Invariant: only operator-visible hierarchy children count as a session's active-child
// work; a session continued or forked from it names it as provenance and never defers its
// stop, and a daemon-owned internal child (auto-title) never holds it waiting.
func TestSessionWorkSourcesChildren(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		agent   string
		typ     session.Type
		lineage store.SessionLineage
	}{
		{
			name:  "Should not count a session continued from the parent as its child work",
			agent: "reviewer", typ: session.SessionTypeUser,
			lineage: store.SessionLineage{
				ParentSessionID: "sess-source", RootSessionID: "sess-source", SpawnDepth: 1,
				Kind: store.LineageKindContinue, OriginAgentName: "coder",
			},
		},
		{
			name:  "Should not count a daemon-owned auto-title child as its child work",
			agent: "titler", typ: session.SessionTypeSpawned,
			lineage: store.SessionLineage{
				ParentSessionID: "sess-source", RootSessionID: "sess-source", SpawnDepth: 1,
				Kind: store.LineageKindSpawn, SpawnRole: session.SpawnRoleAutoTitle,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertNoChildWork(t, tc.agent, tc.typ, tc.lineage)
		})
	}
}

func assertNoChildWork(t *testing.T, agent string, typ session.Type, lineage store.SessionLineage) {
	t.Helper()
	ctx := t.Context()
	registry, err := openDaemonTestGlobalDBAtPath(ctx, filepath.Join(t.TempDir(), "compozy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := registry.Close(context.Background()); err != nil {
			t.Errorf("close work store: %v", err)
		}
	})
	now := time.Now().UTC()
	if err := registry.InsertWorkspace(ctx, workspace.Workspace{
		ID: "ws-work", Name: "ws-work", RootDir: t.TempDir(), CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	homePaths := testHomePaths(t)
	if err := store.WriteSessionMeta(
		store.SessionMetaFile(filepath.Join(homePaths.SessionsDir, "sess-source")),
		&store.SessionMeta{
			ID: "sess-source", ProfileID: store.DefaultProfileID, WorkspaceID: "ws-work",
			AgentName: "coder", Provider: "mock", State: string(session.StateStopped),
			RuntimeStatus: store.SessionRuntimeUnbound, CreatedAt: now, UpdatedAt: now,
		},
	); err != nil {
		t.Fatal(err)
	}
	// The related session has no readable state: counting it would have to read it.
	if err := registry.RegisterSession(ctx, new(store.SessionInfo{
		ProfileID: store.DefaultProfileID, ID: "sess-related", AgentName: agent,
		WorkspaceID: "ws-work", SessionType: string(typ),
		State: string(session.StateActive), RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &lineage, CreatedAt: now, UpdatedAt: now,
	})); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}
	manager, err := session.NewManager(session.WithHomePaths(homePaths), session.WithLogger(discardLogger()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown work manager: %v", err)
		}
	})
	sources := sessionWorkSources{state: &bootState{registry: registry}, manager: manager, now: time.Now}

	work, err := sources.children(ctx, "sess-source")
	if err != nil {
		t.Fatalf("children() error = %v, want the related session ignored", err)
	}
	if len(work) != 0 {
		t.Fatalf("children() = %#v, want no child work", work)
	}
}

func TestSessionWorkSourcesMonitors(t *testing.T) {
	t.Parallel()
	// Owner: daemon monitor adapter; only the creator's covering session watch replaces its alarm.
	t.Run("Should suppress only alarms covered by the creator session monitor", func(t *testing.T) {
		t.Parallel()
		gate := make(chan struct{})
		service, err := monitor.NewService(t.Context(), nil, retryMonitorWaker{},
			monitor.WithSessionTargets(monitorSessionTargets{sessions: func() any { return nil }}),
			monitor.WithSessionWatchGate(gate))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		item, err := service.Create(t.Context(), monitor.Spec{
			WorkspaceID: "ws", ProfileID: store.DefaultProfileID, SessionID: "creator",
			Kind: monitor.KindSession, Description: "wait for child", Targets: []string{"child"},
			States: []string{monitor.TargetStateNeedsAttention},
		})
		if err != nil {
			t.Fatal(err)
		}
		sources := sessionWorkSources{state: &bootState{sessionMonitors: service}}
		child := &session.Info{ID: "child"}
		if !sources.creatorMonitorCovers("creator", child, session.BadgeNeedsAttention) {
			t.Fatal("creator's active watch did not cover the child's alarm")
		}
		if sources.creatorMonitorCovers("other", child, session.BadgeNeedsAttention) ||
			sources.creatorMonitorCovers("creator", &session.Info{ID: "other"}, session.BadgeNeedsAttention) ||
			sources.creatorMonitorCovers("creator", child, session.BadgeWaitingForInput) {
			t.Fatal("an unrelated owner, target, or state covered the child's alarm")
		}
		if _, err := service.Cancel("creator", item.ID); err != nil {
			t.Fatal(err)
		}
		if sources.creatorMonitorCovers("creator", child, session.BadgeNeedsAttention) {
			t.Fatal("a canceled monitor covered the child's alarm")
		}
	})

	for _, scenario := range []struct{ delivering, endedEarly bool }{
		{false, true}, {true, true}, {false, false}, {true, false},
	} {
		name := "Should retain coverage after the monitor wake is admitted"
		if scenario.delivering {
			name = "Should retain coverage while the monitor wake is delivering"
		}
		if !scenario.endedEarly {
			name += " for lifecycle attention"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				now := time.Now()
				child := &session.Info{ID: "child", WorkspaceID: "ws", ProfileID: store.DefaultProfileID,
					State: session.StateActive, EndedEarlyTurnID: "turn-1", AttentionChangedAt: &now}
				if !scenario.endedEarly {
					child.EndedEarlyTurnID = ""
					child.StopVerificationFailed = true
					child.AttentionRevision = 1
				}
				targets := &monitorTargetManagerStub{
					reads: map[string]int{}, infos: map[string]*session.Info{"child": child},
				}
				wakeManager := &monitorWakeManagerStub{status: &session.Info{State: session.StateActive}}
				var waker monitor.Waker = &monitorSessionWaker{
					sessions: func() monitorSessionManager { return wakeManager },
				}
				wantStatus := monitor.StatusFired
				if scenario.delivering {
					waker, wantStatus = retryMonitorWaker{}, monitor.StatusDelivering
				}
				service, err := monitor.NewService(t.Context(), nil, waker,
					monitor.WithSessionTargets(monitorSessionTargets{sessions: func() any { return targets }}))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := service.Shutdown(context.Background()); err != nil {
						t.Error(err)
					}
				})
				if _, err := service.Create(t.Context(), monitor.Spec{
					WorkspaceID: "ws", ProfileID: store.DefaultProfileID, SessionID: "creator",
					Kind: monitor.KindSession, Description: "child attention", Targets: []string{"child"},
					States: []string{monitor.TargetStateNeedsAttention},
				}); err != nil {
					t.Fatal(err)
				}
				synctest.Wait()
				items := service.List("creator")
				if len(items) != 1 || items[0].Status != wantStatus {
					t.Fatalf("monitors = %#v, want %s", items, wantStatus)
				}
				sources := sessionWorkSources{state: &bootState{sessionMonitors: service}}
				if !sources.creatorMonitorCovers("creator", child, session.BadgeNeedsAttention) {
					t.Fatal("a monitor that already observed the attention edge did not cover it")
				}
				seen := *child
				seen.AttentionChangedAt = new(now.Add(time.Second))
				if !sources.creatorMonitorCovers("creator", &seen, session.BadgeNeedsAttention) {
					t.Fatal("operator presence invalidated monitor coverage of the same child occurrence")
				}
				newer := seen
				newer.EndedEarlyTurnID = "turn-2"
				if !scenario.endedEarly {
					newer.EndedEarlyTurnID = ""
					newer.AttentionRevision++
				}
				if sources.creatorMonitorCovers("creator", &newer, session.BadgeNeedsAttention) {
					t.Fatal("a previous monitor outcome covered a newer child attention occurrence")
				}
			})
		})
	}

	// Owner: daemon coverage adapter; an unmatched fired monitor cannot hide another monitor's match.
	t.Run("Should scan past an unmatched fired monitor", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			now := time.Now()
			child := &session.Info{ID: "child", WorkspaceID: "ws", ProfileID: store.DefaultProfileID,
				State: session.StateActive, EndedEarlyTurnID: "turn-1", AttentionChangedAt: &now}
			targets := &monitorTargetManagerStub{
				reads: map[string]int{},
				infos: map[string]*session.Info{"child": child},
			}
			service, err := monitor.NewService(t.Context(), nil, retryMonitorWaker{},
				monitor.WithSessionTargets(monitorSessionTargets{sessions: func() any { return targets }}))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := service.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			spec := monitor.Spec{WorkspaceID: "ws", ProfileID: store.DefaultProfileID, SessionID: "creator",
				Kind: monitor.KindSession, Description: "child attention", Targets: []string{"child"},
				States: []string{monitor.TargetStateNeedsAttention}}
			matched, err := service.Create(t.Context(), spec)
			if err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			time.Sleep(time.Second)
			// The later until=any watch fires on a sibling while the child is running.
			targets.infos["child"] = &session.Info{
				ID: "child", State: session.StateActive,
				Liveness: &store.SessionLivenessMeta{Activity: &store.SessionActivityMeta{TurnID: "active-turn"}},
			}
			targets.infos["sibling"] = &session.Info{
				ID:               "sibling",
				State:            session.StateActive,
				EndedEarlyTurnID: "sibling-turn",
			}
			spec.Targets, spec.Until = []string{"child", "sibling"}, monitor.UntilAny
			unmatched, err := service.Create(t.Context(), spec)
			if err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			items := service.List("creator")
			if len(items) != 2 || items[0].ID != unmatched.ID || items[1].ID != matched.ID ||
				items[0].Status != monitor.StatusDelivering || items[1].Status != monitor.StatusDelivering {
				t.Fatalf("monitors = %+v, want unmatched outcome ahead of matching outcome", items)
			}
			sources := sessionWorkSources{state: &bootState{sessionMonitors: service}}
			if !sources.creatorMonitorCovers("creator", child, session.BadgeNeedsAttention) {
				t.Fatal("unmatched outcome hid child coverage")
			}
		})
	})

	t.Run("Should project only the owning session live monitors without renewing their deadline", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
		service, err := monitor.NewService(
			t.Context(),
			nil,
			&monitorSessionWaker{sessions: func() monitorSessionManager { return nil }},
			monitor.WithClock(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Errorf("shutdown monitors: %v", err)
			}
		})
		sources := sessionWorkSources{state: &bootState{sessionMonitors: service}}
		item, err := service.Create(
			t.Context(),
			monitor.Spec{
				WorkspaceID: "ws",
				SessionID:   "owner",
				Kind:        monitor.KindTimer,
				Description: "wait",
				Delay:       time.Hour,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		signals, err := sources.monitors(t.Context(), "owner")
		if err != nil || len(signals) != 1 {
			t.Fatalf("signals = %#v, %v", signals, err)
		}
		got := signals[0]
		if got.Kind != session.WorkSignalActiveMonitor || got.Ref != item.ID || !got.Since.Equal(item.CreatedAt) ||
			!got.ValidUntil.Equal(item.DeadlineAt) {
			t.Fatalf("signal = %#v", got)
		}
		if signals, err := sources.monitors(t.Context(), "other"); err != nil || len(signals) != 0 {
			t.Fatalf("foreign signals = %#v, %v", signals, err)
		}
		if _, err := service.Cancel("owner", item.ID); err != nil {
			t.Fatal(err)
		}
		if signals, err := sources.monitors(t.Context(), "owner"); err != nil || len(signals) != 0 {
			t.Fatalf("canceled signals = %#v, %v", signals, err)
		}
	})
	t.Run("Should retain waiting while an expired timer retries wake delivery", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			service, err := monitor.NewService(t.Context(), nil, retryMonitorWaker{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := service.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			sources := sessionWorkSources{state: &bootState{sessionMonitors: service}, now: time.Now}
			item, err := service.Create(
				t.Context(),
				monitor.Spec{
					WorkspaceID: "ws",
					SessionID:   "owner",
					Kind:        monitor.KindTimer,
					Description: "wait",
					Delay:       time.Minute,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Minute + time.Second)
			synctest.Wait()
			if current := service.List("owner"); len(current) != 1 || current[0].Status != monitor.StatusDelivering {
				t.Fatalf("monitors = %#v", current)
			}
			signals, err := sources.monitors(t.Context(), "owner")
			if err != nil || len(signals) != 1 || !signals[0].ValidUntil.After(time.Now()) ||
				signals[0].Ref != item.ID {
				t.Fatalf("delivering signals = %#v, %v", signals, err)
			}
			if _, err := service.Cancel("owner", item.ID); err != nil {
				t.Fatal(err)
			}
			if signals, err := sources.monitors(t.Context(), "owner"); err != nil || len(signals) != 0 {
				t.Fatalf("canceled signals = %#v, %v", signals, err)
			}
		})
	})
	t.Run("Should fail closed when the monitor service is unavailable", func(t *testing.T) {
		t.Parallel()
		sources := sessionWorkSources{state: &bootState{}}
		if _, err := sources.monitors(t.Context(), "owner"); err == nil {
			t.Fatal("missing monitor service was reported as no work")
		}
	})
}

type retryMonitorWaker struct{}

func (retryMonitorWaker) WakeSession(context.Context, monitor.Notification) error {
	return errors.New("input queue full")
}

// Owner: daemon wait adapter. Each authoritative source independently parks Goal work.
func TestLoopGoalWaitSources(t *testing.T) {
	for _, source := range []string{"monitor", "child", "none", "inspection error"} {
		t.Run("Should inspect "+source+" independently for Goal waits", func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			registry, err := openDaemonTestGlobalDBAtPath(ctx, filepath.Join(t.TempDir(), "compozy.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := registry.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			now := time.Now().UTC()
			if err := registry.InsertWorkspace(
				ctx,
				workspace.Workspace{ID: "ws-wait", Name: "wait", RootDir: t.TempDir(), CreatedAt: now, UpdatedAt: now},
			); err != nil {
				t.Fatal(err)
			}
			paths := testHomePaths(t)
			for _, id := range []string{"parent", "child"} {
				if err := store.WriteSessionMeta(
					store.SessionMetaFile(filepath.Join(paths.SessionsDir, id)),
					&store.SessionMeta{
						ID:            id,
						ProfileID:     store.DefaultProfileID,
						WorkspaceID:   "ws-wait",
						AgentName:     "coder",
						Provider:      "mock",
						State:         string(session.StateActive),
						RuntimeStatus: store.SessionRuntimeUnbound,
						CreatedAt:     now,
						UpdatedAt:     now,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			manager, err := session.NewManager(session.WithHomePaths(paths), session.WithLogger(discardLogger()))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := manager.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			monitors, err := monitor.NewService(ctx, nil, retryMonitorWaker{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := monitors.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			if source == "monitor" {
				if _, err := monitors.Create(
					ctx,
					monitor.Spec{
						WorkspaceID: "ws-wait",
						SessionID:   "parent",
						Kind:        monitor.KindTimer,
						Description: "wait",
						Delay:       time.Hour,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			if source == "child" {
				if err := registry.RegisterSession(ctx, &store.SessionInfo{
					ID:            "child",
					ProfileID:     store.DefaultProfileID,
					WorkspaceID:   "ws-wait",
					AgentName:     "coder",
					Provider:      "mock",
					State:         string(session.StateActive),
					RuntimeStatus: store.SessionRuntimeUnbound,
					SessionType:   string(session.SessionTypeSpawned),
					CreatedAt:     now,
					UpdatedAt:     now,
					Lineage: &store.SessionLineage{
						Kind:            store.LineageKindSpawn,
						ParentSessionID: "parent",
						RootSessionID:   "parent",
						SpawnDepth:      1,
						SpawnRole:       store.SubagentSpawnRole,
					},
				}); err != nil {
					t.Fatal(err)
				}
			}
			runtime := &loopGoalRuntime{
				waitSources: &sessionWorkSources{
					state:   &bootState{registry: registry, sessionMonitors: monitors},
					manager: manager,
					now:     time.Now,
				},
			}
			if source == "inspection error" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got, err := runtime.HasLiveWait(
				ctx,
				looppkg.ActionSessionBinding{WorkspaceID: "ws-wait", SessionID: "parent"},
			)
			if source == "inspection error" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("inspection error = %v", err)
				}
			} else if err != nil || got != (source != "none") {
				t.Fatalf("live wait = %v, %v", got, err)
			}
		})
	}
}

// Owner: Goal runtime recovery. A restart revives the exact parent without replaying Goal work.
func TestLoopGoalWaitSessionRecovery(t *testing.T) {
	for _, state := range []string{"active", "shutdown", "explicit stop", "resume failure"} {
		t.Run("Should handle "+state+" when restoring a bound wait session", func(t *testing.T) {
			t.Parallel()
			sessions := &goalWaitRecoverySessions{info: &session.Info{ID: "parent", State: session.StateActive}}
			if state != "active" {
				sessions.info.State = session.StateStopped
				sessions.info.StopReason = store.StopShutdown
			}
			if state == "explicit stop" {
				sessions.info.StopReason = store.StopUserCanceled
			}
			resumeErr := errors.New("resume transport unavailable")
			if state == "resume failure" {
				sessions.err = resumeErr
			}
			runtime := &loopGoalRuntime{loopActionSessionBinder: &loopActionSessionBinder{sessions: sessions}}
			err := runtime.RecoverWaitSession(t.Context(), looppkg.ActionSessionBinding{SessionID: "parent"})
			wantCalls := 0
			if state == "shutdown" || state == "resume failure" {
				wantCalls = 1
			}
			if sessions.calls != wantCalls {
				t.Fatalf("resume calls=%d, want %d", sessions.calls, wantCalls)
			}
			switch state {
			case "resume failure":
				if !errors.Is(err, resumeErr) {
					t.Fatalf("resume error=%v", err)
				}
			case "explicit stop":
				if err == nil || !strings.Contains(err.Error(), "stopped explicitly") {
					t.Fatalf("explicit stop=%v", err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

type goalWaitRecoverySessions struct {
	loopPromptSessionManager
	info  *session.Info
	err   error
	calls int
}

func (s *goalWaitRecoverySessions) Status(context.Context, string) (*session.Info, error) {
	return s.info, nil
}
func (s *goalWaitRecoverySessions) ResumeWaitingSession(_ context.Context, id string) error {
	if id != s.info.ID {
		return errors.New("wrong resumed session")
	}
	s.calls++
	return s.err
}
