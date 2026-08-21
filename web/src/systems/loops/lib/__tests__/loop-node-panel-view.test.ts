import { describe, expect, it } from "vitest";

import type { LoopRosterNode } from "../../types";
import type { LoopGraph } from "../loop-graph";
import { buildNodePanel } from "../loop-node-panel-view";

const GRAPH: LoopGraph = {
  nodes: [
    {
      id: "revisor-estilo",
      nodeClass: "action",
      kind: "run-agent",
      isGate: false,
      eventsCount: 0,
      routes: [],
      hasAskExpect: false,
      retryMaxAttempts: 3,
    },
    {
      id: "rota-manual",
      nodeClass: "action",
      kind: "run-agent",
      isGate: false,
      eventsCount: 0,
      routes: [],
      hasAskExpect: false,
    },
  ],
  edges: [],
};

function node(overrides: Partial<LoopRosterNode> = {}): LoopRosterNode {
  return {
    generation: 1,
    node_id: "revisor-estilo",
    item_index: 0,
    state: "succeeded",
    attempt: 2,
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
        state: "succeeded",
        disposition: "settled",
        started_at: "2026-08-19T18:41:07Z",
        ended_at: "2026-08-19T18:43:38Z",
      },
    ],
    session_id: "ses-5d871c99",
    cell_task_id: "loop.looprun-8f3ab2c1d4e5f607.g1.node.revisor-estilo.0",
    started_at: "2026-08-19T18:41:07Z",
    ended_at: "2026-08-19T18:43:38Z",
    ...overrides,
  } as LoopRosterNode;
}

describe("buildNodePanel", () => {
  // UT-047
  it("Should keep the session link on a node whose run has already finished", () => {
    const panel = buildNodePanel({ node: node(), graph: GRAPH });

    // Links are not live-only: diagnosing a run is something people do after it
    // ends, which is exactly when a live-only link is useless (US-015.AC-1).
    expect(panel.links.map(link => link.kind)).toEqual(["session", "record"]);
    expect(panel.links[0].id).toBe("ses-5d871c99");
    expect(panel.degradedLinks).toEqual([]);
  });

  it("Should degrade a pruned session to a sentence instead of a dead link", () => {
    const panel = buildNodePanel({
      node: node(),
      graph: GRAPH,
      prunedSessionIds: new Set(["ses-5d871c99"]),
    });

    expect(panel.links.map(link => link.kind)).toEqual(["record"]);
    expect(panel.degradedLinks).toEqual([{ kind: "session", note: "Session no longer available" }]);
  });

  it("Should offer no links at all for a node the run never took", () => {
    const panel = buildNodePanel({
      node: node({
        node_id: "rota-manual",
        state: "not_taken",
        session_id: undefined,
        cell_task_id: undefined,
        attempts: [],
        attempt: 0,
      }),
      graph: GRAPH,
    });

    // Nothing ran here, so nothing is linkable and no timing is invented.
    expect(panel.neverMaterialized).toBe(true);
    expect(panel.links).toEqual([]);
    expect(panel.degradedLinks).toEqual([]);
    expect(panel.startedAt).toBeNull();
    expect(panel.endedAt).toBeNull();
    expect(panel.attempts).toEqual([]);
  });

  it("Should read a recovered node by its current state and keep its attempt history", () => {
    const panel = buildNodePanel({ node: node(), graph: GRAPH });

    // Current-state truth: recovery hides nothing, it just wins the headline.
    expect(panel.chip.label).toBe("succeeded");
    expect(panel.attempts.map(attempt => attempt.attempt)).toEqual([2, 1]);
    expect(panel.attempts[1].failureLabel).toBe("tool error");
    expect(panel.attempts[1].failureLabel).not.toContain("_");
  });

  it("Should only name an attempt ceiling the definition actually declares", () => {
    const withCeiling = buildNodePanel({ node: node(), graph: GRAPH });
    expect(withCeiling.attemptLabel).toBe("attempt 2 of 3");

    // No authored ceiling means no denominator: the run payload carries no
    // attempt limit, so "of 3" would be an invented policy.
    const withoutCeiling = buildNodePanel({ node: node(), graph: null });
    expect(withoutCeiling.attemptLabel).toBe("attempt 2");
  });

  it("Should say the strategy cancelled it without printing the reason code", () => {
    // The daemon puts `canceled_by_strategy` in `cause` so an agent can match on
    // it. The label already says that in words, so rendering both would print
    // the wire value underneath the sentence — and nobody wrote that sentence.
    const panel = buildNodePanel({
      node: node({
        state: "canceled",
        cancellation: { disposition: "strategy", cause: "canceled_by_strategy" },
      }),
      graph: GRAPH,
    });

    expect(panel.cancellation?.label).toBe("Canceled by the loop's strategy");
    expect(panel.cancellation?.cause).toBeNull();
    // Nobody pressed anything, so there is nobody to name.
    expect(panel.cancellation?.actorLabel).toBeNull();
  });

  it("Should keep an operator's own words even when they read like a code", () => {
    // Suppressing anything with an underscore would swallow a real sentence.
    const panel = buildNodePanel({
      node: node({
        state: "canceled",
        cancellation: {
          disposition: "operator",
          actor_kind: "operator",
          actor_ref: "pedro",
          cause: "stopped_for_the_demo",
        },
      }),
      graph: GRAPH,
    });

    expect(panel.cancellation?.label).toBe("Canceled by an operator");
    expect(panel.cancellation?.actorLabel).toBe("pedro");
    expect(panel.cancellation?.cause).toBe("stopped_for_the_demo");
  });

  it("Should tell a strategy cancellation apart from an operator one", () => {
    const byStrategy = buildNodePanel({
      node: node({
        state: "canceled",
        cancellation: { disposition: "strategy", cause: "the join settled without it" },
      }),
      graph: GRAPH,
    });
    const byOperator = buildNodePanel({
      node: node({
        state: "canceled",
        cancellation: { disposition: "operator", actor_kind: "user", actor_ref: "pedro" },
      }),
      graph: GRAPH,
    });

    expect(byStrategy.cancellation?.label).toBe("Canceled by the loop's strategy");
    expect(byStrategy.cancellation?.actorLabel).toBeNull();
    expect(byOperator.cancellation?.label).toBe("Canceled by an operator");
    expect(byOperator.cancellation?.actorLabel).toBe("pedro");
    // Cancellation is calm on both paths; the cause travels in words.
    expect(byStrategy.chip.tone).toBe("neutral");
    expect(byOperator.chip.tone).toBe("neutral");
  });

  it("Should link a child run when the node spawned one", () => {
    const panel = buildNodePanel({
      node: node({ child_loop_run_id: "looprun-child01" }),
      graph: GRAPH,
    });
    expect(panel.links.map(link => link.kind)).toContain("child-run");
  });
});
