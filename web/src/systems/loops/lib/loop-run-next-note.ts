import type { LoopRunGeneration } from "../types";
import { type LoopGraph, findWatchNode, topoOrder } from "./loop-graph";
import { humanizeLoopNodeId } from "./loop-run-story-rows";

/**
 * The "What happens next" quiet note (redesign spec §5.3): the remaining
 * downstream nodes of the current generation phrased as one static sentence from
 * the pinned graph — no invention, domain flavor only from node ids.
 */

/** Joins humanized labels as an ordered sequence: `a`, `a, then b`, `a, b, then c`. */
function joinLabels(labels: string[]): string {
  if (labels.length <= 1) return labels[0] ?? "";
  return `${labels.slice(0, -1).join(", ")}, then ${labels[labels.length - 1]}`;
}

/** Latest-generation output statuses that still count as "ahead" work. */
const PENDING_OUTPUT_STATUSES = new Set(["pending", "ready", "enqueued"]);

/**
 * The "What happens next" quiet note (§5.3): remaining downstream nodes of the
 * current generation in topological order, humanized into one sentence, ending
 * with the watch/stop clause. Static per definition — no invention.
 */
export function buildNextNote(
  graph: LoopGraph | null,
  generations: readonly LoopRunGeneration[] | undefined
): string | null {
  if (!graph || graph.nodes.length === 0) return null;
  const watchNode = findWatchNode(graph);
  const hasGate = graph.nodes.some(node => node.isGate);
  const latest = (generations ?? []).reduce((max, gen) => Math.max(max, gen.generation), 0);
  const latestOutputs = new Map<string, string>();
  for (const generation of generations ?? []) {
    if (generation.generation !== latest) continue;
    for (const output of generation.outputs) {
      latestOutputs.set(output.node_id, output.status);
    }
  }
  const remaining = topoOrder(graph).filter(id => {
    if (watchNode && id === watchNode.id) return false;
    const status = latestOutputs.get(id);
    return status === undefined || PENDING_OUTPUT_STATUSES.has(status);
  });
  const ahead =
    remaining.length > 0
      ? `Still ahead this round: ${joinLabels(remaining.map(humanizeLoopNodeId))}.`
      : "";
  const clause = watchNode
    ? hasGate
      ? "After that, the run goes back to watching. If the next check comes back clean, it finishes as Done; if new work arrives, a new round starts automatically."
      : "After that, the run goes back to watching and finishes when there's nothing left to do."
    : hasGate
      ? "When every step is done, a final check decides: clean finishes the run as Done; anything left starts another round."
      : "When every step is done, the run finishes.";
  return [ahead, clause].filter(Boolean).join(" ");
}
