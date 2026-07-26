package situation

import (
	"context"

	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (s *Service) priorRunSummaries(
	ctx context.Context,
	taskRecord taskpkg.Task,
	current taskpkg.Run,
	limit int,
) ([]taskpkg.RunSummary, error) {
	if limit <= 0 {
		return []taskpkg.RunSummary{}, nil
	}
	store := s.taskStoreValue()
	if store == nil {
		return []taskpkg.RunSummary{}, nil
	}
	runs, err := store.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		return nil, err
	}
	prior := make([]taskpkg.Run, 0, len(runs))
	for _, run := range runs {
		if strings.TrimSpace(run.ID) == strings.TrimSpace(current.ID) {
			continue
		}
		if run.Attempt > 0 && current.Attempt > 0 && run.Attempt >= current.Attempt {
			continue
		}
		prior = append(prior, run)
	}
	sortRunsByAttemptAndActivity(prior)
	summaries := make([]taskpkg.RunSummary, 0, min(len(prior), limit))
	for _, run := range prior {
		if len(summaries) == limit {
			break
		}
		summaries = append(summaries, runSummaryFromTaskRun(run, taskRecord.MaxAttempts))
	}
	return summaries, nil
}

func (s *Service) recentTaskEvents(
	ctx context.Context,
	taskRecord taskpkg.Task,
	limit int,
) ([]taskpkg.TimelineItem, error) {
	if limit <= 0 {
		return []taskpkg.TimelineItem{}, nil
	}
	store := s.taskStoreValue()
	if store == nil {
		return []taskpkg.TimelineItem{}, nil
	}
	records, err := store.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{
		TaskID:     taskRecord.ID,
		Limit:      limit,
		Descending: true,
	})
	if err != nil {
		return nil, err
	}
	runs, err := store.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		return nil, err
	}
	runsByID := make(map[string]taskpkg.Run, len(runs))
	for _, run := range runs {
		runsByID[strings.TrimSpace(run.ID)] = run
	}
	items := make([]taskpkg.TimelineItem, 0, len(records))
	for _, record := range records {
		event := record.Event
		var runSummary *taskpkg.RunSummary
		if run, ok := runsByID[strings.TrimSpace(event.RunID)]; ok {
			summary := runSummaryFromTaskRun(run, taskRecord.MaxAttempts)
			runSummary = &summary
		}
		payload := cloneRawJSON(event.Payload)
		redactedPayload, err := redactTaskContextPayload(payload)
		if err != nil {
			return nil, err
		}
		items = append(items, taskpkg.TimelineItem{
			Sequence:  record.Sequence,
			EventID:   strings.TrimSpace(event.ID),
			Task:      taskReference(taskRecord),
			Run:       runSummary,
			EventType: strings.TrimSpace(event.EventType),
			Actor:     event.Actor,
			Origin:    event.Origin,
			Payload:   redactedPayload,
			Timestamp: event.Timestamp.UTC(),
		})
	}
	slices.Reverse(items)
	return items, nil
}

func (s *Service) reviewContinuation(
	ctx context.Context,
	run taskpkg.Run,
	cfg aghconfig.TaskOrchestrationConfig,
) (*taskpkg.ReviewContinuation, error) {
	if run.Review == nil ||
		strings.TrimSpace(run.Review.ReviewID) == "" ||
		run.Review.ContinuationReason == "" {
		return nil, nil
	}
	store := s.taskStoreValue()
	if store == nil {
		return nil, nil
	}
	review, err := store.GetRunReview(ctx, run.Review.ReviewID)
	if err != nil {
		return nil, err
	}
	missingWorkRaw := run.Review.MissingWork
	if len(missingWorkRaw) == 0 {
		missingWorkRaw = review.MissingWork
	}
	missingWork, err := boundedMissingWork(
		missingWorkRaw,
		cfg.Review.MissingWorkMaxItems,
		cfg.Review.MissingWorkItemMaxBytes,
	)
	if err != nil {
		return nil, err
	}
	reviewRound := run.Review.ReviewRound
	if reviewRound == 0 {
		reviewRound = review.ReviewRound
	}
	return &taskpkg.ReviewContinuation{
		ReviewID:      strings.TrimSpace(run.Review.ReviewID),
		ReviewedRunID: firstTrimmed(run.Review.ParentRunID, review.RunID),
		ReviewRound:   reviewRound,
		Outcome:       string(review.Outcome.Normalize()),
		Reason: safeTaskContextText(
			firstTrimmed(review.Reason, run.Review.ContinuationReason),
			cfg.Review.ReasonMaxBytes,
		),
		MissingWork: missingWork,
		NextRoundGuidance: safeTaskContextText(
			firstTrimmed(run.Review.NextRoundGuidance, review.NextRoundGuidance),
			cfg.Review.NextRoundGuidanceMaxBytes,
		),
	}, nil
}

func (s *Service) reviewHistory(
	ctx context.Context,
	taskRecord taskpkg.Task,
	cfg aghconfig.TaskOrchestrationConfig,
) ([]taskpkg.RunReviewSummary, error) {
	limit := cfg.ContextPriorAttempts
	if limit <= 0 {
		return []taskpkg.RunReviewSummary{}, nil
	}
	store := s.taskStoreValue()
	if store == nil {
		return []taskpkg.RunReviewSummary{}, nil
	}
	reviews, err := store.ListRunReviews(ctx, taskpkg.RunReviewQuery{TaskID: taskRecord.ID, Limit: limit})
	if err != nil {
		return nil, err
	}
	summaries := make([]taskpkg.RunReviewSummary, 0, len(reviews))
	for _, review := range reviews {
		summaries = append(summaries, runReviewSummary(review, cfg))
	}
	return summaries, nil
}
