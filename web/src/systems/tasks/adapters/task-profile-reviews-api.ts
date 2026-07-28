import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  TaskExecutionProfile,
  TaskExecutionProfileSetRequest,
  TaskReviewsFilter,
  TaskRunReview,
  TaskRunReviewRequest,
  TaskRunReviewRequestResult,
  TaskRunReviewVerdictRequest,
  TaskRunReviewVerdictResult,
  TaskRunReviewsFilter,
} from "../types";
import { normalizeOptionalText, TasksApiError } from "./tasks-api-errors";

export async function getTaskExecutionProfile(
  id: string,
  signal?: AbortSignal
): Promise<TaskExecutionProfile> {
  const { data, error, response } = await apiClient.GET("/api/tasks/{id}/execution-profile", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to fetch execution profile for task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch execution profile for task "${id}"`)
    .profile;
}

export async function setTaskExecutionProfile(
  id: string,
  body: TaskExecutionProfileSetRequest,
  signal?: AbortSignal
): Promise<TaskExecutionProfile> {
  const { data, error, response } = await apiClient.PUT("/api/tasks/{id}/execution-profile", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    if (response.status === 409) {
      throw new TasksApiError(
        defaultApiErrorMessage(`Execution profile conflict for task "${id}"`, response, error),
        409
      );
    }
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to set execution profile for task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to set execution profile for task "${id}"`)
    .profile;
}

export async function deleteTaskExecutionProfile(id: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/tasks/{id}/execution-profile", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(
        `Failed to delete execution profile for task "${id}"`,
        response,
        error
      ),
      response.status
    );
  }
}

export async function listTaskRunReviews(
  runId: string,
  filters: TaskRunReviewsFilter = {},
  signal?: AbortSignal
): Promise<TaskRunReview[]> {
  const { data, error, response } = await apiClient.GET("/api/task-runs/{id}/reviews", {
    params: {
      path: { id: runId },
      query: {
        status: filters.status,
        reviewer_session_id: normalizeOptionalText(filters.reviewer_session_id),
        limit: filters.limit,
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task run not found: ${runId}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to fetch reviews for task run "${runId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch reviews for task run "${runId}"`)
    .reviews;
}

export async function listTaskReviews(
  taskId: string,
  filters: TaskReviewsFilter = {},
  signal?: AbortSignal
): Promise<TaskRunReview[]> {
  const { data, error, response } = await apiClient.GET("/api/tasks/{id}/reviews", {
    params: {
      path: { id: taskId },
      query: {
        status: filters.status,
        reviewer_session_id: normalizeOptionalText(filters.reviewer_session_id),
        limit: filters.limit,
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${taskId}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to fetch reviews for task "${taskId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch reviews for task "${taskId}"`)
    .reviews;
}

export async function requestTaskRunReview(
  runId: string,
  body: TaskRunReviewRequest,
  signal?: AbortSignal
): Promise<TaskRunReviewRequestResult> {
  const { data, error, response } = await apiClient.POST("/api/task-runs/{id}/reviews", {
    params: { path: { id: runId } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task run not found: ${runId}`, 404);
    if (response.status === 409) {
      throw new TasksApiError(
        defaultApiErrorMessage(`Review request conflict for task run "${runId}"`, response, error),
        409
      );
    }
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to request review for task run "${runId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to request review for task run "${runId}"`);
}

export async function getTaskRunReview(
  reviewId: string,
  signal?: AbortSignal
): Promise<TaskRunReview> {
  const { data, error, response } = await apiClient.GET("/api/task-reviews/{id}", {
    params: { path: { id: reviewId } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task review not found: ${reviewId}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to fetch task review "${reviewId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch task review "${reviewId}"`).review;
}

export async function submitTaskRunReviewVerdict(
  reviewId: string,
  body: TaskRunReviewVerdictRequest,
  signal?: AbortSignal
): Promise<TaskRunReviewVerdictResult> {
  const { data, error, response } = await apiClient.POST("/api/task-reviews/{id}/verdict", {
    params: { path: { id: reviewId } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task review not found: ${reviewId}`, 404);
    if (response.status === 409) {
      throw new TasksApiError(
        defaultApiErrorMessage(`Review verdict conflict for review "${reviewId}"`, response, error),
        409
      );
    }
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to submit verdict for review "${reviewId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to submit verdict for review "${reviewId}"`);
}
