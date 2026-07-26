package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type keyedNetworkWakeRunnerFixture struct {
	runner   *networkWakeRunner
	tasks    *keyedNetworkWakeTaskStub
	prompter *keyedNetworkWakePrompterStub
}

func TestNetworkWakeDispatchHonorsCoalesceDeadline(t *testing.T) {
	t.Parallel()

	t.Run("Should hold a committed wake until its durable coalescing deadline", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 19, 13, 15, 0, 0, time.UTC)
		readyAt := now.Add(5 * time.Second)
		pending := makeNetworkWakePending([]store.CommittedNetworkNotification{{
			WorkspaceID:        "ws-coalesce",
			RecipientSessionID: "sess-coalesce",
			TaskRunID:          "run-coalesce",
			ReadyAt:            readyAt,
		}}, now)
		if got := nextRunnableNetworkWake(pending, map[string]struct{}{}, now); got != -1 {
			t.Fatalf("nextRunnableNetworkWake(before deadline) = %d, want -1", got)
		}
		if got := nextRunnableNetworkWake(pending, map[string]struct{}{}, readyAt); got != 0 {
			t.Fatalf("nextRunnableNetworkWake(at deadline) = %d, want 0", got)
		}
	})
}

func newKeyedNetworkWakeRunnerFixture(t *testing.T) keyedNetworkWakeRunnerFixture {
	t.Helper()

	definitions := []struct {
		runID       string
		workspaceID string
		sessionID   string
	}{
		{runID: "run-a-1", workspaceID: "ws-a", sessionID: "sess-a"},
		{runID: "run-a-2", workspaceID: "ws-a", sessionID: "sess-a"},
		{runID: "run-b-1", workspaceID: "ws-b", sessionID: "sess-a"},
		{runID: "run-c-1", workspaceID: "ws-c", sessionID: "sess-c"},
	}
	tasks := &keyedNetworkWakeTaskStub{
		runs:        make(map[string]taskpkg.Run, len(definitions)),
		claimed:     make(map[string]bool, len(definitions)),
		settled:     make(chan struct{}, len(definitions)),
		settlements: make([]taskpkg.NetworkWakeSettlement, 0, len(definitions)),
	}
	wakeStore := &keyedNetworkWakeStoreStub{
		queued:       make([]store.CommittedNetworkNotification, 0, len(definitions)),
		reservations: make(map[string]store.WakeReservation, len(definitions)),
		messages:     make(map[string][]store.NetworkMessageEntry, len(definitions)),
	}
	finish := make(map[string]chan struct{}, len(definitions))
	for index, definition := range definitions {
		wakeID := "wake-" + definition.runID
		run := taskpkg.Run{
			ID: definition.runID, WorkspaceID: definition.workspaceID,
			RunKind: taskpkg.RunKindNetworkWake, Status: taskpkg.TaskRunStatusQueued,
		}
		run.SetNetworkState(
			participation.LocalSpec(),
			wakeID,
			definition.sessionID,
			"session:"+definition.sessionID,
		)
		tasks.runs[run.ID] = run
		wakeStore.queued = append(wakeStore.queued, store.CommittedNetworkNotification{
			WorkspaceID: definition.workspaceID, RecipientSessionID: definition.sessionID,
			TaskRunID: definition.runID, AcceptanceSeq: int64(index + 1),
		})
		wakeStore.reservations[wakeID] = store.WakeReservation{
			WakeID: wakeID, TaskRunID: definition.runID, WorkspaceID: definition.workspaceID,
			OwnerKey: "session:" + definition.sessionID, EnvelopeIDs: []string{"message-" + definition.runID},
			ReservedWallTime: time.Second.String(),
		}
		wakeStore.messages[wakeID] = []store.NetworkMessageEntry{{
			MessageID: "message-" + definition.runID, WorkspaceID: definition.workspaceID,
			Channel: "builders", Kind: "say", Surface: "direct", DirectID: "direct-" + definition.runID,
			PeerFrom: "sender", PeerTo: definition.sessionID,
			Body: json.RawMessage(fmt.Sprintf(`{"text":%q}`, definition.runID)),
		}}
		finish[definition.runID] = make(chan struct{})
	}
	prompter := &keyedNetworkWakePrompterStub{
		runIDs:        []string{"run-a-1", "run-a-2", "run-b-1", "run-c-1"},
		started:       make(chan string, len(definitions)),
		finishByRunID: finish,
	}
	runner, err := newNetworkWakeRunner(tasks, prompter, wakeStore, nil, 2)
	if err != nil {
		t.Fatalf("newNetworkWakeRunner() error = %v", err)
	}
	return keyedNetworkWakeRunnerFixture{runner: runner, tasks: tasks, prompter: prompter}
}

type keyedNetworkWakeTaskStub struct {
	mu          sync.Mutex
	runs        map[string]taskpkg.Run
	claimed     map[string]bool
	settlements []taskpkg.NetworkWakeSettlement
	settled     chan struct{}
}

func (s *keyedNetworkWakeTaskStub) ClaimNextRun(
	_ context.Context,
	criteria taskpkg.ClaimCriteria,
	_ taskpkg.ActorContext,
) (*taskpkg.ClaimResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[strings.TrimSpace(criteria.RunID)]
	if !ok || s.claimed[run.ID] || run.Status != taskpkg.TaskRunStatusQueued {
		return nil, taskpkg.ErrNoClaimableRun
	}
	s.claimed[run.ID] = true
	run.Status = taskpkg.TaskRunStatusClaimed
	s.runs[run.ID] = run
	return &taskpkg.ClaimResult{Run: run, ClaimToken: "claim-" + run.ID}, nil
}

func (s *keyedNetworkWakeTaskStub) HeartbeatRunLease(
	_ context.Context,
	heartbeat taskpkg.LeaseHeartbeat,
	_ taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[heartbeat.RunID]
	return &run, nil
}

func (s *keyedNetworkWakeTaskStub) ReleaseRunLease(
	_ context.Context,
	release taskpkg.LeaseRelease,
	_ taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[release.RunID]
	run.Status = taskpkg.TaskRunStatusQueued
	s.runs[run.ID] = run
	s.claimed[run.ID] = false
	return &run, nil
}

func (s *keyedNetworkWakeTaskStub) SettleNetworkWake(
	_ context.Context,
	settlement taskpkg.NetworkWakeSettlement,
	_ taskpkg.ActorContext,
) (*taskpkg.NetworkWakeSettlementResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[settlement.TaskRunID]
	if !ok || !s.claimed[run.ID] || settlement.ClaimToken != "claim-"+run.ID {
		return nil, taskpkg.ErrLeaseExpired
	}
	if settlement.Outcome.State == store.NetworkWakeStateSucceeded {
		run.Status = taskpkg.TaskRunStatusCompleted
	} else {
		run.Status = taskpkg.TaskRunStatusFailed
	}
	s.runs[run.ID] = run
	s.claimed[run.ID] = false
	s.settlements = append(s.settlements, settlement)
	s.settled <- struct{}{}
	return &taskpkg.NetworkWakeSettlementResult{Run: run, Outcome: settlement.Outcome}, nil
}

func (s *keyedNetworkWakeTaskStub) waitSettlementCount(t *testing.T, count int) {
	t.Helper()
	for index := range count {
		select {
		case <-s.settled:
		case <-time.After(time.Second):
			t.Fatalf("settlement count = %d, want %d", index, count)
		}
	}
}

type keyedNetworkWakeStoreStub struct {
	mu           sync.Mutex
	queued       []store.CommittedNetworkNotification
	reservations map[string]store.WakeReservation
	messages     map[string][]store.NetworkMessageEntry
}

func (*keyedNetworkWakeStoreStub) AcceptNetworkMessage(
	context.Context,
	store.AcceptNetworkMessageRequest,
) (store.AcceptNetworkMessageResult, error) {
	return store.AcceptNetworkMessageResult{}, errors.New("unexpected AcceptNetworkMessage call")
}

func (s *keyedNetworkWakeStoreStub) ListQueuedNetworkWakes(
	context.Context,
	int,
) ([]store.CommittedNetworkNotification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	queued := append([]store.CommittedNetworkNotification(nil), s.queued...)
	s.queued = nil
	return queued, nil
}

func (s *keyedNetworkWakeStoreStub) LoadNetworkWake(
	_ context.Context,
	_ string,
	wakeID string,
) (store.WakeReservation, []store.NetworkMessageEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reservation, ok := s.reservations[wakeID]
	if !ok {
		return store.WakeReservation{}, nil, errors.New("network wake missing")
	}
	return reservation, append([]store.NetworkMessageEntry(nil), s.messages[wakeID]...), nil
}

type keyedNetworkWakePrompterStub struct {
	mu            sync.Mutex
	runIDs        []string
	started       chan string
	finishByRunID map[string]chan struct{}
	active        int
	maxActive     int
}

func (*keyedNetworkWakePrompterStub) Resume(context.Context, string) (*session.Session, error) {
	return nil, nil
}

func (s *keyedNetworkWakePrompterStub) PromptNetwork(
	ctx context.Context,
	_ string,
	prompt string,
	_ ...acp.PromptNetworkMeta,
) (<-chan acp.AgentEvent, error) {
	runID := ""
	for _, candidate := range s.runIDs {
		if strings.Contains(prompt, candidate) {
			runID = candidate
			break
		}
	}
	if runID == "" {
		return nil, errors.New("prompt does not identify a wake run")
	}
	s.mu.Lock()
	s.active++
	s.maxActive = max(s.maxActive, s.active)
	finish := s.finishByRunID[runID]
	s.mu.Unlock()
	s.started <- runID
	events := make(chan acp.AgentEvent, 1)
	go func() {
		select {
		case <-finish:
			events <- acp.AgentEvent{Type: acp.EventTypeDone}
		case <-ctx.Done():
			events <- acp.AgentEvent{
				Type: acp.EventTypeDone, PromptStopReason: acp.PromptStopReasonCancelled,
			}
		}
		s.mu.Lock()
		s.active--
		s.mu.Unlock()
		close(events)
	}()
	return events, nil
}

func (*keyedNetworkWakePrompterStub) CancelPrompt(context.Context, string) error {
	return nil
}

func (s *keyedNetworkWakePrompterStub) waitStarted(t *testing.T) string {
	t.Helper()
	select {
	case runID := <-s.started:
		return runID
	case <-time.After(time.Second):
		t.Fatal("network wake prompt did not start")
		return ""
	}
}

func (s *keyedNetworkWakePrompterStub) finish(runID string) {
	close(s.finishByRunID[runID])
}

func (s *keyedNetworkWakePrompterStub) maxActiveCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxActive
}

func (r *networkWakeRunner) activeCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.active)
}
