import { describe, expect, it } from "vitest";

import { loopNodeLifecycleFixture } from "../../testing/loop-node-lifecycle-fixture";
import type { LoopNodeLifecycle } from "../loop-node-lifecycle";
import { loopNodeVerbs, loopNodeWaitResumeItemIndex } from "../loop-node-controls";

// The verb policy is a pure function that lives here, so its contract is
// asserted here. It was previously exercised from the run-page component suite,
// which made a lib-owned rule look like a rendering concern and put its cases
// out of reach of anyone reading `loop-node-controls.ts`. `loop-node-verb-target`
// deliberately defers to this file for exactly these assertions.
//
// WT-004: node verbs are a pure function of daemon-declared state. Testing the
// gate directly (rather than only through a rendered menu) is what makes "no
// verb for a state the payload doesn't declare" checkable for every state.
describe("loopNodeVerbs", () => {
  const node = (overrides: Partial<LoopNodeLifecycle> = {}): LoopNodeLifecycle =>
    loopNodeLifecycleFixture(overrides);

  const wait = (claimState: string, itemIndex = 0) => ({
    nodeId: "task_03",
    generation: 2,
    itemIndex,
    kind: "event",
    claimState,
    escalationCursor: 0,
    admissionFailures: 0,
    ageSeconds: 120,
    createdAt: "2026-08-03T14:00:00Z",
    expect: undefined,
  });

  it("Should offer pause/cancel on a running node and never resume or requeue", () => {
    expect(loopNodeVerbs(node(), "running")).toEqual(["pause", "cancel"]);
  });

  it("Should offer the three resume modes on a paused node and no requeue", () => {
    expect(loopNodeVerbs(node({ paused: true, state: "paused" }), "running")).toEqual([
      "resume",
      "resume-reset-attempts",
      "resume-immediate",
      "cancel",
    ]);
  });

  it("Should offer requeue only on a quarantined node, and never resume", () => {
    const verbs = loopNodeVerbs(node({ quarantined: true, state: "quarantined" }), "running");
    expect(verbs).toEqual(["open-quarantine", "requeue", "cancel"]);
    expect(verbs).not.toContain("resume");
  });

  it("Should offer a payload resume only while a wait is genuinely open", () => {
    expect(loopNodeVerbs(node({ state: "waiting", waits: [wait("waiting")] }), "running")).toEqual([
      "resume-wait",
      "cancel",
    ]);
    // A resumed cell no longer holds the node, so the wait verb disappears.
    expect(loopNodeVerbs(node({ waits: [wait("resumed")] }), "running")).toEqual([
      "pause",
      "cancel",
    ]);
  });

  it("Should target the open wait instead of an earlier historical cell", () => {
    const lifecycle = node({
      state: "waiting",
      waits: [wait("resumed", 2), wait("waiting", 7)],
    });

    expect(loopNodeWaitResumeItemIndex(lifecycle)).toBe(7);
  });

  it("Should offer nothing once cancellation commits", () => {
    expect(loopNodeVerbs(node({ cancelState: "canceled", state: "canceled" }), "running")).toEqual(
      []
    );
  });

  it("Should offer no verb at all once the run is terminal", () => {
    for (const status of ["done", "failed", "canceled", "exhausted"]) {
      expect(loopNodeVerbs(node({ paused: true, state: "paused" }), status)).toEqual([]);
    }
  });
});
