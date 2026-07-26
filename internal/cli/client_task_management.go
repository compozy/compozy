package cli

import (
	"context"

	"errors"

	"net/http"
	"net/url"

	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
)

func (c *unixSocketClient) ListTasks(ctx context.Context, query TaskListQuery) (TaskListRecord, error) {
	var response contract.TasksResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/tasks", taskValues(query), nil, &response); err != nil {
		return TaskListRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) CreateTask(ctx context.Context, request CreateTaskRequest) (TaskRecord, error) {
	var response contract.TaskResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/tasks", nil, request, &response); err != nil {
		return TaskRecord{}, err
	}
	return response.Task, nil
}

func (c *unixSocketClient) CreateTaskAsAgent(
	ctx context.Context,
	request CreateTaskRequest,
	credentials agentidentity.Credentials,
) (TaskRecord, error) {
	var response contract.TaskResponse
	if err := c.doAgentJSON(ctx, http.MethodPost, "/api/tasks", nil, request, credentials, &response); err != nil {
		return TaskRecord{}, err
	}
	return response.Task, nil
}

func (c *unixSocketClient) GetTask(ctx context.Context, id string) (TaskDetailRecord, error) {
	var response contract.TaskDetailResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return TaskDetailRecord{}, err
	}
	return response.Task, nil
}

func (c *unixSocketClient) InspectTask(ctx context.Context, id string) (TaskInspectRecord, error) {
	var response contract.TaskInspectResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/inspect"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return TaskInspectRecord{}, err
	}
	return response.Inspect, nil
}

func (c *unixSocketClient) InspectRun(ctx context.Context, id string) (TaskInspectRecord, error) {
	var response contract.TaskInspectResponse
	path := "/api/runs/" + url.PathEscape(strings.TrimSpace(id)) + "/inspect"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return TaskInspectRecord{}, err
	}
	return response.Inspect, nil
}

func (c *unixSocketClient) UpdateTask(ctx context.Context, id string, request UpdateTaskRequest) (TaskRecord, error) {
	var response contract.TaskResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodPatch, path, nil, request, &response); err != nil {
		return TaskRecord{}, err
	}
	return response.Task, nil
}

func (c *unixSocketClient) BlockTask(
	ctx context.Context,
	id string,
	request CreateTaskBlockRequest,
) (TaskBlockRecord, error) {
	var response contract.TaskBlockResponse
	if err := c.doJSON(ctx, http.MethodPost, taskBlocksPath(id), nil, request, &response); err != nil {
		return TaskBlockRecord{}, err
	}
	return response.Block, nil
}

func (c *unixSocketClient) BlockTaskAsAgent(
	ctx context.Context,
	id string,
	request CreateTaskBlockRequest,
	credentials agentidentity.Credentials,
) (TaskBlockRecord, error) {
	var response contract.TaskBlockResponse
	if err := c.doAgentJSON(
		ctx,
		http.MethodPost,
		taskBlocksPath(id),
		nil,
		request,
		credentials,
		&response,
	); err != nil {
		return TaskBlockRecord{}, err
	}
	return response.Block, nil
}

func (c *unixSocketClient) ListTaskBlocks(
	ctx context.Context,
	id string,
	includeCleared bool,
) ([]TaskBlockRecord, error) {
	var response contract.TaskBlocksResponse
	values := url.Values{}
	if includeCleared {
		values.Set("include_cleared", "true")
	}
	if err := c.doJSON(ctx, http.MethodGet, taskBlocksPath(id), values, nil, &response); err != nil {
		return nil, err
	}
	return response.Blocks, nil
}

func (c *unixSocketClient) ClearTaskBlock(
	ctx context.Context,
	id string,
	blockID string,
	request ClearTaskBlockRequest,
) (TaskBlockRecord, error) {
	var response contract.TaskBlockResponse
	path := taskBlocksPath(id) + "/" + url.PathEscape(strings.TrimSpace(blockID)) + "/clear"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return TaskBlockRecord{}, err
	}
	return response.Block, nil
}

func (c *unixSocketClient) ClearTaskBlockAsAgent(
	ctx context.Context,
	id string,
	blockID string,
	request ClearTaskBlockRequest,
	credentials agentidentity.Credentials,
) (TaskBlockRecord, error) {
	var response contract.TaskBlockResponse
	path := taskBlocksPath(id) + "/" + url.PathEscape(strings.TrimSpace(blockID)) + "/clear"
	if err := c.doAgentJSON(ctx, http.MethodPost, path, nil, request, credentials, &response); err != nil {
		return TaskBlockRecord{}, err
	}
	return response.Block, nil
}

func (c *unixSocketClient) RecoverTask(
	ctx context.Context,
	id string,
	request RecoverTaskRequest,
) (TaskRecord, error) {
	var response contract.TaskResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/recover"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return TaskRecord{}, err
	}
	return response.Task, nil
}

func taskBlocksPath(id string) string {
	return "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/blocks"
}

func (c *unixSocketClient) GetTaskExecutionProfile(
	ctx context.Context,
	id string,
) (TaskExecutionProfileRecord, error) {
	var response contract.TaskExecutionProfileResponse
	path := taskExecutionProfilePath(id)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return TaskExecutionProfileRecord{}, err
	}
	return response.Profile, nil
}

func (c *unixSocketClient) SetTaskExecutionProfile(
	ctx context.Context,
	id string,
	request *TaskExecutionProfileRequest,
) (TaskExecutionProfileRecord, error) {
	if request == nil {
		return TaskExecutionProfileRecord{}, errors.New("cli: task execution profile request is required")
	}
	var response contract.TaskExecutionProfileResponse
	path := taskExecutionProfilePath(id)
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return TaskExecutionProfileRecord{}, err
	}
	return response.Profile, nil
}

func (c *unixSocketClient) DeleteTaskExecutionProfile(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, taskExecutionProfilePath(id), nil, nil, nil)
}

func taskExecutionProfilePath(id string) string {
	return "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/execution-profile"
}

func (c *unixSocketClient) CreateTaskBridgeNotificationSubscription(
	ctx context.Context,
	taskID string,
	request *TaskBridgeNotificationSubscriptionRequest,
) (TaskBridgeNotificationSubscriptionRecord, error) {
	if request == nil {
		return TaskBridgeNotificationSubscriptionRecord{}, errors.New(
			"cli: task bridge notification subscription request is required",
		)
	}
	var response contract.TaskBridgeNotificationSubscriptionResponse
	path := taskBridgeNotificationSubscriptionsPath(taskID)
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return TaskBridgeNotificationSubscriptionRecord{}, err
	}
	return response.Subscription, nil
}

func (c *unixSocketClient) ListTaskBridgeNotificationSubscriptions(
	ctx context.Context,
	taskID string,
	query TaskBridgeNotificationSubscriptionQuery,
) ([]TaskBridgeNotificationSubscriptionRecord, error) {
	var response contract.TaskBridgeNotificationSubscriptionsResponse
	path := taskBridgeNotificationSubscriptionsPath(taskID)
	values := taskBridgeNotificationSubscriptionValues(query)
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return nil, err
	}
	return response.Subscriptions, nil
}

func (c *unixSocketClient) GetTaskBridgeNotificationSubscription(
	ctx context.Context,
	taskID string,
	subscriptionID string,
) (TaskBridgeNotificationSubscriptionRecord, error) {
	var response contract.TaskBridgeNotificationSubscriptionResponse
	path := taskBridgeNotificationSubscriptionPath(taskID, subscriptionID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return TaskBridgeNotificationSubscriptionRecord{}, err
	}
	return response.Subscription, nil
}

func (c *unixSocketClient) DeleteTaskBridgeNotificationSubscription(
	ctx context.Context,
	taskID string,
	subscriptionID string,
) error {
	path := taskBridgeNotificationSubscriptionPath(taskID, subscriptionID)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
}

func taskBridgeNotificationSubscriptionsPath(taskID string) string {
	return "/api/tasks/" + url.PathEscape(strings.TrimSpace(taskID)) + "/notifications/bridges"
}

func taskBridgeNotificationSubscriptionPath(taskID string, subscriptionID string) string {
	return taskBridgeNotificationSubscriptionsPath(taskID) + "/" + url.PathEscape(strings.TrimSpace(subscriptionID))
}

func (c *unixSocketClient) RequestTaskRunReview(
	ctx context.Context,
	runID string,
	request *TaskRunReviewRequest,
) (TaskRunReviewRequestRecord, error) {
	if request == nil {
		return TaskRunReviewRequestRecord{}, errors.New("cli: task run review request is required")
	}
	var response contract.TaskRunReviewRequestResponse
	path := "/api/task-runs/" + url.PathEscape(strings.TrimSpace(runID)) + "/reviews"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return TaskRunReviewRequestRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) RequestTaskRunReviewAsAgent(
	ctx context.Context,
	runID string,
	request *TaskRunReviewRequest,
	credentials agentidentity.Credentials,
) (TaskRunReviewRequestRecord, error) {
	if request == nil {
		return TaskRunReviewRequestRecord{}, errors.New("cli: task run review request is required")
	}
	var response contract.TaskRunReviewRequestResponse
	path := "/api/task-runs/" + url.PathEscape(strings.TrimSpace(runID)) + "/reviews"
	if err := c.doAgentJSON(ctx, http.MethodPost, path, nil, request, credentials, &response); err != nil {
		return TaskRunReviewRequestRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListTaskRunReviews(
	ctx context.Context,
	query TaskRunReviewListQuery,
) ([]TaskRunReviewRecord, error) {
	var response contract.TaskRunReviewsResponse
	path, values, err := taskRunReviewListPathAndValues(query)
	if err != nil {
		return nil, err
	}
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return nil, err
	}
	return response.Reviews, nil
}

func (c *unixSocketClient) GetTaskRunReview(
	ctx context.Context,
	reviewID string,
) (TaskRunReviewRecord, error) {
	var response contract.TaskRunReviewResponse
	path := "/api/task-reviews/" + url.PathEscape(strings.TrimSpace(reviewID))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return TaskRunReviewRecord{}, err
	}
	return response.Review, nil
}

func (c *unixSocketClient) SubmitTaskRunReviewVerdict(
	ctx context.Context,
	reviewID string,
	request *TaskRunReviewVerdictRequest,
) (TaskRunReviewVerdictRecord, error) {
	if request == nil {
		return TaskRunReviewVerdictRecord{}, errors.New("cli: task run review verdict request is required")
	}
	var response contract.TaskRunReviewVerdictResponse
	path := "/api/task-reviews/" + url.PathEscape(strings.TrimSpace(reviewID)) + "/verdict"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return TaskRunReviewVerdictRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) SubmitTaskRunReviewVerdictAsAgent(
	ctx context.Context,
	reviewID string,
	request *TaskRunReviewVerdictRequest,
	credentials agentidentity.Credentials,
) (TaskRunReviewVerdictRecord, error) {
	if request == nil {
		return TaskRunReviewVerdictRecord{}, errors.New("cli: task run review verdict request is required")
	}
	var response contract.TaskRunReviewVerdictResponse
	path := "/api/task-reviews/" + url.PathEscape(strings.TrimSpace(reviewID)) + "/verdict"
	if err := c.doAgentJSON(ctx, http.MethodPost, path, nil, request, credentials, &response); err != nil {
		return TaskRunReviewVerdictRecord{}, err
	}
	return response, nil
}
