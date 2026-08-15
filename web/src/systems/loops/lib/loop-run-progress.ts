import type { LoopRunGeneration, LoopRunRecord } from "../types";
import type { LoopGateVerdict } from "./loop-events";
import { isTerminalLoopStatus } from "./loop-formatters";
import type { LoopNodeLifecycle } from "./loop-node-lifecycle";
import { parkedNodeIds } from "./loop-node-lifecycle";

/**
 * The group progress model (redesign spec §5.2): the latest generation's fan-out
 * branches as equal-width segments, plus the plain-language meta line. Batch
 * sizes are not first-class, so segments never carry invented weights — one
 * branch, one equal segment.
 *
 * Parked accounting (US-019 AC-2, US-024 AC-1): a paused, quarantined, or
 * waiting node is *excluded from the denominator*, not counted as failing. Its
 * clock is suspended, so "2 of 3 counted tasks done · 1 paused excluded" is the
 * truthful reading — showing it as an unfinished group would imply work is owed
 * that nothing is spending attempts on.
 */

export type LoopProgressSegmentState =
  | "clean"
  | "active"
  | "redo"
  | "failed"
  | "parked"
  | "quarantined"
  | "idle";

export interface LoopRunProgressModel {
  /** One entry per fan-out branch; null hides the bar (no fan-out in this run). */
  segments: LoopProgressSegmentState[] | null;
  leftMeta: string;
  rightMeta: string;
  ariaLabel: string;
  /** Branches excluded from the count because the daemon parked their node. */
  parkedCount: number;
}

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

const CLEAN_STATUSES = new Set(["succeeded", "reused"]);
// `awaiting_child` is already covered by the `awaiting_` prefix rule in segmentState.
const ACTIVE_STATUSES = new Set(["running", "enqueued"]);

function segmentState(
  status: string,
  terminal: boolean,
  runFailed: boolean
): LoopProgressSegmentState {
  if (CLEAN_STATUSES.has(status)) return "clean";
  if (ACTIVE_STATUSES.has(status) || status.startsWith("awaiting_")) {
    // Nothing works on a terminal run — a frozen "active" branch reads idle.
    return terminal ? "idle" : "active";
  }
  if (status === "failed") return runFailed ? "failed" : "redo";
  return "idle";
}

interface FanOutBranches {
  /** Branch statuses ordered by ascending `item_index`. */
  statuses: string[];
  /** True when the node owning these branches is parked by the daemon. */
  parked: boolean;
  /** The node that owns these branches. */
  nodeId: string;
}

/** The latest generation's widest fan-out node (>1 distinct `item_index`). */
function latestFanOut(
  generations: readonly LoopRunGeneration[] | undefined,
  parked: ReadonlySet<string>
): FanOutBranches | null {
  if (!generations || generations.length === 0) return null;
  const latest = generations.reduce((max, gen) => Math.max(max, gen.generation), 0);
  const byNode = new Map<string, Map<number, string>>();
  for (const generation of generations) {
    if (generation.generation !== latest) continue;
    for (const output of generation.outputs) {
      if (typeof output.item_index !== "number") continue;
      const branches = byNode.get(output.node_id) ?? new Map<number, string>();
      branches.set(output.item_index, output.status);
      byNode.set(output.node_id, branches);
    }
  }
  let widestId: string | null = null;
  let widest: Map<number, string> | null = null;
  for (const [nodeId, branches] of byNode) {
    if (branches.size > 1 && (!widest || branches.size > widest.size)) {
      widest = branches;
      widestId = nodeId;
    }
  }
  if (!widest) return null;
  const statuses = [...widest.entries()].sort(([a], [b]) => a - b).map(([, s]) => s);
  return { statuses, parked: widestId !== null && parked.has(widestId), nodeId: widestId ?? "" };
}

function openPointsClause(openPoints: number, past: boolean): string {
  const noun = openPoints === 1 ? "open point" : "open points";
  return past ? `${openPoints} ${noun} remained` : `${openPoints} ${noun} from the last check`;
}

function rightMeta(run: LoopRunRecord, openPoints: number | null): string {
  if (run.status === "failed" && openPoints !== null && openPoints > 0) {
    return openPointsClause(openPoints, true);
  }
  if (openPoints !== null && openPoints > 0) return openPointsClause(openPoints, false);
  if (run.status === "watching") return "waiting for the next check";
  return "";
}

function redoClause(run: LoopRunRecord, activeCount: number): string {
  const redoing =
    run.status === "running" &&
    activeCount > 0 &&
    run.generation > 1 &&
    run.reattempt_strategy === "failed_only";
  if (!redoing) return "";
  return activeCount === 1
    ? " — the failed group is being redone"
    : ` — ${activeCount} groups are being redone`;
}

function leftMeta(run: LoopRunRecord, clean: number, total: number, activeCount: number): string {
  const base = `${clean} of ${total} groups clean`;
  switch (run.status) {
    case "failed":
      return `${base} when the run failed`;
    case "needs-approval":
      return `${base} — group work paused with the run`;
    case "paused":
      return `${base} — group work resumes with the run`;
    case "running":
      return `${base}${redoClause(run, activeCount)}`;
    default:
      return base;
  }
}

/** The single dominant reason branches were parked, for the exclusion clause. */
function parkedNoun(nodes: readonly LoopNodeLifecycle[], nodeId: string): string {
  const state = nodes.find(node => node.parked && node.nodeId === nodeId)?.state;
  if (state === "paused" || state === "quarantined" || state === "waiting") return state;
  return "parked";
}

/**
 * Builds the Progress panel model from the latest generation's outputs and the
 * last check's open points (`gate_verdict.blocking_issues` length, null before
 * any verdict). No fan-out in the graph → the bar hides and the meta line alone
 * summarizes the round.
 *
 * `nodes` carries the daemon's per-node lifecycle truth. Parked branches leave
 * the counted denominator entirely and are named in a trailing clause, so the
 * headline count only ever describes work the run is actually spending on.
 */
export function buildRunProgress(
  run: LoopRunRecord,
  generations: readonly LoopRunGeneration[] | undefined,
  openPoints: number | null,
  nodes: readonly LoopNodeLifecycle[] = []
): LoopRunProgressModel {
  const parked = parkedNodeIds(nodes);
  const fanOut = latestFanOut(generations, parked);
  if (!fanOut) {
    const round = `Round ${Math.max(run.generation, 1)}`;
    return {
      segments: null,
      leftMeta: round,
      rightMeta: rightMeta(run, openPoints),
      ariaLabel: round,
      parkedCount: 0,
    };
  }
  const terminal = isTerminalLoopStatus(run.status);
  const runFailed = run.status === "failed";
  const ownerState = nodes.find(node => node.nodeId === fanOut.nodeId)?.state;
  const parkedState: LoopProgressSegmentState =
    ownerState === "quarantined" ? "quarantined" : "parked";
  const segments = fanOut.parked
    ? fanOut.statuses.map<LoopProgressSegmentState>(() => parkedState)
    : fanOut.statuses.map(status => segmentState(status, terminal, runFailed));
  const parkedCount = segments.filter(
    state => state === "parked" || state === "quarantined"
  ).length;
  const counted = segments.length - parkedCount;
  const cleanCount = segments.filter(state => state === "clean").length;
  const activeCount = segments.filter(state => state === "active" || state === "redo").length;
  const noun = parkedNoun(nodes, fanOut.nodeId);
  const left =
    parkedCount === 0
      ? leftMeta(run, cleanCount, segments.length, activeCount)
      : counted === 0
        ? // Every branch of this node is parked, so there is no counted work at
          // all. "0 of 0 clean" would read as a failure; say what is true.
          `Nothing counting here — all ${parkedCount} groups ${noun}`
        : `${cleanCount} of ${counted} counted groups clean · ${parkedCount} ${noun} excluded`;
  return {
    segments,
    leftMeta: left,
    rightMeta: rightMeta(run, openPoints),
    ariaLabel: `Groups: ${left}`,
    parkedCount,
  };
}
