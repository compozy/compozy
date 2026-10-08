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
    const step = childRunCurrentStep(
      "running",
      PROGRESS,
      [
        node("plan_fix", "succeeded", { generation: 1 }),
        node("run_tests", "running", { generation: 1 }),
        node("finalize_round", "control_pending", { generation: 1, started_at: storyAt(47) }),
        node("apply_fix", "pending", { generation: 1 }),
      ],
      STORY_NOW
    );
    expect(step).toMatchObject({
      nodeId: "finalize_round",
      name: "finalize round",
      alsoActive: 1,
      detail: " — waiting for your decision · 1 more active",
    });
    expect(step?.chip.state).toBe("control_pending");
  });

  it("Should keep counting time on a parked step from the step's own start", () => {
    // The run clock freezes at its last progress; a step stuck for 47 minutes
    // has to read 47 minutes, or the stuck child looks like the healthy ones.
    const step = childRunCurrentStep(
      "needs-approval",
      PROGRESS,
      [node("finalize_round", "control_pending", { generation: 1, started_at: storyAt(47) })],
      STORY_NOW
    );
    expect(step?.onStepSeconds).toBe(47 * 60);
  });

  it("Should time a step parked in a durable wait from its wait cell", () => {
    // The roster does not time a wait step; its wait cell records when it began.
    const step = childRunCurrentStep(
      "running",
      PROGRESS,
      [node("hold_for_release", "waiting", { generation: 1, started_at: null })],
      STORY_NOW,
      [{ node_id: "hold_for_release", item_index: 0, created_at: storyAt(37) }]
    );
    expect(step?.onStepSeconds).toBe(37 * 60);
  });

  it("Should read only the child's current round and count a fanned step once", () => {
    const step = childRunCurrentStep(
      "running",
      { round: 2, steps_done: 0, steps_total: 3 },
      [
        node("review", "control_pending", { generation: 1 }),
        node("fix", "running", { generation: 2, item_index: 0 }),
        node("fix", "running", { generation: 2, item_index: 1 }),
      ],
      STORY_NOW
    );
    expect(step).toMatchObject({ nodeId: "fix", alsoActive: 0 });
  });

  it("Should place a settled child on no step at all", () => {
    expect(
      childRunCurrentStep(
        "done",
        PROGRESS,
        [node("finalize_round", "running", { generation: 1 })],
        STORY_NOW
      )
    ).toBeNull();
  });
});

describe("buildChildRunSummary", () => {
  it("Should take the step count from the briefing and tick a running child's clock", () => {
    // The detail route serves a zeroed `progress`; only the briefing's counts count.
    const summary = buildChildRunSummary(
      childRun({ progress: { round: 0, steps_done: 0, steps_total: 0 } }),
      PROGRESS,
      [node("run_tests", "running", { generation: 1, started_at: storyAt(2) })],
      STORY_NOW
    );
    expect(summary).toMatchObject({
      runId: "r-child",
      loopName: "fix-one-batch",
      status: "running",
      progressLabel: "1 of 4 steps",
      elapsedSeconds: 360,
      metaLabel: "1 of 4 steps · 6m 00s",
      onStepLabel: "2m 00s on this step",
    });
  });

  it("Should carry the runs the child started in its current round, a level down", () => {
    const summary = buildChildRunSummary(
      childRun(),
      PROGRESS,
      [
        node("deep_review", "awaiting_child", { generation: 1, child_loop_run_id: "r-grand" }),
        node("old_review", "succeeded", { generation: 0, child_loop_run_id: "r-stale" }),
      ],
      STORY_NOW
    );
    expect(summary.childRuns).toEqual([
      { key: "deep_review:0", runId: "r-grand", slotLabel: "deep_review" },
    ]);
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
    expect(summary.onStepLabel).toBe("");
    expect(summary.childRuns).toEqual([]);
  });
});
