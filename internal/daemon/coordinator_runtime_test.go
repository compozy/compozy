package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/coordinator"
	eventspkg "github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	storepkg "github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestCoordinatorRuntimeBootstrapsManagedCoordinatorSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	run := coordinatorRuntimeRun()
	run.SetNetworkState(participation.LocalSpec(), "", "", "")
	store := newCoordinatorRuntimeStore(coordinatorRuntimeTask(), run)
	sessions := &coordinatorRuntimeSessions{}
	hooks := &recordingCoordinatorHooks{}
	runtime := newCoordinatorRuntimeForTest(t, store, sessions, hooks, coordinatorRuntimeConfig(), now)

	info, created, err := runtime.bootstrapRun(
		context.Background(),
		store.tasks["task-1"],
		store.runs["run-1"],
		coordinator.ReasonRunEnqueued,
	)
	if err != nil {
		t.Fatalf("bootstrapRun() error = %v", err)
	}
	if !created {
		t.Fatal("bootstrapRun() created = false, want true")
	}
	if info == nil || info.Type != session.SessionTypeCoordinator {
		t.Fatalf("bootstrapRun() info = %#v, want coordinator info", info)
	}

	call := sessions.createCall(0)
	if call.Type != session.SessionTypeCoordinator {
		t.Fatalf("CreateOpts.Type = %q, want coordinator", call.Type)
	}
	if call.AgentName != "coordinator" || call.Provider != "codex" || call.Model != "gpt-5" {
		t.Fatalf(
			"CreateOpts agent/provider/model = %q/%q/%q, want coordinator/codex/gpt-5",
			call.AgentName,
			call.Provider,
			call.Model,
		)
	}
	if call.Workspace != "ws-1" || call.ResolvedNetworkParticipation == nil ||
		call.ResolvedNetworkParticipation.Mode != participation.ModeLocal {
		t.Fatalf(
			"CreateOpts workspace/participation = %q/%#v, want ws-1/local",
			call.Workspace,
			call.ResolvedNetworkParticipation,
		)
	}
	if call.Lineage == nil || call.Lineage.SpawnRole != string(session.SessionTypeCoordinator) {
		t.Fatalf("CreateOpts.Lineage = %#v, want coordinator root lineage", call.Lineage)
	}
	if err := storepkg.ValidateSessionLineage("coord-1", call.Lineage); err != nil {
		t.Fatalf("ValidateSessionLineage(coordinator create) error = %v", err)
	}
	if call.Lineage.TTLExpiresAt == nil || !call.Lineage.TTLExpiresAt.Equal(now.Add(2*time.Hour)) {
		t.Fatalf("Lineage.TTLExpiresAt = %#v, want %s", call.Lineage.TTLExpiresAt, now.Add(2*time.Hour))
	}
	if !coordinator.ToolAllowed(participation.LocalSpec(), toolspkg.ToolIDTaskRunClaimNext.String()) ||
		coordinator.ToolAllowed(participation.LocalSpec(), toolspkg.ToolIDTaskCancel.String()) {
		t.Fatal("coordinator tool allowlist is not restricted as expected")
	}
	for _, networkTool := range []string{
		toolspkg.ToolIDNetworkChannels.String(),
		toolspkg.ToolIDNetworkInbox.String(),
		toolspkg.ToolIDNetworkSend.String(),
	} {
		if slices.Contains(call.Lineage.PermissionPolicy.Tools, networkTool) {
			t.Fatalf(
				"Lineage.PermissionPolicy.Tools = %#v, want no %q",
				call.Lineage.PermissionPolicy.Tools,
				networkTool,
			)
		}
	}
	if got := call.Lineage.PermissionPolicy.NetworkChannels; len(got) != 0 {
		t.Fatalf("Lineage.PermissionPolicy.NetworkChannels = %#v, want empty", got)
	}
	for _, required := range []string{"agh me context", "agh task next", "agh spawn"} {
		if !contains(call.PromptOverlay, required) {
			t.Fatalf("PromptOverlay missing %q:\n%s", required, call.PromptOverlay)
		}
	}
	for _, forbidden := range []string{"agh ch", "ch-run-1", "participation_channel", "coordination_channel_id"} {
		if contains(call.PromptOverlay, forbidden) {
			t.Fatalf("PromptOverlay contains local coordinator guidance %q:\n%s", forbidden, call.PromptOverlay)
		}
	}
	if got := sessions.promptCount(); got != 1 {
		t.Fatalf("PromptSynthetic count = %d, want 1", got)
	}
	prompt := sessions.promptCall(0)
	if prompt.id != info.ID {
		t.Fatalf("PromptSynthetic id = %q, want %q", prompt.id, info.ID)
	}
	assertCoordinatorPromptInterrupt(t, prompt)
	if prompt.opts.Metadata.TaskID != "task-1" ||
		prompt.opts.Metadata.TaskRunID != "run-1" ||
		prompt.opts.Metadata.CoordinatorSessionID != info.ID {
		t.Fatalf("PromptSynthetic metadata = %#v, want task/run/coordinator ids", prompt.opts.Metadata)
	}
	if !contains(prompt.opts.Message, "Claim the run through the AGH task claim path") {
		t.Fatalf("PromptSynthetic message = %q, want claim instruction", prompt.opts.Message)
	}
	if hooks.preSpawnCount() != 1 || hooks.spawnedCount() != 1 {
		t.Fatalf("hook counts pre_spawn/spawned = %d/%d, want 1/1", hooks.preSpawnCount(), hooks.spawnedCount())
	}
	if got := hooks.spawnedPayload(0).Model; got != "gpt-5" {
		t.Fatalf("spawned hook model = %q, want gpt-5", got)
	}
}

func TestCoordinatorRuntimeEmitsRoleFallbackEvents(t *testing.T) {
	t.Parallel()

	t.Run("Should emit the fallback before creating the coordinator on its next route", func(t *testing.T) {
		t.Parallel()

		sessions := &coordinatorFallbackSessions{
			coordinatorRuntimeSessions: &coordinatorRuntimeSessions{},
			primaryErr:                 errors.New("primary coordinator route unavailable"),
		}
		events := &roleEventRecorder{}
		runtime, err := newCoordinatorRuntime(
			t.Context(),
			newCoordinatorRuntimeStore(coordinatorRuntimeTask(), coordinatorRuntimeRun()),
			sessions,
			&staticCoordinatorRoleResolver{cfg: coordinatorRuntimeConfig()},
			nil,
			discardLogger(),
			func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) },
			withCoordinatorRoleEvents(events),
		)
		if err != nil {
			t.Fatalf("newCoordinatorRuntime() error = %v", err)
		}
		cfg := coordinatorRuntimeConfig()
		cfg.Fallbacks = []aghconfig.RoleFallback{{Provider: "claude", Model: "sonnet"}}
		info, err := runtime.startCoordinatorSession(
			t.Context(),
			coordinator.Decision{WorkspaceID: "ws-1"},
			cfg,
			participation.LocalSpec(),
		)
		if err != nil {
			t.Fatalf("startCoordinatorSession() error = %v", err)
		}
		if info == nil || info.Provider != "claude" || info.Model != "sonnet" {
			t.Fatalf("startCoordinatorSession() = %#v, want fallback route", info)
		}
		attempts := sessions.snapshotAttempts()
		if len(attempts) != 2 || attempts[0].Provider != "codex" || attempts[1].Provider != "claude" {
			t.Fatalf("coordinator route attempts = %#v, want codex then claude", attempts)
		}
		event := events.single(t)
		if event.Type != eventspkg.RoleFallbackUsed || event.WorkspaceID != "ws-1" {
			t.Fatalf("coordinator fallback event = %#v", event)
		}
		var payload roleFallbackEventPayload
		if err := json.Unmarshal(event.Content, &payload); err != nil {
			t.Fatalf("json.Unmarshal(fallback event) error = %v", err)
		}
		if payload.Role != string(aghconfig.RoleCoordinator) || payload.Attempt != 1 ||
			payload.Provider != "claude" || payload.Model != "sonnet" {
			t.Fatalf("coordinator fallback payload = %#v", payload)
		}
	})
}

func TestCoordinatorRuntimeBindsLiveParticipationToCoordinatorLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the coordinator snapshot when a Local run wakes the existing session", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		liveRun := coordinatorRuntimeRun()
		liveSpec := liveRun.NetworkSpecSnapshot()
		store := newCoordinatorRuntimeStore(coordinatorRuntimeTask(), liveRun)
		sessions := &coordinatorRuntimeSessions{}
		hooks := &recordingCoordinatorHooks{}
		runtime := newCoordinatorRuntimeForTest(t, store, sessions, hooks, coordinatorRuntimeConfig(), now)

		info, created, err := runtime.bootstrapRun(
			context.Background(),
			store.tasks["task-1"],
			store.runs["run-1"],
			coordinator.ReasonRunEnqueued,
		)
		if err != nil {
			t.Fatalf("bootstrapRun(Live) error = %v", err)
		}
		if !created || info == nil {
			t.Fatalf("bootstrapRun(Live) created/info = %t/%#v, want created coordinator", created, info)
		}
		if got := info.NetworkParticipation; got != liveSpec {
			t.Fatalf("coordinator NetworkParticipation = %#v, want %#v", got, liveSpec)
		}

		call := sessions.createCall(0)
		if call.ResolvedNetworkParticipation == nil || *call.ResolvedNetworkParticipation != liveSpec {
			t.Fatalf(
				"CreateOpts.ResolvedNetworkParticipation = %#v, want %#v",
				call.ResolvedNetworkParticipation,
				liveSpec,
			)
		}
		if call.Lineage == nil ||
			!slices.Equal(call.Lineage.PermissionPolicy.NetworkChannels, []string{liveSpec.ChannelID}) {
			t.Fatalf(
				"Lineage.PermissionPolicy.NetworkChannels = %#v, want [%q]",
				call.Lineage,
				liveSpec.ChannelID,
			)
		}
		if !slices.Contains(call.Lineage.PermissionPolicy.Tools, toolspkg.ToolIDNetworkSend.String()) {
			t.Fatalf("Lineage.PermissionPolicy.Tools = %#v, want Network send", call.Lineage.PermissionPolicy.Tools)
		}
		for _, required := range []string{"participation_channel: " + liveSpec.ChannelID, "agh ch"} {
			if !contains(call.PromptOverlay, required) {
				t.Fatalf("PromptOverlay missing %q:\n%s", required, call.PromptOverlay)
			}
		}

		preSpawn := hooks.preSpawnPayload(0)
		spawned := hooks.spawnedPayload(0)
		if preSpawn.ResolvedNetworkParticipation == nil ||
			*preSpawn.ResolvedNetworkParticipation != liveSpec {
			t.Fatalf(
				"pre-spawn participation = %#v, want %#v",
				preSpawn.ResolvedNetworkParticipation,
				liveSpec,
			)
		}
		if spawned.ResolvedNetworkParticipation == nil ||
			*spawned.ResolvedNetworkParticipation != liveSpec {
			t.Fatalf(
				"spawned participation = %#v, want %#v",
				spawned.ResolvedNetworkParticipation,
				liveSpec,
			)
		}

		localRun := coordinatorRuntimeRun()
		localRun.ID = "run-2"
		localRun.SetNetworkState(participation.LocalSpec(), "", "", "")
		store.runs[localRun.ID] = localRun
		sessions.promptErr = fmt.Errorf("synthetic prompt failed")
		_, created, err = runtime.bootstrapRun(
			context.Background(),
			store.tasks["task-1"],
			store.runs[localRun.ID],
			coordinator.ReasonRunEnqueued,
		)
		if err == nil {
			t.Fatal("bootstrapRun(existing Local wake) error = nil, want prompt failure")
		}
		if created {
			t.Fatal("bootstrapRun(existing Local wake) created = true, want existing coordinator")
		}
		if got := sessions.createCount(); got != 1 {
			t.Fatalf("Create count = %d, want original coordinator only", got)
		}

		decisionPayload := hooks.lastDecisionPayload()
		failedPayload := hooks.lastFailedPayload()
		if decisionPayload.ResolvedNetworkParticipation == nil ||
			*decisionPayload.ResolvedNetworkParticipation != liveSpec {
			t.Fatalf(
				"existing decision participation = %#v, want coordinator %#v",
				decisionPayload.ResolvedNetworkParticipation,
				liveSpec,
			)
		}
		if failedPayload.ResolvedNetworkParticipation == nil ||
			*failedPayload.ResolvedNetworkParticipation != liveSpec {
			t.Fatalf(
				"failed wake participation = %#v, want coordinator %#v",
				failedPayload.ResolvedNetworkParticipation,
				liveSpec,
			)
		}
	})
}

func TestCoordinatorRuntimeBootstrapsWithTaskContextOverlay(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 5, 12, 20, 0, 0, time.UTC)
	store := newCoordinatorRuntimeStore(coordinatorRuntimeTask(), coordinatorRuntimeRun())
	sessions := &coordinatorRuntimeSessions{}
	hooks := &recordingCoordinatorHooks{}
	runtime := newCoordinatorRuntimeForTest(t, store, sessions, hooks, coordinatorRuntimeConfig(), now)
	overlay := &taskContextOverlayStub{overlay: "coordinator task context bundle"}
	runtime.contextOverlay = overlay

	_, created, err := runtime.bootstrapRun(
		context.Background(),
		store.tasks["task-1"],
		store.runs["run-1"],
		coordinator.ReasonRunEnqueued,
	)
	if err != nil {
		t.Fatalf("bootstrapRun() error = %v", err)
	}
	if !created {
		t.Fatal("bootstrapRun() created = false, want true")
	}
	call := sessions.createCall(0)
	if !contains(call.PromptOverlay, "coordinator task context bundle") ||
		!contains(call.PromptOverlay, "agh task next") {
		t.Fatalf("PromptOverlay = %q, want task context plus coordinator instructions", call.PromptOverlay)
	}
	if len(overlay.calls) != 1 ||
		overlay.calls[0].taskID != "task-1" ||
		overlay.calls[0].runID != "run-1" {
		t.Fatalf("overlay calls = %#v, want task/run context", overlay.calls)
	}
}

func TestCoordinatorRuntimeSkipsIneligibleRuns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		task       taskpkg.Task
		run        taskpkg.Run
		cfg        aghconfig.ResolvedCoordinatorRole
		wantReason string
	}{
		{
			name:       "disabled config",
			task:       coordinatorRuntimeTask(),
			run:        coordinatorRuntimeRun(),
			cfg:        aghconfig.DefaultResolvedCoordinatorRole(),
			wantReason: coordinator.DecisionDisabled,
		},
		{
			name: "global scope",
			task: func() taskpkg.Task {
				task := coordinatorRuntimeTask()
				task.Scope = taskpkg.ScopeGlobal
				task.WorkspaceID = ""
				return task
			}(),
			run:        coordinatorRuntimeRun(),
			cfg:        coordinatorRuntimeConfig(),
			wantReason: coordinator.DecisionGlobalScope,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := newCoordinatorRuntimeStore(tc.task, tc.run)
			sessions := &coordinatorRuntimeSessions{}
			hooks := &recordingCoordinatorHooks{}
			runtime := newCoordinatorRuntimeForTest(t, store, sessions, hooks, tc.cfg, time.Now().UTC())

			_, created, err := runtime.bootstrapRun(
				context.Background(),
				tc.task,
				tc.run,
				coordinator.ReasonRunEnqueued,
			)
			if err != nil {
				t.Fatalf("bootstrapRun() error = %v", err)
			}
			if created {
				t.Fatal("bootstrapRun() created = true, want false")
			}
			if got := sessions.createCount(); got != 0 {
				t.Fatalf("Create count = %d, want 0", got)
			}
			if got := hooks.lastDecision(); got != tc.wantReason {
				t.Fatalf("last decision = %q, want %q", got, tc.wantReason)
			}
		})
	}
}

func TestCoordinatorRuntimePreventsDuplicateCoordinatorsUnderConcurrentEnqueue(t *testing.T) {
	t.Parallel()

	const attempts = 24
	store := newCoordinatorRuntimeStore(coordinatorRuntimeTask(), coordinatorRuntimeRun())
	createStarted := make(chan struct{}, attempts)
	releaseCreate := make(chan struct{})
	var releaseCreateOnce sync.Once
	t.Cleanup(func() {
		releaseCreateOnce.Do(func() {
			close(releaseCreate)
		})
	})
	promptStarted := make(chan struct{}, 1)
	releasePrompt := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() {
			close(releasePrompt)
		})
	})
	sessions := &coordinatorRuntimeSessions{
		createStarted: createStarted,
		createRelease: releaseCreate,
		promptStarted: promptStarted,
		promptRelease: releasePrompt,
	}
	runtime := newCoordinatorRuntimeForTest(
		t,
		store,
		sessions,
		&recordingCoordinatorHooks{},
		coordinatorRuntimeConfig(),
		time.Now().UTC(),
	)

	errs := make(chan error, attempts)
	for range attempts {
		go func() {
			_, _, err := runtime.bootstrapRun(
				context.Background(),
				store.tasks["task-1"],
				store.runs["run-1"],
				coordinator.ReasonRunEnqueued,
			)
			errs <- err
		}()
	}
	for range 2 {
		select {
		case <-createStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for concurrent coordinator creates")
		}
	}
	releaseCreateOnce.Do(func() {
		close(releaseCreate)
	})
	select {
	case <-promptStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first coordinator prompt")
	}
	for range attempts - 1 {
		select {
		case err := <-errs:
			if err != nil {
				t.Fatalf("bootstrapRun() concurrent error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for concurrent bootstrap calls to coalesce")
		}
	}
	createCount := sessions.createCount()
	if createCount < 2 {
		t.Fatalf("Create count = %d, want at least 2 to prove create ran outside runtime mutex", createCount)
	}
	if got, want := sessions.stopCount(), createCount-1; got != want {
		t.Fatalf("StopWithCause count = %d, want %d duplicate coordinators stopped", got, want)
	}
	if got := sessions.activeCoordinatorCount("ws-1"); got != 1 {
		t.Fatalf("active coordinator count = %d, want 1", got)
	}
	if got := sessions.promptCount(); got != 1 {
		t.Fatalf("PromptSynthetic count = %d, want 1", got)
	}
	prompt := sessions.promptCall(0)
	assertCoordinatorPromptInterrupt(t, prompt)
	if prompt.id != "coord-1" {
		t.Fatalf("PromptSynthetic id = %q, want coord-1", prompt.id)
	}
	if prompt.opts.Metadata.TaskID != "task-1" ||
		prompt.opts.Metadata.TaskRunID != "run-1" ||
		prompt.opts.Metadata.CoordinatorSessionID != "coord-1" ||
		prompt.opts.Metadata.Reason != coordinator.ReasonRunEnqueued {
		t.Fatalf("PromptSynthetic metadata = %#v, want task/run/coordinator/reason", prompt.opts.Metadata)
	}
	releaseOnce.Do(func() {
		close(releasePrompt)
	})
	select {
	case err := <-errs:
		if err != nil {
			t.Fatalf("bootstrapRun() prompt owner error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for prompt owner bootstrap")
	}
}

func TestCoordinatorRuntimeShutdownStopsPromptEventDrains(t *testing.T) {
	t.Parallel()

	store := newCoordinatorRuntimeStore(coordinatorRuntimeTask(), coordinatorRuntimeRun())
	events := make(chan acp.AgentEvent)
	t.Cleanup(func() {
		close(events)
	})
	sessions := &coordinatorRuntimeSessions{promptEvents: events}
	runtime := newCoordinatorRuntimeForTest(
		t,
		store,
		sessions,
		&recordingCoordinatorHooks{},
		coordinatorRuntimeConfig(),
		time.Now().UTC(),
	)
	if err := runtime.promptCoordinator(
		context.Background(),
		&session.Info{ID: "coord-drain"},
		coordinator.Decision{TaskID: "task-1", RunID: "run-1"},
		coordinator.ReasonRunEnqueued,
	); err != nil {
		t.Fatalf("promptCoordinator() error = %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := runtime.shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}

func TestCoordinatorRuntimeObservesTaskRunEnqueuedButNotTaskCreation(t *testing.T) {
	t.Parallel()

	store := newCoordinatorRuntimeStore(coordinatorRuntimeTask(), coordinatorRuntimeRun())
	sessions := &coordinatorRuntimeSessions{}
	runtime := newCoordinatorRuntimeForTest(
		t,
		store,
		sessions,
		&recordingCoordinatorHooks{},
		coordinatorRuntimeConfig(),
		time.Now().UTC(),
	)

	if got := sessions.createCount(); got != 0 {
		t.Fatalf("Create count before enqueue = %d, want 0", got)
	}
	runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
		TaskRunContext: hookspkg.TaskRunContext{TaskID: "task-1", RunID: "run-1"},
	})
	if got := sessions.createCount(); got != 1 {
		t.Fatalf("Create count after enqueue = %d, want 1", got)
	}
	if got := sessions.promptCount(); got != 1 {
		t.Fatalf("PromptSynthetic count after enqueue = %d, want 1", got)
	}
	assertCoordinatorPromptInterrupt(t, sessions.promptCall(0))
}

func TestCoordinatorRuntimeManualSessionsCoexist(t *testing.T) {
	t.Parallel()

	store := newCoordinatorRuntimeStore(coordinatorRuntimeTask(), coordinatorRuntimeRun())
	sessions := &coordinatorRuntimeSessions{
		infos: []*session.Info{{
			ID:          "manual-1",
			Type:        session.SessionTypeUser,
			WorkspaceID: "ws-1",
			State:       session.StateActive,
		}},
	}
	runtime := newCoordinatorRuntimeForTest(
		t,
		store,
		sessions,
		&recordingCoordinatorHooks{},
		coordinatorRuntimeConfig(),
		time.Now().UTC(),
	)

	_, created, err := runtime.bootstrapRun(
		context.Background(),
		store.tasks["task-1"],
		store.runs["run-1"],
		coordinator.ReasonRunEnqueued,
	)
	if err != nil {
		t.Fatalf("bootstrapRun() error = %v", err)
	}
	if !created {
		t.Fatal("bootstrapRun() created = false, want true")
	}
	if got := sessions.createCount(); got != 1 {
		t.Fatalf("Create count = %d, want 1", got)
	}
	if got := sessions.promptCount(); got != 1 {
		t.Fatalf("PromptSynthetic count = %d, want 1", got)
	}
	assertCoordinatorPromptInterrupt(t, sessions.promptCall(0))
}

func TestCoordinatorRuntimePromptsExistingCoordinatorForQueuedRun(t *testing.T) {
	t.Parallel()

	store := newCoordinatorRuntimeStore(coordinatorRuntimeTask(), coordinatorRuntimeRun())
	ttl := time.Now().UTC().Add(time.Hour)
	sessions := &coordinatorRuntimeSessions{
		infos: []*session.Info{{
			ID:          "coord-existing",
			Type:        session.SessionTypeCoordinator,
			AgentName:   "coordinator",
			WorkspaceID: "ws-1",
			Workspace:   "ws-1",
			State:       session.StateActive,
			Lineage: &storepkg.SessionLineage{
				RootSessionID: "coord-existing",
				SpawnRole:     string(session.SessionTypeCoordinator),
				TTLExpiresAt:  &ttl,
			},
		}},
	}
	runtime := newCoordinatorRuntimeForTest(
		t,
		store,
		sessions,
		&recordingCoordinatorHooks{},
		coordinatorRuntimeConfig(),
		time.Now().UTC(),
	)

	info, created, err := runtime.bootstrapRun(
		context.Background(),
		store.tasks["task-1"],
		store.runs["run-1"],
		coordinator.ReasonRecovery,
	)
	if err != nil {
		t.Fatalf("bootstrapRun(existing) error = %v", err)
	}
	if created {
		t.Fatal("bootstrapRun(existing) created = true, want false")
	}
	if info == nil || info.ID != "coord-existing" {
		t.Fatalf("bootstrapRun(existing) info = %#v, want existing coordinator", info)
	}
	if got := sessions.createCount(); got != 0 {
		t.Fatalf("Create count = %d, want 0", got)
	}
	if got := sessions.promptCount(); got != 1 {
		t.Fatalf("PromptSynthetic count = %d, want 1", got)
	}
	prompt := sessions.promptCall(0)
	assertCoordinatorPromptInterrupt(t, prompt)
	if got := prompt.opts.Metadata.TaskRunID; got != "run-1" {
		t.Fatalf("PromptSynthetic run id = %q, want run-1", got)
	}
}

func TestCoordinatorRuntimeRecoversWhenCoordinatorStopsWithExecutableWork(t *testing.T) {
	t.Parallel()

	store := newCoordinatorRuntimeStore(coordinatorRuntimeTask(), coordinatorRuntimeRun())
	sessions := &coordinatorRuntimeSessions{}
	hooks := &recordingCoordinatorHooks{}
	runtime := newCoordinatorRuntimeForTest(t, store, sessions, hooks, coordinatorRuntimeConfig(), time.Now().UTC())

	runtime.OnSessionStopped(context.Background(), &session.Session{
		ID:          "coord-old",
		AgentName:   "coordinator",
		WorkspaceID: "ws-1",
		Workspace:   "ws-1",
		Type:        session.SessionTypeCoordinator,
		State:       session.StateStopped,
	})
	if got := sessions.createCount(); got != 1 {
		t.Fatalf("Create count after coordinator stop = %d, want 1", got)
	}
	if hooks.stoppedCount() != 1 {
		t.Fatalf("stopped hook count = %d, want 1", hooks.stoppedCount())
	}
}

func TestCoordinatorRuntimePreSpawnDeny(t *testing.T) {
	t.Parallel()

	store := newCoordinatorRuntimeStore(coordinatorRuntimeTask(), coordinatorRuntimeRun())
	sessions := &coordinatorRuntimeSessions{}
	hooks := &recordingCoordinatorHooks{denyPreSpawn: true}
	runtime := newCoordinatorRuntimeForTest(t, store, sessions, hooks, coordinatorRuntimeConfig(), time.Now().UTC())

	_, created, err := runtime.bootstrapRun(
		context.Background(),
		store.tasks["task-1"],
		store.runs["run-1"],
		coordinator.ReasonRunEnqueued,
	)
	if err != nil {
		t.Fatalf("bootstrapRun() error = %v", err)
	}
	if created {
		t.Fatal("bootstrapRun() created = true, want false")
	}
	if got := sessions.createCount(); got != 0 {
		t.Fatalf("Create count = %d, want 0", got)
	}
	if got := hooks.lastDecision(); got != coordinator.DecisionDenied {
		t.Fatalf("last decision = %q, want denied", got)
	}
}

func TestCoordinatorRuntimePreSpawnDenyFromHookError(t *testing.T) {
	t.Parallel()

	store := newCoordinatorRuntimeStore(coordinatorRuntimeTask(), coordinatorRuntimeRun())
	sessions := &coordinatorRuntimeSessions{}
	hooks := &recordingCoordinatorHooks{denyPreSpawn: true, denyWithError: true}
	runtime := newCoordinatorRuntimeForTest(t, store, sessions, hooks, coordinatorRuntimeConfig(), time.Now().UTC())

	_, created, err := runtime.bootstrapRun(
		context.Background(),
		store.tasks["task-1"],
		store.runs["run-1"],
		coordinator.ReasonRunEnqueued,
	)
	if err != nil {
		t.Fatalf("bootstrapRun() error = %v, want denied decision without failure", err)
	}
	if created {
		t.Fatal("bootstrapRun() created = true, want false")
	}
	if got := sessions.createCount(); got != 0 {
		t.Fatalf("Create count = %d, want 0", got)
	}
	if got := hooks.lastDecision(); got != coordinator.DecisionDenied {
		t.Fatalf("last decision = %q, want denied", got)
	}
	if got := hooks.failedCount(); got != 0 {
		t.Fatalf("failed hook count = %d, want 0 for policy denial", got)
	}
}

func TestHooksNotifierTaskRunEnqueuedObserversReceivePayload(t *testing.T) {
	t.Parallel()

	notifier := newHooksNotifier(discardLogger(), func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	})
	observer := &recordingTaskRunEnqueuedObserver{}
	notifier.AddTaskRunEnqueuedObserver(observer)

	_, err := notifier.DispatchTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
		TaskRunContext: hookspkg.TaskRunContext{
			TaskID:      "task-1",
			RunID:       "run-1",
			WorkspaceID: "ws-1",
			ResolvedNetworkParticipation: &participation.Spec{
				Version:         participation.SpecVersion,
				Mode:            participation.ModeLive,
				WorkspaceID:     "ws-1",
				ChannelStrategy: participation.StrategyNamed,
				ChannelID:       "ch-run-1",
				Source:          participation.SourceExplicitRequest,
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskRunEnqueued() error = %v", err)
	}
	if got := observer.count(); got != 1 {
		t.Fatalf("observer count = %d, want 1", got)
	}
	last := observer.last()
	if last.ResolvedNetworkParticipation == nil ||
		last.ResolvedNetworkParticipation.ChannelID != "ch-run-1" {
		t.Fatalf(
			"observer resolved participation = %#v, want channel ch-run-1",
			last.ResolvedNetworkParticipation,
		)
	}
}

func newCoordinatorRuntimeForTest(
	t *testing.T,
	store *coordinatorRuntimeStore,
	sessions *coordinatorRuntimeSessions,
	hooks *recordingCoordinatorHooks,
	cfg aghconfig.ResolvedCoordinatorRole,
	now time.Time,
) *coordinatorRuntime {
	t.Helper()
	runtime, err := newCoordinatorRuntime(
		context.Background(),
		store,
		sessions,
		&staticCoordinatorRoleResolver{cfg: cfg},
		hooks,
		discardLogger(),
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("newCoordinatorRuntime() error = %v", err)
	}
	return runtime
}

func coordinatorRuntimeConfig() aghconfig.ResolvedCoordinatorRole {
	cfg := aghconfig.DefaultResolvedCoordinatorRole()
	cfg.Enabled = true
	cfg.AgentName = "coordinator"
	cfg.Provider = "codex"
	cfg.Model = "gpt-5"
	cfg.TTL = 2 * time.Hour
	cfg.MaxChildren = 5
	cfg.MaxActiveSessionsPerWorkspace = 5
	return cfg
}

func coordinatorRuntimeTask() taskpkg.Task {
	return taskpkg.Task{
		ID:          "task-1",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: "ws-1",
		Status:      taskpkg.TaskStatusReady,
		Title:       "Build the thing",
	}
}

func coordinatorRuntimeRun() taskpkg.Run {
	run := taskpkg.Run{
		ID:       "run-1",
		TaskID:   "task-1",
		Status:   taskpkg.TaskRunStatusQueued,
		Metadata: json.RawMessage(`{"workflow_id":"wf-1"}`),
	}
	run.SetNetworkState(daemonTestLiveParticipation("ws-1", "ch-run-1"), "", "", "")
	return run
}

type staticCoordinatorRoleResolver struct {
	cfg aghconfig.ResolvedCoordinatorRole
	err error
	mu  sync.Mutex
	got []string
}

func (r *staticCoordinatorRoleResolver) ResolveCoordinatorRole(
	_ context.Context,
	workspaceID string,
) (aghconfig.ResolvedCoordinatorRole, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, workspaceID)
	if r.err != nil {
		return aghconfig.ResolvedCoordinatorRole{}, r.err
	}
	return r.cfg, nil
}

type coordinatorRuntimeStore struct {
	mu    sync.Mutex
	tasks map[string]taskpkg.Task
	runs  map[string]taskpkg.Run
}

func newCoordinatorRuntimeStore(tasks taskpkg.Task, runs taskpkg.Run) *coordinatorRuntimeStore {
	return &coordinatorRuntimeStore{
		tasks: map[string]taskpkg.Task{tasks.ID: tasks},
		runs:  map[string]taskpkg.Run{runs.ID: runs},
	}
}

func (s *coordinatorRuntimeStore) GetTask(_ context.Context, id string) (taskpkg.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok {
		return taskpkg.Task{}, taskpkg.ErrTaskNotFound
	}
	return task, nil
}

func (s *coordinatorRuntimeStore) GetTaskRun(_ context.Context, id string) (taskpkg.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	if !ok {
		return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
	}
	return run, nil
}

func (s *coordinatorRuntimeStore) ListTaskRunsByStatus(
	_ context.Context,
	statuses []taskpkg.RunStatus,
) ([]taskpkg.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	allowed := make(map[taskpkg.RunStatus]struct{}, len(statuses))
	for _, status := range statuses {
		allowed[status.Normalize()] = struct{}{}
	}
	runs := make([]taskpkg.Run, 0, len(s.runs))
	for _, run := range s.runs {
		if _, ok := allowed[run.Status.Normalize()]; ok {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

type coordinatorRuntimeSessions struct {
	mu            sync.Mutex
	infos         []*session.Info
	createCalls   []session.CreateOpts
	createErr     error
	createStarted chan struct{}
	createRelease <-chan struct{}
	promptCalls   []coordinatorRuntimePromptCall
	promptErr     error
	promptEvents  <-chan acp.AgentEvent
	promptStarted chan struct{}
	promptRelease <-chan struct{}
	stopCalls     []coordinatorRuntimeStopCall
	stopErr       error
}

type coordinatorFallbackSessions struct {
	*coordinatorRuntimeSessions
	mu         sync.Mutex
	primaryErr error
	attempts   []session.CreateOpts
}

func (s *coordinatorFallbackSessions) Create(
	ctx context.Context,
	opts session.CreateOpts,
) (*session.Session, error) {
	s.mu.Lock()
	s.attempts = append(s.attempts, opts)
	attempt := len(s.attempts)
	s.mu.Unlock()
	if attempt == 1 {
		return nil, s.primaryErr
	}
	return s.coordinatorRuntimeSessions.Create(ctx, opts)
}

func (s *coordinatorFallbackSessions) snapshotAttempts() []session.CreateOpts {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]session.CreateOpts(nil), s.attempts...)
}

type coordinatorRuntimePromptCall struct {
	id   string
	opts session.SyntheticPromptOpts
}

type coordinatorRuntimeStopCall struct {
	id     string
	cause  session.StopCause
	detail string
}

func (s *coordinatorRuntimeSessions) Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error) {
	s.mu.Lock()
	createStarted := s.createStarted
	createRelease := s.createRelease
	s.mu.Unlock()
	if createStarted != nil {
		select {
		case createStarted <- struct{}{}:
		default:
		}
	}
	if createRelease != nil {
		select {
		case <-createRelease:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.createCalls = append(s.createCalls, opts)
	id := fmt.Sprintf("coord-%d", len(s.createCalls))
	info := &session.Info{
		ID:                   id,
		Name:                 opts.Name,
		AgentName:            opts.AgentName,
		Provider:             opts.Provider,
		Model:                opts.Model,
		WorkspaceID:          opts.Workspace,
		Workspace:            opts.Workspace,
		NetworkParticipation: daemonTestParticipationFromCreateOpts(opts),
		Type:                 opts.Type,
		Lineage:              opts.Lineage,
		State:                session.StateActive,
		CreatedAt:            time.Now().UTC(),
	}
	s.infos = append(s.infos, info)
	return &session.Session{
		ID:                   info.ID,
		Name:                 info.Name,
		AgentName:            info.AgentName,
		Provider:             info.Provider,
		Model:                info.Model,
		WorkspaceID:          info.WorkspaceID,
		Workspace:            info.Workspace,
		NetworkParticipation: info.NetworkParticipation,
		Type:                 info.Type,
		Lineage:              info.Lineage,
		State:                info.State,
		CreatedAt:            info.CreatedAt,
	}, nil
}

func (s *coordinatorRuntimeSessions) ListAll(context.Context) ([]*session.Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	infos := make([]*session.Info, len(s.infos))
	for i, info := range s.infos {
		if info == nil {
			continue
		}
		cloned := *info
		infos[i] = &cloned
	}
	return infos, nil
}

func (s *coordinatorRuntimeSessions) PromptSynthetic(
	ctx context.Context,
	id string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	s.mu.Lock()
	s.promptCalls = append(s.promptCalls, coordinatorRuntimePromptCall{id: id, opts: opts})
	promptErr := s.promptErr
	promptEvents := s.promptEvents
	promptStarted := s.promptStarted
	promptRelease := s.promptRelease
	s.mu.Unlock()
	if promptStarted != nil {
		select {
		case promptStarted <- struct{}{}:
		default:
		}
	}
	if promptErr != nil {
		return nil, promptErr
	}
	if promptRelease != nil {
		select {
		case <-promptRelease:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if promptEvents != nil {
		return promptEvents, nil
	}
	ch := make(chan acp.AgentEvent)
	close(ch)
	return ch, nil
}

func (s *coordinatorRuntimeSessions) StopWithCause(
	_ context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopCalls = append(s.stopCalls, coordinatorRuntimeStopCall{id: id, cause: cause, detail: detail})
	if s.stopErr != nil {
		return s.stopErr
	}
	for _, info := range s.infos {
		if info == nil || strings.TrimSpace(info.ID) != strings.TrimSpace(id) {
			continue
		}
		info.State = session.StateStopped
		info.StopDetail = detail
	}
	return nil
}

func (s *coordinatorRuntimeSessions) createCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.createCalls)
}

func (s *coordinatorRuntimeSessions) createCall(index int) session.CreateOpts {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.createCalls) {
		return session.CreateOpts{}
	}
	return s.createCalls[index]
}

func (s *coordinatorRuntimeSessions) activeCoordinatorCount(workspaceID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, info := range s.infos {
		if info == nil ||
			info.Type != session.SessionTypeCoordinator ||
			info.State != session.StateActive ||
			strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(workspaceID) {
			continue
		}
		count++
	}
	return count
}

func (s *coordinatorRuntimeSessions) promptCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.promptCalls)
}

func (s *coordinatorRuntimeSessions) promptCall(index int) coordinatorRuntimePromptCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.promptCalls) {
		return coordinatorRuntimePromptCall{}
	}
	return s.promptCalls[index]
}

func assertCoordinatorPromptInterrupt(t *testing.T, prompt coordinatorRuntimePromptCall) {
	t.Helper()
	if !prompt.opts.InterruptIfAgentWaiting {
		t.Fatal("PromptSynthetic InterruptIfAgentWaiting = false, want true")
	}
}

func (s *coordinatorRuntimeSessions) stopCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.stopCalls)
}

func (s *coordinatorRuntimeSessions) stopCall(index int) coordinatorRuntimeStopCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.stopCalls) {
		return coordinatorRuntimeStopCall{}
	}
	return s.stopCalls[index]
}

type recordingCoordinatorHooks struct {
	mu            sync.Mutex
	denyPreSpawn  bool
	denyWithError bool
	preSpawn      []hookspkg.CoordinatorPreSpawnPayload
	spawned       []hookspkg.CoordinatorSpawnedPayload
	decisions     []hookspkg.CoordinatorDecisionPayload
	stopped       []hookspkg.CoordinatorStoppedPayload
	failed        []hookspkg.CoordinatorFailedPayload
}

type recordingTaskRunEnqueuedObserver struct {
	mu       sync.Mutex
	payloads []hookspkg.TaskRunEnqueuedPayload
}

func (o *recordingTaskRunEnqueuedObserver) OnTaskRunEnqueued(
	_ context.Context,
	payload hookspkg.TaskRunEnqueuedPayload,
) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.payloads = append(o.payloads, payload)
}

func (o *recordingTaskRunEnqueuedObserver) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.payloads)
}

func (o *recordingTaskRunEnqueuedObserver) last() hookspkg.TaskRunEnqueuedPayload {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.payloads) == 0 {
		return hookspkg.TaskRunEnqueuedPayload{}
	}
	return o.payloads[len(o.payloads)-1]
}

func (h *recordingCoordinatorHooks) DispatchCoordinatorPreSpawn(
	_ context.Context,
	payload hookspkg.CoordinatorPreSpawnPayload,
) (hookspkg.CoordinatorPreSpawnPayload, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.preSpawn = append(h.preSpawn, payload)
	if h.denyPreSpawn {
		payload.Denied = true
		payload.DenyReason = "policy"
	}
	if h.denyWithError {
		return payload, fmt.Errorf("hooks: event %q denied", hookspkg.HookCoordinatorPreSpawn)
	}
	return payload, nil
}

func (h *recordingCoordinatorHooks) DispatchCoordinatorSpawned(
	_ context.Context,
	payload hookspkg.CoordinatorSpawnedPayload,
) (hookspkg.CoordinatorSpawnedPayload, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.spawned = append(h.spawned, payload)
	return payload, nil
}

func (h *recordingCoordinatorHooks) DispatchCoordinatorDecision(
	_ context.Context,
	payload hookspkg.CoordinatorDecisionPayload,
) (hookspkg.CoordinatorDecisionPayload, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.decisions = append(h.decisions, payload)
	return payload, nil
}

func (h *recordingCoordinatorHooks) DispatchCoordinatorStopped(
	_ context.Context,
	payload hookspkg.CoordinatorStoppedPayload,
) (hookspkg.CoordinatorStoppedPayload, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopped = append(h.stopped, payload)
	return payload, nil
}

func (h *recordingCoordinatorHooks) DispatchCoordinatorFailed(
	_ context.Context,
	payload hookspkg.CoordinatorFailedPayload,
) (hookspkg.CoordinatorFailedPayload, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.failed = append(h.failed, payload)
	return payload, nil
}

func (h *recordingCoordinatorHooks) preSpawnCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.preSpawn)
}

func (h *recordingCoordinatorHooks) spawnedCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.spawned)
}

func (h *recordingCoordinatorHooks) stoppedCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.stopped)
}

func (h *recordingCoordinatorHooks) failedCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.failed)
}

func (h *recordingCoordinatorHooks) spawnedPayload(index int) hookspkg.CoordinatorSpawnedPayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	if index < 0 || index >= len(h.spawned) {
		return hookspkg.CoordinatorSpawnedPayload{}
	}
	return h.spawned[index]
}

func (h *recordingCoordinatorHooks) preSpawnPayload(index int) hookspkg.CoordinatorPreSpawnPayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	if index < 0 || index >= len(h.preSpawn) {
		return hookspkg.CoordinatorPreSpawnPayload{}
	}
	return h.preSpawn[index]
}

func (h *recordingCoordinatorHooks) lastDecisionPayload() hookspkg.CoordinatorDecisionPayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.decisions) == 0 {
		return hookspkg.CoordinatorDecisionPayload{}
	}
	return h.decisions[len(h.decisions)-1]
}

func (h *recordingCoordinatorHooks) lastFailedPayload() hookspkg.CoordinatorFailedPayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.failed) == 0 {
		return hookspkg.CoordinatorFailedPayload{}
	}
	return h.failed[len(h.failed)-1]
}

func (h *recordingCoordinatorHooks) lastDecision() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.decisions) == 0 {
		return ""
	}
	return h.decisions[len(h.decisions)-1].Decision
}

func contains(value string, needle string) bool {
	return strings.Contains(value, needle)
}
