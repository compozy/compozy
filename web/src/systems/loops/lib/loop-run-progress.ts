import type { LoopRunGeneration, LoopRunRecord } from "../types";
import type { LoopGateVerdict } from "./loop-events";
import { isTerminalLoopStatus } from "./loop-formatters";

/**
 * The group progress model (redesign spec §5.2): the latest generation's fan-out
 * branches as equal-width segments, plus the plain-language meta line. Batch
 * sizes are not first-class, so segments never carry invented weights — one
 * branch, one equal segment.
 */

export type LoopProgressSegmentState = "clean" | "active" | "redo" | "failed" | "idle";

export interface LoopRunProgressModel {
  /** One entry per fan-out branch; null hides the bar (no fan-out in this run). */
  segments: LoopProgressSegmentState[] | null;
  leftMeta: string;
  rightMeta: string;
  ariaLabel: string;
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
}

/** The latest generation's widest fan-out node (>1 distinct `item_index`). */
function latestFanOut(
  generations: readonly LoopRunGeneration[] | undefined
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
  let widest: Map<number, string> | null = null;
  for (const branches of byNode.values()) {
    if (branches.size > 1 && (!widest || branches.size > widest.size)) {
      widest = branches;
    }
  }
  if (!widest) return null;
  const statuses = [...widest.entries()].sort(([a], [b]) => a - b).map(([, s]) => s);
  return { statuses };
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

/**
 * Builds the Progress panel model from the latest generation's outputs and the
 * last check's open points (`gate_verdict.blocking_issues` length, null before
 * any verdict). No fan-out in the graph → the bar hides and the meta line alone
 * summarizes the round.
 */
export function buildRunProgress(
  run: LoopRunRecord,
  generations: readonly LoopRunGeneration[] | undefined,
  openPoints: number | null
): LoopRunProgressModel {
  const fanOut = latestFanOut(generations);
  if (!fanOut) {
    const round = `Round ${Math.max(run.generation, 1)}`;
    return {
      segments: null,
      leftMeta: round,
      rightMeta: rightMeta(run, openPoints),
      ariaLabel: round,
    };
  }
  const terminal = isTerminalLoopStatus(run.status);
  const runFailed = run.status === "failed";
  const segments = fanOut.statuses.map(status => segmentState(status, terminal, runFailed));
  const cleanCount = segments.filter(state => state === "clean").length;
  const activeCount = segments.filter(state => state === "active" || state === "redo").length;
  const left = leftMeta(run, cleanCount, segments.length, activeCount);
  return {
    segments,
    leftMeta: left,
    rightMeta: rightMeta(run, openPoints),
    ariaLabel: `Groups: ${left}`,
  };
}
