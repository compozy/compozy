import { describe, expect, it } from "vitest";

import { loopRunDetailByRunId } from "../../mocks/fixtures";
import type { LoopRunGeneration, LoopRunRecord } from "../../types";
import type { LoopGateVerdict } from "../loop-events";
import { buildRunProgress, latestGateVerdict } from "../loop-run-progress";
import { loopNodeLifecycleFixture } from "../../testing/loop-node-lifecycle-fixture";

function run(overrides: Partial<LoopRunRecord> = {}): LoopRunRecord {
  return { ...loopRunDetailByRunId.get("looprun_running")!.run, ...overrides };
}

function fanOutGeneration(statuses: string[], generation = 2): LoopRunGeneration {
  return {
    generation,
    parent_generation: generation > 1 ? generation - 1 : 0,
    origin: generation > 1 ? "gate_revise" : "initial",
    verdicts: [],
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
          parent_generation: 2,
          origin: "reattempt",
          verdicts: [],
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

// WT-003: a parked node's branches leave the counted denominator entirely. The
// bug this guards against is the natural one — treating a paused or quarantined
// lane as unfinished work, which tells the operator they owe progress that
// nothing is actually spending attempts on.
describe("buildRunProgress parked accounting", () => {
  function lifecycle(nodeId: string, state: "paused" | "quarantined" | "waiting") {
    return loopNodeLifecycleFixture({
      nodeId,
      label: nodeId,
      state,
      parked: true,
      paused: state === "paused",
      quarantined: state === "quarantined",
      outputStatus: "",
    });
  }

  it("Should exclude a paused node's branches from the count and name the exclusion", () => {
    const model = buildRunProgress(
      run({ status: "running", generation: 2 }),
      [fanOutGeneration(["succeeded", "succeeded", "running"])],
      null,
      [lifecycle("fix_batches", "paused")]
    );
    expect(model.segments).toEqual(["parked", "parked", "parked"]);
    expect(model.parkedCount).toBe(3);
    // Every branch is parked, so there is no counted work — saying "0 of 0
    // clean" would read as failure rather than as suspension.
    expect(model.leftMeta).toBe("Nothing counting here — all 3 groups paused");
    // The exclusion is carried in text, not colour alone.
    expect(model.ariaLabel).toContain("paused");
  });

  it("Should still count the branches that are not parked", () => {
    const model = buildRunProgress(
      run({ status: "running", generation: 2 }),
      [
        {
          generation: 2,
          parent_generation: 1,
          origin: "gate_revise",
          verdicts: [],
          outputs: [
            { node_id: "fix_batches", status: "succeeded", generation: 2, item_index: 1 },
            { node_id: "fix_batches", status: "succeeded", generation: 2, item_index: 2 },
            { node_id: "fix_batches", status: "running", generation: 2, item_index: 3 },
          ],
        },
      ],
      null,
      // The parked node is a different one, so this fan-out keeps counting.
      [lifecycle("collect_fixes", "quarantined")]
    );
    expect(model.parkedCount).toBe(0);
    expect(model.leftMeta).toContain("2 of 3 groups clean");
  });

  it("Should say quarantined when quarantine is the reason the branches are parked", () => {
    const model = buildRunProgress(
      run({ status: "running", generation: 2 }),
      [fanOutGeneration(["succeeded", "failed"])],
      null,
      [lifecycle("fix_batches", "quarantined")]
    );
    expect(model.leftMeta).toContain("quarantined");
    // A quarantined branch must never read as a failure the run still owes.
    expect(model.segments).not.toContain("failed");
    expect(model.leftMeta).not.toContain("failed");
  });

  it("Should name the fan-out node's parked state when another node is parked differently", () => {
    const model = buildRunProgress(
      run({ status: "running", generation: 2 }),
      [fanOutGeneration(["succeeded", "running"])],
      null,
      [lifecycle("fix_batches", "paused"), lifecycle("other_node", "quarantined")]
    );
    expect(model.leftMeta).toBe("Nothing counting here — all 2 groups paused");
  });

  it("Should leave normal accounting untouched when the parked node owns no branches", () => {
    const withoutParked = buildRunProgress(
      run({ status: "running", generation: 2 }),
      [fanOutGeneration(["succeeded", "succeeded", "running"])],
      null
    );
    // A node parked elsewhere in the graph must not touch this bar at all.
    const model = buildRunProgress(
      run({ status: "running", generation: 2 }),
      [fanOutGeneration(["succeeded", "succeeded", "running"])],
      null,
      [lifecycle("other_node", "paused")]
    );
    expect(model.parkedCount).toBe(0);
    expect(model.segments).toEqual(["clean", "clean", "active"]);
    expect(model.leftMeta).toBe(withoutParked.leftMeta);
  });
});

describe("latestGateVerdict", () => {
  it("Should pick the verdict from the highest generation", () => {
    const verdicts: Record<string, LoopGateVerdict> = {
      check_all: {
        nodeId: "check_all",
        gateId: "check_all",
        generation: 2,
        verdict: "revise",
        criteria: [],
        blockingIssues: [{ id: "issue_022" }],
      },
      final_gate: {
        nodeId: "final_gate",
        gateId: "final_gate",
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
