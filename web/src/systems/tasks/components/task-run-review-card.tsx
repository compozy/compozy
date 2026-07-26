import { StatusCard, Time } from "@agh/ui";

import { taskRunReviewPresentation } from "../lib/task-run-presentation";
import type { TaskRunReview } from "../types";

/**
 * Run-detail review card: one StatusCard per review round — outcome
 * tone, reason, and next-round guidance in plain language.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.9
 */
export function TaskRunReviewCard({ review }: { review: TaskRunReview }) {
  const presentation = taskRunReviewPresentation(review);
  const body = review.reason ?? review.review_text;

  return (
    <StatusCard
      className="border border-line-soft"
      data-testid={`tasks-run-review-${review.review_id}`}
      tone={presentation.tone}
    >
      <StatusCard.Header label={presentation.title}>
        <span className="ml-auto shrink-0 text-eyebrow tabular-nums text-subtle">
          <Time iso={review.reviewed_at} mode="relative" />
        </span>
      </StatusCard.Header>
      {body ? <StatusCard.Body>{body}</StatusCard.Body> : null}
      {review.next_round_guidance ? (
        <StatusCard.Body className="text-subtle">
          Next round: {review.next_round_guidance}
        </StatusCard.Body>
      ) : null}
      {review.reviewer_agent_name ? (
        <StatusCard.Footer>
          <span className="text-form-hint text-subtle">
            Reviewed by {review.reviewer_agent_name}
          </span>
        </StatusCard.Footer>
      ) : null}
    </StatusCard>
  );
}
