import { useMutation, useQuery, useQueryClient, type QueryKey } from "@tanstack/react-query";

import { runKeys } from "@/systems/runs";

import {
  getLatestReview,
  getReviewIssue,
  listReviewIssues,
  startReviewRun,
  type StartReviewRunParams,
} from "../adapters/reviews-api";
import { reviewKeys } from "../lib/query-keys";
import type { ReviewDetailPayload, ReviewIssue, ReviewSummary, Run } from "../types";

export function useLatestReview(workspaceId: string | null, slug: string | null) {
  return useQuery<ReviewSummary>({
    queryKey: reviewKeys.summary(workspaceId ?? "none", slug ?? "none") as QueryKey,
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load the latest review");
      }
      if (!slug) {
        throw new Error("workflow slug is required to load the latest review");
      }
      return getLatestReview({ workspaceId, slug });
    },
    enabled: Boolean(workspaceId) && Boolean(slug),
  });
}

export function useReviewIssues(
  workspaceId: string | null,
  slug: string | null,
  round: number | null
) {
  return useQuery<ReviewIssue[]>({
    queryKey: reviewKeys.issues(workspaceId ?? "none", slug ?? "none", round ?? -1) as QueryKey,
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load review issues");
      }
      if (!slug) {
        throw new Error("workflow slug is required to load review issues");
      }
      if (round == null) {
        throw new Error("review round is required to load review issues");
      }
      return listReviewIssues({ workspaceId, slug, round });
    },
    enabled: Boolean(workspaceId) && Boolean(slug) && round != null && round >= 0,
  });
}

export function useReviewIssue(
  workspaceId: string | null,
  slug: string | null,
  round: number | null,
  issueId: string | null
) {
  return useQuery<ReviewDetailPayload>({
    queryKey: reviewKeys.issue(
      workspaceId ?? "none",
      slug ?? "none",
      round ?? -1,
      issueId ?? "none"
    ) as QueryKey,
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load a review issue");
      }
      if (!slug) {
        throw new Error("workflow slug is required to load a review issue");
      }
      if (round == null) {
        throw new Error("review round is required to load a review issue");
      }
      if (!issueId) {
        throw new Error("issue id is required to load a review issue");
      }
      return getReviewIssue({ workspaceId, slug, round, issueId });
    },
    enabled:
      Boolean(workspaceId) && Boolean(slug) && round != null && round >= 0 && Boolean(issueId),
  });
}

export function useStartReviewRun() {
  const queryClient = useQueryClient();
  return useMutation<Run, Error, StartReviewRunParams>({
    mutationFn: params => startReviewRun(params),
    onSuccess: (_result, variables) => {
      void queryClient.invalidateQueries({
        queryKey: reviewKeys.issues(
          variables.workspaceId,
          variables.slug,
          variables.round
        ) as QueryKey,
      });
      void queryClient.invalidateQueries({
        queryKey: reviewKeys.summary(variables.workspaceId, variables.slug) as QueryKey,
      });
      void queryClient.invalidateQueries({ queryKey: runKeys.lists() as QueryKey });
    },
  });
}
