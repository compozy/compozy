import type { PillTone } from "@agh/ui";

import type { TaskRun, TaskRunReview, TaskRunReviewOutcome, TaskTimelineItem } from "../types";

const METRIC_PLACEHOLDER = "—";

const REVIEW_OUTCOME_TONE: Record<TaskRunReviewOutcome, PillTone> = {
  approved: "success",
  rejected: "warning",
  blocked: "danger",
  error: "danger",
  timeout: "danger",
  invalid_output: "danger",
};

export function formatTaskRunMetric(value?: number | null): string {
  return typeof value === "number" && Number.isFinite(value)
    ? value.toLocaleString()
    : METRIC_PLACEHOLDER;
}

export function latestTaskRun(
  runs: readonly TaskRun[],
  status?: TaskRun["status"]
): TaskRun | undefined {
  return [...runs]
    .filter(run => !status || run.status === status)
    .sort((left, right) => right.attempt - left.attempt)[0];
}

export function taskRunLineage(
  run: TaskRun,
  runs: readonly TaskRun[]
): { previous: TaskRun | null; next: TaskRun | null } {
  return {
    previous: run.previous_run_id
      ? (runs.find(candidate => candidate.id === run.previous_run_id) ?? null)
      : null,
    next: runs.find(candidate => candidate.previous_run_id === run.id) ?? null,
  };
}

export function taskRunTimelineItems(
  timeline: readonly TaskTimelineItem[],
  runId: string
): TaskTimelineItem[] {
  return timeline.filter(item => item.run?.id === runId);
}

export interface TaskRunReviewPresentation {
  line: string;
  title: string;
  tone: PillTone;
}

export function taskRunReviewPresentation(review: TaskRunReview): TaskRunReviewPresentation {
  const outcome = review.outcome ?? review.status;
  const label = outcome.replaceAll("_", " ");
  return {
    line: review.reason ? `Review: ${label} · ${review.reason}` : `Review: ${label}`,
    title: `Review round ${review.review_round}: ${label}`,
    tone: review.outcome ? REVIEW_OUTCOME_TONE[review.outcome] : "info",
  };
}
