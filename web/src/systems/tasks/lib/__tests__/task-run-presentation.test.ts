import { describe, expect, it } from "vitest";

import {
  buildTaskRunRecordFixture,
  buildTaskRunReviewFixture,
  buildTaskTimelineItemFixture,
} from "../../mocks/fixtures";
import {
  latestTaskRun,
  taskRunLineage,
  taskRunReviewPresentation,
  taskRunTimelineItems,
} from "../task-run-presentation";

describe("task run presentation", () => {
  it("Should select the highest attempt matching the requested status", () => {
    const runs = [
      buildTaskRunRecordFixture({ id: "run_1", attempt: 1, status: "completed" }),
      buildTaskRunRecordFixture({ id: "run_3", attempt: 3, status: "failed" }),
      buildTaskRunRecordFixture({ id: "run_2", attempt: 2, status: "completed" }),
    ];

    expect(latestTaskRun(runs, "completed")?.id).toBe("run_2");
  });

  it("Should resolve both sides of retry lineage from run identity", () => {
    const first = buildTaskRunRecordFixture({ id: "run_1", attempt: 1 });
    const second = buildTaskRunRecordFixture({
      id: "run_2",
      attempt: 2,
      previous_run_id: "run_1",
    });

    expect(taskRunLineage(first, [first, second])).toEqual({ previous: null, next: second });
    expect(taskRunLineage(second, [first, second])).toEqual({ previous: first, next: null });
  });

  it("Should filter activity by the authoritative run id", () => {
    const current = buildTaskTimelineItemFixture({
      event_id: "evt_current",
      run: { id: "run_current" } as never,
    });
    const sibling = buildTaskTimelineItemFixture({
      event_id: "evt_sibling",
      run: { id: "run_sibling" } as never,
    });
    const taskOnly = buildTaskTimelineItemFixture({ event_id: "evt_task", run: null });

    expect(taskRunTimelineItems([current, sibling, taskOnly], "run_current")).toEqual([current]);
  });

  it("Should prefer a recorded review outcome and fall back to pending status", () => {
    const recorded = taskRunReviewPresentation(
      buildTaskRunReviewFixture({
        outcome: "rejected",
        reason: "Missing evidence",
        review_round: 2,
        status: "recorded",
      })
    );
    const pending = taskRunReviewPresentation(
      buildTaskRunReviewFixture({ outcome: undefined, status: "in_review" })
    );

    expect(recorded).toEqual({
      line: "Review: rejected · Missing evidence",
      title: "Review round 2: rejected",
      tone: "warning",
    });
    expect(pending).toMatchObject({ title: "Review round 1: in review", tone: "info" });
  });
});
