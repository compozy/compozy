import type { LoopFanoutRollup, LoopRosterNode } from "../types";
import type { LoopGraph } from "./loop-graph";
import { type LoopStateChip, loopRosterStateChip } from "./loop-run-state-copy";

/**
 * Fan-out, drawn once and read twice.
 *
 * A fan-out never becomes sibling graph entities (US-011.EC-1) — but width, fate
 * and convergence have to be drawn, not implied by a fraction. Both readings of
 * this model (the S4 step row's band, the S5 DAG's bordered fan) speak the same
 * segment vocabulary, so neither invents a second bar.
 *
 * Counts are always the server's `fanout_rollups` entry. Rollups are derived,
 * never entered, and an agent reading the roster gets them without paging a
 * single item.
 */

/**
 * The shared lane/segment vocabulary. `never` is a branch a filter skipped: it
 * leaves the denominator and stays neutral, because absence is calm.
 */
export type LoopProgressSegment =
  | "clean"
  | "active"
  | "parked"
  | "failed"
  | "canceled"
  | "never"
  | "pending";

const SEGMENT_BY_STATE: Record<string, LoopProgressSegment> = {
  succeeded: "clean",
  partial: "clean",
  running: "active",
  retrying: "parked",
  waiting: "parked",
  paused: "parked",
  awaiting_child: "parked",
  control_pending: "parked",
  awaiting_goal: "parked",
  failed: "failed",
  quarantined: "failed",
  canceled: "canceled",
  pending: "pending",
  queued: "pending",
  not_taken: "never",
};

export function progressSegmentForState(state: string): LoopProgressSegment {
  return SEGMENT_BY_STATE[state] ?? "pending";
}

export interface LoopFanOutBranch {
  key: string;
  /** What the runtime calls this branch: an authored node id, or its item slot. */
  label: string;
  chip: LoopStateChip;
  segment: LoopProgressSegment;
  attemptLabel: string | null;
  nodeId: string;
  itemIndex: number;
}

export interface LoopFanOutBand {
  nodeId: string;
  done: number;
  total: number;
  failed: number;
  /**
   * Every branch this fan owns, always. The band knows what belongs to it even
   * when it is too wide to name them — a caller still has to keep those rows out
   * of the lane, and a model that discarded them would leak ten sibling nodes.
   */
  branches: LoopFanOutBranch[];
  /**
   * True once the fan exceeds the naming cap. The renderer then draws lanes and
   * a rollup sentence instead of a named list; the branches above stay intact.
   */
  wide: boolean;
  /** One lane per worker, in branch order — width and fate, drawn. */
  segments: LoopProgressSegment[];
  /** "7 done · 1 failed · 2 still running" — the rollup in words. */
  summary: string;
  /** The chip on the fan itself: "2/3", "3 of 3 done", "partial 7 of 10". */
  countLabel: string;
}

function attemptLabel(node: LoopRosterNode): string | null {
  // "attempt 2" is metadata on the step, never a sibling step. A first attempt
  // is the unremarkable case and says nothing.
  return node.attempt > 1 ? `attempt ${node.attempt}` : null;
}

/**
 * Resolves which roster rows belong to a fan-out.
 *
 * Two authored shapes reach the same place. A fan-out may spread one node across
 * item slots (same `node_id`, ascending `item_index`), or it may fan into
 * separately named nodes the graph draws downstream of it. The rollup names the
 * fan; the graph and the roster together name its branches.
 */
export function resolveFanOutBranches(
  rollup: LoopFanoutRollup,
  nodes: readonly LoopRosterNode[],
  graph: LoopGraph | null
): LoopRosterNode[] {
  const sameNode = nodes.filter(
    node => node.node_id === rollup.node_id && node.generation === rollup.generation
  );
  if (sameNode.length > 1) {
    return [...sameNode].sort((left, right) => left.item_index - right.item_index);
  }
  const downstream = new Set<string>();
  for (const edge of graph?.edges ?? []) {
    if (edge.from === rollup.node_id) downstream.add(edge.to);
  }
  if (downstream.size === 0) return sameNode;
  return nodes
    .filter(node => node.generation === rollup.generation && downstream.has(node.node_id))
    .sort((left, right) =>
      left.node_id === right.node_id
        ? left.item_index - right.item_index
        : left.node_id.localeCompare(right.node_id)
    );
}

function branchLabel(node: LoopRosterNode, fanOutNodeId: string): string {
  // A branch that carries its own authored name says it. One that is a slot of
  // the fan-out node says which slot — the runtime models no other identity for
  // it, and inventing one would be a fiction the CLI could not corroborate.
  return node.node_id === fanOutNodeId ? `item ${node.item_index}` : node.node_id;
}

function summarySentence(rollup: LoopFanoutRollup, branches: LoopFanOutBranch[]): string {
  const parts: string[] = [`${rollup.done} done`];
  if (rollup.failed > 0) parts.push(`${rollup.failed} failed`);
  const running = branches.filter(branch => branch.segment === "active").length;
  if (running > 0) parts.push(`${running} still running`);
  const parked = branches.filter(branch => branch.segment === "parked").length;
  if (parked > 0) parts.push(`${parked} waiting`);
  const skipped = branches.filter(branch => branch.segment === "never").length;
  if (skipped > 0) parts.push(`${skipped} skipped`);
  return parts.join(" · ");
}

function countLabel(rollup: LoopFanoutRollup): string {
  if (rollup.total > 0 && rollup.done === rollup.total && rollup.failed === 0) {
    return `${rollup.done} of ${rollup.total} done`;
  }
  if (rollup.failed > 0) {
    // The graph-eng lock: `partial` is spelled out and carries its coverage.
    return `partial ${rollup.done} of ${rollup.total}`;
  }
  return `${rollup.done}/${rollup.total}`;
}

export interface BuildFanOutBandInput {
  rollup: LoopFanoutRollup;
  nodes: readonly LoopRosterNode[];
  graph: LoopGraph | null;
  /**
   * How many branches may be named before the band folds to lanes plus a
   * sentence. The step row reads at six; the DAG card, being narrower, at four.
   */
  namedLimit: number;
}

export function buildFanOutBand({
  rollup,
  nodes,
  graph,
  namedLimit,
}: BuildFanOutBandInput): LoopFanOutBand {
  const rows = resolveFanOutBranches(rollup, nodes, graph);
  const branches: LoopFanOutBranch[] = rows.map(node => ({
    key: `${node.node_id}:${node.item_index}`,
    label: branchLabel(node, rollup.node_id),
    chip: loopRosterStateChip(node.state),
    segment: progressSegmentForState(node.state),
    attemptLabel: attemptLabel(node),
    nodeId: node.node_id,
    itemIndex: node.item_index,
  }));
  return {
    nodeId: rollup.node_id,
    done: rollup.done,
    total: rollup.total,
    failed: rollup.failed,
    branches,
    wide: branches.length > namedLimit,
    segments: branches.map(branch => branch.segment),
    summary: summarySentence(rollup, branches),
    countLabel: countLabel(rollup),
  };
}
