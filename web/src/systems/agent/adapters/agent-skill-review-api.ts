import {
  apiClient,
  apiErrorCode,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type { AgentSkillReviewPayload } from "../types";

/** A refused review call, with the daemon's stable code such as `skill_review_stale`. */
export class AgentSkillReviewApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(message: string, status: number, code = "") {
    super(message);
    this.name = "AgentSkillReviewApiError";
    this.status = status;
    this.code = code;
  }
}

/** Which review, and the profile that owns its instance: the route refuses any other. */
export interface AgentSkillReviewRef {
  id: string;
  profile?: string;
}

function profileQuery(profile?: string) {
  return profile ? { profile } : {};
}

function failure(fallback: string, response: Response, error: unknown) {
  return new AgentSkillReviewApiError(
    defaultApiErrorMessage(fallback, response, error),
    response.status,
    apiErrorCode(error) ?? ""
  );
}

/** One review with its exact contents. */
export async function fetchAgentSkillReview(
  ref: AgentSkillReviewRef,
  signal?: AbortSignal
): Promise<AgentSkillReviewPayload> {
  const { data, error, response } = await apiClient.GET("/api/agent-skill-reviews/{id}", {
    params: { path: { id: ref.id }, query: profileQuery(ref.profile) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw failure("Failed to fetch the skill review", response, error);
  }
  return requireResponseData(data, response, "Failed to fetch the skill review").review;
}

export type AgentSkillReviewDecision = "approve" | "reject";

/** Approves or rejects the exact contents the operator read, named by their digest. */
export async function decideAgentSkillReview(
  ref: AgentSkillReviewRef,
  decision: AgentSkillReviewDecision,
  digest: string
): Promise<AgentSkillReviewPayload> {
  const options = {
    params: { path: { id: ref.id }, query: profileQuery(ref.profile) },
    body: { digest },
  };
  const { data, error, response } =
    decision === "approve"
      ? await apiClient.POST("/api/agent-skill-reviews/{id}/approve", options)
      : await apiClient.POST("/api/agent-skill-reviews/{id}/reject", options);
  const fallback = decision === "approve" ? "Failed to approve" : "Failed to reject";
  if (apiRequestFailed(response, error)) throw failure(fallback, response, error);
  return requireResponseData(data, response, fallback).review;
}

/**
 * The reviewed digest still matches but CompozyOS can't install the draft (its bytes, name or file
 * paths fail verification). Only rejection settles it.
 */
export function isSkillReviewInvalid(error: unknown) {
  return error instanceof AgentSkillReviewApiError && error.code === "skill_review_invalid";
}

/** The contents changed after the operator read them, so the decision did not apply. */
export function isSkillReviewStale(error: unknown) {
  return error instanceof AgentSkillReviewApiError && error.code === "skill_review_stale";
}
