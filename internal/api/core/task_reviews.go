package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

// RequestTaskRunReview creates or replays one authoritative review request for a task run.
func (h *BaseHandlers) RequestTaskRunReview(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.CreateTaskRunReviewRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode run review request: %w", h.transportName(), err)),
		)
		return
	}

	actor, err := h.taskActorContext(c, taskActionRequestReview)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	view, err := manager.RunDetail(c.Request.Context(), runID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	reviewReq, err := taskRunReviewRequestFromRequest(runID, view.Run.TaskID, &req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	review, created, err := manager.RequestRunReview(c.Request.Context(), reviewReq, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, contract.TaskRunReviewRequestResponse{Review: review, Created: created})
}

// ListTaskRunReviews lists reviews scoped to one task run.
func (h *BaseHandlers) ListTaskRunReviews(c *gin.Context) {
	h.listTaskRunReviews(c, "", c.Param("id"))
}

// ListTaskReviews lists reviews scoped to one task.
func (h *BaseHandlers) ListTaskReviews(c *gin.Context) {
	h.listTaskRunReviews(c, c.Param("id"), "")
}

func (h *BaseHandlers) listTaskRunReviews(c *gin.Context, taskID string, runID string) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	query, err := parseTaskRunReviewQuery(c, taskID, runID)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionListReviews)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	reviews, err := manager.ListRunReviews(c.Request.Context(), query, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskRunReviewsResponse{Reviews: TaskRunReviewPayloadsFromReviews(reviews)})
}

// GetTaskRunReview returns one task-run review.
func (h *BaseHandlers) GetTaskRunReview(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	reviewID, err := requiredPathID(c.Param("id"), "review id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionGetReview)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	review, err := manager.GetRunReview(c.Request.Context(), reviewID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TaskRunReviewResponse{Review: review})
}

// SubmitTaskRunReviewVerdict records one reviewer verdict for a review request.
func (h *BaseHandlers) SubmitTaskRunReviewVerdict(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	reviewID, err := requiredPathID(c.Param("id"), "review id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	var req contract.SubmitTaskRunReviewVerdictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode run review verdict request: %w", h.transportName(), err)),
		)
		return
	}

	recordReq, err := taskRunReviewVerdictFromRequest(reviewID, &req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	actor, err := h.taskActorContext(c, taskActionSubmitReview)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	result, err := manager.RecordRunReview(c.Request.Context(), recordReq, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, TaskRunReviewVerdictResponseFromResult(&result))
}

func taskRunReviewRequestFromRequest(
	runID string,
	taskID string,
	req *contract.CreateTaskRunReviewRequest,
) (taskpkg.RunReviewRequest, error) {
	trimmedRunID := strings.TrimSpace(runID)
	trimmedTaskID := strings.TrimSpace(taskID)
	if trimmedRunID == "" {
		return taskpkg.RunReviewRequest{}, NewTaskValidationError(errors.New("run id is required"))
	}
	if trimmedTaskID == "" {
		return taskpkg.RunReviewRequest{}, NewTaskValidationError(errors.New("task id is required"))
	}
	if req == nil {
		return taskpkg.RunReviewRequest{}, NewTaskValidationError(errors.New("run review request is required"))
	}
	if rawRunID := strings.TrimSpace(req.RunID); rawRunID != "" && rawRunID != trimmedRunID {
		return taskpkg.RunReviewRequest{}, NewTaskValidationError(fmt.Errorf(
			"task_run_review.run_id must match run id %q",
			trimmedRunID,
		))
	}
	if rawTaskID := strings.TrimSpace(req.TaskID); rawTaskID != "" && rawTaskID != trimmedTaskID {
		return taskpkg.RunReviewRequest{}, NewTaskValidationError(fmt.Errorf(
			"task_run_review.task_id must match task id %q",
			trimmedTaskID,
		))
	}

	request := *req
	request.RunID = trimmedRunID
	request.TaskID = trimmedTaskID
	request = request.Normalize()
	if err := request.Validate("task_run_review"); err != nil {
		return taskpkg.RunReviewRequest{}, err
	}
	return request, nil
}

func taskRunReviewVerdictFromRequest(
	reviewID string,
	req *contract.SubmitTaskRunReviewVerdictRequest,
) (taskpkg.RecordRunReviewRequest, error) {
	trimmedReviewID := strings.TrimSpace(reviewID)
	if trimmedReviewID == "" {
		return taskpkg.RecordRunReviewRequest{}, NewTaskValidationError(errors.New("review id is required"))
	}
	if req == nil {
		return taskpkg.RecordRunReviewRequest{}, NewTaskValidationError(
			errors.New("run review verdict request is required"),
		)
	}

	recordReq := taskpkg.RecordRunReviewRequest{
		ReviewID: trimmedReviewID,
		RunID:    strings.TrimSpace(req.RunID),
		Verdict:  req.Verdict,
	}.Normalize()
	if err := recordReq.Validate("task_run_review_verdict"); err != nil {
		return taskpkg.RecordRunReviewRequest{}, err
	}
	return recordReq, nil
}

func parseTaskRunReviewQuery(c *gin.Context, taskID string, runID string) (taskpkg.RunReviewQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return taskpkg.RunReviewQuery{}, NewTaskValidationError(err)
	}

	query := taskpkg.RunReviewQuery{
		TaskID:            strings.TrimSpace(taskID),
		RunID:             strings.TrimSpace(runID),
		Status:            taskpkg.RunReviewStatus(strings.TrimSpace(c.Query("status"))).Normalize(),
		ReviewerSessionID: strings.TrimSpace(c.Query("reviewer_session_id")),
		Limit:             limit,
	}
	if err := query.Validate("task_run_review_query"); err != nil {
		return taskpkg.RunReviewQuery{}, err
	}
	return query, nil
}

// TaskRunReviewPayloadsFromReviews converts review records into shared payloads.
func TaskRunReviewPayloadsFromReviews(reviews []taskpkg.RunReview) []contract.TaskRunReviewPayload {
	payloads := make([]contract.TaskRunReviewPayload, 0, len(reviews))
	payloads = append(payloads, reviews...)
	return payloads
}

// TaskRunReviewVerdictResponseFromResult converts one verdict result into a shared payload.
func TaskRunReviewVerdictResponseFromResult(result *taskpkg.RunReviewResult) contract.TaskRunReviewVerdictResponse {
	if result == nil {
		return contract.TaskRunReviewVerdictResponse{}
	}
	response := contract.TaskRunReviewVerdictResponse{
		Review:        result.Review,
		CircuitOpened: result.CircuitOpened,
	}
	if result.ContinuationRun != nil {
		continuation := TaskRunPayloadFromRun(result.ContinuationRun)
		response.ContinuationRun = &continuation
	}
	return response
}
