import { describe, expect, it } from "vitest";

import { loopRunDetailByRunId } from "../../mocks/fixtures";
import type { LoopRunGeneration, LoopRunRecord } from "../../types";
import type { LoopGateVerdict } from "../loop-events";
import { buildRunProgress, latestGateVerdict } from "../loop-run-progress";

function run(overrides: Partial<LoopRunRecord> = {}): LoopRunRecord {
  return { ...loopRunDetailByRunId.get("looprun_running")!.run, ...overrides };
}

function fanOutGeneration(statuses: string[], generation = 2): LoopRunGeneration {
  return {
    generation,
    outputs: statuses.map((status, index) => ({
      node_id: "fix_batches",
      status,
      generation,
      item_index: index + 1,
    })),
  };
}

describe("buildRunProgress", () => {
  it("Should render one equal segment per branch with clean/active states", () => {
    const model = buildRunProgress(
      run({ status: "running", generation: 2, reattempt_strategy: "failed_only" }),
      [fanOutGeneration(["succeeded", "reused", "running"])],
      2
    );
    expect(model.segments).toEqual(["clean", "clean", "active"]);
    expect(model.leftMeta).toBe("2 of 3 groups clean — the failed group is being redone");
    expect(model.rightMeta).toBe("2 open points from the last check");
  });

  it("Should park the failed branch as neutral redo while live and danger once the run failed", () => {
    const live = buildRunProgress(
      run({ status: "needs-approval" }),
      [fanOutGeneration(["succeeded", "succeeded", "failed"])],
      2
    );
    expect(live.segments).toEqual(["clean", "clean", "redo"]);
    expect(live.leftMeta).toBe("2 of 3 groups clean — group work paused with the run");
    const failed = buildRunProgress(
      run({ status: "failed" }),
      [fanOutGeneration(["succeeded", "succeeded", "failed"])],
      2
    );
    expect(failed.segments).toEqual(["clean", "clean", "failed"]);
    expect(failed.leftMeta).toBe("2 of 3 groups clean when the run failed");
    expect(failed.rightMeta).toBe("2 open points remained");
  });

  it("Should hide the bar without fan-out and summarize the round alone", () => {
    const model = buildRunProgress(
      run({ status: "watching", generation: 3 }),
      [
        {
          generation: 3,
          outputs: [{ node_id: "remediate", status: "succeeded", generation: 3 }],
        },
      ],
      null
    );
    expect(model.segments).toBeNull();
    expect(model.leftMeta).toBe("Round 3");
    expect(model.rightMeta).toBe("waiting for the next check");
  });
});

describe("latestGateVerdict", () => {
  it("Should pick the verdict from the highest generation", () => {
    const verdicts: Record<string, LoopGateVerdict> = {
      check_all: {
        nodeId: "check_all",
        generation: 2,
        verdict: "revise",
        criteria: [],
        blockingIssues: [{ id: "issue_022" }],
      },
      final_gate: {
        nodeId: "final_gate",
        generation: 3,
        verdict: "pass",
        criteria: [],
        blockingIssues: [],
      },
    };
    expect(latestGateVerdict(verdicts)?.nodeId).toBe("final_gate");
    expect(latestGateVerdict({})).toBeNull();
  });
});
