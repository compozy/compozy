package situation

import (
	"encoding/json"

	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (s *Service) enforceTaskContextBudget(
	bundle taskpkg.ContextBundle,
	maxBytes int,
) (taskpkg.ContextBundle, error) {
	if maxBytes <= 0 {
		return taskpkg.NormalizeContextBundle(bundle), nil
	}
	bundle = taskpkg.NormalizeContextBundle(bundle)
	for {
		over, err := taskContextOverBudget(bundle, maxBytes)
		if err != nil {
			return taskpkg.ContextBundle{}, err
		}
		if !over {
			return bundle, nil
		}
		switch {
		case len(bundle.RecentEvents) > 0:
			bundle.RecentEvents = bundle.RecentEvents[1:]
		case len(bundle.PriorAttempts) > 0:
			bundle.PriorAttempts = bundle.PriorAttempts[:len(bundle.PriorAttempts)-1]
		case len(bundle.ReviewHistory) > 0:
			bundle.ReviewHistory = bundle.ReviewHistory[:len(bundle.ReviewHistory)-1]
		case strings.TrimSpace(bundle.HandoffSummary) != "":
			bundle.HandoffSummary = truncateUTF8Bytes(bundle.HandoffSummary, len(bundle.HandoffSummary)/2)
		case bundle.ReviewContinuation != nil && strings.TrimSpace(bundle.ReviewContinuation.NextRoundGuidance) != "":
			continuation := *bundle.ReviewContinuation
			continuation.NextRoundGuidance = truncateUTF8Bytes(
				continuation.NextRoundGuidance,
				len(continuation.NextRoundGuidance)/2,
			)
			bundle.ReviewContinuation = &continuation
		default:
			return taskpkg.ContextBundle{}, fmt.Errorf(
				"%w: task context bundle exceeds %d bytes after trimming",
				taskpkg.ErrPayloadTooLarge,
				maxBytes,
			)
		}
	}
}

func renderTaskBundlePrompt(bundle taskpkg.ContextBundle) (string, error) {
	payload := contract.AgentContextPayload{
		Task: contract.AgentTaskContextPayload{
			Available: true,
			Bundle:    &bundle,
		},
	}
	return RenderPrompt(&payload)
}

func taskContextOverBudget(bundle taskpkg.ContextBundle, maxBytes int) (bool, error) {
	content, err := json.Marshal(bundle)
	if err != nil {
		return false, fmt.Errorf("situation: marshal task context bundle: %w", err)
	}
	return len(content) > maxBytes, nil
}

func taskContextConfig(workspaceSnapshot *workspacepkg.ResolvedWorkspace) aghconfig.TaskOrchestrationConfig {
	defaults := aghconfig.DefaultTaskConfig().Orchestration
	if workspaceSnapshot == nil {
		return defaults
	}
	cfg := workspaceSnapshot.Config.Task.Orchestration
	if err := cfg.Validate("task.orchestration"); err != nil {
		return defaults
	}
	return cfg
}

func taskContextRuntimeLimits(cfg aghconfig.TaskOrchestrationConfig) taskpkg.RuntimeLimits {
	return taskpkg.RuntimeLimits{
		MaxRuntimeSeconds: int64(cfg.DefaultMaxRuntime.Seconds()),
		SummaryMaxBytes:   cfg.SummaryMaxBytes,
		ContextMaxBytes:   cfg.ContextBodyMaxBytes,
	}
}

const maxReviewReasonFallback = 2048

func runReviewSummary(
	review taskpkg.RunReview,
	cfg aghconfig.TaskOrchestrationConfig,
) taskpkg.RunReviewSummary {
	return taskpkg.RunReviewSummary{
		ReviewID:      strings.TrimSpace(review.ReviewID),
		RunID:         strings.TrimSpace(review.RunID),
		ReviewRound:   review.ReviewRound,
		Attempt:       review.Attempt,
		Status:        string(review.Status.Normalize()),
		Outcome:       string(review.Outcome.Normalize()),
		Reason:        safeTaskContextText(review.Reason, cfg.Review.ReasonMaxBytes),
		ReviewedAt:    formatOptionalTime(review.ReviewedAt),
		ReviewerLabel: safeTaskContextText(reviewReviewerLabel(review), maxReviewReasonFallback),
	}
}

func reviewReviewerLabel(review taskpkg.RunReview) string {
	return firstTrimmed(review.ReviewerAgentName, review.ReviewerPeerID, review.ReviewerSessionID)
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func boundedMissingWork(raw json.RawMessage, maxItems int, maxItemBytes int) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("situation: decode review missing work: %w", err)
	}
	if maxItems <= 0 || len(values) < maxItems {
		maxItems = len(values)
	}
	result := make([]string, 0, maxItems)
	for _, value := range values {
		if len(result) == maxItems {
			break
		}
		trimmed := safeTaskContextText(value, maxItemBytes)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result, nil
}

func handoffSummary(
	run taskpkg.Run,
	continuation *taskpkg.ReviewContinuation,
	contextMaxBytes int,
) string {
	if continuation != nil {
		return safeTaskContextText(continuation.NextRoundGuidance, contextMaxBytes/2)
	}
	return safeTaskContextText(run.Error, contextMaxBytes/2)
}
