import type { LoopFanoutRollup, LoopRosterNode, LoopStepProgress } from "../types";
import type { LoopGateVerdict } from "./loop-events";
import { type LoopGraph, topoOrder } from "./loop-graph";
import {
  type LoopFanOutBand,
  type LoopProgressSegment,
  buildFanOutBand,
  progressSegmentForState,
  resolveFanOutBranches,
} from "./loop-run-fanout-band";
import {
  type LoopStateChip,
  isParkedRosterState,
  isSettledRosterState,
  loopParkReason,
  loopRosterStateChip,
} from "./loop-run-state-copy";

/**
 * Progress as steps and rounds — the signal a loop without fan-out was missing.
 *
 * The counts are the server's. `steps_done` / `steps_total` arrive on the
 * briefing already computed as settled action steps out of total action steps
 * for the round, and this model renders them rather than recounting (Safety
 * Invariant 12). What it derives is everything the numbers cannot say: which
 * step is which, how a fan-out spread, and — when nothing is moving — why.
 *
 * Two channels, deliberately: the segmented bar is magnitude, saying how much of
 * the round has settled. The alarm-bearing state lives one line below on each
 * step's chip, where tone, glyph and the literal word travel together. Colouring
 * the bar as well would be signal overload for one fact.
 */

/** How many fan-out branches a step row may name before it folds to lanes. */
const STEP_ROW_NAMED_BRANCH_LIMIT = 6;

/** The most recent verdict across gate nodes (highest generation wins). */
export function latestGateVerdict(
  gateVerdicts: Record<string, LoopGateVerdict>
): LoopGateVerdict | null {
  let latest: LoopGateVerdict | null = null;
  for (const verdict of Object.values(gateVerdicts)) {
    if (!latest || verdict.generation > latest.generation) latest = verdict;
  }
  return latest;
}

export interface LoopStepRow {
  key: string;
  nodeId: string;
  chip: LoopStateChip;
  /** Attempts are metadata on the step, never sibling steps. */
  attemptLabel: string | null;
  /** Present only on a fan-out container; the branches live inside it. */
  fanOut: LoopFanOutBand | null;
  /** True for control segments — they carry state but contribute no step. */
  isControl: boolean;
}

export interface LoopStepsProgressModel {
  /** "Step 4 of 6 · round 2". The round clause is absent on a single-pass round. */
  label: string;
  round: number;
  showRound: boolean;
  stepsDone: number;
  stepsTotal: number;
  /** One segment per action step, fan-out branches counted individually. */
  segments: LoopProgressSegment[];
  leftMeta: string;
  rightMeta: string;
  /**
   * Set only when every action step of the round is parked. The label states the
   * dominant reason instead of a percentage that has stopped meaning anything.
   */
  parkedReason: string | null;
  steps: LoopStepRow[];
  ariaLabel: string;
}

interface RosterIndex {
  byNode: Map<string, LoopRosterNode[]>;
  rollups: LoopFanoutRollup[];
  /** Roster rows already accounted for by a fan-out band. */
  claimed: Set<string>;
}

function rowKey(node: LoopRosterNode): string {
  return `${node.node_id}:${node.item_index}`;
}

function attemptLabel(node: LoopRosterNode): string | null {
  return node.attempt > 1 ? `attempt ${node.attempt}` : null;
}

function indexRoster(
  nodes: readonly LoopRosterNode[],
  rollups: readonly LoopFanoutRollup[],
  graph: LoopGraph | null,
  round: number
): RosterIndex {
  const inRound = nodes.filter(node => node.generation === round);
  const byNode = new Map<string, LoopRosterNode[]>();
  for (const node of inRound) {
    const bucket = byNode.get(node.node_id);
    if (bucket) bucket.push(node);
    else byNode.set(node.node_id, [node]);
  }
  const roundRollups = rollups.filter(rollup => rollup.generation === round);
  const claimed = new Set<string>();
  for (const rollup of roundRollups) {
    for (const branch of resolveFanOutBranches(rollup, inRound, graph)) {
      claimed.add(rowKey(branch));
    }
  }
  return { byNode, rollups: roundRollups, claimed };
}

/**
 * The state a fan-out container wears, derived from the fate of its branches.
 * Trouble outranks calm so a fan whose worker failed never reads as healthy.
 */
const CONTAINER_PRECEDENCE: LoopProgressSegment[] = [
  "failed",
  "parked",
  "active",
  "pending",
  "canceled",
  "never",
  "clean",
];

function containerState(band: LoopFanOutBand, rows: readonly LoopRosterNode[]): string {
  const segments = new Set(band.segments);
  const winner = CONTAINER_PRECEDENCE.find(segment => segments.has(segment));
  if (!winner) return band.total > 0 && band.done === band.total ? "succeeded" : "pending";
  const match = rows.find(row => progressSegmentForState(row.state) === winner);
  if (match) return match.state;
  return winner === "clean" ? "succeeded" : "pending";
}

function isControlNode(graph: LoopGraph | null, nodeId: string): boolean {
  const authored = graph?.nodes.find(node => node.id === nodeId);
  // An unauthored roster row is treated as an action step: it executed, so it
  // counts. Only a node the definition calls `control` contributes zero.
  return authored?.nodeClass === "control";
}

/** Graph order when the definition is readable, roster order otherwise. */
function orderedNodeIds(graph: LoopGraph | null, index: RosterIndex): string[] {
  const seen = new Set<string>();
  const ordered: string[] = [];
  const push = (nodeId: string) => {
    if (seen.has(nodeId)) return;
    seen.add(nodeId);
    ordered.push(nodeId);
  };
  if (graph) {
    for (const nodeId of topoOrder(graph)) {
      const isFanOut = index.rollups.some(rollup => rollup.node_id === nodeId);
      if (index.byNode.has(nodeId) || isFanOut) push(nodeId);
    }
  }
  for (const rollup of index.rollups) push(rollup.node_id);
  for (const nodeId of index.byNode.keys()) push(nodeId);
  return ordered;
}

function buildSteps(
  index: RosterIndex,
  graph: LoopGraph | null,
  nodesInRound: readonly LoopRosterNode[]
): LoopStepRow[] {
  const steps: LoopStepRow[] = [];
  for (const nodeId of orderedNodeIds(graph, index)) {
    const rollup = index.rollups.find(entry => entry.node_id === nodeId);
    if (rollup) {
      const band = buildFanOutBand({
        rollup,
        nodes: nodesInRound,
        graph,
        namedLimit: STEP_ROW_NAMED_BRANCH_LIMIT,
      });
      const rows = resolveFanOutBranches(rollup, nodesInRound, graph);
      steps.push({
        key: nodeId,
        nodeId,
        chip: loopRosterStateChip(containerState(band, rows)),
        attemptLabel: null,
        fanOut: band,
        isControl: false,
      });
      continue;
    }
    for (const node of index.byNode.get(nodeId) ?? []) {
      // A branch already drawn inside its fan-out never reappears as a sibling.
      if (index.claimed.has(rowKey(node))) continue;
      steps.push({
        key: rowKey(node),
        nodeId,
        chip: loopRosterStateChip(node.state),
        attemptLabel: attemptLabel(node),
        fanOut: null,
        isControl: isControlNode(graph, nodeId),
      });
    }
  }
  return steps;
}

/**
 * The bar's population: action steps only, `not_taken` removed.
 *
 * A control segment between two action steps contributes nothing and leaves no
 * gap — the numbering closes over it (US-006.EC-4). A branch the route provably
 * never took leaves the denominator entirely, because counting a road not taken
 * against the run would be a lie about its size (US-006.EC-2).
 */
function buildSegments(steps: readonly LoopStepRow[]): LoopProgressSegment[] {
  const segments: LoopProgressSegment[] = [];
  for (const step of steps) {
    if (step.isControl) continue;
    if (step.fanOut) {
      segments.push(...step.fanOut.segments.filter(segment => segment !== "never"));
      continue;
    }
    if (step.chip.state === "not_taken") continue;
    segments.push(progressSegmentForState(step.chip.state));
  }
  return segments;
}

/** The park reason shared by every action step, when there is exactly one. */
function dominantParkReason(
  nodesInRound: readonly LoopRosterNode[],
  graph: LoopGraph | null
): string | null {
  const actionable = nodesInRound.filter(
    node => !isControlNode(graph, node.node_id) && node.state !== "not_taken"
  );
  if (actionable.length === 0) return null;
  if (!actionable.every(node => isParkedRosterState(node.state))) return null;
  const counts = new Map<string, number>();
  for (const node of actionable) {
    counts.set(node.state, (counts.get(node.state) ?? 0) + 1);
  }
  let dominant = actionable[0].state;
  let best = 0;
  for (const [state, count] of counts) {
    if (count > best) {
      best = count;
      dominant = state;
    }
  }
  return loopParkReason(dominant);
}

function leftMeta(
  segments: readonly LoopProgressSegment[],
  settled: number,
  parkedReason: string | null
): string {
  if (parkedReason) return `Nothing is moving — ${parkedReason}`;
  const parked = segments.filter(segment => segment === "parked").length;
  const settledClause = `${settled} settled`;
  return parked > 0 ? `${settledClause} · ${parked} waiting` : settledClause;
}

export interface BuildStepsProgressInput {
  /** The served counts. Rendered, never recomputed. */
  progress: LoopStepProgress;
  nodes: readonly LoopRosterNode[];
  rollups: readonly LoopFanoutRollup[];
  graph: LoopGraph | null;
}

export function buildStepsProgress({
  progress,
  nodes,
  rollups,
  graph,
}: BuildStepsProgressInput): LoopStepsProgressModel {
  const round = progress.round;
  const index = indexRoster(nodes, rollups, graph, round);
  const nodesInRound = nodes.filter(node => node.generation === round);
  const steps = buildSteps(index, graph, nodesInRound);
  const segments = buildSegments(steps);
  const parkedReason = dominantParkReason(nodesInRound, graph);
  // Past round 1 the counter earns its place; on a single-pass run it is noise.
  const showRound = round > 1;
  const settled = nodesInRound.filter(
    node => !isControlNode(graph, node.node_id) && isSettledRosterState(node.state)
  ).length;
  const remaining = Math.max(progress.steps_total - progress.steps_done, 0);
  return {
    label: stepsLabel(progress, showRound),
    round,
    showRound,
    stepsDone: progress.steps_done,
    stepsTotal: progress.steps_total,
    segments,
    leftMeta: leftMeta(segments, settled, parkedReason),
    rightMeta: remaining > 0 ? `${remaining} to go` : "",
    parkedReason,
    steps,
    ariaLabel: ariaLabel(progress, segments),
  };
}

function stepsLabel(progress: LoopStepProgress, showRound: boolean): string {
  const base =
    progress.steps_total > 0
      ? `Step ${progress.steps_done} of ${progress.steps_total}`
      : "No steps in this round yet";
  return showRound ? `${base} · round ${progress.round}` : base;
}

function ariaLabel(progress: LoopStepProgress, segments: readonly LoopProgressSegment[]): string {
  if (segments.length === 0) return stepsLabel(progress, progress.round > 1);
  const counts = new Map<LoopProgressSegment, number>();
  for (const segment of segments) counts.set(segment, (counts.get(segment) ?? 0) + 1);
  const spoken: Record<LoopProgressSegment, string> = {
    clean: "settled",
    active: "running",
    parked: "waiting",
    failed: "failed",
    canceled: "canceled",
    never: "skipped",
    pending: "not started",
  };
  const parts = [...counts.entries()].map(([segment, count]) => `${count} ${spoken[segment]}`);
  return `${segments.length} steps: ${parts.join(", ")}`;
}
