package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestNetworkWakeRunner(t *testing.T) {
	t.Parallel()

	t.Run("Should reject shutdown without a context", func(t *testing.T) {
		t.Parallel()

		fixture := newNetworkWakeRunnerFixture(t)
		var shutdownCtx context.Context
		err := fixture.runner.Shutdown(shutdownCtx)
		if err == nil || !strings.Contains(err.Error(), "shutdown context is required") {
			t.Fatalf("Shutdown(nil) error = %v, want context-required error", err)
		}
	})

	t.Run("Should claim prompt and settle one committed wake with actual usage", func(t *testing.T) {
		t.Parallel()

		fixture := newNetworkWakeRunnerFixture(t)
		inputTokens := int64(17)
		outputTokens := int64(5)
		fixture.prompter.events = []acp.AgentEvent{{
			Type: acp.EventTypeDone,
			Usage: &acp.TokenUsage{
				InputTokens:  &inputTokens,
				OutputTokens: &outputTokens,
			},
		}}

		if _, err := fixture.runner.processNotification(
			t.Context(),
			fixture.actor,
			fixture.notification,
		); err != nil {
			t.Fatalf("processNotification() error = %v", err)
		}

		if got := fixture.tasks.claimCalls; got != 1 {
			t.Fatalf("claim calls = %d, want 1", got)
		}
		if got := fixture.prompter.calls; got != 1 {
			t.Fatalf("PromptNetwork calls = %d, want 1", got)
		}
		if got := len(fixture.tasks.settlements); got != 1 {
			t.Fatalf("settlement calls = %d, want 1", got)
		}
		settled := fixture.tasks.settledOutcome(t)
		if got := settled.ActualInputTokens + settled.ActualOutputTokens; got != inputTokens+outputTokens {
			t.Fatalf("settlement tokens = %d, want %d", got, inputTokens+outputTokens)
		}
		if settled.State != store.NetworkWakeStateSucceeded || settled.UsageState != store.NetworkWakeUsageActual {
			t.Fatalf("settled outcome = %#v, want succeeded actual usage", settled)
		}
		if settled.ActualInputTokens != inputTokens || settled.ActualOutputTokens != outputTokens {
			t.Fatalf("settled tokens = (%d,%d), want (%d,%d)",
				settled.ActualInputTokens,
				settled.ActualOutputTokens,
				inputTokens,
				outputTokens,
			)
		}
	})

	t.Run("Should release the lease when the terminal transition fails", func(t *testing.T) {
		t.Parallel()

		fixture := newNetworkWakeRunnerFixture(t)
		terminalErr := errors.New("terminal transition unavailable")
		fixture.tasks.settlementErr = terminalErr
		fixture.prompter.events = []acp.AgentEvent{{Type: acp.EventTypeDone}}

		_, err := fixture.runner.processNotification(t.Context(), fixture.actor, fixture.notification)
		if !errors.Is(err, terminalErr) {
			t.Fatalf("processNotification() error = %v, want terminal transition error", err)
		}
		if got := len(fixture.tasks.releases); got != 1 {
			t.Fatalf("release calls = %d, want 1", got)
		}
		if _, err := fixture.tasks.ClaimNextRun(t.Context(), taskpkg.ClaimCriteria{
			RunID:            fixture.notification.TaskRunID,
			TargetSessionID:  fixture.notification.RecipientSessionID,
			ClaimerSessionID: fixture.notification.RecipientSessionID,
		}, fixture.actor); err != nil {
			t.Fatalf("ClaimNextRun() after cleanup error = %v", err)
		}
	})

	t.Run("Should recover a queued durable wake when the runner starts", func(t *testing.T) {
		t.Parallel()

		fixture := newNetworkWakeRunnerFixture(t)
		if got := fixture.runner.State(); got != networkWakeRunnerConstructed {
			t.Fatalf("State() = %q, want %q", got, networkWakeRunnerConstructed)
		}
		select {
		case <-fixture.runner.Ready():
			t.Fatal("Ready() closed before Start")
		default:
		}
		fixture.store.queued = []store.CommittedNetworkNotification{fixture.notification}
		fixture.prompter.events = []acp.AgentEvent{{Type: acp.EventTypeDone}}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		if err := fixture.runner.Start(ctx); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		select {
		case <-fixture.runner.Ready():
		case <-time.After(time.Second):
			t.Fatal("Ready() did not close after recovery")
		}
		if got := fixture.runner.State(); got != networkWakeRunnerReady {
			t.Fatalf("State() = %q, want %q", got, networkWakeRunnerReady)
		}
		fixture.tasks.waitSettled(t)
		shutdownCtx, shutdownCancel := context.WithTimeout(t.Context(), time.Second)
		defer shutdownCancel()
		if err := fixture.runner.Shutdown(shutdownCtx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		if got := fixture.runner.State(); got != networkWakeRunnerStopped {
			t.Fatalf("State() after Shutdown = %q, want %q", got, networkWakeRunnerStopped)
		}
		if got := fixture.prompter.calls; got != 1 {
			t.Fatalf("PromptNetwork calls after recovery = %d, want 1", got)
		}
		if got := fixture.prompter.resumeCalls; got != 1 {
			t.Fatalf("Resume calls after recovery = %d, want 1", got)
		}
	})

	t.Run(
		"Should release and retry prompt contention without settling or canceling another prompt",
		func(t *testing.T) {
			t.Parallel()

			fixture := newNetworkWakeRunnerFixture(t)
			fixture.store.queued = []store.CommittedNetworkNotification{fixture.notification}
			fixture.prompter.events = []acp.AgentEvent{{Type: acp.EventTypeDone}}
			fixture.prompter.resume = func(context.Context, string, int) (*session.Session, error) {
				if fixture.prompter.resumeCalls == 1 {
					return nil, session.ErrPromptInProgress
				}
				return nil, nil
			}

			if err := fixture.runner.Start(t.Context()); err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			fixture.tasks.waitSettled(t)
			shutdownCtx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if err := fixture.runner.Drain(shutdownCtx); err != nil {
				t.Fatalf("Drain() error = %v", err)
			}
			if got := len(fixture.tasks.releases); got != 1 {
				t.Fatalf("release calls = %d, want 1 contention release", got)
			}
			if got := len(fixture.tasks.settlements); got != 1 {
				t.Fatalf("settlement calls = %d, want one post-retry settlement", got)
			}
			if got := fixture.prompter.cancelCalls; got != 0 {
				t.Fatalf("CancelPrompt calls = %d, want 0 for another prompt's contention", got)
			}
		},
	)

	t.Run("Should settle one permanent resume failure", func(t *testing.T) {
		t.Parallel()

		fixture := newNetworkWakeRunnerFixture(t)
		resumeErr := errors.New("provider resume failed")
		fixture.prompter.resume = func(context.Context, string, int) (*session.Session, error) {
			return nil, resumeErr
		}
		if _, err := fixture.runner.processNotification(
			t.Context(),
			fixture.actor,
			fixture.notification,
		); !errors.Is(err, resumeErr) {
			t.Fatalf("processNotification() error = %v, want resume error", err)
		}
		settled := fixture.tasks.settledOutcome(t)
		if settled.State != store.NetworkWakeStateFailed || settled.Reason != "network wake resume failed" {
			t.Fatalf("settled outcome = %#v, want canonical resume failure", settled)
		}
		if got := len(fixture.tasks.releases); got != 0 {
			t.Fatalf("release calls = %d, want 0 after committed failure", got)
		}
	})

	t.Run("Should deduplicate duplicate queued callbacks before settlement", func(t *testing.T) {
		t.Parallel()

		fixture := newNetworkWakeRunnerFixture(t)
		fixture.store.queued = []store.CommittedNetworkNotification{
			fixture.notification,
			fixture.notification,
		}
		fixture.prompter.events = []acp.AgentEvent{{Type: acp.EventTypeDone}}
		if err := fixture.runner.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		fixture.tasks.waitSettled(t)
		shutdownCtx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if err := fixture.runner.Drain(shutdownCtx); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
		if got := len(fixture.tasks.settlements); got != 1 {
			t.Fatalf("settlement calls = %d, want 1", got)
		}
		if got := fixture.tasks.claimCalls; got != 1 {
			t.Fatalf("claim calls = %d, want 1", got)
		}
	})

	t.Run("Should bound global workers and serialize each workspace session key", func(t *testing.T) {
		t.Parallel()

		fixture := newKeyedNetworkWakeRunnerFixture(t)
		if err := fixture.runner.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		first := fixture.prompter.waitStarted(t)
		second := fixture.prompter.waitStarted(t)
		if got := map[string]bool{first: true, second: true}; !got["run-a-1"] || !got["run-b-1"] {
			t.Fatalf("initial starts = %#v, want run-a-1 and run-b-1", got)
		}
		fixture.prompter.finish("run-b-1")
		if got := fixture.prompter.waitStarted(t); got != "run-c-1" {
			t.Fatalf("third start = %q, want run-c-1 while run-a-1 holds its key", got)
		}
		fixture.prompter.finish("run-a-1")
		if got := fixture.prompter.waitStarted(t); got != "run-a-2" {
			t.Fatalf("fourth start = %q, want run-a-2 after run-a-1", got)
		}
		fixture.prompter.finish("run-c-1")
		fixture.prompter.finish("run-a-2")
		fixture.tasks.waitSettlementCount(t, 4)
		shutdownCtx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if err := fixture.runner.Drain(shutdownCtx); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
		if got := fixture.prompter.maxActiveCount(); got != 2 {
			t.Fatalf("maximum active prompts = %d, want global limit 2", got)
		}
		if got := fixture.runner.activeCount(); got != 0 {
			t.Fatalf("active workers after Drain = %d, want 0", got)
		}
	})

	t.Run("Should stop without readiness when the initial durable wake scan fails", func(t *testing.T) {
		t.Parallel()

		fixture := newNetworkWakeRunnerFixture(t)
		scanErr := errors.New("durable wake scan unavailable")
		fixture.store.listQueued = func(context.Context) ([]store.CommittedNetworkNotification, error) {
			return nil, scanErr
		}
		if err := fixture.runner.Start(t.Context()); !errors.Is(err, scanErr) {
			t.Fatalf("Start() error = %v, want durable scan error", err)
		}
		if got := fixture.runner.State(); got != networkWakeRunnerStopped {
			t.Fatalf("State() = %q, want %q", got, networkWakeRunnerStopped)
		}
		select {
		case <-fixture.runner.Ready():
			t.Fatal("Ready() closed after a failed recovery scan")
		default:
		}
	})

	t.Run("Should cancel an active wake after availability disables", func(t *testing.T) {
		t.Parallel()

		fixture := newNetworkWakeRunnerFixture(t)
		started := make(chan struct{})
		fixture.prompter.prompt = func(ctx context.Context) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			close(started)
			go func() {
				<-ctx.Done()
				events <- acp.AgentEvent{
					Type:             acp.EventTypeDone,
					PromptStopReason: acp.PromptStopReasonCancelled,
				}
				close(events)
			}()
			return events, nil
		}

		done := make(chan error, 1)
		go func() {
			_, err := fixture.runner.processNotification(t.Context(), fixture.actor, fixture.notification)
			done <- err
		}()
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("PromptNetwork did not start")
		}
		if err := fixture.runner.SetEnabled(t.Context(), false); err != nil {
			t.Fatalf("SetEnabled(false) error = %v", err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("processNotification() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("network wake did not stop after disable")
		}

		settled := fixture.tasks.settledOutcome(t)
		if settled.State != store.NetworkWakeStateCanceled {
			t.Fatalf("settled state = %q, want %q", settled.State, store.NetworkWakeStateCanceled)
		}
		if got := len(fixture.tasks.settlements); got != 1 {
			t.Fatalf("settlement calls = %d, want 1", got)
		}
		if got := fixture.prompter.cancelCalls; got != 1 {
			t.Fatalf("CancelPrompt calls = %d, want 1", got)
		}
	})

	t.Run("Should cancel and charge a wake when its wall-time deadline expires", func(t *testing.T) {
		t.Parallel()

		fixture := newNetworkWakeRunnerFixture(t)
		fixture.store.reservation.ReservedWallTime = "20ms"
		canceled := make(chan struct{})
		fixture.prompter.prompt = func(ctx context.Context) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				<-ctx.Done()
				close(canceled)
				close(events)
			}()
			return events, nil
		}

		if _, err := fixture.runner.processNotification(
			t.Context(),
			fixture.actor,
			fixture.notification,
		); err != nil {
			t.Fatalf("processNotification() error = %v", err)
		}
		select {
		case <-canceled:
		case <-time.After(time.Second):
			t.Fatal("PromptNetwork context was not canceled at the wake deadline")
		}
		settled := fixture.tasks.settledOutcome(t)
		if settled.State != store.NetworkWakeStateDeadlineExceeded ||
			settled.UsageState != store.NetworkWakeUsageUnavailable {
			t.Fatalf("settled outcome = %#v, want deadline_exceeded with conservative usage", settled)
		}
		if settled.ActualWallTime <= 0 {
			t.Fatalf("settled wall time = %s, want a consumed duration", settled.ActualWallTime)
		}
		if got := len(fixture.tasks.settlements); got != 1 {
			t.Fatalf("settlement calls = %d, want 1", got)
		}
		if got := fixture.prompter.cancelCalls; got != 1 {
			t.Fatalf("CancelPrompt calls = %d, want 1", got)
		}
	})
}

type networkWakeRunnerFixture struct {
	runner       *networkWakeRunner
	tasks        *networkWakeTaskStub
	prompter     *networkWakePrompterStub
	store        *networkWakeStoreStub
	actor        taskpkg.ActorContext
	notification store.CommittedNetworkNotification
}

func newNetworkWakeRunnerFixture(t *testing.T) networkWakeRunnerFixture {
	t.Helper()

	notification := store.CommittedNetworkNotification{
		WorkspaceID:        "workspace-1",
		RecipientSessionID: "session-live",
		TaskRunID:          "run-wake-1",
		AcceptanceSeq:      1,
	}
	run := taskpkg.Run{
		ID:          notification.TaskRunID,
		WorkspaceID: notification.WorkspaceID,
		RunKind:     taskpkg.RunKindNetworkWake,
	}
	run.SetNetworkState(
		participation.LocalSpec(),
		"wake-1",
		notification.RecipientSessionID,
		"session:"+notification.RecipientSessionID,
	)
	tasks := &networkWakeTaskStub{
		claim:   &taskpkg.ClaimResult{Run: run, ClaimToken: "claim-token"},
		settled: make(chan struct{}),
	}
	wakeStore := &networkWakeStoreStub{
		reservation: store.WakeReservation{
			WakeID:           "wake-1",
			TaskRunID:        notification.TaskRunID,
			WorkspaceID:      notification.WorkspaceID,
			OwnerKey:         "session:" + notification.RecipientSessionID,
			EnvelopeIDs:      []string{"message-1"},
			ReservedWallTime: time.Second.String(),
		},
		messages: []store.NetworkMessageEntry{{
			MessageID:   "message-1",
			WorkspaceID: "workspace-1",
			Channel:     "builders",
			Kind:        "say",
			Surface:     "direct",
			DirectID:    "direct-1",
			PeerFrom:    "session-sender",
			PeerTo:      notification.RecipientSessionID,
			Body:        json.RawMessage(`{"text":"review this"}`),
		}},
	}
	prompter := &networkWakePrompterStub{}
	runner, err := newNetworkWakeRunner(tasks, prompter, wakeStore, nil, 2)
	if err != nil {
		t.Fatalf("newNetworkWakeRunner() error = %v", err)
	}
	actor, err := taskpkg.DeriveDaemonActorContext("network-wake-runner", "daemon.network_wake")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	return networkWakeRunnerFixture{
		runner: runner, tasks: tasks, prompter: prompter, store: wakeStore,
		actor: actor, notification: notification,
	}
}

type networkWakeTaskStub struct {
	mu            sync.Mutex
	claim         *taskpkg.ClaimResult
	claimCalls    int
	claimHeld     bool
	settlements   []taskpkg.NetworkWakeSettlement
	settlementErr error
	releases      []taskpkg.LeaseRelease
	settled       chan struct{}
}

func (s *networkWakeTaskStub) ClaimNextRun(
	_ context.Context,
	criteria taskpkg.ClaimCriteria,
	_ taskpkg.ActorContext,
) (*taskpkg.ClaimResult, error) {
	s.claimCalls++
	if s.claim == nil || criteria.RunID != s.claim.Run.ID ||
		criteria.TargetSessionID != s.claim.Run.NetworkTargetSessionID {
		return nil, taskpkg.ErrNoClaimableRun
	}
	if s.claimHeld {
		return nil, taskpkg.ErrActiveRunLease
	}
	s.claimHeld = true
	return s.claim, nil
}

func (s *networkWakeTaskStub) HeartbeatRunLease(
	context.Context,
	taskpkg.LeaseHeartbeat,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return &s.claim.Run, nil
}

func (s *networkWakeTaskStub) SettleNetworkWake(
	_ context.Context,
	settlement taskpkg.NetworkWakeSettlement,
	_ taskpkg.ActorContext,
) (*taskpkg.NetworkWakeSettlementResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settlements = append(s.settlements, settlement)
	if s.settlementErr != nil {
		return nil, s.settlementErr
	}
	s.claimHeld = false
	select {
	case <-s.settled:
	default:
		close(s.settled)
	}
	return &taskpkg.NetworkWakeSettlementResult{Run: s.claim.Run, Outcome: settlement.Outcome}, nil
}

func (s *networkWakeTaskStub) ReleaseRunLease(
	_ context.Context,
	release taskpkg.LeaseRelease,
	_ taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	s.releases = append(s.releases, release)
	s.claimHeld = false
	return &s.claim.Run, nil
}

func (s *networkWakeTaskStub) waitSettled(t *testing.T) {
	t.Helper()
	select {
	case <-s.settled:
	case <-time.After(time.Second):
		t.Fatal("network wake was not settled")
	}
}

func (s *networkWakeTaskStub) settledOutcome(t *testing.T) store.NetworkWakeOutcome {
	t.Helper()
	s.waitSettled(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.settlements) == 0 {
		t.Fatal("network wake settlement is missing")
	}
	return s.settlements[len(s.settlements)-1].Outcome
}

type networkWakePrompterStub struct {
	calls       int
	cancelCalls int
	resumeCalls int
	events      []acp.AgentEvent
	prompt      func(context.Context) (<-chan acp.AgentEvent, error)
	resume      func(context.Context, string, int) (*session.Session, error)
	cancelErr   error
}

func (s *networkWakePrompterStub) Resume(ctx context.Context, sessionID string) (*session.Session, error) {
	s.resumeCalls++
	if s.resume != nil {
		return s.resume(ctx, sessionID, s.resumeCalls)
	}
	return nil, nil
}

func (s *networkWakePrompterStub) PromptNetwork(
	ctx context.Context,
	_ string,
	_ string,
	_ ...acp.PromptNetworkMeta,
) (<-chan acp.AgentEvent, error) {
	s.calls++
	if s.prompt != nil {
		return s.prompt(ctx)
	}
	events := make(chan acp.AgentEvent, len(s.events))
	for _, event := range s.events {
		events <- event
	}
	close(events)
	return events, nil
}

func (s *networkWakePrompterStub) CancelPrompt(context.Context, string) error {
	s.cancelCalls++
	return s.cancelErr
}

type networkWakeStoreStub struct {
	mu          sync.Mutex
	listQueued  func(context.Context) ([]store.CommittedNetworkNotification, error)
	queued      []store.CommittedNetworkNotification
	reservation store.WakeReservation
	messages    []store.NetworkMessageEntry
}

func (*networkWakeStoreStub) AcceptNetworkMessage(
	context.Context,
	store.AcceptNetworkMessageRequest,
) (store.AcceptNetworkMessageResult, error) {
	return store.AcceptNetworkMessageResult{}, errors.New("unexpected AcceptNetworkMessage call")
}

func (s *networkWakeStoreStub) ListQueuedNetworkWakes(
	ctx context.Context,
	_ int,
) ([]store.CommittedNetworkNotification, error) {
	if s.listQueued != nil {
		return s.listQueued(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	queued := append([]store.CommittedNetworkNotification(nil), s.queued...)
	s.queued = nil
	return queued, nil
}

func (s *networkWakeStoreStub) LoadNetworkWake(
	context.Context,
	string,
	string,
) (store.WakeReservation, []store.NetworkMessageEntry, error) {
	return s.reservation, append([]store.NetworkMessageEntry(nil), s.messages...), nil
}
