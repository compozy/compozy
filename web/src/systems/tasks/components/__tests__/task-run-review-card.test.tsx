import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { buildTaskRunReviewFixture } from "../../mocks/fixtures";
import { TaskRunReviewCard } from "../task-run-review-card";

describe("TaskRunReviewCard", () => {
  it("Should present pending review status when no outcome exists", () => {
    render(
      <TaskRunReviewCard
        review={buildTaskRunReviewFixture({ outcome: undefined, status: "in_review" })}
      />
    );

    expect(screen.getByText("Review round 1: in review")).toBeInTheDocument();
  });

  it("Should present recorded outcome, reason, and next-round guidance", () => {
    render(
      <TaskRunReviewCard
        review={buildTaskRunReviewFixture({
          next_round_guidance: "Attach reconciliation evidence.",
          outcome: "rejected",
          reason: "Evidence is incomplete.",
          review_round: 2,
          status: "recorded",
        })}
      />
    );

    expect(screen.getByText("Review round 2: rejected")).toBeInTheDocument();
    expect(screen.getByText("Evidence is incomplete.")).toBeInTheDocument();
    expect(screen.getByText("Next round: Attach reconciliation evidence.")).toBeInTheDocument();
  });
});
