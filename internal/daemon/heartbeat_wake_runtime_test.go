package daemon

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	apitest "github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/providers"
	schedulerpkg "github.com/compozy/compozy/internal/scheduler"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	"github.com/compozy/compozy/internal/testutil/mcpfixture"
	tools "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
	"github.com/jonboulle/clockwork"
	"github.com/kballard/go-shellquote"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
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
	var missingContext context.Context
	if _, err := newSchedulerSessionWaker(missingContext, nil, discardLogger()); err == nil {
		t.Fatal("newSchedulerSessionWaker(nil context) error = nil, want context validation failure")
	}

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
		waker, err := newSchedulerSessionWaker(ctx, sessions, discardLogger())
		if err != nil {
			t.Fatalf("newSchedulerSessionWaker() error = %v", err)
		}
		t.Cleanup(func() {
			if err := waker.shutdown(testutil.Context(t)); err != nil {
				t.Errorf("scheduler waker shutdown error = %v", err)
			}
		})
		if err := waker.configureHeartbeatWake(db, sessions, compozyconfig.DefaultHeartbeatConfig()); err != nil {
			t.Fatalf("configureHeartbeatWake() error = %v", err)
		}

		err = waker.Wake(ctx, &schedulerpkg.WakeTarget{
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
					AgentName:   compozyconfig.BuiltinCoordinatorAgentName,
					WorkspaceID: "ws-coordinator-wake",
					Type:        session.SessionTypeCoordinator,
					State:       session.StateActive,
				}},
			}
			waker, err := newSchedulerSessionWaker(ctx, sessions, discardLogger())
			if err != nil {
				t.Fatalf("newSchedulerSessionWaker() error = %v", err)
			}
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
					AgentName:   compozyconfig.BuiltinCoordinatorAgentName,
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
			if !strings.Contains(call.opts.Message, "compozy__task_run_claim_next") ||
				!strings.Contains(call.opts.Message, "`run_id` set to \"run-coordinator-wake\"") {
				t.Fatalf("synthetic message = %q, want native claim instruction", call.opts.Message)
			}
			if strings.Contains(call.opts.Message, "compozy task next") {
				t.Fatalf("synthetic message = %q, want no CLI claim instruction", call.opts.Message)
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
		cfg := compozyconfig.DefaultHeartbeatConfig()
		cfg.MaxWakesPerCycle = 1
		seedDaemonHeartbeatWakePolicyWithConfig(ctx, t, db, workspaceID, "sess-batch-a", agentName, base, cfg)
		registerDaemonHeartbeatSession(ctx, t, db, workspaceID, "sess-batch-b", agentName, base)
		sessions := &fakeSessionManager{
			infos: []*session.Info{
				{
					ID:          "sess-batch-a",
					ProfileID:   store.DefaultProfileID,
					AgentName:   agentName,
					WorkspaceID: workspaceID,
					State:       session.StateActive,
					CreatedAt:   base.Add(-time.Hour),
				},
				{
					ID:          "sess-batch-b",
					ProfileID:   store.DefaultProfileID,
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
		waker, err := newSchedulerSessionWaker(ctx, sessions, discardLogger())
		if err != nil {
			t.Fatalf("newSchedulerSessionWaker() error = %v", err)
		}
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
		if errs[0] != nil || !errors.Is(errs[1], schedulerpkg.ErrWakeSkipped) {
			t.Fatalf("WakeMany outcomes = %v", errs)
		}
		if got, want := sessions.syntheticPromptCount(), 2; got != want {
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

func TestSchedulerSessionWakerShutdownClosesAdmissionAndJoinsDrains(t *testing.T) {
	t.Parallel()

	events := make(chan acp.AgentEvent)
	sessions := &fakeSessionManager{
		infos: []*session.Info{{ID: "sess-waker-shutdown", State: session.StateActive}},
		syntheticPromptHook: func(
			context.Context,
			string,
			session.SyntheticPromptOpts,
		) (<-chan acp.AgentEvent, error) {
			return events, nil
		},
	}
	waker, err := newSchedulerSessionWaker(testutil.Context(t), sessions, discardLogger())
	if err != nil {
		t.Fatalf("newSchedulerSessionWaker() error = %v", err)
	}
	target := daemonHeartbeatWakeTarget(
		"task-waker-shutdown",
		"run-waker-shutdown",
		"",
		"",
		"sess-waker-shutdown",
	)
	if err := waker.wakePendingTaskRun(testutil.Context(t), &target, "sess-waker-shutdown"); err != nil {
		t.Fatalf("wakePendingTaskRun() error = %v", err)
	}
	if err := waker.shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
	if err := waker.shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("shutdown(retry) error = %v", err)
	}
	if err := waker.wakePendingTaskRun(testutil.Context(t), &target, "sess-waker-shutdown"); err == nil {
		t.Fatal("wakePendingTaskRun(after shutdown) error = nil, want admission failure")
	}
	if got, want := sessions.syntheticPromptCount(), 1; got != want {
		t.Fatalf("synthetic prompt count = %d, want %d", got, want)
	}
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
			withHarnessHeartbeatWake(db, sessions, compozyconfig.DefaultHeartbeatConfig()),
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
					withHarnessHeartbeatWake(db, sessions, compozyconfig.DefaultHeartbeatConfig()),
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
		compozyconfig.DefaultHeartbeatConfig(),
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
	cfg compozyconfig.HeartbeatConfig,
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
	if err := db.RegisterSession(ctx, new(store.SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            sessionID,
		AgentName:     agentName,
		WorkspaceID:   workspaceID,
		State:         string(session.StateActive),
		RuntimeStatus: store.SessionRuntimeUnbound,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	})); err != nil {
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
		"",
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

	if err := db.RegisterSession(ctx, new(store.SessionInfo{
		ID:            sessionID,
		ProfileID:     store.DefaultProfileID,
		AgentName:     agentName,
		WorkspaceID:   workspaceID,
		State:         string(session.StateActive),
		RuntimeStatus: store.SessionRuntimeUnbound,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	})); err != nil {
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

func TestTimerPolicy(t *testing.T) {
	t.Run("Should default a user-level agent that lives outside the workspace", func(t *testing.T) {
		t.Parallel()
		agentDir := t.TempDir()
		path := filepath.Join(agentDir, "AGENT.md")
		if err := os.WriteFile(path, []byte("Agent"), 0600); err != nil {
			t.Fatal(err)
		}
		cfg := compozyconfig.DefaultHeartbeatConfig()
		cfg.Enabled = true
		policy, err := resolveTimerPolicy(
			t.Context(),
			nil,
			heartbeat.AuthoringTarget{WorkspaceRoot: t.TempDir(), AgentPath: path, Config: cfg},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !policy.Enabled || policy.Present {
			t.Fatalf("policy=%+v", policy)
		}
	})

	t.Run("Should default an absent timer policy without changing legacy resolution", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		path := filepath.Join(root, "AGENT.md")
		if err := os.WriteFile(path, []byte("Agent"), 0600); err != nil {
			t.Fatal(err)
		}
		cfg := compozyconfig.DefaultHeartbeatConfig()
		cfg.Enabled = true
		target := heartbeat.AuthoringTarget{WorkspaceRoot: root, AgentPath: path, Config: cfg}
		timer, err := resolveTimerPolicy(t.Context(), nil, target)
		if err != nil {
			t.Fatal(err)
		}
		if !timer.Enabled || timer.Present {
			t.Fatalf("timer=%+v", timer)
		}
		legacy, err := heartbeat.Resolve(
			t.Context(),
			heartbeat.ResolveRequest{WorkspaceRoot: root, AgentPath: path, Config: cfg},
		)
		if err != nil {
			t.Fatal(err)
		}
		if legacy.Present || legacy.Active {
			t.Fatalf("legacy=%+v", legacy)
		}
	})
	t.Run("Should preserve an explicitly disabled policy", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		path := filepath.Join(root, "AGENT.md")
		if err := os.WriteFile(path, []byte("Agent"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(root, "HEARTBEAT.md"),
			[]byte("---\nversion: 1\nenabled: false\n---\n"),
			0600,
		); err != nil {
			t.Fatal(err)
		}
		cfg := compozyconfig.DefaultHeartbeatConfig()
		cfg.Enabled = true
		policy, err := resolveTimerPolicy(
			t.Context(),
			nil,
			heartbeat.AuthoringTarget{WorkspaceRoot: root, AgentPath: path, Config: cfg},
		)
		if err != nil {
			t.Fatal(err)
		}
		if policy.Enabled || !policy.Present {
			t.Fatalf("policy=%+v", policy)
		}
	})
}

func TestHeartbeatScriptCollection(t *testing.T) {
	t.Run("Should poll approved scripts without writes during dry runs [IT-008 IT-006 UT-041]", func(t *testing.T) {
		t.Parallel()
		db := openDaemonTestGlobalDB(t)
		repo := db.Instances()
		now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
		_, err := db.DB().
			ExecContext(t.Context(), `INSERT INTO agent_instances (id,profile_id,agent_name,state,created_by_kind,created_by_id,created_at,updated_at) VALUES ('poll-test',?,'coder','paused','human','operator',?,?)`, store.DefaultProfileID, store.FormatTimestamp(now), store.FormatTimestamp(now))
		if err != nil {
			t.Fatal(err)
		}
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dir, "sources"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(dir, "sources", "poll"),
			[]byte(
				"#!/bin/sh\ncat >/dev/null\nprintf '{\"items\":[{\"key\":\"1\",\"title\":\"new\"}],\"state\":{\"cursor\":1}}'\n",
			),
			0700,
		); err != nil {
			t.Fatal(err)
		}
		_, hash, err := source.ReadScript(dir, "poll")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.ApproveScript(t.Context(), store.DefaultProfileID, dir, "poll", hash, "cli", now); err != nil {
			t.Fatal(err)
		}
		target := heartbeat.CheckTarget{
			DefinitionDir: dir,
			RootDir:       dir,
			Instance: agentinstance.Instance{
				ID:        "poll-test",
				ProfileID: store.DefaultProfileID,
				AgentName: "coder",
			},
			Policy: heartbeat.ResolvedPolicy{
				Preferences: heartbeat.Preferences{
					MinInterval: time.Hour,
					Context:     heartbeat.ContextProjection{Sources: []source.Ref{{ID: "s", Script: "poll"}}},
				},
			},
		}
		cfg := compozyconfig.DefaultHeartbeatSourcesConfig()
		cfg.Timeout = 10 * time.Second
		environment := instanceCheckEnvironment{
			instances:         repo,
			sourceConfig:      cfg,
			sourceNow:         func() time.Time { return now },
			sourceMaxInterval: 24 * time.Hour,
		}
		for range 2 {
			batch, err := environment.PollSources(t.Context(), &target, true)
			if err != nil || len(batch.Sources) != 1 || batch.Sources[0].Err != nil ||
				len(batch.Sources[0].Result.Items) != 1 {
				t.Fatalf("batch=%+v err=%v", batch, err)
			}
		}
		var rows int
		if err := db.DB().
			QueryRowContext(t.Context(), "SELECT count(*) FROM agent_instance_source_states WHERE instance_id='poll-test'").
			Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if rows != 0 {
			t.Fatalf("dry run wrote %d rows", rows)
		}
		batch, err := environment.PollSources(t.Context(), &target, false)
		if err != nil || len(batch.Sources) != 1 || batch.Sources[0].Err != nil {
			t.Fatalf("batch=%+v err=%v", batch, err)
		}
		states, err := repo.SourceStates(t.Context(), "poll-test", target.Policy.Preferences.Context.Sources, true, now)
		if err != nil || states[0].LastItemCount != 1 || len(states[0].State) != 0 {
			t.Fatalf("states=%+v err=%v", states, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sources", "poll"), []byte("#!/bin/sh\nexit 3\n"), 0700); err != nil {
			t.Fatal(err)
		}
		_, hash, err = source.ReadScript(dir, "poll")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.ApproveScript(t.Context(), store.DefaultProfileID, dir, "poll", hash, "cli", now); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Hour)
		for attempt := range 4 {
			batch, err = environment.PollSources(t.Context(), &target, false)
			if err != nil || batch.Sources[0].FailureStreak != int64(attempt+1) || batch.Sources[0].Err == nil ||
				batch.Sources[0].Err.Code != "source_exit_nonzero" {
				t.Fatalf("failure=%+v err=%v", batch, err)
			}
			states, err = repo.SourceStates(
				t.Context(),
				"poll-test",
				target.Policy.Preferences.Context.Sources,
				true,
				now,
			)
			if err != nil {
				t.Fatal(err)
			}
			expected := now.Add(time.Hour * time.Duration(1<<attempt))
			if !states[0].NextPollAt.Equal(expected) {
				t.Fatalf("next=%v expected=%v", states[0].NextPollAt, expected)
			}
			facts := batch.DigestItems(3)
			if (len(facts) == 1) != (attempt >= 2) {
				t.Fatalf("failure facts=%+v", facts)
			}
			now = expected
		}
		if err := os.WriteFile(
			filepath.Join(dir, "sources", "poll"),
			[]byte("#!/bin/sh\necho '{\"items\":[]}'\n"),
			0700,
		); err != nil {
			t.Fatal(err)
		}
		_, hash, err = source.ReadScript(dir, "poll")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.ApproveScript(t.Context(), store.DefaultProfileID, dir, "poll", hash, "cli", now); err != nil {
			t.Fatal(err)
		}
		batch, err = environment.PollSources(t.Context(), &target, false)
		if err != nil || batch.Sources[0].Err != nil || batch.Sources[0].FailureStreak != 0 {
			t.Fatalf("recovery=%+v err=%v", batch, err)
		}
	})
}

// heartbeatSourceRuntime exercises the real script runner, SQLite transactions,
// policy loader, check service and triage dispatcher. Only the provider is the
// existing deterministic ACP process fixture.
type heartbeatSourceRuntime struct {
	db                                *globaldb.GlobalDB
	manager                           *session.Manager
	env                               *instanceCheckEnvironment
	checks                            *heartbeat.CheckService
	instance                          agentinstance.Instance
	dir, requests, reply, diagnostics string
	clock                             atomic.Int64
	config                            heartbeat.CheckConfig
}

func newHeartbeatSourceRuntime(
	t *testing.T,
	approved, blocked bool,
	mainPermissions ...compozyconfig.PermissionMode,
) *heartbeatSourceRuntime {
	t.Helper()
	h := &heartbeatSourceRuntime{db: openDaemonTestGlobalDB(t)}
	h.clock.Store(time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC).UnixNano())
	home := testHomePaths(t)
	fixture := acpmock.Fixture{Version: 2, Agents: []acpmock.AgentFixture{
		{
			Name:        "source-agent",
			Provider:    acpmock.ProviderName,
			Permissions: "approve-all",
			Prompt:      "Review incoming items.",
			ConfigOptions: []acpmock.SessionConfigOptionFixture{
				{
					ID:       "mode",
					Name:     "Mode",
					Category: "mode",
					Current:  "read-only",
					Values:   []acpmock.SessionConfigOptionValueFixture{{Value: "read-only"}},
				},
			},
			Turns: []acpmock.TurnFixture{
				{
					Name:  "source-check",
					Match: acpmock.TurnMatch{RawUserTextContains: "Perform a proactive check."},
					Steps: []acpmock.Step{
						{
							Kind: acpmock.StepKindAssistant,
							Text: `{"proposals":[{"title":"Review source","body":"Inspect the new items.","item_keys":["source:s:1"]}]}`,
						},
					},
				},
			},
		},
	}}
	if blocked {
		fixture.Agents[0].Turns[0].Steps = []acpmock.Step{
			{
				Kind:          acpmock.StepKindDriverControl,
				DriverControl: &acpmock.DriverControlStep{Action: acpmock.DriverControlBlockUntilCancel},
			},
		}
	}
	fixturePath := filepath.Join(home.HomeDir, "source-fixture.json")
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixturePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	registered, err := acpmock.Register(
		home,
		acpmock.RegisterOptions{FixturePath: fixturePath, FixtureAgent: "source-agent"},
	)
	if err != nil {
		t.Fatal(err)
	}
	h.diagnostics = registered.DiagnosticsPath
	h.dir, err = filepath.EvalSymlinks(filepath.Dir(registered.AgentDefPath))
	if err != nil {
		t.Fatal(err)
	}
	def, err := compozyconfig.LoadAgentDefFile(registered.AgentDefPath)
	if err != nil {
		t.Fatal(err)
	}
	def.Kind = compozyconfig.AgentKindAlwaysOn
	cfg := compozyconfig.DefaultWithHome(home)
	cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(registered.Command)
	cfg.Agents.Heartbeat.Sources.Timeout = 10 * time.Second
	agents := stubSessionCommandAgentResolver(
		func(string, *workspacepkg.ResolvedWorkspace) (compozyconfig.AgentDef, error) { return def, nil },
	)
	root := t.TempDir()
	if err := h.db.InsertWorkspace(
		t.Context(),
		workspacepkg.Workspace{
			ID:        "source-workspace",
			Name:      "source-workspace",
			RootDir:   root,
			CreatedAt: h.now(),
			UpdatedAt: h.now(),
		},
	); err != nil {
		t.Fatal(err)
	}
	resolver, err := workspacepkg.NewResolver(
		h.db,
		workspacepkg.WithHomePaths(home),
		workspacepkg.WithProfileConfigLoader(func(string, string) (compozyconfig.Config, error) { return cfg, nil }),
	)
	if err != nil {
		t.Fatal(err)
	}
	h.manager, err = session.NewManager(
		session.WithLifecycleContext(t.Context()),
		session.WithHomePaths(home),
		session.WithDriver(session.NewACPDriverAdapter(acp.New(acp.WithProviderPreStarter(providers.NewPreStarter())))),
		session.WithWorkspaceResolver(resolver),
		session.WithAgentResolver(agents),
		session.WithProfileNameResolver(nativeProfileReaderStub{}),
		session.WithSessionCatalog(h.db),
		session.WithSessionPromptAdmissionStore(h.db),
		session.WithSessionInputQueueStore(h.db),
		session.WithEventLedger(h.db),
		session.WithAttentionWorkspaceMuteReader(h.db),
		session.WithPromptAssembler(NewComposedAssembler()),
		session.WithLogger(discardLogger()),
		session.WithStore(
			func(ctx context.Context, owner store.SessionDBOwner, path string) (session.EventRecorder, error) {
				return sessiondb.OpenSessionDB(ctx, owner, path)
			},
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		infos, err := h.manager.ListAll(ctx)
		if err != nil {
			t.Error(err)
		}
		for _, info := range infos {
			if info.State != session.StateStopped {
				if err := h.manager.Stop(ctx, info.ID); err != nil {
					t.Error(err)
				}
			}
		}
		if err := h.manager.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	mainOpts := &session.CreateOpts{AgentName: def.Name, Workspace: "source-workspace"}
	if len(mainPermissions) > 0 {
		mainOpts.Permissions = mainPermissions[0]
	}
	main, err := h.manager.Create(t.Context(), mainOpts)
	if err != nil {
		t.Fatal(err)
	}
	h.instance, err = h.db.Instances().
		Adopt(t.Context(), agentinstance.Instance{ProfileID: main.ProfileID, AgentName: main.AgentName, WorkspaceID: "source-workspace", PermissionCeiling: "{}", CreatedByKind: "human"}, main.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.DB().
		ExecContext(t.Context(), "UPDATE agent_instances SET created_at=? WHERE id=?", store.FormatTimestamp(h.now().Add(-24*time.Hour)), h.instance.ID); err != nil {
		t.Fatal(err)
	}
	h.requests, h.reply = filepath.Join(h.dir, "requests.jsonl"), filepath.Join(h.dir, "reply.json")
	if err := os.Mkdir(filepath.Join(h.dir, "sources"), 0700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nrequest=$(cat)\nprintf '%s\\n' \"$request\" >> " + shellquote.Join(
		h.requests,
	) + "\nif [ -p " + shellquote.Join(
		filepath.Join(h.dir, "gate"),
	) + " ]; then cat " + shellquote.Join(
		filepath.Join(h.dir, "gate"),
	) + " >/dev/null; fi\ncat " + shellquote.Join(
		h.reply,
	) + "\n"
	if err := os.WriteFile(filepath.Join(h.dir, "sources", "poll"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	h.writePolicy(t, "{}")
	h.writeReply(t, source.Result{Items: []source.Item{}})
	if approved {
		h.approve(t)
	}
	h.env = &instanceCheckEnvironment{
		sourceConfig:      cfg.Agents.Heartbeat.Sources,
		sourceNow:         h.now,
		sourceMaxInterval: cfg.Agents.Heartbeat.MaxInterval,
		instances:         h.db.Instances(),
		sessions:          h.manager,
		definitions: instanceDefinitions{
			sourceDeclarations: &instanceSourceDeclarations{},
			workspaces:         resolver,
			profiles:           nativeProfileReaderStub{},
			sessions:           h.manager,
			agents:             agents,
			scriptsEnabled:     cfg.Agents.Heartbeat.Sources.ScriptsEnabled,
		},
	}
	h.config = heartbeat.CheckConfig{
		Enabled:       true,
		Interval:      time.Hour,
		MinInterval:   cfg.Agents.Heartbeat.MinInterval,
		MaxInterval:   24 * time.Hour,
		Retention:     24 * time.Hour,
		Timeout:       30 * time.Second,
		MaxConcurrent: 1,
		BatchSize:     2,
		Sources:       cfg.Agents.Heartbeat.Sources,
	}
	h.startChecks(t)
	return h
}
func (h *heartbeatSourceRuntime) now() time.Time { return time.Unix(0, h.clock.Load()).UTC() }
func (h *heartbeatSourceRuntime) startChecks(t *testing.T) {
	t.Helper()
	var err error
	h.checks, err = heartbeat.NewCheckService(
		t.Context(),
		h.db.Instances(),
		h.env,
		&session.TriageDispatcher{
			Manager:      h.manager,
			Reports:      h.db.InstanceReports(),
			MaxProposals: 2,
			StopTimeout:  time.Second,
		},
		h.config,
		h.now,
	)
	if err != nil {
		t.Fatal(err)
	}
	checks := h.checks
	t.Cleanup(func() {
		if err := checks.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if err := checks.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
}
func (h *heartbeatSourceRuntime) approve(t *testing.T) {
	t.Helper()
	_, hash, err := source.ReadScript(h.dir, "poll")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.db.Instances().
		ApproveScript(t.Context(), h.instance.ProfileID, h.dir, "poll", hash, "cli", h.now()); err != nil {
		t.Fatal(err)
	}
}
func (h *heartbeatSourceRuntime) writePolicy(t *testing.T, params string) {
	t.Helper()
	body := "---\nversion: 1\nenabled: true\npreferences:\n  min_interval: 1h\ncontext:\n  include: []\n  sources:\n    - id: s\n      script: poll\n      with: " + params + "\n---\nInspect new source items.\n"
	if err := os.WriteFile(filepath.Join(h.dir, "HEARTBEAT.md"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func (h *heartbeatSourceRuntime) writeReply(t *testing.T, result source.Result) {
	t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.reply, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func (h *heartbeatSourceRuntime) states(t *testing.T) []heartbeat.SourceState {
	t.Helper()
	target, err := h.env.ResolveCheck(t.Context(), h.instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	states, err := h.db.Instances().
		SourceStates(t.Context(), h.instance.ID, target.Policy.Preferences.Context.Sources, true, h.now())
	if err != nil {
		t.Fatal(err)
	}
	return states
}
func (h *heartbeatSourceRuntime) inputs(t *testing.T) []source.Request {
	t.Helper()
	data, err := os.ReadFile(h.requests)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	// A live script can still be appending the final JSONL frame. Only the
	// newline publishes a complete request; malformed completed frames still fail.
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		return nil
	}
	var out []source.Request
	for line := range strings.SplitSeq(string(data[:end]), "\n") {
		var req source.Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			t.Fatal(err)
		}
		out = append(out, req)
	}
	return out
}
func (h *heartbeatSourceRuntime) waitIdle(t *testing.T, events int) {
	t.Helper()
	waitForConditionWithin(t, "source check settled", 15*time.Second, func() bool {
		var count int
		if err := h.db.DB().
			QueryRowContext(t.Context(), "SELECT count(*) FROM agent_heartbeat_wake_events WHERE instance_id=?", h.instance.ID).
			Scan(&count); err != nil {
			t.Fatal(err)
		}
		inflight, err := h.db.Instances().Inflight(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		return count == events && len(inflight) == 0 &&
			h.checks.SourceMetrics()["heartbeat_source_collections_inflight"] == 0
	})
}
func (h *heartbeatSourceRuntime) tick(t *testing.T, events int) {
	t.Helper()
	if err := h.checks.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	h.waitIdle(t, events)
}
func (h *heartbeatSourceRuntime) advance(t *testing.T) {
	t.Helper()
	wake, err := h.db.Instances().WakeState(t.Context(), h.instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	next, err := store.ParseTimestamp(wake.NextWakeAt)
	if err != nil {
		t.Fatal(err)
	}
	states := h.states(t)
	if states[0].NextPollAt.After(next) {
		next = states[0].NextPollAt
	}
	h.clock.Store(next.UnixNano())
}

// Invariant (IT-001/002/012): source pages are admitted once, cursor advancement
// follows successful checks, and source fences/provenance reach real triage sessions.
// Owning layer and canonical suite: daemon heartbeat wake runtime integration.
func TestHeartbeatSourceRuntime(t *testing.T) {
	for _, mode := range []string{"session_prompt_active", "heartbeat_invalid", "triage_failed"} {
		t.Run("Should restore timer cadence after post-poll "+mode, func(t *testing.T) {
			h := newHeartbeatSourceRuntime(t, true, false)
			env := &postPollHeartbeatEnvironment{instanceCheckEnvironment: h.env, mode: mode}
			checks, err := heartbeat.NewCheckService(
				t.Context(),
				h.db.Instances(),
				env,
				&session.TriageDispatcher{
					Manager:      h.manager,
					Reports:      h.db.InstanceReports(),
					MaxProposals: 2,
					StopTimeout:  time.Second,
				},
				h.config,
				h.now,
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := checks.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			if err := checks.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := checks.Tick(t.Context()); err != nil {
				t.Fatal(err)
			}
			want := store.FormatTimestamp(heartbeat.NextCheckTime(h.now(), time.Hour, h.instance.ID))
			waitForCondition(t, "post-poll schedule restored", func() bool {
				wake, err := h.db.Instances().WakeState(t.Context(), h.instance.ID)
				if err != nil {
					t.Fatal(err)
				}
				return wake.NextWakeAt == want
			})
			var reason string
			if err := h.db.DB().
				QueryRowContext(t.Context(), "SELECT reason FROM agent_heartbeat_wake_events WHERE instance_id=? ORDER BY created_at DESC LIMIT 1", h.instance.ID).
				Scan(&reason); err != nil {
				t.Fatal(err)
			}
			if reason != mode {
				t.Fatalf("reason=%s want=%s", reason, mode)
			}
		})
	}

	// Invariant: list rows share policy parsing, while edits invalidate the next read.
	t.Run("Should reuse source declarations across list rows and refresh edits", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		resolver := &countingInstancePolicyResolver{AgentResolver: h.env.definitions.agents}
		h.env.definitions.agents = resolver
		for n := range 3 {
			instance := h.instance
			instance.AgentName = fmt.Sprintf("alias-%d", n)
			if _, _, err := h.db.Instances().AdmitCreate(t.Context(), instance, 10); err != nil {
				t.Fatal(err)
			}
		}
		api := core.AgentInstanceAPI{
			Store: h.db.Instances(),
			ReadPayload: func(ctx context.Context, instance agentinstance.Instance) (contract.AgentInstancePayload, error) {
				return h.env.definitions.instancePayload(ctx, instance, h.db.Instances())
			},
		}
		read := func(want string) {
			t.Helper()
			page, err := api.ListPage(t.Context(), agentinstance.ListQuery{
				ProfileID: h.instance.ProfileID, AllWorkspaces: true, Limit: 10,
			}, workspaceaccess.ActorRef{Operator: true})
			if err != nil || len(page.Instances) != 4 {
				t.Fatalf("page=%+v err=%v", page, err)
			}
			for _, payload := range page.Instances {
				if len(payload.Sources) != 1 || payload.Sources[0].ID != want ||
					payload.Sources[0].Status != "pending_first_poll" {
					t.Fatalf("sources=%+v", payload.Sources)
				}
			}
		}
		read("s")
		read("s")
		if got := resolver.reads.Load(); got != 1 {
			t.Fatalf("policy reads=%d want=1", got)
		}
		path := filepath.Join(h.dir, "HEARTBEAT.md")
		body := "---\nversion: 1\nenabled: true\ncontext:\n  sources:\n    - id: edited\n      script: poll\n---\n"
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		read("edited")
		if got := resolver.reads.Load(); got != 2 {
			t.Fatalf("policy reads after edit=%d want=2", got)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Replace(body, "edited", "newest", 1)), 0600); err != nil {
			t.Fatal(err)
		}
		modified := info.ModTime().Add(time.Second)
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
		read("newest")
		if got := resolver.reads.Load(); got != 3 {
			t.Fatalf("policy reads after same-size edit=%d want=3", got)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		payload, err := api.Payload(t.Context(), h.instance)
		if err != nil || len(payload.Sources) != 0 {
			t.Fatalf("removed policy sources=%+v err=%v", payload.Sources, err)
		}
		payload, err = api.Payload(t.Context(), h.instance)
		if err != nil || len(payload.Sources) != 0 || resolver.reads.Load() != 4 {
			t.Fatalf("source-free payload=%+v reads=%d err=%v", payload.Sources, resolver.reads.Load(), err)
		}
	})

	// Invariant: production artifact policies invalidate on file identity or shipped body changes.
	// Owner and canonical suite: daemon instance payload projection in heartbeat source runtime.
	t.Run("Should refresh production artifact source declarations", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		resolver := &artifactInstancePolicyResolver{AgentResolver: h.env.definitions.agents}
		h.env.definitions.agents = resolver
		read := func(want string) {
			t.Helper()
			payload, err := h.env.definitions.instancePayload(t.Context(), h.instance, h.db.Instances())
			if err != nil || len(payload.Sources) != 1 || payload.Sources[0].ID != want {
				t.Fatalf("sources=%+v err=%v", payload.Sources, err)
			}
		}
		read("s")
		read("s")
		path := filepath.Join(h.dir, "HEARTBEAT.md")
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.Replace(body, []byte("id: s"), []byte("id: x"), 1)
		replacement := path + ".replacement"
		if err := os.WriteFile(replacement, body, info.Mode()); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(replacement, path); err != nil {
			t.Fatal(err)
		}
		read("x")
		resolver.packageOwned = true
		resolver.body = string(bytes.Replace(body, []byte("id: x"), []byte("id: y"), 1))
		read("y")
		read("y")
		resolver.body = strings.Replace(resolver.body, "id: y", "id: z", 1)
		read("z")
	})

	// Invariant: canceled reads stop before resolving even a cached policy.
	t.Run("Should cancel source declaration reads", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := h.env.definitions.sourceDeclarations.resolve(ctx, h.env.definitions.agents, nil,
			heartbeat.AuthoringTarget{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	})

	// Invariant: real instance payloads join declarations with matching persisted refs without mutating rows.
	t.Run("Should project current declarations through the real payload producer", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		api := core.AgentInstanceAPI{
			Checks: h.checks,
			ReadPayload: func(ctx context.Context, instance agentinstance.Instance) (contract.AgentInstancePayload, error) {
				return h.env.definitions.instancePayload(ctx, instance, h.db.Instances())
			},
		}
		read := func(status, script, kind string) {
			t.Helper()
			payload, err := api.Payload(t.Context(), h.instance)
			if err != nil || len(payload.Sources) != 1 || payload.Sources[0].Status != status ||
				payload.Sources[0].Script != script ||
				payload.Sources[0].Kind != kind {
				t.Fatalf("payload=%+v err=%v", payload.Sources, err)
			}
		}
		read("pending_first_poll", "poll", "")
		h.tick(t, 1)
		read("ok", "poll", "")
		if err := os.WriteFile(
			filepath.Join(h.dir, "HEARTBEAT.md"),
			[]byte("---\ncontext: [invalid\n---\n"),
			0600,
		); err != nil {
			t.Fatal(err)
		}
		fallback, err := api.Payload(t.Context(), h.instance)
		if err != nil || len(fallback.Sources) != 1 || fallback.Sources[0].Status != "ok" {
			t.Fatalf("fallback=%+v err=%v", fallback.Sources, err)
		}
		h.writePolicy(t, "{repo: changed}")
		read("pending_first_poll", "poll", "")
		body := "---\nversion: 1\nenabled: true\ncontext:\n  sources:\n    - id: other\n      kind: forge-github/issues\n---\n"
		if err := os.WriteFile(filepath.Join(h.dir, "HEARTBEAT.md"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		read("pending_first_poll", "", "forge-github/issues")
		if err := os.WriteFile(
			filepath.Join(h.dir, "HEARTBEAT.md"),
			[]byte("---\nenabled: true\ncontext:\n  include: []\n---\n"),
			0600,
		); err != nil {
			t.Fatal(err)
		}
		payload, err := api.Payload(t.Context(), h.instance)
		if err != nil || len(payload.Sources) != 0 {
			t.Fatalf("removed=%+v err=%v", payload.Sources, err)
		}
	})
	// Invariant (#1258): an unapproved script reads as awaiting approval from its bytes alone, so a
	// new or paused instance that is never polled still asks; approval clears it, an edit asks again,
	// and the read follows the poller's scripts_enabled and approval key (resolved script path).
	t.Run("Should project script approval before the first check and while paused", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, false, false)
		api := core.AgentInstanceAPI{
			Checks: h.checks,
			ReadPayload: func(ctx context.Context, instance agentinstance.Instance) (contract.AgentInstancePayload, error) {
				return h.env.definitions.instancePayload(ctx, instance, h.db.Instances())
			},
		}
		read := func(instance agentinstance.Instance, awaiting bool) heartbeat.HeartbeatSourceStatus {
			t.Helper()
			payload, err := api.Payload(t.Context(), instance)
			if err != nil || len(payload.Sources) != 1 || payload.Sources[0].AwaitingApproval != awaiting ||
				payload.Sources[0].Status != "pending_first_poll" {
				t.Fatalf("payload=%+v err=%v", payload.Sources, err)
			}
			return payload.Sources[0]
		}
		_, hash, err := source.ReadScript(h.dir, "poll")
		if err != nil {
			t.Fatal(err)
		}
		if got := read(h.instance, true); got.ScriptSHA256 != hash ||
			got.ScriptPath != filepath.Join(h.dir, "sources", "poll") {
			t.Fatalf("hash=%q path=%q want %q", got.ScriptSHA256, got.ScriptPath, hash)
		}
		// With scripts off for the poller, nothing would run, so nothing asks.
		h.env.definitions.scriptsEnabled = false
		if got := read(h.instance, false); got.ScriptSHA256 != "" {
			t.Fatalf("disabled hash=%q", got.ScriptSHA256)
		}
		h.env.definitions.scriptsEnabled = true
		result, err := h.db.Instances().Pause(t.Context(), h.instance.ID, "manual")
		if err != nil {
			t.Fatal(err)
		}
		paused := result.Instance
		if paused.State != "paused" {
			t.Fatalf("state=%q", paused.State)
		}
		read(paused, true)
		h.approve(t)
		read(paused, false)
		script := filepath.Join(h.dir, "sources", "poll")
		body, err := os.ReadFile(script)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(script, append(body, []byte("# edited\n")...), 0700); err != nil {
			t.Fatal(err)
		}
		if got := read(paused, true); got.ScriptSHA256 == hash || got.ScriptSHA256 == "" {
			t.Fatalf("changed hash=%q", got.ScriptSHA256)
		}
	})
	// Invariant: deferred hook exits persist the timer event and resolved cadence.
	for _, tc := range []struct {
		name, reason string
		failure      error
	}{
		{"Should persist source hook cancellation", "hook_cancelled", hookspkg.ErrHeartbeatWakeCancelled}, //nolint:misspell // Persisted wake reason.
		{"Should persist source hook error", "triage_failed", errors.New("hook unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHeartbeatSourceRuntime(t, true, false)
			h.env.hooks = &hooksNotifier{
				logger: discardLogger(),
				now:    h.now,
				hooks:  heartbeatCancelTestRuntime{fakeHookRuntime: &fakeHookRuntime{}, failure: tc.failure},
			}
			delay := time.Hour
			if tc.reason == "triage_failed" {
				delay = 4 * time.Hour
				if _, err := h.db.DB().
					ExecContext(t.Context(), "UPDATE agent_instance_wake_state SET failure_streak=2, quiet_streak=9 WHERE instance_id=?", h.instance.ID); err != nil {
					t.Fatal(err)
				}
			}
			h.tick(t, 1)
			wake, err := h.db.Instances().WakeState(t.Context(), h.instance.ID)
			want := store.FormatTimestamp(heartbeat.NextCheckTime(h.now(), delay, h.instance.ID))
			if err != nil || wake.NextWakeAt != want {
				t.Fatalf("wake=%+v want=%s err=%v", wake, want, err)
			}
			var reason string
			if err := h.db.DB().
				QueryRowContext(t.Context(), "SELECT reason FROM agent_heartbeat_wake_events WHERE instance_id=?", h.instance.ID).
				Scan(&reason); err != nil ||
				reason != tc.reason {
				t.Fatalf("reason=%s err=%v", reason, err)
			}
			if len(h.inputs(t)) != 0 {
				t.Fatal("hook exit polled source")
			}
		})
	}
	t.Run("Should reconcile removed sources only on a real check", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		h.tick(t, 1)
		if err := os.WriteFile(
			filepath.Join(h.dir, "HEARTBEAT.md"),
			[]byte("---\nenabled: true\ncontext:\n  include: []\n---\n"),
			0600,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := h.checks.Check(t.Context(), h.instance.ID, true); err != nil {
			t.Fatal(err)
		}
		statuses, err := h.db.Instances().SourceStatuses(t.Context(), h.instance.ID)
		if err != nil || len(statuses) != 1 {
			t.Fatalf("dry run changed rows: %+v %v", statuses, err)
		}
		if _, err := h.checks.Check(t.Context(), h.instance.ID, false); err != nil {
			t.Fatal(err)
		}
		statuses, err = h.db.Instances().SourceStatuses(t.Context(), h.instance.ID)
		if err != nil || len(statuses) != 0 {
			t.Fatalf("stale rows: %+v %v", statuses, err)
		}
	})
	t.Run("Should avoid quiet backoff after a source policy resolution failure", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		h.tick(t, 1)
		h.advance(t)
		if _, err := h.db.DB().
			ExecContext(t.Context(), `UPDATE agent_instance_wake_state SET quiet_streak=9 WHERE instance_id=?`, h.instance.ID); err != nil {
			t.Fatal(err)
		}
		h.env.definitions.agents = stubSessionCommandAgentResolver(
			func(string, *workspacepkg.ResolvedWorkspace) (compozyconfig.AgentDef, error) {
				return compozyconfig.AgentDef{}, errors.New("definition unavailable")
			},
		)
		if err := h.checks.Tick(t.Context()); err == nil {
			t.Fatal("expected preparation failure")
		}
		wake, err := h.db.Instances().WakeState(t.Context(), h.instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if wake.NextWakeAt != store.FormatTimestamp(heartbeat.NextCheckTime(h.now(), time.Hour, h.instance.ID)) {
			t.Fatalf("next=%s", wake.NextWakeAt)
		}
	})
	t.Run("Should poll two approved scripts concurrently within one instance", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		release := h.blockPoll(t)
		target, err := h.env.ResolveCheck(t.Context(), h.instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		target.Policy.Preferences.Context.Sources = []heartbeat.SourceDecl{
			{ID: "one", Script: "poll"},
			{ID: "two", Script: "poll"},
		}
		done := make(chan error, 1)
		go func() { _, err := h.env.PollSources(t.Context(), &target, true); done <- err }()
		waitForCondition(t, "both scripts entered before release", func() bool { return len(h.inputs(t)) == 2 })
		release()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should schedule an unchanged more batch at the daemon floor", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		h.writeReply(t, source.Result{Items: []source.Item{{Key: "1", Title: "same"}}, More: true})
		h.tick(t, 1)
		h.advance(t)
		h.tick(t, 2)
		wake, err := h.db.Instances().WakeState(t.Context(), h.instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if wake.NextWakeAt != store.FormatTimestamp(
			heartbeat.NextCheckTime(h.now(), h.config.MinInterval, h.instance.ID),
		) {
			t.Fatalf("next=%s", wake.NextWakeAt)
		}
		var reason string
		if err := h.db.DB().
			QueryRowContext(t.Context(), `SELECT reason FROM agent_heartbeat_wake_events WHERE instance_id=? ORDER BY created_at DESC LIMIT 1`, h.instance.ID).
			Scan(&reason); err != nil ||
			reason != "digest_unchanged" {
			t.Fatalf("reason=%s err=%v", reason, err)
		}
	})
	t.Run("Should poll a healthy source on a real check immediately after a timer", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		h.tick(t, 1)
		if !h.states(t)[0].NextPollAt.IsZero() {
			t.Fatal("healthy source has failure backoff")
		}
		h.writeReply(
			t,
			source.Result{Items: []source.Item{{Key: "new", Title: "New item"}}, State: []byte(`{"cursor":1}`)},
		)
		result, err := h.checks.Check(t.Context(), h.instance.ID, false)
		if err != nil || result.OccurrenceID == "" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		h.waitIdle(t, 2)
		if len(h.inputs(t)) != 2 || string(h.states(t)[0].State) != `{"cursor":1}` {
			t.Fatalf("source did not poll: %+v", h.states(t))
		}
		h.env.definitions = instanceDefinitions{}
		statuses, err := h.checks.SourceStatuses(t.Context(), h.instance.ID)
		if err != nil || len(statuses) != 1 || statuses[0].ID != "s" || statuses[0].Status != "ok" {
			t.Fatalf("statuses=%+v err=%v", statuses, err)
		}
	})
	t.Run("Should commit empty results on three timer ticks without triage [IT-001]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		for i := 1; i <= 3; i++ {
			h.writeReply(t, source.Result{Items: []source.Item{}, State: []byte(fmt.Sprintf(`{"cursor":%d}`, i))})
			h.tick(t, i)
			if got := string(h.states(t)[0].State); got != fmt.Sprintf(`{"cursor":%d}`, i) {
				t.Fatalf("committed state=%s", got)
			}
			h.advance(t)
		}
		var empty, triage int
		if err := h.db.DB().
			QueryRowContext(t.Context(), "SELECT count(*) FROM agent_heartbeat_wake_events WHERE instance_id=? AND reason='digest_empty'", h.instance.ID).
			Scan(&empty); err != nil {
			t.Fatal(err)
		}
		if err := h.db.DB().
			QueryRowContext(t.Context(), "SELECT count(*) FROM sessions WHERE session_type='triage'").
			Scan(&triage); err != nil {
			t.Fatal(err)
		}
		if empty != 3 || triage != 0 || len(h.inputs(t)) != 3 {
			t.Fatalf("empty=%d triage=%d inputs=%d", empty, triage, len(h.inputs(t)))
		}
	})
	t.Run("Should fence both items and persist proposal provenance [IT-002]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		h.writeReply(
			t,
			source.Result{
				Items: []source.Item{
					{Key: "1", Title: "first", Summary: "first payload"},
					{Key: "2", Title: "second", Summary: "second payload"},
				},
				State: []byte(`{"cursor":2}`),
			},
		)
		h.tick(t, 1)
		reports, err := h.db.InstanceReports().RecentReports(t.Context(), h.instance.ID, 10)
		if err != nil || len(reports) != 1 || !strings.Contains(string(reports[0].Payload), `"kind":"source"`) ||
			!strings.Contains(string(reports[0].Payload), `"source_id":"s"`) {
			t.Fatalf("reports=%+v err=%v", reports, err)
		}
		records, err := acpmock.ReadDiagnostics(h.diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		prompts := acpmock.PromptDiagnostics(records)
		if len(prompts) != 1 {
			t.Fatalf("prompts=%+v", prompts)
		}
		data, err := json.Marshal(prompts[0])
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(prompts[0].Prompt, "<untrusted_source_item ") != 2 {
			t.Fatalf("expected two source fences: %s", prompts[0].Prompt)
		}
		for _, want := range []string{"source:s:1", "source:s:2", "untrusted_source", "first payload", "second payload"} {
			if !strings.Contains(string(data), want) {
				t.Fatalf("prompt missing %s: %s", want, data)
			}
		}
		if got := string(h.states(t)[0].State); got != `{"cursor":2}` {
			t.Fatalf("committed=%s", got)
		}
	})
	t.Run("Should page all thirty five keys at the minimum cadence [IT-012]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		for page := range 2 {
			first, last := 1, 20
			if page == 1 {
				first, last = 21, 35
			}
			items := []source.Item{}
			for i := first; i <= last; i++ {
				items = append(items, source.Item{Key: fmt.Sprint(i), Title: fmt.Sprintf("item %d", i)})
			}
			h.writeReply(
				t,
				source.Result{Items: items, State: []byte(fmt.Sprintf(`{"cursor":%d}`, last)), More: page == 0},
			)
			h.tick(t, page+1)
			wake, err := h.db.Instances().WakeState(t.Context(), h.instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			next, err := store.ParseTimestamp(wake.NextWakeAt)
			if err != nil {
				t.Fatal(err)
			}
			interval := time.Hour
			if page == 0 {
				interval = h.config.MinInterval
			}
			if !next.Equal(heartbeat.NextCheckTime(h.now(), interval, h.instance.ID)) {
				t.Fatalf("page cadence=%v", next.Sub(h.now()))
			}
			h.advance(t)
		}
		records, err := acpmock.ReadDiagnostics(h.diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		prompts := acpmock.PromptDiagnostics(records)
		if len(prompts) != 2 {
			t.Fatalf("prompts=%d", len(prompts))
		}
		for i, prompt := range prompts {
			data, err := json.Marshal(prompt)
			if err != nil {
				t.Fatal(err)
			}
			first, last := 1, 20
			if i == 1 {
				first, last = 21, 35
			}
			for key := first; key <= last; key++ {
				if !strings.Contains(string(data), fmt.Sprintf("source:s:%d\\\"", key)) {
					t.Fatalf("page %d missing key %d: %s", i, key, data)
				}
			}
			if i == 0 && !strings.Contains(string(data), "source:s::more") {
				t.Fatalf("missing more: %s", data)
			}
		}
		inputs := h.inputs(t)
		if len(inputs) != 2 || string(inputs[1].State) != `{"cursor":20}` {
			t.Fatalf("inputs=%+v", inputs)
		}
	})
}

func (h *heartbeatSourceRuntime) blockPoll(t *testing.T) func() {
	t.Helper()
	gate := filepath.Join(h.dir, "gate")
	if out, err := exec.CommandContext(t.Context(), "mkfifo", gate).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %s %v", out, err)
	}
	return func() {
		writer, err := os.OpenFile(gate, os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(gate); err != nil {
			t.Fatal(err)
		}
	}
}

// Invariant (IT-003/009/011): a collected batch cannot advance the cursor after
// timeout, capacity loss or an authored ref edit. Owning suite: daemon heartbeat.
func TestHeartbeatSourceRetention(t *testing.T) {
	t.Run("Should retain the old cursor after triage timeout [IT-003]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, true)
		h.writeReply(t, source.Result{Items: []source.Item{}, State: []byte(`{"cursor":7}`)})
		h.tick(t, 1)
		h.advance(t)
		if err := h.checks.Shutdown(t.Context()); err != nil {
			t.Fatal(err)
		}
		h.config.Timeout = time.Second
		h.startChecks(t)
		h.writeReply(
			t,
			source.Result{Items: []source.Item{{Key: "1", Title: "retry me"}}, State: []byte(`{"cursor":1}`)},
		)
		h.tick(t, 2)
		var reason string
		if err := h.db.DB().
			QueryRowContext(t.Context(), "SELECT reason FROM agent_heartbeat_wake_events WHERE instance_id=? ORDER BY created_at DESC LIMIT 1", h.instance.ID).
			Scan(&reason); err != nil {
			t.Fatal(err)
		}
		if reason != "triage_timeout" || string(h.states(t)[0].State) != `{"cursor":7}` {
			t.Fatalf("reason=%s states=%+v", reason, h.states(t))
		}
		result, err := h.checks.Check(t.Context(), h.instance.ID, true)
		if err != nil || result.Gate != "would_run" {
			t.Fatalf("retry=%+v %v", result, err)
		}
		inputs := h.inputs(t)
		if len(inputs) != 3 || string(inputs[len(inputs)-1].State) != `{"cursor":7}` {
			t.Fatalf("retry stdin=%+v", inputs)
		}
	})
	t.Run("Should discard a batch when another check takes the last slot [IT-003]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		h.writeReply(t, source.Result{Items: []source.Item{}, State: []byte(`{"cursor":7}`)})
		h.tick(t, 1)
		h.advance(t)
		h.writeReply(
			t,
			source.Result{Items: []source.Item{{Key: "1", Title: "retry me"}}, State: []byte(`{"cursor":1}`)},
		)
		release := h.blockPoll(t)
		finished := make(chan heartbeat.CheckResult, 1)
		errors := make(chan error, 1)
		go func() {
			result, err := h.checks.Check(t.Context(), h.instance.ID, false)
			finished <- result
			errors <- err
		}()
		waitForCondition(t, "script waiting on gate", func() bool { return len(h.inputs(t)) == 2 })
		if _, err := h.db.DB().
			ExecContext(t.Context(), `INSERT INTO agent_instances (id,profile_id,agent_name,state,created_by_kind,created_by_id,created_at,updated_at) VALUES ('other-check',?,'other','active','human','operator',?,?)`, store.DefaultProfileID, store.FormatTimestamp(h.now()), store.FormatTimestamp(h.now())); err != nil {
			t.Fatal(err)
		}
		if _, err := h.db.DB().
			ExecContext(t.Context(), "INSERT INTO agent_instance_wake_state(instance_id,occurrence_id) VALUES ('other-check','other-occurrence')"); err != nil {
			t.Fatal(err)
		}
		release()
		select {
		case result := <-finished:
			if err := <-errors; err != nil || result.Gate != "no_slot" {
				t.Fatalf("result=%+v %v", result, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("poll did not return")
		}
		var pending *string
		if err := h.db.DB().
			QueryRowContext(t.Context(), "SELECT pending_occurrence_id FROM agent_instance_source_states WHERE instance_id=?", h.instance.ID).
			Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if pending != nil || string(h.states(t)[0].State) != `{"cursor":7}` {
			t.Fatalf("pending=%v states=%+v", pending, h.states(t))
		}
	})
	t.Run("Should admit one timer batch while manual polling is gated [IT-009]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		h.writeReply(t, source.Result{Items: []source.Item{{Key: "1", Title: "one"}}, State: []byte(`{"cursor":1}`)})
		release := h.blockPoll(t)
		if err := h.checks.Tick(t.Context()); err != nil {
			t.Fatal(err)
		}
		waitForCondition(t, "timer poll started", func() bool { return len(h.inputs(t)) == 1 })
		before := h.states(t)[0]
		result, err := h.checks.Check(t.Context(), h.instance.ID, false)
		if err != nil || result.Gate != "source_poll_in_flight" {
			t.Fatalf("manual=%+v %v", result, err)
		}
		release()
		h.waitIdle(t, 1)
		after := h.states(t)[0]
		if string(after.State) != `{"cursor":1}` || after.Generation != before.Generation+2 || len(h.inputs(t)) != 1 {
			t.Fatalf("before=%+v after=%+v", before, after)
		}
	})
	t.Run("Should reject a late batch after with changes and repoll null state [IT-011]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		h.writeReply(t, source.Result{Items: []source.Item{}, State: []byte(`{"cursor":9}`)})
		h.tick(t, 1)
		h.advance(t)
		h.writeReply(
			t,
			source.Result{Items: []source.Item{{Key: "1", Title: "old params"}}, State: []byte(`{"cursor":10}`)},
		)
		release := h.blockPoll(t)
		if err := h.checks.Tick(t.Context()); err != nil {
			t.Fatal(err)
		}
		waitForCondition(t, "old params poll started", func() bool { return len(h.inputs(t)) == 2 })
		before := h.states(t)[0]
		h.writePolicy(t, `{"project":"new"}`)
		target, err := h.env.ResolveCheck(t.Context(), h.instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		reset, err := h.db.Instances().
			SourceStates(t.Context(), h.instance.ID, target.Policy.Preferences.Context.Sources, false, h.now())
		if err != nil {
			t.Fatal(err)
		}
		if reset[0].Generation <= before.Generation || len(reset[0].State) != 0 {
			t.Fatalf("reset=%+v before=%+v", reset, before)
		}
		release()
		waitForCondition(
			t,
			"late batch discarded",
			func() bool { return h.checks.SourceMetrics()["heartbeat_source_collections_inflight"] == 0 },
		)
		h.waitIdle(t, 2)
		var reason string
		if err := h.db.DB().QueryRowContext(t.Context(),
			"SELECT reason FROM agent_heartbeat_wake_events WHERE instance_id=? ORDER BY created_at DESC LIMIT 1",
			h.instance.ID).Scan(&reason); err != nil {
			t.Fatal(err)
		}
		if reason != "heartbeat_invalid" {
			t.Fatalf("late batch reason = %q, want heartbeat_invalid", reason)
		}
		after := h.states(t)[0]
		if after.Generation != reset[0].Generation || len(after.State) != 0 {
			t.Fatalf("late batch committed: %+v", after)
		}
		result, err := h.checks.Check(t.Context(), h.instance.ID, true)
		if err != nil || result.Gate != "would_run" {
			t.Fatalf("next=%+v %v", result, err)
		}
		inputs := h.inputs(t)
		if len(inputs) != 3 || string(inputs[2].State) != "null" || !strings.Contains(string(inputs[2].Params), "new") {
			t.Fatalf("next stdin=%+v", inputs)
		}
	})
}

// Invariant (IT-008/013): preview never mutates the source rows, and an
// unapproved script publishes one transition notice before operator approval.
// Owning suite: daemon heartbeat runtime; CLI transport is covered in cli.
func TestHeartbeatSourceOperatorRuntime(t *testing.T) {
	t.Run("Should return identical HTTP previews without writes when active or paused [IT-008]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		h.writeReply(t, source.Result{Items: []source.Item{{Key: "1", Title: "one"}}, State: []byte(`{"cursor":1}`)})
		handlers := &core.BaseHandlers{
			AgentInstances: &core.AgentInstanceAPI{Store: h.db.Instances(), Checks: h.checks},
		}
		router := gin.New()
		router.POST("/api/agent-instances/:id/check", handlers.CheckAgentInstance)
		for _, paused := range []bool{false, true} {
			if paused {
				if _, err := h.db.Instances().Pause(t.Context(), h.instance.ID, "manual"); err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequestWithContext(
					t.Context(),
					http.MethodPost,
					"/api/agent-instances/"+h.instance.ID+"/check",
					strings.NewReader(`{"dry_run":true}`),
				)
				request.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(recorder, request)
				var result contract.AgentInstanceCheckResponse
				if err := json.Unmarshal(
					recorder.Body.Bytes(),
					&result,
				); err != nil || recorder.Code != http.StatusOK || len(result.Sources) != 1 ||
					result.Sources[0].Items != 1 {
					t.Fatalf("preview=%d %s %v", recorder.Code, recorder.Body, err)
				}
			}
		}
		var rows int
		if err := h.db.DB().
			QueryRowContext(t.Context(), "SELECT count(*) FROM agent_instance_source_states WHERE instance_id=?", h.instance.ID).
			Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if rows != 0 {
			t.Fatalf("preview wrote %d rows", rows)
		}
		inputs := h.inputs(t)
		if len(inputs) != 4 {
			t.Fatalf("inputs=%+v", inputs)
		}
		for _, input := range inputs {
			if !input.DryRun || string(input.State) != "null" {
				t.Fatalf("preview input=%+v", input)
			}
		}
	})
	t.Run("Should notify once then execute the approved source [IT-013]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, false, false)
		events, unsubscribe, err := h.manager.SubscribeSessionCatalogEvents(
			t.Context(),
			session.CatalogScope{
				ReadScope:   store.ReadScope{ProfileID: h.instance.ProfileID},
				WorkspaceID: h.instance.WorkspaceID,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		defer unsubscribe()
		for i := 1; i <= 2; i++ {
			h.tick(t, i)
			h.advance(t)
		}
		notices := 0
		draining := true
		for draining {
			select {
			case event := <-events:
				if event.OperatorNotification != nil &&
					event.OperatorNotification.Kind == session.NotifyKindHeartbeatSourceUnapproved {
					notices++
				}
			default:
				draining = false
			}
		}
		if notices != 1 || len(h.inputs(t)) != 0 {
			t.Fatalf("notices=%d inputs=%+v", notices, h.inputs(t))
		}
		h.approve(t)
		h.writeReply(t, source.Result{Items: []source.Item{}, State: []byte(`{"cursor":1}`)})
		h.tick(t, 3)
		if got := string(h.states(t)[0].State); got != `{"cursor":1}` || len(h.inputs(t)) != 1 {
			t.Fatalf("approved state=%s inputs=%+v", got, h.inputs(t))
		}
	})
}

// Invariant (IT-004/006): interrupted triage discards pending cursors, while a
// persistent source failure reports once and disappears from the digest on recovery.
// Owning layer: daemon heartbeat runtime and its real SQLite recovery transactions.
func TestHeartbeatSourceRecovery(t *testing.T) {
	t.Run("Should keep policy and check settings readable with an invalid runtime", func(t *testing.T) {
		t.Parallel()
		h := newHeartbeatSourceRuntime(t, true, false)
		original := h.env.definitions.agents
		h.env.definitions.agents = stubSessionCommandAgentResolver(
			func(name string, workspace *workspacepkg.ResolvedWorkspace) (compozyconfig.AgentDef, error) {
				agent, err := original.ResolveAgent(name, workspace)
				agent.Provider = "missing-provider"
				return agent, err
			},
		)
		target, err := h.env.ResolveCheck(t.Context(), h.instance.ID)
		if err != nil || target.Gate != "" || target.TriageProvider != "" || !target.Policy.Enabled {
			t.Fatalf("policy=%+v error=%v", target, err)
		}
		if _, err := h.checks.SetChecks(t.Context(), h.instance.ID, true); err != nil {
			t.Fatal(err)
		}
	})

	for _, mode := range []string{
		"config", "agent", "live", "other_workspace", "other_profile", "stale", "unknown", "other_model",
	} {
		t.Run("Should recover rejected configuration through "+mode, func(t *testing.T) {
			t.Parallel()
			h := newHeartbeatSourceRuntime(t, true, false)
			repo, now := h.db.Instances(), h.now()
			if admitted, err := repo.Admit(t.Context(), heartbeat.AdmitRequest{
				InstanceID: h.instance.ID, OccurrenceID: "config-rejected", ManualCheck: true,
				Now: now, MaxConcurrent: 1,
			}); err != nil || !admitted {
				t.Fatalf("admit=%v error=%v", admitted, err)
			}
			if result, err := repo.Finish(t.Context(), heartbeat.FinishRequest{
				InstanceID: h.instance.ID, OccurrenceID: "config-rejected", Source: "timer",
				Result: "failed", Reason: heartbeat.ReasonTriageConfigUnavailable,
				Interval: time.Hour, MaxInterval: 24 * time.Hour, Now: now, ExpiresAt: now.Add(48 * time.Hour),
			}); err != nil || !result.Changed {
				t.Fatalf("finish=%+v error=%v", result, err)
			}
			before, err := repo.WakeState(t.Context(), h.instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			previous := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
			state := &bootState{cfg: previous, logger: slog.Default(),
				runtimeWorkers: daemonRuntimeWorkers{proactiveChecks: h.checks}}
			switch mode {
			case "config":
				next := previous
				next.Agents.Heartbeat.TriageModels = map[string]string{acpmock.ProviderName: "fixed-model"}
				applier := daemonSettingsRuntimeApplier{daemon: &Daemon{config: previous}, state: state}
				if failures := applier.ApplyActiveConfig(t.Context(), &next); len(failures) != 0 {
					t.Fatalf("apply failures=%+v", failures)
				}
			case "agent":
				state.agentCatalog = newResourceCatalog(cloneAgentDef)
				bindInstanceCatalogReconciliation(t.Context(), state, func(context.Context) error { return nil })
				state.agentCatalog.Replace(1, nil)
			default:
				target, err := h.env.ResolveCheck(t.Context(), h.instance.ID)
				if err != nil {
					t.Fatal(err)
				}
				model := modelcatalog.Model{ProviderID: target.TriageProvider, ModelID: target.TriageModel,
					AvailabilityState: modelcatalog.AvailabilityStateAvailableLive}
				scope := modelcatalog.CatalogExecutionContext{Scope: modelcatalog.ExecutionScopeWorkspace,
					ProfileID: h.instance.ProfileID, WorkspaceID: h.instance.WorkspaceID}
				switch mode {
				case "other_workspace":
					scope.WorkspaceID = "other"
				case "other_profile":
					scope.ProfileID = "other"
				case "stale":
					model.Stale = true
				case "unknown":
					model.AvailabilityState = modelcatalog.AvailabilityStateUnknown
				case "other_model":
					model.ModelID = "other"
				}
				catalog, err := newModelCatalogRuntime(
					t.Context(),
					&recordingModelCatalogService{models: []modelcatalog.Model{model}},
					slog.Default(), h.now, time.Second,
				)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := catalog.Shutdown(context.Background()); err != nil {
						t.Error(err)
					}
				})
				state.modelCatalog = catalog
				bindCheckConfigRecovery(state, h.checks)
				if _, err := catalog.Refresh(t.Context(), modelcatalog.RefreshOptions{
					ProviderID: target.TriageProvider, ExecutionContext: scope,
				}); err != nil {
					t.Fatal(err)
				}
			}
			after, err := repo.WakeState(t.Context(), h.instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			wantRecovery := mode == "config" || mode == "agent" || mode == "live"
			if !wantRecovery {
				if after.NextWakeAt != before.NextWakeAt {
					t.Fatalf("unmatched recovery changed deadline: before=%+v after=%+v", before, after)
				}
				return
			}
			target, err := h.env.ResolveCheck(t.Context(), h.instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			cadence := target.Policy.Preferences.MinInterval
			if cadence <= 0 {
				cadence = h.config.Interval
			}
			if after.NextWakeAt != store.FormatTimestamp(now.Add(cadence)) {
				t.Fatalf("before=%+v after=%+v want cadence=%v", before, after, cadence)
			}
		})
	}

	t.Run("Should recover an interrupted bound triage and repeat its input [IT-004]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, true)
		h.writeReply(t, source.Result{Items: []source.Item{}, State: []byte(`{"cursor":7}`)})
		h.tick(t, 1)
		h.advance(t)
		if err := h.checks.Shutdown(t.Context()); err != nil {
			t.Fatal(err)
		}
		h.writeReply(
			t,
			source.Result{
				Items: []source.Item{{Key: "1", Title: "retry after restart"}},
				State: []byte(`{"cursor":1}`),
			},
		)
		target, err := h.env.ResolveCheck(t.Context(), h.instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		batch, err := h.env.PollSources(t.Context(), &target, false)
		if err != nil {
			t.Fatal(err)
		}
		admitted, err := h.db.Instances().
			Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: h.instance.ID, OccurrenceID: "instance_check-before-restart", ManualCheck: true, Now: h.now(), NextWakeAt: h.now().Add(time.Hour), MaxConcurrent: 1, Sources: batch.Sources})
		if err != nil || !admitted {
			t.Fatalf("admit=%v %v", admitted, err)
		}
		triage, err := h.manager.Create(
			t.Context(),
			&session.CreateOpts{
				AgentName:                 h.instance.AgentName,
				Workspace:                 h.instance.WorkspaceID,
				ProfileID:                 h.instance.ProfileID,
				InstanceID:                h.instance.ID,
				Type:                      session.SessionTypeTriage,
				ProvenanceParentSessionID: h.instance.MainSessionID,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		bound, err := h.db.Instances().
			BindTriage(t.Context(), h.instance.ID, "instance_check-before-restart", triage.ID)
		if err != nil || !bound {
			t.Fatalf("bind=%v %v", bound, err)
		}
		promptCtx, cancelPrompt := context.WithCancel(t.Context())
		defer cancelPrompt()
		events, err := h.manager.Prompt(promptCtx, triage.ID, "Perform a proactive check.")
		if err != nil {
			t.Fatal(err)
		}
		drained := make(chan struct{})
		go func() {
			defer close(drained)
			for range events {
			}
		}()
		waitForCondition(t, "triage provider received pending check", func() bool {
			records, err := acpmock.ReadDiagnostics(h.diagnostics)
			if err != nil {
				return false
			}
			for _, record := range acpmock.ProtocolDiagnostics(records) {
				if record.ProtocolMethod == "session/prompt" {
					return true
				}
			}
			return false
		})
		// A fresh service owns no in-memory run; it must recover the persisted occurrence.
		h.startChecks(t)
		select {
		case <-drained:
		case <-time.After(5 * time.Second):
			t.Fatal("recovered triage did not drain")
		}
		var reason string
		var pending *string
		if err := h.db.DB().
			QueryRowContext(t.Context(), "SELECT reason FROM agent_heartbeat_wake_events WHERE instance_id=? ORDER BY created_at DESC LIMIT 1", h.instance.ID).
			Scan(&reason); err != nil {
			t.Fatal(err)
		}
		if err := h.db.DB().
			QueryRowContext(t.Context(), "SELECT pending_occurrence_id FROM agent_instance_source_states WHERE instance_id=?", h.instance.ID).
			Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if reason != "triage_interrupted" || pending != nil || string(h.states(t)[0].State) != `{"cursor":7}` {
			t.Fatalf("reason=%s pending=%v states=%+v", reason, pending, h.states(t))
		}
		result, err := h.checks.Check(t.Context(), h.instance.ID, true)
		if err != nil || result.Digest == nil || len(result.Digest.Items) != 1 ||
			result.Digest.Items[0].Key != "source:s:1" {
			t.Fatalf("retry=%+v %v", result, err)
		}
		inputs := h.inputs(t)
		if len(inputs) != 3 || string(inputs[len(inputs)-1].State) != `{"cursor":7}` {
			t.Fatalf("recovered stdin=%+v", inputs)
		}
	})
	t.Run("Should report the failure threshold once and clear it on success [IT-006]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		if err := os.Remove(h.reply); err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= 4; i++ {
			h.tick(t, i)
			h.advance(t)
		}
		states := h.states(t)
		if states[0].FailureStreak != 4 || states[0].Error == nil || states[0].Error.Code != "source_exit_nonzero" {
			t.Fatalf("failing=%+v", states)
		}
		var unchanged int
		if err := h.db.DB().
			QueryRowContext(t.Context(), "SELECT count(*) FROM agent_heartbeat_wake_events WHERE instance_id=? AND reason='digest_unchanged'", h.instance.ID).
			Scan(&unchanged); err != nil {
			t.Fatal(err)
		}
		records, err := acpmock.ReadDiagnostics(h.diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		prompts := acpmock.PromptDiagnostics(records)
		data, err := json.Marshal(prompts)
		if err != nil {
			t.Fatal(err)
		}
		if unchanged != 1 || len(prompts) != 1 ||
			!strings.Contains(string(data), "source:s:failing:source_exit_nonzero") {
			t.Fatalf("unchanged=%d prompts=%s", unchanged, data)
		}
		h.writeReply(t, source.Result{Items: []source.Item{}})
		h.tick(t, 5)
		statuses, err := h.checks.SourceStatuses(t.Context(), h.instance.ID)
		if err != nil || len(statuses) != 1 || statuses[0].Status != "ok" || statuses[0].FailureStreak != 0 {
			t.Fatalf("statuses=%+v %v", statuses, err)
		}
		result, err := h.checks.Check(t.Context(), h.instance.ID, true)
		if err != nil || result.Gate != "digest_empty" {
			t.Fatalf("recovered digest=%+v %v", result, err)
		}
	})
}

// Invariant (IT-010): a blocked source cannot delay task-run wakes in the same
// mechanical scheduler cycle. Fast-source admission is owned by the globaldb
// TestSourceCollectionScheduling suite; this test owns daemon scheduler wiring.
func TestHeartbeatSourceSchedulerCycle(t *testing.T) {
	t.Run("Should dispatch queued task work while a source poll remains blocked [IT-010]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		release := h.blockPoll(t)
		work := daemonTaskRecordForTest("source-cycle-task", h.now())
		work.Scope = taskpkg.ScopeWorkspace
		work.WorkspaceID = h.instance.WorkspaceID
		if err := h.db.CreateTask(t.Context(), work); err != nil {
			t.Fatal(err)
		}
		enqueueSchedulerRunForTest(
			t,
			h.db,
			h.now(),
			work.ID,
			"source-cycle-run",
			taskpkg.RunKindWorker,
			"",
			"source.cycle",
		)
		sessions := &fakeSessionManager{
			infos: []*session.Info{
				{
					ID:          "task-worker",
					ProfileID:   h.instance.ProfileID,
					AgentName:   "task-worker",
					WorkspaceID: h.instance.WorkspaceID,
					State:       session.StateActive,
					CreatedAt:   h.now().Add(-time.Hour),
				},
			},
		}
		waker, err := newSchedulerSessionWaker(t.Context(), sessions, discardLogger())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := waker.shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		tasks, err := taskpkg.NewManager(taskpkg.WithStore(h.db), taskpkg.WithManagerNow(h.now))
		if err != nil {
			t.Fatal(err)
		}
		scheduler, err := schedulerpkg.New(
			schedulerTaskSource{manager: tasks, store: h.db},
			schedulerSessionSource{
				sessions:  sessions,
				situation: &schedulerSituationContextStub{},
				logger:    discardLogger(),
			},
			waker,
			schedulerpkg.WithClock(clockwork.NewFakeClockAt(h.now())),
			schedulerpkg.WithWakeCooldown(0),
			schedulerpkg.WithProactiveChecks(h.checks.Tick),
			schedulerpkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		cycle, err := scheduler.RunOnce(t.Context())
		if err != nil || time.Since(started) >= time.Second || cycle.WakeSucceeded != 1 ||
			sessions.syntheticPromptCount() != 1 {
			t.Fatalf("cycle=%+v duration=%v err=%v", cycle, time.Since(started), err)
		}
		waitForCondition(t, "source still blocked after task dispatch", func() bool { return len(h.inputs(t)) == 1 })
		if h.checks.SourceMetrics()["heartbeat_source_collections_inflight"] != 1 {
			t.Fatal("source poll finished before task wake assertion")
		}
		release()
		h.waitIdle(t, 1)
	})
}

type heartbeatFixturePoller struct{}

var _ source.ExtensionPoller = heartbeatFixturePoller{}

func (heartbeatFixturePoller) SourceKinds(context.Context, source.CatalogScope) ([]source.KindDescriptor, error) {
	return []source.KindDescriptor{{Kind: "fake/issues"}}, nil
}

func (heartbeatFixturePoller) PollSource(
	_ context.Context,
	_ source.CatalogScope,
	_ string,
	req source.Request,
) (source.Result, error) {
	if string(req.Params) != `{"repo":"a/b"}` {
		return source.Result{}, source.Failure("source_params_invalid", "repo is required")
	}
	return source.Result{Items: []source.Item{{Key: "issue:1", Title: "New issue"}}, State: []byte(`{"cursor":1}`)}, nil
}
func TestHeartbeatExtensionSourceRuntime(t *testing.T) {
	t.Run("Should share individual poll limits across simultaneous collections", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		h.env.sourceConfig.MaxConcurrentPolls = 2
		poller := &boundedHeartbeatPoller{}
		h.env.sourceExtensions = poller
		target, err := h.env.ResolveCheck(t.Context(), h.instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		target.Policy.Preferences.Context.Sources = nil
		for i := range 8 {
			target.Policy.Preferences.Context.Sources = append(
				target.Policy.Preferences.Context.Sources,
				source.Ref{ID: fmt.Sprintf("s%d", i), Kind: "fake/issues"},
			)
		}
		done := make(chan error, 2)
		for range 2 {
			go func() { _, err := h.env.PollSources(t.Context(), &target, true); done <- err }()
		}
		for range 2 {
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		}
		if peak := poller.peak.Load(); peak > 2 || poller.calls.Load() != 16 {
			t.Fatalf("peak=%d calls=%d", peak, poller.calls.Load())
		}
	})
	t.Run("Should collect extension items and preserve parameter failures [IT-005]", func(t *testing.T) {
		t.Parallel()
		h := newHeartbeatSourceRuntime(t, true, false)
		h.env.sourceExtensions = heartbeatFixturePoller{}
		body := "---\nversion: 1\nenabled: true\ncontext:\n  include: []\n  sources:\n    - id: incoming\n      kind: fake/issues\n      with: {repo: a/b}\n---\nReview incoming issues.\n"
		if err := os.WriteFile(filepath.Join(h.dir, "HEARTBEAT.md"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		target, err := h.env.ResolveCheck(t.Context(), h.instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		batch, err := h.env.PollSources(t.Context(), &target, true)
		if err != nil || len(batch.Sources) != 1 || batch.Sources[0].Err != nil ||
			len(batch.Sources[0].Result.Items) != 1 {
			t.Fatalf("batch=%+v err=%v", batch, err)
		}
		digest := batch.DigestItems(3)
		if len(digest) != 1 || digest[0].Source.SourceRef != "fake/issues" {
			t.Fatalf("digest=%+v", digest)
		}
		target.Policy.Preferences.Context.Sources[0].With = []byte(`{}`)
		batch, err = h.env.PollSources(t.Context(), &target, true)
		if err != nil || len(batch.Sources) != 1 || batch.Sources[0].Err == nil ||
			batch.Sources[0].Err.Code != "source_params_invalid" {
			t.Fatalf("batch=%+v err=%v", batch, err)
		}
	})
}

func TestNativeHeartbeatSourcesCatalog(t *testing.T) {
	t.Run("Should return the shared catalog JSON [UT-060]", func(t *testing.T) {
		t.Parallel()
		h := newHeartbeatSourceRuntime(t, true, false)
		resolver := apitest.StubWorkspaceService{
			ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					ID:          "ws",
					WorkspaceID: "ws",
					RootDir:     h.dir,
					ProfileID:   store.DefaultProfileID,
					Config: compozyconfig.Config{
						Agents: compozyconfig.AgentsConfig{Heartbeat: compozyconfig.DefaultHeartbeatConfig()},
					},
					Agents: []compozyconfig.AgentDef{
						{Name: h.instance.AgentName, SourcePath: filepath.Join(h.dir, "AGENT.md")},
					},
				}, nil
			},
		}
		api := &core.AgentInstanceAPI{Store: h.db.Instances(), SourceExtensions: heartbeatFixturePoller{}}
		native := &daemonNativeTools{
			deps: &daemonNativeToolsDeps{
				WorkspaceResolver: resolver,
				AgentInstances:    func() *core.AgentInstanceAPI { return api },
			},
		}
		input, err := json.Marshal(map[string]string{"agent": h.instance.AgentName, "workspace": "ws"})
		if err != nil {
			t.Fatal(err)
		}
		result, err := native.agentHeartbeatSources(
			t.Context(),
			tools.Scope{Operator: true, ProfileID: store.DefaultProfileID},
			tools.CallRequest{ToolID: tools.ToolIDAgentHeartbeatSources, Input: input, View: tools.ResultViewFull},
		)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := core.HeartbeatSourceCatalog(
			t.Context(),
			h.instance.AgentName,
			h.dir,
			false,
			source.CatalogScope{ProfileID: store.DefaultProfileID, WorkspaceID: "ws"},
			compozyconfig.DefaultHeartbeatSourcesConfig(),
			h.db.Instances(),
			heartbeatFixturePoller{},
		)
		if err != nil {
			t.Fatal(err)
		}
		var actual contract.HeartbeatSourceCatalogPayload
		if err := json.Unmarshal(result.Structured, &actual); err != nil {
			t.Fatal(err)
		}
		actualJSON, err := json.Marshal(actual)
		if err != nil {
			t.Fatal(err)
		}
		expectedJSON, err := json.Marshal(expected)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(actualJSON, expectedJSON) {
			t.Fatalf("catalog=%s expected=%s", actualJSON, expectedJSON)
		}
	})
}

type countingInstancePolicyResolver struct {
	session.AgentResolver
	reads atomic.Int64
}

func (r *countingInstancePolicyResolver) ResolveHeartbeatPolicy(
	_ context.Context, _ heartbeat.AuthoringTarget,
) (heartbeat.ResolvedPolicy, bool, error) {
	r.reads.Add(1)
	return heartbeat.ResolvedPolicy{}, false, nil
}

// artifactInstancePolicyResolver exercises the catalog artifact branch with real local files.
type artifactInstancePolicyResolver struct {
	session.AgentResolver
	packageOwned bool
	body         string
}

func (r *artifactInstancePolicyResolver) ResolveAgentArtifacts(
	name string, workspace *workspacepkg.ResolvedWorkspace,
) (session.AgentArtifacts, error) {
	agent, err := r.ResolveAgent(name, workspace)
	path := filepath.Join(filepath.Dir(agent.SourcePath), "HEARTBEAT.md")
	if r.packageOwned {
		path = ".compozy/bundles/source-agent/HEARTBEAT.md"
	}
	return session.AgentArtifacts{
		Agent: agent, PackageOwned: r.packageOwned,
		HeartbeatSourcePath: path,
		HeartbeatBody:       r.body,
	}, err
}

type boundedHeartbeatPoller struct{ active, peak, calls atomic.Int32 }

var _ source.ExtensionPoller = (*boundedHeartbeatPoller)(nil)

func (*boundedHeartbeatPoller) SourceKinds(context.Context, source.CatalogScope) ([]source.KindDescriptor, error) {
	return nil, nil
}

func (p *boundedHeartbeatPoller) PollSource(
	ctx context.Context,
	_ source.CatalogScope,
	_ string,
	_ source.Request,
) (source.Result, error) {
	active := p.active.Add(1)
	defer p.active.Add(-1)
	p.calls.Add(1)
	for peak := p.peak.Load(); active > peak; peak = p.peak.Load() {
		if p.peak.CompareAndSwap(peak, active) {
			break
		}
	}
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return source.Result{Items: []source.Item{}}, nil
	case <-ctx.Done():
		return source.Result{}, ctx.Err()
	}
}

type postPollHeartbeatEnvironment struct {
	*instanceCheckEnvironment
	mode   string
	polled bool
}

func (e *postPollHeartbeatEnvironment) PollSources(
	ctx context.Context,
	target *heartbeat.CheckTarget,
	dry bool,
) (heartbeat.PollBatch, error) {
	batch, err := e.instanceCheckEnvironment.PollSources(ctx, target, dry)
	e.polled = true
	return batch, err
}
func (e *postPollHeartbeatEnvironment) ResolveCheck(ctx context.Context, id string) (heartbeat.CheckTarget, error) {
	target, err := e.instanceCheckEnvironment.ResolveCheck(ctx, id)
	if err != nil || !e.polled {
		return target, err
	}
	switch e.mode {
	case "session_prompt_active":
		target.MainPrompting = true
	case "heartbeat_invalid":
		target.Policy.Digest += "changed"
	case "triage_failed":
		return target, errors.New("policy unavailable")
	}
	return target, nil
}

func TestCheckInToolsDisabled(t *testing.T) {
	t.Run("Should complete a real check-in without hosted tools when disabled [IT-008]", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		session.WithCheckInTools(false)(h.manager)
		h.writeReply(
			t,
			source.Result{Items: []source.Item{{Key: "1", Title: "Fixture input"}}, State: []byte(`{"cursor":1}`)},
		)
		h.tick(t, 1)
		rows, err := h.db.Instances().ListCheckIns(t.Context(), h.instance.ID, "", 10)
		if err != nil || len(rows) != 1 || rows[0].ToolsStatus != "disabled" || len(rows[0].ToolCalls) != 0 {
			t.Fatalf("rows=%+v err=%v", rows, err)
		}
	})
}

type checkInGatewayFixture struct {
	inner heartbeat.TriageDispatcher
	call  func(context.Context, *heartbeat.CheckTarget, string) error
}

var _ heartbeat.TriageDispatcher = checkInGatewayFixture{}

func (f checkInGatewayFixture) Stop(ctx context.Context, id string) error {
	return f.inner.Stop(ctx, id)
}

func (f checkInGatewayFixture) Dispatch(
	ctx context.Context,
	target *heartbeat.CheckTarget,
	digest heartbeat.CheckDigest,
	bind func(string) error,
) (heartbeat.TriageOutcome, error) {
	return f.inner.Dispatch(ctx, target, digest, func(id string) error {
		if err := bind(id); err != nil {
			return err
		}
		return f.call(ctx, target, id)
	})
}
func TestCheckInToolAuthorityIntegration(t *testing.T) {
	t.Run("Should retry an authority notice after publication fails without skipping the check", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		target, err := h.env.ResolveCheck(t.Context(), h.instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		target.Policy.Frontmatter.CheckIns.AllowWrites = []string{"compozy__notify"}
		digest, err := heartbeat.AuthorityDigest(target.Policy.Frontmatter.CheckIns, nil)
		if err != nil {
			t.Fatal(err)
		}
		env := &instanceCheckEnvironment{instances: h.db.Instances()}
		if err := env.notifyCheckInAuthority(t.Context(), &target); err == nil {
			t.Fatal("expected publication failure")
		}
		sent, err := h.db.Instances().
			AuthorityNoticeSent(t.Context(), target.Instance.ProfileID, target.DefinitionDir, digest)
		if err != nil || sent {
			t.Fatalf("failed notice was marked: %v %v", sent, err)
		}
		reason, err := env.BeforeCheck(t.Context(), &target, "timer")
		if err != nil || reason != "" {
			t.Fatalf("notice failure skipped check: %s %v", reason, err)
		}
	})

	for _, mode := range []string{"approved", "narrowed", "no-main", "semantic-error", "override-unapproved", "override-invalid", "override-malformed"} {
		t.Run("Should enforce current main authority and project MCP audit for "+mode+" [IT-006]", func(t *testing.T) {
			permission := compozyconfig.PermissionModeApproveAll
			if mode == "narrowed" {
				permission = compozyconfig.PermissionModeApproveReads
			}
			h := newHeartbeatSourceRuntime(t, true, false, permission)
			if err := h.checks.Shutdown(t.Context()); err != nil {
				t.Fatal(err)
			}
			remoteFixture := mcpfixture.MustNew(mcpfixture.ProfileModern2026)
			remoteFixture.Server().
				AddTool(&sdkmcp.Tool{Name: "read", InputSchema: map[string]any{"type": "object"}, Annotations: &sdkmcp.ToolAnnotations{ReadOnlyHint: true}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
					return &sdkmcp.CallToolResult{
						Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "read fixture"}},
					}, nil
				})
			remoteFixture.Server().
				AddTool(&sdkmcp.Tool{Name: "unannotated", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
					return &sdkmcp.CallToolResult{}, nil
				})
			if mode == "semantic-error" {
				remoteFixture.Server().
					AddTool(&sdkmcp.Tool{Name: "echo", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
						return &sdkmcp.CallToolResult{
							IsError: true,
							Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "failed"}},
						}, nil
					})
			}
			remote := remoteFixture.StartHTTP(t)
			overrideNames := []string(nil)
			if strings.HasPrefix(mode, "override-") {
				overrideNames = []string{"unannotated"}
				if mode == "override-malformed" {
					overrideNames = append(overrideNames, "invalid name")
				}
			}
			executor, err := mcppkg.NewMCPCallExecutor(
				mcppkg.ServerResolverFunc(
					func(context.Context, tools.SourceRef, string) (mcppkg.ResolvedServer, error) {
						return mcppkg.ResolvedServer{
							Target:                 mcpauth.Target{Scope: mcpauth.ScopeUser, ServerName: "fixture"},
							ReadOnlyOverrideOrigin: new(tools.MCPReadOnlyOverrideAgentDefinition),
							Server: compozyconfig.MCPServer{
								Name:          "fixture",
								ReadOnlyTools: overrideNames,
								Transport:     compozyconfig.MCPServerTransportHTTP,
								URL:           remote.URL,
							},
						}, nil
					},
				),
				mcppkg.WithHTTPClient(remote.Client()),
			)
			if err != nil {
				t.Fatal(err)
			}
			mcpSource := tools.SourceRef{
				Kind:          tools.SourceMCP,
				Owner:         "fixture",
				RawServerName: "fixture",
				RawToolName:   "*",
			}
			if _, err := executor.ListTools(t.Context(), mcpSource); err != nil {
				t.Fatalf("fixture discovery: %v", err)
			}
			provider, err := tools.NewMCPProvider(
				tools.MCPSourceListerFunc(
					func(context.Context) ([]tools.SourceRef, error) { return []tools.SourceRef{mcpSource}, nil },
				),
				executor,
				executor,
			)
			if err != nil {
				t.Fatal(err)
			}
			workspace, err := h.env.definitions.workspaces.Resolve(t.Context(), h.instance.WorkspaceID)
			if err != nil {
				t.Fatal(err)
			}
			def, err := h.env.definitions.agents.ResolveAgent(h.instance.AgentName, &workspace)
			if err != nil {
				t.Fatal(err)
			}
			def.Tools = []string{"mcp__fixture__read", "mcp__fixture__echo", "mcp__fixture__unannotated"}
			def.MCPServers = []compozyconfig.MCPServer{
				{Name: "fixture", Transport: compozyconfig.MCPServerTransportHTTP, URL: remote.URL},
			}
			agents := stubSessionCommandAgentResolver(
				func(string, *workspacepkg.ResolvedWorkspace) (compozyconfig.AgentDef, error) { return def, nil },
			)
			session.WithAgentResolver(agents)(h.manager)
			session.WithToolUniverse(
				[]tools.ToolID{"mcp__fixture__read", "mcp__fixture__echo", "mcp__fixture__unannotated"},
			)(
				h.manager,
			)
			h.env.definitions.agents = agents
			h.manager.SetInstanceCeilingReader(instanceCeilingReader(h.db.Instances()))
			ceiling, err := json.Marshal(
				// Admission records MCP servers by name and never their tool IDs or materialized flags.
				store.SessionPermissionPolicy{
					MCPServers:     []string{"fixture"},
					WorkspacePaths: []string{workspace.RootDir, h.dir},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := h.db.DB().
				ExecContext(t.Context(), "UPDATE agent_instances SET permission_ceiling=? WHERE id=?", string(ceiling), h.instance.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := h.db.DB().
				ExecContext(t.Context(), "UPDATE sessions SET permission_policy_json=? WHERE id=?", string(ceiling), h.instance.MainSessionID); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(filepath.Join(h.dir, "HEARTBEAT.md"))
			if err != nil {
				t.Fatal(err)
			}
			body = bytes.Replace(
				body,
				[]byte("version: 1"),
				[]byte("version: 1\ncheck_ins:\n  allow_writes: [mcp__fixture__echo]"),
				1,
			)
			if err := os.WriteFile(filepath.Join(h.dir, "HEARTBEAT.md"), body, 0600); err != nil {
				t.Fatal(err)
			}
			hash, err := heartbeat.AuthorityDigest(
				heartbeat.CheckInsPolicy{AllowWrites: []string{"mcp__fixture__echo"}},
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := h.db.Instances().
				ApproveAuthority(t.Context(), h.instance.ProfileID, h.dir, hash, "cli", h.now()); err != nil {
				t.Fatal(err)
			}
			if mode == "no-main" {
				if _, err := h.db.DB().
					ExecContext(t.Context(), "UPDATE agent_instances SET main_session_id=NULL WHERE id=?", h.instance.ID); err != nil {
					t.Fatal(err)
				}
			}
			if strings.HasPrefix(mode, "override-") {
				def.MCPServers[0].ReadOnlyTools = overrideNames
			}
			if mode == "override-invalid" {
				if err := os.WriteFile(
					filepath.Join(h.dir, "HEARTBEAT.md"),
					[]byte("---\ncheck_ins: invalid\n---\n"),
					0600,
				); err != nil {
					t.Fatal(err)
				}
			}
			resolver, err := newNativeToolPolicyResolver(
				nativeToolPolicyResolverDeps{
					Config:            &workspace.Config,
					Sessions:          h.manager,
					WorkspaceResolver: h.env.definitions.workspaces,
					ProfileNames:      nativeProfileReaderStub{},
					AgentResolver:     &nativeToolPolicyAgentResolverStub{agent: def},
					MCPSourceGrants: func(context.Context, tools.Scope) ([]tools.SourceGrant, error) {
						return []tools.SourceGrant{{Kind: tools.SourceMCP, Owner: "fixture"}}, nil
					},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			resolver.instanceCallers = h.db.Instances()

			registry, err := tools.NewRegistry(
				tools.WithProviders(provider),
				tools.WithPolicyInputResolver(resolver, tools.ToolsetCatalog{}),
				tools.WithCheckInRecorder(h.db.Instances().CheckInRecorder()),
			)
			if err != nil {
				t.Fatal(err)
			}

			if strings.HasPrefix(mode, "override-") {
				views, err := registry.DiagnosticProjection(
					t.Context(),
					tools.Scope{
						SessionID:   h.instance.MainSessionID,
						ProfileID:   h.instance.ProfileID,
						WorkspaceID: h.instance.WorkspaceID,
						AgentName:   h.instance.AgentName,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, view := range views {
					if view.Descriptor.ID == "mcp__fixture__unannotated" {
						found = true
						if tools.AccessOf(view.Descriptor) != tools.AccessWrite {
							t.Fatalf("main override applied without approval: %+v", view)
						}
					}
				}
				if !found {
					t.Fatal("main tool missing")
				}
			}
			if mode == "override-invalid" || mode == "override-malformed" {
				result, err := registry.Call(
					t.Context(),
					tools.Scope{
						SessionID:   h.instance.MainSessionID,
						ProfileID:   h.instance.ProfileID,
						WorkspaceID: h.instance.WorkspaceID,
						AgentName:   h.instance.AgentName,
					},
					tools.CallRequest{ToolID: "mcp__fixture__read", Input: []byte(`{}`)},
				)
				if err != nil {
					t.Fatalf("main call with invalid authority: %+v %v", result, err)
				}
				if mode == "override-malformed" {
					return
				}
				repaired := []byte("---\nversion: 1\ncheck_ins:\n  max_tool_calls: 2\n---\n")
				if err := os.WriteFile(filepath.Join(h.dir, "HEARTBEAT.md"), repaired, 0600); err != nil {
					t.Fatal(err)
				}
				cfg := workspace.Config
				cfg.Agents.AlwaysOn.CheckInMaxToolCalls = 1
				info := &session.Info{
					AgentName:   h.instance.AgentName,
					ProfileID:   h.instance.ProfileID,
					WorkspaceID: h.instance.WorkspaceID,
				}
				policy := &tools.CheckInPolicy{}
				if err := resolver.applyCheckInAuthority(
					t.Context(),
					policy,
					info,
					nil,
					&cfg,
					h.db.Instances(),
				); err != nil ||
					policy.AuthorityApproved {
					t.Fatalf("lowered cap: %+v %v", policy, err)
				}
				cfg.Agents.AlwaysOn.CheckInMaxToolCalls = 2
				digest, err := heartbeat.AuthorityDigest(
					heartbeat.CheckInsPolicy{MaxToolCalls: new(2)},
					nil,
					heartbeat.AuthorityOverrides(def.MCPServers),
				)
				if err != nil {
					t.Fatal(err)
				}
				if err := h.db.Instances().
					ApproveAuthority(t.Context(), info.ProfileID, h.dir, digest, "cli", h.now()); err != nil {
					t.Fatal(err)
				}
				if err := resolver.applyCheckInAuthority(
					t.Context(),
					policy,
					info,
					nil,
					&cfg,
					h.db.Instances(),
				); err != nil ||
					!policy.AuthorityApproved {
					t.Fatalf("repaired authority: %+v %v", policy, err)
				}
				return
			}
			fixture := checkInGatewayFixture{
				inner: &session.TriageDispatcher{
					Manager:      h.manager,
					Reports:      h.db.InstanceReports(),
					MaxProposals: 2,
					StopTimeout:  time.Second,
				},
				call: func(ctx context.Context, target *heartbeat.CheckTarget, id string) error {
					scope := tools.Scope{
						SessionID:   id,
						ProfileID:   target.Instance.ProfileID,
						WorkspaceID: target.Instance.WorkspaceID,
						AgentName:   target.Instance.AgentName,
					}
					if _, err := registry.Call(
						ctx,
						scope,
						tools.CallRequest{ToolID: "mcp__fixture__read", Input: []byte(`{}`)},
					); (err == nil) != (mode != "no-main") {
						return fmt.Errorf("fixture read: %w", err)
					}
					if _, err := registry.Call(
						ctx,
						scope,
						tools.CallRequest{ToolID: "mcp__fixture__unannotated", Input: []byte(`{}`)},
					); err == nil {
						return errors.New("unannotated tool ran without opt-in")
					}
					_, err := registry.Call(
						ctx,
						scope,
						tools.CallRequest{
							ToolID: "mcp__fixture__echo",
							Input:  []byte(`{"message":"fixture write"}`),
						},
					)
					if (err == nil) != (mode == "approved" || mode == "semantic-error") {
						return fmt.Errorf("write at %s: %v", mode, err)
					}
					return nil
				},
			}
			h.checks, err = heartbeat.NewCheckService(t.Context(), h.db.Instances(), h.env, fixture, h.config, h.now)
			if err != nil {
				t.Fatal(err)
			}
			if err := h.checks.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
			checks := h.checks
			t.Cleanup(func() {
				if err := checks.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			h.writeReply(
				t,
				source.Result{Items: []source.Item{{Key: "1", Title: "Tool fixture"}}, State: []byte(`{"cursor":1}`)},
			)
			h.tick(t, 1)
			checksRows, err := h.db.Instances().ListCheckIns(t.Context(), h.instance.ID, "", 10)
			if err != nil || len(checksRows) != 1 || len(checksRows[0].ToolCalls) != 3 {
				t.Fatalf("checks=%+v err=%v", checksRows, err)
			}
			calls := checksRows[0].ToolCalls
			metrics, err := h.db.Instances().Metrics(t.Context(), h.instance.ProfileID, h.instance.WorkspaceID)
			if err != nil {
				t.Fatal(err)
			}
			var total int64
			for _, decisions := range metrics.CheckInToolCalls {
				for _, count := range decisions {
					total += count
				}
			}
			if total != 3 {
				t.Fatalf("tool-call metrics=%+v", metrics.CheckInToolCalls)
			}

			if mode == "semantic-error" && (calls[2].Outcome != "error" || calls[2].Effect != "maybe") {
				t.Fatalf("semantic MCP failure=%+v", calls[2])
			}
			if mode == "override-unapproved" && calls[2].DenyReason != "check_in_write_not_allowed" {
				t.Fatalf("unapproved override widened writes: %+v", calls[2])
			}

			if mode == "no-main" {
				if calls[0].DenyReason != "check_in_no_main_session" {
					t.Fatalf("read=%+v", calls[0])
				}
			} else if calls[0].Outcome != "ok" || calls[0].Access != "read" {
				t.Fatalf("read=%+v", calls[0])
			}
			if calls[1].Decision != "denied" || calls[1].Access != "write" || calls[1].Effect != "none" {
				t.Fatalf("unannotated=%+v", calls[1])
			}
			if mode != "approved" && mode != "semantic-error" && mode != "override-unapproved" &&
				(calls[2].Decision != "denied" || calls[2].DenyReason != map[bool]string{true: "check_in_no_main_session", false: "check_in_write_needs_approval"}[mode == "no-main"]) {
				t.Fatalf("denied write=%+v", calls[2])
			}
			reports, err := h.db.InstanceReports().
				ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: h.instance.ID, Limit: 10})
			if err != nil || len(reports) != 1 || !bytes.Contains(reports[0].Payload, []byte(`"tool_calls"`)) {
				t.Fatalf("reports=%+v err=%v", reports, err)
			}
		})
	}
}

// Invariant: committed auto picks use the real session queue, including a released main.
// Owning layer and canonical suite: daemon heartbeat wake runtime integration.
func TestAutoPickQueueDelivery(t *testing.T) {
	t.Run("Should keep durable publication unchanged when pick projections change", func(t *testing.T) {
		h := newHeartbeatSourceRuntime(t, true, false)
		report, err := h.db.InstanceReports().
			AppendReport(t.Context(), h.instance.ID, agentinstance.ReportInput{SourceKey: "occ:immutable", Kind: "proposal", Payload: []byte(`{"proposals":[{"title":"Review","body":"Evidence","item_keys":[]}]}`)})
		if err != nil {
			t.Fatal(err)
		}
		report.EventSessionID = h.instance.MainSessionID
		state := "queued"
		writer := instanceReportEventWriter{
			sessions: h.manager,
			project: func(_ context.Context, _ string, report *agentinstance.Report) error {
				payload, err := json.Marshal(
					map[string]any{
						"proposals": []map[string]any{
							{
								"title":              "Review",
								"body":               "Evidence",
								"item_keys":          []string{},
								"auto_accept_status": "picked",
							},
						},
						"picks": []map[string]any{{"actor": "auto_accept", "state": state}},
					},
				)
				report.Payload = payload
				return err
			},
		}
		first, err := writer.Publish(t.Context(), h.instance.MainSessionID, report)
		if err != nil {
			t.Fatal(err)
		}
		state = "delivered"
		second, err := writer.Publish(t.Context(), h.instance.MainSessionID, report)
		if err != nil || first != second {
			t.Fatalf("replay changed: first=%d second=%d err=%v", first, second, err)
		}
	})

	for _, released := range []bool{false, true} {
		t.Run(
			fmt.Sprintf("Should deliver one auto pick to released=%v main [IT-007 IT-010]", released),
			func(t *testing.T) {
				h := newHeartbeatSourceRuntime(t, true, false)
				if released {
					if err := h.manager.Stop(t.Context(), h.instance.MainSessionID); err != nil {
						t.Fatal(err)
					}
				}
				actions := []heartbeat.ActionDecl{
					{ID: "report", Title: "Report", Instructions: "Write the report.", AutoAccept: true},
				}
				digest, err := heartbeat.AuthorityDigest(
					heartbeat.CheckInsPolicy{},
					heartbeat.AuthorityActions(actions),
				)
				if err != nil {
					t.Fatal(err)
				}
				if err := h.db.Instances().
					ApproveAuthority(t.Context(), h.instance.ProfileID, h.dir, digest, "cli", h.now()); err != nil {
					t.Fatal(err)
				}
				admitted, err := h.db.Instances().
					Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: h.instance.ID, OccurrenceID: "auto-pick", Now: h.now(), NextWakeAt: h.now().Add(time.Hour), MaxConcurrent: 1})
				if err != nil || !admitted {
					t.Fatalf("admit=%v err=%v", admitted, err)
				}
				outcome := heartbeat.TriageReplyOutcome(
					`{"proposals":[{"title":"Report","body":"Evidence","item_keys":[],"action":"report"}]}`,
					heartbeat.CheckDigest{},
					2,
					actions...)
				req := heartbeat.FinishRequest{
					InstanceID:          h.instance.ID,
					OccurrenceID:        "auto-pick",
					Source:              "timer",
					Result:              outcome.Result,
					Reason:              outcome.Reason,
					Report:              outcome.Report,
					Now:                 h.now(),
					ExpiresAt:           h.now().Add(time.Hour),
					Actions:             actions,
					AuthorityDigest:     digest,
					DefinitionDir:       h.dir,
					AutoAcceptPerDay:    20,
					QueueCap:            10,
					UnattendedTurnLimit: 300,
				}
				result, err := h.db.Instances().Finish(t.Context(), req)
				if err != nil || len(result.Picks) != 1 {
					t.Fatalf("result=%+v err=%v", result, err)
				}
				pick := result.Picks[0]
				payload := proposalPickHook(h.instance, pick)
				if payload.Event != "agent.instance.proposal.picked" || payload.Actor != "auto_accept" ||
					payload.ReportID != pick.ReportID {
					t.Fatalf("hook=%+v", payload)
				}
				if err := releaseInstanceInputs(
					t.Context(),
					&bootState{sessions: h.manager},
					h.db.Instances(),
					h.instance,
				); err != nil {
					t.Fatal(err)
				}
				waitForConditionWithin(t, "auto pick delivered", 15*time.Second, func() bool {
					entry, err := h.db.GetSessionInputQueueEntry(t.Context(), pick.TargetSession, pick.QueueEntryID)
					if err != nil {
						t.Fatal(err)
					}
					return entry.Status == "sent"
				})
				state, err := h.db.Instances().WakeState(t.Context(), h.instance.ID)
				if err != nil || state.UnattendedTurns < 2 {
					t.Fatalf("state=%+v err=%v", state, err)
				}
				manualReport, err := h.db.InstanceReports().
					AppendReport(t.Context(), h.instance.ID, agentinstance.ReportInput{SourceKey: "occ:operator-pick", Kind: "proposal", Payload: []byte(`{"proposals":[{"title":"Review manually","body":"Operator request","item_keys":[]}]}`)})
				if err != nil {
					t.Fatal(err)
				}
				manualPick, err := h.db.Instances().
					PickProposal(t.Context(), agentinstance.PickRequest{InstanceID: h.instance.ID, ReportID: manualReport.ID, Actor: "operator", ProposalIndex: 0, QueueCap: 10})
				if err != nil {
					t.Fatal(err)
				}
				if err := releaseInstanceInputs(
					t.Context(),
					&bootState{sessions: h.manager},
					h.db.Instances(),
					h.instance,
				); err != nil {
					t.Fatal(err)
				}
				waitForConditionWithin(t, "operator pick delivered", 15*time.Second, func() bool {
					entry, err := h.db.GetSessionInputQueueEntry(
						t.Context(),
						manualPick.TargetSession,
						manualPick.QueueEntryID,
					)
					if err != nil {
						t.Fatal(err)
					}
					return entry.Status == "sent"
				})
				state, err = h.db.Instances().WakeState(t.Context(), h.instance.ID)
				if err != nil || state.UnattendedTurns != 0 {
					t.Fatalf("operator state=%+v err=%v", state, err)
				}
				repeated, err := h.db.Instances().Finish(t.Context(), req)
				if err != nil || len(repeated.Picks) != 0 {
					t.Fatalf("repeated=%+v err=%v", repeated, err)
				}
			},
		)
	}
}
