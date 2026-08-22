import type { LoopDefinitionGraph, LoopFanoutRollup } from "../../types";
import { reviewAndFixDefinition } from "./loop-run-page-fixture-world";
import { registerWideFanOutScenario } from "./loop-run-register-fixtures";
import { makeRosterNode as node } from "./loop-run-read-builders";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";

/**
 * A run whose graph is long as well as wide.
 *
 * VC-21 and VC-22 both used to stage `registerWideFanOutScenario`, so the two
 * rows captured the same picture and VC-22 — "a wide graph stays navigable
 * rather than clipping" — proved nothing VC-21 had not already shown. Width
 * alone also cannot demonstrate navigation: the fan-out band draws its workers
 * as lanes inside one card, so it never leaves the viewport. Depth does, which
 * is the same reason `E2E-016` grew a chain case beside its rollup case.
 *
 * Lives in its own file because `loop-run-register-fixtures.ts` is close to the
 * 500-line cap and a graph this size is a distinct subject from the register
 * scenarios.
 */

/** The verification chain that carries the run past one viewport of graph. */
const VERIFY_STEPS = [
  "verify_contracts",
  "verify_migrations",
  "verify_fixtures",
  "verify_docs",
  "verify_release_notes",
  "verify_changelog",
] as const;

/**
 * The review-and-fix graph with a six-step verification tail appended.
 *
 * Built by extending the canonical definition rather than minting a second
 * loop: `DESIGN-NOTES.md` fixes the data story at two loops and forbids a third.
 */
function deepGraph(): LoopDefinitionGraph {
  const graph = reviewAndFixDefinition.graph as unknown as {
    nodes: { id: string; class: string; kind: string }[];
    edges: { from: string; to: string }[];
  };
  const tail = VERIFY_STEPS.map(id => ({ id, class: "action", kind: "run-agent" }));
  const chain = ["collect_fixes", ...VERIFY_STEPS, "finalize_round"];
  const tailEdges = chain
    .slice(0, -1)
    .map((from, index) => ({ from, to: chain[index + 1] as string }));
  return {
    ...graph,
    nodes: [...graph.nodes, ...tail],
    // The original `collect_fixes -> finalize_round` edge is replaced by the
    // chain, so the tail is a path rather than a second branch out of collect.
    edges: [
      ...graph.edges.filter(
        edge => !(edge.from === "collect_fixes" && edge.to === "finalize_round")
      ),
      ...tailEdges,
    ],
  } as unknown as LoopDefinitionGraph;
}

const DEEP_FAN_OUT: LoopFanoutRollup = {
  generation: 2,
  node_id: "fix_batches",
  done: 7,
  total: 10,
  failed: 1,
};

/** VC-22: fourteen steps and a ten-way fan-out — the graph has to be navigable. */
export function registerDeepAndWideScenario(): LoopRunStoryScenario {
  const wide = registerWideFanOutScenario();
  return {
    ...wide,
    definition: { ...reviewAndFixDefinition, graph: deepGraph() },
    rosterNodes: [
      node("review", "succeeded"),
      node("has_issues", "succeeded"),
      node("write_artifacts", "succeeded"),
      ...Array.from({ length: 10 }, (_unused, index) =>
        node("fix_batch", index < 7 ? "succeeded" : index === 7 ? "failed" : "running", {
          item_index: index,
        })
      ),
      node("collect_fixes", "succeeded"),
      // The tail is reachable and not reached yet — the far end of the lane.
      ...VERIFY_STEPS.map((id, index) => node(id, index === 0 ? "running" : "pending")),
      node("finalize_round", "pending"),
    ],
    rosterRollups: [DEEP_FAN_OUT],
  };
}
