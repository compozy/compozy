package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/providerdefaults"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

var reviewProviderRegistry = providerdefaults.DefaultRegistry

func (j *jobExecutionContext) afterJobSuccess(ctx context.Context, jb *job) error {
	if j.cfg.Mode == model.ExecutionModePRDTasks {
		return j.afterTaskJobSuccess(jb)
	}

	if j.cfg.Mode != model.ExecutionModePRReview {
		return nil
	}
	return j.afterReviewJobSuccess(ctx, jb)
}

func (j *jobExecutionContext) afterTaskJobSuccess(jb *job) error {
	if strings.TrimSpace(j.cfg.TasksDir) == "" {
		return fmt.Errorf("missing tasks directory for task post-processing")
	}

	entry, err := singleTaskEntry(jb)
	if err != nil {
		return err
	}
	oldTask, err := tasks.ParseTaskFile(entry.Content)
	if err != nil {
		return fmt.Errorf("parse task file %s before completion: %w", entry.AbsPath, err)
	}
	if err := tasks.MarkTaskCompleted(j.cfg.TasksDir, entry.Name); err != nil {
		return err
	}
	j.submitEventOrWarn(
		events.EventKindTaskFileUpdated,
		kinds.TaskFileUpdatedPayload{
			TasksDir:  j.cfg.TasksDir,
			TaskName:  entry.Name,
			FilePath:  entry.AbsPath,
			OldStatus: oldTask.Status,
			NewStatus: "completed",
		},
	)

	meta, err := tasks.RefreshTaskMeta(j.cfg.TasksDir)
	if err != nil {
		return err
	}
	j.submitEventOrWarn(
		events.EventKindTaskMetadataRefreshed,
		kinds.TaskMetadataRefreshedPayload{
			TasksDir:  j.cfg.TasksDir,
			CreatedAt: meta.CreatedAt,
			UpdatedAt: meta.UpdatedAt,
			Total:     meta.Total,
			Completed: meta.Completed,
			Pending:   meta.Pending,
		},
	)
	j.runtimeLogger().Info(
		"updated task workflow metadata",
		"tasks_dir",
		j.cfg.TasksDir,
		"completed",
		meta.Completed,
		"pending",
		meta.Pending,
		"total",
		meta.Total,
	)
	return nil
}

func (j *jobExecutionContext) afterReviewJobSuccess(ctx context.Context, jb *job) error {
	if strings.TrimSpace(j.cfg.ReviewsDir) == "" {
		return fmt.Errorf("missing reviews directory for review post-processing")
	}

	batchEntries := prompt.FlattenAndSortIssues(jb.Groups, model.ExecutionModePRReview)
	if len(batchEntries) == 0 {
		return errors.New("missing review entries for review post-processing")
	}
	if err := reviews.FinalizeIssueStatuses(j.cfg.ReviewsDir, batchEntries); err != nil {
		return err
	}
	issueIDs := make([]string, 0, len(batchEntries))
	for _, entry := range batchEntries {
		issueIDs = append(issueIDs, entry.Name)
	}
	j.submitEventOrWarn(
		events.EventKindReviewStatusFinalized,
		kinds.ReviewStatusFinalizedPayload{
			ReviewsDir: j.cfg.ReviewsDir,
			IssueIDs:   issueIDs,
		},
	)

	resolvedIssues, err := collectNewlyResolvedIssues(jb.Groups)
	if err != nil {
		return err
	}
	providerBackedIssues := filterResolvedIssuesWithProviderRefs(resolvedIssues)
	if err := j.resolveProviderBackedIssues(ctx, providerBackedIssues); err != nil {
		return err
	}

	meta, err := reviews.RefreshRoundMeta(j.cfg.ReviewsDir)
	if err != nil {
		return err
	}
	j.submitEventOrWarn(
		events.EventKindReviewRoundRefreshed,
		kinds.ReviewRoundRefreshedPayload{
			ReviewsDir: j.cfg.ReviewsDir,
			Provider:   meta.Provider,
			PR:         meta.PR,
			Round:      meta.Round,
			CreatedAt:  meta.CreatedAt,
			Total:      meta.Total,
			Resolved:   meta.Resolved,
			Unresolved: meta.Unresolved,
		},
	)
	j.runtimeLogger().Info(
		"updated review round metadata",
		"provider",
		meta.Provider,
		"pr",
		meta.PR,
		"round",
		meta.Round,
		"resolved",
		meta.Resolved,
		"unresolved",
		meta.Unresolved,
	)
	return nil
}

func singleTaskEntry(jb *job) (model.IssueEntry, error) {
	if jb == nil {
		return model.IssueEntry{}, errors.New("missing job for task post-processing")
	}

	entries := prompt.FlattenAndSortIssues(jb.Groups, model.ExecutionModePRDTasks)
	if len(entries) != 1 {
		return model.IssueEntry{}, fmt.Errorf("expected exactly 1 task entry, got %d", len(entries))
	}
	return entries[0], nil
}

func (j *jobExecutionContext) resolveProviderBackedIssues(
	ctx context.Context,
	providerBackedIssues []provider.ResolvedIssue,
) error {
	if len(providerBackedIssues) == 0 {
		return nil
	}

	startedAt := time.Now().UTC()
	callID := fmt.Sprintf("%s-%d", strings.TrimSpace(j.cfg.Provider), startedAt.UnixNano())
	j.emitProviderCallStarted(callID, len(providerBackedIssues))

	reviewProvider, err := j.lookupReviewProvider()
	if err != nil {
		return j.handleProviderResolveFailure(
			callID,
			providerBackedIssues,
			startedAt,
			err,
			"review provider integration unavailable; skipping remote issue resolution",
		)
	}

	if err := reviewProvider.ResolveIssues(ctx, j.cfg.PR, providerBackedIssues); err != nil {
		return j.handleProviderResolveFailure(
			callID,
			providerBackedIssues,
			startedAt,
			err,
			"review provider resolution completed with warnings",
		)
	}

	completedAt := time.Now().UTC()
	j.emitProviderCallCompleted(callID, startedAt, completedAt, 0)
	j.emitReviewIssueResolved(providerBackedIssues, true, completedAt)

	j.runtimeLogger().Info(
		"resolved review provider issues",
		"provider",
		j.cfg.Provider,
		"pr",
		j.cfg.PR,
		"resolved_issues",
		len(providerBackedIssues),
	)
	return nil
}

func (j *jobExecutionContext) emitProviderCallStarted(callID string, issueCount int) {
	j.submitEventOrWarn(
		events.EventKindProviderCallStarted,
		kinds.ProviderCallStartedPayload{
			CallID:     callID,
			Provider:   j.cfg.Provider,
			Method:     "resolve_issues",
			PR:         j.cfg.PR,
			IssueCount: issueCount,
		},
	)
}

func (j *jobExecutionContext) emitProviderCallCompleted(
	callID string,
	startedAt time.Time,
	completedAt time.Time,
	statusCode int,
) {
	j.submitEventOrWarn(
		events.EventKindProviderCallCompleted,
		kinds.ProviderCallCompletedPayload{
			CallID:     callID,
			Provider:   j.cfg.Provider,
			Method:     "resolve_issues",
			StatusCode: statusCode,
			DurationMs: completedAt.Sub(startedAt).Milliseconds(),
		},
	)
}

func (j *jobExecutionContext) lookupReviewProvider() (provider.Provider, error) {
	registry := reviewProviderRegistry()
	return registry.Get(j.cfg.Provider)
}

func (j *jobExecutionContext) handleProviderResolveFailure(
	callID string,
	providerBackedIssues []provider.ResolvedIssue,
	startedAt time.Time,
	err error,
	message string,
) error {
	j.emitProviderCallFailed(callID, startedAt, err)
	j.emitReviewIssueResolved(providerBackedIssues, false, time.Time{})
	j.runtimeLogger().Warn(
		message,
		"provider",
		j.cfg.Provider,
		"pr",
		j.cfg.PR,
		"resolved_issues",
		len(providerBackedIssues),
		"error",
		err,
	)
	return nil
}

func (j *jobExecutionContext) emitProviderCallFailed(
	callID string,
	startedAt time.Time,
	err error,
) {
	j.submitEventOrWarn(
		events.EventKindProviderCallFailed,
		kinds.ProviderCallFailedPayload{
			CallID:     callID,
			Provider:   j.cfg.Provider,
			Method:     "resolve_issues",
			StatusCode: providerStatusCode(err),
			DurationMs: time.Since(startedAt).Milliseconds(),
			Error:      err.Error(),
		},
	)
}

func (j *jobExecutionContext) emitReviewIssueResolved(
	issues []provider.ResolvedIssue,
	providerPosted bool,
	postedAt time.Time,
) {
	for _, issue := range issues {
		payload := kinds.ReviewIssueResolvedPayload{
			ReviewsDir:     j.cfg.ReviewsDir,
			IssueID:        issueIDFromPath(issue.FilePath),
			FilePath:       issue.FilePath,
			Provider:       j.cfg.Provider,
			PR:             j.cfg.PR,
			ProviderRef:    issue.ProviderRef,
			ProviderPosted: providerPosted,
		}
		if providerPosted {
			payload.PostedAt = postedAt
		}
		j.submitEventOrWarn(events.EventKindReviewIssueResolved, payload)
	}
}

func collectNewlyResolvedIssues(groups map[string][]model.IssueEntry) ([]provider.ResolvedIssue, error) {
	resolved := make([]provider.ResolvedIssue, 0)
	for _, entries := range groups {
		for _, entry := range entries {
			currentBody, err := os.ReadFile(entry.AbsPath)
			if err != nil {
				return nil, fmt.Errorf("read updated issue file %s: %w", entry.AbsPath, err)
			}
			currentContent := string(currentBody)
			currentResolved, err := reviews.IsReviewResolved(currentContent)
			if err != nil {
				return nil, fmt.Errorf("parse updated review issue %s: %w", entry.AbsPath, err)
			}
			previouslyResolved, err := reviews.IsReviewResolved(entry.Content)
			if err != nil {
				return nil, fmt.Errorf("parse original review issue %s: %w", entry.AbsPath, err)
			}
			if !currentResolved || previouslyResolved {
				continue
			}

			reviewContext, err := reviews.ParseReviewContext(currentContent)
			if err != nil {
				return nil, fmt.Errorf("parse review context for %s: %w", entry.AbsPath, err)
			}
			resolved = append(resolved, provider.ResolvedIssue{
				FilePath:    entry.AbsPath,
				ProviderRef: reviewContext.ProviderRef,
			})
		}
	}

	sort.SliceStable(resolved, func(i, j int) bool {
		return resolved[i].FilePath < resolved[j].FilePath
	})
	return resolved, nil
}

func filterResolvedIssuesWithProviderRefs(issues []provider.ResolvedIssue) []provider.ResolvedIssue {
	filtered := make([]provider.ResolvedIssue, 0, len(issues))
	for _, issue := range issues {
		if strings.TrimSpace(issue.ProviderRef) == "" {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}
