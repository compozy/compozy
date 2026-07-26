package task

import (
	"context"
	"errors"
	"testing"
	"time"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	diagnosticitems "github.com/compozy/agh/internal/diagnostics"
	eventspkg "github.com/compozy/agh/internal/events"
	storepkg "github.com/compozy/agh/internal/store"
)

type schedulerControlTestStore struct {
	*inMemoryManagerStore
	pause                  SchedulerPauseState
	activeClaimCount       []int
	summaries              []storepkg.EventSummary
	cancelAfterPause       context.CancelFunc
	requireDeadline        bool
	starvedRunCount        int
	escalatingCountCalls   int
	needsAttentionRunCount int
}

var _ taskPauseStore = (*schedulerControlTestStore)(nil)
var _ schedulerControlStore = (*schedulerControlTestStore)(nil)
var _ eventSummaryStore = (*schedulerControlTestStore)(nil)

func newSchedulerControlTestStore() *schedulerControlTestStore {
	return &schedulerControlTestStore{inMemoryManagerStore: newInMemoryManagerStore()}
}

func (s *schedulerControlTestStore) PauseTask(ctx context.Context, mutation PauseMutation) (Task, error) {
	if err := ctx.Err(); err != nil {
		return Task{}, err
	}
	record, err := s.GetTask(ctx, mutation.TaskID)
	if err != nil {
		return Task{}, err
	}
	record.Paused = true
	record.PausedBy = mutation.Actor
	record.PausedAt = mutation.PausedAt
	record.PausedReason = mutation.Reason
	record.UpdatedAt = mutation.PausedAt
	s.tasks[record.ID] = record
	return record, nil
}

func (s *schedulerControlTestStore) ResumeTask(ctx context.Context, mutation ResumeMutation) (Task, error) {
	if err := ctx.Err(); err != nil {
		return Task{}, err
	}
	record, err := s.GetTask(ctx, mutation.TaskID)
	if err != nil {
		return Task{}, err
	}
	record.Paused = false
	record.PausedBy = ""
	record.PausedAt = time.Time{}
	record.PausedReason = ""
	record.UpdatedAt = mutation.ResumedAt
	s.tasks[record.ID] = record
	return record, nil
}

func (s *schedulerControlTestStore) GetSchedulerPause(ctx context.Context) (SchedulerPauseState, error) {
	if err := ctx.Err(); err != nil {
		return SchedulerPauseState{}, err
	}
	if err := s.requireStatusDeadline(ctx); err != nil {
		return SchedulerPauseState{}, err
	}
	return s.pause, nil
}

func (s *schedulerControlTestStore) SetSchedulerPaused(
	ctx context.Context,
	actor string,
	reason string,
) (SchedulerPauseState, error) {
	if err := ctx.Err(); err != nil {
		return SchedulerPauseState{}, err
	}
	now := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
	s.pause = SchedulerPauseState{Paused: true, PausedBy: actor, Reason: reason, PausedAt: now, UpdatedAt: now}
	if s.cancelAfterPause != nil {
		s.cancelAfterPause()
		s.cancelAfterPause = nil
	}
	return s.pause, nil
}

func (s *schedulerControlTestStore) SetSchedulerResumed(ctx context.Context) (SchedulerPauseState, error) {
	if err := ctx.Err(); err != nil {
		return SchedulerPauseState{}, err
	}
	now := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
	s.pause = SchedulerPauseState{UpdatedAt: now}
	return s.pause, nil
}

func (s *schedulerControlTestStore) CountActiveTaskRunClaims(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := s.requireStatusDeadline(ctx); err != nil {
		return 0, err
	}
	if len(s.activeClaimCount) == 0 {
		return 0, nil
	}
	count := s.activeClaimCount[0]
	s.activeClaimCount = s.activeClaimCount[1:]
	return count, nil
}

func (s *schedulerControlTestStore) CountQueuedTaskRuns(ctx context.Context, _ bool) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := s.requireStatusDeadline(ctx); err != nil {
		return 0, err
	}
	count := 0
	for _, run := range s.runs {
		if run.Status.Normalize() == TaskRunStatusQueued {
			count++
		}
	}
	return count, nil
}

func (s *schedulerControlTestStore) CountPausedTasks(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := s.requireStatusDeadline(ctx); err != nil {
		return 0, err
	}
	count := 0
	for _, record := range s.tasks {
		if record.Paused {
			count++
		}
	}
	return count, nil
}

func (s *schedulerControlTestStore) CountEscalatingQueuedTaskRuns(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.escalatingCountCalls++
	return s.starvedRunCount, nil
}

func (s *schedulerControlTestStore) CountNeedsAttentionTaskRuns(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return s.needsAttentionRunCount, nil
}

func (s *schedulerControlTestStore) SchedulerBacklog(
	ctx context.Context,
	_ SchedulerBacklogQuery,
) (SchedulerBacklog, error) {
	if err := ctx.Err(); err != nil {
		return SchedulerBacklog{}, err
	}
	return SchedulerBacklog{}, nil
}

func (s *schedulerControlTestStore) WriteEventSummary(ctx context.Context, summary storepkg.EventSummary) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.summaries = append(s.summaries, summary)
	return nil
}

func (s *schedulerControlTestStore) requireStatusDeadline(ctx context.Context) error {
	if !s.requireDeadline {
		return nil
	}
	if _, ok := ctx.Deadline(); ok {
		return nil
	}
	return errors.New("scheduler status context missing deadline")
}

func TestSchedulerControls(t *testing.T) {
	t.Parallel()

	for _, status := range []Status{TaskStatusCompleted, TaskStatusFailed, TaskStatusCanceled} {
		t.Run("Should reject pausing "+string(status)+" task", func(t *testing.T) {
			t.Parallel()

			store := newSchedulerControlTestStore()
			store.tasks["task-terminal"] = Task{
				ID:     "task-terminal",
				Scope:  ScopeGlobal,
				Title:  "Terminal task",
				Status: status,
			}
			manager := newTaskManagerForTest(t, store)

			_, err := manager.PauseTask(
				context.Background(),
				"task-terminal",
				PauseTaskRequest{Reason: "hold"},
				validActorContext(),
			)
			if !errors.Is(err, ErrInvalidStatusTransition) {
				t.Fatalf("PauseTask(%s) error = %v, want %v", status, err, ErrInvalidStatusTransition)
			}
			item, ok := diagnosticitems.ItemFromError(err)
			if !ok {
				t.Fatalf("PauseTask(%s) error = %v, want structured diagnostic", status, err)
			}
			if item.Code != diagnosticcontract.CodeTaskRunAlreadyTerminal {
				t.Fatalf("diagnostic code = %q, want %q", item.Code, diagnosticcontract.CodeTaskRunAlreadyTerminal)
			}
			if store.tasks["task-terminal"].Paused {
				t.Fatal("PauseTask() mutated terminal task pause state")
			}
		})
	}

	t.Run("Should complete drain audit after request context is canceled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		store := newSchedulerControlTestStore()
		store.activeClaimCount = []int{1, 0}
		store.cancelAfterPause = cancel
		store.requireDeadline = true
		manager := newTaskManagerForTest(t, store)

		result, err := manager.DrainScheduler(
			ctx,
			SchedulerDrainRequest{Reason: "deploy handoff", Timeout: time.Millisecond},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("DrainScheduler() error = %v", err)
		}
		if !result.Completed || result.TimedOut || result.RemainingClaims != 0 {
			t.Fatalf("DrainScheduler() result = %#v, want completed without remaining claims", result)
		}
		if !schedulerEventSummariesContain(store.summaries, eventspkg.SchedulerDrainStarted) {
			t.Fatalf("event summaries = %#v, want %s", store.summaries, eventspkg.SchedulerDrainStarted)
		}
		if !schedulerEventSummariesContain(store.summaries, eventspkg.SchedulerDrainCompleted) {
			t.Fatalf("event summaries = %#v, want %s", store.summaries, eventspkg.SchedulerDrainCompleted)
		}
	})

	t.Run("Should hold the public starvation count through pause and resume grace", func(t *testing.T) {
		t.Parallel()

		asOf := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
		store := newSchedulerControlTestStore()
		store.starvedRunCount = 12
		manager := newTaskManagerForTestWithOptions(t, store, WithStarvationAge(2*time.Minute))

		store.pause = SchedulerPauseState{
			Paused:    true,
			PausedAt:  asOf.Add(-10 * time.Minute),
			UpdatedAt: asOf.Add(-10 * time.Minute),
		}
		paused, err := manager.SchedulerStatus(context.Background(), validActorContext())
		if err != nil {
			t.Fatalf("SchedulerStatus(paused) error = %v", err)
		}
		if paused.StarvedRunCount != 0 {
			t.Fatalf("paused StarvedRunCount = %d, want 0", paused.StarvedRunCount)
		}

		store.pause = SchedulerPauseState{UpdatedAt: asOf.Add(-time.Minute)}
		grace, err := manager.SchedulerStatus(context.Background(), validActorContext())
		if err != nil {
			t.Fatalf("SchedulerStatus(grace) error = %v", err)
		}
		if grace.StarvedRunCount != 0 {
			t.Fatalf("grace StarvedRunCount = %d, want 0", grace.StarvedRunCount)
		}
		if store.escalatingCountCalls != 0 {
			t.Fatalf("escalating count calls = %d, want 0 before grace boundary", store.escalatingCountCalls)
		}

		store.pause = SchedulerPauseState{UpdatedAt: asOf.Add(-3 * time.Minute)}
		eligible, err := manager.SchedulerStatus(context.Background(), validActorContext())
		if err != nil {
			t.Fatalf("SchedulerStatus(eligible) error = %v", err)
		}
		if eligible.StarvedRunCount != 12 {
			t.Fatalf("eligible StarvedRunCount = %d, want 12", eligible.StarvedRunCount)
		}
		if store.escalatingCountCalls != 1 {
			t.Fatalf("escalating count calls = %d, want 1 at grace boundary", store.escalatingCountCalls)
		}
	})
}

func schedulerEventSummariesContain(summaries []storepkg.EventSummary, eventType string) bool {
	for _, summary := range summaries {
		if summary.Type == eventType {
			return true
		}
	}
	return false
}
