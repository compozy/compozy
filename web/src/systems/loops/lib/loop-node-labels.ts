import { type LoopGraphNode, nodeClassLabel } from "./loop-graph";

/**
 * Plain words for a node, owned by nobody in particular.
 *
 * The panel, the DAG, the roster, the lifecycle rows, the inventory and the verb
 * bridge all name the same nodes. Letting one concrete projection own the naming
 * made every other projection depend on that view's refactors, so the labels sit
 * here instead — pure, view-free, and safe for any consumer to import.
 */

/** `fix_batches` → "fix batches" — the generic humanization of a node id. */
export function humanizeLoopNodeId(nodeId: string): string {
  return nodeId.replace(/[_-]+/g, " ").trim();
}

/** Plain words for the authored kind. The DSL spelling stays in the definition. */
const KIND_LABELS: Record<string, string> = {
  "run-agent": "agent",
  "fan-out": "fan-out",
  gate: "gate",
  collect: "collect",
  route: "route",
  ask: "ask",
  wait: "wait",
  goal: "goal",
  transform: "transform",
  "run-loop": "child run",
  "sub-loop": "sub-loop",
  "watch-source": "watch",
  "watch-events": "watch",
  "file-import": "file import",
  input: "input",
};

export function loopDagKindLabel(node: LoopGraphNode | undefined): string {
  if (!node) return "step";
  return KIND_LABELS[node.kind] ?? nodeClassLabel(node);
}
