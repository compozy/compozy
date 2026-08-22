import { describe, expect, it } from "vitest";

import { loopNodeLifecycleFixture } from "../../testing/loop-node-lifecycle-fixture";
import type { LoopRosterNode } from "../../types";
import { resolveNodeVerbTarget } from "../loop-node-verb-target";

function rosterNode(overrides: Partial<LoopRosterNode> = {}): LoopRosterNode {
  return {
    generation: 1,
    node_id: "revisor-perf",
    item_index: 0,
    state: "running",
    attempt: 1,
    attempts: [],
    ...overrides,
  } as LoopRosterNode;
}

const SELECTION = { nodeId: "revisor-perf", itemIndex: 0, generation: 1 };

// US-014: the verbs offered from the graph or the roster must be the verbs the
// daemon would actually accept. These cases guard the identity bridge that makes
// that possible — not the verb rules themselves, which `loopNodeVerbs` owns.
describe("resolveNodeVerbTarget", () => {
  it("Should prefer the durable lifecycle row over anything derived from the roster", () => {
    const quarantined = loopNodeLifecycleFixture({
      nodeId: "revisor-perf",
      // Same cell as SELECTION — node, round and item all. The durable row only
      // wins for the identity that was actually selected.
      generation: 1,
      itemIndex: 0,
      quarantined: true,
      outputStatus: "failed",
    });

    const target = resolveNodeVerbTarget(SELECTION, [quarantined], [rosterNode()]);

    // The roster does not model quarantine. Synthesizing over the durable row
    // would strip it and offer pause/cancel/kill on a node the daemon has held
    // back — verbs it would refuse.
    expect(target).toBe(quarantined);
    // The quarantine reaches the verb rules; which verbs it then permits is
    // `loopNodeVerbs`' own contract, asserted where that policy lives.
    expect(target?.quarantined).toBe(true);
  });

  it("Should build a faithful stand-in for a healthy node the projection skipped", () => {
    const target = resolveNodeVerbTarget(
      SELECTION,
      [],
      [
        rosterNode({
          state: "running",
          attempt: 2,
          session_id: "ses-5d871c99",
          attempts: [
            {
              attempt: 1,
              state: "failed",
              disposition: "retried",
              failure_class: "tool_error",
              started_at: "2026-08-19T18:40:10Z",
              ended_at: "2026-08-19T18:40:52Z",
            },
            {
              attempt: 2,
              state: "running",
              disposition: "",
              started_at: "2026-08-19T18:41:07Z",
            },
          ],
        }),
      ]
    );

    // A healthy node is skipped by the lifecycle projection precisely because it
    // has no control, no open wait and no scheduled retry — so asserting exactly
    // that is truthful, not invented.
    expect(target?.nodeId).toBe("revisor-perf");
    expect(target?.outputStatus).toBe("running");
    expect(target?.attempt).toBe(2);
    expect(target?.sessionId).toBe("ses-5d871c99");
    // Current-state truth: the node is running its second attempt, so it has no
    // failure class right now. Carrying attempt 1's `tool_error` forward would
    // make a recovered node read by the failure it already survived.
    expect(target?.failureClass).toBe("");
    expect(target?.paused).toBe(false);
    expect(target?.quarantined).toBe(false);
    expect(target?.waits).toEqual([]);
    // A stand-in asserts nothing the roster did not already say: every
    // control-shaped field is empty, which is why the node was skipped at all.
    expect(target?.state).toBeNull();
    expect(target?.parked).toBe(false);
  });

  it("Should carry the failure class of an attempt that is actually the current one", () => {
    const target = resolveNodeVerbTarget(
      SELECTION,
      [],
      [
        rosterNode({
          state: "failed",
          attempt: 3,
          attempts: [
            {
              attempt: 3,
              state: "failed",
              disposition: "exhausted",
              failure_class: "model_refusal",
              started_at: "2026-08-19T18:44:00Z",
              ended_at: "2026-08-19T18:44:22Z",
            },
          ],
        }),
      ]
    );

    expect(target?.outputStatus).toBe("failed");
    expect(target?.failureClass).toBe("model_refusal");
    expect(target?.disposition).toBe("exhausted");
  });

  it("Should offer nothing for a node the run never reached", () => {
    expect(resolveNodeVerbTarget(SELECTION, [], [rosterNode({ state: "not_taken" })])).toBeNull();
    expect(resolveNodeVerbTarget(SELECTION, [], [rosterNode({ state: "pending" })])).toBeNull();
  });

  it("Should offer nothing when neither view knows the node", () => {
    expect(resolveNodeVerbTarget(SELECTION, [], [])).toBeNull();
    expect(resolveNodeVerbTarget(null, [], [rosterNode()])).toBeNull();
  });

  it("Should match a fan-out worker by its own item and round, not just its node id", () => {
    const workers = [
      rosterNode({ item_index: 0, state: "succeeded" }),
      rosterNode({ item_index: 1, state: "running" }),
    ];

    const second = resolveNodeVerbTarget({ ...SELECTION, itemIndex: 1 }, [], workers);
    expect(second?.itemIndex).toBe(1);
    expect(second?.outputStatus).toBe("running");

    // A worker from another round is a different cell entirely.
    expect(resolveNodeVerbTarget({ ...SELECTION, generation: 2 }, [], workers)).toBeNull();
  });
});

// The identity a verb acts on. A fan-out worker shares its node id with every
// sibling, and the same step recurs each round — so `nodeId` alone addresses a
// set, not a cell. Acting on the wrong member of that set is silent and wrong.
describe("resolveNodeVerbTarget identity", () => {
  it("Should not hand a sibling's durable state to the selected fan-out item", () => {
    const pausedSibling = loopNodeLifecycleFixture({
      nodeId: "revisor-perf",
      generation: 1,
      itemIndex: 0,
      paused: true,
    });

    const target = resolveNodeVerbTarget(
      { nodeId: "revisor-perf", itemIndex: 1, generation: 1 },
      [pausedSibling],
      [rosterNode({ item_index: 1, state: "running" })]
    );

    // Item 1 is running. Borrowing item 0's pause would offer Resume on a node
    // that was never paused.
    expect(target).not.toBe(pausedSibling);
    expect(target?.itemIndex).toBe(1);
    expect(target?.paused).toBe(false);
  });

  it("Should not carry a previous round's durable state into this one", () => {
    const lastRound = loopNodeLifecycleFixture({
      nodeId: "revisor-perf",
      generation: 1,
      itemIndex: 0,
      quarantined: true,
    });

    const target = resolveNodeVerbTarget(
      { nodeId: "revisor-perf", itemIndex: 0, generation: 2 },
      [lastRound],
      [rosterNode({ generation: 2, state: "running" })]
    );

    expect(target).not.toBe(lastRound);
    expect(target?.generation).toBe(2);
    expect(target?.quarantined).toBe(false);
  });

  it("Should still prefer the durable row when the identity matches exactly", () => {
    const exact = loopNodeLifecycleFixture({
      nodeId: "revisor-perf",
      generation: 2,
      itemIndex: 3,
      quarantined: true,
    });

    const target = resolveNodeVerbTarget(
      { nodeId: "revisor-perf", itemIndex: 3, generation: 2 },
      [exact],
      [rosterNode({ generation: 2, item_index: 3, state: "failed" })]
    );

    expect(target).toBe(exact);
  });
});
