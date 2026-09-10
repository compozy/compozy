package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

func TestPromptActivitySupervisorReportPersistsHeartbeatWithoutEvent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	h := newHarness(t, WithNow(func() time.Time { return now }))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	supervisor := newPromptActivitySupervisor(
		testutil.Context(t),
		h.manager,
		session,
		newPromptTurnDispatchState(session, "turn-activity", TurnSourceUser, "hello"),
		testSupervisionConfig(),
	)
	supervisor.touch(now, runtimeActivityKindPromptStarted, "prompt started")
	supervisor.report(acp.PromptActivityReport{
		Timestamp: now.Add(5 * time.Second),
		Kind:      "agent_waiting",
		Detail:    "waiting for provider",
	})

	meta := readMeta(t, session.MetaPath())
	if meta.Liveness == nil || meta.Liveness.Activity == nil {
		t.Fatal("meta.Liveness.Activity = nil, want persisted activity")
	}
	if got, want := meta.Liveness.Activity.LastActivityKind, "agent_waiting"; got != want {
		t.Fatalf("activity kind = %q, want %q", got, want)
	}
	if got, want := meta.Liveness.Activity.LastActivityDetail, "waiting for provider"; got != want {
		t.Fatalf("activity detail = %q, want %q", got, want)
	}
	if meta.Liveness.Activity.LastActivityAt == nil ||
		!meta.Liveness.Activity.LastActivityAt.Equal(now) {
		t.Fatalf(
			"activity LastActivityAt = %#v, want last real activity timestamp",
			meta.Liveness.Activity.LastActivityAt,
		)
	}
	if got, want := meta.Liveness.Activity.IdleSeconds, int64(5); got != want {
		t.Fatalf("activity IdleSeconds = %d, want %d", got, want)
	}

	select {
	case event := <-supervisor.eventsChannel():
		t.Fatalf("unexpected runtime event from heartbeat-only report: %#v", event)
	default:
	}
}

// Invariant: waiting reports never renew work evidence. Owner: session; canonical activity/supervision suite.
func TestPromptActivitySupervisorWaitingHeartbeatDoesNotPreventTimeout(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	config := testSupervisionConfig()
	config.QuietAfter, config.StopGrace = time.Second, time.Second
	h := newHarness(t, WithNow(func() time.Time { return now }), WithSessionSupervision(config))
	target := createSession(t, h)
	installAbsentWorkSources(h.manager)
	supervisor := newPromptActivitySupervisor(t.Context(), h.manager, target,
		newPromptTurnDispatchState(target, "turn-heartbeat-timeout", TurnSourceUser, "hello"), config)
	supervisor.touch(now, runtimeActivityKindPromptStarted, "prompt started")
	if err := h.manager.Supervise(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		now = now.Add(time.Second)
		supervisor.report(
			acp.PromptActivityReport{
				Timestamp: now,
				Kind:      runtimeActivityKindAgentWaiting,
				Detail:    "waiting for provider",
			},
		)
		if err := h.manager.Supervise(t.Context(), now); err != nil {
			t.Fatal(err)
		}
	}
	outcome, err := h.manager.AwaitStopped(t.Context(), target.ID)
	if err != nil || !outcome.Verified || outcome.Cause != CauseInactivity {
		t.Fatalf("stop = %#v, %v", outcome, err)
	}
	meta := readMeta(t, target.MetaPath())
	if meta.StopReason == nil || *meta.StopReason != store.StopTimeout || meta.StopDetail != "inactivity" {
		t.Fatalf("stop reason = %#v, %q", meta.StopReason, meta.StopDetail)
	}
	for _, kind := range []string{eventspkg.SessionSupervisionWarning, eventspkg.SessionSupervisionStopped} {
		assertSupervisionEventCorrelation(t, h.manager, target, kind)
	}
}

func TestPromptActivitySupervisorProgressIsPersistedThroughPromptPump(t *testing.T) {
	h := newHarness(t,
		WithSessionSupervision(compozyconfig.SessionSupervisionConfig{
			ActivityHeartbeatInterval: time.Millisecond,
			ProgressNotifyInterval:    time.Millisecond,
			QuietAfter:                0,
			StopGrace:                 0,
			TimeoutCancelGrace:        time.Second,
		}),
	)
	session := createSession(t, h)
	source := make(chan acp.AgentEvent)
	h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		return source, nil
	}

	ctx, cancel := context.WithCancel(testutil.Context(t))
	events, err := h.manager.Prompt(ctx, session.ID, "long running")
	if err != nil {
		cancel()
		t.Fatalf("Prompt() error = %v", err)
	}
	defer cancel()

	progress := waitForPromptEvent(t, events, acp.EventTypeRuntimeProgress)
	if !strings.Contains(progress.Text, "Still working") {
		t.Fatalf("runtime progress text = %q, want Still working", progress.Text)
	}
	if progress.Runtime == nil {
		t.Fatal("runtime progress Runtime = nil, want activity payload")
	}

	source <- acp.AgentEvent{Type: acp.EventTypeDone}
	close(source)
	drainPromptEvents(t, events)
	stored := readStoredEvents(t, session)
	if !storedEventsContainType(stored, acp.EventTypeRuntimeProgress) {
		t.Fatalf("stored events = %#v, want runtime_progress persisted", stored)
	}

	meta := readMeta(t, session.MetaPath())
	if meta.Liveness != nil && meta.Liveness.Activity != nil {
		t.Fatalf("meta activity after prompt stream close = %#v, want cleared", meta.Liveness.Activity)
	}
}

// Invariant: warning is emitted once per quiet episode and real progress clears it. Owner: session; canonical activity suite.
func TestPromptActivitySupervisorWarningEmitsOnce(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	cfg := testSupervisionConfig()
	cfg.QuietAfter = time.Minute
	h := newHarness(t, WithNow(func() time.Time { return now }), WithSessionSupervision(cfg))
	target := createSession(t, h)
	installAbsentWorkSources(h.manager)
	for range 4 {
		if err := h.manager.Supervise(t.Context(), now); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Minute)
	}
	if countEventType(readStoredEvents(t, target), eventspkg.SessionSupervisionWarning) != 1 {
		t.Fatal("quiet episode must warn exactly once")
	}
	warning := target.Info().Supervision.QuietWarning
	if warning == nil || warning.StopAt != nil {
		t.Fatalf("warning-only policy = %#v", warning)
	}
	h.manager.recordWorkProgress(target, now)
	if target.Info().Supervision.QuietWarning != nil {
		t.Fatal("progress did not clear quiet warning")
	}
	now = now.Add(2*cfg.ActivityHeartbeatInterval + time.Second)
	if err := h.manager.Supervise(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := h.manager.Supervise(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	if countEventType(readStoredEvents(t, target), eventspkg.SessionSupervisionWarning) != 2 {
		t.Fatal("new quiet episode did not warn")
	}
	reportSessionStop(t, h, target.ID)
}

func installAbsentWorkSources(manager *Manager) {
	for _, name := range WorkSignalKindValues() {
		if WorkSignalKind(name) == WorkSignalAgentProgress {
			continue
		}
		manager.SetWorkSignalSources(
			WorkSignalSource{
				Kind:   WorkSignalKind(name),
				Source: SignalSourceFunc(func(context.Context, string) ([]WorkSignal, error) { return nil, nil }),
			},
		)
	}
}

func TestPromptActivityRuntimeEventDoesNotClearStallState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	h := newHarness(t, WithNow(func() time.Time { return now }))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	supervisor := newPromptActivitySupervisor(
		testutil.Context(t),
		h.manager,
		session,
		newPromptTurnDispatchState(session, "turn-stalled", TurnSourceUser, "hello"),
		testSupervisionConfig(),
	)
	supervisor.touch(now, runtimeActivityKindPromptStarted, "prompt started")
	session.markRuntimeStalled(store.SessionStallReasonActivityTimeout, now.Add(time.Second))

	supervisor.emitRuntimeEvent(acp.EventTypeRuntimeWarning, "Runtime activity timed out.", now.Add(2*time.Second), nil)

	meta := readMeta(t, session.MetaPath())
	if meta.Liveness == nil {
		t.Fatal("meta.Liveness = nil, want preserved stalled liveness")
	}
	if got, want := meta.Liveness.StallState, store.SessionStallStateDetected; got != want {
		t.Fatalf("meta.Liveness.StallState = %q, want %q", got, want)
	}
	if got, want := meta.Liveness.StallReason, store.SessionStallReasonActivityTimeout; got != want {
		t.Fatalf("meta.Liveness.StallReason = %q, want %q", got, want)
	}
	if meta.Liveness.Activity == nil {
		t.Fatal("meta.Liveness.Activity = nil, want runtime activity preserved")
	}
}

func TestPromptActivitySupervisorMarksUnhealthyProcessAsStalled(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	healthStore := newFakeSessionHealthStore()
	h := newHarness(t,
		WithNow(func() time.Time { return now }),
		WithSessionHealthStore(healthStore),
	)
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	supervisor := newPromptActivitySupervisor(
		testutil.Context(t),
		h.manager,
		session,
		newPromptTurnDispatchState(session, "turn-unhealthy", TurnSourceUser, "hello"),
		testSupervisionConfig(),
	)
	supervisor.touch(now, runtimeActivityKindPromptStarted, "prompt started")

	proc := h.driver.lastProcess()
	if proc == nil {
		t.Fatal("lastProcess() = nil, want process")
	}
	proc.setHealth(subprocess.HealthState{
		Healthy:             false,
		LastCheckedAt:       now.Add(5 * time.Second),
		ConsecutiveFailures: 2,
		LastError:           "health_check: context deadline exceeded",
	})

	supervisor.evaluate(now.Add(6 * time.Second))

	meta := readMeta(t, session.MetaPath())
	if meta.Liveness == nil {
		t.Fatal("meta.Liveness = nil, want stalled liveness")
	}
	if got, want := meta.Liveness.StallState, store.SessionStallStateDetected; got != want {
		t.Fatalf("meta.Liveness.StallState = %q, want %q", got, want)
	}
	if got, want := meta.Liveness.StallReason, store.SessionStallReasonProcessUnhealthy; got != want {
		t.Fatalf("meta.Liveness.StallReason = %q, want %q", got, want)
	}
	if meta.Liveness.Activity == nil || meta.Liveness.Activity.LastActivityAt == nil ||
		!meta.Liveness.Activity.LastActivityAt.Equal(now) {
		t.Fatalf("meta.Liveness.Activity = %#v, want original prompt activity timestamp", meta.Liveness.Activity)
	}

	warning := readRuntimeEvent(t, supervisor.eventsChannel())
	if got, want := warning.Type, acp.EventTypeRuntimeWarning; got != want {
		t.Fatalf("warning event type = %q, want %q", got, want)
	}
	if !strings.Contains(warning.Text, "Runtime health check failed") {
		t.Fatalf("warning.Text = %q, want health failure text", warning.Text)
	}

	health, err := h.manager.storedSessionHealth(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("storedSessionHealth() error = %v", err)
	}
	if health.State != heartbeat.SessionHealthStatePrompting || health.Health != heartbeat.SessionHealthDegraded {
		t.Fatalf("storedSessionHealth() = %#v, want prompting/degraded", health)
	}
}

func TestPromptActivitySupervisorIgnoresSyntheticHeartbeatWhenProcessIsUnhealthy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	healthStore := newFakeSessionHealthStore()
	h := newHarness(t,
		WithNow(func() time.Time { return now }),
		WithSessionHealthStore(healthStore),
	)
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	supervisor := newPromptActivitySupervisor(
		testutil.Context(t),
		h.manager,
		session,
		newPromptTurnDispatchState(session, "turn-heartbeat", TurnSourceUser, "hello"),
		testSupervisionConfig(),
	)
	supervisor.touch(now, runtimeActivityKindPromptStarted, "prompt started")

	proc := h.driver.lastProcess()
	if proc == nil {
		t.Fatal("lastProcess() = nil, want process")
	}
	proc.setHealth(subprocess.HealthState{
		Healthy:             false,
		LastCheckedAt:       now.Add(5 * time.Second),
		ConsecutiveFailures: 2,
		LastError:           "health_check: context deadline exceeded",
	})

	supervisor.report(acp.PromptActivityReport{
		Timestamp: now.Add(10 * time.Second),
		Kind:      runtimeActivityKindAgentWaiting,
		Detail:    "waiting for provider",
	})

	meta := readMeta(t, session.MetaPath())
	if meta.Liveness == nil {
		t.Fatal("meta.Liveness = nil, want stalled liveness")
	}
	if got, want := meta.Liveness.StallReason, store.SessionStallReasonProcessUnhealthy; got != want {
		t.Fatalf("meta.Liveness.StallReason = %q, want %q", got, want)
	}
	if meta.Liveness.Activity == nil {
		t.Fatal("meta.Liveness.Activity = nil, want original activity preserved")
	}
	if got, want := meta.Liveness.Activity.LastActivityKind, runtimeActivityKindPromptStarted; got != want {
		t.Fatalf("meta.Liveness.Activity.LastActivityKind = %q, want %q", got, want)
	}
	if meta.Liveness.Activity.LastActivityAt == nil || !meta.Liveness.Activity.LastActivityAt.Equal(now) {
		t.Fatalf(
			"meta.Liveness.Activity.LastActivityAt = %#v, want original prompt activity timestamp",
			meta.Liveness.Activity.LastActivityAt,
		)
	}

	warning := readRuntimeEvent(t, supervisor.eventsChannel())
	if got, want := warning.Type, acp.EventTypeRuntimeWarning; got != want {
		t.Fatalf("warning event type = %q, want %q", got, want)
	}
}

func TestPromptActivitySupervisorIgnoresUnknownProcessHealthSnapshot(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	h := newHarness(t, WithNow(func() time.Time { return now }))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	supervisor := newPromptActivitySupervisor(
		testutil.Context(t),
		h.manager,
		session,
		newPromptTurnDispatchState(session, "turn-unknown-health", TurnSourceUser, "hello"),
		testSupervisionConfig(),
	)
	supervisor.touch(now, runtimeActivityKindPromptStarted, "prompt started")

	proc := h.driver.lastProcess()
	if proc == nil {
		t.Fatal("lastProcess() = nil, want process")
	}
	proc.setHealth(subprocess.HealthState{})

	supervisor.evaluate(now.Add(5 * time.Second))

	meta := readMeta(t, session.MetaPath())
	if meta.Liveness == nil {
		t.Fatal("meta.Liveness = nil, want liveness")
	}
	if got := meta.Liveness.StallState; got != "" {
		t.Fatalf("meta.Liveness.StallState = %q, want empty", got)
	}
	if got := meta.Liveness.StallReason; got != "" {
		t.Fatalf("meta.Liveness.StallReason = %q, want empty", got)
	}

	select {
	case event := <-supervisor.eventsChannel():
		t.Fatalf("unexpected runtime event for unknown health snapshot: %#v", event)
	default:
	}
}

// Invariant: quiet grace settles through the shared verified stop ladder and emits a truthful terminal event.
// Owner: session lifecycle; this existing activity suite owns the supervision trigger.
func TestPromptActivitySupervisorTimeoutCancelsThenStopsSession(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	cfg := testSupervisionConfig()
	cfg.QuietAfter, cfg.StopGrace = time.Second, time.Second
	h := newHarness(t, WithNow(func() time.Time { return now }), WithSessionSupervision(cfg))
	target := createSession(t, h)
	installAbsentWorkSources(h.manager)
	for range 2 {
		if err := h.manager.Supervise(t.Context(), now); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Second)
	}
	if countEventType(readStoredEvents(t, target), eventspkg.SessionSupervisionStopped) != 0 {
		t.Fatal("supervision stopped before settlement")
	}
	if err := h.manager.Supervise(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	outcome, err := h.manager.AwaitStopped(t.Context(), target.ID)
	if err != nil || !outcome.Verified || outcome.Cause != CauseInactivity {
		t.Fatalf("stop = %#v, %v", outcome, err)
	}
	if h.driver.cancelCalls != 1 || h.driver.stopCalls != 1 {
		t.Fatalf("cancel/stop = %d/%d", h.driver.cancelCalls, h.driver.stopCalls)
	}
	if countEventType(readStoredEvents(t, target), eventspkg.SessionSupervisionStopped) != 1 {
		t.Fatal("missing unique supervision settlement")
	}
}

func TestPromptActivitySupervisorRecordsRecoveredMarker(t *testing.T) {
	t.Parallel()

	t.Run("Should persist recovered marker when activity clears a stalled session", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 22, 10, 15, 0, 0, time.UTC)
		h := newHarness(t, WithNow(func() time.Time { return now }))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})

		supervisor := newPromptActivitySupervisor(
			testutil.Context(t),
			h.manager,
			session,
			newPromptTurnDispatchState(session, "turn-recovered", TurnSourceUser, "hello"),
			testSupervisionConfig(),
		)
		session.markRuntimeStalled(store.SessionStallReasonProcessUnhealthy, now)
		supervisor.touch(now.Add(time.Second), runtimeActivityKindAgentWaiting, "provider responded")

		meta := readMeta(t, session.MetaPath())
		if meta.Liveness == nil {
			t.Fatal("meta.Liveness = nil, want liveness after recovery")
		}
		if meta.Liveness.StallState != "" || meta.Liveness.StallReason != "" {
			t.Fatalf("meta.Liveness stall = %#v, want cleared after recovery", meta.Liveness)
		}
		marker := requireTranscriptMarker(t, h.manager, session.ID, transcript.MarkerSessionRecovered)
		if got, want := marker.Evidence["stall_reason"], store.SessionStallReasonProcessUnhealthy; got != want {
			t.Fatalf("recovered marker stall_reason = %#v, want %q", got, want)
		}
	})

	t.Run("Should emit one recovered marker for repeated activity after one stall", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 22, 10, 20, 0, 0, time.UTC)
		h := newHarness(t, WithNow(func() time.Time { return now }))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})

		supervisor := newPromptActivitySupervisor(
			testutil.Context(t),
			h.manager,
			session,
			newPromptTurnDispatchState(session, "turn-recovered-once", TurnSourceUser, "hello"),
			testSupervisionConfig(),
		)
		session.markRuntimeStalled(store.SessionStallReasonActivityTimeout, now)
		supervisor.touch(now.Add(time.Second), runtimeActivityKindAgentWaiting, "provider responded")
		supervisor.touch(now.Add(2*time.Second), runtimeActivityKindAgentWaiting, "provider responded again")

		got := countTranscriptMarkers(
			t,
			h.manager,
			session.ID,
			transcript.MarkerSessionRecovered,
		)
		if want := 1; got != want {
			t.Fatalf("recovered marker count = %d, want %d", got, want)
		}
	})
}

func TestPromptActivitySupervisorPromptDeadlineStopsWithDeadlineDetail(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	h := newHarness(t, WithNow(func() time.Time { return now }))
	session := createSession(t, h)
	session.setCurrentTurnSource(TurnSourceUser)
	session.setCurrentPromptMeta(acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser})

	config := testSupervisionConfig()
	config.TimeoutCancelGrace = 200 * time.Millisecond
	supervisor := newPromptActivitySupervisor(
		testutil.Context(t),
		h.manager,
		session,
		newPromptTurnDispatchState(session, "turn-deadline", TurnSourceUser, "hello"),
		config,
	)
	deadline := now.Add(time.Second)
	supervisor.deadlineAt = &deadline
	supervisor.touch(now, runtimeActivityKindPromptStarted, "prompt started")

	done := make(chan struct{})
	go func() {
		defer close(done)
		supervisor.handlePromptDeadline(now.Add(2 * time.Second))
	}()

	warning := readRuntimeEvent(t, supervisor.eventsChannel())
	if got, want := warning.Type, acp.EventTypeRuntimeWarning; got != want {
		t.Fatalf("warning event type = %q, want %q", got, want)
	}
	if warning.Runtime == nil {
		t.Fatal("warning.Runtime = nil, want runtime activity payload")
	}
	if warning.Runtime.DeadlineAt == nil || !warning.Runtime.DeadlineAt.Equal(deadline) {
		t.Fatalf("warning.Runtime.DeadlineAt = %#v, want %s", warning.Runtime.DeadlineAt, deadline)
	}
	if got, want := warning.Runtime.ElapsedMS, int64(2000); got != want {
		t.Fatalf("warning.Runtime.ElapsedMS = %d, want %d", got, want)
	}

	var raw map[string]any
	if err := json.Unmarshal(warning.Raw, &raw); err != nil {
		t.Fatalf("json.Unmarshal(warning.Raw) error = %v", err)
	}
	if got, want := raw["deadline_at"], deadline.Format(time.RFC3339Nano); got != want {
		t.Fatalf("warning.Raw deadline_at = %#v, want %#v", got, want)
	}
	if got, want := raw["elapsed_ms"], float64(2000); got != want {
		t.Fatalf("warning.Raw elapsed_ms = %#v, want %#v", got, want)
	}
	if got := h.driver.cancelCalls; got != 0 {
		t.Fatalf("driver cancel calls before runtime warning ack = %d, want 0", got)
	}
	if got := h.driver.stopCalls; got != 0 {
		t.Fatalf("driver stop calls before runtime warning ack = %d, want 0", got)
	}

	supervisor.ackPromptDeadlineWarning(warning)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handlePromptDeadline() did not return after runtime warning ack")
	}

	if got := h.driver.cancelCalls; got != 1 {
		t.Fatalf("driver cancel calls = %d, want 1", got)
	}
	if got := h.driver.stopCalls; got != 1 {
		t.Fatalf("driver stop calls = %d, want 1", got)
	}

	meta := readMeta(t, session.MetaPath())
	if meta.StopReason == nil || *meta.StopReason != store.StopTimeout {
		t.Fatalf("meta.StopReason = %#v, want %q", meta.StopReason, store.StopTimeout)
	}
	if got, want := meta.StopDetail, store.SessionStallReasonPromptDeadlineExceeded; got != want {
		t.Fatalf("meta.StopDetail = %q, want %q", got, want)
	}
	if meta.Liveness == nil || meta.Liveness.Activity != nil {
		t.Fatalf("meta.Liveness = %#v, want cleared activity after forced stop", meta.Liveness)
	}
}

func TestPromptActivitySupervisorTimeoutStopDeadline(t *testing.T) {
	t.Parallel()

	defaultGrace := compozyconfig.DefaultSessionSupervisionConfig().TimeoutCancelGrace
	testCases := []struct {
		name       string
		supervisor *promptActivitySupervisor
		want       time.Duration
	}{
		{
			name:       "Should use default timeout cancel grace for nil supervisor",
			supervisor: nil,
			want:       defaultGrace,
		},
		{
			name: "Should use default timeout cancel grace for zero configured grace",
			supervisor: &promptActivitySupervisor{
				config: compozyconfig.SessionSupervisionConfig{},
			},
			want: defaultGrace,
		},
		{
			name: "Should use default timeout cancel grace for negative configured grace",
			supervisor: &promptActivitySupervisor{
				config: compozyconfig.SessionSupervisionConfig{
					TimeoutCancelGrace: -time.Millisecond,
				},
			},
			want: defaultGrace,
		},
		{
			name: "Should use lifecycle minimum for short configured grace",
			supervisor: &promptActivitySupervisor{
				config: compozyconfig.SessionSupervisionConfig{
					TimeoutCancelGrace: 42 * time.Millisecond,
				},
			},
			want: defaultLifecycleTimeout,
		},
		{
			name: "Should use configured timeout cancel grace above lifecycle minimum",
			supervisor: &promptActivitySupervisor{
				config: compozyconfig.SessionSupervisionConfig{
					TimeoutCancelGrace: defaultLifecycleTimeout + time.Second,
				},
			},
			want: defaultLifecycleTimeout + time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.supervisor.timeoutStopDeadline(); got != tc.want {
				t.Fatalf("timeoutStopDeadline() = %s, want %s", got, tc.want)
			}
		})
	}
}

func testSupervisionConfig() compozyconfig.SessionSupervisionConfig {
	return compozyconfig.SessionSupervisionConfig{
		ActivityHeartbeatInterval: time.Hour,
		ProgressNotifyInterval:    0,
		QuietAfter:                0,
		StopGrace:                 0,
		TimeoutCancelGrace:        time.Second,
	}
}

func waitForPromptEvent(t *testing.T, events <-chan acp.AgentEvent, eventType string) acp.AgentEvent {
	t.Helper()

	deadline := time.After(time.Second)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatalf("prompt events closed before %s", eventType)
			}
			if event.Type == eventType {
				return event
			}
		case <-deadline:
			t.Fatalf("timed out waiting for prompt event %s", eventType)
		}
	}
}

func readRuntimeEvent(t *testing.T, events <-chan acp.AgentEvent) acp.AgentEvent {
	t.Helper()

	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for runtime event")
	}
	return acp.AgentEvent{}
}

func drainPromptEvents(t *testing.T, events <-chan acp.AgentEvent) {
	t.Helper()

	for {
		select {
		case _, ok := <-events:
			if !ok {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for prompt event channel to close")
		}
	}
}

func requireTranscriptMarker(t *testing.T, manager *Manager, sessionID string, kind string) transcript.Marker {
	t.Helper()

	eventsList, err := manager.Events(
		testutil.Context(t),
		sessionID,
		store.EventQuery{Type: eventspkg.TranscriptMarkerCreated},
	)
	if err != nil {
		t.Fatalf("Events(transcript marker) error = %v", err)
	}
	for _, event := range eventsList {
		agentEvent, err := transcript.UnmarshalAgentEvent(event.Content)
		if err != nil {
			t.Fatalf("transcript.UnmarshalAgentEvent(%s) error = %v", event.ID, err)
		}
		marker, ok := transcript.ParseMarker(agentEvent.Raw)
		if ok && marker.Kind == kind {
			return marker.Normalize()
		}
	}
	t.Fatalf("transcript marker %q not found in %#v", kind, eventsList)
	return transcript.Marker{}
}

func countTranscriptMarkers(t *testing.T, manager *Manager, sessionID string, kind string) int {
	t.Helper()

	eventsList, err := manager.Events(
		testutil.Context(t),
		sessionID,
		store.EventQuery{Type: eventspkg.TranscriptMarkerCreated},
	)
	if err != nil {
		t.Fatalf("Events(transcript marker) error = %v", err)
	}
	count := 0
	for _, event := range eventsList {
		agentEvent, err := transcript.UnmarshalAgentEvent(event.Content)
		if err != nil {
			t.Fatalf("transcript.UnmarshalAgentEvent(%s) error = %v", event.ID, err)
		}
		marker, ok := transcript.ParseMarker(agentEvent.Raw)
		if ok && marker.Kind == kind {
			count++
		}
	}
	return count
}

func storedEventsContainType(events []store.SessionEvent, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

// Invariant: only fresh authoritative evidence protects a session; source failures remain explicit.
// Owner: session signal aggregation; canonical activity/supervision suite (UT-058..061, UT-123..126/130).
func TestWorkSignalRegistryFreshnessAndUnknownSources(t *testing.T) {
	t.Run("Should preserve real attention during fresh progress and clear it after recovery", func(t *testing.T) {
		now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
		registry := NewWorkSignalRegistry()
		attention := "Loop node is quarantined; retry after repairing the failure."
		for _, kind := range WorkSignalKindValues() {
			registry.Register(WorkSignalSource{Kind: WorkSignalKind(kind), Source: SignalSourceFunc(
				func(context.Context, string) ([]WorkSignal, error) { return nil, nil },
			)})
		}
		for _, kind := range []WorkSignalKind{WorkSignalAgentProgress, WorkSignalLoopRun} {
			registry.Register(
				WorkSignalSource{
					Kind: kind,
					Source: SignalSourceFunc(func(context.Context, string) ([]WorkSignal, error) {
						signal := WorkSignal{Kind: kind, Since: now, ValidUntil: now.Add(time.Minute)}
						if kind == WorkSignalLoopRun {
							signal.AttentionReason = attention
						}
						return []WorkSignal{signal}, nil
					}),
				},
			)
		}
		if state := registry.Inspect(
			t.Context(),
			"session",
			now,
		); !supervisionNeedsAttention(state) ||
			len(state.WorkSignals) != 2 {
			t.Fatalf("fresh work hid real attention: %#v", state)
		}
		attention = ""
		if state := registry.Inspect(t.Context(), "session", now); supervisionNeedsAttention(state) {
			t.Fatalf("recovered run retained attention: %#v", state)
		}
	})

	now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	for _, name := range WorkSignalKindValues() {
		t.Run(name, func(t *testing.T) {
			kind := WorkSignalKind(name)
			registry := NewWorkSignalRegistry()
			for _, sourceKind := range WorkSignalKindValues() {
				registry.Register(
					WorkSignalSource{
						Kind: WorkSignalKind(sourceKind),
						Source: SignalSourceFunc(
							func(context.Context, string) ([]WorkSignal, error) { return nil, nil },
						),
					},
				)
			}
			registry.Register(
				WorkSignalSource{
					Kind: kind,
					Source: SignalSourceFunc(func(context.Context, string) ([]WorkSignal, error) {
						return []WorkSignal{
							{
								Kind:       kind,
								Since:      now.Add(-time.Minute),
								ValidUntil: now,
								Ref:        "work",
								StaleAttention: kind == WorkSignalToolRunning || kind == WorkSignalActiveChild ||
									kind == WorkSignalLoopRun,
							},
						}, nil
					}),
				},
			)
			atBoundary := registry.Inspect(t.Context(), "session", now)
			strict := kind == WorkSignalTaskLease || kind == WorkSignalScheduledWait
			if (len(atBoundary.WorkSignals) == 0) != strict {
				t.Fatalf("boundary evidence = %#v", atBoundary)
			}
			stale := registry.Inspect(t.Context(), "session", now.Add(time.Nanosecond))
			if len(stale.WorkSignals) != 0 {
				t.Fatal("stale evidence remains present")
			}
			wantAttention := kind == WorkSignalToolRunning || kind == WorkSignalActiveChild || kind == WorkSignalLoopRun
			if supervisionNeedsAttention(stale) != wantAttention {
				t.Fatalf("stale attention = %#v", stale)
			}
			registry.Register(
				WorkSignalSource{
					Kind: kind,
					Source: SignalSourceFunc(
						func(context.Context, string) ([]WorkSignal, error) { return nil, errors.New("inspection failed") },
					),
				},
			)
			unknown := registry.Inspect(t.Context(), "session", now)
			if len(unknown.WorkSignals) != 0 || !supervisionNeedsAttention(unknown) {
				t.Fatalf("unknown = %#v", unknown)
			}
		})
	}
}

// Invariant: zero quiet disables both actions, and unknown inspection cannot trigger an automatic stop.
// Owner: session; canonical activity/supervision suite (UT-064, IT-025 decision boundary).
func TestSupervisionDisableAndSourceFailure(t *testing.T) {
	for _, mode := range []string{"disabled", "unknown", "valid", "stale"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
			cfg := testSupervisionConfig()
			cfg.QuietAfter, cfg.StopGrace = time.Second, time.Second
			if mode == "disabled" {
				cfg.QuietAfter = 0
			}
			h := newHarness(t, WithNow(func() time.Time { return now }), WithSessionSupervision(cfg))
			target := createSession(t, h)
			installAbsentWorkSources(h.manager)
			fixed := now
			if mode != "disabled" {
				h.manager.SetWorkSignalSources(
					WorkSignalSource{
						Kind: WorkSignalToolRunning,
						Source: SignalSourceFunc(func(context.Context, string) ([]WorkSignal, error) {
							if mode == "unknown" {
								return nil, errors.New("registry unavailable")
							}
							until := fixed.Add(time.Hour)
							if mode == "stale" {
								until = fixed.Add(-time.Second)
							}
							return []WorkSignal{
								{
									Kind:           WorkSignalToolRunning,
									Since:          fixed.Add(-time.Minute),
									ValidUntil:     until,
									Ref:            "tool",
									StaleAttention: true,
								},
							}, nil
						}),
					},
				)
			}
			for range 3 {
				if err := h.manager.Supervise(t.Context(), now); err != nil {
					t.Fatal(err)
				}
				if mode != "stale" {
					now = now.Add(time.Minute)
				} else if target.Info().State == StateActive {
					now = now.Add(time.Second)
				}
			}
			if mode == "stale" {
				outcome, err := h.manager.AwaitStopped(t.Context(), target.ID)
				if err != nil || !outcome.Verified {
					t.Fatalf("stale work did not stop: %#v, %v", outcome, err)
				}
				return
			}
			if target.Info().State != StateActive || target.Info().Supervision.QuietWarning != nil {
				t.Fatalf("unexpected action: %#v", target.Info())
			}
			if mode == "unknown" {
				assertSupervisionEventCorrelation(t, h.manager, target, eventspkg.SessionSupervisionSourceError)
				if BadgeForInfo(target.Info()) != BadgeNeedsAttention {
					t.Fatal("source error did not surface attention")
				}
				if countEventType(readStoredEvents(t, target), eventspkg.SessionSupervisionSourceError) != 1 {
					t.Fatal("source error must emit once while unchanged")
				}
			}
			reportSessionStop(t, h, target.ID)
		})
	}
}

// Invariant: retrying a failed warning append keeps the same episode identity.
// Owner: session supervision; canonical activity suite, recorder I/O boundary.
func TestSupervisionWarningRetriesSameEpisode(t *testing.T) {
	for _, afterCommit := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should retry once after commit %t", afterCommit), func(t *testing.T) {
			now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
			cfg := testSupervisionConfig()
			cfg.QuietAfter, cfg.StopGrace = time.Second, time.Hour
			h := newHarness(t, WithNow(func() time.Time { return now }), WithSessionSupervision(cfg))
			target := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, target.ID) })
			installAbsentWorkSources(h.manager)
			recorder := &quietWarningFailingRecorder{EventRecorder: target.recorderHandle(), afterCommit: afterCommit}
			recorder.fail.Store(true)
			target.mu.Lock()
			target.recorder = recorder
			target.mu.Unlock()
			if err := h.manager.Supervise(t.Context(), now); err != nil {
				t.Fatal(err)
			}
			now = now.Add(time.Second)
			if err := h.manager.Supervise(t.Context(), now); err == nil {
				t.Fatal("warning persistence failure was hidden")
			}
			first := target.Info().Supervision.QuietWarning
			if first == nil {
				t.Fatal("warning episode was discarded")
			}
			recorder.fail.Store(false)
			now = now.Add(time.Second)
			for range 2 {
				if err := h.manager.Supervise(t.Context(), now); err != nil {
					t.Fatal(err)
				}
			}
			current := target.Info().Supervision.QuietWarning
			if current == nil || !current.WarnedAt.Equal(first.WarnedAt) {
				t.Fatalf("warning changed identity: %+v", current)
			}
			if countEventType(readStoredEvents(t, target), eventspkg.SessionSupervisionWarning) != 1 {
				t.Fatal("warning was duplicated or lost")
			}
		})
	}
}

type quietWarningFailingRecorder struct {
	EventRecorder
	fail        atomic.Bool
	afterCommit bool
}

func (r *quietWarningFailingRecorder) AppendEventIfAbsent(
	ctx context.Context,
	event store.SessionEvent,
) (store.SessionEvent, error) {
	failing := event.Type == eventspkg.SessionSupervisionWarning && r.fail.Load()
	if failing && !r.afterCommit {
		return store.SessionEvent{}, errors.New("warning persistence failed")
	}
	persisted, err := recordIdempotentSessionEvent(ctx, r.EventRecorder, event)
	if failing {
		return persisted, errors.Join(err, errors.New("warning acknowledgement lost"))
	}
	return persisted, err
}

// Canonical supervision event coverage: lifecycle and failure paths above own emissions.
func assertSupervisionEventCorrelation(t *testing.T, manager *Manager, target *Session, kind string) {
	t.Helper()
	rows, err := manager.Events(t.Context(), target.ID, store.EventQuery{Type: kind})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("%s event count = %d", kind, len(rows))
	}
	event, err := transcript.UnmarshalAgentEvent(rows[0].Content)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(event.Raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["workspace_id"] != target.WorkspaceID || payload["session_id"] != target.ID ||
		payload["turn_id"] != rows[0].TurnID ||
		rows[0].TurnID == "" ||
		payload["actor_id"] != "daemon" ||
		payload["actor_kind"] != "system" ||
		event.ActorID != "daemon" ||
		event.ActorKind != "system" {
		t.Fatalf("%s correlation = %+v / %+v", kind, payload, event.EventCorrelation)
	}
}

func TestPromptActivitySupervisorEventBatch(t *testing.T) {
	t.Run("Should preserve tool transitions and latest activity across a batch", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
		h := newHarness(t, WithNow(func() time.Time { return now }))
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		supervisor := newPromptActivitySupervisor(
			testutil.Context(t), h.manager, sess,
			newPromptTurnDispatchState(sess, "turn-batch", TurnSourceUser, "batch"), testSupervisionConfig(),
		)
		supervisor.observeEvent(acp.AgentEvent{
			Type: acp.EventTypeToolCall, Title: "Old tool", ToolCallID: "old", Timestamp: now,
		})
		supervisor.observeEventBatch([]acp.AgentEvent{
			{Type: acp.EventTypeToolResult, ToolCallID: "old", Timestamp: now.Add(time.Second)},
			{Type: acp.EventTypeToolCall, ToolCallID: "new", Timestamp: now.Add(2 * time.Second)},
			{Type: acp.EventTypeAgentMessage, Text: "Still working", Timestamp: now.Add(3 * time.Second)},
		})
		meta := readMeta(t, sess.MetaPath())
		if meta.Liveness == nil || meta.Liveness.Activity == nil {
			t.Fatal("batch activity was not persisted")
		}
		activity := meta.Liveness.Activity
		if activity.CurrentTool != "" || activity.ToolCallID != "new" ||
			activity.LastActivityKind != acp.EventTypeAgentMessage || activity.LastActivityDetail != "Still working" ||
			activity.LastActivityAt == nil || !activity.LastActivityAt.Equal(now.Add(3*time.Second)) {
			t.Fatalf("batch activity = %#v, want newest tool identity without stale title and latest prose", activity)
		}
		supervisor.observeEventBatch([]acp.AgentEvent{
			{Type: acp.EventTypeToolResult, ToolCallID: "new", Timestamp: now.Add(4 * time.Second)},
			{Type: acp.EventTypeThought, Text: "Next step", Timestamp: now.Add(5 * time.Second)},
		})
		activity = readMeta(t, sess.MetaPath()).Liveness.Activity
		if activity.CurrentTool != "" || activity.ToolCallID != "" ||
			activity.LastActivityKind != acp.EventTypeThought {
			t.Fatalf("completed batch activity = %#v, want tool cleared with latest thought", activity)
		}
	})
}
