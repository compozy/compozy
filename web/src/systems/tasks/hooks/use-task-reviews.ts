import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { requestTaskRunReview, submitTaskRunReviewVerdict } from "../adapters/tasks-api";
import {
  taskReviewsOptions,
  taskRunReviewDetailOptions,
  taskRunReviewsOptions,
} from "../lib/query-options";
import { tasksKeys } from "../lib/query-keys";
import { acknowledgeTaskMutationSettlement } from "../lib/task-mutation";
import type {
  TaskReviewsFilter,
  TaskRunReviewRequest,
  TaskRunReviewVerdictRequest,
  TaskRunReviewsFilter,
} from "../types";

interface QueryHookOptions {
  enabled?: boolean;
}

interface RequestReviewParams {
  runId: string;
  data: TaskRunReviewRequest;
}

interface SubmitVerdictParams {
  reviewId: string;
  data: TaskRunReviewVerdictRequest;
}

type QueryClient = ReturnType<typeof useQueryClient>;

function invalidateReviewQueries(
  queryClient: QueryClient,
  options: { runId?: string; taskId?: string; reviewId?: string }
) {
  const pending: Promise<void>[] = [
    queryClient.invalidateQueries({ queryKey: tasksKeys.reviewsRoot() }),
  ];

  if (options.runId) {
    pending.push(queryClient.invalidateQueries({ queryKey: tasksKeys.runDetail(options.runId) }));
  }
  if (options.taskId) {
    pending.push(
      queryClient.invalidateQueries({ queryKey: tasksKeys.detail(options.taskId) }),
      queryClient.invalidateQueries({ queryKey: tasksKeys.timelineRoot() })
    );
  }
  if (options.reviewId) {
    pending.push(
      queryClient.invalidateQueries({ queryKey: tasksKeys.reviewDetail(options.reviewId) })
    );
  }

  pending.push(
    queryClient.invalidateQueries({ queryKey: tasksKeys.runDetails() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.lists() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.dashboardRoot() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.inboxRoot() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.contextBundle() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.agentContext() })
  );

  return Promise.all(pending);
}

export function useTaskRunReviews(
  runId: string,
  filters: TaskRunReviewsFilter = {},
  options: QueryHookOptions = {}
) {
  return useQuery(taskRunReviewsOptions(runId, filters, options.enabled ?? true));
}

export function useTaskReviews(
  taskId: string,
  filters: TaskReviewsFilter = {},
  options: QueryHookOptions = {}
) {
  return useQuery(taskReviewsOptions(taskId, filters, options.enabled ?? true));
}

export function useTaskRunReview(reviewId: string, options: QueryHookOptions = {}) {
  return useQuery(taskRunReviewDetailOptions(reviewId, options.enabled ?? true));
}

export function useRequestTaskRunReview() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ runId, data }: RequestReviewParams) => requestTaskRunReview(runId, data),
    onSettled: (result, error, { runId, data }) => {
      acknowledgeTaskMutationSettlement(error);
      return invalidateReviewQueries(queryClient, {
        runId,
        taskId: result?.review.task_id ?? data.task_id,
        reviewId: result?.review.review_id,
      });
    },
  });
}

export function useSubmitTaskRunReviewVerdict() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ reviewId, data }: SubmitVerdictParams) =>
      submitTaskRunReviewVerdict(reviewId, data),
    onSettled: (result, error, { reviewId, data }) => {
      acknowledgeTaskMutationSettlement(error);
      return invalidateReviewQueries(queryClient, {
        runId: data.run_id,
        taskId: result?.review.task_id,
        reviewId,
      });
    },
  });
}
