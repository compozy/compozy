import type { LoopFanoutRollup, LoopRosterNode } from "../types";
import type { LoopGraph } from "./loop-graph";
import {
  type LoopRosterState,
  type LoopStateChip,
  loopRosterStateChip,
} from "./loop-run-state-copy";

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
 * The shared lane/segment vocabulary. `never` is durable evidence that a route
 * branch was not taken. It is resolved work and stays visually neutral.
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

/**
 * "attempt 2" is metadata on the step, never a sibling step. A first attempt is
 * the unremarkable case and says nothing.
 *
 * One owner for the wording and the threshold: branch metadata and ordinary-step
 * metadata cannot disagree about when an attempt is worth mentioning.
 */
export function loopAttemptLabel(attempt: number): string | null {
  return attempt > 1 ? `attempt ${attempt}` : null;
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

/**
 * The rollup in words, for a fan too wide to name its branches.
 *
 * Once the branch chips fold away this sentence is the only place a partial or a
 * parked worker can still be read, so every state that carries meaning has to
 * appear here — a fan that quietly reads "10 done" when one of them came back
 * partial is the calm-but-false rollup Safety Invariant 12 forbids.
 */
function summarySentence(rollup: LoopFanoutRollup, branches: LoopFanOutBranch[]): string {
  const partial = branches.filter(branch => branch.chip.state === "partial").length;
  const parts: string[] = [`${rollup.done} done`];
  if (rollup.failed > 0) parts.push(`${rollup.failed} failed`);
  if (partial > 0) parts.push(`${partial} partial`);
  const running = branches.filter(branch => branch.segment === "active").length;
  if (running > 0) parts.push(`${running} still running`);
  const parked = branches.filter(branch => branch.segment === "parked").length;
  if (parked > 0) parts.push(`${parked} waiting`);
  const canceled = branches.filter(branch => branch.segment === "canceled").length;
  if (canceled > 0) parts.push(`${canceled} canceled`);
  const skipped = branches.filter(branch => branch.segment === "never").length;
  if (skipped > 0) parts.push(`${skipped} skipped`);
  return parts.join(" · ");
}

/**
 * The state a fan-out container wears, derived from the fate of its branches.
 *
 * One owner for a question three surfaces used to answer three ways. Trouble
 * outranks waiting, waiting outranks activity, and calm comes last — so a fan
 * never hides a failed or partial worker behind a healthy-looking chip. The
 * order is exhaustive over the roster vocabulary: a state missing from it would
 * fall through to the rollup arithmetic and read as calm.
 *
 * The calm tail matters as much as the head. `canceled` and `not_taken` count as
 * done in the server's rollup and are not work anyone is still owed, so a fan of
 * one success and one skipped branch reads succeeded rather than skipped.
 */
const CONTAINER_PRECEDENCE = [
  "failed",
  "quarantined",
  "partial",
  "control_pending",
  "awaiting_goal",
  "awaiting_child",
  "waiting",
  "retrying",
  "paused",
  "running",
  "queued",
  "pending",
  "succeeded",
  "canceled",
  "not_taken",
] as const satisfies readonly LoopRosterState[];

export function loopFanOutContainerState(
  states: readonly string[],
  rollup: Pick<LoopFanoutRollup, "done" | "total">
): string {
  for (const candidate of CONTAINER_PRECEDENCE) {
    if (candidate === "queued" && rollup.done > 0 && rollup.done < rollup.total) {
      // Worker completions and the next claims are separate commits. Once the
      // fan has made progress it remains live across that handoff instead of
      // flashing back to queued while unfinished branches still exist.
      return "running";
    }
    if (states.includes(candidate)) return candidate;
  }
  // No branch state we recognise: fall back on the served arithmetic rather than
  // inventing a state the roster never reported.
  return rollup.total > 0 && rollup.done === rollup.total ? "succeeded" : "pending";
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
    attemptLabel: loopAttemptLabel(node.attempt),
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
