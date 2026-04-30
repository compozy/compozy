import { apiErrorMessage, daemonApiClient, requireData } from "@/lib/api-client";
import { ACTIVE_WORKSPACE_HEADER } from "@/systems/app-shell";

import type {
  ReviewDetailPayload,
  ReviewIssue,
  ReviewRound,
  ReviewRunRequest,
  ReviewSummary,
  Run,
} from "../types";

function workspaceHeader(workspaceId: string) {
  return { header: { [ACTIVE_WORKSPACE_HEADER]: workspaceId } } as const;
}

export interface ReviewSummaryParams {
  workspaceId: string;
  slug: string;
}

export async function getLatestReview(params: ReviewSummaryParams): Promise<ReviewSummary> {
  const { data, error, response } = await daemonApiClient.GET("/api/reviews/{slug}", {
    params: {
      path: { slug: params.slug },
      ...workspaceHeader(params.workspaceId),
    },
  });
  const payload = requireData(
    data,
    response,
    `Failed to load latest review for ${params.slug}`,
    error
  );
  return payload.review;
}

export interface ReviewRoundParams {
  workspaceId: string;
  slug: string;
  round: number;
}

export async function getReviewRound(params: ReviewRoundParams): Promise<ReviewRound> {
  const { data, error, response } = await daemonApiClient.GET(
    "/api/reviews/{slug}/rounds/{round}",
    {
      params: {
        path: { slug: params.slug, round: params.round },
        ...workspaceHeader(params.workspaceId),
      },
    }
  );
  const payload = requireData(
    data,
    response,
    `Failed to load review round ${params.round} for ${params.slug}`,
    error
  );
  return payload.round;
}

export interface ReviewIssuesParams {
  workspaceId: string;
  slug: string;
  round: number;
}

export async function listReviewIssues(params: ReviewIssuesParams): Promise<ReviewIssue[]> {
  const { data, error, response } = await daemonApiClient.GET(
    "/api/reviews/{slug}/rounds/{round}/issues",
    {
      params: {
        path: { slug: params.slug, round: params.round },
        ...workspaceHeader(params.workspaceId),
      },
    }
  );
  const payload = requireData(
    data,
    response,
    `Failed to load review issues for ${params.slug} round ${params.round}`,
    error
  );
  return payload.issues ?? [];
}

export interface ReviewIssueParams {
  workspaceId: string;
  slug: string;
  round: number;
  issueId: string;
}

export async function getReviewIssue(params: ReviewIssueParams): Promise<ReviewDetailPayload> {
  const { data, error, response } = await daemonApiClient.GET(
    "/api/reviews/{slug}/rounds/{round}/issues/{issue_id}",
    {
      params: {
        path: { slug: params.slug, round: params.round, issue_id: params.issueId },
        ...workspaceHeader(params.workspaceId),
      },
    }
  );
  const payload = requireData(
    data,
    response,
    `Failed to load review issue ${params.issueId} for ${params.slug} round ${params.round}`,
    error
  );
  return payload.review;
}

export interface StartReviewRunParams {
  workspaceId: string;
  slug: string;
  round: number;
  body?: ReviewRunRequest;
}

export async function startReviewRun(params: StartReviewRunParams): Promise<Run> {
  const { data, error, response } = await daemonApiClient.POST(
    "/api/reviews/{slug}/rounds/{round}/runs",
    {
      params: {
        path: { slug: params.slug, round: params.round },
        ...workspaceHeader(params.workspaceId),
      },
      body: {
        ...params.body,
        workspace: params.workspaceId,
      },
    }
  );
  if (!data) {
    throw new Error(
      apiErrorMessage(
        error,
        `Failed to dispatch review fix for ${params.slug} round ${params.round}`
      )
    );
  }
  const payload = requireData(
    data,
    response,
    `Failed to dispatch review fix for ${params.slug} round ${params.round}`,
    error
  );
  return payload.run;
}
