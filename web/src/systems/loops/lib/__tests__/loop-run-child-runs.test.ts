import { describe, expect, it } from "vitest";

import { makeRosterNode as node, storyAt } from "../../components/stories/loop-run-read-builders";
import { STORY_NOW, reviewAndFixRun } from "../../components/stories/loop-run-page-fixture-world";
import {
  buildChildRunSummary,
  childRunCurrentStep,
  childRunsToggleLabel,
  stepChildRuns,
} from "../loop-run-child-runs";

const PROGRESS = { round: 1, steps_done: 1, steps_total: 4 };

function childRun(overrides: Parameters<typeof reviewAndFixRun>[0] = {}) {
  return reviewAndFixRun({
    id: "r-child",
    loop_name: "fix-one-batch",
    status: "running",
    generation: 1,
    created_at: storyAt(6),
    started_at: storyAt(6),
    ...overrides,
  });
}

describe("stepChildRuns", () => {
  it("Should keep only the slots that recorded a child run, in branch order", () => {
    expect(
      stepChildRuns([
        { key: "fix:0", childRunId: "r-a", slotLabel: "item 0", itemIndex: 0 },
        { key: "fix:1", childRunId: null, slotLabel: "item 1", itemIndex: 1 },
        { key: "fix:2", childRunId: "  ", slotLabel: "item 2", itemIndex: 2 },
        { key: "fix:3", childRunId: "r-b", slotLabel: "item 3", itemIndex: 3 },
      ])
    ).toEqual([
      { key: "fix:0", runId: "r-a", slotLabel: "item 0" },
      { key: "fix:3", runId: "r-b", slotLabel: "item 3" },
    ]);
  });

  it("Should tell branches of one fanned node apart by their item slot", () => {
    expect(
      stepChildRuns([
        { key: "fix_batch:0", childRunId: "r-a", slotLabel: "fix_batch", itemIndex: 0 },
        { key: "fix_batch:1", childRunId: "r-b", slotLabel: "fix_batch", itemIndex: 1 },
        { key: "review:0", childRunId: "r-c", slotLabel: "review", itemIndex: 0 },
      ]).map(child => child.slotLabel)
    ).toEqual(["fix_batch · item 0", "fix_batch · item 1", "review"]);
  });

  it("Should name how many child runs the disclosure holds", () => {
    expect(childRunsToggleLabel(1)).toBe("1 child run");
    expect(childRunsToggleLabel(3)).toBe("3 child runs");
  });
});

describe("childRunCurrentStep", () => {
  it("Should lead with the step holding for a person over one merely running", () => {
    const step = childRunCurrentStep("running", PROGRESS, [
      node("plan_fix", "succeeded", { generation: 1 }),
      node("run_tests", "running", { generation: 1 }),
      node("finalize_round", "control_pending", { generation: 1 }),
      node("apply_fix", "pending", { generation: 1 }),
    ]);
    expect(step?.nodeId).toBe("finalize_round");
    expect(step?.chip.state).toBe("control_pending");
    expect(step?.alsoActive).toBe(1);
  });

  it("Should read only the child's current round and count a fanned step once", () => {
    const step = childRunCurrentStep("running", { round: 2, steps_done: 0, steps_total: 3 }, [
      node("review", "control_pending", { generation: 1 }),
      node("fix", "running", { generation: 2, item_index: 0 }),
      node("fix", "running", { generation: 2, item_index: 1 }),
    ]);
    expect(step).toMatchObject({ nodeId: "fix", alsoActive: 0 });
  });

  it("Should place a settled child on no step at all", () => {
    expect(
      childRunCurrentStep("done", PROGRESS, [node("finalize_round", "running", { generation: 1 })])
    ).toBeNull();
  });
});

describe("buildChildRunSummary", () => {
  it("Should take the step count from the briefing and tick a running child's clock", () => {
    // The detail route serves a zeroed `progress`; only the briefing's counts count.
    const summary = buildChildRunSummary(
      childRun({ progress: { round: 0, steps_done: 0, steps_total: 0 } }),
      PROGRESS,
      [node("run_tests", "running", { generation: 1 })],
      STORY_NOW
    );
    expect(summary).toMatchObject({
      runId: "r-child",
      loopName: "fix-one-batch",
      status: "running",
      progressLabel: "1 of 4 steps",
      elapsedSeconds: 360,
    });
  });

  it("Should say nothing about steps before the child has any", () => {
    const summary = buildChildRunSummary(
      childRun({ status: "queued" }),
      { round: 0, steps_done: 0, steps_total: 0 },
      [],
      STORY_NOW
    );
    expect(summary.progressLabel).toBe("");
    expect(summary.currentStep).toBeNull();
  });
});
