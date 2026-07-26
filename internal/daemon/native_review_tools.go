package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	nativeReviewToolsReviewKey = "review"
)

const (
	nativeReviewToolsCreatedKey = "created"
	nativeReviewToolsReviewsKey = "reviews"
)

type submitRunReviewInput struct {
	ReviewID          string   `json:"review_id"`
	RunID             string   `json:"run_id"`
	Outcome           string   `json:"outcome"`
	Confidence        *float64 `json:"confidence"`
	Reason            string   `json:"reason"`
	MissingWork       []string `json:"missing_work"`
	NextRoundGuidance string   `json:"next_round_guidance"`
	ReviewText        string   `json:"review_text,omitempty"`
	DeliveryID        string   `json:"delivery_id"`
}

type taskRunReviewRequestInput struct {
	TaskID         string `json:"task_id"`
	RunID          string `json:"run_id"`
	Policy         string `json:"policy"`
	ReviewRound    int    `json:"review_round"`
	Attempt        int    `json:"attempt"`
	ParentReviewID string `json:"parent_review_id"`
	Reason         string `json:"reason"`
}

type taskRunReviewListInput struct {
	TaskID            string `json:"task_id"`
	RunID             string `json:"run_id"`
	Status            string `json:"status"`
	ReviewerSessionID string `json:"reviewer_session_id"`
	Limit             int    `json:"limit"`
}

type taskRunReviewShowInput struct {
	ReviewID string `json:"review_id"`
}

func (n *daemonNativeTools) taskRunReviewRequest(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskRunReviewRequestInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	request, err := input.request(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	review, created, err := n.deps.Tasks.RequestRunReview(ctx, request, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeReviewToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{nativeReviewToolsReviewKey: review, nativeReviewToolsCreatedKey: created},
		fmt.Sprintf("review %s", review.ReviewID),
	)
}

func (n *daemonNativeTools) taskRunReviewList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskRunReviewListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	query := input.query()
	if err := query.Validate("task_run_review_list"); err != nil {
		return toolspkg.ToolResult{}, nativeReviewToolError(req.ToolID, err)
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	reviews, err := n.deps.Tasks.ListRunReviews(ctx, query, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeReviewToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{nativeReviewToolsReviewsKey: reviews},
		fmt.Sprintf("%d reviews", len(reviews)),
	)
}

func (n *daemonNativeTools) taskRunReviewShow(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskRunReviewShowInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	reviewID, err := requiredNativeString(req.ToolID, "review_id", input.ReviewID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	review, err := n.deps.Tasks.GetRunReview(ctx, reviewID, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeReviewToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{nativeReviewToolsReviewKey: review},
		fmt.Sprintf("review %s", review.ReviewID),
	)
}

func (n *daemonNativeTools) submitRunReviewAvailability(
	ctx context.Context,
	scope toolspkg.Scope,
) toolspkg.Availability {
	if n == nil || n.deps == nil || n.deps.Tasks == nil {
		return toolspkg.Unavailable(toolspkg.ReasonDependencyMissing)
	}
	actor, sessionID, err := reviewToolActorContext(toolspkg.ToolIDTaskRunReviewSubmit, scope)
	if err != nil {
		return toolspkg.Unavailable(toolspkg.ReasonAutonomySessionRequired)
	}
	if _, err := n.deps.Tasks.LookupRunReviewForSession(ctx, sessionID, actor); err != nil {
		if isReviewBindingError(err) {
			return toolspkg.Unavailable(toolspkg.ReasonSessionDenied)
		}
		return toolspkg.Unavailable(toolspkg.ReasonBackendUnhealthy)
	}
	return toolspkg.Available()
}

func (n *daemonNativeTools) submitRunReview(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input submitRunReviewInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, sessionID, err := reviewToolActorContext(req.ToolID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	binding, err := n.lookupBoundRunReview(ctx, req.ToolID, sessionID, actor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	record, err := input.recordRequest(req.ToolID, &binding)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := n.deps.Tasks.RecordRunReview(ctx, record, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeReviewToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{
			nativeReviewToolsReviewKey: result.Review,
			"continuation_run":         result.ContinuationRun,
			"circuit_opened":           result.CircuitOpened,
		},
		submitRunReviewPreview(result),
	)
}

func (i taskRunReviewRequestInput) request(id toolspkg.ToolID) (taskpkg.RunReviewRequest, error) {
	taskID, err := requiredNativeString(id, "task_id", i.TaskID)
	if err != nil {
		return taskpkg.RunReviewRequest{}, err
	}
	runID, err := requiredNativeString(id, "run_id", i.RunID)
	if err != nil {
		return taskpkg.RunReviewRequest{}, err
	}
	request := taskpkg.RunReviewRequest{
		TaskID:         taskID,
		RunID:          runID,
		Policy:         taskpkg.ReviewPolicy(strings.TrimSpace(i.Policy)),
		ReviewRound:    i.ReviewRound,
		Attempt:        i.Attempt,
		ParentReviewID: strings.TrimSpace(i.ParentReviewID),
		Reason:         strings.TrimSpace(i.Reason),
	}
	normalized := request.Normalize()
	if err := normalized.Validate("task_run_review_request"); err != nil {
		return taskpkg.RunReviewRequest{}, nativeReviewToolError(id, err)
	}
	return normalized, nil
}

func (i taskRunReviewListInput) query() taskpkg.RunReviewQuery {
	return taskpkg.RunReviewQuery{
		TaskID:            strings.TrimSpace(i.TaskID),
		RunID:             strings.TrimSpace(i.RunID),
		Status:            taskpkg.RunReviewStatus(strings.TrimSpace(i.Status)).Normalize(),
		ReviewerSessionID: strings.TrimSpace(i.ReviewerSessionID),
		Limit:             i.Limit,
	}
}

func (i submitRunReviewInput) recordRequest(
	id toolspkg.ToolID,
	binding *taskpkg.RunReviewBinding,
) (taskpkg.RecordRunReviewRequest, error) {
	if binding == nil {
		return taskpkg.RecordRunReviewRequest{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			"review verdict requires a caller session binding",
			fmt.Errorf("%w: review binding is required", toolspkg.ErrToolDenied),
			toolspkg.ReasonSessionDenied,
		)
	}
	reviewID, err := requiredNativeString(id, "review_id", i.ReviewID)
	if err != nil {
		return taskpkg.RecordRunReviewRequest{}, err
	}
	runID, err := requiredNativeString(id, "run_id", i.RunID)
	if err != nil {
		return taskpkg.RecordRunReviewRequest{}, err
	}
	if reviewID != binding.Review.ReviewID || runID != binding.Review.RunID {
		return taskpkg.RecordRunReviewRequest{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			"review verdict does not match the caller session binding",
			fmt.Errorf("%w: review binding mismatch", toolspkg.ErrToolDenied),
			toolspkg.ReasonSessionDenied,
		)
	}
	confidence := i.Confidence
	if confidence == nil {
		return taskpkg.RecordRunReviewRequest{}, nativeRequiredInputError(id, "confidence")
	}
	missingItems := trimNativeStrings(i.MissingWork)
	if missingItems == nil {
		missingItems = []string{}
	}
	missingWork, err := json.Marshal(missingItems)
	if err != nil {
		return taskpkg.RecordRunReviewRequest{}, fmt.Errorf("daemon: marshal review missing_work: %w", err)
	}
	record := taskpkg.RecordRunReviewRequest{
		ReviewID: reviewID,
		RunID:    runID,
		Verdict: taskpkg.RunReviewVerdict{
			Outcome:           taskpkg.RunReviewOutcome(strings.TrimSpace(i.Outcome)),
			Confidence:        confidence,
			Reason:            strings.TrimSpace(i.Reason),
			DeliveryID:        strings.TrimSpace(i.DeliveryID),
			MissingWork:       json.RawMessage(missingWork),
			NextRoundGuidance: strings.TrimSpace(i.NextRoundGuidance),
			ReviewText:        strings.TrimSpace(i.ReviewText),
		},
	}
	normalized := record.Normalize()
	if err := normalized.Validate("submit_run_review"); err != nil {
		return taskpkg.RecordRunReviewRequest{}, nativeReviewToolError(id, err)
	}
	return normalized, nil
}

func (n *daemonNativeTools) lookupBoundRunReview(
	ctx context.Context,
	id toolspkg.ToolID,
	sessionID string,
	actor taskpkg.ActorContext,
) (taskpkg.RunReviewBinding, error) {
	if n == nil || n.deps == nil || n.deps.Tasks == nil {
		return taskpkg.RunReviewBinding{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			"task review authority is unavailable",
			fmt.Errorf("%w: task review authority is unavailable", toolspkg.ErrToolUnavailable),
			toolspkg.ReasonDependencyMissing,
		)
	}
	binding, err := n.deps.Tasks.LookupRunReviewForSession(ctx, sessionID, actor)
	if err != nil {
		return taskpkg.RunReviewBinding{}, nativeReviewToolError(id, err)
	}
	return binding, nil
}

func reviewToolActorContext(id toolspkg.ToolID, scope toolspkg.Scope) (taskpkg.ActorContext, string, error) {
	sessionID := strings.TrimSpace(scope.SessionID)
	if sessionID == "" {
		return taskpkg.ActorContext{}, "", toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			"submit_run_review requires a caller reviewer session",
			fmt.Errorf("%w: session_id is required", toolspkg.ErrToolDenied),
			toolspkg.ReasonAutonomySessionRequired,
		)
	}
	actor, err := taskpkg.DeriveAgentSessionActorContext(sessionID, scope.WorkspaceID)
	if err != nil {
		return taskpkg.ActorContext{}, "", nativeReviewToolError(id, err)
	}
	return actor, sessionID, nil
}

func submitRunReviewPreview(result taskpkg.RunReviewResult) string {
	parts := []string{
		nativeReviewToolsReviewKey,
		strings.TrimSpace(result.Review.ReviewID),
		string(result.Review.Outcome.Normalize()),
	}
	if result.ContinuationRun != nil {
		parts = append(parts, "continuation", strings.TrimSpace(result.ContinuationRun.ID))
	}
	return strings.Join(trimNativeStrings(parts), " ")
}

func nativeReviewToolError(id toolspkg.ToolID, err error) error {
	if err == nil {
		return nil
	}
	message := taskpkg.RedactClaimTokens(err.Error())
	switch {
	case isReviewBindingError(err):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			message,
			fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			toolspkg.ReasonSessionDenied,
		)
	case errors.Is(err, taskpkg.ErrValidation),
		errors.Is(err, taskpkg.ErrPayloadTooLarge),
		errors.Is(err, taskpkg.ErrImmutableField):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			message,
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, taskpkg.ErrRunReviewNotFound):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			message,
			fmt.Errorf("%w: %w", toolspkg.ErrToolNotFound, err),
			toolspkg.ReasonToolUnknown,
		)
	case errors.Is(err, taskpkg.ErrConflict),
		errors.Is(err, taskpkg.ErrInvalidStatusTransition):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			id,
			message,
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
			toolspkg.ReasonPolicyDenied,
		)
	default:
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			id,
			message,
			fmt.Errorf("%w: %w", toolspkg.ErrToolBackendFailed, err),
		)
	}
}

func isReviewBindingError(err error) bool {
	return errors.Is(err, taskpkg.ErrRunReviewNotFound) ||
		errors.Is(err, taskpkg.ErrPermissionDenied)
}
