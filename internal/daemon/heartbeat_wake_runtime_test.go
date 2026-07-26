package daemon

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	schedulerpkg "github.com/compozy/agh/internal/scheduler"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestHeartbeatWakeHealthReader(t *testing.T) {
	t.Parallel()

	t.Run("Should translate missing sessions into heartbeat wake decisions", func(t *testing.T) {
		t.Parallel()

		reader := heartbeatWakeHealthReader{reader: sessionMissingHealthReader{}}
		_, err := reader.GetSessionHealth(testutil.Context(t), "sess-missing")
		if !errors.Is(err, heartbeat.ErrSessionHealthNotFound) {
			t.Fatalf("GetSessionHealth() error = %v, want ErrSessionHealthNotFound", err)
		}
	})
}

func TestSchedulerHeartbeatWakeIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should wake eligible sessions through the heartbeat synthetic prompt path", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		workspaceID := "ws-heartbeat-scheduler"
		sessionID := "sess-heartbeat-scheduler"
		agentName := "coder"
		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		snapshot := seedDaemonHeartbeatWakePolicy(ctx, t, db, workspaceID, sessionID, agentName, base)
		sessions := &fakeSessionManager{
			infos: []*session.Info{{
				ID:          sessionID,
				AgentName:   agentName,
				WorkspaceID: workspaceID,
				State:       session.StateActive,
				CreatedAt:   base.Add(-time.Hour),
			}},
			healthRows: map[string]heartbeat.SessionHealth{
				sessionID: daemonEligibleHeartbeatHealth(sessionID, workspaceID, agentName, base),
			},
		}
		waker := newSchedulerSessionWaker(ctx, sessions, discardLogger())
		t.Cleanup(func() {
			if err := waker.shutdown(testutil.Context(t)); err != nil {
				t.Errorf("scheduler waker shutdown error = %v", err)
			}
		})
		if err := waker.configureHeartbeatWake(db, sessions, aghconfig.DefaultHeartbeatConfig()); err != nil {
			t.Fatalf("configureHeartbeatWake() error = %v", err)
		}

		err := waker.Wake(ctx, &schedulerpkg.WakeTarget{
			Work: schedulerpkg.RunSnapshot{
				Task: taskpkg.Task{ID: "task-heartbeat", WorkspaceID: workspaceID, Scope: taskpkg.ScopeWorkspace},
				Run:  taskpkg.Run{ID: "run-heartbeat", TaskID: "task-heartbeat", Status: taskpkg.TaskRunStatusQueued},
			},
			Session: schedulerpkg.SessionSnapshot{
				ID:          sessionID,
				AgentName:   agentName,
				WorkspaceID: workspaceID,
				State:       string(session.StateActive),
			},
			Reason: "pending_task_run",
		})
		if err != nil {
			t.Fatalf("Wake() error = %v", err)
		}
		if got, want := sessions.syntheticPromptCount(), 1; got != want {
			t.Fatalf("synthetic prompt count = %d, want %d", got, want)
		}
		call := sessions.syntheticPromptCalls[0]
		if got, want := call.opts.Metadata.Reason, heartbeat.SyntheticReasonHeartbeatWake; got != want {
			t.Fatalf("synthetic reason = %q, want %q", got, want)
		}
		if call.opts.Metadata.TaskRunID != "" {
			t.Fatalf("synthetic task run id = %q, want empty heartbeat metadata", call.opts.Metadata.TaskRunID)
		}
		if got, want := call.opts.Metadata.PolicySnapshotID, snapshot.ID; got != want {
			t.Fatalf("synthetic policy snapshot id = %q, want %q", got, want)
		}
		assertDaemonHeartbeatWakeEvent(t, db, workspaceID, agentName, heartbeat.WakeSourceScheduler)
	})

	t.Run(
		"Should carry coordinator session id on pending task synthetic wakes for coordinator sessions",
		func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			sessions := &fakeSessionManager{
				infos: []*session.Info{{
					ID:          "sess-coordinator-wake",
					AgentName:   aghconfig.BuiltinCoordinatorAgentName,
					WorkspaceID: "ws-coordinator-wake",
					Type:        session.SessionTypeCoordinator,
					State:       session.StateActive,
				}},
			}
			waker := newSchedulerSessionWaker(ctx, sessions, discardLogger())
			t.Cleanup(func() {
				if err := waker.shutdown(testutil.Context(t)); err != nil {
					t.Errorf("scheduler waker shutdown error = %v", err)
				}
			})
			target := &schedulerpkg.WakeTarget{
				Work: schedulerpkg.RunSnapshot{
					Task: taskpkg.Task{
						ID:          "task-coordinator-wake",
						WorkspaceID: "ws-coordinator-wake",
						Scope:       taskpkg.ScopeWorkspace,
					},
					Run: taskpkg.Run{
						ID:             "run-coordinator-wake",
						TaskID:         "task-coordinator-wake",
						Status:         taskpkg.TaskRunStatusQueued,
						ClaimTokenHash: "sha256:coordinator",
					},
				},
				Session: schedulerpkg.SessionSnapshot{
					ID:          "sess-coordinator-wake",
					AgentName:   aghconfig.BuiltinCoordinatorAgentName,
					WorkspaceID: "ws-coordinator-wake",
					Type:        string(session.SessionTypeCoordinator),
					State:       string(session.StateActive),
				},
				Reason: "pending_task_run",
			}

			if err := waker.wakePendingTaskRun(ctx, target, "sess-coordinator-wake"); err != nil {
				t.Fatalf("wakePendingTaskRun() error = %v", err)
			}
			if got, want := sessions.syntheticPromptCount(), 1; got != want {
				t.Fatalf("synthetic prompt count = %d, want %d", got, want)
			}

			call := sessions.syntheticPromptCalls[0]
			if got, want := call.opts.Metadata.CoordinatorSessionID, "sess-coordinator-wake"; got != want {
				t.Fatalf("synthetic coordinator session id = %q, want %q", got, want)
			}
			if got, want := call.opts.Metadata.TaskRunID, "run-coordinator-wake"; got != want {
				t.Fatalf("synthetic task run id = %q, want %q", got, want)
			}
			if got, want := call.opts.Metadata.ClaimTokenHash, "sha256:coordinator"; got != want {
				t.Fatalf("synthetic claim token hash = %q, want %q", got, want)
			}
		},
	)

	t.Run("Should apply heartbeat max wakes across scheduler batch dispatch", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		workspaceID := "ws-heartbeat-scheduler-batch"
		agentName := "coder"
		base := time.Date(2026, 5, 2, 12, 30, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		cfg.MaxWakesPerCycle = 1
		seedDaemonHeartbeatWakePolicyWithConfig(ctx, t, db, workspaceID, "sess-batch-a", agentName, base, cfg)
		registerDaemonHeartbeatSession(ctx, t, db, workspaceID, "sess-batch-b", agentName, base)
		sessions := &fakeSessionManager{
			infos: []*session.Info{
				{
					ID:          "sess-batch-a",
					AgentName:   agentName,
					WorkspaceID: workspaceID,
					State:       session.StateActive,
					CreatedAt:   base.Add(-time.Hour),
				},
				{
					ID:          "sess-batch-b",
					AgentName:   agentName,
					WorkspaceID: workspaceID,
					State:       session.StateActive,
					CreatedAt:   base.Add(-time.Hour),
				},
			},
			healthRows: map[string]heartbeat.SessionHealth{
				"sess-batch-a": daemonEligibleHeartbeatHealth("sess-batch-a", workspaceID, agentName, base),
				"sess-batch-b": daemonEligibleHeartbeatHealth("sess-batch-b", workspaceID, agentName, base),
			},
		}
		waker := newSchedulerSessionWaker(ctx, sessions, discardLogger())
		t.Cleanup(func() {
			if err := waker.shutdown(testutil.Context(t)); err != nil {
				t.Errorf("scheduler waker shutdown error = %v", err)
			}
		})
		if err := waker.configureHeartbeatWake(db, sessions, cfg); err != nil {
			t.Fatalf("configureHeartbeatWake() error = %v", err)
		}

		errs := waker.WakeMany(ctx, []schedulerpkg.WakeTarget{
			daemonHeartbeatWakeTarget("task-batch-a", "run-batch-a", workspaceID, agentName, "sess-batch-a"),
			daemonHeartbeatWakeTarget("task-batch-b", "run-batch-b", workspaceID, agentName, "sess-batch-b"),
		})
		if got, want := len(errs), 2; got != want {
			t.Fatalf("WakeMany() errors = %d, want %d", got, want)
		}
		for idx, err := range errs {
			if err != nil {
				t.Fatalf("WakeMany() error[%d] = %v", idx, err)
			}
		}
		if got, want := sessions.syntheticPromptCount(), 1; got != want {
			t.Fatalf("synthetic prompt count = %d, want %d", got, want)
		}
		assertDaemonHeartbeatWakeEventResults(
			t,
			db,
			workspaceID,
			agentName,
			map[heartbeat.WakeResult]int{
				heartbeat.WakeResultSent:        1,
				heartbeat.WakeResultRateLimited: 1,
			},
		)
	})
}

func TestHarnessHeartbeatWakeIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should route harness reentry through the heartbeat wake service and audit path", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		workspaceID := "ws-heartbeat-harness"
		sessionID := "sess-heartbeat-harness"
		agentName := "coder"
		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		snapshot := seedDaemonHeartbeatWakePolicy(ctx, t, db, workspaceID, sessionID, agentName, base)
		sessions := &fakeSessionManager{
			infos: []*session.Info{{
				ID:          sessionID,
				AgentName:   agentName,
				WorkspaceID: workspaceID,
				State:       session.StateActive,
				CreatedAt:   base.Add(-time.Hour),
			}},
			healthRows: map[string]heartbeat.SessionHealth{
				sessionID: daemonEligibleHeartbeatHealth(sessionID, workspaceID, agentName, base),
			},
		}
		bridge, err := newHarnessReentryBridge(
			ctx,
			NewHarnessContextResolver(HarnessRuntimeSignals{
				SyntheticTurnsEnabled:      true,
				DetachedTaskRuntimeEnabled: true,
			}),
			nil,
			db,
			sessions,
			discardLogger(),
			withHarnessHeartbeatWake(db, sessions, aghconfig.DefaultHeartbeatConfig()),
		)
		if err != nil {
			t.Fatalf("newHarnessReentryBridge() error = %v", err)
		}
		t.Cleanup(bridge.shutdown)

		handled := bridge.dispatchHeartbeatWake(harnessSyntheticWake{
			runID:             "run-heartbeat-harness",
			targetSessionID:   sessionID,
			targetAgentName:   agentName,
			targetWorkspaceID: workspaceID,
			syntheticMeta: acp.PromptSyntheticMeta{
				TaskID:         "task-heartbeat-harness",
				TaskRunID:      "run-heartbeat-harness",
				ClaimTokenHash: "sha256:heartbeat-harness",
			},
		})
		if !handled {
			t.Fatal("dispatchHeartbeatWake() = false, want heartbeat service handling")
		}
		if got, want := sessions.syntheticPromptCount(), 1; got != want {
			t.Fatalf("synthetic prompt count = %d, want %d", got, want)
		}
		call := sessions.syntheticPromptCalls[0]
		if got, want := call.opts.Metadata.Reason, heartbeat.SyntheticReasonHeartbeatWake; got != want {
			t.Fatalf("synthetic reason = %q, want %q", got, want)
		}
		if got, want := call.opts.Metadata.TaskID, "task-heartbeat-harness"; got != want {
			t.Fatalf("synthetic task id = %q, want %q", got, want)
		}
		if got, want := call.opts.Metadata.TaskRunID, "run-heartbeat-harness"; got != want {
			t.Fatalf("synthetic task run id = %q, want %q", got, want)
		}
		if got, want := call.opts.Metadata.ClaimTokenHash, "sha256:heartbeat-harness"; got != want {
			t.Fatalf("synthetic claim token hash = %q, want %q", got, want)
		}
		if got, want := call.opts.Metadata.PolicySnapshotID, snapshot.ID; got != want {
			t.Fatalf("synthetic policy snapshot id = %q, want %q", got, want)
		}
		assertDaemonHeartbeatWakeEvent(t, db, workspaceID, agentName, heartbeat.WakeSourceHarnessReentry)
	})

	t.Run("Should fall back to direct reentry when heartbeat cannot own the prompt", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name           string
			suffix         string
			configure      func(*heartbeat.SessionHealth, *fakeSessionManager)
			wantReason     heartbeat.WakeReason
			wantPrompts    int
			wantWakeEvents int
		}{
			{
				name:   "Should fall back after active prompt health",
				suffix: "active",
				configure: func(health *heartbeat.SessionHealth, _ *fakeSessionManager) {
					health.ActivePrompt = true
					health.EligibleForWake = false
					health.IneligibilityReason = string(heartbeat.SessionHealthReasonPromptActive)
				},
				wantReason:     heartbeat.WakeReasonSessionPromptActive,
				wantPrompts:    1,
				wantWakeEvents: 1,
			},
			{
				name:   "Should fall back after prompt race",
				suffix: "race",
				configure: func(_ *heartbeat.SessionHealth, sessions *fakeSessionManager) {
					sessions.syntheticPromptHook = func(
						_ context.Context,
						_ string,
						opts session.SyntheticPromptOpts,
					) (<-chan acp.AgentEvent, error) {
						if opts.SkipIfBusy {
							return nil, session.ErrPromptInProgress
						}
						ch := make(chan acp.AgentEvent)
						close(ch)
						return ch, nil
					}
				},
				wantReason:     heartbeat.WakeReasonSessionPromptRace,
				wantPrompts:    2,
				wantWakeEvents: 2,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				ctx := testutil.Context(t)
				db := openDaemonTestGlobalDB(t)
				workspaceID := "ws-heartbeat-busy-harness-" + tc.suffix
				sessionID := "sess-heartbeat-busy-harness-" + tc.suffix
				agentName := "coder"
				base := time.Date(2026, 5, 2, 13, 0, 0, 0, time.UTC)
				seedDaemonHeartbeatWakePolicy(ctx, t, db, workspaceID, sessionID, agentName, base)
				health := daemonEligibleHeartbeatHealth(sessionID, workspaceID, agentName, base)
				sessions := &fakeSessionManager{
					infos: []*session.Info{{
						ID:          sessionID,
						AgentName:   agentName,
						WorkspaceID: workspaceID,
						State:       session.StateActive,
						CreatedAt:   base.Add(-time.Hour),
					}},
					healthRows: map[string]heartbeat.SessionHealth{sessionID: health},
				}
				tc.configure(&health, sessions)
				sessions.healthRows[sessionID] = health
				bridge, err := newHarnessReentryBridge(
					ctx,
					NewHarnessContextResolver(HarnessRuntimeSignals{
						SyntheticTurnsEnabled:      true,
						DetachedTaskRuntimeEnabled: true,
					}),
					nil,
					db,
					sessions,
					discardLogger(),
					withHarnessHeartbeatWake(db, sessions, aghconfig.DefaultHeartbeatConfig()),
				)
				if err != nil {
					t.Fatalf("newHarnessReentryBridge() error = %v", err)
				}
				t.Cleanup(bridge.shutdown)

				wake := harnessSyntheticWake{
					runID:             "run-heartbeat-busy-harness-" + tc.suffix,
					targetSessionID:   sessionID,
					targetAgentName:   agentName,
					targetWorkspaceID: workspaceID,
					syntheticMessage:  "wake the busy session directly",
					syntheticMeta: acp.PromptSyntheticMeta{
						TaskID:    "task-heartbeat-busy-harness-" + tc.suffix,
						TaskRunID: "run-heartbeat-busy-harness-" + tc.suffix,
					},
				}
				bridge.dispatchWake(wake)
				if got := sessions.syntheticPromptCount(); got != tc.wantPrompts {
					t.Fatalf("synthetic prompt count = %d, want %d", got, tc.wantPrompts)
				}
				events, err := db.ListHeartbeatWakeEvents(testutil.Context(t), heartbeat.WakeEventListQuery{
					WorkspaceID: workspaceID,
					AgentName:   agentName,
					Source:      heartbeat.WakeSourceHarnessReentry,
				})
				if err != nil {
					t.Fatalf("ListHeartbeatWakeEvents() error = %v", err)
				}
				if got, want := len(events), tc.wantWakeEvents; got != want {
					t.Fatalf("wake event count = %d, want %d: %#v", got, want, events)
				}
				matchingSkippedEvents := 0
				for _, event := range events {
					if event.Result == heartbeat.WakeResultSkipped && event.Reason == tc.wantReason {
						matchingSkippedEvents++
					}
				}
				if got, want := matchingSkippedEvents, 1; got != want {
					t.Fatalf("skipped wake events = %d, want %d for reason %s: %#v", got, want, tc.wantReason, events)
				}

				sessions.mu.Lock()
				syntheticCalls := append([]fakeSyntheticPromptCall(nil), sessions.syntheticPromptCalls...)
				sessions.mu.Unlock()
				directCall := syntheticCalls[len(syntheticCalls)-1]
				if got := directCall.id; got != sessionID {
					t.Fatalf("synthetic prompt session id = %q, want %q", got, sessionID)
				}
				if !directCall.opts.InterruptIfAgentWaiting {
					t.Fatal("synthetic prompt InterruptIfAgentWaiting = false, want true")
				}
				if got, want := directCall.opts.Metadata.TaskRunID, wake.runID; got != want {
					t.Fatalf("synthetic prompt task run id = %q, want %q", got, want)
				}
			})
		}
	})
}

func (f *fakeSessionManager) GetSessionHealth(
	_ context.Context,
	sessionID string,
) (heartbeat.SessionHealth, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.healthRows == nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("fake: session health: %w", heartbeat.ErrSessionHealthNotFound)
	}
	health, ok := f.healthRows[sessionID]
	if !ok {
		return heartbeat.SessionHealth{}, fmt.Errorf("fake: session health: %w", heartbeat.ErrSessionHealthNotFound)
	}
	return health, nil
}

type sessionMissingHealthReader struct{}

func (sessionMissingHealthReader) GetSessionHealth(
	context.Context,
	string,
) (heartbeat.SessionHealth, error) {
	return heartbeat.SessionHealth{}, fmt.Errorf("fake: %w", session.ErrSessionNotFound)
}

func seedDaemonHeartbeatWakePolicy(
	ctx context.Context,
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceID string,
	sessionID string,
	agentName string,
	createdAt time.Time,
) heartbeat.Snapshot {
	return seedDaemonHeartbeatWakePolicyWithConfig(
		ctx,
		t,
		db,
		workspaceID,
		sessionID,
		agentName,
		createdAt,
		aghconfig.DefaultHeartbeatConfig(),
	)
}

func seedDaemonHeartbeatWakePolicyWithConfig(
	ctx context.Context,
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceID string,
	sessionID string,
	agentName string,
	createdAt time.Time,
	cfg aghconfig.HeartbeatConfig,
) heartbeat.Snapshot {
	t.Helper()

	root := t.TempDir()
	if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID:        workspaceID,
		Name:      workspaceID,
		RootDir:   root,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	if err := db.RegisterSession(ctx, store.SessionInfo{
		ID:          sessionID,
		AgentName:   agentName,
		WorkspaceID: workspaceID,
		State:       string(session.StateActive),
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}
	resolved, err := heartbeat.Parse(ctx, heartbeat.ParseRequest{
		SourcePath:    root + "/agents/" + agentName + "/" + heartbeat.FileName,
		WorkspaceRoot: root,
		Content: []byte(`---
version: 1
enabled: true
summary: "Runtime wake policy"
preferences:
  min_interval: "30m"
---
Inspect /agent/context before acting.
`),
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("heartbeat.Parse() error = %v", err)
	}
	snapshot, err := heartbeat.SnapshotFromResolved(
		"hb-"+sessionID,
		workspaceID,
		agentName,
		&resolved,
		createdAt,
	)
	if err != nil {
		t.Fatalf("SnapshotFromResolved() error = %v", err)
	}
	saved, err := db.UpsertHeartbeatSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("UpsertHeartbeatSnapshot() error = %v", err)
	}
	return saved
}

func registerDaemonHeartbeatSession(
	ctx context.Context,
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceID string,
	sessionID string,
	agentName string,
	createdAt time.Time,
) {
	t.Helper()

	if err := db.RegisterSession(ctx, store.SessionInfo{
		ID:          sessionID,
		AgentName:   agentName,
		WorkspaceID: workspaceID,
		State:       string(session.StateActive),
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}); err != nil {
		t.Fatalf("RegisterSession(%s) error = %v", sessionID, err)
	}
}

func daemonHeartbeatWakeTarget(
	taskID string,
	runID string,
	workspaceID string,
	agentName string,
	sessionID string,
) schedulerpkg.WakeTarget {
	return schedulerpkg.WakeTarget{
		Work: schedulerpkg.RunSnapshot{
			Task: taskpkg.Task{ID: taskID, WorkspaceID: workspaceID, Scope: taskpkg.ScopeWorkspace},
			Run:  taskpkg.Run{ID: runID, TaskID: taskID, Status: taskpkg.TaskRunStatusQueued},
		},
		Session: schedulerpkg.SessionSnapshot{
			ID:          sessionID,
			AgentName:   agentName,
			WorkspaceID: workspaceID,
			State:       string(session.StateActive),
		},
		Reason: "pending_task_run",
	}
}

func daemonEligibleHeartbeatHealth(
	sessionID string,
	workspaceID string,
	agentName string,
	at time.Time,
) heartbeat.SessionHealth {
	return heartbeat.SessionHealth{
		SessionID:       sessionID,
		WorkspaceID:     workspaceID,
		AgentName:       agentName,
		State:           heartbeat.SessionHealthStateIdle,
		Health:          heartbeat.SessionHealthHealthy,
		Attachable:      true,
		EligibleForWake: true,
		LastActivityAt:  at.Add(-2 * time.Minute),
		LastPresenceAt:  at.Add(-time.Minute),
		UpdatedAt:       at,
	}
}

func assertDaemonHeartbeatWakeEvent(
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceID string,
	agentName string,
	source heartbeat.WakeSource,
) {
	t.Helper()

	events, err := db.ListHeartbeatWakeEvents(testutil.Context(t), heartbeat.WakeEventListQuery{
		WorkspaceID: workspaceID,
		AgentName:   agentName,
		Source:      source,
	})
	if err != nil {
		t.Fatalf("ListHeartbeatWakeEvents() error = %v", err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("wake event count = %d, want %d: %#v", got, want, events)
	}
	if events[0].Result != heartbeat.WakeResultSent || events[0].Reason != heartbeat.WakeReasonSent {
		t.Fatalf("wake event = %#v, want sent heartbeat wake", events[0])
	}
}

func assertDaemonHeartbeatWakeEventResults(
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceID string,
	agentName string,
	want map[heartbeat.WakeResult]int,
) {
	t.Helper()

	events, err := db.ListHeartbeatWakeEvents(testutil.Context(t), heartbeat.WakeEventListQuery{
		WorkspaceID: workspaceID,
		AgentName:   agentName,
		Source:      heartbeat.WakeSourceScheduler,
	})
	if err != nil {
		t.Fatalf("ListHeartbeatWakeEvents() error = %v", err)
	}
	got := make(map[heartbeat.WakeResult]int, len(want))
	for _, event := range events {
		got[event.Result]++
	}
	wantTotal := 0
	for result, count := range want {
		wantTotal += count
		if got[result] != count {
			t.Fatalf("wake result %q count = %d, want %d: %#v", result, got[result], count, events)
		}
	}
	if len(events) != wantTotal {
		t.Fatalf("wake events = %d, want %d: %#v", len(events), wantTotal, events)
	}
}
