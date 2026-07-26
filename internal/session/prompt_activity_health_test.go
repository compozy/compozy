package session

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
)

func TestPromptActivitySupervisorHealthContract(t *testing.T) {
	t.Parallel()

	t.Run("Should persist unhealthy process diagnostics in session health", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
		healthStore := newFakeSessionHealthStore()
		h := newHarness(t,
			WithNow(func() time.Time { return now }),
			WithSessionHealthStore(healthStore),
		)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
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
			Message:             "provider health probe failed",
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

		health, err := h.manager.storedSessionHealth(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("storedSessionHealth() error = %v", err)
		}
		if health.State != heartbeat.SessionHealthStatePrompting || health.Health != heartbeat.SessionHealthDegraded {
			t.Fatalf("storedSessionHealth() = %#v, want prompting/degraded", health)
		}
		if !strings.Contains(health.LastError, "provider health probe failed") ||
			!strings.Contains(health.LastError, "health_check: context deadline exceeded") {
			t.Fatalf("storedSessionHealth().LastError = %q, want unhealthy process diagnostic", health.LastError)
		}
	})

	t.Run("Should publish immutable unhealthy subprocess snapshots", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC)
		notifier := &subprocessHealthRecordingNotifier{}
		h := newHarness(t,
			WithNow(func() time.Time { return now }),
			WithNotifier(notifier),
		)
		sess := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop(%q) error = %v", sess.ID, err)
			}
		})

		supervisor := newPromptActivitySupervisor(
			testutil.Context(t),
			h.manager,
			sess,
			newPromptTurnDispatchState(sess, "turn-health-notify", TurnSourceUser, "hello"),
			testSupervisionConfig(),
		)
		supervisor.touch(now, runtimeActivityKindPromptStarted, "prompt started")

		proc := h.driver.lastProcess()
		if proc == nil {
			t.Fatal("lastProcess() = nil, want process")
		}
		proc.setHealth(subprocess.HealthState{
			Healthy:             false,
			Message:             "provider health probe failed",
			Details:             json.RawMessage(`{"reason":"unresponsive"}`),
			LastCheckedAt:       now.Add(5 * time.Second),
			ConsecutiveFailures: 3,
			LastError:           "health_check: context deadline exceeded",
		})

		supervisor.evaluate(now.Add(6 * time.Second))

		observations := notifier.subprocessHealthObservations()
		if got, want := len(observations), 1; got != want {
			t.Fatalf("subprocess health observation count = %d, want %d", got, want)
		}
		observation := observations[0]
		if observation.SessionID != sess.ID || observation.WorkspaceID != sess.WorkspaceID ||
			observation.AgentName != sess.AgentName || observation.Health.ConsecutiveFailures != 3 {
			t.Fatalf("subprocess health observation = %#v, want correlated third failure", observation)
		}

		snapshots := h.manager.SubprocessHealthSnapshots()
		if got, want := len(snapshots), 1; got != want {
			t.Fatalf("SubprocessHealthSnapshots() count = %d, want %d", got, want)
		}
		snapshots[0].Health.Details[0] = '['
		fresh := h.manager.SubprocessHealthSnapshots()
		if got, want := string(fresh[0].Health.Details), `{"reason":"unresponsive"}`; got != want {
			t.Fatalf("immutable health details = %q, want %q", got, want)
		}
	})
}

type subprocessHealthRecordingNotifier struct {
	mu           sync.Mutex
	observations []SubprocessHealthSnapshot
}

func (*subprocessHealthRecordingNotifier) OnSessionCreated(context.Context, *Session) {}

func (*subprocessHealthRecordingNotifier) OnSessionStopped(context.Context, *Session) {}

func (*subprocessHealthRecordingNotifier) OnAgentEvent(context.Context, string, any) {}

func (n *subprocessHealthRecordingNotifier) OnSubprocessHealth(
	_ context.Context,
	observation SubprocessHealthSnapshot,
) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.observations = append(n.observations, observation)
}

func (n *subprocessHealthRecordingNotifier) subprocessHealthObservations() []SubprocessHealthSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]SubprocessHealthSnapshot(nil), n.observations...)
}
