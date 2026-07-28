package task

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func (s *inMemoryManagerStore) RequestRunReview(
	_ context.Context,
	review *RunReview,
) (RunReview, bool, error) {
	normalized, err := review.Normalize(time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC))
	if err != nil {
		return RunReview{}, false, err
	}
	run, ok := s.runs[normalized.RunID]
	if !ok {
		return RunReview{}, false, ErrTaskRunNotFound
	}
	if strings.TrimSpace(run.TaskID) != normalized.TaskID {
		return RunReview{}, false, ErrValidation
	}
	if _, ok := s.tasks[normalized.TaskID]; !ok {
		return RunReview{}, false, ErrTaskNotFound
	}
	for _, existing := range s.reviews {
		if existing.RunID == normalized.RunID &&
			existing.ReviewRound == normalized.ReviewRound &&
			existing.Attempt == normalized.Attempt {
			return cloneRunReview(&existing), false, nil
		}
	}
	s.reviews[normalized.ReviewID] = cloneRunReview(&normalized)
	return cloneRunReview(&normalized), true, nil
}

func (s *inMemoryManagerStore) GetRunReview(_ context.Context, reviewID string) (RunReview, error) {
	review, ok := s.reviews[strings.TrimSpace(reviewID)]
	if !ok {
		return RunReview{}, ErrRunReviewNotFound
	}
	return cloneRunReview(&review), nil
}

func (s *inMemoryManagerStore) RecordRunReview(
	_ context.Context,
	req RecordRunReviewRequest,
	actor ActorContext,
	recordedAt time.Time,
	continuationRunID string,
) (RunReviewResult, error) {
	normalized := req.Normalize()
	if err := normalized.Validate("record_run_review"); err != nil {
		return RunReviewResult{}, err
	}
	if err := actor.Validate(); err != nil {
		return RunReviewResult{}, err
	}
	review, ok := s.reviews[normalized.ReviewID]
	if !ok {
		return RunReviewResult{}, ErrRunReviewNotFound
	}
	if review.RunID != normalized.RunID {
		return RunReviewResult{}, ErrValidation
	}
	run, ok := s.runs[review.RunID]
	if !ok {
		return RunReviewResult{}, ErrTaskRunNotFound
	}
	taskRecord, ok := s.tasks[review.TaskID]
	if !ok {
		return RunReviewResult{}, ErrTaskNotFound
	}
	if review.Status == RunReviewStatusRecorded {
		result := RunReviewResult{Review: cloneRunReview(&review)}
		if review.Outcome == RunReviewOutcomeRejected {
			for _, candidate := range s.runs {
				if candidate.Review == nil {
					continue
				}
				if candidate.Review.ReviewID == review.ReviewID {
					continuation := cloneTaskRun(candidate)
					result.ContinuationRun = &continuation
					break
				}
			}
		}
		return result, nil
	}
	if review.ReviewerSessionID != "" &&
		actor.Actor.Kind.Normalize() == ActorKindAgentSession &&
		actor.Actor.Ref != review.ReviewerSessionID {
		return RunReviewResult{}, ErrPermissionDenied
	}
	continuationAttempt := 0
	if normalized.Verdict.Outcome.Normalize() == RunReviewOutcomeRejected {
		continuationAttempt = nextRunAttempt(s.runsForTask(review.TaskID))
		maxAttempts := normalizeTaskMaxAttemptsOrDefault(taskRecord.MaxAttempts)
		if continuationAttempt > maxAttempts {
			return RunReviewResult{}, fmtTestError(
				"%w: task %q exhausted max_attempts=%d",
				ErrInvalidStatusTransition,
				taskRecord.ID,
				maxAttempts,
			)
		}
	}
	review.Status = RunReviewStatusRecorded
	review.Outcome = normalized.Verdict.Outcome
	review.Confidence = normalized.Verdict.Confidence
	review.Reason = normalized.Verdict.Reason
	review.DeliveryID = normalized.Verdict.DeliveryID
	review.MissingWork = cloneRawJSON(normalized.Verdict.MissingWork)
	review.NextRoundGuidance = normalized.Verdict.NextRoundGuidance
	review.ReviewText = normalized.Verdict.ReviewText
	review.ReviewedBy = cloneActorIdentity(&actor.Actor)
	review.ReviewedAt = recordedAt.UTC()
	review.UpdatedAt = recordedAt.UTC()
	s.reviews[review.ReviewID] = cloneRunReview(&review)

	result := RunReviewResult{Review: cloneRunReview(&review)}
	if review.Outcome != RunReviewOutcomeRejected {
		return result, nil
	}
	continuation := Run{
		ID:      strings.TrimSpace(continuationRunID),
		TaskID:  review.TaskID,
		Status:  TaskRunStatusQueued,
		Attempt: int32(continuationAttempt),
		Origin:  actor.Origin,
		Review: &RunReviewLineage{
			ParentRunID:        run.ID,
			ReviewID:           review.ReviewID,
			ReviewRound:        review.ReviewRound + 1,
			ContinuationReason: review.Reason,
			MissingWork:        cloneRawJSON(review.MissingWork),
			NextRoundGuidance:  review.NextRoundGuidance,
		},
		QueuedAt:              recordedAt.UTC(),
		RequiredCapabilities:  append([]string(nil), run.RequiredCapabilities...),
		PreferredCapabilities: append([]string(nil), run.PreferredCapabilities...),
	}
	wakeID, targetSessionID, ownerKey := run.NetworkWakeCorrelation()
	continuation.SetNetworkState(run.NetworkSpecSnapshot(), wakeID, targetSessionID, ownerKey)
	if err := continuation.Validate(); err != nil {
		return RunReviewResult{}, err
	}
	s.runs[continuation.ID] = cloneTaskRun(continuation)
	result.ContinuationRun = &continuation
	return result, nil
}

func (s *inMemoryManagerStore) BindRunReviewSession(
	_ context.Context,
	req BindRunReviewSessionRequest,
	boundAt time.Time,
) (RunReview, error) {
	normalized := req.Normalize()
	if err := normalized.Validate("run_review_binding"); err != nil {
		return RunReview{}, err
	}
	review, ok := s.reviews[normalized.ReviewID]
	if !ok {
		return RunReview{}, ErrRunReviewNotFound
	}
	if strings.TrimSpace(review.ReviewerSessionID) != "" &&
		strings.TrimSpace(review.ReviewerSessionID) != normalized.SessionID &&
		review.Status.Normalize() != RunReviewStatusInReview {
		return RunReview{}, ErrInvalidStatusTransition
	}
	for _, existing := range s.reviews {
		if existing.ReviewID != review.ReviewID &&
			existing.ReviewerSessionID == normalized.SessionID &&
			existing.Status == RunReviewStatusInReview {
			return RunReview{}, ErrInvalidStatusTransition
		}
	}
	review.Status = RunReviewStatusInReview
	review.ReviewerSessionID = normalized.SessionID
	review.ReviewerAgentName = normalized.ReviewerAgentName
	review.ReviewerPeerID = normalized.ReviewerPeerID
	review.ReviewerChannelID = normalized.ReviewerChannelID
	review.StartedAt = boundAt.UTC()
	review.UpdatedAt = boundAt.UTC()
	s.reviews[review.ReviewID] = cloneRunReview(&review)
	return cloneRunReview(&review), nil
}

func (s *inMemoryManagerStore) LookupRunReviewBySession(_ context.Context, sessionID string) (RunReview, error) {
	trimmedID := strings.TrimSpace(sessionID)
	for _, review := range s.reviews {
		if strings.TrimSpace(review.ReviewerSessionID) == trimmedID && review.Status == RunReviewStatusInReview {
			return cloneRunReview(&review), nil
		}
	}
	return RunReview{}, ErrRunReviewNotFound
}

func (s *inMemoryManagerStore) ListRunReviews(_ context.Context, query RunReviewQuery) ([]RunReview, error) {
	if err := query.Validate("run_review_query"); err != nil {
		return nil, err
	}
	normalized := query
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.Status = normalized.Status.Normalize()
	normalized.ReviewerSessionID = strings.TrimSpace(normalized.ReviewerSessionID)

	reviews := make([]RunReview, 0)
	for _, review := range s.reviews {
		if normalized.TaskID != "" && review.TaskID != normalized.TaskID {
			continue
		}
		if normalized.RunID != "" && review.RunID != normalized.RunID {
			continue
		}
		if normalized.Status.Normalize() != "" && review.Status != normalized.Status {
			continue
		}
		if normalized.ReviewerSessionID != "" && review.ReviewerSessionID != normalized.ReviewerSessionID {
			continue
		}
		reviews = append(reviews, cloneRunReview(&review))
	}
	sort.Slice(reviews, func(i int, j int) bool {
		return reviews[i].ReviewID < reviews[j].ReviewID
	})
	if normalized.Limit > 0 && len(reviews) > normalized.Limit {
		return append([]RunReview(nil), reviews[:normalized.Limit]...), nil
	}
	return reviews, nil
}

func (s *inMemoryManagerStore) runsForTask(taskID string) []Run {
	runs := make([]Run, 0)
	for _, run := range s.runs {
		if run.TaskID == taskID {
			runs = append(runs, cloneTaskRun(run))
		}
	}
	return runs
}

type recordingRunReviewRequestedObserver struct {
	mu      sync.Mutex
	records []RunReviewRequestedNotification
}

func (o *recordingRunReviewRequestedObserver) OnRunReviewRequested(
	_ context.Context,
	notification *RunReviewRequestedNotification,
) {
	if notification == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.records = append(o.records, *notification)
}

func (o *recordingRunReviewRequestedObserver) notifications() []RunReviewRequestedNotification {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]RunReviewRequestedNotification(nil), o.records...)
}

func TestTaskManagerRunReviews(t *testing.T) {
	t.Parallel()

	t.Run("Should request terminal run review idempotently and emit audit event once", func(t *testing.T) {
		t.Parallel()

		store := reviewManagerStoreForTest(TaskRunStatusCompleted)
		observer := &recordingRunReviewRequestedObserver{}
		manager := newTaskManagerForTestWithOptions(t, store, WithRunReviewRequestedObserver(observer))

		review, created, err := manager.RequestRunReview(
			context.Background(),
			RunReviewRequest{TaskID: "task-1", RunID: "run-1", Policy: ReviewPolicyAlways},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("RequestRunReview(create) error = %v", err)
		}
		if !created {
			t.Fatal("RequestRunReview(create) created = false, want true")
		}
		if got, want := review.Status, RunReviewStatusRequested; got != want {
			t.Fatalf("review.Status = %q, want %q", got, want)
		}

		duplicate, created, err := manager.RequestRunReview(
			context.Background(),
			RunReviewRequest{TaskID: "task-1", RunID: "run-1", Policy: ReviewPolicyAlways},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("RequestRunReview(duplicate) error = %v", err)
		}
		if created {
			t.Fatal("RequestRunReview(duplicate) created = true, want false")
		}
		if got, want := duplicate.ReviewID, review.ReviewID; got != want {
			t.Fatalf("duplicate.ReviewID = %q, want %q", got, want)
		}
		if got, want := countEventType(store.events, taskEventRunReviewRequested), 1; got != want {
			t.Fatalf("review requested event count = %d, want %d", got, want)
		}
		notifications := observer.notifications()
		if len(notifications) != 1 {
			t.Fatalf("review observer notifications = %d, want 1", len(notifications))
		}
		if got, want := notifications[0].Review.ReviewID, review.ReviewID; got != want {
			t.Fatalf("observer ReviewID = %q, want %q", got, want)
		}
		if got, want := notifications[0].Run.ID, "run-1"; got != want {
			t.Fatalf("observer Run.ID = %q, want %q", got, want)
		}
	})

	t.Run("Should reject review requests for non terminal runs", func(t *testing.T) {
		t.Parallel()

		store := reviewManagerStoreForTest(TaskRunStatusRunning)
		manager := newTaskManagerForTest(t, store)

		_, _, err := manager.RequestRunReview(
			context.Background(),
			RunReviewRequest{TaskID: "task-1", RunID: "run-1", Policy: ReviewPolicyAlways},
			validActorContext(),
		)
		if !errors.Is(err, ErrInvalidStatusTransition) {
			t.Fatalf("RequestRunReview(non terminal) error = %v, want %v", err, ErrInvalidStatusTransition)
		}
	})

	t.Run("Should read one persisted run review through service authority", func(t *testing.T) {
		t.Parallel()

		store := reviewManagerStoreForTest(TaskRunStatusCompleted)
		manager := newTaskManagerForTest(t, store)
		review, _, err := manager.RequestRunReview(
			context.Background(),
			RunReviewRequest{TaskID: "task-1", RunID: "run-1", Policy: ReviewPolicyAlways},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("RequestRunReview() error = %v", err)
		}

		got, err := manager.GetRunReview(context.Background(), review.ReviewID, validActorContext())
		if err != nil {
			t.Fatalf("GetRunReview() error = %v", err)
		}
		if got.ReviewID != review.ReviewID {
			t.Fatalf("GetRunReview().ReviewID = %q, want %q", got.ReviewID, review.ReviewID)
		}

		_, err = manager.GetRunReview(context.Background(), " ", validActorContext())
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("GetRunReview(blank) error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should bind lookup and list reviewer session reviews", func(t *testing.T) {
		t.Parallel()

		store := reviewManagerStoreForTest(TaskRunStatusFailed)
		manager := newTaskManagerForTest(t, store)
		review, _, err := manager.RequestRunReview(
			context.Background(),
			RunReviewRequest{TaskID: "task-1", RunID: "run-1", Policy: ReviewPolicyOnFailure},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("RequestRunReview() error = %v", err)
		}

		binding, err := manager.BindRunReviewSession(
			context.Background(),
			BindRunReviewSessionRequest{
				ReviewID:          review.ReviewID,
				SessionID:         "sess-reviewer",
				ReviewerAgentName: "reviewer",
				ReviewerChannelID: "review-channel",
			},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("BindRunReviewSession() error = %v", err)
		}
		if got, want := binding.SessionID, "sess-reviewer"; got != want {
			t.Fatalf("binding.SessionID = %q, want %q", got, want)
		}

		lookup, err := manager.LookupRunReviewForSession(context.Background(), "sess-reviewer", validActorContext())
		if err != nil {
			t.Fatalf("LookupRunReviewForSession() error = %v", err)
		}
		if got, want := lookup.Review.ReviewID, review.ReviewID; got != want {
			t.Fatalf("lookup.ReviewID = %q, want %q", got, want)
		}
		listed, err := manager.ListRunReviews(
			context.Background(),
			RunReviewQuery{Status: RunReviewStatusInReview, ReviewerSessionID: "sess-reviewer"},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("ListRunReviews() error = %v", err)
		}
		if len(listed) != 1 || listed[0].ReviewID != review.ReviewID {
			t.Fatalf("ListRunReviews() = %#v, want bound review", listed)
		}
		if !containsEventType(store.events, taskEventRunReviewBound) {
			t.Fatalf("events = %#v, want %q", sortedEventTypes(store.events), taskEventRunReviewBound)
		}
	})

	t.Run("Should record approved review verdict and emit audit events", func(t *testing.T) {
		t.Parallel()

		store := reviewManagerStoreForTest(TaskRunStatusCompleted)
		manager := newTaskManagerForTest(t, store)
		review, _, err := manager.RequestRunReview(
			context.Background(),
			RunReviewRequest{TaskID: "task-1", RunID: "run-1", Policy: ReviewPolicyAlways},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("RequestRunReview() error = %v", err)
		}

		confidence := 0.91
		result, err := manager.RecordRunReview(
			context.Background(),
			RecordRunReviewRequest{
				ReviewID: review.ReviewID,
				RunID:    review.RunID,
				Verdict: RunReviewVerdict{
					Outcome:     RunReviewOutcomeApproved,
					Confidence:  &confidence,
					Reason:      "run satisfies the requested outcome",
					DeliveryID:  "delivery-approved",
					MissingWork: []byte(`[]`),
				},
			},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("RecordRunReview(approved) error = %v", err)
		}
		if got, want := result.Review.Status, RunReviewStatusRecorded; got != want {
			t.Fatalf("Review.Status = %q, want %q", got, want)
		}
		if result.ContinuationRun != nil {
			t.Fatalf("ContinuationRun = %#v, want nil", result.ContinuationRun)
		}
		if !containsEventType(store.events, taskEventRunReviewRecorded) ||
			!containsEventType(store.events, taskEventRunReviewApproved) {
			t.Fatalf("events = %#v, want recorded and approved", sortedEventTypes(store.events))
		}
	})

	t.Run("Should record rejected review and enqueue one continuation run", func(t *testing.T) {
		t.Parallel()

		store := reviewManagerStoreForTest(TaskRunStatusCompleted)
		manager := newTaskManagerForTest(t, store)
		review, _, err := manager.RequestRunReview(
			context.Background(),
			RunReviewRequest{TaskID: "task-1", RunID: "run-1", Policy: ReviewPolicyAlways},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("RequestRunReview() error = %v", err)
		}

		confidence := 0.73
		result, err := manager.RecordRunReview(
			context.Background(),
			RecordRunReviewRequest{
				ReviewID: review.ReviewID,
				RunID:    review.RunID,
				Verdict: RunReviewVerdict{
					Outcome:           RunReviewOutcomeRejected,
					Confidence:        &confidence,
					Reason:            "missing regression coverage",
					DeliveryID:        "delivery-rejected",
					MissingWork:       []byte(`["add regression tests"]`),
					NextRoundGuidance: "Add the missing tests and rerun verification.",
				},
			},
			validActorContext(),
		)
		if err != nil {
			t.Fatalf("RecordRunReview(rejected) error = %v", err)
		}
		if result.ContinuationRun == nil {
			t.Fatal("ContinuationRun = nil, want queued continuation")
		}
		if result.ContinuationRun.Review == nil {
			t.Fatal("ContinuationRun.Review = nil, want lineage")
		}
		if got, want := result.ContinuationRun.Review.ParentRunID, "run-1"; got != want {
			t.Fatalf("ContinuationRun.ParentRunID = %q, want %q", got, want)
		}
		if got, want := result.ContinuationRun.Review.ReviewID, review.ReviewID; got != want {
			t.Fatalf("ContinuationRun.ReviewID = %q, want %q", got, want)
		}
		if !containsEventType(store.events, taskEventRunReviewRejected) ||
			!containsEventType(store.events, taskEventRunReviewRetry) {
			t.Fatalf("events = %#v, want rejected and retry", sortedEventTypes(store.events))
		}
	})

	t.Run("Should reject rejected-review continuation when max attempts are exhausted", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithSessionExecutor(testSessionExecutor{}))
		actor := validActorContext()

		taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
			Scope:       ScopeGlobal,
			Title:       "Review exhaustion",
			MaxAttempts: new(1),
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		run, err = seedNonLeasedClaimedRunForTest(context.Background(), manager, run.ID, actor)
		if err != nil {
			t.Fatalf("claimExactRunForTest() error = %v", err)
		}
		run, err = manager.StartRun(context.Background(), run.ID, StartRun{}, actor)
		if err != nil {
			t.Fatalf("StartRun() error = %v", err)
		}
		if _, err := manager.CompleteRun(context.Background(), run.ID, RunResult{
			Value: []byte(`{"ok":true}`),
		}, actor); err != nil {
			t.Fatalf("CompleteRun() error = %v", err)
		}

		review, _, err := manager.RequestRunReview(
			context.Background(),
			RunReviewRequest{TaskID: taskRecord.ID, RunID: run.ID, Policy: ReviewPolicyAlways},
			actor,
		)
		if err != nil {
			t.Fatalf("RequestRunReview() error = %v", err)
		}

		confidence := 0.61
		if _, err := manager.RecordRunReview(
			context.Background(),
			RecordRunReviewRequest{
				ReviewID: review.ReviewID,
				RunID:    review.RunID,
				Verdict: RunReviewVerdict{
					Outcome:           RunReviewOutcomeRejected,
					Confidence:        &confidence,
					Reason:            "needs another pass",
					DeliveryID:        "delivery-exhausted",
					MissingWork:       []byte(`["retry"]`),
					NextRoundGuidance: "Try again.",
				},
			},
			actor,
		); !errors.Is(err, ErrInvalidStatusTransition) {
			t.Fatalf("RecordRunReview(exhausted) error = %v, want %v", err, ErrInvalidStatusTransition)
		}

		storedReview, err := manager.GetRunReview(context.Background(), review.ReviewID, actor)
		if err != nil {
			t.Fatalf("GetRunReview() error = %v", err)
		}
		if got, want := storedReview.Status, RunReviewStatusRequested; got != want {
			t.Fatalf("stored review status = %q, want %q", got, want)
		}
		runs, err := manager.ListTaskRuns(context.Background(), taskRecord.ID, RunQuery{}, actor)
		if err != nil {
			t.Fatalf("ListTaskRuns() error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("len(runs) = %d, want %d", got, want)
		}
		if got := countEventType(store.events, taskEventRunReviewRetry); got != 0 {
			t.Fatalf("retry event count = %d, want 0", got)
		}
	})
}

func reviewManagerStoreForTest(status RunStatus) *inMemoryManagerStore {
	store := newInMemoryManagerStore()
	taskRecord := validTask()
	taskRecord.ID = "task-1"
	store.tasks[taskRecord.ID] = taskRecord
	run := validRun()
	run.ID = "run-1"
	run.TaskID = taskRecord.ID
	run.Status = status
	run.EndedAt = time.Date(2026, 4, 14, 14, 0, 0, 0, time.UTC)
	store.runs[run.ID] = run
	return store
}

func countEventType(events []Event, eventType string) int {
	count := 0
	for _, event := range events {
		if event.EventType == eventType {
			count++
		}
	}
	return count
}
